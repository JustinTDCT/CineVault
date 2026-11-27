import { useEffect, useMemo, useState } from "react";
import { api, type Library, type Title } from "../api";
import { PosterCard } from "../components/PosterCard";
import { Icon } from "../components/Icon";
import { linkTo } from "../router";
import { isUnowned, itemNoun, ownedEditions } from "../format";
import { continueTitles, progressCaption, progressFraction, recentlyAdded, spotlightTitles } from "../home";

/** How long each spotlight title shows before the next. */
const SPOTLIGHT_MS = 9000;

/** How many posters a library's row on Home shows. */
const ROW_LENGTH = 7;

/** Home: a spotlight on one title, a row per library, and every library as a tile. */
export function HomePage({ libraries }: { libraries: Library[] | null }) {
  const [titles, setTitles] = useState<Record<number, Title[]>>({});
  const [error, setError] = useState("");

  useEffect(() => {
    if (!libraries) return;
    let stale = false;
    Promise.all(libraries.map((l) => api.titles(l.id).then((ts) => [l.id, ts] as const)))
      .then((pairs) => {
        if (!stale) setTitles(Object.fromEntries(pairs));
      })
      .catch((e: Error) => !stale && setError(e.message));
    return () => {
      stale = true;
    };
  }, [libraries]);

  const all = useMemo(() => Object.values(titles).flat(), [titles]);
  const spots = useMemo(() => spotlightTitles(all), [all]);
  const continuing = useMemo(() => continueTitles(all), [all]);
  const recent = useMemo(() => recentlyAdded(all), [all]);

  if (!libraries) return <p className="cv-page cv-sub">Loading…</p>;
  const ready = Object.keys(titles).length > 0;

  return (
    <div className="home">
      {error ? (
        <div className="content-error">
          <div className="cv-notice cv-notice--error" role="alert">
            <p>{error}</p>
          </div>
        </div>
      ) : null}
      {spots.length ? <Spotlight titles={spots} /> : null}

      {continuing.length ? (
        <section className="cv-section" aria-label="Continue">
          <div className="cv-section-head">
            <h2 className="cv-section-title">Continue</h2>
          </div>
          <div className="cv-row cv-row--posters">
            {continuing.map((t) => (
              <PosterCard
                key={t.id}
                title={t}
                progress={progressFraction(t)}
                caption={progressCaption(t)}
                href={`/libraries/${t.library_id}/titles/${t.id}${t.media_type === "movie" || t.media_type === "tv" ? "?play=1" : ""}`}
              />
            ))}
          </div>
        </section>
      ) : null}

      {ready && recent.length > 0 ? (
        <section className="cv-section" aria-label="Recently added">
          <div className="cv-section-head">
            <h2 className="cv-section-title">Recently added</h2>
          </div>
          <div className="cv-row cv-row--posters">
            {recent.map((t) => (
              <PosterCard key={t.id} title={t} caption={libraryName(libraries, t.library_id)} />
            ))}
          </div>
        </section>
      ) : null}

      {libraries.map((l) => {
        const ts = titles[l.id];
        if (!ts?.length) return null;
        return (
          <section key={l.id} className="cv-section" aria-label={l.name}>
            <div className="cv-section-head">
              <h2 className="cv-section-title">{l.name}</h2>
              <a className="cv-link" {...linkTo(`/libraries/${l.id}`)}>
                See all {ts.length}
              </a>
            </div>
            <div className={l.media_type === "musicvideo" ? "cv-row cv-row--stills" : "cv-row cv-row--posters"}>
              {ts.slice(0, l.media_type === "musicvideo" ? 4 : ROW_LENGTH).map((t) => (
                <PosterCard key={t.id} title={t} />
              ))}
            </div>
          </section>
        );
      })}

      <section className="cv-section" aria-label="Your libraries">
        <div className="cv-section-head">
          <h2 className="cv-section-title">Your libraries</h2>
          <a className="cv-link" {...linkTo("/setup")}>
            Add a library
          </a>
        </div>
        <div className="cv-row" style={{ ["--cols" as string]: Math.min(4, Math.max(1, libraries.length)) }}>
          {libraries.map((l) => {
            const ts = titles[l.id];
            const extra = ts ? ts.filter((t) => t.editions.length > 1).length : 0;
            return (
              <a key={l.id} className="cv-tile" {...linkTo(`/libraries/${l.id}`)}>
                <div className="library-tile-name">
                  <Icon name={l.media_type} /> {l.name}
                </div>
                <div className="cv-caption">
                  {ts ? `${ts.length} ${itemNoun(l.media_type, ts.length)}` : "…"}
                  {extra && (l.media_type === "movie" || l.media_type === "tv") ? ` · ${extra} with more than one edition` : ""}
                </div>
              </a>
            );
          })}
        </div>
      </section>
    </div>
  );
}

