import { useCallback, useEffect, useRef, useState } from "react";
import { api, type CacheSettings, type CacheStatus } from "../api";
import { formatDate, timeAgo } from "../format";

const INTERVALS: { value: CacheSettings["check_interval"]; label: string }[] = [
  { value: "1h", label: "Every hour" },
  { value: "6h", label: "Every 6 hours" },
  { value: "24h", label: "Once a day" },
  { value: "168h", label: "Once a week" },
  { value: "manual", label: "Only when I check" },
];

export function SettingsPage() {
  return (
    <section className="settings">
      <h1 className="cv-page-title">Settings</h1>
      <CacheSettingsSection />
    </section>
  );
}

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

  return (
    <section className="settings-group" aria-labelledby="cache-heading">
      <h2 id="cache-heading" className="settings-heading">
        CineVault Cache
      </h2>
      <p className="settings-intro">
        The CineVault Cache knows which editions of a movie exist, how long each runs, and what's different in each cut. Subscribers get it for every
        title in their library.
      </p>
      {error ? (
        <div className="cv-notice cv-notice--error" role="alert">
          <p>{error}</p>
          <button className="cv-btn" onClick={load}>
            Try again
          </button>
        </div>
      ) : !status ? (
        <p className="cv-sub">Loading…</p>
      ) : (
        <>
          <Subscription status={status} />

          {status.configured ? (
            <div className="settings-row">
              <label htmlFor="cache-interval" className="settings-label">
                Check for updates
              </label>
              <select
                id="cache-interval"
                className="settings-select"
                value={status.settings.check_interval}
                onChange={(e) => save({ ...status.settings, check_interval: e.target.value as CacheSettings["check_interval"] })}
              >
                {INTERVALS.map((i) => (
                  <option key={i.value} value={i.value}>
                    {i.label}
                  </option>
                ))}
              </select>
              <div className="settings-check">
                <button className="cv-btn" onClick={checkNow} disabled={status.checking || checkRequested}>
                  {status.checking || checkRequested ? "Checking…" : "Check now"}
                </button>
                <span className="settings-note" role="status">
                  {status.last_check && timeAgo(status.last_check) ? `Last checked ${timeAgo(status.last_check)}.` : "Not checked yet."}{" "}
                  {status.last_error && !status.checking ? <span className="settings-problem">{status.last_error}</span> : null}
                </span>
              </div>
              <Counts status={status} />
            </div>
          ) : null}

          <div className="settings-row">
            <label className="settings-toggle">
              <input type="checkbox" checked={status.settings.share} onChange={(e) => save({ ...status.settings, share: e.target.checked })} />
              <span>
                <span className="settings-label">Help build the CineVault Cache</span>
                <span className="settings-note">
                  Once a day, send which movies and shows you have, the editions in their file names, and how long each edition runs. That tells
                  the cache which titles to research next, and your measured runtimes help confirm its figures. File names, folders and anything
                  that identifies you or this server are never sent.
                </span>
              </span>
            </label>
            {status.settings.share ? (
              <p className="settings-note settings-indent">
                {status.last_share && timeAgo(status.last_share) ? `Last shared ${timeAgo(status.last_share)}.` : "Not shared yet."}
                {status.last_share_error ? <span className="settings-problem"> {status.last_share_error}</span> : null}
              </p>
            ) : null}
          </div>
          {saveError ? (
            <p className="form-error" role="alert">
              {saveError}
            </p>
          ) : null}
        </>
      )}
    </section>
  );
}

function Subscription({ status }: { status: CacheStatus }) {
  const sub = status.subscription;
  if (!status.configured) {
    return (
      <div className="settings-row">
        <p className="settings-label">Not subscribed</p>
        <p className="settings-note">
          To use the cache, put your token in <code>CINEVAULT_CACHE_TOKEN</code> (for example in <code>~/.config/cinevault/cache.env</code>) and
          restart CineVault.
        </p>
      </div>
    );
  }
  if (!sub) {
    return (
      <div className="settings-row">
        <p className="settings-label">Subscribed</p>
        <p className="settings-note settings-problem">Can't check the subscription right now: {status.problem}</p>
      </div>
    );
  }
  const active = sub.status === "active";
  return (
    <div className="settings-row">
      <p className="settings-label">{active ? "Subscribed" : sub.status === "lapsed" ? "Subscription lapsed" : "Subscription cancelled"}</p>
      <p className={active ? "settings-note" : "settings-note settings-problem"}>
        {sub.server_label ? `This server is "${sub.server_label}". ` : ""}
        {active && sub.expires_at ? `Renews ${formatDate(sub.expires_at.slice(0, 10))}. ` : ""}
        {active ? "" : "Edition details you already have stay; updates stop until the subscription is renewed."}
      </p>
    </div>
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
  if (!parts.length) return <p className="settings-note">No matched movies or shows yet.</p>;
  return <p className="settings-note">Titles: {parts.join(", ")}.</p>;
}
