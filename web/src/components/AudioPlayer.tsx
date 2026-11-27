import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { formatClock } from "../format";

/** One song in the play queue. */
export interface QueueItem {
  versionId: number;
  title: string;
  artist: string;
  album: string;
  coverUrl?: string;
  /** Where the album page is, so the bar can link back to it. */
  href?: string;
}

interface AudioApi {
  queue: QueueItem[];
  index: number;
  playing: boolean;
  /** Replace the queue and start at item start. */
  play: (queue: QueueItem[], start?: number) => void;
  toggle: () => void;
}

const AudioContext = createContext<AudioApi | null>(null);

export function useAudio(): AudioApi {
  const a = useContext(AudioContext);
  if (!a) throw new Error("useAudio needs an AudioProvider");
  return a;
}

/**
 * Keeps one <audio> element for the whole app, so music carries on while you
 * move between pages, and shows the now-playing bar at the bottom.
 */
export function AudioProvider({ children }: { children: ReactNode }) {
  const audioRef = useRef<HTMLAudioElement>(null);
  const [queue, setQueue] = useState<QueueItem[]>([]);
  const [index, setIndex] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [time, setTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [error, setError] = useState("");

  const current = queue[index];

  // Load and start the current song when it changes.
  useEffect(() => {
    const a = audioRef.current;
    if (!a || !current) return;
    setError("");
    setTime(0);
    a.src = `/api/versions/${current.versionId}/file`;
    a.play().catch(() => {
      /* autoplay refused or file unplayable: the error handler reports the latter */
    });
  }, [current]);

  const play = useCallback((q: QueueItem[], start = 0) => {
    setQueue(q);
    setIndex(start);
  }, []);

  const toggle = useCallback(() => {
    const a = audioRef.current;
    if (!a) return;
    if (a.paused) a.play().catch(() => {});
    else a.pause();
  }, []);

  const next = useCallback(
    () => setIndex((i) => (i + 1 < queue.length ? i + 1 : i)),
    [queue.length],
  );
  const prev = useCallback(() => {
    const a = audioRef.current;
    // Like most players: back to the start of the song unless you're in its first seconds.
    if (a && a.currentTime > 3) a.currentTime = 0;
    else setIndex((i) => Math.max(0, i - 1));
  }, []);

  const close = useCallback(() => {
    audioRef.current?.pause();
    setQueue([]);
    setIndex(0);
  }, []);

  // The operating system's media keys and lock-screen controls.
  useEffect(() => {
    const ms = navigator.mediaSession;
    if (!ms || !current) return;
    ms.metadata = new MediaMetadata({
      title: current.title,
      artist: current.artist,
      album: current.album,
      artwork: current.coverUrl ? [{ src: current.coverUrl }] : [],
    });
    ms.setActionHandler("play", toggle);
    ms.setActionHandler("pause", toggle);
    ms.setActionHandler("nexttrack", index + 1 < queue.length ? next : null);
    ms.setActionHandler("previoustrack", prev);
  }, [current, index, queue.length, next, prev, toggle]);

  const api = useMemo(
    () => ({ queue, index, playing, play, toggle }),
    [queue, index, playing, play, toggle],
  );

  return (
    <AudioContext.Provider value={api}>
      {children}
      <audio
        ref={audioRef}
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
        onTimeUpdate={(e) => setTime(e.currentTarget.currentTime)}
        onDurationChange={(e) => setDuration(e.currentTarget.duration)}
        onEnded={() => (index + 1 < queue.length ? next() : setPlaying(false))}
        onError={() =>
          current && setError("This browser can't play this file's format.")
        }
        preload="auto"
      />
      {current ? (
        <div className="nowplaying" role="region" aria-label="Now playing">
          <div className="np-song">
            {current.coverUrl ? (
              <img className="np-cover" src={current.coverUrl} alt="" />
            ) : (
              <span className="np-cover np-cover-blank" aria-hidden="true" />
            )}
            <div className="np-text">
              <p className="np-title">{current.title}</p>
              <p className="np-sub">
                {error ||
                  [current.artist, current.album].filter(Boolean).join(" · ")}
              </p>
            </div>
          </div>

          <div className="np-controls">
            <button
              className="np-btn"
              onClick={prev}
              aria-label="Previous song"
            >
              <Icon d="M6 5h2v14H6zM20 5v14L9 12z" />
            </button>
            <button
              className="np-btn np-play"
              onClick={toggle}
              aria-label={playing ? "Pause" : "Play"}
            >
              {playing ? (
                <Icon d="M6 5h4v14H6zM14 5h4v14h-4z" />
              ) : (
                <Icon d="M7 4v16l13-8z" />
              )}
            </button>
            <button
              className="np-btn"
              onClick={next}
              disabled={index + 1 >= queue.length}
              aria-label="Next song"
            >
              <Icon d="M16 5h2v14h-2zM4 5v14l11-7z" />
            </button>
          </div>

          <div className="np-seek">
            <span className="np-time">{formatClock(time)}</span>
            <input
              type="range"
              min={0}
              max={duration || 0}
              step={1}
              value={Math.min(time, duration || 0)}
              onChange={(e) => {
                if (audioRef.current)
                  audioRef.current.currentTime = Number(e.target.value);
              }}
              aria-label="Position in song"
              aria-valuetext={`${formatClock(time)} of ${formatClock(duration)}`}
            />
            <span className="np-time">{formatClock(duration)}</span>
          </div>

          <div className="np-end">
            <span className="np-count">
              {index + 1} of {queue.length}
            </span>
            <button
              className="np-btn"
              onClick={close}
              aria-label="Stop and close the player"
            >
              <Icon d="M6.4 5 5 6.4 10.6 12 5 17.6 6.4 19l5.6-5.6 5.6 5.6 1.4-1.4-5.6-5.6L19 6.4 17.6 5 12 10.6z" />
            </button>
          </div>
        </div>
      ) : null}
    </AudioContext.Provider>
  );
}

function Icon({ d }: { d: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      width="20"
      height="20"
      aria-hidden="true"
      focusable="false"
    >
      <path d={d} fill="currentColor" />
    </svg>
  );
}
