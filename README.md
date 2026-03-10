# Tanuki

Self-hosted media vault for videos, images, manga, comics, doujinshi and source downloads.

Tanuki combines a library scanner, downloader, reader/player, tagging workflow and collections UI in a single Docker stack.

## What Tanuki Can Do

- Scan and organize a local library from `/media`
- Import unorganized files through `/inbox`
- Download media from supported public sources into the library
- Browse videos, images, manga, comics and doujinshi in one UI
- Generate thumbnails for local files automatically
- Save reading progress and video resume position
- Edit metadata from the frontend
- Create manual and smart collections
- Find duplicates with perceptual hashing
- Auto-tag media through reverse-image matching
- Run as a multi-user app with authentication

## Current Feature Set

### Library

- Recursive scan of `/media`
- Supported media types:
  - `video`
  - `image`
  - `manga`
  - `comic`
  - `doujinshi`
- Automatic type detection for common video/image/archive formats
- Thumbnail generation for videos and images
- Manual metadata editing in the UI:
  - title
  - date
  - language
  - source URL
  - tags
  - custom cover upload or remote cover URL
- Delete from database only or delete local file too

### Reader and Player

- Custom video player with:
  - click-anywhere play/pause
  - speed control
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

### Downloads

- Queue-based download manager with live progress
- Scheduled downloads
- Batch URL submission
- Automatic post-download organize + library refresh
- Supported public connectors currently include:
  - `hentai0.com` video pages
  - `doujins.com` gallery pages
  - `rule34.art` comic and video pages
  - `danbooru.donmai.us` post pages
  - `safebooru.org` post pages
  - `gelbooru.com` post pages
- Generic tools still available where useful:
  - `yt-dlp`
  - `gallery-dl`
  - HTTP fallback for direct media files

### Tags, Duplicates and Auto-Tagging

- Tag filters and tag search in the library
- Tag counts based on real media usage
- Reverse-image-based auto-tag flow
- Perceptual hash duplicate detection

### Collections

- Manual collections
- Smart collections with rules for:
  - media type
  - title contains
  - tag
  - favorites only
  - minimum rating
- Manual items and automatic rule matches can coexist in the same collection

## Quick Start

### Requirements

- Docker
- Docker Compose

### Start the stack

```bash
git clone https://github.com/nacl-dev/Tanuki.git
cd Tanuki
docker compose up -d --build
```

Then open:

- [http://localhost:8080](http://localhost:8080)

Important notes:

- `media/` and `inbox/` are created automatically by Docker bind mounts
- you do not need to pre-create the library folders manually
- the app runs database migrations automatically on startup

## Default Services

| Service | Purpose | Port |
|---|---|---|
| `app` | API server + frontend | `8080` |
| `worker` | scanner, thumbnails, background processing | - |
| `downloader` | queued download daemon | - |
| `db` | PostgreSQL 16 | internal |
| `cache` | Redis 7 | internal |

## Default Paths

Inside containers:

- media library: `/media`
- inbox: `/inbox`
- thumbnails: `/thumbnails`

On the host:

- library files: `./media`
- intake folder: `./inbox`

Typical library structure created by organize/download flows:

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

## Configuration

The stack works with defaults, but you can override settings through environment variables.

Important variables:

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | App port |
| `DATABASE_URL` | `postgresql://tanuki:secret@db:5432/tanuki?sslmode=disable` | PostgreSQL DSN |
| `REDIS_URL` | `redis://cache:6379` | Redis URL |
| `MEDIA_PATH` | `/media` | Library root |
| `INBOX_PATH` | `/inbox` | Intake/import root |
| `THUMBNAILS_PATH` | `/thumbnails` | Thumbnail storage |
| `DOWNLOADS_PATH` | `/media` | Download staging target root |
| `SECRET_KEY` | `change-me-in-production` | Auth/session secret |
| `JWT_SECRET` | falls back to `SECRET_KEY` | JWT signing key |
| `JWT_EXPIRY_HOURS` | `24` | Login token lifetime |
| `SCAN_INTERVAL` | `300` | Background scan interval in seconds |
| `MAX_CONCURRENT_DOWNLOADS` | `3` | Parallel download jobs |
| `RATE_LIMIT_DELAY` | `1000` | Delay between source requests in ms |
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

## Typical Workflow

### Import existing files

1. Drop files or folders into `./inbox`
2. Use `Scan Library` or the organize flow in the app
3. Tanuki moves or copies them into the library structure
4. The worker scans them and generates thumbnails

### Download from a supported source

1. Open Downloads
2. Paste one or more URLs
3. Watch live progress
4. On completion, files are organized and scanned into the library automatically

### Build a smart collection

Examples:

- all `video` items with title containing `Venus Blood`
- all media tagged `tentacles`
- favorites with rating `4★+`

## API Surface

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

## Project Structure

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

## Notes

- Empty library/runtime folders are intentionally not tracked in Git
- real downloaded media should stay out of Git history
- `media/` and `inbox/` are runtime data, not source files
- some older docs or comments may still refer to earlier paths like `/downloads`; current Docker setup stores into `/media`

## License

[MIT](LICENSE)
