import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "../api";

export interface ReadTarget {
  titleId: number;
  versionId: number;
  heading: string;
  /** The file name, so the reader can tell EPUB from AZW3. */
  fileName: string;
  /** Where to open: a saved EPUB CFI. */
  locator?: string;
}

type Theme = "dark" | "sepia" | "light";

const THEMES: Record<Theme, { bg: string; fg: string; link: string; label: string }> = {
  dark: { bg: "#15181d", fg: "#ddd8cf", link: "#f2b84b", label: "Dark" },
  sepia: { bg: "#f3ead7", fg: "#4a3b2a", link: "#8a4b14", label: "Sepia" },
  light: { bg: "#fbfaf7", fg: "#1c1c1c", link: "#8a4b14", label: "Light" },
};

const SIZES = [80, 90, 100, 112, 125, 140, 160];

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
    /* private mode: settings just aren't remembered */
  }
}

const bookCSS = (t: Theme, size: number) => `
  html { color: ${THEMES[t].fg} !important; background: ${THEMES[t].bg} !important; font-size: ${size}% !important; }
  body { color: inherit !important; background: transparent !important; }
  a:link, a:visited { color: ${THEMES[t].link} !important; }
  p, li, blockquote, dd { line-height: 1.55; hyphens: auto; -webkit-hyphens: auto; widows: 2; orphans: 2; }
  img, svg { max-width: 100%; height: auto; }
  pre { white-space: pre-wrap !important; }
`;

/**
 * Reads EPUB, AZW3, MOBI and FB2 files in the browser with foliate-js,
 * remembering the place in the book on the server.
 */
