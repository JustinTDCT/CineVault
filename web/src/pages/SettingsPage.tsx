import { useCallback, useEffect, useRef, useState, type FormEvent, type ReactNode } from "react";
import { api, type CacheSettings, type CacheStatus, type Health, type Library, type ScanResult } from "../api";
import { formatDate, itemNoun, timeAgo, canRematch, matchSource } from "../format";
import { Icon } from "../components/Icon";
import { linkTo, type SettingsSection } from "../router";
import { rematchSummary, scanSummary } from "./LibraryPage";
import { KINDS } from "./SetupPage";

/** The groups of Settings, listed down the side as in Plex and Jellyfin. */
const SECTIONS: { id: SettingsSection; label: string; icon: string; blurb: string }[] = [
  { id: "general", label: "General", icon: "server", blurb: "This server: version, database and metadata" },
  { id: "libraries", label: "Libraries", icon: "folders", blurb: "Folders, scanning and adding libraries" },
  { id: "playback", label: "Playback", icon: "playback", blurb: "Converting video for players that need it" },
  { id: "cache", label: "CineVault Cache", icon: "cache", blurb: "Edition details, your subscription and sharing" },
];

export function SettingsPage({ section, libraries }: { section: SettingsSection; libraries: Library[] | null }) {
  const current = SECTIONS.find((s) => s.id === section) ?? SECTIONS[0];
  // On phones the groups are a row that scrolls: keep the current one in view.
  const currentLink = useRef<HTMLAnchorElement>(null);
  useEffect(() => {
    currentLink.current?.scrollIntoView?.({ block: "nearest", inline: "nearest" });
  }, [current.id]);
  return (
    <section className="cv-settings" aria-labelledby="settings-title">
      <div className="cv-settings__head">
        <h1 id="settings-title" className="cv-page-title">
          Settings
        </h1>
      </div>
      <nav className="cv-settings-nav" aria-label="Settings">
        {SECTIONS.map((s) => (
          <a
            key={s.id}
            ref={s.id === current.id ? currentLink : undefined}
            className={`cv-settings-nav__item${s.id === current.id ? " is-current" : ""}`}
            aria-current={s.id === current.id ? "page" : undefined}
            {...linkTo(`/settings/${s.id}`)}
          >
            <Icon name={s.icon} />
            <span>{s.label}</span>
          </a>
        ))}
      </nav>
      <div className="cv-settings__panel">
        <h2 className="cv-section-title">{current.label}</h2>
        <p className="cv-caption settings-blurb">{current.blurb}</p>
        {current.id === "general" ? (
          <GeneralSettings />
        ) : current.id === "libraries" ? (
          <LibrarySettings libraries={libraries} />
        ) : current.id === "playback" ? (
          <PlaybackSettings />
        ) : (
          <CacheSettingsSection />
        )}
      </div>
    </section>
  );
}

/** A setting: name and explanation on the left, its control on the right. */
function Row({ label, note, children, problem }: { label: ReactNode; note?: ReactNode; children?: ReactNode; problem?: boolean }) {
  return (
    <div className="cv-setting">
      <div className="cv-setting__text">
        <div className="cv-setting__label">{label}</div>
        {note ? <div className={`cv-setting__note${problem ? " is-problem" : ""}`}>{note}</div> : null}
      </div>
      {children ? <div className="cv-setting__control">{children}</div> : null}
    </div>
  );
}

/** An on/off switch. */
function Switch({ on, onChange, label, disabled }: { on: boolean; onChange: (on: boolean) => void; label: string; disabled?: boolean }) {
  return (
    <button type="button" role="switch" aria-checked={on} aria-label={label} className="cv-switch" disabled={disabled} onClick={() => onChange(!on)}>
      <span className="cv-switch__thumb" />
    </button>
  );
}

function useHealth() {
  const [health, setHealth] = useState<Health | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    // Health answers 503 when degraded, with the same body; show it either way.
    fetch("/api/health")
      .then((r) => r.json())
      .then((h: Health) => setHealth(h))
      .catch(() => setError("Can't reach the CineVault server. Check that it's running."));
  }, []);
  return { health, error };
}

