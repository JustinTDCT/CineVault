# Media Server Feature Master List

Every feature found in Plex, Emby and Jellyfin (docs, pricing pages, source code), merged with what users vote for on each project's request board. Researched 30 Sep 2026. One row per feature, each with an ID for planning.

- **Interactive version** (search, filters, sort by votes): open [`feature-master-list.html`](feature-master-list.html) in a browser after downloading.
- **Spreadsheet version:** [`feature-master-list.csv`](feature-master-list.csv).

**Totals:** 164 features · 40 offered by all three · 50 gaps (none of the three fully offers them, marked **Gap**).

**Status key:** Yes · Paid (needs a subscription) · Partial · Plugin · In progress · Planned · Requested (open user request) · No · — (no evidence found either way, not necessarily missing).

Vote counts come from different boards and aren't comparable across them: Plex's forum totals accumulate since 2012, Jellyfin's board was read live on 30 Sep 2026, and Emby figures are forum likes.

## Library & media types

| ID | Feature | Plex | Emby | Jellyfin | Top votes | Notes |
|---|---|---|---|---|---|---|
| LIB-01 | Movie and TV libraries with automatic season and episode grouping | Yes | Yes | Yes | — |  |
| LIB-02 | Music library | Yes | Yes | Yes | — |  |
| LIB-03 | Photo library | Yes | Yes | Yes | — |  |
| LIB-04 | Home videos and other video libraries | Yes | Yes | Yes | — |  |
| LIB-05 | Music videos | Yes | Yes | Yes | — |  |
| LIB-06 | Books, comics and PDF reader | Requested | — | Yes | 3,182 (Plex) | Plex's #1 request since 2013. Jellyfin's web app includes book, comic and PDF readers. |
| LIB-07 | Audiobooks as a full library type | Requested | Yes | In progress | 2,264 (Plex) | Jellyfin: 857 votes, in progress. |
| LIB-08 | Podcasts | — | Paid | Requested | 563 (Jellyfin) |  |
| LIB-09 | Internet radio stations **Gap** | Requested | Plugin | — | 284 (Plex) | Emby via the TuneIn plugin. |
| LIB-10 | HEIC/HEIF photos **Gap** | Requested | — | — | 637 (Plex) |  |
| LIB-11 | Multiple versions or cuts of one movie | Yes | Yes | Partial | 1,459 (Plex) | Implemented in Plex. Jellyfin: better handling across folders requested (265). |
| LIB-12 | Missing and upcoming episodes shown in a series | — | Yes | In progress | 343 (Jellyfin) | Jellyfin's main branch adds a TMDb task that creates placeholder episodes. |
| LIB-13 | Trailers and extras (behind the scenes, deleted scenes) | Yes | Yes | Yes | 585 (Jellyfin) | Jellyfin: online trailers plugin requested. Plex: TV series DVD extras requested (516). |
| LIB-14 | CUE sheet support for albums **Gap** | — | — | Requested | 170 (Jellyfin) |  |

## Metadata & organization

