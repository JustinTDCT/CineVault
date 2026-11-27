import type { MediaType } from "../api";

// Line icons in the design system's style: 24px grid, 2px stroke, currentColor.

const paths: Record<string, string> = {
  home: "M3 11l9-7 9 7v9h-6v-6H9v6H3z",
  movie: "M3 5h18v14H3zM7 5v14M17 5v14",
  tv: "M3 6h18v12H3zM8 21h8",
  music: "M11 18V5l9-2v13M8 21a3 3 0 100-6 3 3 0 000 6zM17 19a3 3 0 100-6 3 3 0 000 6z",
  musicvideo: "M4 6h12v12H4zM16 10l5-3v10l-5-3",
  audiobook: "M4 14a8 8 0 0116 0v4h-4v-5h4M4 13h4v5H4z",
  book: "M4 5a2 2 0 012-2h13v16H6a2 2 0 00-2 2zM4 5v16",
  comic: "M4 4h16v16H4zM4 12h16M12 4v8",
  settings: "M12 15a3 3 0 100-6 3 3 0 000 6zM12 2v3M12 19v3M2 12h3M19 12h3M5 5l2 2M17 17l2 2M5 19l2-2M17 7l2-2",
  plus: "M12 5v14M5 12h14",
  play: "M7 4l13 8-13 8z",
};

export function Icon({ name, filled = false }: { name: MediaType | keyof typeof paths; filled?: boolean }) {
  return (
    <svg
      viewBox="0 0 24 24"
      width="18"
      height="18"
      fill={filled ? "currentColor" : "none"}
      stroke={filled ? "none" : "currentColor"}
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d={paths[name] ?? paths.movie} />
    </svg>
  );
}
