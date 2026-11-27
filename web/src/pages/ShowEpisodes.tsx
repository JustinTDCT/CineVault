import { useEffect, useState, type ReactNode } from "react";
import { api, type Episode, type Season, type Title } from "../api";
import { Player, type PlayTarget } from "../components/Player";
import { episodeCode, formatDate, nextEpisode, playableEpisodes, playableVersion, resolution, seasonLabel } from "../format";

/** A show's seasons as tabs, each listing every episode, with or without a file. */
export function ShowEpisodes({ title }: { title: Title }) {
  const [seasons, setSeasons] = useState<Season[] | null>(null);
  const [error, setError] = useState("");
  const [seasonNumber, setSeasonNumber] = useState<number | null>(null);
  const [playing, setPlaying] = useState<Episode | null>(null);

  useEffect(() => {
    api
      .episodes(title.id)
      .then((s) => {
        setSeasons(s);
        // Open the first season that has a file to play.
        const first = s.find((x) => x.episodes.some((e) => playableVersion(e))) ?? s[0];
        setSeasonNumber(first?.number ?? null);
      })
      .catch((e: Error) => setError(e.message));
  }, [title.id]);

  if (error) {
    return (
      <div className="notice notice-error" role="alert">
        <p>{error}</p>
      </div>
    );
  }
  if (!seasons) return <p className="page-sub">Loading episodes…</p>;
  if (seasons.length === 0) return <p className="page-sub">No episodes found. Scan the library again.</p>;

  const season = seasons.find((s) => s.number === seasonNumber) ?? seasons[0];
  const first = playableEpisodes(seasons)[0];
  const target = (e: Episode): PlayTarget => ({
    versionId: playableVersion(e)!.id,
    heading: `${title.name} · ${episodeCode(e.season, e.number, playableVersion(e)!.last_episode)}${e.name ? ` · ${e.name}` : ""}`,
  });
  const next = playing ? nextEpisode(seasons, playing.id) : undefined;

  return (
    <section className="episodes" aria-label="Episodes">
      <div className="episodes-head">
        <h2 className="section-title">{seasons.length === 1 ? seasonLabel(season) : `${seasons.length} seasons`}</h2>
        {first ? (
          <button className="btn btn-primary" onClick={() => setPlaying(first)}>
            Play {episodeCode(first.season, first.number)}
          </button>
        ) : null}
      </div>

      {seasons.length > 1 ? (
        <div className="edition-tabs" role="tablist">
          {seasons.map((s) => {
            const owned = s.episodes.filter((e) => e.versions.length > 0 || e.part_of).length;
            return (
              <button
                key={s.number}
                role="tab"
                id={`season-tab-${s.number}`}
                aria-selected={s.number === season.number}
                aria-controls="season-panel"
                className="edition-tab"
                onClick={() => setSeasonNumber(s.number)}
              >
                <span className="edition-tab-name">{seasonLabel(s)}</span>
                <span className="edition-tab-runtime">
                  {owned === s.episodes.length ? `${owned} episodes` : `${owned} of ${s.episodes.length} episodes`}
                </span>
              </button>
            );
          })}
        </div>
      ) : null}

      <div
        className="edition-panel season-panel"
        id="season-panel"
        role={seasons.length > 1 ? "tabpanel" : undefined}
        aria-labelledby={seasons.length > 1 ? `season-tab-${season.number}` : undefined}
      >
        {season.overview ? <p className="season-overview">{season.overview}</p> : null}
        <ol className="episode-list">
          {season.episodes.map((e) => (
            <EpisodeRow key={e.id} episode={e} onPlay={() => setPlaying(e)} />
          ))}
        </ol>
      </div>

      {playing ? (
        <Player
          target={target(playing)}
          onClose={() => setPlaying(null)}
          upNext={next ? `${episodeCode(next.season, next.number)}${next.name ? ` · ${next.name}` : ""}` : undefined}
          onEnded={next ? () => setPlaying(next) : undefined}
        />
      ) : null}
    </section>
  );
}

function EpisodeRow({ episode: e, onPlay }: { episode: Episode; onPlay: () => void }) {
  const v = playableVersion(e);
  const owned = e.versions.length > 0;
  const facts = [formatDate(e.air_date), e.runtime_minutes ? `${e.runtime_minutes} min` : "", v ? resolution(v) : ""].filter(Boolean);
  const label = `Play episode ${e.number}${e.name ? `, ${e.name}` : ""}`;

  let status: ReactNode = null;
  if (e.part_of) status = <span className="episode-note">In the episode {e.part_of} file</span>;
  else if (!owned) status = <span className="episode-note">Not in your library</span>;
  else if (!v) status = <span className="episode-note episode-note-alert">File missing</span>;

  return (
    <li className={owned || e.part_of ? "episode" : "episode episode-absent"}>
      <div className="episode-still">
        {e.still_url ? <img src={e.still_url} alt="" loading="lazy" decoding="async" /> : <span aria-hidden="true">{e.number}</span>}
        {v ? (
          <button className="episode-play" onClick={onPlay} aria-label={label}>
            <span aria-hidden="true" className="episode-play-icon" />
          </button>
        ) : null}
      </div>
      <div className="episode-body">
        <h3 className="episode-title">
          <span className="episode-number">{e.season === 0 ? "Special" : "Episode"} {v?.last_episode ? `${e.number}–${v.last_episode}` : e.number}</span>
          {e.name || "Untitled"}
        </h3>
        {facts.length ? <p className="episode-facts">{facts.join("   ")}</p> : null}
        {e.overview ? <p className="episode-overview">{e.overview}</p> : null}
        {status}
      </div>
    </li>
  );
}
