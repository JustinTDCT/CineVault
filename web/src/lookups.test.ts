import { describe, expect, it } from "vitest";
import type { Title } from "./api";
import { canRematch, editableFields, fieldLabel, matchSource } from "./format";
import { editFromDraft } from "./components/TitleEditor";
import { artCandidates } from "./components/PosterCard";

const album = (over: Partial<Title> = {}): Title => ({
  id: 4,
  library_id: 2,
  media_type: "music",
  name: "The Bridge",
  sort_name: "bridge",
  artist: "Ace of Base",
  year: 1995,
  genres: ["Pop"],
  match_status: "matched",
  added_at: "",
  editions: [],
  locked_fields: [],
  ...over,
});

describe("lookups", () => {
  it("name each library's source", () => {
    expect(matchSource("music")).toBe("MusicBrainz");
    expect(matchSource("audiobook")).toBe("Open Library");
    expect(matchSource("comic")).toBe("Comic Vine");
    expect(matchSource("tv")).toBe("TMDb");
    expect(canRematch("book")).toBe(true);
    expect(canRematch("movie")).toBe(false);
  });
  it("label fields for the media type", () => {
    expect(fieldLabel("artist", "book")).toBe("Author");
    expect(fieldLabel("artist", "music")).toBe("Artist");
    expect(fieldLabel("name", "comic")).toBe("Series");
    expect(fieldLabel("series_index", "book")).toBe("Number in series");
    expect(editableFields("audiobook")).toContain("narrator");
    expect(editableFields("comic")).toContain("publisher");
  });
  it("send only what changed", () => {
    const t = album();
    expect(editFromDraft(t, { name: "The Bridge", artist: "Ace of Base", year: "1995", genres: "Pop", overview: "" })).toEqual({});
    expect(
      editFromDraft(t, { name: " The Bridge (Remastered) ", artist: "Ace of Base", year: "", genres: "Pop; Eurodance", overview: "Second album." }),
    ).toEqual({ name: "The Bridge (Remastered)", year: 0, genres: ["Pop", "Eurodance"], overview: "Second album." });
  });
  it("try the looked-up cover, then the file's", () => {
    expect(artCandidates(album({ poster_url: "https://coverartarchive.org/x", cover_version_id: 9 }))).toEqual([
      "https://coverartarchive.org/x",
      "/api/versions/9/cover",
    ]);
    expect(artCandidates(album({ cover_version_id: 9 }))).toEqual(["/api/versions/9/cover"]);
    expect(artCandidates(album({ media_type: "movie", poster_url: "p", cover_version_id: 9 }))).toEqual(["p"]);
  });
});
