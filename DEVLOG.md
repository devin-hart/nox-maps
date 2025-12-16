# NOX MAPS - DEVELOPMENT LOG
**Date:** December 16, 2025
**Project:** Real-time Map Overlay for EverQuest (Project 1999)
**Stack:** Go (Golang), Ebitengine (2D Graphics)

## 1. Project Overview
Nox Maps is a standalone, external map overlay application for Project 1999. It reads the EverQuest log file in real-time to determine player position (`/loc`), calculates heading based on movement vectors, and renders standard EQ map files (`.txt` vector format).

## 2. Core Architecture
The application is split into three distinct modules:
1.  **Log Reader (`internal/eqlog`):** Polls the EQ directory for the active character's log file. Handles file rotation (character switching) and real-time tailing.
2.  **Parser (`internal/parser`):** Regex engine that converts raw log lines into game state (X, Y, Z, Zone, Heading, Death status).
3.  **UI/Renderer (`internal/ui`):** Ebitengine implementation that handles drawing, zooming, panning, and input processing.

## 3. Implemented Features

### Navigation & Tracking
* **Player Position:** Updates via `/loc` spam (requires macro).
* **Heading/Direction:** Since logs do not provide heading, we calculate it using `Math.Atan2(dy, dx)` between the current and previous coordinate read.
* **Breadcrumb Trail:** Draws a cyan trail of recent movement. Toggleable (`T`).
* **Zone Connection Highlighting:** Automatically identifies labels starting with `to_` (e.g., `to_qeynos`) and renders them as large green indicators for easy navigation.

### Visuals & UI
* **Vector Rendering:** Anti-aliased line rendering for map geometry.
* **Coordinate Flipping:** * EQ Log coordinates are Right-Handed.
    * EQ Map files are Left-Handed.
    * **Solution:** We apply a `-1.0` multiplier to Player X/Y to align the dot with the map geometry without flipping the map itself.
* **Smart HUD:** Displays Zone Name, Opacity, Status toggles, and Mouse Cursor Coordinates (translated to in-game world coordinates).
* **Window Management:** Resizable window, borderless support, adjustable transparency (`[` / `]`).

### "Corpse Run" Mode
* **Death Detection:** Parser listens for "You have been slain".
* **Visual Aid:** Records death location, draws a Red Line and Skull Marker ("X") from current player position to the corpse.
* **Auto-Clear:** Clears marker on "You summon your corpse".
* **Manual Clear:** Hotkey `K`.

### Multi-Character Support
* **Polling Reader:** The log reader checks the directory every 3 seconds.
* **Auto-Switching:** If a newer log file appears (character switch), it automatically closes the old handle and opens the new one.
* **Smart Seek:** When switching files, it seeks to `End - 5KB` rather than `End` to ensure the "You have entered [Zone]" message is caught during login.

## 4. Input Map / Controls
| Key | Action |
| :--- | :--- |
| **Right Click + Drag** | Pan Map |
| **Scroll Wheel** | Zoom In/Out |
| **Space** | Center on Player |
| **T** | Toggle Breadcrumb Trail |
| **L** | Toggle Map Labels |
| **C** | Clear Breadcrumb History |
| **K** | Clear Corpse Marker |
| **[ / ]** | Decrease / Increase Background Opacity |
| **F5** | Recenter on Map Geometry (Emergency Reset) |

## 5. Known Technical Quirks (For AI Context)
* **Coordinate System:** The `Window` struct uses `PlayerMultX` and `PlayerMultY` (default `-1.0`) to invert player movement logic. **Do not** attempt to flip the map geometry parsing; we flip the *player* to match the map.
* **Zone Loading:** The parser detects zone changes via "You have entered...". It attempts to load `zone_shortname.txt`. A lookup table (`map_keys.ini`) handles long-to-short name conversion.
* **Mouse Coordinates:** The `screenToMap` function applies the same `-1.0` multiplier so that hovering over the map shows the correct `/loc` for that spot.

## 6. Pending / Future Features
* **Search:** "Find" command to draw lines to specific NPC labels (Bank, Guard, etc).
* **Waypoints:** Click-to-set custom destination lines.
* **Spawn Timers:** Overlay for tracking mob respawns (approx 6:40).