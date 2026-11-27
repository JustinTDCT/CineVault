import { useEffect, useState } from "react";
import { api, type AlbumEdition, type Chapter, type Progress, type Title, type Track } from "../api";
import { useAudio, type QueueItem } from "../components/AudioPlayer";
import { chapterAt, coverUrl, formatClock, formatRuntime, timeLeft, trackFile } from "../format";

/**
 * An audiobook's chapters (or parts, for a book split into several files),
 * with a Resume button that picks up where you stopped.
 */
export function BookChapters({ title }: { title: Title }) {
  const [editions, setEditions] = useState<AlbumEdition[] | null>(null);
  const [saved, setSaved] = useState<Progress | null>(null);
  const [error, setError] = useState("");
  const audio = useAudio();

  useEffect(() => {
    Promise.all([api.tracks(title.id), api.progress(title.id)])
      .then(([eds, p]) => {
        setEditions(eds);
        setSaved(p);
      })
      .catch((e: Error) => setError(e.message));
  }, [title.id]);

  if (error) {
    return (
      <div className="notice notice-error" role="alert">
        <p>{error}</p>
      </div>
    );
  }
  if (!editions) return <p className="page-sub">Loading chapters…</p>;
  const parts = editions[0]?.tracks.filter((t) => trackFile(t)) ?? [];
  if (parts.length === 0) return <p className="page-sub">The files for this book are missing. Scan the library again.</p>;

  const item = (t: Track, startAt = 0): QueueItem => {
    const f = trackFile(t)!;
    return {
      versionId: f.id,
      title: title.name,
      artist: title.artist ?? "",
      album: parts.length > 1 ? t.name : "",
      coverUrl: coverUrl(title),
      href: `/libraries/${title.library_id}/titles/${title.id}`,
      book: { titleId: title.id, chapters: f.chapters ?? [], startAt },
    };
  };
  const playFrom = (partIndex: number, seconds: number) => {
    // Already playing that file: just move within it.
    const f = trackFile(parts[partIndex])!;
    if (audio.queue[audio.index]?.versionId === f.id && audio.queue[audio.index]?.book?.titleId === title.id) {
      audio.seek(seconds);
      if (!audio.playing) audio.toggle();
      return;
    }
    audio.play(
      parts.map((t, i) => item(t, i === partIndex ? seconds : 0)),
      partIndex,
    );
  };

  // What's playing now, if it's this book.
  const playingVersion = audio.queue[audio.index]?.book?.titleId === title.id ? audio.queue[audio.index].versionId : undefined;
  const nowAt = playingVersion ? audio.time : undefined;

  // Where to resume: the saved part and position, unless it's in the last minute (or last 2% of a short book).
  const savedPart = saved ? parts.findIndex((t) => trackFile(t)!.id === saved.version_id) : -1;
  const total = parts.reduce((s, t) => s + (trackFile(t)!.duration_seconds ?? 0), 0);
  const before = savedPart > 0 ? parts.slice(0, savedPart).reduce((s, t) => s + (trackFile(t)!.duration_seconds ?? 0), 0) : 0;
  const resumeAt = savedPart >= 0 && saved ? saved.position_seconds : 0;
  const finished = savedPart === parts.length - 1 && resumeAt > 5 && total - (before + resumeAt) < Math.min(60, total * 0.02);
  const canResume = savedPart >= 0 && resumeAt > 5 && !finished;
  const resumeChapter = canResume ? chapterAt(trackFile(parts[savedPart])!.chapters, resumeAt) : undefined;

  return (
    <section className="album" aria-label="Chapters">
      <div className="book-actions">
        {canResume ? (
          <>
            <button className="btn btn-primary" onClick={() => playFrom(savedPart, resumeAt)}>
              Resume{resumeChapter ? ` ${resumeChapter.name}` : ""} at {formatClock(resumeAt)}
            </button>
            <span className="hint">{timeLeft(total, before + resumeAt)}</span>
            <button className="btn" onClick={() => playFrom(0, 0)}>
              Start from the beginning
            </button>
          </>
        ) : (
          <>
            <button className="btn btn-primary" onClick={() => playFrom(0, 0)}>
              {finished ? "Listen again" : "Play"}
            </button>
            <span className="hint">{formatRuntime(total)}</span>
          </>
        )}
      </div>

      {parts.map((t, pi) => {
        const f = trackFile(t)!;
        const chapters: Chapter[] = f.chapters?.length ? f.chapters : [{ start: 0, end: f.duration_seconds ?? 0, name: t.name }];
        const current = f.id === playingVersion && nowAt !== undefined ? chapterAt(chapters, nowAt) : undefined;
        return (
          <div key={t.id} className="disc">
            {parts.length > 1 ? <h3 className="disc-name">{t.name}</h3> : <h2 className="section-title">{chapters.length} chapters</h2>}
            <ol className="tracklist">
              {chapters.map((c, ci) => {
                const now = current === c;
                return (
                  <li key={ci} className={`track${now ? " track-now" : ""}`}>
                    <button
                      className="track-play"
                      onClick={() => playFrom(pi, c.start)}
                      aria-label={`Play ${c.name}`}
                      aria-current={now ? "true" : undefined}
                    >
                      <span className="track-no" aria-hidden="true">
                        {now && audio.playing ? <span className="track-eq" /> : ci + 1}
                      </span>
                      <span className="track-name">{c.name}</span>
                      <span className="track-format">{formatClock(c.start)}</span>
                      <span className="track-time">{formatClock(c.end - c.start)}</span>
                    </button>
                  </li>
                );
              })}
            </ol>
          </div>
        );
      })}
    </section>
  );
}
