# Metadata And Downloads TODO

## Current Call

- [x] Evaluate `HDoujinDownloader` as a possible replacement for Tanuki's downloader stack
- [x] Add first-pass namespace tag support to imports, manual editing, filters, and smart collections
- [x] Start emitting namespaced tags from connectors that already expose category information

## Decision Log

- [x] Keep Tanuki's downloader as the primary path for now
  Reason: Tanuki's downloader is already queue-driven, headless, and wired into jobs, schedules, sidecars, scans, and thumbnails.
- [x] Treat `HDoujinDownloader` as a reference implementation, not a drop-in replacement
  Reason: it appears strongest as a desktop-oriented downloader/organizer with broad site coverage, while Tanuki needs server-friendly automation and direct integration.
- [x] Revisit the earlier HDoujin caution and allow a constrained local mirror/runtime
  Reason: we now keep the full Lua corpus local and can execute a limited, auditable compatibility layer for simple modules, while still treating encrypted/login/JS-heavy modules as out of scope for now.
- [x] Keep `media.work_title` / `media.work_index` as the active work model for now
  Reason: the current feature set still does not need shared work covers, stable work IDs, or rename propagation across multiple items.
- [x] Do not pursue an `HDoujinDownloader` bridge right now
  Reason: the latest verified GitHub release exposes a Windows `.exe` / `.zip`, but not a documented headless CLI or server-oriented export/import workflow.

## Implemented In This Pass

- [x] `namespace:value` tags from sidecars now map into Tanuki categories during scan/import
- [x] Manual media tag editing now understands `artist:foo`, `series:bar`, `rating:explicit`, and similar inputs
- [x] Media filtering and smart collection tag matching now accept namespace syntax while remaining backward-compatible with legacy raw tags
- [x] Booru and Danbooru connectors now emit namespaced tags when the source already distinguishes artists, characters, series, and meta tags
- [x] `yt-dlp` metadata now emits richer structured tags such as artist, series, genre, language, and uploader when the extractor exposes those fields
- [x] `rule34art` and `porncomics` now emit more conservative structured tags, using `artist`, `genre`, `language`, and `site` namespaces instead of flattening every source section into raw tags
- [x] `image_gallery` (`doujins.com`) and `hentai0` now emit conservative structured tags too, using `site:` plus namespaced `genre:` values instead of only raw tag strings
- [x] Tag category promotion now prefers more specific metadata over weaker `general`/`meta` categories
- [x] Tag aliases and implications now resolve through the API, manual edits, imports, auto-tagging, and smart-collection filters
- [x] The Tags page can now manage aliases and implications directly, instead of leaving rule maintenance to SQL or ad-hoc edits
- [x] Smart collections now canonicalize `auto_tag` expressions on save, so alias inputs persist as stable canonical filters
- [x] Tag-rule and collection forms now use category-aware tag suggestions, so namespace-aware metadata is easier to enter consistently
- [x] Media detail editing now uses a structured tag chip editor instead of raw comma-only textarea input
- [x] `media.work_title` and `media.work_index` now provide a first explicit Work/Series grouping layer without introducing a separate works table
- [x] Media detail editing and the main Library view now expose those work fields for manual grouping and grouped browsing
- [x] Downloader-written `.tanuki.json` sidecars now infer `work_title` and `work_index` conservatively from episodic titles, with explicit support from `yt-dlp` and `hentai0` when upstream metadata already exposes series/episode structure
- [x] The download queue now surfaces prefetched source titles and detected work hints, so grouped-series metadata is visible before scan/import finishes
- [x] `gallery-dl`-style `.json` sidecars now get translated into Tanuki import metadata during organize/import, and the scanner can read them directly on manual library drops
- [x] External JSON sidecars now understand more extractor-specific gallery-dl schemas, including nested namespaced tag maps plus chapter/volume/uploader-style fields for richer work hints and structured tags
- [x] Automatic work metadata extraction now handles richer chapter and volume-style titles such as `Chapter 12 - Title` and `Vol. 2 Ch. 14`, instead of only simple episodic suffixes
- [x] Automatic work metadata extraction now also understands common release/video naming such as `S02E05`, bracketed release prefixes like `[Studio] Title 03v2`, and similar series-style filenames
- [x] Download creation now exposes structured auto-tags with tag suggestions, stores them on jobs, and applies them automatically to imported media after scan
- [x] Schedule creation now supports structured default tags with tag suggestions, stores canonicalized expressions, and applies them to the queued jobs triggered by the scheduler
- [x] Inbox uploads now accept structured default tags, write companion `.tanuki.json` metadata for uploaded media, and preserve companion sidecars through the organize step instead of leaving them behind in staging
- [x] Added a first HDoujin fixture manifest plus routing test for already covered sites, so future module-to-connector translation work has concrete native-engine checkpoints
- [x] Auto-tag review now normalizes namespace expressions and accepts manual `namespace:value` additions, so the last remaining metadata-ingest flow matches the newer tag helper patterns
- [x] The HDoujin fixture manifest now also includes gallery-dl fallback candidates for high-value hentai/doujin sites such as FAKKU and Hitomi, so connector translation work can be prioritized without embedding the Lua runtime
- [x] Added a first-pass `hdoujin-audit` tool that scans a checked-out HDoujin Lua module directory, extracts domains/runtime flags, and suggests native-vs-fallback coverage paths before we write any new connector
- [x] Corrected the HDoujin fixture examples so they only reference real upstream module names instead of mismatched site/module pairs
- [x] Mirrored the full upstream HDoujin Lua corpus locally and wired a first-pass compatibility engine into the downloader for simple gallery-style modules
- [x] Video quick preview has been rebuilt as a teleported full-screen overlay with a proper large player layout, instead of a card-bound mini player embedded inside the capsule
- [x] Video cards no longer render the generic media placeholder when a thumbnail is missing; they stay visually clean and rely on the preview flow instead
- [x] Library organize now preserves large pure-image folders as gallery/doujin directory moves instead of flattening them into individual image files

