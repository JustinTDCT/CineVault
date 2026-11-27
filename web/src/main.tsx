import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "@fontsource-variable/big-shoulders-display";
import "@fontsource/atkinson-hyperlegible-next/400.css";
import "@fontsource/atkinson-hyperlegible-next/400-italic.css";
import "@fontsource/atkinson-hyperlegible-next/600.css";
import "./styles.css";
import { App } from "./App";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
