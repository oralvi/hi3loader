# HI3LOADER

A small desktop launcher that quietly talks to games, pulls down the dispatch/auth tokens it needs, and wraps the whole thing in a tiny GUI — without dragging a bunch of build artifacts and caches into version control.

> Think of it like a polite butler: it fetches the token for you, starts the game, and keeps the repo clean by ignoring all the smoke-and-mirrors build junk.

---

## ⚙️ What it does

- Uses a light Wails-based GUI to provide one-click actions (launch game, etc.)
- Keeps state/config in a small JSON file and emits a log for debugging
- Ignores build output, caches, and generated assets so the repo stays lean

## 🛠️ Getting started

### Prerequisites
- Go (1.26+ recommended)
- Node.js (for the frontend build)
- `wails` CLI installed (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

### Run in development

```powershell
cd <repo root>
# install frontend deps
cd frontend
npm install

# run dev mode
cd ..
wails dev
```

### Build release

```powershell
cd <repo root>
wails build
```

## 🧹 Why the `.gitignore` matters

This repo intentionally ignores:

- build outputs (`build/`, `dist/`, `frontend/dist/`)
- Go build cache, module cache, and compiled binaries (`pkg/`, `bin/`)
- node_modules, temp files, logs, and other generated junk

That keeps the repository focused on the real source: Go code, the tiny frontend, and the integration glue.

## 📦 License

This project is released under the **MIT License** (see `LICENSE`).

