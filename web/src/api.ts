// Types mirror the Go JSON in internal/catalog and internal/server.

export type MediaType = "movie" | "tv" | "music" | "musicvideo" | "audiobook" | "book" | "comic";

export interface Library {
  id: number;
  name: string;
  media_type: MediaType;
  paths: string[];
  created_at: string;
}

export interface Version {
  id: number;
  path: string;
  size_bytes: number;
  label?: string;
  container?: string;
  video_codec?: string;
  width?: number;
  height?: number;
  hdr?: string;
  audio_codec?: string;
  audio_channels?: number;
  duration_seconds?: number;
  bitrate?: number;
  edition_locked: boolean;
  missing: boolean;
  /** Last episode in a multi-episode file (S01E01-E02). */
  last_episode?: number;
  /** Chapter marks (audiobooks). */
  chapters?: Chapter[];
  /** Comics: number of pages. */
  page_count?: number;
}

export interface Chapter {
  start: number; // seconds
  end: number;
  name: string;
}

/** Where playback of a title stopped; version_id is 0 when it hasn't been played. */
export interface Progress {
  version_id: number;
  position_seconds: number;
  /** Books: the exact spot (an EPUB CFI). */
  locator?: string;
  updated_at: string;
}

export interface Edition {
  id: number;
  edition_type_id: string;
  display_name: string;
  short_label: string;
  runtime_seconds?: number;
  /** Set for editions the CineVault Cache told us about. With no versions it's one the library doesn't have. */
  from_cache?: boolean;
  cache?: EditionCache;
  versions: Version[];
}

/** What the CineVault Cache says about an edition. */
export interface EditionCache {
  /** "withdrawn" shows as No longer verified. */
  status: "published" | "withdrawn";
  display_name?: string;
  runtime_min?: number;
  runtime_sources?: string[];
  difference_short?: string;
  difference_full?: { change: "added" | "removed" | "changed" | "extended" | "shortened"; text: string; sources: string[] }[];
  confidence?: "high" | "medium" | "low";
}

export interface CacheSource {
  id: string;
  name: string;
  url: string;
}

export type CacheState = "none" | "pending" | "cached" | "not_found" | "error";

export interface CacheSettings {
  /** The master switch: off, the server never contacts the cache. */
  enabled: boolean;
  check_interval: "1h" | "6h" | "24h" | "168h" | "manual";
  share: boolean;
}

/** GET /api/health: what this server is running and can do. */
export interface Health {
  status: "ok" | "degraded";
  version: string;
  database: { reachable: boolean; schema_version: number; error?: string };
  metadata: { tmdb_configured: boolean };
  playback: { ready: boolean; encoder?: string; device?: string; note?: string };
  transcode: {
    ffmpeg: { found: boolean; path: string; version?: string; hwaccels: string[]; encoders: { name: string; codec: string; kind: string; device_name?: string; usable: boolean; error?: string }[]; error?: string };
    gpus: { index: number; name: string; driver: string; memory_mib: number }[] | null;
  };
}

export interface CacheStatus {
  /** The cache is on and a subscriber token is set. */
  configured: boolean;
  url: string;
  /** Where the token comes from; its last characters. The token itself is never sent. */
  token_source?: "environment" | "saved";
  token_hint?: string;
  subscription?: { status: "active" | "lapsed" | "cancelled"; server_label?: string; expires_at?: string };
  /** Why the subscription couldn't be checked. */
  problem?: string;
  settings: CacheSettings;
  last_check?: string;
  last_error?: string;
  last_share?: string;
  last_share_error?: string;
  checking: boolean;
  counts: Partial<Record<CacheState, number>>;
}

export type ReportField = "exists" | "runtime" | "difference_short" | "difference_full" | "content_rating" | "other";

export interface Title {
  id: number;
  library_id: number;
  media_type: MediaType;
  name: string;
  sort_name: string;
  year?: number;
  tmdb_id?: number;
  original_title?: string;
  overview?: string;
  tagline?: string;
  release_date?: string;
  runtime_minutes?: number;
  poster_url?: string;
  backdrop_url?: string;
  imdb_id?: string;
  genres: string[];
  rating?: number;
  collection_name?: string;
  match_status: "unmatched" | "matched" | "not_found" | "error";
  match_error?: string;
  /** Music videos and albums: who performs it. */
  artist?: string;
  /** Albums: a track file whose cover art stands for the album, and how many tracks have files. */
  cover_version_id?: number;
  track_count?: number;
  /** Audiobooks: series, number in it, narrator. */
  series?: string;
  series_index?: number;
  narrator?: string;
  /** TV shows: seasons and episodes that have files. */
  season_count?: number;
  episode_count?: number;
  /** Movies and shows: the CineVault Cache's state for this title. */
  cache_state?: CacheState;
  /** The cache withdrew its record: No longer verified. */
  cache_withdrawn?: boolean;
  cache_sources?: CacheSource[];
  /** Movies and shows: the top of the bill from TMDb, and the directors (a show's creators). */
  cast?: CastMember[];
  directors?: string[];
  /** When the newest file arrived. */
  added_at: string;
  /** Where playback or reading stopped; absent when never started. */
  progress?: TitleProgress;
  /** Music videos: the MusicBrainz match, its album and year, and the cache's
   *  credits (sent only to subscribed servers) for the MTV-style overlay. */
  mbid?: string;
  album?: string;
  album_year?: number;
  credits?: TitleCredits;
  editions: Edition[];
}

