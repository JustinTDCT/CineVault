import { useCallback, useEffect, useState } from "react";
import { api, type Library } from "./api";
import { linkTo, navigate, useRoute } from "./router";
import { HomePage } from "./pages/HomePage";
import { LibraryPage } from "./pages/LibraryPage";
import { TitlePage } from "./pages/TitlePage";
import { SetupPage } from "./pages/SetupPage";
import { SettingsPage } from "./pages/SettingsPage";
import { AudioProvider } from "./components/AudioPlayer";
import { Icon } from "./components/Icon";

export function App() {
  const route = useRoute();
  const [libraries, setLibraries] = useState<Library[] | null>(null);
  const [error, setError] = useState("");

  const loadLibraries = useCallback(async () => {
    try {
      setLibraries(await api.libraries());
      setError("");
    } catch (e) {
      setError((e as Error).message);
    }
  }, []);

  useEffect(() => {
    loadLibraries();
  }, [loadLibraries]);

  // With no libraries yet, Home is setup.
  useEffect(() => {
    if (route.page === "home" && libraries !== null && libraries.length === 0) navigate("/setup", true);
  }, [route, libraries]);

  const currentLibraryId = route.page === "library" || route.page === "title" ? route.libraryId : null;
  const current = (yes: boolean) => (yes ? { "aria-current": "page" as const, className: "cv-nav-item is-current" } : { className: "cv-nav-item" });

  return (
    <AudioProvider>
      <div className="cv-app">
        <nav className="cv-sidebar" aria-label="Main">
          <a className="cv-brand" {...linkTo("/")}>
            CINE<span className="cv-wordmark-bar" aria-hidden="true"></span>VAULT
          </a>
          <a {...linkTo("/")} {...current(route.page === "home")}>
            <Icon name="home" />
            Home
          </a>
          {libraries?.length ? <div className="cv-nav-label">Libraries</div> : null}
          {libraries?.map((l) => (
            <a key={l.id} {...linkTo(`/libraries/${l.id}`)} {...current(l.id === currentLibraryId)}>
              <Icon name={l.media_type} />
              {l.name}
            </a>
          ))}
          <a {...linkTo("/setup")} {...current(route.page === "setup")}>
            <Icon name="plus" />
            <span className="cv-nav-add">Add library</span>
          </a>
          <div className="cv-sidebar__spacer" />
          <a {...linkTo("/settings")} {...current(route.page === "settings")}>
            <Icon name="settings" />
            Settings
          </a>
        </nav>

        <main className="cv-main">
          {error ? (
            <div className="content-error">
              <div className="cv-notice cv-notice--error" role="alert">
                <p>{error}</p>
                <button className="cv-btn" onClick={loadLibraries}>
                  Try again
                </button>
              </div>
            </div>
          ) : route.page === "home" ? (
            <HomePage libraries={libraries} />
          ) : route.page === "setup" ? (
            <div className="cv-page">
              <SetupPage
                hasLibraries={(libraries?.length ?? 0) > 0}
                onCreated={async (lib) => {
                  await loadLibraries();
                  navigate(`/libraries/${lib.id}?scan=1`);
                }}
              />
            </div>
          ) : route.page === "settings" ? (
            <div className="cv-page">
              <SettingsPage section={route.section} libraries={libraries} />
            </div>
          ) : route.page === "library" ? (
            <div className="cv-page">
              <LibraryPage key={route.libraryId} library={libraries?.find((l) => l.id === route.libraryId)} libraryId={route.libraryId} />
            </div>
          ) : route.page === "title" ? (
            <TitlePage key={route.titleId} libraryId={route.libraryId} titleId={route.titleId} />
          ) : route.page === "notFound" ? (
            <div className="cv-page">
              <div className="cv-notice">
                <p>There's no page at this address.</p>
                <a className="cv-btn" {...linkTo("/")}>
                  Go home
                </a>
              </div>
            </div>
          ) : null}

          <footer className="cv-footer">
            Movie and TV information and images from{" "}
            <a href="https://www.themoviedb.org" target="_blank" rel="noreferrer">
              TMDb
            </a>
            . This product uses the TMDB API but is not endorsed or certified by TMDB. Music video information from{" "}
            <a href="https://musicbrainz.org" target="_blank" rel="noreferrer">
              MusicBrainz
            </a>
            .
          </footer>
        </main>
      </div>
    </AudioProvider>
  );
}
