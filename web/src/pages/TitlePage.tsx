import { useEffect, useState, type FormEvent } from "react";
import { api, type Edition, type ReportField, type Title } from "../api";
import { Poster } from "../components/PosterCard";
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
  usesTMDb,
} from "../format";
import { ShowEpisodes } from "./ShowEpisodes";
import { AlbumTracks } from "./AlbumTracks";
import { BookChapters } from "./BookChapters";
import { EbookFormats } from "./EbookFormats";
import { ComicIssues } from "./ComicIssues";
import { linkTo } from "../router";

export function TitlePage({ libraryId, titleId }: { libraryId: number; titleId: number }) {
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

  const edition = title.editions.find((e) => e.id === editionId) ?? title.editions[0];
  const isShow = title.media_type === "tv";
  const runtime = title.runtime_minutes ? title.runtime_minutes * 60 : edition ? editionRuntime(edition) : 0;
  const isAlbum = title.media_type === "music";
  const isBook = title.media_type === "audiobook";
  const isEbook = title.media_type === "book";
  const isComic = title.media_type === "comic";
  const facts = (
    isShow
      ? [title.year, showCounts(title), title.genres.join(", ")]
      : isComic
        ? [title.year, title.track_count ? `${title.track_count} ${title.track_count === 1 ? "issue" : "issues"}` : ""]
        : isAlbum || isBook || isEbook
          ? [title.artist, title.year, title.genres.filter((g) => !/^(audio ?books?|speech|spoken word)$/i.test(g)).join(", ")]
          : [title.artist, title.year, runtime ? formatRuntime(runtime) : "", title.genres.join(", ")]
  ).filter(Boolean);

  return (
    <article className="title-page">
      {title.backdrop_url ? <div className="backdrop" style={{ backgroundImage: `url(${title.backdrop_url})` }} aria-hidden="true" /> : null}

      <a className="back" {...linkTo(`/libraries/${libraryId}`)}>
        All {itemNoun(title.media_type, 2)}
      </a>

      <div className="title-head">
        <Poster title={title} size="large" />
        <div className="title-info">
          <h1 className="title-name">{title.name}</h1>
          {facts.length ? <p className="title-facts">{facts.join("   ")}</p> : null}
          {(isBook || isEbook) && (title.series || title.narrator) ? (
            <p className="tagline">{[seriesLabel(title), title.narrator ? `Read by ${title.narrator}` : ""].filter(Boolean).join(". ")}</p>
          ) : null}
          {title.tagline ? <p className="tagline">{title.tagline}</p> : null}
          {title.overview ? <p className="overview">{title.overview}</p> : null}
          <MatchNote title={title} />
          <CacheNote title={title} />
          {title.collection_name ? <p className="collection">Part of the {title.collection_name}</p> : null}
        </div>
      </div>

      {isComic ? (
        <ComicIssues title={title} />
      ) : isEbook ? (
        <EbookFormats title={title} />
      ) : isBook ? (
        <BookChapters title={title} />
      ) : isAlbum ? (
        <AlbumTracks title={title} />
      ) : isShow ? (
        <ShowEpisodes title={title} />
      ) : (
        <EditionTabs title={title} selected={edition} onSelect={setEditionId} onPlay={setPlaying} />
      )}
      {playing ? <Player target={playing} onClose={() => setPlaying(null)} /> : null}
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

function CacheNote({ title }: { title: Title }) {
  if (title.cache_state === "pending") {
    return <p className="match-note">The CineVault Cache is researching this title's editions. They'll appear here when it's done.</p>;
  }
  if (title.cache_state === "cached" && title.cache_withdrawn) {
    return <p className="match-note">No longer verified: the CineVault Cache withdrew its information about this title.</p>;
  }
  return null;
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
  const owned = ownedEditions(title);
  const unowned = title.editions.filter(isUnowned);
  const single = title.editions.length === 1;
  const missing = isUnowned(selected);
  const heading = owned.length === 1 && !missing ? title.name : `${title.name} · ${selected.display_name}`;
  // Play the highest-resolution file that's present; versions arrive tallest first.
  const best = selected.versions.find((v) => !v.missing);
  const sectionTitle = single
    ? "Your copy"
    : owned.length === 1
      ? `1 edition in your library${unowned.length ? `, ${unowned.length} more known` : ""}`
      : `${owned.length} editions in your library${unowned.length ? `, ${unowned.length} more known` : ""}`;

  return (
    <section className="editions" aria-label="Editions">
      <h2 className="section-title">{sectionTitle}</h2>
      {single ? null : (
        <div className="edition-tabs" role="tablist">
          {[...owned, ...unowned].map((e) => (
            <button
              key={e.id}
              role="tab"
              id={`edition-tab-${e.id}`}
              aria-selected={e.id === selected.id}
              aria-controls="edition-panel"
              className={isUnowned(e) ? "edition-tab edition-tab-unowned" : "edition-tab"}
              onClick={() => onSelect(e.id)}
            >
              <span className="edition-tab-name">{e.cache?.display_name || e.display_name}</span>
              <span className="edition-tab-runtime">
                {isUnowned(e) ? "Not in library" : formatRuntime(editionRuntime(e)) || "Runtime unknown"}
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
        {missing ? (
          <p className="edition-unowned-note">You don't have this edition. The CineVault Cache knows it exists.</p>
        ) : (
          <div className="edition-actions">
            <button className="btn btn-primary" disabled={!best} onClick={() => best && onPlay({ versionId: best.id, heading })}>
              Play {owned.length === 1 && !unowned.length ? "" : selected.display_name}
            </button>
            {best ? null : <span className="hint">The files for this edition are missing.</span>}
          </div>
        )}
        <EditionCacheInfo title={title} edition={selected} />
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
        )}
      </div>
    </section>
  );
}

const CHANGE_WORD: Record<string, string> = {
  added: "Added",
  removed: "Removed",
  changed: "Changed",
  extended: "Longer",
  shortened: "Shorter",
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

  return (
    <div className={withdrawn ? "cache-info cache-info-withdrawn" : "cache-info"}>
      {withdrawn ? <p className="cache-flag">No longer verified. The CineVault Cache withdrew this information, so it may be wrong.</p> : null}
      {c.difference_short ? <p className="cache-short">{c.difference_short}</p> : null}
      {full.length ? (
        <details className="cache-full">
          <summary>What's different in this cut</summary>
          <ul>
            {full.map((d, i) => (
              <li key={i}>
                <span className={`cache-change cache-change-${d.change}`}>{CHANGE_WORD[d.change] ?? d.change}</span> {d.text}
                {d.sources.length ? <span className="cache-cite"> ({cites(d.sources)})</span> : null}
              </li>
            ))}
          </ul>
        </details>
      ) : null}
      <p className="cache-meta">
        {c.runtime_min ? (
          <>
            Runtime {formatRuntime(c.runtime_min * 60)}
            {c.runtime_sources?.length ? <> from {cites(c.runtime_sources)}</> : null}.{" "}
          </>
        ) : null}
        From the CineVault Cache.
        {canReport && !reporting ? (
          <>
            {" "}
            <button className="linkish" onClick={() => setReporting(true)}>
              Report a mistake
            </button>
          </>
        ) : null}
      </p>
      {reporting ? <ReportForm title={title} edition={edition} onDone={() => setReporting(false)} /> : null}
    </div>
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
        <button className="linkish" onClick={onDone}>
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
        <button className="btn btn-primary" type="submit" disabled={state === "sending" || !message.trim()}>
          {state === "sending" ? "Sending…" : "Send report"}
        </button>
        <button className="btn" type="button" onClick={onDone}>
          Cancel
        </button>
      </div>
    </form>
  );
}
