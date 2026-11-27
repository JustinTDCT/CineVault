import { useEffect, useState } from "react";
import { api, type AlbumEdition, type Progress, type Title } from "../api";
import { ComicReader, type ComicIssue } from "../components/ComicReader";
import { trackFile } from "../format";

/** A comic series' issues as covers, with Continue reading. */
export function ComicIssues({ title }: { title: Title }) {
  const [editions, setEditions] = useState<AlbumEdition[] | null>(null);
  const [saved, setSaved] = useState<Progress | null>(null);
  const [error, setError] = useState("");
  const [open, setOpen] = useState<{ start: number; page: number } | null>(null);

  useEffect(() => {
    api
      .tracks(title.id)
      .then(setEditions)
      .catch((e: Error) => setError(e.message));
  }, [title.id]);

  // Reload the saved place whenever the reader closes.
  useEffect(() => {
    if (open) return;
    api
      .progress(title.id)
      .then(setSaved)
      .catch(() => setSaved(null));
  }, [title.id, open]);

  if (error) {
    return (
      <div className="cv-notice cv-notice--error" role="alert">
        <p>{error}</p>
      </div>
    );
  }
  if (!editions) return <p className="cv-sub">Loading issues…</p>;
  const tracks = (editions[0]?.tracks ?? []).filter((t) => trackFile(t));
  if (tracks.length === 0) return <p className="cv-sub">The files for this series are missing. Scan the library again.</p>;

  const issues: ComicIssue[] = tracks.map((t) => {
    const f = trackFile(t)!;
    return { versionId: f.id, name: t.name, pages: f.page_count || 1 };
  });

  const savedIdx = saved ? issues.findIndex((i) => i.versionId === saved.version_id) : -1;
  const savedPage = savedIdx >= 0 ? Math.round(saved!.position_seconds) : 0;
  // Finished an issue? Continue with the next one.
  let cont: { start: number; page: number } | null = null;
  if (savedIdx >= 0) {
    if (savedPage < issues[savedIdx].pages - 1) cont = { start: savedIdx, page: savedPage };
    else if (savedIdx + 1 < issues.length) cont = { start: savedIdx + 1, page: 0 };
  }

  return (
    <section className="album" aria-label="Issues">
      <div className="book-actions">
        {cont ? (
          <>
            <button className="cv-btn cv-btn--primary" onClick={() => setOpen(cont)}>
              Continue {issues[cont.start].name}
              {cont.page > 0 ? `, page ${cont.page + 1}` : ""}
            </button>
            <button className="cv-btn" onClick={() => setOpen({ start: 0, page: 0 })}>
              Start from the first issue
            </button>
          </>
        ) : (
          <button className="cv-btn cv-btn--primary" onClick={() => setOpen({ start: 0, page: 0 })}>
            Read from the first issue
          </button>
        )}
      </div>

      <h2 className="cv-section-title">{issues.length === 1 ? "1 issue" : `${issues.length} issues`}</h2>
      <ol className="cv-grid comic-grid">
        {issues.map((iss, i) => (
          <li key={iss.versionId} className="cv-card">
            <button
              className="cv-card__art comic-cover"
              aria-label={`Read ${iss.name}`}
              onClick={() => setOpen({ start: i, page: i === savedIdx && savedPage < iss.pages - 1 ? savedPage : 0 })}
            >
              <img className="poster" src={`/api/versions/${iss.versionId}/cover`} alt="" loading="lazy" decoding="async" />
              {i === savedIdx && savedPage > 0 ? (
                <span className="comic-progress" style={{ width: `${Math.min(100, ((savedPage + 1) / iss.pages) * 100)}%` }} aria-hidden="true" />
              ) : null}
            </button>
            <h3 className="cv-card__title">{iss.name}</h3>
            <p className="cv-card__meta">{iss.pages} pages</p>
          </li>
        ))}
      </ol>

      {open ? (
        <ComicReader titleId={title.id} series={title.name} issues={issues} start={open.start} startPage={open.page} onClose={() => setOpen(null)} />
      ) : null}
    </section>
  );
}