| ID | Feature | Plex | Emby | Jellyfin | Top votes | Notes |
|---|---|---|---|---|---|---|
| MET-01 | Automatic metadata and artwork (TMDb, TVDB, OMDb, MusicBrainz, AudioDB, Fanart.tv) | Yes | Yes | Yes | — |  |
| MET-02 | Local NFO metadata (Kodi style) | No | Plugin | Yes | — |  |
| MET-03 | Custom metadata provider framework | Yes | Plugin | Plugin | 705 (Plex) | Plex providers are standalone web services in any language. External provider IDs request implemented. |
| MET-04 | Anime metadata sources **Gap** | — | Plugin | Plugin | — |  |
| MET-05 | Collections (automatic and manual) | Yes | Yes | Yes | — |  |
| MET-06 | Edit metadata, tags and artwork | Yes | Yes | Yes | — |  |
| MET-07 | Bulk-edit metadata for many items | Yes | Requested | Requested | 186 (Jellyfin) | Emby: 93 likes. |
| MET-08 | Add artwork by URL **Gap** | — | — | Requested | 252 (Jellyfin) |  |
| MET-09 | Keep original title option **Gap** | — | — | Requested | 315 (Jellyfin) |  |
| MET-10 | Metadata in multiple languages **Gap** | — | — | Requested | 212 (Jellyfin) |  |
| MET-11 | Alternate episode orders (DVD, absolute) | Yes | Yes | Yes | — |  |
| MET-12 | Flatten single-season shows | Yes | — | Requested | 170 (Jellyfin) | A second similar request has 160 votes. |
| MET-13 | Hide shows or folders with no video files **Gap** | — | Requested | Planned | 190 (Jellyfin) |  |
| MET-14 | Ignore rules for files and folders | Yes | Yes | Yes | — | Jellyfin 10.11 uses .gitignore-style rules; Plex uses .plexignore. |
| MET-15 | Folder view for libraries | Yes | Yes | Requested | 438 (Jellyfin) |  |
| MET-16 | Robust music tag organization (ID3 and similar) | Requested | Yes | Yes | 866 (Plex) |  |
| MET-17 | Auto-organize downloaded files into the library **Gap** | No | Plugin | No | — |  |
| MET-18 | Search across all libraries | Yes | Yes | Yes | — |  |
| MET-19 | Search within one library **Gap** | — | Requested | — | 30 (Emby likes) | Emby's most-replied open request (130 replies). |
| MET-20 | Search and filter by tag or genre **Gap** | — | — | Requested | 256 (Jellyfin) |  |
| MET-21 | Filter by audio language **Gap** | — | — | Requested | 172 (Jellyfin) |  |
| MET-22 | Continuous scrolling in library views (no pages) **Gap** | — | — | In progress | 1,178 (Jellyfin) |  |

## Playback & streaming

| ID | Feature | Plex | Emby | Jellyfin | Top votes | Notes |
|---|---|---|---|---|---|---|
| PLY-01 | Direct play and on-the-fly transcoding | Yes | Yes | Yes | — |  |
| PLY-02 | Hardware-accelerated transcoding | Paid | Paid | Yes | — | Jellyfin supports NVENC, QSV, VAAPI, AMF, VideoToolbox, V4L2 and RKMPP. Emby is free on Shield and WD NAS. |
| PLY-03 | AMD hardware encoding | Requested | — | Yes | 848 (Plex) |  |
| PLY-04 | HDR tone mapping | Paid | Paid | Yes | — |  |
| PLY-05 | AV1 support | Yes | — | Yes | 534 (Plex) | Implemented in Plex. |
| PLY-06 | HEVC (H.265) transcoding output | Requested | Yes | Yes | 247 (Plex) |  |
| PLY-07 | Pre-transcoded or optimized versions | Yes | Paid | Requested | 979 (Jellyfin) |  |
| PLY-08 | Automatic hardware transcoding self-test **Gap** | — | — | Planned | 423 (Jellyfin) |  |
| PLY-09 | Default remote streaming to maximum quality **Gap** | Requested | — | — | 1,289 (Plex) |  |
| PLY-10 | Faster remote streaming over parallel connections **Gap** | Requested | — | Planned | 231 (Jellyfin) | Plex: 208 votes. |
| PLY-11 | Buffer the main movie during pre-roll **Gap** | Requested | — | — | 589 (Plex) |  |
| PLY-12 | Pre-buffer the next episode **Gap** | — | — | Requested | 222 (Jellyfin) |  |
| PLY-13 | Identify Dolby Atmos, DTS:X and DTS-HD MA correctly **Gap** | Requested | — | — | 701 (Plex) |  |
| PLY-14 | Playback stats overlay (stats for nerds) **Gap** | — | — | Planned | 219 (Jellyfin) | Requested for the Android TV app. |
| PLY-15 | Playback speed control | Yes | Yes | Yes | 1,298 (Plex) | Implemented in Plex. |
| PLY-16 | Skip intro, credits, recap, previews and commercials | Paid | Paid | Yes | 271 (Plex) | Jellyfin segment types: Intro, Outro, Recap, Preview, Commercial. Plex: always-on auto-skip requested. |
| PLY-17 | Seek-bar preview thumbnails | Yes | Yes | Yes | — |  |
| PLY-18 | Cinema mode (trailers and pre-rolls before a movie) | Yes | Paid | Plugin | — |  |
| PLY-19 | “Are you still watching?” prompt | Yes | — | Yes | — |  |
| PLY-20 | Autoplay next episode | Yes | Yes | Yes | — |  |
| PLY-21 | Casting to Chromecast | Yes | Yes | Yes | — |  |
| PLY-22 | DLNA | Yes | Yes | Plugin | — |  |
| PLY-23 | Watch together (synced remote viewing) | Partial | Requested | Yes | 2,875 (Plex) | Plex removed it from native apps in 2025 (web only now). Jellyfin SyncPlay: invite link (266) and cross-account viewing (164) requested. |
| PLY-24 | Audio normalization (volume leveling) | Yes | — | Yes | — |  |

