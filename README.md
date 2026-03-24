# Ember (TUI Emby Client)

Ember is a terminal-first Emby client for browsing libraries and playing media from a TUI.

## Features

- Library browsing for movies, series, seasons, and episodes
- Continue Watching, Favorites, and History sections
- Advanced search filters (type, played/unplayed, favorite-only, year)
- Favorite management from list view
- MPV playback integration with resume support
- Multi-server management inside the TUI

## Requirements

- Go (latest stable recommended)
- [mpv](https://mpv.io/) installed and available in `PATH`

## Quick Start

Run directly:

```bash
go run .
```

On first launch, open server management in the TUI and add your Emby server.

## Build and Install

This repository includes a minimal `Makefile`:

```bash
make build
make install
make clean
```

Default install path:

- `~/.local/bin/ember`

Override install prefix if needed:

```bash
make install PREFIX=/usr/local
```

## Useful Keys (TUI)

- `1` Continue
- `2` Favorites
- `3` History
- `4` or `/` Advanced Search
- `p` Play current item
- `R` Replay current item from beginning
- `f` Toggle favorite
- `a` Add favorite
- `u` Remove favorite
- `m` Server management
- `q` Quit

## Screenshots

![Screenshot 1](image.png)
![Screenshot 2](image2.png)
