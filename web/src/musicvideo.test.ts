import { describe, expect, it } from "vitest";
import { creditLines, creditsShowing } from "./components/MusicVideoCredits";
import { rematchSummary } from "./pages/LibraryPage";

describe("music video credits", () => {
  it("show shortly after the start and again before the end", () => {
    expect(creditsShowing(2, 240)).toBe(false);
    expect(creditsShowing(5, 240)).toBe(true);
    expect(creditsShowing(12.9, 240)).toBe(true);
    expect(creditsShowing(13, 240)).toBe(false);
    expect(creditsShowing(120, 240)).toBe(false);
    expect(creditsShowing(225, 240)).toBe(true); // 15 s before the end, for 8 s
    expect(creditsShowing(233, 240)).toBe(false);
    expect(creditsShowing(50, 55)).toBe(false); // a short video shows them once
    expect(creditsShowing(6, NaN)).toBe(true);
  });
  it("write the lines MTV style", () => {
    expect(
      creditLines({ song: "Take On Me", artist: "a-ha", album: "Hunting High and Low", year: 1985, label: "Warner Bros.", director: "Steve Barron" }),
    ).toEqual({ artist: "a-ha", song: "“Take On Me”", rest: ["Hunting High and Low (1985)", "Warner Bros.", "Director: Steve Barron"] });
    expect(creditLines({ song: "Song", artist: "Band" }).rest).toEqual([]);
  });
});

describe("music video re-match summary", () => {
  it("says what matched and what didn't", () => {
    expect(rematchSummary({ matched: 7, not_found: 1, errors: 0, messages: [] })).toBe("Matched 7 to MusicBrainz, 1 not found.");
    expect(rematchSummary({ matched: 6, not_found: 0, errors: 2, messages: [] })).toBe(
      "Matched 6 to MusicBrainz, 2 failed (try again in a minute).",
    );
    expect(rematchSummary({ matched: 0, not_found: 0, errors: 0, messages: [], error: "boom" })).toBe("Matching stopped: boom");
  });
});
