# 🦝 Tanuki

> **Self-Hosted Media Vault** – Organize, browse, and download images, videos, manga, doujinshi and comics from a single Docker stack.

---

## ✨ Features

### 📚 Library
- Recursive filesystem scanner with automatic media detection
- Supported types: **Videos** (MP4, MKV, WEBM), **Images** (JPG, PNG, WEBP, GIF), **Archives** (ZIP, CBZ, CBR)
- SHA-256 checksums & perceptual hashing for duplicate detection
- Automatic thumbnail generation via **FFmpeg** (video) and **libvips** (image)

### 🏷️ Tagging
- Booru-style tag categories: `general`, `artist`, `character`, `parody`, `genre`, `meta`
- Autocomplete tag search
- Bulk tagging, tag aliases and implications

### 🖼️ Viewer
- Responsive masonry gallery with lazy loading
- Embedded video player with speed control
- Manga / comic reader with continuous scroll and RTL support
- Dark theme by default (amber/orange accent)

### ⬇️ Download Manager
- Supports **gallery-dl** and **yt-dlp** for thousands of sites
- Direct HTTP file download as fallback
- Real-time progress tracking with pause / resume / cancel
- Cron-based scheduled downloads
- Batch URL submission

---

## 🚀 Quick Start

```bash
# 1. Clone the repository
git clone https://github.com/nacl-dev/Tanuki.git
cd Tanuki

# 2. Copy environment file and adjust values
cp .env.example .env

# 3. Create the media directory and start the stack
mkdir -p media
docker compose up -d

# 4. Open in your browser
open http://localhost:8080
```

---

## 🐳 Services

| Service       | Description                                       | Port  |
|---------------|---------------------------------------------------|-------|
| `app`         | Go API server + Vue 3 frontend                    | 8080  |
| `worker`      | Background scanner, thumbnails, hashing, tagging  | –     |
| `downloader`  | gallery-dl / yt-dlp download daemon               | –     |
| `db`          | PostgreSQL 16                                     | 5432  |
| `cache`       | Redis 7                                           | 6379  |

---

## 🛠️ Tech Stack

| Layer        | Technology                             |
|--------------|----------------------------------------|
| Backend      | Go 1.22, Gin, sqlx, go-redis           |
| Frontend     | Vue 3, Vite, TypeScript, Pinia         |
| Database     | PostgreSQL 16                          |
| Cache/Queue  | Redis 7                                |
| Thumbnails   | FFmpeg + libvips                       |
| Downloads    | gallery-dl + yt-dlp                    |

---

## ⚙️ Configuration

Copy `.env.example` to `.env` and adjust the values:

| Variable                       | Default                                 | Description                                        |
|--------------------------------|-----------------------------------------|----------------------------------------------------|
| `DATABASE_URL`                 | `postgresql://tanuki:secret@db:5432/…`  | PostgreSQL connection string                       |
| `REDIS_URL`                    | `redis://cache:6379`                    | Redis connection string                            |
| `MEDIA_PATH`                   | `/media`                                | Where your media files live                        |
| `THUMBNAILS_PATH`              | `/thumbnails`                           | Generated thumbnails                               |
| `DOWNLOADS_PATH`               | `/downloads`                            | Download destination                               |
| `SECRET_KEY`                   | *(change me!)*                          | Session signing key                                |
| `PORT`                         | `8080`                                  | HTTP port                                          |
| `SCAN_INTERVAL`                | `300`                                   | Auto-scan interval (seconds)                       |
| `MAX_CONCURRENT_DOWNLOADS`     | `3`                                     | Parallel download limit                            |
| `RATE_LIMIT_DELAY`             | `1000`                                  | ms delay between source requests                   |
| `SAUCENAO_API_KEY`             | *(empty)*                               | SauceNAO API key; leave empty to disable           |
| `IQDB_ENABLED`                 | `true`                                  | Enable IQDB fallback for auto-tagging              |
| `AUTOTAG_SIMILARITY_THRESHOLD` | `80`                                    | Minimum match similarity % to accept tags          |
| `AUTOTAG_ON_SCAN`              | `false`                                 | Auto-tag new items after every scan                |
| `AUTOTAG_RATE_LIMIT_MS`        | `5000`                                  | ms between reverse-image-search API calls          |
| `DUPLICATE_THRESHOLD`          | `10`                                    | Max pHash Hamming distance to consider a duplicate |
| `PHASH_ON_SCAN`                | `true`                                  | Compute perceptual hash during library scan        |

---

## 📁 Project Structure

```
Tanuki/
├── backend/
│   ├── cmd/
│   │   ├── server/          # HTTP server entry point
│   │   ├── worker/          # Background worker entry point
│   │   └── downloader/      # Download manager entry point
│   ├── internal/
│   │   ├── api/             # Gin route handlers
│   │   ├── config/          # Environment config loader
│   │   ├── database/        # DB connection & migrations
│   │   ├── downloader/      # Download engines & scheduler
│   │   ├── models/          # sqlx data models
│   │   └── scanner/         # Filesystem scanner
│   └── migrations/          # SQL migration files
├── frontend/
│   └── src/
│       ├── api/             # Axios API clients
│       ├── components/      # Reusable Vue components
│       ├── pages/           # Page-level components
│       ├── router/          # Vue Router
│       └── stores/          # Pinia state stores
├── config/                  # Config templates
├── docs/                    # Documentation
├── docker-compose.yml
├── Dockerfile
└── .env.example
```

---

## 🗺️ Roadmap

| Version | Features | Status |
|---------|----------|--------|
| **v0.1** | Filesystem scan, thumbnails, basic gallery | ✅ Done |
| **v0.2** | Booru-style tag search, filtering, sort options, ratings | ✅ Done |
| **v0.3** | Video player, manga/comic reader | ✅ Done |
| **v0.4** | Auto-tagging via SauceNAO / IQDB | ✅ Done |
| **v0.5** | Perceptual hash duplicate detection | ✅ Done |
| **v0.6** | Multi-user authentication | ✅ Done |
| **v1.0** | Stable release, community plugins | 📋 Planned |

---

## 🤝 Contributing

Contributions are welcome! Please open an issue or PR.

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Commit your changes: `git commit -m 'feat: add my feature'`
4. Push: `git push origin feature/my-feature`
5. Open a Pull Request

---

## 📄 License

[MIT](LICENSE)
