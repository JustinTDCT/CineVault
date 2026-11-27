import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import type { Plugin } from "vite";

// foliate-js can open PDFs with pdf.js, which would add 13 MB to the server.
// PDFs open in the browser's own viewer instead, so its PDF module is replaced
// by a stub that's never called.
const skipFoliatePDF: Plugin = {
  name: "skip-foliate-pdf",
  enforce: "pre",
  resolveId(id, importer) {
    if (id === "./pdf.js" && importer?.includes("foliate-js")) return "\0foliate-pdf-stub";
    return null;
  },
  load(id) {
    if (id === "\0foliate-pdf-stub") return "export const makePDF = () => { throw new Error('PDFs open in the browser viewer') }";
    return null;
  },
};

// The production build goes into the Go server's embed folder
// (internal/webui/static). During `npm run dev`, API calls are proxied to
// the Go server on :8456.
export default defineConfig({
  plugins: [skipFoliatePDF, react()],
  build: {
    outDir: "../internal/webui/static",
    emptyOutDir: false, // keeps static/.gitkeep; `make web` cleans old files
  },
  server: {
    host: true,
    // The design system lives in ../pkg/theme, shared with the CineVault Cache.
    fs: { allow: [".", "../pkg/theme"] },
    proxy: { "/api": "http://localhost:8456" },
  },
});
