import { Fragment, useEffect, useState, type FormEvent } from "react";
import { api, type CastMember, type Edition, type Progress, type ReportField, type Title } from "../api";
import { Poster } from "../components/PosterCard";
import { TitleEditor } from "../components/TitleEditor";
import { Player, type PlayTarget } from "../components/Player";
import {
  defaultEdition,
  editionRuntime,
  fileName,
  formatRuntime,
  formatSize,
  isUnowned,
  itemNoun,
  labelWithoutResolution,
  ownedEditions,
  resolution,
  seriesLabel,
  showCounts,
  timeLeft,
  matchSource,
  usesTMDb,
} from "../format";
import { ShowEpisodes } from "./ShowEpisodes";
import { AlbumTracks } from "./AlbumTracks";
import { BookChapters } from "./BookChapters";
import { EbookFormats } from "./EbookFormats";
import { ComicIssues } from "./ComicIssues";
import { linkTo } from "../router";
import { Icon } from "../components/Icon";

export function TitlePage({ libraryId, titleId }: { libraryId: number; titleId: number }) {
  const [title, setTitle] = useState<Title | null | undefined>(undefined);
  const [error, setError] = useState("");
  const [editionId, setEditionId] = useState<number | null>(null);
  const [playing, setPlaying] = useState<PlayTarget | null>(null);
  const [progress, setProgress] = useState<Progress | undefined>(undefined);
  // Home's Play and Continue link here with ?play=1.
  const [autoplay] = useState(() => new URLSearchParams(location.search).get("play") === "1");
  const [reloads, setReloads] = useState(0);

  useEffect(() => {
    api
      .titles(libraryId)
      .then((all) => {
        const t = all.find((x) => x.id === titleId) ?? null;
        setTitle(t);
        setProgress(t?.progress);
        // Open on the edition being watched, else the default one.
        const watching = t ? resumeFor(t, t.progress) : undefined;
        const first = watching?.edition ?? (t ? defaultEdition(t) : undefined);
        if (first) setEditionId(first.id);
        if (autoplay) {
          history.replaceState(null, "", location.pathname);
          const v = first?.versions.find((x) => !x.missing);
          if (t && t.media_type === "movie" && first && v) {
            setPlaying({
              versionId: watching?.versionId ?? v.id,
              heading: ownedEditions(t).length > 1 ? `${t.name} · ${first.display_name}` : t.name,
              titleId: t.id,
              startAt: watching?.startAt,
            });
          }
        }
      })
      .catch((e: Error) => setError(e.message));
  }, [libraryId, titleId, autoplay, reloads]);

  if (error) {
    return (
      <div className="cv-notice cv-notice--error" role="alert">
        <p>{error}</p>
      </div>
    );
  }
  if (title === undefined) return <p className="cv-sub">Loading…</p>;
  if (title === null) {
    return (
      <div className="cv-notice">
        <p>This title isn't in the library any more.</p>
        <a className="cv-btn" {...linkTo(`/libraries/${libraryId}`)}>
          Back to the library
        </a>
      </div>
    );
  }

  const edition = title.editions.find((e) => e.id === editionId) ?? title.editions[0];
  const isShow = title.media_type === "tv";
  const runtime = title.runtime_minutes ? title.runtime_minutes * 60 : edition ? editionRuntime(edition) : 0;
  const isAlbum = title.media_type === "music";
  const isBook = title.media_type === "audiobook";
  const isEbook = title.media_type === "book";
  const isComic = title.media_type === "comic";
  const isMovie = !isShow && !isAlbum && !isBook && !isEbook && !isComic;
  const facts = (
    isShow
      ? [title.year, showCounts(title), title.genres.join(" · ")]
      : isComic
        ? [title.publisher, title.year, title.track_count ? `${title.track_count} ${title.track_count === 1 ? "issue" : "issues"}` : ""]
        : isAlbum || isBook || isEbook
          ? [title.artist, title.year, title.genres.filter((g) => !/^(audio ?books?|speech|spoken word)$/i.test(g)).join(" · ")]
          : [
              title.artist,
              title.media_type === "musicvideo" && title.album ? `${title.album}${title.album_year ? ` (${title.album_year})` : ""}` : title.year,
              runtime ? formatRuntime(runtime) : "",
              title.genres.join(" · "),
            ]
  ).filter(Boolean);
  const owned = ownedEditions(title);
  const best = edition && !isUnowned(edition) ? edition.versions.find((v) => !v.missing) : undefined;
  const heading = owned.length === 1 && edition && !isUnowned(edition) ? title.name : `${title.name} · ${edition?.display_name ?? ""}`;
  const eyebrow = title.collection_name || (isMovie && owned.length > 1 ? `${owned.length} editions in your library` : "");
  const resume = isMovie ? resumeFor(title, progress) : undefined;
  const resumeHere = resume && edition && resume.edition.id === edition.id ? resume : undefined;
  const play = (t: PlayTarget) =>
    setPlaying({ titleId: title.id, credits: title.media_type === "musicvideo" ? title.credits : undefined, ...t });
  const closePlayer = () => {
    setPlaying(null);
    // Pick up the position the player just saved.
    window.setTimeout(() => api.progress(title.id).then(setProgress, () => {}), 300);
  };
  const credit = title.directors?.length ? `${isShow ? "Created by" : "Directed by"} ${title.directors.join(", ")}` : "";

  return (
    <article className="title-page">
      <section className="cv-hero" style={title.backdrop_url ? { backgroundImage: `url(${title.backdrop_url})` } : undefined}>
        <a className="back" {...linkTo(`/libraries/${libraryId}`)}>
          All {itemNoun(title.media_type, 2)}
        </a>
        <div className="cv-hero__body">
          <Poster title={title} size="large" />
          <div className="cv-hero__text">
            {eyebrow ? <div className="cv-eyebrow">{eyebrow}</div> : null}
            <h1 className="cv-hero-title">{title.name}</h1>
            {facts.length ? (
              <div className="cv-meta">
                {facts.map((f) => (
                  <span key={String(f)}>{f}</span>
                ))}
              </div>
            ) : null}
            {(isBook || isEbook) && (title.series || title.narrator) ? (
              <p className="tagline">{[seriesLabel(title), title.narrator ? `Read by ${title.narrator}` : ""].filter(Boolean).join(". ")}</p>
            ) : null}
            {title.tagline ? <p className="tagline">{title.tagline}</p> : null}
            {title.overview ? <p className="overview">{title.overview}</p> : null}
            {credit ? <p className="credit-line">{credit}</p> : null}
            {isMovie && edition ? (
              <div className="cv-actions">
                {isUnowned(edition) ? (
                  <span className="cv-caption">You don't have this edition. Choose one you have to play.</span>
                ) : resumeHere ? (
                  <>
                    <button className="cv-btn cv-btn--primary" onClick={() => play({ versionId: resumeHere.versionId, heading, startAt: resumeHere.startAt })}>
                      <Icon name="play" filled />
                      Resume{owned.length > 1 ? ` ${edition.cache?.display_name || edition.display_name}` : ""}
                      {resumeHere.left ? <span className="cv-btn__note">{resumeHere.left}</span> : null}
                    </button>
                    <button className="cv-btn" disabled={!best} onClick={() => best && play({ versionId: best.id, heading })}>
                      Start over
                    </button>
                  </>
                ) : (
                  <button className="cv-btn cv-btn--primary" disabled={!best} onClick={() => best && play({ versionId: best.id, heading })}>
                    <Icon name="play" filled />
                    Play{owned.length > 1 ? ` ${edition.cache?.display_name || edition.display_name}` : ""}
                  </button>
                )}
                {edition && !isUnowned(edition) && !best ? <span className="cv-hint">The files for this edition are missing.</span> : null}
              </div>
            ) : null}
            <MatchNote title={title} />
            <CacheNote title={title} />
            <TitleEditor key={`${title.id}-${reloads}`} title={title} onChanged={() => setReloads((n) => n + 1)} />
          </div>
        </div>
      </section>

      <div className="title-body">
        {isComic ? (
          <ComicIssues title={title} />
        ) : isEbook ? (
          <EbookFormats title={title} />
        ) : isBook ? (
          <BookChapters title={title} />
        ) : isAlbum ? (
          <AlbumTracks title={title} />
        ) : isShow ? (
          <ShowEpisodes title={title} autoplay={autoplay} />
        ) : (
          <Editions title={title} selected={edition} onSelect={setEditionId} onPlay={play} />
        )}
        {title.cast?.length ? <Cast cast={title.cast} /> : null}
      </div>
      {playing ? <Player target={playing} onClose={closePlayer} /> : null}
    </article>
  );
}