function GeneralSettings() {
  const { health, error } = useHealth();
  if (error) return <Notice text={error} />;
  if (!health) return <p className="cv-caption">Loading…</p>;
  return (
    <div className="cv-settings-group">
      <Row label="Version" note="The build this server is running.">
        <span className="cv-setting__value">{health.version}</span>
      </Row>
      <Row
        label="Database"
        note={health.database.reachable ? `PostgreSQL, schema version ${health.database.schema_version}.` : health.database.error}
        problem={!health.database.reachable}
      >
        <StatusChip ok={health.database.reachable} okText="Connected" badText="Not reachable" />
      </Row>
      <Row
        label="Movie and TV information"
        note={
          health.metadata.tmdb_configured
            ? "Titles are matched to TMDb for posters, descriptions and cast."
            : "No TMDb token: titles aren't matched. Set CINEVAULT_TMDB_TOKEN (in ~/.config/cinevault/tmdb.env) and restart."
        }
        problem={!health.metadata.tmdb_configured}
      >
        <StatusChip ok={health.metadata.tmdb_configured} okText="TMDb" badText="Not set up" />
      </Row>
      <Row label="Music, book and audiobook information" note="Albums are looked up on MusicBrainz (with covers from the Cover Art Archive), books and audiobooks on Open Library. Neither needs a key.">
        <StatusChip ok okText="On" badText="" />
      </Row>
      <Row
        label="Comic information"
        note={
          health.metadata.comicvine_configured
            ? "Comic series are looked up on Comic Vine for covers, publishers and descriptions."
            : "No Comic Vine key: comics keep what ComicInfo.xml and file names say. Get a free key at comicvine.gamespot.com/api, put CINEVAULT_COMICVINE_KEY in ~/.config/cinevault/comicvine.env and restart."
        }
      >
        <StatusChip ok={health.metadata.comicvine_configured} okText="Comic Vine" badText="Not set up" />
      </Row>
    </div>
  );
}

function StatusChip({ ok, okText, badText }: { ok: boolean; okText: string; badText: string }) {
  return <span className={`cv-chip ${ok ? "cv-chip--added" : "cv-chip--removed"}`}>{ok ? okText : badText}</span>;
}

