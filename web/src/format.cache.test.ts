import { describe, expect, it } from "vitest";
import type { Edition, Title } from "./api";
import { defaultEdition, editionRuntime, isUnowned, ownedEditions, posterBadge, timeAgo } from "./format";

const file = { id: 1, path: "/m/Alien.mkv", size_bytes: 1, edition_locked: false, missing: false };
const owned: Edition = { id: 1, edition_type_id: "directors_cut", display_name: "Director's Cut", short_label: "DC", runtime_seconds: 6960, versions: [file] };
const placeholder: Edition = {
  id: 2,
  edition_type_id: "theatrical",
  display_name: "Theatrical",
  short_label: "Theatrical",
  from_cache: true,
  cache: { status: "published", runtime_min: 117 },
  versions: [],
};
const alien: Title = {
  id: 1, library_id: 1, media_type: "movie", name: "Alien", sort_name: "alien", genres: [], match_status: "matched",
  cache_state: "cached", added_at: "2026-11-01T00:00:00Z", editions: [placeholder, owned],
};

describe("cache placeholders", () => {
  it("are editions from the cache with no files", () => {
    expect(isUnowned(placeholder)).toBe(true);
    expect(isUnowned(owned)).toBe(false);
    expect(isUnowned({ ...placeholder, versions: [file] })).toBe(false);
  });
  it("don't count as editions you have", () => {
    expect(ownedEditions(alien)).toEqual([owned]);
    expect(posterBadge(alien)).toBe("");
    expect(posterBadge({ ...alien, editions: [placeholder, owned, { ...owned, id: 3 }] })).toBe("2 editions");
  });
  it("aren't opened first", () => {
    expect(defaultEdition(alien)).toBe(owned);
    expect(defaultEdition({ ...alien, editions: [placeholder, { ...owned, versions: [{ ...file, missing: true }] }] })?.id).toBe(1);
  });
  it("use the cache's runtime when there's no file", () => {
    expect(editionRuntime(placeholder)).toBe(117 * 60);
    expect(editionRuntime(owned)).toBe(6960);
  });
});

describe("timeAgo", () => {
  const now = new Date("2026-11-27T12:00:00Z");
  it("says how long ago", () => {
    expect(timeAgo("2026-11-27T11:59:40Z", now)).toBe("just now");
    expect(timeAgo("2026-11-27T11:55:00Z", now)).toBe("5 minutes ago");
    expect(timeAgo("2026-11-27T09:00:00Z", now)).toBe("3 hours ago");
    expect(timeAgo("2026-11-26T10:00:00Z", now)).toBe("yesterday");
    expect(timeAgo(undefined, now)).toBe("");
    expect(timeAgo("0001-01-01T00:00:00Z", now)).toBe("");
  });
});
