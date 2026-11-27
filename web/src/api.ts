// Types mirror the Go JSON in internal/catalog and internal/server.

export type MediaType = "movie" | "tv" | "music" | "audiobook" | "book";

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
  editions: Edition[];
}

export interface ScanResult {
  files_seen: number;
  added: number;
  updated: number;
  unchanged: number;
  missing: number;
  errors: { path: string; error: string }[];
  match: { matched: number; not_found: number; errors: number; messages: string[]; error?: string } | null;
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
  scan: (libraryId: number) => request<ScanResult>("POST", `/api/libraries/${libraryId}/scan`),
};
