import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { api, type Chapter } from "../api";
import { chapterAt, formatClock } from "../format";

/** One song or audiobook file in the play queue. */
export interface QueueItem {
  versionId: number;
  title: string;
  artist: string;
  album: string;
  coverUrl?: string;
  /** Where its album or book page is. */
  href?: string;
  /** Audiobooks: the title whose listening position is saved, the file's chapters, and where to start. */
  book?: { titleId: number; chapters: Chapter[]; startAt?: number };
}

interface AudioApi {
  queue: QueueItem[];
  index: number;
  playing: boolean;
  /** Seconds into the current file. */
  time: number;
  /** Replace the queue and start at item start. */
  play: (queue: QueueItem[], start?: number) => void;
  toggle: () => void;
  /** Jump within the current file. */
  seek: (seconds: number) => void;
}

const AudioContext = createContext<AudioApi | null>(null);

export function useAudio(): AudioApi {
  const a = useContext(AudioContext);
  if (!a) throw new Error("useAudio needs an AudioProvider");
  return a;
}

const SPEEDS = [1, 1.25, 1.5, 1.75, 2, 0.75];
const SAVE_EVERY_MS = 15_000;

/**
 * Keeps one <audio> element for the whole app, so music and audiobooks carry
 * on while you move between pages, and shows the now-playing bar. For
 * audiobooks it saves the listening position as you go.
 */