## Subtitles & tracks

| ID | Feature | Plex | Emby | Jellyfin | Top votes | Notes |
|---|---|---|---|---|---|---|
| SUB-01 | Embedded and external subtitles with styling | Yes | Yes | Yes | — |  |
| SUB-02 | Download subtitles from online sources | Yes | Paid | Plugin | — |  |
| SUB-03 | Automatic subtitle sync **Gap** | Requested | — | — | 687 (Plex) |  |
| SUB-04 | Subtitle timing offset | Yes | Yes | Partial | 179 (Jellyfin) | Missing on Android TV. |
| SUB-05 | Per-series subtitle and audio preferences | Yes | — | — | 701 (Plex) | Implemented in Plex. |
| SUB-06 | Better subtitle appearance options **Gap** | — | — | Requested | 188 (Jellyfin) |  |
| SUB-07 | Readable names for audio and subtitle tracks **Gap** | Requested | — | — | 421 (Plex) |  |
| SUB-08 | Profanity filter **Gap** | — | — | Requested | — | Open request on Jellyfin's board. |

## Music

| ID | Feature | Plex | Emby | Jellyfin | Top votes | Notes |
|---|---|---|---|---|---|---|
| MUS-01 | Instant mix and artist radio | Yes | Yes | Yes | — |  |
| MUS-02 | Smart (rule-based) playlists | Yes | Requested | Planned | 588 (Jellyfin) | Also on Emby's tracked list. |
| MUS-03 | Better and shareable playlists **Gap** | Requested | — | — | 1,425 (Plex) |  |
| MUS-04 | Import playlists (M3U) **Gap** | — | Requested | — | — | On Emby's tracked list. |
| MUS-05 | Gapless playback | Yes | Requested | Requested | 647 (Jellyfin) |  |
| MUS-06 | Lyrics (synced display and editor) | Paid | Paid | Yes | 189 (Plex) | Plex: always-visible lyrics requested. |
| MUS-07 | Sound-alike recommendations (sonic analysis) | Paid | — | Partial | — | Jellyfin uses ListenBrainz similar artists. |
| MUS-08 | Music visualizer **Gap** | — | Requested | — | — |  |
| MUS-09 | Lossless (FLAC) playback on Android **Gap** | — | — | Requested | 210 (Jellyfin) |  |
| MUS-10 | Dedicated music app | Yes | — | — | — | Plexamp. Third-party music clients exist for Jellyfin. |

## Accounts & security

