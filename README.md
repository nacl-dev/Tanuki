# 🦝 Tanuki

> **Self-hosted media vault** for videos, images, manga, comics, doujinshi and source downloads.

Tanuki brings library management, downloading, tagging, reading and playback together in one Docker-based stack.

## ✨ Overview

Tanuki is built for collections that are:

- mixed across multiple media types
- partially unorganized
- spread across local files and public source sites
- meant to be browsed, edited and resumed from the browser

It combines:

- a recursive media library scanner
- a download queue with site-specific connectors
- a video player and manga/comic reader
- metadata editing and thumbnail management
- reverse-image auto-tagging
- duplicate detection
- manual and smart collections
- multi-user authentication

## 🚀 Highlights

| Area | What you get |
|---|---|
| 📚 Library | Scan `/media`, detect media types, generate thumbnails, edit metadata |
| 📥 Intake | Import unorganized files from `/inbox` |
| 🎬 Viewer | Custom video player, manga/comic reader, fullscreen, resume/progress |
| ⬇️ Downloads | Queue, schedules, live progress, automatic organize + scan |
| 🏷️ Tags | Filtering, search, counts, auto-tagging |
| 🧩 Duplicates | Perceptual hash duplicate detection |
| 📦 Collections | Manual collections and rule-based smart collections |
| 🔐 Auth | Multi-user login and protected API routes |

## 🧰 Feature Set

### 📚 Library

- Recursive scan of `/media`
- Supported media types:
  - `video`
  - `image`
  - `manga`
  - `comic`
  - `doujinshi`
- Automatic type detection for common video, image and archive files
- Archive reader support for `.cbz`, `.zip`, `.rar` and `.cbr` through `bsdtar`
- Automatic thumbnail generation for local media
- Frontend metadata editing:
  - title
  - date
  - language
  - source URL
  - tags
  - custom cover upload
  - remote cover URL
- Delete from database only or delete local file too

### 🎬 Reader and Player

- Custom video player with:
  - click-anywhere play/pause
  - speed controls
  - fullscreen
  - saved resume position
- Manga/comic reader with:
  - single page
  - double page
  - scroll mode
  - RTL mode
  - zoom controls
  - fullscreen
  - saved read progress

### ⬇️ Downloads

- Queue-based download manager
- Live progress updates
- Scheduled downloads
- Batch URL submission
- Automatic organize after download
- Automatic library refresh after completion

Currently supported public connectors:

- `hentai0.com` video pages
- `doujins.com` gallery pages
- `rule34.art` comic and video pages
- `danbooru.donmai.us` post pages
- `safebooru.org` post pages
- `gelbooru.com` post pages

Generic tools still available where useful:

- `yt-dlp`
- `gallery-dl`
- HTTP fallback for direct media files

### 🏷️ Tags, Duplicates and Auto-Tagging

- Tag filters and tag search in the library
- Real usage-based tag counts
- Reverse-image-based auto-tag flow
- Perceptual hash duplicate detection

### 📦 Collections

- Manual collections
- Smart collections with rules for:
  - media type
  - title contains
  - tag
  - favorites only
  - minimum rating
- Manual items and automatic matches can coexist in the same collection

## ⚡ Quick Start

### Requirements

- Docker
- Docker Compose

### Start

```bash
git clone https://github.com/nacl-dev/Tanuki.git
cd Tanuki
docker compose up -d --build
```

Open:

