# Feature spec: Editions & Versions (merge and choose at play)

Status: Draft 4 · 26 Nov 2026 · Master list ref: LIB-11 ("Multiple versions or cuts of one movie") · All open questions resolved

## Decisions log

| # | Question | Decision (26 Nov 2026) |
|---|---|---|
| 1 | Several editions, no saved preference: what does Play do? | **Ask.** Play opens the edition picker. |
| 2 | Is watched status per title or per edition? | **Per edition only.** Watching one cut marks only that cut. |
| 3 | Badge on posters? | **Yes**, for example "2 editions · 4K". |
| 4 | Edition names: free text or a list? | **A fixed list.** It drives indexing, prefill and the edition listing on cards (§6–§7). |
| 5 | Limit on editions listed on a card? | **No limit.** The card shows the first three, then a "Show all" control that lists every edition (§7). |
| 6 | Edition ribbon on fallback artwork? | **On by default**, with a setting to turn it off (§6c). |
| 7 | Starter Edition List | **Approved as listed in §4.** Settings includes an option to add custom entries (§4.1), in V1. |

## 1. Goal

One library item per work, however many copies the user owns. The user merges copies into a single title and labels each one with its cut and its quality. Cards and details pages show which editions they own, and the user picks the copy when playing. No separate 4K library and no duplicate posters.

**Example: Aliens (1986)**

```
Aliens (1986)                      ← one card in the library
├── Edition: Theatrical (137 min)
│   ├── Version: 4K HDR10 · TrueHD Atmos · 58 GB
│   └── Version: 1080p · DTS-HD MA · 14 GB
└── Edition: Director's Cut (154 min)
    └── Version: 1080p · DTS-HD MA · 16 GB
```

## 2. Why: what goes wrong today

- **Plex** merges "versions" (same cut, different quality) into one item, but "editions" (different cuts) show up as separate items, so the grid shows duplicate posters. Users report:
  - Different editions are hard to tell apart.
  - The "Other Editions" section is buried at the bottom of the page.
  - Progress isn't tracked across editions.
  - Shared extras break when editions are split.
  - Scans re-merge items the user split by hand.
  - Removing one file can drop every version from the library.
  - Unmatching a split item can lose its metadata.
- **Many users keep a separate 4K library** to control who sees 4K and to avoid converting 4K for remote users. The cost is duplicate items and split watch history.
- **Jellyfin and Emby** group versions through naming or a "Group versions" action. They have no first-class concept of an edition.

## 3. Core concepts

| Concept | Meaning | Examples |
|---|---|---|
| **Title** | The work itself. Holds the shared metadata. | Aliens (1986); *Rumours* (album) |
| **Edition** | A distinct cut or release, with different content, runtime or tracklist. Always chosen from the Edition List (§4). | Theatrical, Director's Cut; Original, Deluxe |
| **Version** | A file or file set for one edition. Same content, different encoding. | 4K HDR, 1080p; FLAC 24/96, MP3 320 |

Every title has at least one edition, and every edition has at least one version. A title with one file gets a default edition (Theatrical, Broadcast, Original and so on), and the editions layer stays hidden until a second edition exists.

**Not editions:** language dubs and subtitle languages are tracks inside a version. Episode orders (aired or DVD order) are a separate TV setting.

## 4. The Edition List (fixed vocabulary)

Edition names are never free text at the point of labeling a file. Each entry in the list has:

| Field | Purpose | Example |
|---|---|---|
| `id` | Stable key used for matching, filters and the API | `directors_cut` |
| Display name | Shown on details pages and in the picker | Director's Cut |
| Short label | Shown on cards and badges | DC |
| Aliases | Filename, folder and tag keywords used during indexing | `directors.cut`, `DC`, `{edition-Director's Cut}` |
| Media types | Which libraries the entry applies to | Movie, TV |
| Sort order | Order in which editions are listed on cards and in the picker | Theatrical first |

**Starter list by media type (approved, decision 7):**

- **Movies:** Theatrical, Director's Cut, Extended, Unrated, Uncut, Special Edition, Final/Ultimate Cut, IMAX, Remastered, Restored, Anniversary Edition, Black & White, Open Matte, 3D, TV/Censored Edit, Alternate Ending, Fan Edit
- **TV:** Broadcast, Extended, Uncut, Remastered, Original Aspect (4:3), Widescreen, Director's Cut
- **Music (albums):** Original, Remaster, Deluxe/Expanded, Anniversary, Mono, Stereo, Live, Explicit, Clean, Instrumental, Demos
- **Audiobooks:** Unabridged, Abridged, Dramatized, Alternate Narrator
- **Books and comics:** Original, Revised, Annotated, Collected/Omnibus

