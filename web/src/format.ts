import type { Chapter, Edition, Episode, Season, Title, Track, Version } from "./api";

/** How many edition labels a card shows before "Show all" (Editions spec §7). */
export const CARD_EDITION_LIMIT = 3;

/** Splits a title's editions into the labels shown first and how many are hidden. */
export function editionListing(editions: Edition[], expanded: boolean) {
  if (expanded || editions.length <= CARD_EDITION_LIMIT) {
    return { shown: editions, hidden: 0 };
  }
  return { shown: editions.slice(0, CARD_EDITION_LIMIT), hidden: editions.length - CARD_EDITION_LIMIT };
}

/** "1 h 56 min" from seconds; "" when unknown. */
export function formatRuntime(seconds?: number): string {
  if (!seconds || seconds <= 0) return "";
  const minutes = Math.round(seconds / 60);
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  if (h === 0) return `${m} min`;
  return m === 0 ? `${h} h` : `${h} h ${m} min`;
}

/** "1.8 GB" from bytes. */
export function formatSize(bytes: number): string {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let i = 0;
  while (value >= 1000 && i < units.length - 1) {
    value /= 1000;
    i++;
  }
  return `${value >= 100 || i === 0 ? Math.round(value) : value.toFixed(1)} ${units[i]}`;
}

/** "4K", "1080p", "720p", "SD", or "" — same rules as internal/mediainfo. */
export function resolution(v: Pick<Version, "width" | "height">): string {
  const w = v.width ?? 0;
  const h = v.height ?? 0;
  if (w === 0 && h === 0) return "";
  if (w >= 3200 || h >= 1800) return "4K";
  if (w >= 1800 || h >= 1000) return "1080p";
  if (w >= 1200 || h >= 700) return "720p";
  return "SD";
}

const RES_RANK: Record<string, number> = { "4K": 4, "1080p": 3, "720p": 2, SD: 1, "": 0 };

/** The best resolution across all of a title's files, for the poster badge. */
export function bestResolution(t: Title): string {
  let best = "";
  for (const e of t.editions) {
    for (const v of e.versions) {
      const r = resolution(v);
      if (RES_RANK[r] > RES_RANK[best]) best = r;
    }
  }
  return best;
}

/** True when any file of the title has HDR or Dolby Vision. */
export function hasHDR(t: Title): boolean {
  return t.editions.some((e) => e.versions.some((v) => !!v.hdr));
}

/** True for an edition the CineVault Cache knows about but the library has no file for. */
export function isUnowned(e: Edition): boolean {
  return !!e.from_cache && e.versions.length === 0;
}

/** The editions the library has: everything except the cache's placeholders. */
export function ownedEditions(t: Pick<Title, "editions">): Edition[] {
  return t.editions.filter((e) => !isUnowned(e));
}

/** An edition's runtime in seconds: the measured file first, then the cache's figure. */
export function editionRuntime(e: Edition): number {
  return e.runtime_seconds || (e.cache?.runtime_min ?? 0) * 60;
}

