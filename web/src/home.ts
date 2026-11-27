import type { Title } from "./api";
import { episodeCode, formatRuntime, isUnowned, ownedEditions } from "./format";

/** How far through a title its saved progress is, from 0 to 1; undefined when unknown. */
export function progressFraction(t: Pick<Title, "media_type" | "progress">): number | undefined {
  const p = t.progress;
  if (!p) return undefined;
  if (t.media_type === "book") return Math.min(1, Math.max(0, p.position_seconds));
  if (t.media_type === "comic") {
    return p.page_count ? Math.min(1, (p.position_seconds + 1) / p.page_count) : undefined;
  }
  return p.duration_seconds ? Math.min(1, p.position_seconds / p.duration_seconds) : undefined;
}

/** Started and not finished: worth a place in "Continue". Albums don't count. */
export function isInProgress(t: Pick<Title, "media_type" | "progress">): boolean {
  if (t.media_type === "music") return false;
  const f = progressFraction(t);
  if (f === undefined) return false;
  if (t.media_type === "comic") return (t.progress?.position_seconds ?? 0) > 0 && f < 1;
  return f >= 0.02 && f < 0.95;
}

/** Titles to continue, most recently played first. */
export function continueTitles(titles: Title[], limit = 7): Title[] {
  return titles
    .filter(isInProgress)
    .sort((a, b) => (b.progress!.updated_at > a.progress!.updated_at ? 1 : -1))
    .slice(0, limit);
}

/** Titles whose newest file arrived most recently. */
export function recentlyAdded(titles: Title[], limit = 7): Title[] {
  return titles
    .filter((t) => ownedEditions(t).length > 0)
    .sort((a, b) => (b.added_at > a.added_at ? 1 : b.added_at < a.added_at ? -1 : b.id - a.id))
    .slice(0, limit);
}

/** What a "Continue" card says: "S1 E3 · 32 min left", "42% read", "Page 12 of 30". */
export function progressCaption(t: Pick<Title, "media_type" | "progress">): string {
  const p = t.progress;
  if (!p) return "";
  if (t.media_type === "book") return `${Math.round(p.position_seconds * 100)}% read`;
  if (t.media_type === "comic") return p.page_count ? `Page ${p.position_seconds + 1} of ${p.page_count}` : "";
  const left = p.duration_seconds ? Math.max(0, p.duration_seconds - p.position_seconds) : 0;
  const leftText = left >= 60 ? `${formatRuntime(left)} left` : left > 0 ? "Under a minute left" : "";
  const episode = p.episode_number ? episodeCode(p.season_number ?? 0, p.episode_number) : "";
  return [episode, leftText].filter(Boolean).join(" · ");
}

/** How many titles the spotlight cycles through. */
export const SPOTLIGHT_COUNT = 5;

/** Spotlight candidates: movies and shows with a backdrop, best first. A title gains
 *  for every edition, for a cut the CineVault Cache knows that the library lacks, for
 *  being new to the library, and for being half-watched; finished-looking titles don't
 *  lose. The order among equals changes each day, so Home doesn't always open on the
 *  same title. */
export function spotlightTitles(titles: Title[], now = new Date(), count = SPOTLIGHT_COUNT): Title[] {
  const day = Math.floor(now.getTime() / 86_400_000);
  const weekAgo = now.getTime() - 14 * 86_400_000;
  const score = (t: Title) =>
    ownedEditions(t).length * 2 +
    (t.editions.some(isUnowned) ? 2 : 0) +
    (Date.parse(t.added_at) > weekAgo ? 2 : 0) +
    (isInProgress(t) ? 1 : 0) +
    // A small, stable shuffle that changes daily.
    ((t.id * 2654435761 + day * 40503) % 1000) / 1000;
  return titles
    .filter((t) => (t.media_type === "movie" || t.media_type === "tv") && t.backdrop_url && ownedEditions(t).length > 0)
    .map((t) => ({ t, s: score(t) }))
    .sort((a, b) => b.s - a.s)
    .slice(0, count)
    .map((x) => x.t);
}