/** Where to resume a movie: the edition and file last played, if it's part-way through. */
function resumeFor(t: Title, p: Progress | undefined) {
  if (!p) return undefined;
  for (const e of t.editions) {
    const v = e.versions.find((x) => x.id === p.version_id && !x.missing);
    if (!v) continue;
    const length = v.duration_seconds ?? 0;
    const f = length ? p.position_seconds / length : 0;
    if (f < 0.02 || f >= 0.95) return undefined;
    return { edition: e, versionId: v.id, startAt: p.position_seconds, left: timeLeft(length, p.position_seconds) };
  }
  return undefined;
}

/** The top of the bill, with photos from TMDb. */
function Cast({ cast }: { cast: CastMember[] }) {
  return (
    <section className="cv-section cast-section" aria-label="Cast">
      <div className="cv-section-head">
        <h2 className="cv-section-title">Cast</h2>
      </div>
      <ul className="cv-cast">
        {cast.map((c, i) => (
          <li key={`${c.name}-${i}`} className="cv-cast__member">
            {c.profile_url ? (
              <img className="cv-cast__photo" src={c.profile_url} alt="" loading="lazy" decoding="async" />
            ) : (
              <span className="cv-cast__photo cv-cast__photo--none" aria-hidden="true">
                {initials(c.name)}
              </span>
            )}
            <span className="cv-cast__name">{c.name}</span>
            {c.character ? <span className="cv-cast__role">{c.character}</span> : null}
          </li>
        ))}
      </ul>
    </section>
  );
}