function LibrarySettings({ libraries }: { libraries: Library[] | null }) {
  const [busy, setBusy] = useState<{ id: number; what: "scan" | "rematch" } | null>(null);
  const [results, setResults] = useState<Record<number, { text: string; problem?: boolean }>>({});
  const [preferLocal, setPreferLocal] = useState<Record<number, boolean>>({});
  if (!libraries) return <p className="cv-caption">Loading…</p>;

  async function setPrefer(lib: Library, on: boolean) {
    setPreferLocal((x) => ({ ...x, [lib.id]: on }));
    try {
      const saved = await api.updateLibrary(lib.id, { prefer_local_metadata: on });
      setPreferLocal((x) => ({ ...x, [lib.id]: saved.prefer_local_metadata }));
    } catch (e) {
      setPreferLocal((x) => ({ ...x, [lib.id]: !on }));
      setResults((x) => ({ ...x, [lib.id]: { text: (e as Error).message, problem: true } }));
    }
  }

  async function rematch(lib: Library) {
    setBusy({ id: lib.id, what: "rematch" });
    try {
      const r = await api.rematch(lib.id);
      setResults((x) => ({ ...x, [lib.id]: { text: rematchSummary(r, lib.media_type), problem: !!r.error } }));
    } catch (e) {
      setResults((x) => ({ ...x, [lib.id]: { text: (e as Error).message, problem: true } }));
    } finally {
      setBusy(null);
    }
  }

  async function scan(lib: Library) {
    setBusy({ id: lib.id, what: "scan" });
    try {
      const r: ScanResult = await api.scan(lib.id);
      setResults((x) => ({ ...x, [lib.id]: { text: scanSummary(r, lib.media_type) } }));
    } catch (e) {
      setResults((x) => ({ ...x, [lib.id]: { text: (e as Error).message, problem: true } }));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="cv-settings-group">
      {libraries.map((lib) => {
        const kind = KINDS.find((k) => k.type === lib.media_type);
        const res = results[lib.id];
        return [
          <Row
            key={lib.id}
            label={
              <span className="settings-lib-name">
                <Icon name={lib.media_type} /> {lib.name}
              </span>
            }
            note={
              <>
                {kind?.label ?? itemNoun(lib.media_type, 2)} · {lib.paths.join(", ")}
                {res ? <span className={`settings-result${res.problem ? " is-problem" : ""}`}>{res.text}</span> : null}
              </>
            }
          >
            <a className="cv-btn cv-btn--small" {...linkTo(`/libraries/${lib.id}`)}>
              Open
            </a>
            {canRematch(lib.media_type) ? (
              <button
                className="cv-btn cv-btn--small"
                onClick={() => rematch(lib)}
                disabled={busy !== null}
                title={`Look every title up on ${matchSource(lib.media_type)} again (locked titles are left alone)`}
              >
                {busy?.id === lib.id && busy.what === "rematch" ? "Matching…" : "Match again"}
              </button>
            ) : null}
            <button className="cv-btn cv-btn--small" onClick={() => scan(lib)} disabled={busy !== null}>
              {busy?.id === lib.id && busy.what === "scan" ? "Scanning…" : "Scan now"}
            </button>
          </Row>,
          <div key={`${lib.id}-local`} className="cv-setting settings-sub">
            <div className="cv-setting__text">
              <div className="cv-setting__label">Prefer what the files say</div>
              <div className="cv-setting__note">
                {(preferLocal[lib.id] ?? lib.prefer_local_metadata)
                  ? `Tags, embedded details and file names win${matchSource(lib.media_type) ? `; ${matchSource(lib.media_type)} only fills in what's missing` : ""}.`
                  : `${matchSource(lib.media_type) || "Looked-up details"} win${matchSource(lib.media_type) ? "s" : ""} over the files. Fields you edit by hand are always kept.`}
              </div>
            </div>
            <div className="cv-setting__control">
              <Switch
                on={preferLocal[lib.id] ?? lib.prefer_local_metadata}
                label={`Prefer what the files say in ${lib.name}`}
                onChange={(v) => setPrefer(lib, v)}
              />
            </div>
          </div>,
        ];
      })}
      <div className="cv-settings-footer">
        <a className="cv-btn" {...linkTo("/setup")}>
          <Icon name="plus" /> Add a library
        </a>
      </div>
    </div>
  );
}

function PlaybackSettings() {
  const { health, error } = useHealth();
  if (error) return <Notice text={error} />;
  if (!health) return <p className="cv-caption">Loading…</p>;
  const pb = health.playback;
  const ff = health.transcode.ffmpeg;
  const gpus = health.transcode.gpus ?? [];
  return (
    <div className="cv-settings-group">
      <Row
        label="Converting video"
        note={
          pb.ready
            ? `When a player can't play a file as it is, it's converted with ${pb.encoder}${pb.device ? ` on the ${pb.device}` : ""}.${pb.note ? ` ${pb.note}` : ""}`
            : `Only files a player supports will play. ${pb.note ?? ""}`
        }
        problem={!pb.ready}
      >
        <StatusChip ok={pb.ready} okText={pb.device ? "GPU" : "Ready"} badText="Unavailable" />
      </Row>
      <Row label="FFmpeg" note={ff.found ? `${ff.version ?? "Found"} · ${ff.path}` : ff.error ?? "Not found."} problem={!ff.found}>
        <StatusChip ok={ff.found} okText="Found" badText="Missing" />
      </Row>
      <Row
        label="Graphics cards"
        note={gpus.length ? gpus.map((g) => `${g.name} (${Math.round(g.memory_mib / 1024)} GB, driver ${g.driver})`).join("; ") : "None found: converting uses the processor."}
      />
      <Row
        label="Encoders"
        note={
          ff.encoders.length
            ? ff.encoders.map((e) => `${e.name}${e.usable ? "" : " (not working)"}`).join(", ")
            : "None listed."
        }
      />
      <p className="cv-caption settings-footnote">
        These come from the server's start-up checks. To change them, set <code>CINEVAULT_TRANSCODE_MODE</code> and related settings, then
        restart.
      </p>
    </div>
  );
}

function Notice({ text, onRetry }: { text: string; onRetry?: () => void }) {
  return (
    <div className="cv-notice cv-notice--error" role="alert">
      <p>{text}</p>
      {onRetry ? (
        <button className="cv-btn" onClick={onRetry}>
          Try again
        </button>
      ) : null}
    </div>
  );
}

const INTERVALS: { value: CacheSettings["check_interval"]; label: string }[] = [
  { value: "1h", label: "Every hour" },
  { value: "6h", label: "Every 6 hours" },
  { value: "24h", label: "Once a day" },
  { value: "168h", label: "Once a week" },
  { value: "manual", label: "Only when I check" },
];

function CacheSettingsSection() {
  const [status, setStatus] = useState<CacheStatus | null>(null);
  const [error, setError] = useState("");
  const [saveError, setSaveError] = useState("");
  const [checkRequested, setCheckRequested] = useState(false);
  const poll = useRef<number | undefined>(undefined);

  const load = useCallback(async () => {
    try {
      const s = await api.cache();
      setStatus(s);
      setError("");
      return s;
    } catch (e) {
      setError((e as Error).message);
      return null;
    }
  }, []);

  useEffect(() => {
    load();
    return () => window.clearTimeout(poll.current);
  }, [load]);

  // While a check runs, refresh every few seconds so the counts move.
  useEffect(() => {
    window.clearTimeout(poll.current);
    if (status?.checking || checkRequested) {
      poll.current = window.setTimeout(async () => {
        const s = await load();
        if (s && !s.checking) setCheckRequested(false);
      }, 2500);
    }
  }, [status, checkRequested, load]);

  async function save(next: CacheSettings) {
    if (!status) return;
    setSaveError("");
    const before = status;
    setStatus({ ...status, settings: next });
    try {
      await api.saveCacheSettings(next);
      if (next.enabled !== before.settings.enabled) {
        await load();
        if (next.enabled) setCheckRequested(true);
      }
    } catch (e) {
      setStatus(before);
      setSaveError((e as Error).message);
    }
  }

  async function checkNow() {
    setSaveError("");
    try {
      await api.checkCache();
      setCheckRequested(true);
    } catch (e) {
      setSaveError((e as Error).message);
    }
  }

  if (error) return <Notice text={error} onRetry={load} />;
  if (!status) return <p className="cv-caption">Loading…</p>;
  const on = status.settings.enabled;

  return (
    <div className="cv-settings-group">
      <Row
        label="Use the CineVault Cache"
        note={
          on
            ? "The cache knows which editions of a movie exist, how long each runs and what's different in each cut. Subscribers get it for every title in their library."
            : "Off: this server never contacts the CineVault Cache. Edition details you already have stay as they are."
        }
      >
        <Switch on={on} label="Use the CineVault Cache" onChange={(v) => save({ ...status.settings, enabled: v })} />
      </Row>

      {on ? (
        <>
          <h3 className="cv-settings-subhead">Subscription</h3>
          <TokenRow status={status} onChanged={async (checking) => {
            await load();
            if (checking) setCheckRequested(true);
          }} />
          {status.token_source ? <SubscriptionRow status={status} /> : null}

          {status.configured ? (
            <>
              <h3 className="cv-settings-subhead">Updates</h3>
              <Row
                label="Check for updates"
                note={
                  <>
                    {status.last_check && timeAgo(status.last_check) ? `Last checked ${timeAgo(status.last_check)}.` : "Not checked yet."}{" "}
                    {status.last_error && !status.checking ? <span className="is-problem">{status.last_error}</span> : null}
                    <span className="settings-result">
                      <Counts status={status} />
                    </span>
                  </>
                }
              >
                <select
                  aria-label="How often to check for updates"
                  className="cv-select"
                  value={status.settings.check_interval}
                  onChange={(e) => save({ ...status.settings, check_interval: e.target.value as CacheSettings["check_interval"] })}
                >
                  {INTERVALS.map((i) => (
                    <option key={i.value} value={i.value}>
                      {i.label}
                    </option>
                  ))}
                </select>
                <button className="cv-btn cv-btn--small" onClick={checkNow} disabled={status.checking || checkRequested}>
                  {status.checking || checkRequested ? "Checking…" : "Check now"}
                </button>
              </Row>
            </>
          ) : null}

          <h3 className="cv-settings-subhead">Sharing</h3>
          <Row
            label="Help build the CineVault Cache"
            note={
              <>
                Once a day, send which movies and shows you have, the editions in their file names, and how long each edition runs. That tells the
                cache which titles to research next, and your measured runtimes help confirm its figures. File names, folders and anything that
                identifies you or this server are never sent.
                {status.settings.share ? (
                  <span className="settings-result">
                    {status.last_share && timeAgo(status.last_share) ? `Last shared ${timeAgo(status.last_share)}.` : "Not shared yet."}
                    {status.last_share_error ? <span className="is-problem"> {status.last_share_error}</span> : null}
                  </span>
                ) : null}
              </>
            }
          >
            <Switch on={status.settings.share} label="Help build the CineVault Cache" onChange={(v) => save({ ...status.settings, share: v })} />
          </Row>
          <p className="cv-caption settings-footnote">Service: {status.url}</p>
        </>
      ) : null}

      {saveError ? (
        <p className="form-error" role="alert">
          {saveError}
        </p>
      ) : null}
    </div>
  );
}

/** The subscriber token: set by the environment, saved here, or a field to add one. */
function TokenRow({ status, onChanged }: { status: CacheStatus; onChanged: (checking: boolean) => void | Promise<void> }) {
  const [editing, setEditing] = useState(false);
  const [token, setToken] = useState("");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<{ text: string; problem: boolean } | null>(null);

  if (status.token_source === "environment") {
    return (
      <Row
        label="Token"
        note={
          <>
            Set by <code>CINEVAULT_CACHE_TOKEN</code> ({status.token_hint}). To manage it here instead, remove that line from{" "}
            <code>~/.config/cinevault/cache.env</code> and restart.
          </>
        }
      />
    );
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token.trim()) return;
    setBusy(true);
    setMessage(null);
    try {
      const r = await api.saveCacheToken(token.trim());
      setToken("");
      setEditing(false);
      setMessage(r.note ? { text: r.note, problem: true } : { text: "Token saved. Looking your titles up now.", problem: false });
      await onChanged(true);
    } catch (err) {
      setMessage({ text: (err as Error).message, problem: true });
    } finally {
      setBusy(false);
    }
  }

  async function remove() {
    if (!window.confirm("Remove the token? Edition details you already have stay, but updates stop until you add one again.")) return;
    setBusy(true);
    setMessage(null);
    try {
      await api.clearCacheToken();
      setMessage({ text: "Token removed.", problem: false });
      await onChanged(false);
    } catch (err) {
      setMessage({ text: (err as Error).message, problem: true });
    } finally {
      setBusy(false);
    }
  }

  const saved = status.token_source === "saved";
  const note = (
    <>
      {saved ? `A token is saved (${status.token_hint}).` : "Paste the token from your CineVault Cache account. It's checked with the cache before it's saved, and it's never shown again."}
      {message ? <span className={`settings-result${message.problem ? " is-problem" : ""}`}>{message.text}</span> : null}
    </>
  );

  if (saved && !editing) {
    return (
      <Row label="Token" note={note}>
        <button className="cv-btn cv-btn--small" onClick={() => setEditing(true)} disabled={busy}>
          Replace
        </button>
        <button className="cv-btn cv-btn--small" onClick={remove} disabled={busy}>
          Remove
        </button>
      </Row>
    );
  }
  return (
    <Row label="Token" note={note}>
      <form className="settings-token" onSubmit={submit}>
        <input
          className="cv-input"
          type="password"
          autoComplete="off"
          spellCheck={false}
          placeholder="cvc_…"
          aria-label="CineVault Cache token"
          value={token}
          onChange={(e) => setToken(e.target.value)}
        />
        <button className="cv-btn cv-btn--small cv-btn--primary" disabled={busy || !token.trim()}>
          {busy ? "Checking…" : "Save"}
        </button>
        {saved ? (
          <button type="button" className="cv-btn cv-btn--small" onClick={() => setEditing(false)} disabled={busy}>
            Cancel
          </button>
        ) : null}
      </form>
    </Row>
  );
}

