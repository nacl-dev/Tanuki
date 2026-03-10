# Setup Guide

## Prerequisites

- Docker & Docker Compose (v2.x)
- At least 2 GB of free RAM
- A directory containing your media files

## Quick Start

```bash
git clone https://github.com/nacl-dev/Tanuki.git
cd Tanuki

# Copy and edit environment variables
cp .env.example .env
# Edit .env: set SECRET_KEY to a long random string

# Mount your media directory
mkdir -p media

# Start all services
docker compose up -d

# Watch logs
docker compose logs -f app
```

Open **http://localhost:8080** in your browser.

## Mounting Media

Edit `docker-compose.yml` to point to your media path:

```yaml
volumes:
  - /path/to/your/media:/media:ro
```

## Persistent Volumes

| Volume       | Purpose                              |
|--------------|--------------------------------------|
| `pgdata`     | PostgreSQL data                      |
| `thumbnails` | Generated thumbnail cache            |
| `downloads`  | Downloaded files                     |
| `config`     | gallery-dl / yt-dlp config files     |
| `redisdata`  | Redis AOF persistence                |

## Updating

```bash
docker compose pull
docker compose up -d
```

## Config Files

Copy the example configs into the `config` volume (usually `./config/`):

```bash
cp config/gallery-dl.example.conf config/gallery-dl.conf
cp config/yt-dlp.example.conf     config/yt-dlp.conf
```

## Ports

Only port **8080** is exposed by default.  
If you use a reverse proxy (Nginx, Traefik), bind only to localhost:

```yaml
ports:
  - "127.0.0.1:8080:8080"
```
