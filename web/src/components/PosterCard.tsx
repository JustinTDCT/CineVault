import { useState } from "react";
import type { Title } from "../api";
import { coverUrl, editionListing, posterBadge, showCounts, thumbUrl } from "../format";
import { linkTo } from "../router";

export function Poster({ title, size = "card" }: { title: Title; size?: "card" | "large" }) {
  const [failed, setFailed] = useState(false);
  // Music videos have no posters: a still from the video, in 16:9.
  const wide = title.media_type === "musicvideo";
  const square = title.media_type === "music" || title.media_type === "audiobook";
  const shape = wide ? " poster-wide" : square ? " poster-square" : "";
  const src = wide ? thumbUrl(title) : square ? coverUrl(title) : title.poster_url;
  if (src && !failed) {
    return (
      <img
        className={`poster poster-${size}${shape}`}
        src={src}
        alt=""
        loading="lazy"
        decoding="async"
        onError={() => setFailed(true)}
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

export function PosterCard({ title }: { title: Title }) {
  const [expanded, setExpanded] = useState(false);
  const href = `/libraries/${title.library_id}/titles/${title.id}`;
  const badge = posterBadge(title);
  const multi = title.editions.length > 1;
  const { shown, hidden } = editionListing(title.editions, expanded);

  return (
    <article className="card">
      <a className="card-poster" {...linkTo(href)} aria-label={`${title.name}${title.year ? ` (${title.year})` : ""}`}>
        <Poster title={title} />
        {badge ? <span className="badge">{badge}</span> : null}
      </a>
      <h3 className="card-title">
        <a {...linkTo(href)}>{title.name}</a>
      </h3>
      {title.media_type === "tv" ? (
        <p className="card-year">{[title.year, showCounts(title)].filter(Boolean).join("   ")}</p>
      ) : title.media_type === "audiobook" && title.series ? (
        <p className="card-year">{title.series_index ? `Book ${title.series_index}` : title.year}</p>
      ) : title.year ? (
        <p className="card-year">{title.year}</p>
      ) : null}
      {multi ? (
        <p className="card-editions" aria-label="Editions you have">
          {shown.map((e, i) => (
            <span key={e.id}>
              {i > 0 ? <span className="sep" aria-hidden="true"> · </span> : null}
              <span title={e.display_name}>{e.short_label}</span>
            </span>
          ))}
          {hidden > 0 || expanded ? (
            <button className="linkish" onClick={() => setExpanded(!expanded)} aria-expanded={expanded}>
              {expanded ? "Show fewer" : `Show all (${title.editions.length})`}
            </button>
          ) : null}
        </p>
      ) : null}
    </article>
  );
}
