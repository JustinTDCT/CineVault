# Feature spec: CineVault Cache (subscription) & edition intelligence

Status: Draft 2 · 27 Nov 2026 · Related: [Editions & Versions](editions-and-versions.md), [Collections & Viewing Orders](collections-and-viewing-orders.md)

## Decisions log

| # | Question | Decision (26 Nov 2026) |
|---|---|---|
| 1 | Business model | **Free core system.** The paid subscription is access to the CineVault Cache server. |
| 2 | What does the cache hold? | Title metadata, plus edition data the cache researches itself: which editions exist and what each one changes, gathered with AI. |
| 3 | What if the cache doesn't have a title? | The user's server falls back to the normal sources (TMDb, TVDB) and flags the item. The cache fills itself in the background. |
| 4 | How does the user's server learn about updates? | It checks the cache periodically. The interval is a user setting, and there's a "Check now" button. |
| 5 | Edition display | Each edition shows a short "what's different" line, and each edition has its own tab on the details page. |
| 6 | Can free servers help the cache grow? (27 Nov 2026) | **Yes.** Every CineVault server, subscribed or not, can share which titles it has (§5a). The setting is **on by default**, shown on the setup page, and can be turned off at any time. |
| 7 | What do servers share? (27 Nov 2026) | Type and TMDb ID, the Edition List IDs found in file names, and **each edition's runtime measured from the file**, rounded to the minute. Nothing else. |
| 8 | Research for titles only free servers have (27 Nov 2026) | **Full AI research**, because the development model runs locally (Ollama with SearXNG) at no cost per title. Subscriber requests still come first in the queue. To revisit if a paid model is chosen. |

## 1. Goal

- **Everyone** gets a complete, free media server that fetches metadata the normal way.
- **Subscribers** get faster, richer metadata from the CineVault Cache. Above all, they get **edition intelligence**: every known edition of a title, what each one adds or changes, and which of those editions they own.

## 2. Free vs subscription

| | Free core | Subscriber |
|---|---|---|
| Media server, apps, editions and versions, collections | ✓ | ✓ |
| Metadata source | TMDb, TVDB, MusicBrainz and others, directly | CineVault Cache first, then direct sources as fallback |
| Edition matching from file names (Editions spec §6) | ✓ | ✓ |
| Known editions the user doesn't own, shown greyed | — | ✓ |
| "What's different" lines and full edition details | — | ✓ |
| Corrections shared across all subscribers | — | ✓ |

**The core never depends on the cache.** If the cache is unreachable or the subscription ends, the server keeps working with direct sources.

## 3. How it works

```
User's CineVault server                     CineVault Cache server
───────────────────────                     ──────────────────────
1. Index finds Aliens (1986)
   + "Aliens Director's Cut"
2. Ask cache for TMDb ID 679 ────────────▶  Hit?  ──yes──▶ return title + editions
                                              │
                                              no
                                              ▼
3. "Not cached yet" ◀──────────────────────  Queue title for research
4. Fill from TMDb directly                   ├─ fetch base metadata
   Mark item: CACHE PENDING                  ├─ AI edition research (§5)
                                             └─ store cache record
5. Periodic check (user interval)
   "Any updates for pending items?" ──────▶  Return records now ready
6. Apply update:
   - enrich title metadata
   - match owned files to known editions
   - add unowned editions as greyed placeholders
   Mark item: CACHED
```

**Item cache states,** shown in the admin view and filterable:

| State | Meaning |
|---|---|
| Not subscribed | The item uses direct sources only. |
| Cache pending | The user's server asked, and the cache is still researching. The item is filled from direct sources meanwhile. |
| Cached | The item is filled from the cache and up to date. |
| Update available | The cache record changed (new edition found, correction made). It applies at the next check. |
| Cache error | The request failed. It retries automatically, and direct-source data stays in place. |

## 4. Update checks (decision 4)

- **Setting:** Settings → CineVault Cache → "Check for updates" with the options Every hour / Every 6 hours / **Daily (default)** / Weekly / Manual only.
- **"Check now"** button in the same place, plus "Refresh from cache" on any single item.
- **Checks are batched:** one request lists every pending item and every cached item whose record version is older than the cache's.
- **The cache can also say "check sooner"** when a pending item finishes research, so new items don't wait a whole day. That hint never shortens a "Manual only" setting.
- **Back-off:** if the cache is down, checks back off and retry. Nothing in the library is blocked or removed.
- **Local edits always win.** A field the user edited by hand (title, poster, edition label) is locked and never overwritten by a cache update. The update screen notes "1 field kept as you edited it".