**Benefits of a fixed list:** reliable auto-matching at index time, consistent badges, library filters ("show every Director's Cut I own"), and per-edition artwork and metadata lookups.

### 4.1 Custom entries (Settings)

**Location:** Settings → Library → Edition List.

- **The screen** shows the full list grouped by media type. Built-in entries are marked "Built-in"; custom entries are marked "Custom".
- **"Add edition"** opens a form with the same fields as built-in entries:
  - **display name** (required, unique within its media types)
  - **short label** (required, up to about 12 characters, so it fits on cards)
  - **media types** (one or more)
  - **aliases** (optional). Each alias is a keyword the indexer matches in file and folder names and tags, for example `rogue.cut`. `{edition-<display name>}` is always added automatically.
  - **sort position**, set by dragging the entry in the list
- **Built-in entries:**
  - can't be deleted, but can be hidden, which removes them from dropdowns
  - can be given extra aliases and a different sort position
- **Custom entries:**
  - can be edited freely
  - can be deleted only when no file uses them. If files do, the user must first pick a replacement entry, and those files are relabeled.
- **After an entry is added or its aliases change,** a "Re-check library" prompt offers to rescan file names against the new aliases. Matches go to the review queue as suggestions; nothing is relabeled without confirmation.
- **The merge screen dropdown** ends with **"+ Add edition…"**, which opens the same form inline, so the user doesn't have to leave the merge flow.
- **Permissions:** only admins can add, edit or hide entries. Other users choose from the list.
- **Export/import:** the list can be exported or imported as a file, for backups and for moving to another server.

## 5. Merge flow

1. Multi-select two or more items and click **Merge**, or open a title and click **Add copy…**.
2. **Choose the primary metadata** (poster, synopsis, cast). The other matches are kept and can be restored.
3. **Review screen**, with one row per file:
   - **Edition:** a dropdown from the Edition List, already filled in by indexing (§6). It ends with "+ Add edition…" (§4.1).
   - **Version label:** auto-filled from media info, for example "4K · HDR10 · Atmos".
   - Runtime, size, codecs and path, read-only.
   - A warning when files under one edition differ in runtime by more than about 3 minutes.
4. **Preview, then confirm.** Undo is available right after merging, and **Split** is available at any time. Any version or edition can be moved to its own title or into another title, keeping metadata on both sides.

## 6. Indexing & prefill

When a file is scanned, CineVault fills in three things before the user touches it.

**a. Edition match.** The file is matched to an Edition List entry, built-in or custom, using, in order:
1. explicit naming (`{edition-…}`)
2. alias keywords in file and folder names
3. embedded tags
4. runtime clustering against the title's known runtime (within about 2 minutes means the same edition)
5. for music, the release type from MusicBrainz

Each match carries a confidence score. High-confidence matches are applied automatically. Low-confidence matches are queued as suggestions ("New copy of *Aliens* found: Director's Cut?").

**b. Video and audio data**, probed per version:
- resolution, HDR/Dolby Vision, codec, bitrate, frame rate
- audio format, channels and Atmos; for music, bit depth and sample rate
- runtime, chapters, subtitle and audio tracks

Per-edition processing then runs, because timelines differ between cuts:
- intro, credit and recap detection
- seek-bar thumbnails
- chapter images

**c. Image data**, filled per edition with fallbacks:
1. **Edition-specific artwork** from providers when it exists. Music is strong here: MusicBrainz models a release group (title) with multiple releases (editions), and the Cover Art Archive has per-release covers. Edition-specific art is rarer for movies and TV, so fanart.tv and similar sources are tried.
2. **Fallback:** the title's poster with a generated edition ribbon (for example "DIRECTOR'S CUT"). **On by default** (decision 6). Settings → Display → "Edition ribbon on artwork" turns it off.
3. **User override:** upload or pick replacement art per edition.

**Manual decisions are locked.** Rescans, drive swaps and path changes never re-merge or re-split what the user arranged. Files are tracked by content fingerprint (size plus partial hash), not by path.

## 7. Library cards: badge and edition listing

This applies to every media type: movies, TV shows, albums, audiobooks and books.

