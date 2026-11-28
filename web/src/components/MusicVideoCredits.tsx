import type { TitleCredits } from "../api";

/** When the credits show, in seconds: shortly after the start, and again
 *  before the end, as music channels did. A short video shows them once. */
export const CREDITS_AT = 5;
export const CREDITS_FOR = 8;
export const CREDITS_BEFORE_END = 15;

/** Whether the credits are on screen at a moment of the video. */
export function creditsShowing(t: number, duration: number): boolean {
  if (t >= CREDITS_AT && t < CREDITS_AT + CREDITS_FOR) return true;
  if (!isFinite(duration) || duration < 60) return false;
  const end = duration - CREDITS_BEFORE_END;
  return t >= end && t < end + CREDITS_FOR;
}

/** The lines of the corner credits: artist, "song", album (year), label, director. */
export function creditLines(c: TitleCredits): { artist: string; song: string; rest: string[] } {
  const album = c.album ? (c.year ? `${c.album} (${c.year})` : c.album) : c.year ? String(c.year) : "";
  return {
    artist: c.artist,
    song: `“${c.song}”`,
    rest: [album, c.label ?? "", c.director ? `Director: ${c.director}` : ""].filter(Boolean),
  };
}

/** The MTV-style credits in the corner of a playing music video. */
export function MusicVideoCredits({ credits, visible }: { credits: TitleCredits; visible: boolean }) {
  const l = creditLines(credits);
  return (
    <div className={`mv-credits${visible ? " is-visible" : ""}`} aria-hidden={!visible} role="note" aria-label="Song credits">
      <div className="mv-credits__artist">{l.artist}</div>
      <div className="mv-credits__song">{l.song}</div>
      {l.rest.map((line) => (
        <div key={line} className="mv-credits__line">
          {line}
        </div>
      ))}
    </div>
  );
}
