import { describe, expect, it } from "vitest";
import { creditLines, creditsPlacement, creditsShowing, pictureBox } from "./components/MusicVideoCredits";
import { rematchSummary } from "./pages/LibraryPage";

describe("music video credits", () => {
  it("show from 0:01 for 10 seconds, and for the last 10 seconds", () => {
    expect(creditsShowing(0.5, 240)).toBe(false);
    expect(creditsShowing(1, 240)).toBe(true);
    expect(creditsShowing(10.9, 240)).toBe(true);
    expect(creditsShowing(11, 240)).toBe(false);
    expect(creditsShowing(120, 240)).toBe(false);
    expect(creditsShowing(229.9, 240)).toBe(false);
    expect(creditsShowing(230, 240)).toBe(true); // held to the end
    expect(creditsShowing(240, 240)).toBe(true);
    expect(creditsShowing(20, 24)).toBe(false); // too short for two showings: once
    expect(creditsShowing(6, NaN)).toBe(true);
  });
  it("sit in the picture's corner, not the player's", () => {
    // 4:3 video in a 16:9 player: black bars left and right.
    expect(pictureBox(1600, 900, 640, 480)).toEqual({ left: 200, top: 0, width: 1200, height: 900 });
    const pillar = creditsPlacement(1600, 900, 640, 480);
    expect(pillar.right).toBe(200 + 48); // the bar, then 4% of the picture
    expect(pillar.bottom).toBe(88); // the picture reaches the bottom: clear the controls
    expect(pillar.pictureWidth).toBe(1200);
    // A wide film in a tall player: bars above and below; the bar clears the controls.
    const letter = creditsPlacement(1000, 1000, 1920, 800);
    expect(letter.right).toBe(40);
    expect(letter.bottom).toBe(Math.round((1000 - 1000 * (800 / 1920)) / 2 + 40));
    // Before the video's size is known, the whole element is the picture.
    expect(creditsPlacement(800, 450, 0, 0)).toEqual({ right: 32, bottom: 88, pictureWidth: 800 });
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
