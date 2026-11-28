import { useCallback, useEffect, useState } from "react";
import { api, type Library, type ScanResult, type Title, type MatchResult } from "../api";
import { PosterCard } from "../components/PosterCard";
import { groupByArtist, itemNoun, matchSource } from "../format";

export function scanSummary(r: ScanResult, mediaType?: string): string {
  const parts: string[] = [];
  if (r.added) parts.push(`${r.added} new`);
  if (r.updated) parts.push(`${r.updated} changed`);
  if (r.missing) parts.push(`${r.missing} missing`);
  const files = `${r.files_seen} ${r.files_seen === 1 ? "file" : "files"}`;
  let text = parts.length ? `Scanned ${files}: ${parts.join(", ")}.` : `Scanned ${files}. Nothing changed.`;
  if (r.match && r.match.matched) text += ` Matched ${r.match.matched} to ${matchSource(mediaType ?? "movie") || "TMDb"}.`;
  if (r.match?.episodes) text += ` Fetched details for ${r.match.episodes} episodes.`;
  return text;
}

/** What a music video re-match did, for Settings → Libraries. */
export function rematchSummary(r: MatchResult, mediaType = "musicvideo"): string {
  if (r.error) return `Matching stopped: ${r.error}`;
  const parts = [`Matched ${r.matched} to ${matchSource(mediaType)}`];
  if (r.not_found) parts.push(`${r.not_found} not found`);
  if (r.errors) parts.push(`${r.errors} failed (try again in a minute)`);
  return parts.join(", ") + ".";
}

export function LibraryPage({ library, libraryId }: { library?: Library; libraryId: number }) {
  const [titles, setTitles] = useState<Title[] | null>(null);
  const [error, setError] = useState("");
  const [scanning, setScanning] = useState(false);
  const [scanResult, setScanResult] = useState<ScanResult | null>(null);

  const load = useCallback(async () => {
    try {
      setTitles(await api.titles(libraryId));
      setError("");
    } catch (e) {
      setError((e as Error).message);
    }
  }, [libraryId]);

  const scan = useCallback(async () => {
    setScanning(true);
    setScanResult(null);
    try {
      setScanResult(await api.scan(libraryId));
      await load();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setScanning(false);
    }
  }, [libraryId, load]);

  useEffect(() => {
    load();
    // A library that was just added scans straight away.
    if (new URLSearchParams(location.search).get("scan") === "1") {
      history.replaceState(null, "", location.pathname);
      scan();
    }
  }, [load, scan]);

  const problems = scanResult
    ? [...scanResult.errors.map((e) => `${e.path}: ${e.error}`), ...(scanResult.match?.error ? [`${matchSource(library?.media_type ?? "movie")}: ${scanResult.match.error}`] : [])]
    : [];

  return (
    <section>
      <div className="cv-page-head">
        <div>
          <h1 className="cv-page-title">{library?.name ?? "Library"}</h1>
          {titles ? (
            <p className="cv-sub">
              {titles.length} {itemNoun(library?.media_type, titles.length)}
              {library ? <> in {library.paths.join(", ")}</> : null}
            </p>
          ) : null}
        </div>
        <button className="cv-btn cv-btn--primary" onClick={scan} disabled={scanning}>
          {scanning ? "Scanning…" : "Scan library"}
        </button>
      </div>

      {scanResult ? (
        <div className={`cv-notice ${problems.length ? "cv-notice--warn" : ""}`} role="status">
          <p>{scanSummary(scanResult, library?.media_type)}</p>
          {problems.length ? (
            <ul className="problems">
              {problems.map((p) => (
                <li key={p}>{p}</li>
              ))}
            </ul>
          ) : null}
        </div>
      ) : null}

      {error ? (
        <div className="cv-notice cv-notice--error" role="alert">
          <p>{error}</p>
        </div>
      ) : titles === null ? (
        <p className="cv-sub">Loading…</p>
      ) : titles.length === 0 ? (
        <div className="empty">
          <p>No {itemNoun(library?.media_type, 2)} here yet. Scan the library to find the files in its folders.</p>
        </div>
      ) : library?.media_type === "musicvideo" ||
        library?.media_type === "music" ||
        library?.media_type === "audiobook" ||
        library?.media_type === "book" ? (
        groupByArtist(titles).map((g) => (
          <section key={g.artist} className="artist-group" aria-label={g.artist}>
            <h2 className="artist-name cv-section-title">{g.artist}</h2>
            <div className={library.media_type === "musicvideo" ? "cv-grid cv-grid--wide" : "cv-grid"}>
              {g.titles.map((t) => (
                <PosterCard key={t.id} title={t} />
              ))}
            </div>
          </section>
        ))
      ) : (
        <div className="cv-grid">
          {titles.map((t) => (
            <PosterCard key={t.id} title={t} />
          ))}
        </div>
      )}
    </section>
  );
}
