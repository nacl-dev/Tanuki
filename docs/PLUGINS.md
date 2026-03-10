# Plugin Development Guide

## Overview

Tanuki supports **Python-based scraper plugins** for fetching metadata from external sources. Each plugin is a standalone `.py` file that the worker discovers at startup and makes available through the Plugin API.

## Plugin Interface

Each plugin is a Python module placed in `/app/config/plugins/` that exposes:

```python
# plugins/my_source.py

SOURCE_NAME = "my_source"
SOURCE_URL  = "https://my-source.example.com"
VERSION     = "1.0.0"

def can_handle(url: str) -> bool:
    """Return True if this plugin can process the given URL."""
    return "my-source.example.com" in url

def fetch_metadata(url: str) -> dict:
    """
    Fetch metadata for the given URL.

    Returns a dict with optional keys:
      title:       str
      tags:        list[str]   – namespace:value format, e.g. "artist:foo"
      description: str
      language:    str
      source_url:  str
      extra:       dict        – arbitrary extra metadata
    """
    ...
```

## Example: SauceNAO

```python
import requests

SOURCE_NAME = "saucenao"
SOURCE_URL  = "https://saucenao.com"
VERSION     = "1.0.0"

def can_handle(url: str) -> bool:
    return url.startswith("http")

def fetch_metadata(url: str) -> dict:
    api_key = "YOUR_SAUCENAO_API_KEY"
    r = requests.get(
        "https://saucenao.com/search.php",
        params={"url": url, "api_key": api_key, "output_type": 2},
        timeout=10,
    )
    data = r.json()
    results = data.get("results", [])
    if not results:
        return {}

    best = results[0]
    header = best.get("header", {})
    return {
        "title":    header.get("title", ""),
        "tags":     [],
        "extra":    {"similarity": header.get("similarity")},
    }
```

## Installing Plugins

1. Place your plugin file in the `plugins` volume at `plugins/<name>.py`.
2. In the web UI, navigate to **Plugins** and click **Scan for Plugins**.
3. The plugin will appear in the list and can be enabled/disabled with a toggle.

Alternatively, restart the `worker` service: `docker compose restart worker`.

## Plugin Management API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET`  | `/api/plugins` | List all installed plugins |
| `POST` | `/api/plugins/scan` | Re-scan the plugins directory |
| `PATCH`| `/api/plugins/:id` | Enable/disable a plugin (`{"enabled": true}`) |
| `DELETE`| `/api/plugins/:id` | Remove a plugin (deletes file and DB record) |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PLUGINS_PATH` | `/app/config/plugins` | Directory where plugin `.py` files are stored |
| `PLUGINS_ENABLED` | `true` | Enable or disable the plugin system entirely |

## Roadmap

- [x] Auto-tagging using SauceNAO / IQDB reverse image search *(built-in as of v0.4)*
- [x] Perceptual hash duplicate detection *(built-in as of v0.5)*
- [x] Plugin system with management UI *(v1.0)*
- [ ] Metadata scraping from nhentai / e-hentai
- [ ] Tag implication & alias resolution
- [ ] Community plugin registry
