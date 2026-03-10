# Download Manager Guide

## Overview

Tanuki's download manager uses three engines in priority order:

| Engine       | Use case                                          |
|--------------|---------------------------------------------------|
| **yt-dlp**   | Video sites: YouTube, Vimeo, Twitch, Pornhub, …  |
| **gallery-dl** | Image galleries: Pixiv, DeviantArt, nhentai, … |
| **HTTP**     | Direct file URLs (fallback)                       |

The correct engine is selected automatically based on the URL.

## Adding Downloads

### Via the Web UI

1. Open **Downloads** in the sidebar.
2. Paste a URL in the input field.
3. Optionally set a target directory and auto-tags.
4. Click **Download**.

### Via the API

```bash
curl -X POST http://localhost:8080/api/downloads \
  -H 'Content-Type: application/json' \
  -d '{"url": "https://www.pixiv.net/en/artworks/123456"}'
```

### Batch

```bash
curl -X POST http://localhost:8080/api/downloads/batch \
  -H 'Content-Type: application/json' \
  -d '{"urls": ["https://…", "https://…"]}'
```

## Scheduled Downloads

Use cron expressions to automatically download from a URL on a schedule:

| Expression    | Meaning              |
|---------------|----------------------|
| `0 3 * * *`   | Every day at 03:00   |
| `0 */6 * * *` | Every 6 hours        |
| `0 0 * * 0`   | Every Sunday at 00:00|

## Configuration

### gallery-dl

Copy `config/gallery-dl.example.conf` to `config/gallery-dl.conf` (inside the `config` volume). See the [gallery-dl documentation](https://github.com/mikf/gallery-dl) for all options.

### yt-dlp

Copy `config/yt-dlp.example.conf` to `config/yt-dlp.conf`. See the [yt-dlp documentation](https://github.com/yt-dlp/yt-dlp) for all options.

## Job States

```
queued → downloading → processing → completed
                    ↘            ↘
                     failed       paused
```

- **queued** – Waiting to be picked up by the worker.
- **downloading** – Actively downloading files.
- **processing** – Post-processing (metadata, thumbnails).
- **completed** – All files downloaded successfully.
- **failed** – An error occurred (see `error_message`). Can be retried.
- **paused** – Manually paused by the user.

## Concurrency & Rate Limiting

Set `MAX_CONCURRENT_DOWNLOADS` (default: 3) and `RATE_LIMIT_DELAY` (default: 1000 ms) in `.env` to avoid getting rate-limited by sources.
