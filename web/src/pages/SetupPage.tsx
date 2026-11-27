import { useState, type FormEvent } from "react";
import { api, type Library } from "../api";

export function SetupPage({ hasLibraries, onCreated }: { hasLibraries: boolean; onCreated: (lib: Library) => void }) {
  const [name, setName] = useState("");
  const [folder, setFolder] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      onCreated(await api.createLibrary(name.trim(), "movie", [folder.trim()]));
    } catch (err) {
      setError((err as Error).message);
      setBusy(false);
    }
  }

  return (
    <section className="setup">
      <h1 className="page-title">{hasLibraries ? "Add a library" : "Add your movies"}</h1>
      <p className="page-sub">
        Point CineVault at a folder of movie files on this server. It reads each file, works out the title, year and edition from the file name,
        and fetches posters and details from TMDb.
      </p>
      <form className="form" onSubmit={submit}>
        <label htmlFor="lib-name">Library name</label>
        <input id="lib-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="Movies" required autoComplete="off" />
        <label htmlFor="lib-folder">Folder on the server</label>
        <input
          id="lib-folder"
          value={folder}
          onChange={(e) => setFolder(e.target.value)}
          placeholder="/media/movies"
          required
          autoComplete="off"
          spellCheck={false}
        />
        <p className="hint">Use the full path, starting with /. Only movie libraries are supported for now.</p>
        {error ? (
          <p className="form-error" role="alert">
            {error}
          </p>
        ) : null}
        <button className="btn btn-primary" type="submit" disabled={busy}>
          {busy ? "Adding…" : "Add and scan"}
        </button>
      </form>
    </section>
  );
}
