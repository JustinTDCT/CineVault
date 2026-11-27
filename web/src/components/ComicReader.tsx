import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "../api";

export interface ComicIssue {
  versionId: number;
  name: string;
  pages: number;
}

type Fit = "page" | "width";

function load<T>(key: string, fallback: T): T {
  try {
    const v = localStorage.getItem(key);
    return v ? (JSON.parse(v) as T) : fallback;
  } catch {
    return fallback;
  }
}

function store(key: string, v: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(v));
  } catch {
    /* settings just aren't remembered */
  }
}

const pageUrl = (versionId: number, n: number) => `/api/versions/${versionId}/pages/${n}`;

/**
 * Reads a comic one page (or two-page spread) at a time. Pages come from the
 * server as images; the next few are fetched ahead so turning is instant.
 */
export function ComicReader({
  titleId,
  series,
  issues,
  start,
  startPage,
  onClose,
}: {
  titleId: number;
  series: string;
  issues: ComicIssue[];
  start: number;
  startPage: number;
  onClose: () => void;
}) {
  const [idx, setIdx] = useState(start);
  const [page, setPage] = useState(startPage);
  const [fit, setFit] = useState<Fit>(() => load("comic.fit", "page"));
  const [spread, setSpread] = useState<boolean>(() => load("comic.spread", false));
  const [error, setError] = useState("");
  const saveTimer = useRef<number | undefined>(undefined);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;

  const issue = issues[idx];
  const last = issue.pages - 1;
  // In spreads, the cover stands alone and then pages pair up: 1-2, 3-4...
  // Spreads only where there's room for them.
  const wideEnough = typeof window === "undefined" || window.innerWidth > 720;
  const second = spread && wideEnough && page > 0 && page < last ? page + 1 : -1;
  const step = second >= 0 ? 2 : 1;

  const go = useCallback(
    (to: number) => {
      if (to < 0) {
        if (idx > 0) {
          setIdx(idx - 1);
          setPage(Math.max(0, issues[idx - 1].pages - 1));
        }
        return;
      }
      if (to > last) {
        if (idx + 1 < issues.length) {
          setIdx(idx + 1);
          setPage(0);
        }
        return;
      }
      // Keep spreads aligned on odd pages.
      setPage(spread && to > 0 ? to - ((to - 1) % 2) : to);
    },
    [idx, issues, last, spread],
  );

  const next = useCallback(() => go(page + step), [go, page, step]);
  const prev = useCallback(() => go(page - (spread && page > 1 ? 2 : 1)), [go, page, spread]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "ArrowRight" || e.key === " " || e.key === "PageDown") {
        e.preventDefault();
        next();
      } else if (e.key === "ArrowLeft" || e.key === "PageUp") {
        e.preventDefault();
        prev();
      } else if (e.key === "Escape") closeRef.current();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [next, prev]);

  // Fetch the next few pages ahead of time.
  useEffect(() => {
    setError("");
    for (let n = page + 1; n <= Math.min(last, page + 4); n++) {
      const img = new Image();
      img.src = pageUrl(issue.versionId, n);
    }
  }, [issue.versionId, page, last]);

  // Save the place a moment after turning.
  useEffect(() => {
    window.clearTimeout(saveTimer.current);
    saveTimer.current = window.setTimeout(() => api.saveProgress(titleId, issue.versionId, page), 800);
  }, [titleId, issue.versionId, page]);
  useEffect(() => () => window.clearTimeout(saveTimer.current), []);

  useEffect(() => store("comic.fit", fit), [fit]);
  useEffect(() => store("comic.spread", spread), [spread]);

  const atEnd = page + step > last;
  const nextIssue = issues[idx + 1];

  return (
    <div className="reader comic" role="dialog" aria-modal="true" aria-label={`Reading ${series}, ${issue.name}`}>
      <div className="reader-bar">
        <div className="reader-heading">
          <h2 className="player-title">{series}</h2>
          <p className="player-status">{error || issue.name}</p>
        </div>
        <div className="reader-tools">
          <button className="btn comic-opt" aria-pressed={spread} onClick={() => setSpread(!spread)}>
            Two pages
          </button>
          <button
            className="btn"
            onClick={() => setFit(fit === "page" ? "width" : "page")}
            aria-label={fit === "page" ? "Fit to width" : "Fit whole page"}
          >
            {fit === "page" ? "Fit width" : "Fit page"}
          </button>
          <button className="btn" onClick={onClose}>
            Close
          </button>
        </div>
      </div>

      <div className={`comic-stage comic-fit-${fit}${second >= 0 ? " comic-spread" : ""}`}>
        <button className="comic-zone comic-zone-prev" onClick={prev} aria-label="Previous page" tabIndex={-1} />
        <div className="comic-pages">
          <img
            key={`${issue.versionId}-${page}`}
            src={pageUrl(issue.versionId, page)}
            alt={`Page ${page + 1}`}
            onError={() => setError("This page couldn't be loaded.")}
          />
          {second >= 0 ? <img key={`${issue.versionId}-${second}`} src={pageUrl(issue.versionId, second)} alt={`Page ${second + 1}`} /> : null}
        </div>
        <button className="comic-zone comic-zone-next" onClick={next} aria-label="Next page" tabIndex={-1} />
        {atEnd && nextIssue ? (
          <button className="btn btn-primary comic-next-issue" onClick={() => go(last + 1)}>
            Next: {nextIssue.name}
          </button>
        ) : null}
      </div>

      <div className="reader-foot">
        <button className="np-btn" onClick={prev} disabled={page === 0 && idx === 0} aria-label="Previous page">
          <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
            <path d="M15.4 7.4 14 6l-6 6 6 6 1.4-1.4-4.6-4.6z" fill="currentColor" />
          </svg>
        </button>
        <input
          type="range"
          min={0}
          max={last}
          step={1}
          value={page}
          onChange={(e) => go(Number(e.target.value))}
          aria-label="Page"
          aria-valuetext={`Page ${page + 1} of ${issue.pages}`}
        />
        <span className="np-time">
          {second >= 0 ? `${page + 1}–${second + 1}` : page + 1} / {issue.pages}
        </span>
        <button className="np-btn" onClick={next} disabled={atEnd && !nextIssue} aria-label="Next page">
          <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
            <path d="M8.6 16.6 10 18l6-6-6-6-1.4 1.4 4.6 4.6z" fill="currentColor" />
          </svg>
        </button>
      </div>
    </div>
  );
}
