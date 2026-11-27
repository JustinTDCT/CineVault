import type { Edition, Title, Version } from "./api";

/** How many edition labels a card shows before "Show all" (Editions spec §7). */
export const CARD_EDITION_LIMIT = 3;

/** Splits a title's editions into the labels shown first and how many are hidden. */
export function editionListing(editions: Edition[], expanded: boolean) {
  if (expanded || editions.length <= CARD_EDITION_LIMIT) {
    return { shown: editions, hidden: 0 };
  }
  return { shown: editions.slice(0, CARD_EDITION_LIMIT), hidden: editions.length - CARD_EDITION_LIMIT };
}

/** "1 h 56 min" from seconds; "" when unknown. */
export function formatRuntime(seconds?: number): string {
  if (!seconds || seconds <= 0) return "";
  const minutes = Math.round(seconds / 60);
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  if (h === 0) return `${m} min`;
  return m === 0 ? `${h} h` : `${h} h ${m} min`;
}

/** "1.8 GB" from bytes. */
export function formatSize(bytes: number): string {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let i = 0;
  while (value >= 1000 && i < units.length - 1) {
    value /= 1000;
    i++;
  }
  return `${value >= 100 || i === 0 ? Math.round(value) : value.toFixed(1)} ${units[i]}`;
}

/** "4K", "1080p", "720p", "SD", or "" — same rules as internal/mediainfo. */
export function resolution(v: Pick<Version, "width" | "height">): string {
  const w = v.width ?? 0;
  const h = v.height ?? 0;
  if (w === 0 && h === 0) return "";
  if (w >= 3200 || h >= 1800) return "4K";
  if (w >= 1800 || h >= 1000) return "1080p";
  if (w >= 1200 || h >= 700) return "720p";
  return "SD";
}

const RES_RANK: Record<string, number> = { "4K": 4, "1080p": 3, "720p": 2, SD: 1, "": 0 };

/** The best resolution across all of a title's files, for the poster badge. */
export function bestResolution(t: Title): string {
  let best = "";
  for (const e of t.editions) {
    for (const v of e.versions) {
      const r = resolution(v);
      if (RES_RANK[r] > RES_RANK[best]) best = r;
    }
  }
  return best;
}

/** True when any file of the title has HDR or Dolby Vision. */
export function hasHDR(t: Title): boolean {
  return t.editions.some((e) => e.versions.some((v) => !!v.hdr));
}

/** The poster badge, e.g. "2 editions · 4K" (Editions spec §7). Empty when there's nothing to say. */
export function posterBadge(t: Title): string {
  const parts: string[] = [];
  if (t.editions.length > 1) parts.push(`${t.editions.length} editions`);
  const res = bestResolution(t);
  if (res === "4K") parts.push(hasHDR(t) ? "4K HDR" : "4K");
  return parts.join(" · ");
}

/** The edition to open first on a details page: the first with a file present. */
export function defaultEdition(t: Title): Edition | undefined {
  return t.editions.find((e) => e.versions.some((v) => !v.missing)) ?? t.editions[0];
}

/** File name from a path. */
export function fileName(path: string): string {
  const i = path.lastIndexOf("/");
  return i >= 0 ? path.slice(i + 1) : path;
}

/** The version label without its leading resolution, which is shown separately. */
export function labelWithoutResolution(label: string | undefined, res: string): string {
  if (!label) return "";
  const parts = label.split(" · ");
  return parts[0] === res ? parts.slice(1).join(" · ") : label;
}
