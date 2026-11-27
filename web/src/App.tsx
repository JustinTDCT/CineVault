import { useCallback, useEffect, useState } from "react";
import { api, type Library } from "./api";
import { linkTo, navigate, useRoute } from "./router";
import { LibraryPage } from "./pages/LibraryPage";
import { TitlePage } from "./pages/TitlePage";
import { SetupPage } from "./pages/SetupPage";

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

  // The home page opens the first library, or setup when there are none.
  useEffect(() => {
    if (route.page !== "home" || libraries === null) return;
    navigate(libraries.length > 0 ? `/libraries/${libraries[0].id}` : "/setup", true);
  }, [route, libraries]);

  const currentLibraryId = route.page === "library" || route.page === "title" ? route.libraryId : null;

  return (
    <div className="shell">
      <header className="topbar">
        <a className="wordmark" {...linkTo("/")}>
          CineVault
        </a>
        <nav className="libnav" aria-label="Libraries">
          {libraries?.map((l) => (
            <a key={l.id} {...linkTo(`/libraries/${l.id}`)} aria-current={l.id === currentLibraryId ? "page" : undefined}>
              {l.name}
            </a>
          ))}
          <a {...linkTo("/setup")} aria-current={route.page === "setup" ? "page" : undefined} className="libnav-add">
            Add library
          </a>
        </nav>
      </header>

      <main className="content">
        {error ? (
          <div className="notice notice-error" role="alert">
            <p>{error}</p>
            <button className="btn" onClick={loadLibraries}>
              Try again
            </button>
          </div>
        ) : route.page === "setup" ? (
          <SetupPage
            hasLibraries={(libraries?.length ?? 0) > 0}
            onCreated={async (lib) => {
              await loadLibraries();
              navigate(`/libraries/${lib.id}?scan=1`);
            }}
          />
        ) : route.page === "library" ? (
          <LibraryPage key={route.libraryId} library={libraries?.find((l) => l.id === route.libraryId)} libraryId={route.libraryId} />
        ) : route.page === "title" ? (
          <TitlePage key={route.titleId} libraryId={route.libraryId} titleId={route.titleId} />
        ) : route.page === "notFound" ? (
          <div className="notice">
            <p>There's no page at this address.</p>
            <a className="btn" {...linkTo("/")}>
              Go to your library
            </a>
          </div>
        ) : null}
      </main>

      <footer className="footer">
        <span>
          Movie information and images from{" "}
          <a href="https://www.themoviedb.org" target="_blank" rel="noreferrer">
            TMDb
          </a>
          . This product uses the TMDB API but is not endorsed or certified by TMDB.
        </span>
      </footer>
    </div>
  );
}
