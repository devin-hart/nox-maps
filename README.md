# Nox Maps 🗺️

**A lightweight, single-binary map overlay for EverQuest Project 1999.**

Nox Maps is a modern rewrite of the classic log-parsing map tools (like nParse), built specifically for the Linux/P99 player who wants performance, stability, and simplicity.

## 🚀 Why Nox?
Current tools rely on massive frameworks (Qt, WebEngine) that are prone to crashing on Linux, especially with NVIDIA drivers. Nox Maps strips away the bloat.

* **Single Binary:** No Python, no `venv`, no `pip install`. Download one file and run it.
* **Native Performance:** Written in **Go** using **Ebitengine** for lightweight, crash-free 2D rendering.
* **P99 Safe:** Strictly passive. It reads your `eqlog.txt` file. It never touches game memory or injects code.
* **Brewall Compatible:** Native support for Brewall's map format, with a built-in "Classic Filter" to ignore non-P99 zones.

## 🛠️ Tech Stack
* **Language:** Go (Golang) 1.21+
* **Graphics:** Ebitengine (v2) - A dead-simple 2D game library.
* **GUI:** Immediate Mode (Custom or EbitenUI) for overlays.

## 📂 Project Structure
* `cmd/nox-maps`: Entry point. The `main.go` lives here.
* `internal/eqlog`: The "Tailer". Watches your log file for changes in real-time.
* `internal/parser`: The "Brain". Regex engine that converts log lines into coordinates `(x, y)` and zone changes.
* `internal/maps`: The "Cartographer". Loads Brewall's `.txt` files and filters out non-classic zones.
* `internal/ui`: The "Painter". Draws the transparent overlay window using Ebitengine.

## 📝 Usage (Planned)
```bash
# Run it directly (Linux)
./nox-maps

# Config is auto-generated on first run at ~/.config/nox-maps/config.yaml
````

## 📜 Legal

This tool is compliant with Project 1999's rules regarding log parsers. It does not automate gameplay, broadcast information to other clients, or read memory.