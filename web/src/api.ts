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
  updated_at: string;
}

export interface Edition {
  id: number;
  edition_type_id: string;
  display_name: string;
  short_label: string;
  runtime_seconds?: number;
  versions: Version[];
}

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
  editions: Edition[];
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
  episodes: (titleId: number) => request<Season[]>("GET", `/api/titles/${titleId}/episodes`),
  scan: (libraryId: number) => request<ScanResult>("POST", `/api/libraries/${libraryId}/scan`),
  play: (versionId: number, caps: ClientCaps) => request<PlayResponse>("POST", "/api/playback", { version_id: versionId, caps }),
  /** Ends a streaming session; keepalive lets it finish while the page closes. */
  stopPlayback: (sessionId: string) => {
    fetch(`/api/playback/${sessionId}`, { method: "DELETE", keepalive: true }).catch(() => {});
  },
};