function initials(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0])
    .join("")
    .toUpperCase();
}

function MatchNote({ title }: { title: Title }) {
  const source = matchSource(title.media_type);
  if (title.match_status === "matched" || !source || title.locked) return null;
  if (title.media_type === "comic" && title.match_status === "unmatched") return null; // no Comic Vine key, or not scanned since
  const text =
    title.match_status === "not_found"
      ? usesTMDb(title)
        ? title.media_type === "tv"
          ? "TMDb has no show with this name, so there are no posters or episode details. Check the show's folder name."
          : "TMDb has no movie with this name and year, so there's no poster or description. Check the file name."
        : `${source} has no match${title.match_error ? ` (${title.match_error})` : ""}. Check the name and artist or author, or fix them with Edit details and use Match again.`
      : title.match_status === "error"
        ? `Matching with ${source} failed: ${title.match_error ?? "unknown error"}. It will be retried on the next scan.`
        : `Not matched with ${source} yet. It will be matched on the next scan.`;
  return <p className="match-note">{text}</p>;
}

function CacheNote({ title }: { title: Title }) {
  if (title.cache_state === "pending") {
    return <p className="match-note">The CineVault Cache is researching this title's editions. They'll appear here when it's done.</p>;
  }
  if (title.cache_state === "cached" && title.cache_withdrawn) {
    return <p className="match-note">No longer verified: the CineVault Cache withdrew its information about this title.</p>;
  }
  return null;
}

