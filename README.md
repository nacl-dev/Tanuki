# Tanuki

Tanuki is a self-hosted media vault for mixed libraries that include videos, images, manga, comics and source downloads. It combines scanning, playback, reading, metadata management, downloads, auto-tagging, duplicate detection and collections in a Docker-based stack.

![Tanuki library preview with blurred covers and names](docs/assets/readme-preview-blurred.png)

Tanuki is designed for self-hosted deployments with a shared library model, background workers, and persistent storage for media, database data and generated assets.

## Features

### Library

- Recursive scan of `/media`
- Automatic media type detection for common video, image and archive formats
- Thumbnail generation for local media
- Metadata editing in the browser
- Optional delete from database only or from disk as well

Supported library media types:

- `video`
- `image`
- `manga`
- `comic`
- `doujinshi`

Supported archive reader formats:

- `.cbz`
- `.zip`
- `.cbr`
- `.rar`

### Reader and Player

- Video playback with resume support
- Manga and comic reader with single-page, double-page, scroll and RTL modes
- Persistent reader and player preferences in the browser

### Downloads

- Queue-based download manager
- Scheduled downloads
- Batch URL submission
- Automatic organize and follow-up scan after completion
- Site-specific connectors plus `yt-dlp`, `gallery-dl` and direct HTTP fallback

Currently supported public connectors:

- `hentai0.com`
- `doujins.com`
- `rule34.art`
- `danbooru.donmai.us`
- `safebooru.org`
- `gelbooru.com`

### Metadata and Organization

- Tag search and tag-based filtering
- Reverse-image auto-tagging
- Duplicate detection via perceptual hash
- Manual collections
- Smart collections based on type, title, tag, favorite flag and minimum rating

### Access Control

- Multi-user authentication
- Admin-only user management
- Admin-only plugin management

## Quick Start

### Requirements

- Docker
- Docker Compose

### First Startup

```bash
git clone https://github.com/nacl-dev/Tanuki.git
cd Tanuki
cp .env.example .env
docker compose up -d --build
```

Open the application:

- [http://localhost:8080](http://localhost:8080)

On first start, the default Compose setup creates `media/` and `inbox/` automatically. If `SECRET_KEY` is left empty, Tanuki generates one and stores it persistently. The first registered user becomes admin.

### Use Prebuilt Images

If you want to skip local image builds, you can use the published GitHub Container Registry images instead:

```bash
git clone https://github.com/nacl-dev/Tanuki.git
cd Tanuki
cp .env.example .env
docker compose -f docker-compose.ghcr.yml up -d
```

This uses the published `tanuki-app`, `tanuki-worker` and `tanuki-downloader` images. By default it pulls `latest`. To pin a specific release, set `IMAGE_TAG` in `.env` before starting the stack.

## Configuration

The stack works with sensible defaults. These are the settings you are most likely to change. See `.env.example` for the full list.

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP port exposed by the app container |
| `MEDIA_PATH` | `/media` | library root |
| `INBOX_PATH` | `/inbox` | import root |
| `DOWNLOADS_PATH` | `/media` | allowed download root |
| `SECRET_KEY` | auto-generated if empty | application secret, minimum 32 characters |
| `REGISTRATION_ENABLED` | `true` | allow self-registration |
| `SCAN_INTERVAL` | `300` | automatic scan interval in seconds |
| `DOWNLOADER_COOKIES_FILE` | empty | optional Netscape `cookies.txt` path |
| `SAUCENAO_API_KEY` | empty | SauceNAO API key for auto-tagging |
| `BASE_URL` | `/` | frontend base path when deploying behind a reverse proxy prefix |

If you access Tanuki behind a reverse proxy subpath, set `BASE_URL` to that prefix before building the frontend.

## Shared Library Model

Tanuki uses a shared-library model with user-specific areas where it makes sense:

- media files and tags are shared across the instance
- collections are user-scoped
- download jobs are user-scoped
- download schedules are user-scoped
- runtime and path details are visible only to admins
- plugins are admin-only

Tanuki does not currently provide strict per-user library isolation.

## Operational Notes

- keep the `config` volume persistent if you use the auto-generated secret
- put the stack behind a reverse proxy with TLS
- keep `.env` out of version control
- back up PostgreSQL and the `media/` volume regularly
- stricter source sites may still require browser-exported cookies

## License

[PolyForm Noncommercial 1.0.0](LICENSE)

Tanuki is source-available for noncommercial use. Commercial use is not permitted under this license.
