import { useEffect, useState } from "react";
import { api, type Progress, type Title, type Version } from "../api";
import { BookReader, type ReadTarget } from "../components/BookReader";
import { fileName, formatSize } from "../format";

/** Formats the browser reader opens; others (PDF) open in the browser's own viewer. */
const READABLE = new Set(["epub", "azw3", "mobi", "fb2"]);

/** An ebook's files with Read, Continue and Download. */
export function EbookFormats({ title }: { title: Title }) {
  const [saved, setSaved] = useState<Progress | null>(null);
  const [reading, setReading] = useState<ReadTarget | null>(null);

  useEffect(() => {
    api
      .progress(title.id)
      .then(setSaved)
      .catch(() => setSaved(null));
  }, [title.id, reading]);

  const files = title.editions.flatMap((e) => e.versions).filter((v) => !v.missing);
  // EPUB first: it reads best in the browser.
  const order = ["epub", "azw3", "mobi", "fb2", "pdf"];
  files.sort((a, b) => order.indexOf(a.container ?? "") - order.indexOf(b.container ?? ""));
  if (files.length === 0) return <p className="cv-sub">The files for this book are missing. Scan the library again.</p>;

  const open = (v: Version) => {
    if (!READABLE.has(v.container ?? "")) {
      window.open(`/api/versions/${v.id}/file`, "_blank", "noopener");
      return;
    }
    setReading({
      titleId: title.id,
      versionId: v.id,
      heading: title.name,
      fileName: fileName(v.path),
      locator: saved?.version_id === v.id ? saved.locator : undefined,
    });
  };

  const savedFile = files.find((v) => v.id === saved?.version_id);
  const pct = saved && savedFile ? Math.round(saved.position_seconds * 100) : 0;
  const main = savedFile && pct > 0 && pct < 100 ? savedFile : (files.find((v) => READABLE.has(v.container ?? "")) ?? files[0]);

  return (
    <section className="album" aria-label="Read">
      <div className="book-actions">
        <button className="cv-btn cv-btn--primary" onClick={() => open(main)}>
          {main === savedFile && pct > 0 && pct < 100 ? `Continue reading · ${pct}%` : "Read"}
        </button>
        {main === savedFile && pct > 0 && pct < 100 ? <span className="cv-hint">in the {(main.label ?? "").toLowerCase() || "book"} copy</span> : null}
      </div>

      <h2 className="cv-section-title">{files.length === 1 ? "Your copy" : `${files.length} formats`}</h2>
      <ul className="versions">
        {files.map((v) => (
          <li key={v.id} className="version">
            <div className="version-main">
              <span className="version-res">{v.label || (v.container ?? "").toUpperCase()}</span>
              <span className="version-label">{READABLE.has(v.container ?? "") ? "Reads in the browser" : "Opens in the browser's viewer"}</span>
            </div>
            <div className="version-file">
              <span className="version-path" title={v.path}>
                {fileName(v.path)}
              </span>
              <span className="version-size">{formatSize(v.size_bytes)}</span>
              <button className="cv-linkish" onClick={() => open(v)}>
                {READABLE.has(v.container ?? "") ? "Read" : "Open"}
              </button>
              <a className="cv-linkish" href={`/api/versions/${v.id}/file`} download={fileName(v.path)}>
                Download
              </a>
            </div>
          </li>
        ))}
      </ul>

      {reading ? <BookReader target={reading} onClose={() => setReading(null)} /> : null}
    </section>
  );
}
