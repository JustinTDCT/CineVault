import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The production build goes into the Go server's embed folder
// (internal/webui/static). During `npm run dev`, API calls are proxied to
// the Go server on :8456.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "../internal/webui/static",
    emptyOutDir: false, // keeps static/.gitkeep; `make web` cleans old files
  },
  server: {
    host: true,
    proxy: { "/api": "http://localhost:8456" },
  },
});
