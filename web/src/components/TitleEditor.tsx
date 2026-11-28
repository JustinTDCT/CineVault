import { useState, type FormEvent } from "react";
import { api, type Title, type TitleEdit } from "../api";
import { editableFields, fieldLabel, matchSource } from "../format";

/** The form's values as text, one per editable field. */
type Draft = Record<string, string>;

function draftOf(t: Title): Draft {
  return {
    name: t.name,
    artist: t.artist ?? "",
    year: t.year ? String(t.year) : "",
    overview: t.overview ?? "",
    genres: (t.genres ?? []).join(", "),
    series: t.series ?? "",
    series_index: t.series_index ? String(t.series_index) : "",
    narrator: t.narrator ?? "",
    publisher: t.publisher ?? "",
  };
}

/** The changes between the title and the form, as an edit to send. */
export function editFromDraft(t: Title, d: Draft): TitleEdit {
  const before = draftOf(t);
  const e: TitleEdit = {};
  for (const f of editableFields(t.media_type)) {
    const v = (d[f] ?? "").trim();
    if (v === before[f].trim()) continue;
    switch (f) {
      case "year":
        e.year = v ? Number.parseInt(v, 10) || 0 : 0;
        break;
      case "series_index":
        e.series_index = v ? Number.parseFloat(v) || 0 : 0;
        break;
      case "genres":
        e.genres = v
          .split(/[,;]/)
          .map((g) => g.trim())
          .filter(Boolean);
        break;
      default:
        (e as Record<string, string>)[f] = v;
    }
  }
  return e;
}

/** "Edit details" and the title's locks, under its description. */
export function TitleEditor({ title, onChanged }: { title: Title; onChanged: () => void }) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<Draft>(() => draftOf(title));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const fields = editableFields(title.media_type);
  const source = matchSource(title.media_type);

  async function save(ev: FormEvent) {
    ev.preventDefault();
    const e = editFromDraft(title, draft);
    if (Object.keys(e).length === 0) {
      setEditing(false);
      return;
    }
    if (e.name !== undefined && !e.name) {
      setError("A title needs a name.");
      return;
    }
    setBusy(true);
    setError("");
    try {
      await api.editTitle(title.id, e);
      setEditing(false);
      onChanged();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  async function setLocks(locked: boolean, fieldsLocked: string[]) {
    setBusy(true);
    setError("");
    try {
      await api.setLocks(title.id, { locked, fields: fieldsLocked });
      onChanged();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  const locked = !!title.locked;
  const lockedFields = title.locked_fields ?? [];

  if (editing) {
    return (
      <form className="title-editor" onSubmit={save}>
        <p className="cv-caption">
          Fields you change are kept as you set them: {source ? `${source} lookups` : "lookups"} and the CineVault Cache won't change them.
        </p>
        {fields.map((f) => (
          <label key={f} className={`title-editor__field${f === "overview" ? " is-wide" : ""}`}>
            <span>{fieldLabel(f, title.media_type)}</span>
            {f === "overview" ? (
              <textarea rows={5} value={draft[f]} onChange={(e) => setDraft({ ...draft, [f]: e.target.value })} />
            ) : (
              <input
                value={draft[f]}
                inputMode={f === "year" || f === "series_index" ? "decimal" : undefined}
                placeholder={f === "genres" ? "Rock, Pop" : undefined}
                onChange={(e) => setDraft({ ...draft, [f]: e.target.value })}
              />
            )}
          </label>
        ))}
        {error ? <p className="cv-hint is-problem">{error}</p> : null}
        <div className="cv-actions">
          <button className="cv-btn cv-btn--primary" type="submit" disabled={busy}>
            {busy ? "Saving…" : "Save"}
          </button>
          <button
            className="cv-btn"
            type="button"
            onClick={() => {
              setDraft(draftOf(title));
              setEditing(false);
              setError("");
            }}
          >
            Cancel
          </button>
        </div>
      </form>
    );
  }

  return (
    <div className="title-locks">
      <button
        className="cv-btn cv-btn--small"
        onClick={() => {
          setDraft(draftOf(title));
          setEditing(true);
        }}
      >
        Edit details
      </button>
      <button
        className={`cv-btn cv-btn--small${locked ? " is-active" : ""}`}
        aria-pressed={locked}
        disabled={busy}
        onClick={() => setLocks(!locked, lockedFields)}
        title={locked ? "Let lookups and the CineVault Cache update this title again" : "Stop lookups and the CineVault Cache from changing this title"}
      >
        {locked ? "Locked" : "Lock"}
      </button>
      {locked ? <span className="cv-caption">Nothing changes this title automatically.</span> : null}
      {!locked && lockedFields.length ? (
        <span className="cv-caption">
          Kept as edited:{" "}
          {lockedFields.map((f, i) => (
            <span key={f}>
              {i ? ", " : ""}
              {fieldLabel(f, title.media_type)}{" "}
              <button
                type="button"
                className="link-button"
                disabled={busy}
                onClick={() => setLocks(false, lockedFields.filter((x) => x !== f))}
                title={`Let the next lookup change the ${fieldLabel(f, title.media_type).toLowerCase()}`}
              >
                (unlock)
              </button>
            </span>
          ))}
        </span>
      ) : null}
      {error ? <span className="cv-hint is-problem">{error}</span> : null}
    </div>
  );
}