- [http://localhost:8080](http://localhost:8080)

### Good to know

- `media/` and `inbox/` are created automatically by Docker bind mounts
- you do not need to create the folder structure manually
- database migrations run automatically on startup

## 🐳 Services

| Service | Purpose | Port |
|---|---|---|
| `app` | API server + frontend | `8080` |
| `worker` | scanner, thumbnails, background processing | - |
| `downloader` | queued download daemon | - |
| `db` | PostgreSQL 16 | internal |
| `cache` | Redis 7 | internal |

## 📁 Paths

### Inside containers

- media library: `/media`
- inbox: `/inbox`
- thumbnails: `/thumbnails`
- downloads: `/media`

### On the host

- library files: `./media`
- intake folder: `./inbox`

### Typical library structure

```text
media/
  Video/
    2D (Hentai)/
    3D (Real)/
  Image/
    CG Sets/
    GIFs/
    Random/
  Comics/
    Manga/
    Doujins/
```

## ⚙️ Configuration

The stack works with defaults, but environment variables can override runtime behavior.

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | App port |
| `DATABASE_URL` | `postgresql://tanuki:secret@db:5432/tanuki?sslmode=disable` | PostgreSQL DSN |
| `REDIS_URL` | `redis://cache:6379` | Redis URL |
| `MEDIA_PATH` | `/media` | Library root |
| `INBOX_PATH` | `/inbox` | Intake/import root |
| `THUMBNAILS_PATH` | `/thumbnails` | Thumbnail storage |
| `DOWNLOADS_PATH` | `/media` | Download target root |
| `SECRET_KEY` | `change-me-in-production` | App/auth secret |
| `JWT_SECRET` | falls back to `SECRET_KEY` | JWT signing key |
| `JWT_EXPIRY_HOURS` | `24` | Login token lifetime |
| `SCAN_INTERVAL` | `300` | Background scan interval in seconds |
| `MAX_CONCURRENT_DOWNLOADS` | `3` | Parallel download jobs |
| `RATE_LIMIT_DELAY` | `1000` | Delay between source requests in ms |
| `DOWNLOADER_COOKIES_FILE` | empty | Optional Netscape `cookies.txt` for sources behind browser checks |
| `YTDLP_IMPERSONATE` | `chrome` | yt-dlp impersonation target for stricter sources |
| `SAUCENAO_API_KEY` | empty | SauceNAO support for auto-tagging |
| `IQDB_ENABLED` | `true` | IQDB fallback |
| `AUTOTAG_SIMILARITY_THRESHOLD` | `80` | Auto-tag confidence threshold |
| `AUTOTAG_ON_SCAN` | `false` | Auto-tag during scan |
| `AUTOTAG_RATE_LIMIT_MS` | `5000` | Auto-tag request spacing |
| `DUPLICATE_THRESHOLD` | `10` | pHash duplicate threshold |
| `PHASH_ON_SCAN` | `true` | Compute pHash on scan |
| `REGISTRATION_ENABLED` | `true` | Allow self-registration |
| `PLUGINS_ENABLED` | `true` | Plugin system toggle |
| `PLUGINS_PATH` | `/app/config/plugins` | Plugin folder |

## 🛠️ Typical Workflows

### Import existing files

1. Drop files or folders into `./inbox`
2. Trigger `Scan Library` or use the organize flow
3. Tanuki moves or copies them into the library structure
4. The worker scans them and generates thumbnails

### Download from a supported source

1. Open Downloads
2. Paste one or more URLs
3. Watch live progress
4. On completion, files are organized and scanned into the library automatically

For stricter sources protected by Cloudflare or browser verification, export a Netscape-format `cookies.txt` file from your browser and set `DOWNLOADER_COOKIES_FILE` to a path that is mounted into the `downloader` container, for example `/media/.cookies/rule34.txt`.

### Build a smart collection

Examples:

- all `video` items with title containing `Venus Blood`
- all media tagged `tentacles`
- favorites with rating `4★+`

## 🔌 API Surface

Main authenticated API groups:

- `/api/auth`
- `/api/media`
- `/api/collections`
- `/api/tags`
- `/api/downloads`
- `/api/schedules`
- `/api/library`
- `/api/duplicates`
- `/api/plugins`

Health endpoints:

- `/healthz`
- `/api/health`

## 🗂️ Project Structure

```text
Tanuki/
  backend/
    cmd/
      server/
      worker/
      downloader/
    internal/
      api/
      auth/
      autotag/
      config/
      database/
      dedup/
      downloader/
      models/
      plugins/
      scanner/
      thumbnails/
    migrations/
  frontend/
    src/
      api/
      components/
      pages/
      router/
      stores/
  docs/
  media/
  inbox/
  docker-compose.yml
  Dockerfile
```

## 📝 Notes

- `media/` and `inbox/` are runtime data, not source files
- empty runtime folders are intentionally not tracked in Git
- downloaded media should not be committed
- current Docker defaults write downloads into `/media`; use `DOWNLOADS_PATH` only if you want a different allowed root

## 👥 Multi-user Behavior

Current behavior is mixed by product area:

- media files and tags behave as one shared library
- collections are user-scoped
- download jobs and schedules are user-scoped
- authentication controls access and admin-only actions
- plugins are admin-only
- `owner_id` exists in the schema for future evolution, but is not part of the active product model today

If you need hard per-user library isolation, plan that as a dedicated follow-up instead of assuming it today.

## 📄 License

[MIT](LICENSE)