## 5. Edition research (cache server, AI-assisted)

For each title, the cache server does the following.

**1. Find known editions.** Sources include:
- metadata providers' alternative titles and release data
- edition naming seen across subscribers' libraries, collected anonymously as aggregated counts only (§9)
- reference sources that document cuts, where their terms allow it

**2. Map each edition to the Edition List** (Editions spec §4): Theatrical, Special Edition / Director's Cut, Extended and so on.

**3. Work out the differences.** AI compares the edition sources and writes:
- **runtime** for each edition, with its source
- a **short difference line**, about 15 words, for cards and edition lists
- **full difference notes:** added, removed and changed material, listed by scene in story order
- a **content-rating difference** if there is one, for example Unrated

**4. Attach sources and confidence** to every fact. Each entry records which sources it came from and a confidence level: high, medium or low.

**5. Review:**
- High-confidence entries publish automatically.
- Low-confidence entries go to a human review queue before they are served.
- Entries never publish with an unsupported number. A runtime without a source is left blank, not estimated.

**6. Corrections:** every edition page has **"Report a mistake"**. Reports go to the review queue, and fixes reach all subscribers at their next check.

**Example cache record (simplified)**

```json
{
  "tmdb_id": 679,
  "title": "Aliens",
  "record_version": 3,
  "editions": [
    {
      "edition": "theatrical",
      "runtime_min": 137,
      "difference_short": null,
      "sources": ["tmdb"],
      "confidence": "high"
    },
    {
      "edition": "special_edition",
      "display_name": "Special Edition (Director's Cut)",
      "runtime_min": 154,
      "difference_short": "Adds 17 minutes, including the colony before the outbreak, Ripley's daughter and the sentry guns.",
      "difference_full": ["…scene-by-scene notes…"],
      "sources": ["…"],
      "confidence": "high",
      "reviewed": true
    }
  ]
}
```

## 5a. Shared titles from every server (decisions 6–8)

The cache grows from every CineVault server, not only subscribers. A free server can't look titles up, but it can tell the cache what it has, so most titles are researched before a subscriber ever asks.

**What a server sends.** Once a day, at a random time, each server sends one batch of the titles added or changed since its last batch:

```json
{
  "install": "k3v…",
  "items": [
    { "kind": "movie", "tmdb_id": 679,
      "editions": [ { "edition": "theatrical", "runtime_min": 137 },
                    { "edition": "special_edition", "runtime_min": 154 } ] }
  ]
}
```

- `install` is a random ID made once per server. It isn't tied to an account, a token or a person.
- Editions are Edition List IDs only. `runtime_min` is the owned file's duration, rounded.
- Never file names, paths, file sizes, watch history or anything else from the library.
- Subscribed servers send the same batch: their lookups don't carry runtimes.

**The setting.** Settings → CineVault Cache → "Help build the CineVault Cache", **on by default**. The setup page shows it with one sentence explaining what's sent, so nobody shares without seeing it. Turning it off stops sending at once.

**What the cache does with it.**
1. **Popularity:** each title counts the separate servers that have it, and the research queue uses that order (after subscriber lookups and reports).
2. **Research:** titles not yet researched are queued for the full pipeline (§5). With the local model this costs nothing per title.
3. **Real-world runtimes:** for each title and edition, the cache counts how many separate servers have a file of each length. Once at least **5 separate servers** agree within 2 minutes, that counts as one corroborating source for the confidence rules. It's never the only source for publishing, and it's shown to reviewers as "seen in 37 libraries at 154 min".

**Free servers get nothing back** apart from "accepted". Records, editions and differences stay subscriber-only, so the subscription keeps its value.

**Abuse.** The endpoint needs no token, so anyone could send fake data. Shared titles therefore only raise priority and add to counts. They never publish anything on their own, and a count only matters once enough separate servers agree. Each install is rate-limited.

## 6. Display (decision 5)

**Details page: one tab per edition**

```
┌──────────────────────────┬─────────────────────────────────────┐
│ ALIENS (Theatrical) ✓    │ ALIENS (Special Edition) ✓          │
└──────────────────────────┴─────────────────────────────────────┘
```

- **Tab order** follows the Edition List sort order. Owned editions show ✓. Unowned editions show greyed, with "Not in library".
- **The first tab** is the user's preferred or last-watched edition, falling back to Theatrical or the type's default edition.
- **On a TV remote**, the tabs are the first row under the title and are reached with left and right.