function Editions({
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
  const owned = ownedEditions(title);
  const unowned = title.editions.filter(isUnowned);
  const all = [...owned, ...unowned];
  const single = title.editions.length === 1;
  const missing = isUnowned(selected);
  const heading = owned.length === 1 && !missing ? title.name : `${title.name} · ${selected.display_name}`;
  // Bar length: this edition's runtime over the longest one's.
  const runtimeOf = (e: Edition) => editionRuntime(e) || (e.cache?.runtime_min ?? 0) * 60;
  const longest = Math.max(1, ...all.map(runtimeOf));
  const sectionTitle = single ? "Your copy" : "Editions";
  const sectionNote = `${owned.length} in your library${unowned.length ? `, ${unowned.length} more known` : ""} · bar = running time`;
  const short = (sec: number) => `${Math.round(sec / 60)} m`;

  return (
    <section className="editions" aria-label="Editions">
      <div className="editions-main">
        <h2 className="cv-section-title">
          {sectionTitle}
          {single ? null : <span className="editions-hint">{sectionNote}</span>}
        </h2>
        {single ? null : (
          <div className="cv-editions" role="radiogroup" aria-label="Edition">
            {all.map((e) => {
              const rt = runtimeOf(e);
              const best = e.versions.find((v) => !v.missing);
              const info = isUnowned(e)
                ? [rt ? short(rt) : "", "not owned"].filter(Boolean).join(" · ")
                : [rt ? short(rt) : "", best ? resolution(best) : ""].filter(Boolean).join(" · ") || "Runtime unknown";
              return (
                <button
                  key={e.id}
                  role="radio"
                  aria-checked={e.id === selected.id}
                  className={`cv-edition${e.id === selected.id ? " is-selected" : ""}${isUnowned(e) ? " is-missing" : ""}`}
                  onClick={() => onSelect(e.id)}
                >
                  <span className="cv-edition__name">{e.cache?.display_name || e.display_name}</span>
                  <span className="cv-edition__bar">
                    <span style={{ width: `${rt ? Math.max(4, Math.round((rt / longest) * 100)) : 0}%` }} />
                  </span>
                  <span className="cv-edition__info">{info}</span>
                </button>
              );
            })}
          </div>
        )}
        {missing ? <p className="edition-unowned-note">You don't have this edition. The CineVault Cache knows it exists.</p> : null}
        {missing ? null : (
          <ul className="versions">
            {selected.versions.map((v) => (
              <li key={v.id} className={v.missing ? "version version-missing" : "version"}>
                <div className="version-main">
                  <span className="version-res">{resolution(v) || "Unknown"}</span>
                  <span className="version-label">{labelWithoutResolution(v.label, resolution(v))}</span>
                </div>
                <div className="version-file">
                  <span className="version-path" title={v.path}>
                    {fileName(v.path)}
                  </span>
                  <span className="version-size">{formatSize(v.size_bytes)}</span>
                  {v.missing ? (
                    <span className="version-flag">File missing</span>
                  ) : selected.versions.length > 1 ? (
                    <button className="cv-linkish" onClick={() => onPlay({ versionId: v.id, heading: `${heading} (${resolution(v)})` })}>
                      Play this file
                    </button>
                  ) : null}
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
      <div className="editions-side">
        <EditionCacheInfo title={title} edition={selected} />
      </div>
    </section>
  );
}

/** How each kind of change is written: words and a sign, never colour alone. */
const CHANGE_WORD: Record<string, { word: string; cls: string }> = {
  added: { word: "+ Added", cls: "is-added" },
  removed: { word: "− Cut", cls: "is-removed" },
  changed: { word: "~ Changed", cls: "is-changed" },
  extended: { word: "+ Longer", cls: "is-added" },
  shortened: { word: "− Shorter", cls: "is-removed" },
};

/** What the CineVault Cache says about the edition: how it differs, with sources. */
function EditionCacheInfo({ title, edition }: { title: Title; edition: Edition }) {
  const [reporting, setReporting] = useState(false);
  const c = edition.cache;
  if (!c) return null;
  const sources = new Map((title.cache_sources ?? []).map((s) => [s.id, s]));
  const withdrawn = c.status === "withdrawn" || title.cache_withdrawn;
  const full = c.difference_full ?? [];
  const canReport = title.cache_state === "cached";
  const cites = (ids: string[] | undefined) =>
    (ids ?? [])
      .map((id) => sources.get(id))
      .filter((s): s is NonNullable<typeof s> => !!s)
      .map((s, i) => (
        <span key={s.id}>
          {i > 0 ? ", " : ""}
          <a href={s.url} target="_blank" rel="noreferrer">
            {s.name}
          </a>
        </span>
      ));
  const allSources = [...new Set([...(c.runtime_sources ?? []), ...full.flatMap((d) => d.sources)])];

  return (
    <section className={withdrawn ? "cv-panel cache-info cache-info-withdrawn" : "cv-panel cache-info"} aria-label="What's different in this cut">
      <div className="cv-panel__head">
        <h3 className="cv-card-title">
          {full.length || c.difference_short ? "What's different in this cut" : "From the CineVault Cache"}
        </h3>
        {c.confidence === "medium" && !withdrawn ? (
          <span className="cv-chip cv-chip--caution" title="Published automatically from one independent source. It's raised to high when a second source agrees.">
            Medium confidence
          </span>
        ) : null}
        {withdrawn ? <span className="cv-chip cv-chip--removed">No longer verified</span> : null}
      </div>
      {withdrawn ? <p className="cache-flag">The CineVault Cache withdrew this information, so it may be wrong.</p> : null}
      {c.difference_short ? <p className="cv-lead">{c.difference_short}</p> : null}
      {full.length ? (
        <dl className="cv-diff">
          {full.map((d, i) => {
            const w = CHANGE_WORD[d.change] ?? { word: d.change, cls: "" };
            return (
              <Fragment key={i}>
                <dt className={w.cls}>{w.word}</dt>
                <dd>
                  {d.text}
                  {d.sources.length ? <span className="cache-cite"> ({cites(d.sources)})</span> : null}
                </dd>
              </Fragment>
            );
          })}
        </dl>
      ) : null}
      <p className="cv-hint cache-meta">
        {c.runtime_min ? <>Runtime {formatRuntime(c.runtime_min * 60)}. </> : null}
        {allSources.length ? <>Sources: {cites(allSources)} · </> : null}
        CineVault Cache
        {canReport && !reporting ? (
          <>
            {" · "}
            <button className="cv-linkish" onClick={() => setReporting(true)}>
              Report a mistake
            </button>
          </>
        ) : null}
      </p>
      {reporting ? <ReportForm title={title} edition={edition} onDone={() => setReporting(false)} /> : null}
    </section>
  );
}

const REPORT_FIELDS: { value: ReportField; label: string }[] = [
  { value: "exists", label: "This edition doesn't exist" },
  { value: "runtime", label: "The runtime is wrong" },
  { value: "difference_short", label: "The summary of differences is wrong" },
  { value: "difference_full", label: "A listed difference is wrong" },
  { value: "other", label: "Something else" },
];

function ReportForm({ title, edition, onDone }: { title: Title; edition: Edition; onDone: () => void }) {
  const [field, setField] = useState<ReportField>("runtime");
  const [message, setMessage] = useState("");
  const [state, setState] = useState<"idle" | "sending" | "sent">("idle");
  const [error, setError] = useState("");

  async function send(e: FormEvent) {
    e.preventDefault();
    setState("sending");
    setError("");
    try {
      await api.report(title.id, edition.edition_type_id, field, message.trim());
      setState("sent");
    } catch (err) {
      setError((err as Error).message);
      setState("idle");
    }
  }

  if (state === "sent") {
    return (
      <p className="report-sent" role="status">
        Report sent. Thanks: the cache will check it again.{" "}
        <button className="cv-linkish" onClick={onDone}>
          Close
        </button>
      </p>
    );
  }
  return (
    <form className="form report-form" onSubmit={send}>
      <label htmlFor="report-field">What's wrong with {edition.cache?.display_name || edition.display_name}</label>
      <select id="report-field" value={field} onChange={(e) => setField(e.target.value as ReportField)}>
        {REPORT_FIELDS.map((f) => (
          <option key={f.value} value={f.value}>
            {f.label}
          </option>
        ))}
      </select>
      <label htmlFor="report-message">What's right, and where you found it</label>
      <textarea
        id="report-message"
        value={message}
        onChange={(e) => setMessage(e.target.value)}
        rows={3}
        maxLength={2000}
        required
        placeholder={field === "runtime" ? "My copy runs 1 h 56 min, and the Blu-ray case says 116 minutes." : ""}
      />
      {error ? (
        <p className="form-error" role="alert">
          {error}
        </p>
      ) : null}
      <div className="report-actions">
        <button className="cv-btn cv-btn--primary" type="submit" disabled={state === "sending" || !message.trim()}>
          {state === "sending" ? "Sending…" : "Send report"}
        </button>
        <button className="cv-btn" type="button" onClick={onDone}>
          Cancel
        </button>
      </div>
    </form>
  );
}
