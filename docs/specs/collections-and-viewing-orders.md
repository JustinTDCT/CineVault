# Feature spec: Collections & Viewing Orders

Status: Draft 2 · 26 Nov 2026 · Related: [Editions & Versions](editions-and-versions.md) · All open questions resolved

## Decisions log

| # | Question | Decision (26 Nov 2026) |
|---|---|---|
| 1 | Should a collection's sort order be independent of the library's sort? | **Yes.** Each collection has its own orders; library sorting is unaffected. |
| 2 | Can a collection mix media types? | **Yes.** Movies and TV (whole shows, seasons or individual episodes) in one collection, in any order. |
| 3 | Can titles the user doesn't own be added as placeholders? | **Yes, in V1.** Shown greyed as "Not in library" (§9). |
| 4 | Personal or server-wide collections in V1? | **Server-wide only in V1.** Personal collections move to Later. |
| 5 | Nested collections? | **Later.** |

## 1. Goal

A collection groups related titles, such as a franchise, and plays them in an order the user chooses: release order, story order, or a custom order. That order applies only inside the collection. Libraries keep their normal sorting, by name for example. No folder structures and no extra libraries are needed. A collection can also show the titles the user doesn't own yet.

## 2. Why: the workaround today

Getting a custom-ordered franchise in Plex today means:

1. Keeping the franchise in its own folder (`/movies/series/lord-of-the-rings`).
2. Adding a second library pointed at `/movies/series`.
3. Creating a collection in that library and setting custom sorting on it.

The order is tied to how the files are stored and to a library created just for it. Collections also belong to a single library, so movies and TV shows can't share one collection. Playlists can mix media types, but they are flat lists without the collection page, sections, progress or multiple orders.

## 3. Core concepts

| Concept | Meaning |
|---|---|
| **Collection** | A named group, such as "Middle-earth" or "Star Wars". It has artwork, a description, and one or more orders. It isn't tied to folders or libraries. |
| **Entry** | One item placed in an order. See the entry types below. It can be owned or a placeholder (§9). |
| **Order** | A named sequence of entries, such as "Story order" or "Release order". A collection can have several, and one is the default. |
| **Section** | An optional heading inside an order that groups entries, such as "The Prequels". |

**Entry types:**

| Entry type | Example | Behavior |
|---|---|---|
| Movie | *The Fellowship of the Ring* | Can pin an edition, for example Extended (see the Editions spec). |
| Whole TV show | *The Rings of Power* | Plays every season in episode order. |
| Season | *The Mandalorian* Season 2 | Plays that season's episodes in order. |
| Episode range | *The Book of Boba Fett* S1 E1–E4 | Lets a show be split across the order. |
| Single episode | *The Book of Boba Fett* S1 E5 | Places one episode precisely. |
| Other media | Soundtrack album, book, audiobook | Listed and openable from the collection; not part of "Play collection" by default. |

A show can therefore sit in one place as a whole, or be broken into seasons, ranges or episodes and placed between other titles.

## 4. Orders

- **Automatic orders**, always available and kept up to date: release or air date, name, date added.
- **Custom orders,** built by dragging. A collection can have any number, each with its own name.
- **Default order:** the admin sets one per collection. Each user's last-chosen order is remembered for them.
- **The order switcher** on the collection page changes the listing, the playback order and progress at once.

**Example: Middle-earth**

*Story order (custom)*
1. *The Rings of Power*, all seasons
2. *The Hobbit: An Unexpected Journey*
3. *The Hobbit: The Desolation of Smaug*
4. *The Hobbit: The Battle of the Five Armies*
5. *The Fellowship of the Ring*, pinned to Extended
6. *The Two Towers*, pinned to Extended
7. *The Return of the King*, pinned to Extended

*Release order (automatic):* the Lord of the Rings trilogy (2001–03), the Hobbit trilogy (2012–14), *The Rings of Power* (2022–).

**Example: Star Wars, story order (custom, with sections)**
- **The Prequels:** *The Phantom Menace*, *Attack of the Clones*, *Revenge of the Sith*
- **Anthology:** *Solo*, *Rogue One*
- **The Classics:** *A New Hope*, *The Empire Strikes Back*, *Return of the Jedi*
- **Series:** *The Book of Boba Fett*
- **The Sequels:** *The Force Awakens*, *The Last Jedi*, *The Rise of Skywalker*

Users can add further shows anywhere, as whole shows or split by season or episode. If any title in the order isn't owned, it appears as a placeholder (§9).

## 5. Building and editing a collection

- **Create** from the library (multi-select → "Add to collection"), from a title's menu, or from the Collections page. Collections are server-wide (decision 4).
- **The order editor:**
  - search the whole server (all libraries and media types) and add results
  - **search online metadata** (TMDb, TVDB) to add titles that aren't in the library, as placeholders (§9)
  - drag to reorder
  - add or rename sections
  - **expand a show** to its seasons and episodes and drag any part of it to its own position. Episodes not in the library appear as placeholders.
  - pin an edition on a movie entry
- **Duplicate entries:** the editor warns when an episode or movie appears twice in the same order. The user can keep it on purpose.
- **Duplicating an order** creates a copy to start a new one from, for example "Story order" → "Story order + Andor".
- **Undo** while editing. Changes save automatically.

## 6. Display