**Theatrical (base) tab**
- the normal synopsis, runtime (137 min), cast, ratings and versions owned (4K, 1080p)
- an **Editions** section under it listing every other edition with its short difference line:
  - **Special Edition (Director's Cut)** · 154 min · ✓ owned. *Adds 17 minutes, including the colony before the outbreak, Ripley's daughter and the sentry guns.*

**Special Edition tab**
- the same base details (synopsis, cast and so on), with this edition's runtime and versions
- a **"What's different in this cut"** section with the full notes, sources and a "Report a mistake" link
- an **Editions** section listing the other editions with their difference lines, the Theatrical cut included

**Cards and lists:** the edition listing on cards (Editions spec §7) uses the short difference line as a tooltip or focus text. Unowned editions appear greyed in the "Show all" list.

**Edition picker at Play** (Editions spec §8): each owned edition shows its short difference line, so the choice is informed.

**Collections:** unowned editions from the cache can be pinned in a collection order as placeholders (Collections spec §9).

**Free users** see only the editions they own, with no difference text. A subtle "See what's different with CineVault Cache" link appears on the Editions section, which the admin can turn off.

## 7. Failure and fallback rules

- **The cache is never in the critical path.** Indexing, browsing and playback work whether or not the cache answers.
- **A cache response that conflicts with the user's files** (for example a runtime that doesn't match any owned file) never changes file-to-edition matching on its own. It is offered as a suggestion in the review queue (Editions spec §6).
- **A record withdrawn from the cache** after review keeps what the user already has and marks it "No longer verified".

## 8. Licensing (must be resolved before launch)

- **TMDb** offers its API free for non-commercial use with attribution. It defines a project as commercial when its "primary purpose is to create revenue." A paid cache that stores and serves TMDb data needs a **commercial license** from TMDb's sales team. The free core may also count as commercial if it's part of the business.
- **TheTVDB** directs commercial use to its sales team and requires a link to TheTVDB wherever its data is shown, unless it approves otherwise.
- **MusicBrainz and the Cover Art Archive** have their own terms to review for commercial and redistributed use.
- **AI research sources:** only sources whose terms permit this use. Difference notes are written as original summaries and never copy a source's text.
- **Design implication:** the most valuable cache content (editions and differences) is CineVault's own work. The plan should allow serving only licensed base metadata, plus original edition data, if licensing limits what can be redistributed.
- **Attribution** (TMDb, TVDB and others) shows wherever their data appears, in both the free and paid tiers.

This section isn't legal advice. It needs confirming with each provider and with counsel.

## 9. Privacy & security

- **Lookups send only metadata IDs** (TMDb, TVDB, MusicBrainz), plus the edition labels found when needed. Never file names, paths or other library details.
- **Lookups are separated from the account.** Authentication uses a subscription token checked at the edge, and lookup logs don't store which subscriber asked for which title.
- **Shared titles (§5a)** are kept only as counts, plus a keyed hash of each install ID so separate servers can be counted. The hash key changes every 90 days and older rows are deleted, so no long-lived list of what one server has is kept. The cache doesn't store the sender's IP address.
- **On by default needs care (not legal advice):** in some places, the EU in particular, sharing on by default may need a lawful basis other than consent, a clear notice and an easy opt-out. Check this with counsel before launch; the setup-page notice is designed to meet the notice part.
- **The privacy policy** states plainly what is sent and kept.
- **Abuse protection:** tokens are per server, rate limits apply per token, and tokens can be revoked.

## 10. Subscription lifecycle

- **Subscribe:** enter a token or sign in. The server marks all items "Cache pending" for a first sync, done in batches.
- **Lapse:** handling of cached data already on the user's server depends on licensing (open question 1). In every case, the server continues on direct sources without losing library data, watch history or the user's own edits.

## 11. Scope

- **V1:**
  - cache lookup with fallback
  - item cache states
  - batched update checks with a user-set interval and "Check now"
  - AI edition research with sources, confidence and human review
  - edition tabs and short difference lines
  - greyed unowned editions
  - "Report a mistake"
  - shared titles from every server, with real-world runtimes (§5a)
- **V1.1:**
  - the "check sooner" hint
  - difference text in the edition picker
- **Later:**
  - difference data for TV episodes (extended episodes)
  - difference data for music editions (deluxe tracklists)
  - subscriber community contributions

## 12. Open questions

1. When a subscription lapses, does the user keep cached data already on their server, or does it revert to direct-source data? This depends on the licensing terms.
2. Who reviews flagged edition entries and mistake reports: your team only, or trusted community reviewers too?
3. Is Daily the right default for update checks?
4. Pricing and tiers: one plan, or tiers such as by number of servers?
