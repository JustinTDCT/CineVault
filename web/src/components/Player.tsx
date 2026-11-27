import { useEffect, useRef, useState } from "react";
import type Hls from "hls.js";
import { api, type PlayResponse } from "../api";
import { detectCaps } from "../caps";

export interface PlayTarget {
  versionId: number;
  heading: string; // e.g. "Alien · Director's Cut"
}

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

    const onUnload = () => sessionId && api.stopPlayback(sessionId);
    window.addEventListener("pagehide", onUnload);
    return () => {
      cancelled = true;
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
      <video ref={videoRef} className="player-video" controls playsInline onEnded={onEnded} />
    </div>
  );
}
