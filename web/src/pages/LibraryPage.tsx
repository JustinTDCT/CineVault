import { useCallback, useEffect, useState } from "react";
import { api, type Library, type ScanResult, type Title } from "../api";
import { PosterCard } from "../components/PosterCard";

function scanSummary(r: ScanResult): string {
  const parts: string[] = [];
  if (r.added) parts.push(`${r.added} new`);
  if (r.updated) parts.push(`${r.updated} changed`);
  if (r.missing) parts.push(`${r.missing} missing`);
  const files = `${r.files_seen} ${r.files_seen === 1 ? "file" : "files"}`;
  let text = parts.length ? `Scanned ${files}: ${parts.join(", ")}.` : `Scanned ${files}. Nothing changed.`;
  if (r.match && r.match.matched) text += ` Matched ${r.match.matched} to TMDb.`;
  return text;
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

  const problems = scanResult ? [...scanResult.errors.map((e) => `${e.path}: ${e.error}`), ...(scanResult.match?.error ? [`TMDb: ${scanResult.match.error}`] : [])] : [];

  return (
    <section>
      <div className="page-head">
        <div>
          <h1 className="page-title">{library?.name ?? "Library"}</h1>
          {titles ? (
            <p className="page-sub">
              {titles.length} {titles.length === 1 ? "title" : "titles"}
              {library ? <> in {library.paths.join(", ")}</> : null}
            </p>
          ) : null}
        </div>
        <button className="btn btn-primary" onClick={scan} disabled={scanning}>
          {scanning ? "Scanning…" : "Scan library"}
        </button>
      </div>

      {scanResult ? (
        <div className={`notice ${problems.length ? "notice-warn" : ""}`} role="status">
          <p>{scanSummary(scanResult)}</p>
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
        <div className="notice notice-error" role="alert">
          <p>{error}</p>
        </div>
      ) : titles === null ? (
        <p className="page-sub">Loading…</p>
      ) : titles.length === 0 ? (
        <div className="empty">
          <p>No movies here yet. Scan the library to find the files in its folders.</p>
        </div>
      ) : (
        <div className="grid">
          {titles.map((t) => (
            <PosterCard key={t.id} title={t} />
          ))}
        </div>
      )}
    </section>
  );
}