/** When a date was, in words: "just now", "5 minutes ago", "yesterday", or the date. */
export function timeAgo(iso: string | undefined, now = new Date()): string {
  if (!iso) return "";
  const then = new Date(iso);
  if (Number.isNaN(then.getTime()) || then.getFullYear() < 2000) return "";
  const mins = Math.round((now.getTime() - then.getTime()) / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return mins === 1 ? "a minute ago" : `${mins} minutes ago`;
  const hours = Math.round(mins / 60);
  if (hours < 24) return hours === 1 ? "an hour ago" : `${hours} hours ago`;
  const days = Math.round(hours / 24);
  if (days === 1) return "yesterday";
  if (days < 7) return `${days} days ago`;
  return then.toLocaleDateString(undefined, { day: "numeric", month: "long", year: "numeric" });
}

/** The poster badge, e.g. "2 editions · 4K" (Editions spec §7). Empty when there's nothing to say. */
export function posterBadge(t: Title): string {
  const parts: string[] = [];
  const owned = ownedEditions(t).length;
  if (owned > 1) parts.push(`${owned} editions`);
  const res = bestResolution(t);
  if (res === "4K") parts.push(hasHDR(t) ? "4K HDR" : "4K");
  return parts.join(" · ");
}

/** The edition to open first on a details page: the first with a file present. */
export function defaultEdition(t: Title): Edition | undefined {
  return t.editions.find((e) => e.versions.some((v) => !v.missing)) ?? ownedEditions(t)[0] ?? t.editions[0];
}

/** File name from a path. */
export function fileName(path: string): string {
  const i = path.lastIndexOf("/");
  return i >= 0 ? path.slice(i + 1) : path;
}

/** The version label without its leading resolution, which is shown separately. */
export function labelWithoutResolution(label: string | undefined, res: string): string {
  if (!label) return "";
  const parts = label.split(" · ");
  return parts[0] === res ? parts.slice(1).join(" · ") : label;
}

// ---------- TV ----------

/** "Season 1", "Specials", or TMDb's season name when it's more than that. */
export function seasonLabel(s: Pick<Season, "number" | "name">): string {
  if (s.number === 0) return "Specials";
  return s.name && s.name !== `Season ${s.number}` ? s.name : `Season ${s.number}`;
}

/** "S1 E3", "S1 E1–E2" for a two-episode file, "Special 2" for season 0. */
export function episodeCode(season: number, episode: number, last?: number): string {
  const range = last && last > episode ? `${episode}–E${last}` : `${episode}`;
  return season === 0 ? `Special ${range.replace("E", "")}` : `S${season} E${range}`;
}

/** The file to play for an episode: the tallest one that's present. */
export function playableVersion(e: Episode): Version | undefined {
  return e.versions.find((v) => !v.missing);
}

/** Episodes with a playable file, in viewing order (specials come last from the server). */
export function playableEpisodes(seasons: Season[]): Episode[] {
  return seasons.flatMap((s) => s.episodes.filter((e) => playableVersion(e)));
}

/** The next playable episode after the given one, for Up Next. */
export function nextEpisode(seasons: Season[], episodeId: number): Episode | undefined {
  const list = playableEpisodes(seasons);
  const i = list.findIndex((e) => e.id === episodeId);
  return i >= 0 ? list[i + 1] : undefined;
}

/** "2 seasons · 14 episodes" for a show card; "" when nothing is known. */
export function showCounts(t: Pick<Title, "season_count" | "episode_count">): string {
  const parts: string[] = [];
  if (t.season_count) parts.push(`${t.season_count} ${t.season_count === 1 ? "season" : "seasons"}`);
  if (t.episode_count) parts.push(`${t.episode_count} ${t.episode_count === 1 ? "episode" : "episodes"}`);
  return parts.join(" · ");
}

/** "21 Mar 2024" from "2024-03-21". */
export function formatDate(iso?: string): string {
  if (!iso) return "";
  const d = new Date(`${iso}T00:00:00`);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleDateString(undefined, { day: "numeric", month: "short", year: "numeric" });
}

/** True when an episode's air date is after today (it hasn't aired yet). */
export function isUpcoming(airDate: string | undefined, today = new Date()): boolean {
  if (!airDate) return false;
  const d = new Date(`${airDate}T00:00:00`);
  return !Number.isNaN(d.getTime()) && d > today;
}

/** TMDb's placeholder names like "Episode 3" add nothing next to the number. */
export function isPlaceholderName(name: string | undefined, number: number): boolean {
  return !name || name.trim().toLowerCase() === `episode ${number}`;
}

// ---------- Music videos ----------

/** Movies and TV are matched to TMDb. */
export function usesTMDb(t: Pick<Title, "media_type">): boolean {
  return t.media_type === "movie" || t.media_type === "tv";
}

/** Where a library's titles are looked up. */
export function matchSource(mediaType: string): string {
  switch (mediaType) {
    case "movie":
    case "tv":
      return "TMDb";
    case "music":
    case "musicvideo":
      return "MusicBrainz";
    case "book":
    case "audiobook":
      return "Open Library";
    case "comic":
      return "Comic Vine";
  }
  return "";
}

/** Whether "Match again" is offered: every lookup but TMDb's. */
export function canRematch(mediaType: string): boolean {
  return matchSource(mediaType) !== "" && mediaType !== "movie" && mediaType !== "tv";
}

/** Names of the fields a person can edit and lock, as the title page shows them. */
export function fieldLabel(field: string, mediaType: string): string {
  switch (field) {
    case "name":
      return mediaType === "comic" ? "Series" : "Title";
    case "artist":
      return mediaType === "book" || mediaType === "audiobook" ? "Author" : "Artist";
    case "overview":
      return "Description";
    case "series_index":
      return "Number in series";
    case "poster":
      return "Artwork";
  }
  return field.charAt(0).toUpperCase() + field.slice(1).replace(/_/g, " ");
}

/** The fields each media type's edit form offers, in order. */
export function editableFields(mediaType: string): string[] {
  switch (mediaType) {
    case "music":
      return ["name", "artist", "year", "genres", "overview"];
    case "musicvideo":
      return ["name", "artist", "year", "genres"];
    case "book":
    case "audiobook":
      return mediaType === "audiobook"
        ? ["name", "artist", "narrator", "series", "series_index", "year", "genres", "overview"]
        : ["name", "artist", "series", "series_index", "year", "genres", "overview"];
    case "comic":
      return ["name", "publisher", "year", "genres", "overview"];
  }
  return ["name", "year", "genres", "overview"];
}

/** A still from the title's first present file, for titles without artwork. */
export function thumbUrl(t: Pick<Title, "editions">): string | undefined {
  for (const e of t.editions) {
    const v = e.versions.find((x) => !x.missing);
    if (v) return `/api/versions/${v.id}/thumb`;
  }
  return undefined;
}

/** Titles grouped by artist, in the order they arrive (the server sorts by artist). */
export function groupByArtist<T extends Pick<Title, "artist">>(titles: T[]): { artist: string; titles: T[] }[] {
  const groups: { artist: string; titles: T[] }[] = [];
  for (const t of titles) {
    const artist = t.artist || "Unknown artist";
    const last = groups[groups.length - 1];
    if (last && last.artist.toLowerCase() === artist.toLowerCase()) last.titles.push(t);
    else groups.push({ artist, titles: [t] });
  }
  return groups;
}

/** What a library calls its items: "movies", "shows", "music videos". */
export function itemNoun(mediaType: string | undefined, count: number): string {
  const one = count === 1;
  switch (mediaType) {
    case "tv":
      return one ? "show" : "shows";
    case "musicvideo":
      return one ? "music video" : "music videos";
    case "music":
      return one ? "album" : "albums";
    case "audiobook":
      return one ? "audiobook" : "audiobooks";
    case "book":
      return one ? "book" : "books";
    case "comic":
      return "series";
    default:
      return one ? "movie" : "movies";
  }
}

// ---------- Music ----------

/** Album cover from the album's files (a cover.jpg next to them, or embedded art). */
export function coverUrl(t: Pick<Title, "cover_version_id">): string | undefined {
  return t.cover_version_id ? `/api/versions/${t.cover_version_id}/cover` : undefined;
}

/** "3:56", or "1:02:03" past an hour. */
export function formatClock(seconds: number | undefined): string {
  if (!seconds || !Number.isFinite(seconds) || seconds < 0) return "0:00";
  const s = Math.floor(seconds);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const ss = String(s % 60).padStart(2, "0");
  return h ? `${h}:${String(m).padStart(2, "0")}:${ss}` : `${m}:${ss}`;
}

/** The file to play for a track: the first one that's present. */
export function trackFile(t: Track): Version | undefined {
  return t.versions.find((v) => !v.missing);
}

/** "12 songs · 47 min" for an album edition. */
export function albumSummary(tracks: Track[]): string {
  const secs = tracks.reduce((sum, t) => sum + (trackFile(t)?.duration_seconds ?? 0), 0);
  const songs = `${tracks.length} ${tracks.length === 1 ? "song" : "songs"}`;
  return secs ? `${songs} · ${formatRuntime(secs)}` : songs;
}

/** Tracks split by disc, only when an album has more than one. */
export function byDisc(tracks: Track[]): { disc: number; tracks: Track[] }[] {
  const out: { disc: number; tracks: Track[] }[] = [];
  for (const t of tracks) {
    const last = out[out.length - 1];
    if (last && last.disc === t.disc) last.tracks.push(t);
    else out.push({ disc: t.disc, tracks: [t] });
  }
  return out;
}

// ---------- Audiobooks ----------

/** The chapter playing at a time, if the file has chapters. */
export function chapterAt(chapters: Chapter[] | undefined, seconds: number): Chapter | undefined {
  if (!chapters?.length) return undefined;
  let found = chapters[0];
  for (const c of chapters) {
    if (c.start <= seconds + 0.5) found = c;
    else break;
  }
  return found;
}

/** "The John Matherson Series, book 2", "The Lord of the Rings, book 4.5", or the series alone. */
export function seriesLabel(t: Pick<Title, "series" | "series_index">): string {
  if (!t.series) return "";
  return t.series_index ? `${t.series}, book ${t.series_index}` : t.series;
}

/** "3 h 12 min left" for a resume button. */
export function timeLeft(total: number, at: number): string {
  const secs = Math.max(0, total - at);
  if (secs <= 0) return "";
  return secs < 60 ? "Under a minute left" : `${formatRuntime(secs)} left`;
}