| ID | Feature | Plex | Emby | Jellyfin | Top votes | Notes |
|---|---|---|---|---|---|---|
| USR-01 | Multiple users with per-user libraries and parental controls | Yes | Yes | Yes | — |  |
| USR-02 | Quick profile switching | Yes | Yes | In progress | 154 (Jellyfin) |  |
| USR-03 | User groups **Gap** | — | Requested | Requested | 202 (Jellyfin) |  |
| USR-04 | Single sign-on (OIDC / OAuth) **Gap** | Partial | Requested | Planned | 1,191 (Jellyfin) | Centralized login is Emby's top tracked server request. |
| USR-05 | LDAP | No | Paid | Plugin | — |  |
| USR-06 | Two-factor authentication | Yes | Requested | Planned | 1,653 (Plex) | Implemented in Plex. Jellyfin 1,103 votes; Emby's most active thread (459 replies). |
| USR-07 | Sign in on a TV with a code or QR | Yes | — | Yes | 225 (Jellyfin) | Jellyfin Quick Connect exists; QR code is planned. |
| USR-08 | Self-service sign-up or invite page | Yes | — | Requested | 227 (Jellyfin) |  |
| USR-09 | Per-user interface language **Gap** | — | — | Requested | 219 (Jellyfin) |  |
| USR-10 | Works without a cloud account | No | Partial | Yes | — |  |
| USR-11 | No tracking or telemetry | No | — | Yes | — |  |

## Home screen & personal tracking

| ID | Feature | Plex | Emby | Jellyfin | Top votes | Notes |
|---|---|---|---|---|---|---|
| HOM-01 | Continue Watching and Next Up rows | Yes | Yes | Yes | — |  |
| HOM-02 | Remove an item from Continue Watching | Yes | Yes | Planned | 1,724 (Jellyfin) |  |
| HOM-03 | Merge Next Up and Continue Watching **Gap** | — | — | Requested | 202 (Jellyfin) |  |
| HOM-04 | Include playlists and collections in Next Up **Gap** | — | Requested | — | 44 (Emby likes) |  |
| HOM-05 | Customizable home screen sections | Yes | Yes | Planned | 169 (Jellyfin) |  |
| HOM-06 | Watchlist | Yes | Requested | Planned | 1,293 (Jellyfin) |  |
| HOM-07 | Watch history | Yes | — | Planned | 830 (Jellyfin) |  |
| HOM-08 | Personal ratings | Yes | — | Planned | 453 (Jellyfin) |  |
| HOM-09 | IMDb and Rotten Tomatoes ratings on posters | Yes | — | Requested | 196 (Jellyfin) |  |
| HOM-10 | Hide a title from your own view **Gap** | — | — | Requested | — | Open request on Jellyfin's board. |
| HOM-11 | Favorites and played/unplayed marks | Yes | Yes | Yes | — |  |

## Social & discovery

| ID | Feature | Plex | Emby | Jellyfin | Top votes | Notes |
|---|---|---|---|---|---|---|
| SOC-01 | Shareable, importable lists | Yes | — | — | — | Plex, 2026. |
| SOC-02 | Public discussions on titles with spoiler tags | Yes | — | — | — | Plex, June 2026. |
| SOC-03 | Predicted enjoyment score | Yes | — | — | — | Plex Match Score, 2026. |
| SOC-04 | Recommend a title to another user | Yes | Requested | — | — |  |
| SOC-05 | Discover content across streaming services | Yes | No | No | 1,000 (Plex) | The votes were for a way to turn it off (now implemented). |
| SOC-06 | Trakt sync **Gap** | Requested | Plugin | Plugin | 327 (Plex) |  |
| SOC-07 | Last.fm and ListenBrainz scrobbling | Yes | Plugin | Plugin | — |  |

## Remote access & sharing

| ID | Feature | Plex | Emby | Jellyfin | Top votes | Notes |
|---|---|---|---|---|---|---|
| REM-01 | Remote streaming | Paid | Yes | Yes | — | Plex needs Plex Pass or Remote Watch Pass. Jellyfin needs manual setup since 10.11 removed auto port forwarding. |
| REM-02 | Remote access without port forwarding (relay) | Yes | — | No | — |  |
| REM-03 | Automatic server discovery on the home network | Yes | Yes | Partial | — |  |
| REM-04 | Share libraries with friends and family | Yes | Yes | Yes | — |  |
| REM-05 | Temporary share links for a file **Gap** | Requested | — | Requested | 579 (Jellyfin) | Plex: share link for people without accounts (331). |
| REM-06 | Several servers in one app | Yes | Yes | Requested | 177 (Jellyfin) |  |
| REM-07 | Merge or federate libraries from several servers **Gap** | — | Requested | Requested | — | Open requests on Jellyfin's and Emby's boards. |
| REM-08 | Bandwidth limits per user | Yes | Yes | Yes | — |  |
| REM-09 | IPv6 remote access **Gap** | Requested | — | — | 549 (Plex) |  |