**Library grid**, set per collection:
- **"Show as one card"**: the members are hidden from the library grid and the collection card appears instead.
- **"Show members individually"**: no collection card in that library.
- **"Both"** (default): members sort normally, by name for example, and the collection card also appears.

**Collection card:**
- badge with the contents, for example "7 movies · 1 show"
- progress for the user's current order, for example "4 of 10 watched · 2 not in library"

**Collection page:**
- header with artwork, description and the order switcher
- the entries in the chosen order, grouped into sections where they exist
- each entry shows its watched or in-progress state, or **Not in library**
- a show entry shows its own progress, for example "Season 1: 5 of 8"

**Each title's details page** shows "Part of: Middle-earth (Story order #5)" with links to the previous and next entries.

## 7. Playback

- **"Play from the start"** and **"Continue"** on the collection page follow the selected order.
- **Up Next follows the collection.** When the last *Rings of Power* episode finishes, Up Next offers *An Unexpected Journey*. A show, season or range entry plays its episodes in order, then moves to the next entry.
- **Placeholders are skipped.** Up Next notes the skip: "Next: *Rogue One* (skipping *Solo*, not in library)".
- **Editions:** a pinned edition plays without asking. For an unpinned movie with several editions, the edition picker appears, following the Editions spec (decision: ask).
- **Continue Watching** shows the collection context, for example "Middle-earth · Story order · Next: *The Hobbit*".
- **Leaving the collection:** if the user plays a member title from the library instead, Up Next follows that title's own logic. A one-time prompt offers "Continue in Middle-earth order?"

## 8. Watch state

- **Collection progress is derived** from each item's own watched state. Watching *The Two Towers* anywhere counts toward the collection, and there's no separate collection watched flag.
- **A pinned edition** counts only when that edition is watched, consistent with per-edition watched status.
- **Each order has its own progress,** because entries and their positions differ.
- **Placeholders are counted separately,** for example "6 of 8 watched · 2 not in library". They never count as unwatched, so an order can still reach "complete" for everything owned.

## 9. Placeholders: titles not in the library (decision 3)

- **Adding:** in the order editor, search online metadata and add any movie, show, season or episode the user doesn't own. It's stored by its metadata ID (TMDb or TVDB) with title, year, poster and synopsis.
- **Appearance:**
  - a greyed poster with a "Not in library" label
  - it keeps its position, section and pinned edition, if one is chosen
  - selecting it opens a details page with metadata and the note "Not in your library"
- **Becoming real:** when a matching title is added to the library (matched by metadata ID), the placeholder turns into a normal entry automatically and keeps its position.
- **Prefill:**
  - TMDb auto-created collections include placeholders for collection members the user doesn't own
  - whole-show entries include placeholders for missing episodes, using the missing-episodes data from indexing
  - both can be turned off per collection
- **"Missing from this collection" view:** a filter on the collection page that lists only placeholders, as a shopping or wish list.
- **Visibility:**
  - collections are server-wide, so everyone who can see the collection sees its placeholders
  - each user can turn them off with "Show missing titles" in their own settings
  - placeholders never reveal content a user's parental controls would hide
- **Playback:** placeholders can't be played and are skipped (§7).

## 10. Auto-creation & prefill

- **Movie collections are auto-created** from TMDb's movie collections, for example "The Lord of the Rings Collection" and "The Hobbit Collection". They are prefilled in release order, with placeholders for missing members (§9). The admin can turn auto-creation off.
- **Merge suggestions:** when two auto-created collections share a franchise, such as the Lord of the Rings and Hobbit collections, CineVault suggests merging them into one collection.
- **Related TV shows are suggested, never added automatically:** "*The Rings of Power* may belong to Middle-earth: add?" Franchise data for TV is unreliable, so suggestions need confirmation.
- **Export and import** a collection's orders as a file, to share a story order or move it to another server. Entries, including placeholders, match by metadata ID, not by file path.

## 11. Access & ownership

- **Collections are server-wide** (decision 4). Admins create and edit them; every user can see them, subject to their library access.
- **Entries a user can't access** are hidden for that user, along with any section left empty. This covers parental controls and per-user library access.
- **Per-user state:** chosen order, progress and the "Show missing titles" setting are stored per user.

## 12. Edge cases

- **A removed title** becomes a placeholder automatically, so its position isn't lost. It returns to normal when the title is added again.
- **New episodes** of a show added as a whole show appear in its entry automatically. Episodes of a show placed by season or range don't move on their own; the editor flags "3 new episodes not placed".
- **A merged or split title** (see the Editions spec) keeps its position in every collection.
- **A placeholder with no metadata match** (for example a fan edit) can be added as a manual placeholder with just a title and optional artwork.
- **Deleting a collection** deletes its orders and placeholders only, never the media.

## 13. Scope

- **V1:**
  - server-wide collections that span libraries
  - mixed movie and TV entries (show, season, range, episode)
  - placeholders for titles not in the library, the "Missing from this collection" view and prefill from TMDb
  - multiple custom and automatic orders, with sections
  - the collection page, card and library display options
  - playback and Up Next in collection order
  - derived progress
  - pinned editions
  - TMDb auto-creation
- **V1.1:**
  - export and import of orders
  - TV suggestions
  - other media entries (albums, books)
- **Later:**
  - personal (per-user) collections
  - nested collections, for example Star Wars → Skywalker Saga
  - community-shared viewing orders

## 14. Open questions

None. All decisions are recorded in the log above.
