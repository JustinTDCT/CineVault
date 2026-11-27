import { describe, expect, it } from "vitest";
import type { Track } from "./api";
import { albumSummary, byDisc, coverUrl, formatClock, itemNoun } from "./format";

const tr = (id: number, disc: number, secs: number, missing = false): Track => ({
  id,
  disc,
  number: id,
  name: `Song ${id}`,
  versions: [{ id: id * 10, path: "/x.flac", size_bytes: 1, edition_locked: false, missing, duration_seconds: secs }],
});

describe("music", () => {
  it("formats track times", () => {
    expect(formatClock(236.9)).toBe("3:56");
    expect(formatClock(3723)).toBe("1:02:03");
    expect(formatClock(undefined)).toBe("0:00");
  });
  it("summarises an album", () => {
    expect(albumSummary([tr(1, 1, 1500), tr(2, 1, 1320)])).toBe("2 songs · 47 min");
    expect(albumSummary([tr(1, 1, 0, true)])).toBe("1 song");
  });
  it("splits discs", () => {
    expect(byDisc([tr(1, 1, 1), tr(2, 1, 1), tr(3, 2, 1)]).map((d) => [d.disc, d.tracks.length])).toEqual([[1, 2], [2, 1]]);
  });
  it("builds cover URLs and nouns", () => {
    expect(coverUrl({ cover_version_id: 42 })).toBe("/api/versions/42/cover");
    expect(coverUrl({})).toBeUndefined();
    expect(itemNoun("music", 3)).toBe("albums");
  });
});