## Offline & downloads

| ID | Feature | Plex | Emby | Jellyfin | Top votes | Notes |
|---|---|---|---|---|---|---|
| OFF-01 | Download to phones and tablets | Paid | Paid | In progress | 1,820 (Jellyfin) | Jellyfin's #1 request (Android offline mode). |
| OFF-02 | Offline sync that keeps downloads up to date | Paid | Paid | Planned | 816 (Jellyfin) |  |
| OFF-03 | Download smaller transcoded copies | Paid | Paid | Planned | 517 (Jellyfin) |  |
| OFF-04 | Batch downloads (whole season or album) | Yes | Paid | In progress | 219 (Jellyfin) |  |
| OFF-05 | Downloads on desktop apps | Yes | Paid | — | — |  |

## Live TV & DVR

| ID | Feature | Plex | Emby | Jellyfin | Top votes | Notes |
|---|---|---|---|---|---|---|
| LTV-01 | HDHomeRun tuners | Paid | Paid | Yes | — |  |
| LTV-02 | M3U IPTV playlists | No | Paid | Yes | — |  |
| LTV-03 | Program guide (XMLTV, Schedules Direct, built-in) | Paid | Paid | Yes | — | Emby includes free guide data for the US, Canada and UK. |
| LTV-04 | DVR recording and series rules | Paid | Paid | Yes | — |  |
| LTV-05 | Multiple guide sources for multiple tuners | Requested | — | Yes | 611 (Plex) |  |
| LTV-06 | Channel groups, renaming and reordering **Gap** | Requested | — | Planned | 437 (Plex) | Jellyfin: IPTV channel groups planned (376). |
| LTV-07 | Virtual channels from your own library (PseudoTV) **Gap** | Requested | — | — | 622 (Plex) |  |
| LTV-08 | Skip commercials in recordings | Paid | — | Partial | — |  |
| LTV-09 | Tvheadend and Vu+ backends **Gap** | No | Plugin | Plugin | — |  |

## Apps & devices

| ID | Feature | Plex | Emby | Jellyfin | Top votes | Notes |
|---|---|---|---|---|---|---|
| APP-01 | Web app | Yes | Yes | Yes | — |  |
| APP-02 | Windows, macOS and Linux apps | Yes | Paid | Yes | — | Emby's free desktop playback stops after one minute. |
| APP-03 | iOS and Android apps | Yes | Paid | Yes | — | Emby needs an app unlock or Premiere for full playback. |
| APP-04 | Android TV and Fire TV | Yes | Yes | Yes | — |  |
| APP-05 | Apple TV | Yes | Yes | In progress | 462 (Jellyfin) | Third-party options (Swiftfin, Infuse) exist today. |
| APP-06 | Roku | Yes | Yes | Yes | — |  |
| APP-07 | Samsung and LG smart TVs | Yes | Yes | In progress | 253 (Jellyfin) |  |
| APP-08 | Hisense VIDAA TVs **Gap** | — | — | Requested | 453 (Jellyfin) |  |
| APP-09 | Xbox and PlayStation | Yes | Paid | — | — |  |
| APP-10 | CarPlay and Android Auto | Partial | Paid | — | — |  |
| APP-11 | Apple Watch | — | Paid | — | 182 (Plex) |  |
| APP-12 | VR headsets and 360° video **Gap** | — | — | Requested | 167 (Jellyfin) |  |
| APP-13 | Kodi add-on **Gap** | Plugin | Plugin | Plugin | — |  |
| APP-14 | Camera upload from phones | Paid | Paid | No | — |  |
| APP-15 | Voice assistants (Google, Alexa) | Partial | Paid | Requested | 2,461 (Plex) | Google Home is Plex's #3 request; Jellyfin 243. |
| APP-16 | Remote-friendly TV navigation with a pinned-library sidebar **Gap** | Partial | — | — | — | Plex users demanded the sidebar back after the 2025–26 redesign. |

