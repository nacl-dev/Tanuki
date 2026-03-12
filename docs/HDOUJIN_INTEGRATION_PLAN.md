# HDoujin Integration Plan

## Verified Findings

- The public GitHub repository currently has no declared GitHub license metadata and no root `LICENSE` file exposed through the GitHub contents API.
- The latest verified release on GitHub as of January 7, 2025 ships a Windows `.exe` and `.zip`, not a documented headless CLI or server package.
- The Lua modules are not plain parser snippets. They rely on a custom host API with objects such as `module`, `dom`, `info`, `chapters`, `pages`, `http`, `global`, and sometimes `JavaScript`.
- Some high-value modules, including `Hitomi.lua`, are encrypted/obfuscated rather than directly reusable source files.

## What We Reuse Right Now

- Tanuki now keeps a local mirror of the upstream HDoujin Lua corpus and ships it inside the downloader image.
- The downloader now also includes a constrained first-pass Lua compatibility engine for simple gallery-style modules.
- We still prefer three paths in this order for long-term maintenance:
  1. Use the local Lua compatibility engine when a mirrored module works inside the supported host surface.
  2. Map the site to an existing Tanuki native engine when we already have a better first-class implementation.
  3. Fall back to `gallery-dl` or write a new native connector only when that gives materially better coverage or metadata.

## New Audit Workflow

Tanuki now includes a small audit tool:

```bash
go run ./cmd/hdoujin-audit -modules-dir C:\path\to\HDoujinDownloader\modules\lua -out C:\path\to\hdoujin-audit.json
```

The report extracts:

- declared domains
- whether the module exposes `GetInfo`, `GetChapters`, or `GetPages`
- whether it requires login
- whether it relies on embedded JavaScript
- whether it is encrypted
- a first-pass suggested Tanuki engine or fallback path

## Local Mirror

Tanuki now also ships a local mirror of the upstream Lua module set under:

`backend/third_party/hdoujin/modules/lua`

That gives us two immediate benefits:

- the modules are available inside the Docker image for runtime experiments
- audit, fixture, and compatibility work can run against a stable local corpus instead of remote GitHub calls

Current local snapshot:

- 315 mirrored `.lua` modules
- audit summary from the mirrored corpus: 3 `native-engine`, 3 `gallery-dl-fallback`, 309 `manual-review`

## First Compatibility Pass

The downloader now includes a first HDoujin Lua compatibility engine and wires it into the downloader engine order with this scope:

- loads local modules and executes `Register()`
- matches URLs by declared module domains
- supports a simple HTML/XPath runtime for `GetInfo()`, `GetPages()`, and `BeforeDownloadPage()`
- supports both `dom.Select*` and `page.Select*`
- supports `Dom.New(...)` for HTML-fragment-backed readers
- supports a first JSON runtime via `Json.New`, `json.SelectValue(s)`, and `json.SelectNode(s)`
- supports a minimal `JavaScript.New().Execute("name = <json>")` bridge for sites that expose JSON payloads in the page HTML
- supports collection helpers such as `Count()`, `First()`, `Last()`, and simple iteration over `SelectElements(...)`
- supports `pages/chapters Reverse()` and `Sort()`
- supports a first chapter-series path that downloads one archive per chapter when the root URL only exposes `GetChapters()`
- downloads simple gallery-style page sets into `.cbz`
- collection-backed tag lists now skip `*lua.LFunction` values, preventing `function:0x…` tag leaks
- scanner skips `.tanuki-job-*` staging directories and `.part` files during walks
- runtime helpers: `Fail`, `SetParameter`, `DecodeBase64`, `StripParameters`, `Paginator.New` (in addition to existing `RegexReplace`, `GetRooted`, `GetParameter`, `GetRoot`)
- lightweight test fixtures simulate large-gallery and chapter-series hosts with 3–5 pages for routine validation

Current deliberate limits:

- no full login workflow
- no encrypted/obfuscated modules
- no embedded JavaScript runtime yet (minimal `JavaScript.New().Execute("name = <json>")` bridge only)
- no general POST/form workflow beyond plain GET navigation yet

## Highest-Impact Missing Runtime Surface

Scan results against the 315 mirrored modules show these next priorities:

- chapter/page collection mutators:
  `chapters.Reverse` in 134 modules, `chapters.Sort` in 4, `pages.Reverse` in 5, `pages.Sort` in 1
- JSON runtime:
  `Json.New` in 116 modules, plus `json.SelectValue`, `json.SelectValues`, `json.SelectTokens`, and `json.SelectNodes`
- HTTP write/response helpers:
  `http.Post`, `http.PostResponse`, `http.GetResponse`, `http.Cookies.*`, `response.Cookies`, `response.Body`, and real `global.SetCookies`
- DOM constructors/helpers:
  `Dom.New` in 80 modules, plus `dom.Title`, `dom.SelectNodes`, and `dom.SelectNode`
- common helper functions:
  `RegexReplace`, `Paginator.New`, `Fail`, `SetParameter`, `GetRooted`, `DecodeBase64`, `StripParameters`
- JavaScript / encrypted runtime:
  `JavaScript.New`, `DoEncryptedString`, `JavaScript.Deobfuscate`

That means the next useful runtime batch is not "random more helpers", but:

1. HTTP response/cookie surface
2. richer chapter-list helpers such as broader `chapters.AddRange(...)` patterns
3. remaining DOM helpers like `dom.Title`
4. broader JavaScript runtime only where it unlocks concrete high-value sites

## Suggested Triage Rules

- `native-engine`: only for sites we already support natively, such as Danbooru/Gelbooru-style sources
- `gallery-dl-fallback`: preferred for broad hentai/doujin sites already served well enough by `gallery-dl`
- `manual-review`: only worth native work if the site exposes materially better hentai metadata than the existing fallback path

## Practical Next Step

Use the mirrored corpus and the local runtime together:

1. prioritize non-encrypted, non-login, non-JS hentai modules that already expose `GetInfo()` + `GetPages()`
2. expand collection mutators and chapter helpers next, because `chapters.Reverse` and related calls appear in a very large share of modules
3. add JSON runtime support right after that, because `Json.New` is the biggest missing object surface
4. only then decide whether the next wave should be HTTP response/post helpers or `Dom.New`, depending on which target sites we want first
5. leave encrypted/login/JavaScript-heavy modules for last

That keeps us moving toward broad site coverage without rebuilding every source as a separate native connector first.