## Next Steps

- [x] Add first-pass tag merge tooling with preview and impact counts before destructive metadata changes
- [x] Expand the same structured tag helper patterns into any remaining ingest/admin flows beyond downloads, schedules, and inbox uploads
- [x] Continue expanding external metadata parsing where additional hentai-/gallery-oriented sidecar schemas provide materially better work/title/tag structure than the initial gallery-dl coverage
- [x] Expand automatic work metadata extraction beyond episodic title patterns into richer chapter/volume heuristics and connector-specific metadata where sources expose it cleanly
- [x] Promote the first-pass work fields into a dedicated works model only if shared cover metadata, rename propagation, or stable work IDs become necessary
- [x] Revisit remaining download connectors where the source already exposes enough structured metadata to justify additional namespace mapping
- [x] Revisit remaining download connectors again only if a new source exposes materially richer structured metadata than the current set
- [x] Expand the HDoujin fixture manifest beyond already covered native engines and use it to drive the next connector translations without embedding the Lua runtime
- [x] Use the expanded HDoujin fixture manifest to pick and translate the next highest-value native connector instead of adding modules ad hoc
- [x] Revisit `HDoujinDownloader` only if a stable headless/CLI workflow or export/import bridge can be verified
- [x] If module reuse remains interesting, prototype a compatibility layer only after licensing is clarified and the minimal host API surface is mapped

## HDoujin Runtime Progress

- [x] Mirror the full upstream `modules/lua` corpus into the repo under `backend/third_party/hdoujin/modules/lua`
- [x] Ship the mirrored HDoujin corpus inside the Docker image and wire a first-pass Lua compatibility engine into the downloader engine order
- [x] Support the most common simple runtime surface for local modules:
  `Register()`, `GetInfo()`, `GetPages()`, `BeforeDownloadPage()`, `dom.Select*`, `page.Select*`, collection `Count()/First()/Last()`, and simple `pages.AddRange(...)`
- [x] Verify the first compatibility pass with Go tests for metadata fetch and archive download against local fixture modules
- [x] Run an audit snapshot against the mirrored corpus
  Current snapshot: 315 local Lua modules, 3 native-engine matches, 3 gallery-dl-first matches, 309 modules still in manual-review because they need broader host-surface coverage or better static classification
- [x] Add first-pass high-impact collection mutators: `chapters.Reverse`, `chapters.Sort`, `pages.Reverse`, `pages.Sort`, plus placeholder `pages.Referer` / `pages.Headers`
- [x] Add a first JSON runtime surface: `Json.New`, `json.SelectValue(s)`, `json.SelectNode(s)`, and a minimal `JavaScript.New().Execute("name = <json>")` bridge for JSON-backed page data
- [x] Add a first chapter-series download path so manga root URLs can emit one archive per chapter instead of failing outright on `GetChapters()`
- [x] Add `Dom.New(...)` plus DOM aliases like `SelectNode(s)` and attribute reads such as `SelectValue("@href")`
- [x] Expand chapter-oriented helpers further so `chapters.AddRange(dom/page.SelectElements(...))` and related chapter-list modules work for more real-world sites
  Added `First()`, `Last()`, `Clear()`, and `FilterDuplicates()` for both chapters and pages collections, covering the most common module patterns for duplicate filtering, conditional chapter rebuilding, and chapter introspection
- [x] Add the smallest useful non-login HTTP form/post helpers after that: `http.Post`, `http.PostResponse`, `http.GetResponse`, response cookies/body, and real `global.SetCookies`
- [x] Add the remaining small DOM helpers like `dom.Title` once the collection+JSON+HTTP layers are in place
- [ ] Add `chapters.SelectElements()`-style collection constructors that accept CSS-like selectors in addition to XPath, since some modules use mixed selector syntax
- [ ] Add pagination helpers (`chapters.AddPage`, `chapters.NextPage`) for modules that load chapter lists across multiple pages instead of a single DOM
- [ ] Revisit a minimal `JavaScript` host surface only for concrete high-value hentai sites after the non-JS module set is exhausted
- [ ] Keep encrypted or login-heavy modules out of the executable runtime until the host API surface is mapped more safely

## Live Validation 2026-03-12
- [x] End-to-end validated the HDoujin compatibility path against 5 real hosts: manhwa18.net, hentai18.net, rokuhentai.com, boards.4chan.org, and asmhentai.com
- [x] Closed the first useful non-login HTTP helper batch for live modules: http.Get, http.Post, http.GetResponse, http.PostResponse, response cookies and body, and global.SetCookies
- [x] Closed the next small DOM helper batch for live modules: dom.Title, Dom.New, and dom.New compatibility on DOM instances
- [x] Confirmed chapter-list helpers now work on real sites that depend on chapters.AddRange and chapters.Reverse
- [x] Follow up on two live-run findings: collection-backed tag lists must ignore helper methods when converting tags, and large story hosts should be sanity-checked with smaller fixtures during future validation
  Fixed: `tableStrings()` and `luaCollectionValues()` now skip `lua.LTFunction` entries so collection helper methods never leak into tag/metadata strings; added 9 focused test fixtures covering the bug fix, new chapter helpers, chapter-series parsing, metadata extraction, empty/single-chapter edge cases, and reversed ordering