## Server & admin

| ID | Feature | Plex | Emby | Jellyfin | Top votes | Notes |
|---|---|---|---|---|---|---|
| ADM-01 | Admin dashboard: sessions, activity, logs, devices, API keys | Yes | Yes | Yes | — |  |
| ADM-02 | Stop another user's stream | Yes | Yes | Requested | 402 (Jellyfin) |  |
| ADM-03 | Send messages to users' screens | Requested | Yes | Yes | 1,205 (Plex) |  |
| ADM-04 | Limit concurrent streams | — | Paid | Yes | — |  |
| ADM-05 | Server-wide default settings for users **Gap** | — | — | Requested | 258 (Jellyfin) |  |
| ADM-06 | Let non-admins refresh metadata **Gap** | — | Requested | — | — |  |
| ADM-07 | Scheduled maintenance tasks | Yes | Yes | Yes | — |  |
| ADM-08 | Backup and restore | Partial | Paid | Yes | — | Jellyfin 10.11 lets you pick database, metadata, trickplay and subtitles. |
| ADM-09 | Choice of database (MySQL, PostgreSQL) **Gap** | No | Requested | In progress | 663 (Jellyfin) |  |
| ADM-10 | Usage statistics and reports **Gap** | Partial | Plugin | Plugin | — | Plex users rely on Tautulli. |
| ADM-11 | Custom branding and splash screen | — | — | Yes | — |  |
| ADM-12 | Storage checks and startup status screen | — | — | Yes | — |  |
| ADM-13 | Keep the computer awake while streaming or recording **Gap** | — | — | Requested | 222 (Plex) | Plex: sleep/wake around DVR recordings. Jellyfin: inhibit idle while streaming. |
| ADM-14 | Plugin system with a catalog | Partial | Yes | Yes | — | Plex removed plugins in 2018 and now allows only metadata providers. |

## Integrations

| ID | Feature | Plex | Emby | Jellyfin | Top votes | Notes |
|---|---|---|---|---|---|---|
| INT-01 | Webhooks | Paid | Paid | Plugin | — |  |
| INT-02 | Notifications (email, push) | Yes | Plugin | Plugin | — |  |
| INT-03 | Smart home triggers (lights and scenes) | Partial | Paid | Plugin | 265 (Plex) | Philips Hue integration requested. |
| INT-04 | Public API with documentation | Yes | Yes | Yes | — |  |

## Lessons, not features

- Plex's 2025–26 TV redesign drew heavy backlash: tasks went from about two clicks to six, the top menu suited a mouse more than a remote, and nobody could opt out.
- Removing a loved feature (Watch Together) produced Plex's second-highest request within months. Users asked for polls before features are cut.
- Reliability complaints (resume position, scrubbing, 30-second playback starts) came up as often as new-feature requests.
- Price is a feature: Plex's lifetime pass rose to $749.99 in July 2026, and paywalled basics (remote streaming, hardware transcoding, downloads) are the main reason users switch.

## Sources

- [Jellyfin Feature Requests (most wanted)](https://features.jellyfin.org/?view=most-wanted)
- [Plex Forum: Feature Suggestions by votes](https://forums.plex.tv/c/feature-suggestions/l/votes)
- [Emby: Feature Request Tracking Thread](https://emby.media/community/topic/85654-feature-request-tracking-thread/)
- [Emby Premiere Feature Matrix](https://emby.media/support/articles/Premiere-Feature-Matrix.html)
- [Plex Pass Feature Overview](https://support.plex.tv/articles/201751006-plex-pass-feature-overview/)
- [jellyfin/jellyfin](https://github.com/jellyfin/jellyfin) and [jellyfin/jellyfin-web](https://github.com/jellyfin/jellyfin-web) source code
- [plexinc/tmdb-example-provider](https://github.com/plexinc/tmdb-example-provider)
