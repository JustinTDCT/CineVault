import { useEffect, useState, type MouseEvent } from "react";

// A tiny history-based router: the app has only a handful of pages.

export type Route =
  | { page: "home" }
  | { page: "setup" }
  | { page: "settings"; section: SettingsSection }
  | { page: "library"; libraryId: number }
  | { page: "title"; libraryId: number; titleId: number }
  | { page: "notFound" };

/** The groups of Settings, each its own page (as in Plex and Jellyfin). */
export const SETTINGS_SECTIONS = ["general", "libraries", "playback", "cache"] as const;
export type SettingsSection = (typeof SETTINGS_SECTIONS)[number];

export function parseRoute(path: string): Route {
  const parts = path.split("/").filter(Boolean);
  if (parts.length === 0) return { page: "home" };
  if (parts[0] === "setup" && parts.length === 1) return { page: "setup" };
  if (parts[0] === "settings" && parts.length === 1) return { page: "settings", section: "general" };
  if (parts[0] === "settings" && parts.length === 2 && (SETTINGS_SECTIONS as readonly string[]).includes(parts[1])) {
    return { page: "settings", section: parts[1] as SettingsSection };
  }
  if (parts[0] === "libraries") {
    const libraryId = Number(parts[1]);
    if (!Number.isInteger(libraryId) || libraryId <= 0) return { page: "notFound" };
    if (parts.length === 2) return { page: "library", libraryId };
    const titleId = Number(parts[3]);
    if (parts.length === 4 && parts[2] === "titles" && Number.isInteger(titleId) && titleId > 0) {
      return { page: "title", libraryId, titleId };
    }
  }
  return { page: "notFound" };
}

export function navigate(path: string, replace = false) {
  if (replace) history.replaceState(null, "", path);
  else history.pushState(null, "", path);
  window.dispatchEvent(new PopStateEvent("popstate"));
  window.scrollTo(0, 0);
}

export function useRoute(): Route {
  const [route, setRoute] = useState(() => parseRoute(location.pathname));
  useEffect(() => {
    const onPop = () => setRoute(parseRoute(location.pathname));
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);
  return route;
}

/** Click handler for in-app links: normal clicks navigate without reloading. */
export function linkTo(path: string) {
  return {
    href: path,
    onClick: (e: MouseEvent) => {
      if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
      e.preventDefault();
      navigate(path);
    },
  };
}
