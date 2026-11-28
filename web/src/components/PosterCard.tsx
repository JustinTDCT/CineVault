import { useState } from "react";
import type { Title } from "../api";
import { coverUrl, editionListing, ownedEditions, posterBadge, showCounts, thumbUrl } from "../format";
import { linkTo } from "../router";

export function Poster({ title, size = "card" }: { title: Title; size?: "card" | "large" }) {
  // Each candidate is tried in turn; one that fails to load is skipped.
  const [failed, setFailed] = useState(0);
  // Music videos have no posters: a still from the video, in 16:9.
  const wide = title.media_type === "musicvideo";
  const square = title.media_type === "music" || title.media_type === "audiobook";
  const shape = wide ? " poster-wide" : square ? " poster-square" : "";
  const src = artCandidates(title)[failed];
  if (src) {
    return (
      <img
        key={src}
        className={`poster poster-${size}${shape}`}
        src={src}
        alt=""
        loading="lazy"
        decoding="async"
        referrerPolicy="no-referrer"
        onError={() => setFailed((n) => n + 1)}
      />
    );
  }
  // No artwork yet: a title card in marquee lettering.
  return (
    <div className={`poster poster-${size} poster-blank${shape}`} aria-hidden="true">
      <span>{title.name}</span>
      {title.year ? <span className="poster-blank-year">{title.year}</span> : null}
    </div>
  );
}

/** Art to show, best first: a still for music videos; for albums, books,
 *  audiobooks and comics the looked-up cover (the server leaves it out when
 *  the library prefers its files), then the file's own; a movie's poster. */
export function artCandidates(title: Title): string[] {
  if (title.media_type === "musicvideo") return [thumbUrl(title)].filter(Boolean) as string[];
  const fileCover = ["music", "audiobook", "book", "comic"].includes(title.media_type) ? coverUrl(title) : undefined;
  return [title.poster_url, fileCover].filter(Boolean) as string[];
}

export function PosterCard({
  title,
  progress,
  caption,
  href: hrefOverride,
}: {
  title: Title;
  /** How far along, 0 to 1: drawn as a bar on the art ("Continue"). */
  progress?: number;
  /** Replaces the year line, e.g. "S1 E3 · 45 min left". */
  caption?: string;
  /** Where the card goes; the title page by default. */
  href?: string;
}) {
  const [expanded, setExpanded] = useState(false);
  const href = hrefOverride ?? `/libraries/${title.library_id}/titles/${title.id}`;
  const badge = posterBadge(title);
  const owned = ownedEditions(title);
  const multi = owned.length > 1;
  const { shown, hidden } = editionListing(owned, expanded);
  const shape =
    title.media_type === "musicvideo"
      ? " cv-card__art--still"
      : title.media_type === "music" || title.media_type === "audiobook"
        ? " cv-card__art--square"
        : "";
  const meta =
    title.media_type === "tv"
      ? [title.year, showCounts(title)].filter(Boolean).join(" · ")
      : title.media_type === "comic"
        ? [title.year, title.track_count ? `${title.track_count} ${title.track_count === 1 ? "issue" : "issues"}` : ""].filter(Boolean).join(" · ")
        : (title.media_type === "audiobook" || title.media_type === "book") && title.series
          ? title.series_index
            ? `Book ${title.series_index}`
            : String(title.year ?? "")
          : String(title.year ?? "");

  return (
    <article className="cv-card">
      <a className={`cv-card__art${shape}`} {...linkTo(href)} aria-label={`${title.name}${title.year ? ` (${title.year})` : ""}`}>
        <Poster title={title} />
        {badge ? <span className="badge">{badge}</span> : null}
        {progress !== undefined ? (
          <span className="cv-progress" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(progress * 100)} aria-label="Progress">
            <span style={{ width: `${Math.round(progress * 100)}%` }} />
          </span>
        ) : null}
      </a>
      <h3 className="cv-card__title">
        <a {...linkTo(href)}>
          {title.name}
        </a>
      </h3>
      {caption ? <p className="cv-card__meta"><em>{caption}</em></p> : meta ? <p className="cv-card__meta">{meta}</p> : null}
      {multi && !caption ? (
        <p className="card-editions" aria-label="Editions you have">
          {shown.map((e, i) => (
            <span key={e.id}>
              {i > 0 ? (
                <span className="sep" aria-hidden="true">
                  {" "}
                  ·{" "}
                </span>
              ) : null}
              <span title={e.display_name}>{e.short_label}</span>
            </span>
          ))}
          {hidden > 0 || expanded ? (
            <button className="cv-linkish" onClick={() => setExpanded(!expanded)} aria-expanded={expanded}>
              {expanded ? "Show fewer" : `Show all (${owned.length})`}
            </button>
          ) : null}
        </p>
      ) : null}
    </article>
  );
}
