import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
// The design system "CineVault", shared with the CineVault Cache (pkg/theme).
import "../../pkg/theme/fonts.css";
import "../../pkg/theme/tokens.css";
import "../../pkg/theme/cinevault.css";
import "./styles.css";
import { App } from "./App";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
