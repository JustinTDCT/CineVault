import { useEffect, useState, type FormEvent } from "react";
import { api, type CacheSettings, type Library, type MediaType } from "../api";

export const KINDS: { type: MediaType; label: string; name: string; folder: string; naming: string }[] = [
  { type: "movie", label: "Movies", name: "Movies", folder: "/media/movies", naming: "Alien (1979).mkv" },
  { type: "tv", label: "TV shows", name: "TV Shows", folder: "/media/tv", naming: "3 Body Problem/Season 1/3 Body Problem - S01E01.mkv" },
  { type: "music", label: "Music", name: "Music", folder: "/media/music", naming: "3 Doors Down/Away From the Sun (2002)/01 - When I'm Gone.flac" },
  { type: "audiobook", label: "Audiobooks", name: "Audiobooks", folder: "/media/audiobooks", naming: "Author/Series/Book Title.m4b" },
  { type: "book", label: "Ebooks", name: "Ebooks", folder: "/media/ebooks", naming: "Author/Author - [Series 01] - Title.epub" },
  { type: "comic", label: "Comics", name: "Comics", folder: "/media/comics", naming: "Aliens - Book One/Aliens - Book One - 1.cbr" },
  { type: "musicvideo", label: "Music videos", name: "Music Videos", folder: "/media/musicvideos", naming: "Alice In Chains - No Excuses (MTV Unplugged).mp4" },
];

export function SetupPage({ hasLibraries, onCreated }: { hasLibraries: boolean; onCreated: (lib: Library) => void }) {
  const [kind, setKind] = useState<MediaType>("movie");
  const [name, setName] = useState("");
  const [folder, setFolder] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  // The first library is when to mention sharing with the CineVault Cache.
  const [cache, setCache] = useState<CacheSettings | null>(null);
  const [share, setShare] = useState(true);

  useEffect(() => {
    if (hasLibraries) return;
    api
      .cache()
      .then((s) => {
        setCache(s.settings);
        setShare(s.settings.share);
      })
      .catch(() => setCache(null));
  }, [hasLibraries]);

  const current = KINDS.find((k) => k.type === kind)!;

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      if (cache && share !== cache.share) await api.saveCacheSettings({ ...cache, share });
      onCreated(await api.createLibrary(name.trim(), kind, [folder.trim()]));
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  return (
    <section className="setup">
      <h1 className="cv-page-title">{hasLibraries ? "Add a library" : "Add your first library"}</h1>
      <p className="cv-sub">
        Point CineVault at a folder of video files on this server. It works out each title from the file and folder names, and fetches posters and
        details from TMDb.
      </p>
      <form className="form" onSubmit={submit}>
        <fieldset className="kind-choice">
          <legend>What's in the folder</legend>
          {KINDS.map((k) => (
            <label key={k.type} className="kind-option">
              <input type="radio" name="kind" value={k.type} checked={kind === k.type} onChange={() => setKind(k.type)} />
              {k.label}
            </label>
          ))}
        </fieldset>
        <label htmlFor="lib-name">Library name</label>
        <input id="lib-name" value={name} onChange={(e) => setName(e.target.value)} placeholder={current.name} required autoComplete="off" />
        <label htmlFor="lib-folder">Folder on the server</label>
        <input
          id="lib-folder"
          value={folder}
          onChange={(e) => setFolder(e.target.value)}
          placeholder={current.folder}
          required
          autoComplete="off"
          spellCheck={false}
        />
        <p className="cv-hint">
          Use the full path, starting with /. Files named like <code>{current.naming}</code> work best.
        </p>
        {cache && (kind === "movie" || kind === "tv") ? (
          <label className="settings-toggle setup-share">
            <input type="checkbox" checked={share} onChange={(e) => setShare(e.target.checked)} />
            <span>
              <span className="settings-label">Help build the CineVault Cache</span>
              <span className="settings-note">
                Once a day, CineVault sends which movies and shows you have, their editions and runtimes, so the cache can work out every edition of
                them. No file names, folders or anything that identifies you. You can change this in Settings.
              </span>
            </span>
          </label>
        ) : null}
        {error ? (
          <p className="form-error" role="alert">
            {error}
          </p>
        ) : null}
        <button className="cv-btn cv-btn--primary" type="submit" disabled={busy}>
          {busy ? "Adding…" : "Add and scan"}
        </button>
      </form>
    </section>
  );
}
