import { useCallback, useEffect, useRef, useState } from "react";
import type Hls from "hls.js";
import { api, type PlayResponse, type TitleCredits } from "../api";
import { MusicVideoCredits, creditsPlacement, creditsShowing } from "./MusicVideoCredits";
import { detectCaps } from "../caps";

export interface PlayTarget {
  versionId: number;
  heading: string; // e.g. "Alien · Director's Cut"
  /** The title the file belongs to; when set, the position is saved for "Continue". */
  titleId?: number;
  /** Where to start, in seconds (resume). */
  startAt?: number;
  /** A music video's credits, shown MTV-style in the corner as it plays. */
  credits?: TitleCredits;
}

/** How often the position is saved while playing. */
const SAVE_EVERY_MS = 10_000;

/** Full-window player. Plays the file directly when the browser supports it,
 *  otherwise streams it converted (HLS) by the server. */
export function Player({
  target,
  onClose,
  upNext,
  onEnded,
}: {
  target: PlayTarget;
  onClose: () => void;
  /** Label of what plays next (an episode), shown with a button to skip to it. */
  upNext?: string;
  /** Called when the video finishes, e.g. to play the next episode. */
  onEnded?: () => void;
}) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [info, setInfo] = useState<PlayResponse | null>(null);
  const [error, setError] = useState("");
  const lastSave = useRef(0);
  const frame = useRef<HTMLDivElement>(null);
  const [showCredits, setShowCredits] = useState(false);
  const [creditsPlace, setCreditsPlace] = useState<ReturnType<typeof creditsPlacement>>();

  // Keep the credits in the picture's corner as the player resizes, goes
  // full screen or learns the video's size.
  useEffect(() => {
    const v = videoRef.current;
    if (!target.credits || !v) return;
    const place = () => {
      const p = creditsPlacement(v.clientWidth, v.clientHeight, v.videoWidth, v.videoHeight);
      const fr = frame.current;
      if (fr) {
        // The offsets are from the video element; add any gap between it and the frame.
        const fb = fr.getBoundingClientRect();
        const vb = v.getBoundingClientRect();
        p.right += Math.round(fb.right - vb.right);
        p.bottom += Math.round(fb.bottom - vb.bottom);
      }
      setCreditsPlace(p);
    };
    place();
    v.addEventListener("loadedmetadata", place);
    v.addEventListener("resize", place); // the video's own size changed (adaptive streams)
    const ro = typeof ResizeObserver !== "undefined" ? new ResizeObserver(place) : null;
    ro?.observe(v);
    document.addEventListener("fullscreenchange", place);
    return () => {
      v.removeEventListener("loadedmetadata", place);
      v.removeEventListener("resize", place);
      ro?.disconnect();
      document.removeEventListener("fullscreenchange", place);
    };
  }, [target.credits]);

  // The credits sit over the video, so going full screen takes the whole
  // player, not the <video> alone (which nothing can draw over).
  useEffect(() => {
    if (!target.credits) return;
    const onFull = () => {
      const v = videoRef.current;
      if (v && document.fullscreenElement === v && frame.current?.requestFullscreen) {
        document.exitFullscreen().then(() => frame.current?.requestFullscreen()).catch(() => {});
      }
    };
    document.addEventListener("fullscreenchange", onFull);
    return () => document.removeEventListener("fullscreenchange", onFull);
  }, [target.credits]);

  // Save the position now; at the end, save the full length so it counts as finished.
  const save = useCallback(
    (ended = false) => {
      const v = videoRef.current;
      if (!target.titleId || !v || !(v.currentTime > 0)) return;
      lastSave.current = Date.now();
      api.saveProgress(target.titleId, target.versionId, ended && isFinite(v.duration) ? v.duration : v.currentTime);
    },
    [target.titleId, target.versionId],
  );

  useEffect(() => {
    let hls: Hls | null = null;
    let sessionId: string | undefined;
    let cancelled = false;

    (async () => {
      try {
        const res = await api.play(target.versionId, detectCaps());
        sessionId = res.session_id;
        if (cancelled) {
          if (sessionId) api.stopPlayback(sessionId);
          return;
        }
        setInfo(res);
        const video = videoRef.current!;
        if (target.startAt && target.startAt > 0) {
          const start = target.startAt;
          video.addEventListener("loadedmetadata", () => (video.currentTime = start), { once: true });
        }
        if (res.mode === "direct") {
          video.src = res.url;
        } else {
          const { default: HlsJs } = await import("hls.js");
          if (HlsJs.isSupported()) {
            hls = new HlsJs({ fragLoadingTimeOut: 90_000, manifestLoadingTimeOut: 20_000, maxBufferLength: 60 });
            hls.on(HlsJs.Events.ERROR, (_e, data) => {
              if (data.fatal) setError(`Playback stopped: ${data.details}. Close the player and try again.`);
            });
            hls.loadSource(res.url);
            hls.attachMedia(video);
          } else if (video.canPlayType("application/vnd.apple.mpegurl")) {
            video.src = res.url; // Safari plays HLS itself
          } else {
            setError("This browser can't play streamed video.");
            return;
          }
        }
        video.play().catch(() => {
          /* autoplay may be blocked; the controls are there */
        });
      } catch (e) {
        if (!cancelled) setError((e as Error).message);
      }
    })();

    const onUnload = () => {
      save();
      if (sessionId) api.stopPlayback(sessionId);
    };
    window.addEventListener("pagehide", onUnload);
    return () => {
      cancelled = true;
      save();
      window.removeEventListener("pagehide", onUnload);
      hls?.destroy();
      if (sessionId) api.stopPlayback(sessionId);
    };
  }, [target.versionId]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const status = !info
    ? "Starting…"
    : info.mode === "direct"
      ? "Playing the original file"
      : `Converting on ${info.device ?? "the server"} for this browser`;

  return (
    <div className="player" role="dialog" aria-modal="true" aria-label={`Playing ${target.heading}`}>
      <div className="player-bar">
        <div className="player-meta">
          <p className="player-title">{target.heading}</p>
          <p className="player-status" title={info?.reasons.join("; ")}>
            {error || status}
          </p>
        </div>
        <div className="player-actions">
          {upNext && onEnded ? (
            <button className="cv-btn" onClick={onEnded} title={`Play ${upNext}`}>
              Next: {upNext}
            </button>
          ) : null}
          <button className="cv-btn" onClick={onClose} autoFocus>
            Close
          </button>
        </div>
      </div>
      <div className="player-frame" ref={frame}>
      <video
        ref={videoRef}
        className="player-video"
        controls
        playsInline
        onTimeUpdate={(e) => {
          if (Date.now() - lastSave.current > SAVE_EVERY_MS) save();
          if (target.credits) setShowCredits(creditsShowing(e.currentTarget.currentTime, e.currentTarget.duration));
        }}
        onPause={() => save()}
        onEnded={() => {
          save(true);
          onEnded?.();
        }}
      />
      {target.credits ? <MusicVideoCredits credits={target.credits} visible={showCredits} place={creditsPlace} /> : null}
      </div>
    </div>
  );
}
