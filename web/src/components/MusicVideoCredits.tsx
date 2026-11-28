import type { TitleCredits } from "../api";

import type { CSSProperties } from "react";

/** When the credits show, in seconds, as music channels did: from 0:01 for
 *  10 seconds, and again for the last 10 seconds, held to the end. A video
 *  too short for two separate showings shows them once. */
export const CREDITS_AT = 1;
export const CREDITS_FOR = 10;
export const CREDITS_LAST = 10;

/** Whether the credits are on screen at a moment of the video. */
export function creditsShowing(t: number, duration: number): boolean {
  if (t >= CREDITS_AT && t < CREDITS_AT + CREDITS_FOR) return true;
  if (!isFinite(duration) || duration < CREDITS_AT + CREDITS_FOR + CREDITS_LAST + 5) return false;
  return t >= duration - CREDITS_LAST;
}

/** Where the picture sits inside a <video> element that letterboxes or
 *  pillarboxes it (object-fit: contain), in the element's pixels. */
export function pictureBox(elW: number, elH: number, vidW: number, vidH: number) {
  if (!(elW > 0 && elH > 0 && vidW > 0 && vidH > 0)) return { left: 0, top: 0, width: elW, height: elH };
  const scale = Math.min(elW / vidW, elH / vidH);
  const width = vidW * scale;
  const height = vidH * scale;
  return { left: (elW - width) / 2, top: (elH - height) / 2, width, height };
}

/** Space kept clear at the bottom of the element for the player's controls. */
export const CONTROLS_CLEARANCE = 88;

/** Places the credits in the picture's own bottom-right corner, not the
 *  player's: offsets from the element's right and bottom edges, and the
 *  picture's width, which sets the type size. */
export function creditsPlacement(elW: number, elH: number, vidW: number, vidH: number) {
  const p = pictureBox(elW, elH, vidW, vidH);
  const margin = Math.max(16, p.width * 0.04);
  const gapRight = elW - (p.left + p.width);
  const gapBottom = elH - (p.top + p.height);
  return {
    right: Math.round(gapRight + margin),
    bottom: Math.round(Math.max(gapBottom + margin, CONTROLS_CLEARANCE)),
    pictureWidth: Math.round(p.width),
  };
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
export function MusicVideoCredits({
  credits,
  visible,
  place,
}: {
  credits: TitleCredits;
  visible: boolean;
  place?: ReturnType<typeof creditsPlacement>;
}) {
  const l = creditLines(credits);
  const style = place
    ? ({ right: place.right, bottom: place.bottom, "--mv-w": `${place.pictureWidth}px` } as CSSProperties)
    : undefined;
  return (
    <div
      className={`mv-credits${visible ? " is-visible" : ""}`}
      style={style}
      aria-hidden={!visible}
      role="note"
      aria-label="Song credits"
    >
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
