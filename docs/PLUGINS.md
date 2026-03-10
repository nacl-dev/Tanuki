# Plugin Development Guide

> **Note:** The plugin system is planned for a future release. This document describes the intended API.

## Overview

Tanuki will support Python-based scraper plugins for fetching metadata from external sources (SauceNAO, IQDB, e-hentai, etc.).

## Plugin Interface

Each plugin is a Python module placed in `/app/config/plugins/` that exposes:

```python
# plugins/my_source.py

SOURCE_NAME = "my_source"
SOURCE_URL   = "https://my-source.example.com"

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

1. Place your plugin file in the `config` volume at `plugins/<name>.py`.
2. Restart the `worker` service: `docker compose restart worker`.

## Roadmap

- [ ] Auto-tagging using SauceNAO / IQDB reverse image search
- [ ] Metadata scraping from nhentai / e-hentai
- [ ] Tag implication & alias resolution
- [ ] Community plugin registry