function libraryName(libraries: Library[], id: number): string {
  return libraries.find((l) => l.id === id)?.name ?? "";
}

function spotlightLine(t: Title): string {
  const owned = ownedEditions(t);
  const missing = t.editions.filter(isUnowned);
  if (missing.length) {
    return `${owned.length} ${owned.length === 1 ? "edition" : "editions"} in your library · ${missing.map((e) => e.cache?.display_name || e.display_name).join(", ")} also known`;
  }
  if (owned.length > 1) return `${owned.length} editions in your library: ${owned.map((e) => e.display_name).join(", ")}`;
  return [t.year, t.genres.slice(0, 2).join(", ")].filter(Boolean).join(" · ");
}

/** The spotlight: a few titles in turn, with dots to pick one. It pauses while the
 *  pointer or keyboard focus is on it, and doesn't move for people who ask for less motion. */
function Spotlight({ titles }: { titles: Title[] }) {
  const [index, setIndex] = useState(0);
  const [paused, setPaused] = useState(false);
  const still = typeof matchMedia === "function" && matchMedia("(prefers-reduced-motion: reduce)").matches;

  useEffect(() => {
    if (paused || still || titles.length < 2) return;
    const id = window.setInterval(() => setIndex((i) => (i + 1) % titles.length), SPOTLIGHT_MS);
    return () => window.clearInterval(id);
  }, [paused, still, titles.length]);

  const title = titles[index % titles.length];
  const href = `/libraries/${title.library_id}/titles/${title.id}`;
  const eyebrow = title.editions.some(isUnowned)
    ? "Another cut known"
    : ownedEditions(title).length > 1
      ? "Several cuts in your library"
      : title.collection_name || "From your library";
  return (
    <div className="home-spotlight" onMouseEnter={() => setPaused(true)} onMouseLeave={() => setPaused(false)} onFocus={() => setPaused(true)} onBlur={() => setPaused(false)}>
      <section className="cv-spotlight" style={{ backgroundImage: `url(${title.backdrop_url})` }} aria-roledescription="carousel" aria-label="Spotlight">
        <div className="cv-spotlight__content" key={title.id}>
          <div className="cv-eyebrow">{eyebrow}</div>
          <h1 className="cv-hero-title">{title.name}</h1>
          <p className="spotlight-line">{spotlightLine(title)}</p>
          <div className="cv-actions">
            <a className="cv-btn cv-btn--primary" {...linkTo(`${href}?play=1`)}>
              <Icon name="play" filled />
              Play
            </a>
            <a className="cv-btn cv-btn--on-art" {...linkTo(href)}>
              More info
            </a>
          </div>
        </div>
        {titles.length > 1 ? (
          <div className="cv-spotlight__dots">
            {titles.map((t, i) => (
              <button
                key={t.id}
                className={i === index ? "is-current" : undefined}
                aria-label={`Show ${t.name}`}
                aria-current={i === index ? "true" : undefined}
                onClick={() => setIndex(i)}
              />
            ))}
          </div>
        ) : null}
      </section>
    </div>
  );
}