export function BookReader({ target, onClose }: { target: ReadTarget; onClose: () => void }) {
  const hostRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<FoliateView | null>(null);
  const saveTimer = useRef<number | undefined>(undefined);
  const closeRef = useRef(onClose);
  closeRef.current = onClose;
  const [status, setStatus] = useState("Opening the book…");
  const [where, setWhere] = useState<{ fraction: number; chapter: string }>({ fraction: 0, chapter: "" });
  const [toc, setToc] = useState<FoliateTocItem[]>([]);
  const [panel, setPanel] = useState<"" | "toc" | "text">("");
  const [theme, setTheme] = useState<Theme>(() => load("reader.theme", "dark"));
  const [size, setSize] = useState<number>(() => load("reader.size", 100));

  const save = useCallback(
    (loc: FoliateLocation) => {
      window.clearTimeout(saveTimer.current);
      saveTimer.current = window.setTimeout(() => api.saveReading(target.titleId, target.versionId, loc.fraction, loc.cfi), 1500);
    },
    [target.titleId, target.versionId],
  );

  // Open the book.
  useEffect(() => {
    let cancelled = false;
    const host = hostRef.current;
    if (!host) return;
    let view: FoliateView | null = null;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "ArrowLeft") view?.goLeft();
      else if (e.key === "ArrowRight") view?.goRight();
      else if (e.key === "Escape") closeRef.current();
    };
    (async () => {
      try {
        await import("foliate-js/view.js");
        const res = await fetch(`/api/versions/${target.versionId}/file`);
        if (!res.ok) throw new Error("The book file couldn't be loaded. It may be missing from disk.");
        const file = new File([await res.blob()], target.fileName);
        if (cancelled) return;
        view = document.createElement("foliate-view") as FoliateView;
        view.className = "reader-view";
        host.append(view);
        viewRef.current = view;
        view.addEventListener("relocate", (e) => {
          const loc = (e as CustomEvent<FoliateLocation>).detail;
          setWhere({ fraction: loc.fraction ?? 0, chapter: loc.tocItem?.label?.trim() ?? "" });
          save(loc);
        });
        // Arrow keys work while the book (an iframe) has focus too.
        view.addEventListener("load", (e) => {
          (e as CustomEvent<{ doc: Document }>).detail.doc.addEventListener("keydown", onKey);
        });
        await view.open(file);
        view.renderer.setAttribute("max-inline-size", "42em");
        view.renderer.setAttribute("gap", "6%");
        view.renderer.setStyles?.(bookCSS(load("reader.theme", "dark"), load("reader.size", 100)));
        setToc(view.book.toc ?? []);
        await view.init(target.locator ? { lastLocation: target.locator } : { showTextStart: false });
        if (!cancelled) setStatus("");
      } catch (e) {
        if (!cancelled) setStatus((e as Error).message || "This book couldn't be opened.");
      }
    })();
    document.addEventListener("keydown", onKey);
    return () => {
      cancelled = true;
      document.removeEventListener("keydown", onKey);
      const loc = view?.lastLocation;
      window.clearTimeout(saveTimer.current);
      if (loc) api.saveReading(target.titleId, target.versionId, loc.fraction, loc.cfi);
      view?.close();
      view?.remove();
      viewRef.current = null;
    };
  }, [target, save]);

  // Theme and text size.
  useEffect(() => {
    viewRef.current?.renderer?.setStyles?.(bookCSS(theme, size));
    store("reader.theme", theme);
    store("reader.size", size);
  }, [theme, size]);

  const go = (href: string) => {
    viewRef.current?.goTo(href).catch(() => {});
    setPanel("");
  };

  return (
    <div className={`reader reader-${theme}`} role="dialog" aria-modal="true" aria-label={`Reading ${target.heading}`}>
      <div className="reader-bar">
        <div className="reader-heading">
          <h2 className="player-title">{target.heading}</h2>
          <p className="player-status">{status || where.chapter || " "}</p>
        </div>
        <div className="reader-tools">
          <button className="btn" aria-expanded={panel === "toc"} onClick={() => setPanel(panel === "toc" ? "" : "toc")} disabled={toc.length === 0}>
            Contents
          </button>
          <button
            className="btn"
            aria-expanded={panel === "text"}
            onClick={() => setPanel(panel === "text" ? "" : "text")}
            aria-label="Text size and colours"
          >
            Aa
          </button>
          <button className="btn" onClick={onClose}>
            Close
          </button>
        </div>

        {panel === "toc" ? (
          <nav className="reader-panel reader-toc" aria-label="Contents">
            <TocList items={toc} onPick={go} current={where.chapter} />
          </nav>
        ) : panel === "text" ? (
          <div className="reader-panel reader-text">
            <div className="reader-row" role="group" aria-label="Text size">
              <button
                className="btn"
                onClick={() => setSize(SIZES[Math.max(0, SIZES.indexOf(size) - 1)] ?? 100)}
                disabled={size <= SIZES[0]}
                aria-label="Smaller text"
              >
                A−
              </button>
              <span className="reader-size">{size}%</span>
              <button
                className="btn"
                onClick={() => setSize(SIZES[Math.min(SIZES.length - 1, SIZES.indexOf(size) + 1)] ?? 100)}
                disabled={size >= SIZES[SIZES.length - 1]}
                aria-label="Larger text"
              >
                A+
              </button>
            </div>
            <div className="reader-row" role="radiogroup" aria-label="Page colour">
              {(Object.keys(THEMES) as Theme[]).map((t) => (
                <button
                  key={t}
                  role="radio"
                  aria-checked={theme === t}
                  className="reader-swatch"
                  style={{ background: THEMES[t].bg, color: THEMES[t].fg }}
                  onClick={() => setTheme(t)}
                >
                  {THEMES[t].label}
                </button>
              ))}
            </div>
          </div>
        ) : null}
      </div>

      <div className="reader-page" ref={hostRef} style={{ background: THEMES[theme].bg }} />

      <div className="reader-foot">
        <button className="np-btn" onClick={() => viewRef.current?.goLeft()} aria-label="Previous page">
          <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
            <path d="M15.4 7.4 14 6l-6 6 6 6 1.4-1.4-4.6-4.6z" fill="currentColor" />
          </svg>
        </button>
        <input
          type="range"
          min={0}
          max={1}
          step={0.001}
          value={where.fraction}
          onChange={(e) => viewRef.current?.goToFraction(Number(e.target.value))}
          aria-label="Position in book"
          aria-valuetext={`${Math.round(where.fraction * 100)}%`}
        />
        <span className="np-time">{Math.round(where.fraction * 100)}%</span>
        <button className="np-btn" onClick={() => viewRef.current?.goRight()} aria-label="Next page">
          <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true">
            <path d="M8.6 16.6 10 18l6-6-6-6-1.4 1.4 4.6 4.6z" fill="currentColor" />
          </svg>
        </button>
      </div>
    </div>
  );
}

function TocList({ items, onPick, current }: { items: FoliateTocItem[]; onPick: (href: string) => void; current: string }) {
  return (
    <ul>
      {items.map((it, i) => (
        <li key={`${it.href}-${i}`}>
          <button className="linkish" onClick={() => onPick(it.href)} aria-current={it.label.trim() === current ? "true" : undefined}>
            {it.label.trim()}
          </button>
          {it.subitems?.length ? <TocList items={it.subitems} onPick={onPick} current={current} /> : null}
        </li>
      ))}
    </ul>
  );
}
