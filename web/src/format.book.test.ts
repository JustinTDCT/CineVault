import { describe, expect, it } from "vitest";
import { chapterAt, itemNoun, seriesLabel, timeLeft } from "./format";

const ch = [
  { start: 0, end: 10, name: "Opening Credits" },
  { start: 10, end: 25, name: "Chapter 1" },
  { start: 25, end: 40, name: "Chapter 2" },
];

describe("audiobooks", () => {
  it("finds the chapter playing", () => {
    expect(chapterAt(ch, 0)?.name).toBe("Opening Credits");
    expect(chapterAt(ch, 9.8)?.name).toBe("Chapter 1"); // within half a second of the next mark
    expect(chapterAt(ch, 30)?.name).toBe("Chapter 2");
    expect(chapterAt([], 5)).toBeUndefined();
  });
  it("labels series", () => {
    expect(seriesLabel({ series: "The John Matherson Series", series_index: 2 })).toBe("The John Matherson Series, book 2");
    expect(seriesLabel({ series: "Middle-earth" })).toBe("Middle-earth");
    expect(seriesLabel({})).toBe("");
  });
  it("says how much is left", () => {
    expect(timeLeft(36000, 7200)).toBe("8 h left");
    expect(timeLeft(100, 100)).toBe("");
    expect(itemNoun("audiobook", 4)).toBe("audiobooks");
  });
});
