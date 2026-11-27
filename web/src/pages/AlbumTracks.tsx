import { useEffect, useState } from "react";
import { api, type AlbumEdition, type Title, type Track } from "../api";
import { useAudio, type QueueItem } from "../components/AudioPlayer";
import { albumSummary, byDisc, coverUrl, formatClock, trackFile } from "../format";

/** An album's songs, with edition tabs when there's more than one edition. */
export function AlbumTracks({ title }: { title: Title }) {
  const [editions, setEditions] = useState<AlbumEdition[] | null>(null);
  const [error, setError] = useState("");
  const [editionId, setEditionId] = useState<number | null>(null);
  const audio = useAudio();

  useEffect(() => {
    api
      .tracks(title.id)
      .then((eds) => {
        setEditions(eds);
        setEditionId(eds[0]?.edition_id ?? null);
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
  if (!editions) return <p className="page-sub">Loading songs…</p>;
  if (editions.length === 0) return <p className="page-sub">No songs found. Scan the library again.</p>;

  const edition = editions.find((e) => e.edition_id === editionId) ?? editions[0];
  const nameOf = (id: number) => title.editions.find((e) => e.id === id)?.display_name ?? "Edition";
  const playable = edition.tracks.filter((t) => trackFile(t));
  const queue: QueueItem[] = playable.map((t) => ({
    versionId: trackFile(t)!.id,
    title: t.name,
    artist: t.artist || title.artist || "",
    album: title.name,
    coverUrl: coverUrl(title),
    href: `/libraries/${title.library_id}/titles/${title.id}`,
  }));
  const playFrom = (t: Track) => audio.play(queue, Math.max(0, playable.indexOf(t)));
  const nowId = audio.queue[audio.index]?.versionId;
  const discs = byDisc(edition.tracks);

  return (
    <section className="album" aria-label="Songs">
      <div className="episodes-head">
        <h2 className="section-title">{albumSummary(edition.tracks)}</h2>
        <button className="btn btn-primary" disabled={queue.length === 0} onClick={() => audio.play(queue, 0)}>
          Play album
        </button>
      </div>

      {editions.length > 1 ? (
        <div className="edition-tabs" role="tablist">
          {editions.map((e) => (
            <button
              key={e.edition_id}
              role="tab"
              id={`album-tab-${e.edition_id}`}
              aria-selected={e.edition_id === edition.edition_id}
              aria-controls="album-panel"
              className="edition-tab"
              onClick={() => setEditionId(e.edition_id)}
            >
              <span className="edition-tab-name">{nameOf(e.edition_id)}</span>
              <span className="edition-tab-runtime">{albumSummary(e.tracks)}</span>
            </button>
          ))}
        </div>
      ) : null}

      <div
        className={editions.length > 1 ? "edition-panel" : undefined}
        id="album-panel"
        role={editions.length > 1 ? "tabpanel" : undefined}
        aria-labelledby={editions.length > 1 ? `album-tab-${edition.edition_id}` : undefined}
      >
        {discs.map((d) => (
          <div key={d.disc} className="disc">
            {discs.length > 1 ? <h3 className="disc-name">Disc {d.disc}</h3> : null}
            <ol className="tracklist">
              {d.tracks.map((t) => {
                const f = trackFile(t);
                const now = f && f.id === nowId;
                return (
                  <li key={t.id} className={`track${now ? " track-now" : ""}${f ? "" : " track-missing"}`}>
                    <button
                      className="track-play"
                      disabled={!f}
                      onClick={() => playFrom(t)}
                      aria-label={`Play ${t.name}`}
                      aria-current={now ? "true" : undefined}
                    >
                      <span className="track-no" aria-hidden="true">
                        {now && audio.playing ? <span className="track-eq" /> : t.number || "–"}
                      </span>
                      <span className="track-name">
                        {t.name}
                        {t.artist ? <span className="track-artist">{t.artist}</span> : null}
                      </span>
                      <span className="track-format">{f ? (f.label ?? "").split(" · ")[0].replace(/ 2\.0$/, "") : "File missing"}</span>
                      <span className="track-time">{f ? formatClock(f.duration_seconds) : ""}</span>
                    </button>
                  </li>
                );
              })}
            </ol>
          </div>
        ))}
      </div>
    </section>
  );
}
