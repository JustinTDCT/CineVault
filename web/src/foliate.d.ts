// foliate-js ships plain JavaScript; these are the parts CineVault uses.
declare module "foliate-js/view.js";

interface FoliateTocItem {
  label: string;
  href: string;
  subitems?: FoliateTocItem[];
}

interface FoliateLocation {
  fraction: number;
  cfi: string;
  tocItem?: { label?: string; href?: string };
}

interface FoliateRenderer extends HTMLElement {
  setStyles?(css: string): void;
  next(): Promise<void>;
  prev(): Promise<void>;
}

interface FoliateView extends HTMLElement {
  book: { toc?: FoliateTocItem[]; dir?: string; metadata?: { title?: unknown } };
  renderer: FoliateRenderer;
  lastLocation: FoliateLocation | null;
  open(book: File | Blob): Promise<void>;
  init(opts: { lastLocation?: string; showTextStart?: boolean }): Promise<void>;
  goTo(target: string | number): Promise<void>;
  goToFraction(f: number): Promise<void>;
  goLeft(): Promise<void>;
  goRight(): Promise<void>;
  close(): void;
}
