import { useState } from "react";
import type { Title } from "../api";
import { coverUrl, editionListing, ownedEditions, posterBadge, showCounts, thumbUrl } from "../format";
import { linkTo } from "../router";

export function Poster({ title, size = "card" }: { title: Title; size?: "card" | "large" }) {
  const [failed, setFailed] = useState(false);
  // Music videos have no posters: a still from the video, in 16:9.
  const wide = title.media_type === "musicvideo";
  const square = title.media_type === "music" || title.media_type === "audiobook";
  const shape = wide ? " poster-wide" : square ? " poster-square" : "";
  const src = wide ? thumbUrl(title) : square || title.media_type === "book" || title.media_type === "comic" ? coverUrl(title) : title.poster_url;
  if (src && !failed) {
    return <img className={`poster poster-${size}${shape}`} src={src} alt="" loading="lazy" decoding="async" onError={() => setFailed(true)} />;
  }
  // No artwork yet: a title card in marquee lettering.
  return (
    <div className={`poster poster-${size} poster-blank${shape}`} aria-hidden="true">
      <span>{title.name}</span>
      {title.year ? <span className="poster-blank-year">{title.year}</span> : null}
    </div>
  );
}

export function PosterCard({ title }: { title: Title }) {
  const [expanded, setExpanded] = useState(false);
  const href = `/libraries/${title.library_id}/titles/${title.id}`;
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
      </a>
      <h3 className="cv-card__title">
        <a {...linkTo(href)}>
          {title.name}
        </a>
      </h3>
      {meta ? <p className="cv-card__meta">{meta}</p> : null}
      {multi ? (
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
