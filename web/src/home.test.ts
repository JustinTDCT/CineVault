import { describe, expect, it } from "vitest";
import type { Title, TitleProgress } from "./api";
import { continueTitles, isInProgress, progressCaption, progressFraction, recentlyAdded, spotlightTitles } from "./home";

const owned = [{ id: 1, edition_type_id: "theatrical", display_name: "Theatrical", short_label: "Theatrical", versions: [] }];

const title = (id: number, over: Partial<Title> = {}): Title =>
  ({
    id,
    library_id: 1,
    media_type: "movie",
    name: `Title ${id}`,
    sort_name: `title ${id}`,
    genres: [],
    match_status: "matched",
    added_at: "2026-11-01T00:00:00Z",
    editions: owned,
    ...over,
  }) as Title;

const at = (p: Partial<TitleProgress>): TitleProgress => ({ version_id: 9, position_seconds: 0, updated_at: "2026-11-20T00:00:00Z", ...p });

describe("home", () => {
  it("measures progress for each kind of title", () => {
    expect(progressFraction(title(1, { progress: at({ position_seconds: 3600, duration_seconds: 7200 }) }))).toBe(0.5);
    expect(progressFraction(title(1, { media_type: "book", progress: at({ position_seconds: 0.42 }) }))).toBe(0.42);
    expect(progressFraction(title(1, { media_type: "comic", progress: at({ position_seconds: 9, page_count: 20 }) }))).toBe(0.5);
    expect(progressFraction(title(1, { progress: at({ position_seconds: 30 }) }))).toBeUndefined(); // length unknown
    expect(progressFraction(title(1))).toBeUndefined();
  });

  it("continues only what's started and not finished", () => {
    expect(isInProgress(title(1, { progress: at({ position_seconds: 60, duration_seconds: 7200 }) }))).toBe(false); // barely started
    expect(isInProgress(title(1, { progress: at({ position_seconds: 7000, duration_seconds: 7200 }) }))).toBe(false); // credits
    expect(isInProgress(title(1, { progress: at({ position_seconds: 2460, duration_seconds: 7200 }) }))).toBe(true);
    expect(isInProgress(title(1, { media_type: "music", progress: at({ position_seconds: 60, duration_seconds: 200 }) }))).toBe(false);
    expect(isInProgress(title(1, { media_type: "comic", progress: at({ position_seconds: 0, page_count: 20 }) }))).toBe(false);
    expect(isInProgress(title(1, { media_type: "comic", progress: at({ position_seconds: 3, page_count: 20 }) }))).toBe(true);
  });

  it("lists the latest played first", () => {
    const ts = [
      title(1, { progress: at({ position_seconds: 2000, duration_seconds: 7200, updated_at: "2026-11-20T00:00:00Z" }) }),
      title(2, { progress: at({ position_seconds: 2000, duration_seconds: 7200, updated_at: "2026-11-25T00:00:00Z" }) }),
      title(3),
    ];
    expect(continueTitles(ts).map((t) => t.id)).toEqual([2, 1]);
  });

  it("lists the newest files first, owned titles only", () => {
    const ts = [
      title(1, { added_at: "2026-11-01T00:00:00Z" }),
      title(2, { added_at: "2026-11-26T00:00:00Z" }),
      title(3, { added_at: "2026-11-27T00:00:00Z", editions: [] }),
    ];
    expect(recentlyAdded(ts).map((t) => t.id)).toEqual([2, 1]);
  });

  it("captions progress", () => {
    expect(progressCaption(title(1, { progress: at({ position_seconds: 2460, duration_seconds: 7020 }) }))).toBe("1 h 16 min left");
    expect(
      progressCaption(title(1, { media_type: "tv", progress: at({ position_seconds: 600, duration_seconds: 3300, season_number: 1, episode_number: 3 }) })),
    ).toBe("S1 E3 · 45 min left");
    expect(progressCaption(title(1, { media_type: "book", progress: at({ position_seconds: 0.42 }) }))).toBe("42% read");
    expect(progressCaption(title(1, { media_type: "comic", progress: at({ position_seconds: 11, page_count: 30 }) }))).toBe("Page 12 of 30");
  });

  it("picks spotlight titles with backdrops, favouring several editions, and varies by day", () => {
    const two = [...owned, { ...owned[0], id: 2, edition_type_id: "directors_cut" }];
    const ts = [
      title(1, { backdrop_url: "/b1" }),
      title(2, { backdrop_url: "/b2", editions: two }),
      title(3), // no backdrop
      title(4, { backdrop_url: "/b4", media_type: "music" }),
    ];
    const picks = spotlightTitles(ts, new Date("2026-11-27T12:00:00Z"));
    expect(picks.map((t) => t.id)).toEqual([2, 1]);
    // The same day gives the same order.
    expect(spotlightTitles(ts, new Date("2026-11-27T20:00:00Z")).map((t) => t.id)).toEqual([2, 1]);
  });
});
