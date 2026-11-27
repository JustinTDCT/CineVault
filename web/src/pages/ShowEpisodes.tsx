import { useEffect, useState, type ReactNode } from "react";
import { api, type Episode, type Progress, type Season, type Title } from "../api";
import { Player, type PlayTarget } from "../components/Player";
import {
  episodeCode,
  formatDate,
  isPlaceholderName,
  isUpcoming,
  nextEpisode,
  playableEpisodes,
  playableVersion,
  resolution,
  seasonLabel,
  timeLeft,
} from "../format";

/** Where to pick a show up: the episode last played, from where it stopped, or the
 *  next one when that episode was finished. Undefined when nothing was played. */
export function resumePoint(seasons: Season[], p: Progress | undefined): { episode: Episode; startAt: number; left: string } | undefined {
  if (!p) return undefined;
  for (const s of seasons) {
    for (const e of s.episodes) {
      const v = e.versions.find((x) => x.id === p.version_id);
      if (!v) continue;
      const length = v.duration_seconds ?? 0;
      if (length && p.position_seconds / length >= 0.95) {
        const next = nextEpisode(seasons, e.id);
        return next ? { episode: next, startAt: 0, left: "" } : undefined;
      }
      return { episode: e, startAt: p.position_seconds, left: length ? timeLeft(length, p.position_seconds) : "" };
    }
  }
  return undefined;
}

/** A show's seasons as tabs, each listing every episode, with or without a file. */
export function ShowEpisodes({ title, autoplay = false }: { title: Title; autoplay?: boolean }) {
  const [seasons, setSeasons] = useState<Season[] | null>(null);
  const [error, setError] = useState("");
  const [seasonNumber, setSeasonNumber] = useState<number | null>(null);
  const [playing, setPlaying] = useState<{ episode: Episode; startAt?: number } | null>(null);
  const [progress, setProgress] = useState<Progress | undefined>(title.progress);

  useEffect(() => {
    api
      .episodes(title.id)
      .then((s) => {
        setSeasons(s);
        const resume = resumePoint(s, title.progress);
        // Open the season to resume in, or the first that has a file to play.
        const open = resume ? s.find((x) => x.number === resume.episode.season) : s.find((x) => x.episodes.some((e) => playableVersion(e)));
        setSeasonNumber((open ?? s[0])?.number ?? null);
        // Home's Play and Continue link here with ?play=1.
        if (autoplay) {
          const first = playableEpisodes(s)[0];
          if (resume) setPlaying({ episode: resume.episode, startAt: resume.startAt });
          else if (first) setPlaying({ episode: first });
        }
      })
      .catch((e: Error) => setError(e.message));
    // Autoplay only on arrival.
  }, [title.id]);

  if (error) {
    return (
      <div className="cv-notice cv-notice--error" role="alert">
        <p>{error}</p>
      </div>
    );
  }
  if (!seasons) return <p className="cv-sub">Loading episodes…</p>;
  if (seasons.length === 0) return <p className="cv-sub">No episodes found. Scan the library again.</p>;

  const season = seasons.find((s) => s.number === seasonNumber) ?? seasons[0];
  const first = playableEpisodes(seasons)[0];
  const resume = resumePoint(seasons, progress);
  const target = (e: Episode, startAt?: number): PlayTarget => ({
    versionId: playableVersion(e)!.id,
    heading: `${title.name} · ${episodeCode(e.season, e.number, playableVersion(e)!.last_episode)}${e.name ? ` · ${e.name}` : ""}`,
    titleId: title.id,
    startAt,
  });
  const next = playing ? nextEpisode(seasons, playing.episode.id) : undefined;
  const close = () => {
    setPlaying(null);
    // Pick up the position the player just saved.
    window.setTimeout(() => api.progress(title.id).then(setProgress, () => {}), 300);
  };

  return (
    <section className="episodes" aria-label="Episodes">
      <div className="episodes-head">
        <h2 className="cv-section-title">{seasons.length === 1 ? seasonLabel(season) : "Episodes"}</h2>
        {resume ? (
          <div className="cv-actions">
            <button className="cv-btn cv-btn--primary" onClick={() => setPlaying({ episode: resume.episode, startAt: resume.startAt })}>
              {resume.startAt > 0 ? "Resume" : "Play"} {episodeCode(resume.episode.season, resume.episode.number)}
              {resume.left ? <span className="cv-btn__note">{resume.left}</span> : null}
            </button>
            {resume.startAt > 0 ? (
              <button className="cv-btn" onClick={() => setPlaying({ episode: resume.episode })}>
                Start over
              </button>
            ) : null}
          </div>
        ) : first ? (
          <button className="cv-btn cv-btn--primary" onClick={() => setPlaying({ episode: first })}>
            Play {episodeCode(first.season, first.number)}
          </button>
        ) : null}
      </div>

      {seasons.length > 1 ? (
        <div className="edition-tabs" role="tablist">
          {seasons.map((s) => {
            const owned = s.episodes.filter((e) => e.versions.length > 0 || e.part_of).length;
            const aired = s.episodes.filter((e) => !isUpcoming(e.air_date));
            const firstAir = s.episodes.map((e) => e.air_date).find(Boolean);
            const count =
              owned === 0 && aired.length === 0 && firstAir
                ? `Airs ${formatDate(firstAir)}`
                : owned === s.episodes.length
                  ? `${owned} episodes`
                  : `${owned} of ${s.episodes.length} episodes`;
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
                  {count}
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
            <EpisodeRow key={e.id} episode={e} onPlay={() => setPlaying({ episode: e })} />
          ))}
        </ol>
      </div>

      {playing ? (
        <Player
          target={target(playing.episode, playing.startAt)}
          onClose={close}
          upNext={next ? `${episodeCode(next.season, next.number)}${next.name ? ` · ${next.name}` : ""}` : undefined}
          onEnded={next ? () => setPlaying({ episode: next }) : undefined}
        />
      ) : null}
    </section>
  );
}

function EpisodeRow({ episode: e, onPlay }: { episode: Episode; onPlay: () => void }) {
  const v = playableVersion(e);
  const owned = e.versions.length > 0;
  const facts = [isUpcoming(e.air_date) ? "" : formatDate(e.air_date), e.runtime_minutes ? `${e.runtime_minutes} min` : "", v ? resolution(v) : ""].filter(Boolean);
  const label = `Play episode ${e.number}${e.name ? `, ${e.name}` : ""}`;

  const upcoming = isUpcoming(e.air_date);
  let status: ReactNode = null;
  if (e.part_of) status = <span className="episode-note">In the episode {e.part_of} file</span>;
  else if (!owned && upcoming) status = <span className="episode-note">Airs {formatDate(e.air_date)}</span>;
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
          {isPlaceholderName(e.name, e.number) ? null : e.name}
        </h3>
        {facts.length ? <p className="episode-facts">{facts.join("   ")}</p> : null}
        {e.overview ? <p className="episode-overview">{e.overview}</p> : null}
        {status}
      </div>
    </li>
  );
}
