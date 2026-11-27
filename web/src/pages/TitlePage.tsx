import { useEffect, useState } from "react";
import { api, type Edition, type Title } from "../api";
import { Poster } from "../components/PosterCard";
import { Player, type PlayTarget } from "../components/Player";
import {
  defaultEdition,
  fileName,
  formatRuntime,
  formatSize,
  itemNoun,
  labelWithoutResolution,
  resolution,
  showCounts,
  usesTMDb,
} from "../format";
import { ShowEpisodes } from "./ShowEpisodes";
import { AlbumTracks } from "./AlbumTracks";
import { linkTo } from "../router";

export function TitlePage({
  libraryId,
  titleId,
}: {
  libraryId: number;
  titleId: number;
}) {
  const [title, setTitle] = useState<Title | null | undefined>(undefined);
  const [error, setError] = useState("");
  const [editionId, setEditionId] = useState<number | null>(null);
  const [playing, setPlaying] = useState<PlayTarget | null>(null);

  useEffect(() => {
    api
      .titles(libraryId)
      .then((all) => {
        const t = all.find((x) => x.id === titleId) ?? null;
        setTitle(t);
        if (t) setEditionId(defaultEdition(t)?.id ?? null);
      })
      .catch((e: Error) => setError(e.message));
  }, [libraryId, titleId]);

  if (error) {
    return (
      <div className="notice notice-error" role="alert">
        <p>{error}</p>
      </div>
    );
  }
  if (title === undefined) return <p className="page-sub">Loading…</p>;
  if (title === null) {
    return (
      <div className="notice">
        <p>This title isn't in the library any more.</p>
        <a className="btn" {...linkTo(`/libraries/${libraryId}`)}>
          Back to the library
        </a>
      </div>
    );
  }

  const edition =
    title.editions.find((e) => e.id === editionId) ?? title.editions[0];
  const isShow = title.media_type === "tv";
  const runtime = title.runtime_minutes
    ? title.runtime_minutes * 60
    : (edition?.runtime_seconds ?? 0);
  const isAlbum = title.media_type === "music";
  const facts = (
    isShow
      ? [title.year, showCounts(title), title.genres.join(", ")]
      : isAlbum
        ? [title.artist, title.year, title.genres.join(", ")]
        : [
            title.artist,
            title.year,
            runtime ? formatRuntime(runtime) : "",
            title.genres.join(", "),
          ]
  ).filter(Boolean);

  return (
    <article className="title-page">
      {title.backdrop_url ? (
        <div
          className="backdrop"
          style={{ backgroundImage: `url(${title.backdrop_url})` }}
          aria-hidden="true"
        />
      ) : null}

      <a className="back" {...linkTo(`/libraries/${libraryId}`)}>
        All {itemNoun(title.media_type, 2)}
      </a>

      <div className="title-head">
        <Poster title={title} size="large" />
        <div className="title-info">
          <h1 className="title-name">{title.name}</h1>
          {facts.length ? (
            <p className="title-facts">{facts.join("   ")}</p>
          ) : null}
          {title.tagline ? <p className="tagline">{title.tagline}</p> : null}
          {title.overview ? <p className="overview">{title.overview}</p> : null}
          <MatchNote title={title} />
          {title.collection_name ? (
            <p className="collection">Part of the {title.collection_name}</p>
          ) : null}
        </div>
      </div>

      {isAlbum ? (
        <AlbumTracks title={title} />
      ) : isShow ? (
        <ShowEpisodes title={title} />
      ) : (
        <EditionTabs
          title={title}
          selected={edition}
          onSelect={setEditionId}
          onPlay={setPlaying}
        />
      )}
      {playing ? (
        <Player target={playing} onClose={() => setPlaying(null)} />
      ) : null}
    </article>
  );
}

function MatchNote({ title }: { title: Title }) {
  if (title.match_status === "matched" || !usesTMDb(title)) return null;
  const text =
    title.match_status === "not_found"
      ? title.media_type === "tv"
        ? "TMDb has no show with this name, so there are no posters or episode details. Check the show's folder name."
        : "TMDb has no movie with this name and year, so there's no poster or description. Check the file name."
      : title.match_status === "error"
        ? `Matching with TMDb failed: ${title.match_error ?? "unknown error"}. It will be retried on the next scan.`
        : "Not matched with TMDb yet. It will be matched on the next scan.";
  return <p className="match-note">{text}</p>;
}

function EditionTabs({
  title,
  selected,
  onSelect,
  onPlay,
}: {
  title: Title;
  selected?: Edition;
  onSelect: (id: number) => void;
  onPlay: (t: PlayTarget) => void;
}) {
  if (!selected) return null;
  const single = title.editions.length === 1;
  const heading = single
    ? title.name
    : `${title.name} · ${selected.display_name}`;
  // Play the highest-resolution file that's present; versions arrive tallest first.
  const best = selected.versions.find((v) => !v.missing);

  return (
    <section className="editions" aria-label="Editions">
      <h2 className="section-title">
        {single
          ? "Your copy"
          : `${title.editions.length} editions in your library`}
      </h2>
      {single ? null : (
        <div className="edition-tabs" role="tablist">
          {title.editions.map((e) => (
            <button
              key={e.id}
              role="tab"
              id={`edition-tab-${e.id}`}
              aria-selected={e.id === selected.id}
              aria-controls="edition-panel"
              className="edition-tab"
              onClick={() => onSelect(e.id)}
            >
              <span className="edition-tab-name">{e.display_name}</span>
              <span className="edition-tab-runtime">
                {formatRuntime(e.runtime_seconds) || "Runtime unknown"}
              </span>
            </button>
          ))}
        </div>
      )}

      <div
        className="edition-panel"
        id="edition-panel"
        role={single ? undefined : "tabpanel"}
        aria-labelledby={single ? undefined : `edition-tab-${selected.id}`}
      >
        <div className="edition-actions">
          <button
            className="btn btn-primary"
            disabled={!best}
            onClick={() => best && onPlay({ versionId: best.id, heading })}
          >
            Play {single ? "" : selected.display_name}
          </button>
          {best ? null : (
            <span className="hint">
              The files for this edition are missing.
            </span>
          )}
        </div>
        <ul className="versions">
          {selected.versions.map((v) => (
            <li
              key={v.id}
              className={v.missing ? "version version-missing" : "version"}
            >
              <div className="version-main">
                <span className="version-res">
                  {resolution(v) || "Unknown"}
                </span>
                <span className="version-label">
                  {labelWithoutResolution(v.label, resolution(v))}
                </span>
              </div>
              <div className="version-file">
                <span className="version-path" title={v.path}>
                  {fileName(v.path)}
                </span>
                <span className="version-size">{formatSize(v.size_bytes)}</span>
                {v.missing ? (
                  <span className="version-flag">File missing</span>
                ) : selected.versions.length > 1 ? (
                  <button
                    className="linkish"
                    onClick={() =>
                      onPlay({
                        versionId: v.id,
                        heading: `${heading} (${resolution(v)})`,
                      })
                    }
                  >
                    Play this file
                  </button>
                ) : null}
              </div>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}