export function AudioProvider({ children }: { children: ReactNode }) {
  const audioRef = useRef<HTMLAudioElement>(null);
  const [queue, setQueue] = useState<QueueItem[]>([]);
  const [index, setIndex] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [time, setTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [speed, setSpeed] = useState(1);
  const [error, setError] = useState("");
  const lastSave = useRef(0);

  const current = queue[index];
  const book = current?.book;

  const saveProgress = useCallback(() => {
    const a = audioRef.current;
    if (!a || !current?.book || !Number.isFinite(a.currentTime)) return;
    lastSave.current = Date.now();
    api.saveProgress(current.book.titleId, current.versionId, a.currentTime);
  }, [current]);

  // Load and start the current file when it changes.
  useEffect(() => {
    const a = audioRef.current;
    if (!a || !current) return;
    setError("");
    setTime(0);
    a.src = `/api/versions/${current.versionId}/file`;
    const start = current.book?.startAt ?? 0;
    if (start > 0) {
      a.addEventListener("loadedmetadata", () => (a.currentTime = start), { once: true });
    }
    a.playbackRate = current.book ? speed : 1;
    a.play().catch(() => {
      /* autoplay refused or file unplayable: the error handler reports the latter */
    });
    // speed is applied separately below; reloading the file for it would restart it
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current]);

  useEffect(() => {
    if (audioRef.current) audioRef.current.playbackRate = book ? speed : 1;
  }, [speed, book]);

  // Save the position when the page is closed mid-book.
  useEffect(() => {
    window.addEventListener("pagehide", saveProgress);
    return () => window.removeEventListener("pagehide", saveProgress);
  }, [saveProgress]);

  const play = useCallback(
    (q: QueueItem[], start = 0) => {
      saveProgress();
      setQueue(q);
      setIndex(start);
    },
    [saveProgress],
  );

  const toggle = useCallback(() => {
    const a = audioRef.current;
    if (!a) return;
    if (a.paused) a.play().catch(() => {});
    else a.pause();
  }, []);

  const seek = useCallback((s: number) => {
    const a = audioRef.current;
    if (a) a.currentTime = Math.max(0, Math.min(s, a.duration || s));
  }, []);

  const next = useCallback(() => setIndex((i) => (i + 1 < queue.length ? i + 1 : i)), [queue.length]);
  const prev = useCallback(() => {
    const a = audioRef.current;
    // Like most players: back to the start of the song unless you're in its first seconds.
    if (a && a.currentTime > 3) a.currentTime = 0;
    else setIndex((i) => Math.max(0, i - 1));
  }, []);

  const close = useCallback(() => {
    saveProgress();
    audioRef.current?.pause();
    setQueue([]);
    setIndex(0);
  }, [saveProgress]);

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
    ms.setActionHandler("seekbackward", current.book ? () => seek((audioRef.current?.currentTime ?? 0) - 30) : null);
    ms.setActionHandler("seekforward", current.book ? () => seek((audioRef.current?.currentTime ?? 0) + 30) : null);
  }, [current, index, queue.length, next, prev, toggle, seek]);

  const value = useMemo(() => ({ queue, index, playing, time, play, toggle, seek }), [queue, index, playing, time, play, toggle, seek]);
  const chapter = book ? chapterAt(book.chapters, time) : undefined;

  return (
    <AudioContext.Provider value={value}>
      {children}
      <audio
        ref={audioRef}
        onPlay={() => setPlaying(true)}
        onPause={() => {
          setPlaying(false);
          saveProgress();
        }}
        onTimeUpdate={(e) => {
          setTime(e.currentTarget.currentTime);
          if (book && Date.now() - lastSave.current > SAVE_EVERY_MS) saveProgress();
        }}
        onDurationChange={(e) => setDuration(e.currentTarget.duration)}
        onEnded={() => (index + 1 < queue.length ? next() : setPlaying(false))}
        onError={() => current && setError("This browser can't play this file's format.")}
        preload="auto"
      />
      {current ? (
        <div className={`nowplaying${book ? " nowplaying-book" : ""}`} role="region" aria-label="Now playing">
          <div className="np-song">
            {current.coverUrl ? (
              <img className="np-cover" src={current.coverUrl} alt="" />
            ) : (
              <span className="np-cover np-cover-blank" aria-hidden="true" />
            )}
            <div className="np-text">
              <p className="np-title">{current.title}</p>
              <p className="np-sub">{error || (chapter ? chapter.name : [current.artist, current.album].filter(Boolean).join(" · "))}</p>
            </div>
          </div>

          <div className="np-controls">
            {book ? (
              <button className="np-btn np-skip" onClick={() => seek(time - 30)} aria-label="Back 30 seconds">
                <Icon d="M12 5V1L7 6l5 5V7a6 6 0 1 1-6 6H4a8 8 0 1 0 8-8z" />
                <span aria-hidden="true">30</span>
              </button>
            ) : (
              <button className="np-btn" onClick={prev} aria-label="Previous song">
                <Icon d="M6 5h2v14H6zM20 5v14L9 12z" />
              </button>
            )}
            <button className="np-btn np-play" onClick={toggle} aria-label={playing ? "Pause" : "Play"}>
              {playing ? <Icon d="M6 5h4v14H6zM14 5h4v14h-4z" /> : <Icon d="M7 4v16l13-8z" />}
            </button>
            {book ? (
              <button className="np-btn np-skip" onClick={() => seek(time + 30)} aria-label="Forward 30 seconds">
                <Icon d="M12 5V1l5 5-5 5V7a6 6 0 1 0 6 6h2a8 8 0 1 1-8-8z" />
                <span aria-hidden="true">30</span>
              </button>
            ) : (
              <button className="np-btn" onClick={next} disabled={index + 1 >= queue.length} aria-label="Next song">
                <Icon d="M16 5h2v14h-2zM4 5v14l11-7z" />
              </button>
            )}
          </div>

          <div className="np-seek">
            <span className="np-time">{formatClock(time)}</span>
            <input
              type="range"
              min={0}
              max={duration || 0}
              step={1}
              value={Math.min(time, duration || 0)}
              onChange={(e) => seek(Number(e.target.value))}
              aria-label={book ? "Position in book" : "Position in song"}
              aria-valuetext={`${formatClock(time)} of ${formatClock(duration)}`}
            />
            <span className="np-time">{book ? `-${formatClock(Math.max(0, duration - time))}` : formatClock(duration)}</span>
          </div>

          <div className="np-end">
            {book ? (
              <button
                className="np-speed"
                onClick={() => setSpeed(SPEEDS[(SPEEDS.indexOf(speed) + 1) % SPEEDS.length])}
                aria-label={`Playback speed ${speed} times; change`}
              >
                {speed}×
              </button>
            ) : (
              <span className="np-count">
                {index + 1} of {queue.length}
              </span>
            )}
            <button className="np-btn" onClick={close} aria-label="Stop and close the player">
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
    <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true" focusable="false">
      <path d={d} fill="currentColor" />
    </svg>
  );
}
