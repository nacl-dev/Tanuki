# Public Sources Roadmap

Tanuki prioritizes public sources that do not require logins, paid memberships, or browser session scraping.

## Current Direction

We explicitly prefer sources with:

- public media access
- stable metadata
- predictable URLs
- no account requirement

## Priority Order

1. `Danbooru`-style sources
   Why: public JSON metadata, strong tag taxonomy, simple single-post downloads.

2. `Hitomi`
   Why: strong doujin/manga metadata and good category coverage.
   Note: download hosts currently need extra runtime handling because their image hosts are not resolving reliably in our container environment.

3. `Pixiv` public pages
   Why: strong artist and illustration coverage.
   Constraint: only truly public pages; no login or cookie-dependent flows.

4. `E-Hentai` / public archive readers
   Why: broad doujin and gallery coverage.
   Constraint: only public-access flows that work without authentication.

5. Generic booru adapters
   Why: one connector architecture can cover multiple public boards over time.

## Out of Scope

For now, Tanuki does not target:

- FANBOX
- Fantia
- Patreon
- DLsite libraries
- any connector that depends on user logins, paid entitlements, or account cookies

## Connector Rules

Every public connector should ideally provide:

- `CanHandle(url)`
- `FetchMetadata(url)`
- `Download(ctx, job)`
- sidecar metadata via `.tanuki.json`

That metadata should map into Tanuki's library model with:

- `title`
- `source_url`
- `poster_url` when available
- normalized `tags`

## Implemented Public Connectors

- `hentai0.com` video pages
- `doujins.com` image-gallery manga pages
- `rule34.art` comic and video pages
- `danbooru.donmai.us` post pages
- `safebooru.org` and `gelbooru.com` post pages
