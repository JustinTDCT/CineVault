import { describe, expect, it } from "vitest";
import type { Episode, Season } from "./api";
import { isPlaceholderName, isUpcoming, episodeCode, nextEpisode, playableEpisodes, seasonLabel, showCounts } from "./format";

const ep = (id: number, season: number, number: number, files: { missing?: boolean }[] = [{}]): Episode => ({
  id,
  season,
  number,
  versions: files.map((f, i) => ({ id: id * 10 + i, path: "/x", size_bytes: 1, edition_locked: false, missing: !!f.missing })),
});

const seasons: Season[] = [
  { number: 1, episodes: [ep(1, 1, 1), ep(2, 1, 2, []), ep(3, 1, 3, [{ missing: true }]), ep(4, 1, 4)] },
  { number: 2, episodes: [ep(5, 2, 1)] },
  { number: 0, episodes: [ep(6, 0, 1)] },
];

describe("tv", () => {
  it("labels seasons", () => {
    expect(seasonLabel({ number: 1, name: "Season 1" })).toBe("Season 1");
    expect(seasonLabel({ number: 0, name: "Specials" })).toBe("Specials");
    expect(seasonLabel({ number: 2, name: "Book Two: Fire" })).toBe("Book Two: Fire");
    expect(seasonLabel({ number: 3 })).toBe("Season 3");
  });
  it("codes episodes", () => {
    expect(episodeCode(1, 3)).toBe("S1 E3");
    expect(episodeCode(1, 1, 2)).toBe("S1 E1–E2");
    expect(episodeCode(0, 2)).toBe("Special 2");
  });
  it("skips episodes without a playable file for Up Next", () => {
    expect(playableEpisodes(seasons).map((e) => e.id)).toEqual([1, 4, 5, 6]);
    expect(nextEpisode(seasons, 1)?.id).toBe(4);
    expect(nextEpisode(seasons, 4)?.id).toBe(5); // into the next season
    expect(nextEpisode(seasons, 6)).toBeUndefined();
  });
  it("counts a show's seasons and episodes", () => {
    expect(showCounts({ season_count: 1, episode_count: 8 })).toBe("1 season · 8 episodes");
    expect(showCounts({ season_count: 2, episode_count: 1 })).toBe("2 seasons · 1 episode");
    expect(showCounts({})).toBe("");
  });
  it("spots episodes that haven't aired and placeholder names", () => {
    const today = new Date("2026-11-26T12:00:00");
    expect(isUpcoming("2027-01-14", today)).toBe(true);
    expect(isUpcoming("2024-03-21", today)).toBe(false);
    expect(isUpcoming(undefined, today)).toBe(false);
    expect(isPlaceholderName("Episode 3", 3)).toBe(true);
    expect(isPlaceholderName("Countdown", 1)).toBe(false);
  });
});