function SubscriptionRow({ status }: { status: CacheStatus }) {
  const sub = status.subscription;
  if (!sub) {
    return <Row label="Status" note={`Can't check the subscription right now: ${status.problem ?? "no answer"}`} problem />;
  }
  const active = sub.status === "active";
  return (
    <Row
      label="Status"
      note={
        <>
          {sub.server_label ? `This server is "${sub.server_label}". ` : ""}
          {active && sub.expires_at ? `Renews ${formatDate(sub.expires_at.slice(0, 10))}. ` : ""}
          {active ? "" : "Edition details you already have stay; updates stop until the subscription is renewed."}
        </>
      }
      problem={!active}
    >
      <StatusChip ok={active} okText="Subscribed" badText={sub.status === "lapsed" ? "Lapsed" : "Cancelled"} />
    </Row>
  );
}

function Counts({ status }: { status: CacheStatus }) {
  const c = status.counts;
  const parts = [
    c.cached ? `${c.cached} with edition details` : "",
    c.pending ? `${c.pending} being researched` : "",
    c.not_found ? `${c.not_found} the cache doesn't know yet` : "",
    c.none ? `${c.none} not looked up yet` : "",
    c.error ? `${c.error} couldn't be looked up` : "",
  ].filter(Boolean);
  if (!parts.length) return <>No matched movies or shows yet.</>;
  return <>Titles: {parts.join(", ")}.</>;
}
