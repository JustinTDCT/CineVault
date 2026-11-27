import { describe, expect, it } from "vitest";
import type { Title } from "./api";
import { groupByArtist, itemNoun, thumbUrl } from "./format";

describe("music videos", () => {
  it("groups by artist, ignoring case", () => {
    const g = groupByArtist([{ artist: "311" }, { artist: "311" }, { artist: "Ace of Base" }, { artist: "ace of base" }, {}]);
    expect(g.map((x) => [x.artist, x.titles.length])).toEqual([["311", 2], ["Ace of Base", 2], ["Unknown artist", 1]]);
  });
  it("takes the thumbnail from the first file that's present", () => {
    const t = { editions: [{ versions: [{ id: 4, missing: true }, { id: 5, missing: false }] }] } as unknown as Title;
    expect(thumbUrl(t)).toBe("/api/versions/5/thumb");
    expect(thumbUrl({ editions: [] })).toBeUndefined();
  });
  it("names library items", () => {
    expect(itemNoun("musicvideo", 8)).toBe("music videos");
    expect(itemNoun("tv", 1)).toBe("show");
    expect(itemNoun("movie", 2)).toBe("movies");
  });
});
