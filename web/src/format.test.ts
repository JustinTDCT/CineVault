import { describe, expect, it } from "vitest";
import type { Edition, Title } from "./api";
import { editionListing, formatRuntime, formatSize, labelWithoutResolution, posterBadge, resolution, defaultEdition } from "./format";

const ed = (id: number, label: string, versions: Partial<Edition["versions"][number]>[] = []): Edition => ({
  id,
  edition_type_id: label.toLowerCase(),
  display_name: label,
  short_label: label,
  versions: versions.map((v, i) => ({ id: id * 10 + i, path: "/x", size_bytes: 1, edition_locked: false, missing: false, ...v })),
});

const title = (editions: Edition[]): Title => ({
  id: 1, library_id: 1, media_type: "movie", name: "Alien", sort_name: "alien", genres: [], match_status: "matched", added_at: "2026-11-01T00:00:00Z", editions,
});

describe("editionListing", () => {
  const five = [1, 2, 3, 4, 5].map((i) => ed(i, `E${i}`));
  it("shows everything when there are three or fewer", () => {
    expect(editionListing(five.slice(0, 3), false)).toEqual({ shown: five.slice(0, 3), hidden: 0 });
  });
  it("shows three and counts the rest", () => {
    const r = editionListing(five, false);
    expect(r.shown.map((e) => e.short_label)).toEqual(["E1", "E2", "E3"]);
    expect(r.hidden).toBe(2);
  });
  it("shows all when expanded", () => {
    expect(editionListing(five, true).shown).toHaveLength(5);
  });
});

describe("formatting", () => {
  it("formats runtimes", () => {
    expect(formatRuntime(6998)).toBe("1 h 57 min");
    expect(formatRuntime(7200)).toBe("2 h");
    expect(formatRuntime(1500)).toBe("25 min");
    expect(formatRuntime(undefined)).toBe("");
  });
  it("formats sizes", () => {
    expect(formatSize(1948726885)).toBe("1.9 GB");
    expect(formatSize(1004484377)).toBe("1.0 GB");
    expect(formatSize(512)).toBe("512 B");
    expect(formatSize(158_000_000_000)).toBe("158 GB");
  });
  it("labels resolution like the server", () => {
    expect(resolution({ width: 1920, height: 816 })).toBe("1080p");
    expect(resolution({ width: 3840, height: 1600 })).toBe("4K");
    expect(resolution({})).toBe("");
  });
});

describe("posterBadge", () => {
  it("counts editions for Alien", () => {
    expect(posterBadge(title([ed(1, "DC", [{ width: 1920, height: 816 }]), ed(2, "Remastered", [{ width: 1920, height: 800 }])]))).toBe("2 editions");
  });
  it("adds 4K HDR", () => {
    expect(posterBadge(title([ed(1, "Theatrical", [{ width: 3840, height: 2160, hdr: "hdr10" }])]))).toBe("4K HDR");
  });
  it("is empty for a single 1080p edition", () => {
    expect(posterBadge(title([ed(1, "Theatrical", [{ width: 1920, height: 1080 }])]))).toBe("");
  });
});

describe("defaultEdition", () => {
  it("skips editions whose files are all missing", () => {
    const t = title([ed(1, "DC", [{ missing: true }]), ed(2, "Remastered", [{}])]);
    expect(defaultEdition(t)?.id).toBe(2);
  });
});

describe("labelWithoutResolution", () => {
  it("drops the leading resolution", () => {
    expect(labelWithoutResolution("1080p · HEVC · AAC 2.0", "1080p")).toBe("HEVC · AAC 2.0");
    expect(labelWithoutResolution("HEVC · AAC 2.0", "1080p")).toBe("HEVC · AAC 2.0");
    expect(labelWithoutResolution(undefined, "")).toBe("");
  });
});