- **Badge** on the poster, for example "2 editions · 4K". The badge shows when a title has more than one edition, or a 4K or HDR version.
- **Edition listing on the card.** When a title has more than one edition, the card lists them under the title using short labels, in the Edition List sort order.
  - **No limit** on how many editions a title can list (decision 5).
  - The card shows the first three, followed by a **"Show all (N)"** control. Selecting it expands the listing in place to every edition, and selecting again collapses it.
  - With three or fewer editions, all are shown and the control is hidden.
  - Examples:
    - Movie: `Theatrical · DC · Extended · Show all (5)`
    - TV show: `Broadcast · Remastered`, or `4 extended episodes` when only some episodes have editions
    - Album: `Original · Deluxe · 2011 Remaster`
    - Audiobook: `Unabridged · Abridged`
- **Per-edition watched marks** sit inside the listing, for example `✓ Theatrical · DC`. A progress bar shows only for the edition in progress.
- **Hover or focus** (TV remote) expands the card to show each edition with its quality badges, runtime and watched state. The listing never needs a separate page.
- **Details page:** edition chips under the title. Selecting a chip switches the runtime, chapters, artwork and description shown.

## 8. Playback: choosing a version

- **Play on a title with more than one edition and no saved preference opens the edition picker** (decision 1). The picker lists:
  - each edition: name, runtime, edition artwork, watched or in-progress state
  - each version under it: quality badges, audio, and a status line for this device: **Direct play** / **Will convert to 1080p** / **Not available remotely**
- **"Remember for this title"** is a checkbox in the picker. If it's ticked, later Plays skip the picker.
- **Choosing a version within an edition is automatic:** the best version this device can direct play within current bandwidth, respecting the user's quality limit. A "Choose version…" option is always available.
- **Titles with one edition** play immediately. The picker appears only when there are several versions and the setting "Always ask which version" is on.
- **Continue Watching** shows the edition, for example "Aliens · Director's Cut · 48 min left". Resuming from Continue Watching never asks, because the edition is already known.

## 9. Watch state (decision 2: per edition)

- **Watched status is per edition.** Watching the Theatrical cut doesn't mark the Director's Cut watched.
- **The title-level display** shows "1 of 2 editions watched". The title counts as fully watched only when every edition is watched.
- **Library filters:** "Unwatched" includes any title with an unwatched edition. An "Unwatched – any edition seen" toggle lets users hide titles where they've seen at least one cut.
- **Resume position** is per edition. It carries across versions of the same edition and never across editions.
- **Mark watched and unwatched** work per edition, plus a "Mark all editions" shortcut.
- History records which edition and version was played.

## 10. What belongs where

| Title-level (shared) | Edition-level | Version-level |
|---|---|---|
| Synopsis, cast, crew, genres, ratings, extras/trailers, collections, user rating | Edition List entry, runtime, chapters, intro/credit segments, artwork, edition description, content rating if different, watched state, resume position, tracklist (music) | File(s), codecs, HDR, audio/subtitle tracks, bitrate, size, path, seek-bar thumbnails |

## 11. Access & quality rules (replaces the separate 4K library)

- **Per-user maximum quality**, for example "Friends: up to 1080p". Versions above the limit are invisible to that user.
- **Remote and low-bandwidth playback** picks a lower existing version before resorting to transcoding.
- **The admin can hide specific editions or versions** from chosen users.
- **Parental controls apply per edition**, because an Unrated cut can carry a different rating from the theatrical release.

## 12. Downloads

- The download dialog lets the user pick the edition, then the version, or convert to a smaller copy.
- It suggests the smallest version that looks good on the device.

## 13. Edge cases

- **Removing a file** removes only that version. An edition disappears only when its last version is gone, and a title only when its last edition is gone.
- **An offline drive** greys out its versions but keeps the title, its data and its watch state.
- **Exact duplicate files** (same fingerprint) are flagged so one can be removed.
- **Multi-part files** (CD1/CD2) form a single version.
- **Merging items matched to different works** asks for confirmation.
- **Extras in a shared folder** attach to the title, so every edition keeps them.
- **A file that matches no Edition List entry** gets the type's default edition plus a low-confidence suggestion in the review queue.
- **A hidden built-in entry that files still use** keeps working on those files. It is only removed from dropdowns for new labeling.

## 14. Scope

- **V1:**
  - the Edition List for all media types, **including custom entries in Settings (§4.1)**
  - indexing and prefill
  - card badge and edition listing for all types
  - merge, split and the picker for movies, TV and music
  - per-edition watch state and resume
  - per-user quality limits
- **V1.1:**
  - per-episode editions in TV
  - audiobook and book editions
- **Later:** community-shared Edition List additions and an artwork source for movie editions.

## 15. Open questions

None. All decisions are recorded in the log above.