/** A music video's on-screen credits from the CineVault Cache. */
export interface TitleCredits {
  song: string;
  artist: string;
  album?: string;
  year?: number;
  label?: string;
  director?: string;
  confidence?: string;
}

export interface CastMember {
  name: string;
  character?: string;
  profile_url?: string;
}

/** Saved progress with what's needed to show how far along it is. */
export interface TitleProgress extends Progress {
  /** Videos and audio: the file's length. */
  duration_seconds?: number;
  /** Comics: the issue's page count (position_seconds is the page, from 0). */
  page_count?: number;
  /** Shows: the episode the file belongs to. */
  season_number?: number;
  episode_number?: number;
  episode_name?: string;
}

export interface Episode {
  id: number;
  season: number;
  number: number;
  name?: string;
  overview?: string;
  air_date?: string;
  runtime_minutes?: number;
  still_url?: string;
  rating?: number;
  /** Set when this episode is inside another episode's file: that episode's number. */
  part_of?: number;
  /** Empty when the library has no file for this episode. */
  versions: Version[];
}

export interface Track {
  id: number;
  disc: number;
  number: number; // 0 when unknown
  name: string;
  /** Set when it differs from the album artist. */
  artist?: string;
  versions: Version[];
}

/** One edition of an album (Original, Deluxe...) with its tracks. */
export interface AlbumEdition {
  edition_id: number;
  tracks: Track[];
}

export interface Season {
  number: number; // 0 is Specials
  name?: string;
  overview?: string;
  poster_url?: string;
  air_date?: string;
  episodes: Episode[];
}

export interface ScanResult {
  files_seen: number;
  added: number;
  updated: number;
  unchanged: number;
  missing: number;
  errors: { path: string; error: string }[];
  match: { matched: number; not_found: number; errors: number; episodes?: number; messages: string[]; error?: string } | null;
}

export interface ClientCaps {
  containers: string[];
  video: string[];
  audio: string[];
}

export interface PlayResponse {
  mode: "direct" | "hls";
  url: string;
  session_id?: string;
  reasons: string[];
  device?: string;
  encoder?: string;
}

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      headers: body ? { "Content-Type": "application/json" } : undefined,
      body: body ? JSON.stringify(body) : undefined,
    });
  } catch {
    throw new ApiError("Can't reach the CineVault server. Check that it's running.", 0);
  }
  const data = await res.json().catch(() => null);
  if (!res.ok) {
    const msg = data && typeof data.error === "string" ? data.error : `The server answered ${res.status}.`;
    throw new ApiError(msg, res.status);
  }
  return data as T;
}

export const api = {
  libraries: () => request<Library[]>("GET", "/api/libraries"),
  createLibrary: (name: string, mediaType: MediaType, paths: string[]) =>
    request<Library>("POST", "/api/libraries", { name, media_type: mediaType, paths }),
  titles: (libraryId: number) => request<Title[]>("GET", `/api/libraries/${libraryId}/titles`),
  tracks: (titleId: number) => request<AlbumEdition[]>("GET", `/api/titles/${titleId}/tracks`),
  progress: (titleId: number) => request<Progress>("GET", `/api/titles/${titleId}/progress`),
  /** Saves a listening position; keepalive lets it finish while the page closes. */
  saveProgress: (titleId: number, versionId: number, seconds: number) => {
    fetch(`/api/titles/${titleId}/progress`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ version_id: versionId, position_seconds: Math.max(0, seconds) }),
      keepalive: true,
    }).catch(() => {});
  },
  /** Saves a reading position: the fraction read and the exact spot. */
  saveReading: (titleId: number, versionId: number, fraction: number, locator: string) => {
    fetch(`/api/titles/${titleId}/progress`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ version_id: versionId, position_seconds: Math.min(1, Math.max(0, fraction || 0)), locator }),
      keepalive: true,
    }).catch(() => {});
  },
  cache: () => request<CacheStatus>("GET", "/api/cache"),
  saveCacheSettings: (s: CacheSettings) => request<CacheSettings>("PUT", "/api/cache/settings", s),
  checkCache: () => request<{ started: boolean }>("POST", "/api/cache/check"),
  /** Checks and saves a subscriber token; note is set when it couldn't be checked. */
  saveCacheToken: (token: string) => request<{ note: string }>("PUT", "/api/cache/token", { token }),
  clearCacheToken: () => request<null>("DELETE", "/api/cache/token"),
  health: () => request<Health>("GET", "/api/health"),
  report: (titleId: number, edition: string, field: ReportField, message: string) =>
    request<{ report_id: string }>("POST", `/api/titles/${titleId}/report`, { edition, field, message }),
  episodes: (titleId: number) => request<Season[]>("GET", `/api/titles/${titleId}/episodes`),
  scan: (libraryId: number) => request<ScanResult>("POST", `/api/libraries/${libraryId}/scan`),
  play: (versionId: number, caps: ClientCaps) => request<PlayResponse>("POST", "/api/playback", { version_id: versionId, caps }),
  /** Ends a streaming session; keepalive lets it finish while the page closes. */
  stopPlayback: (sessionId: string) => {
    fetch(`/api/playback/${sessionId}`, { method: "DELETE", keepalive: true }).catch(() => {});
  },
};
