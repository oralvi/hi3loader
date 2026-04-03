# HI3 Loader

Portable Wails desktop utility for local workflows.

## Overview

- Windows desktop GUI built with Wails.
- Supports local account management, game launch, and QR handling from the game window.
- Uses a configurable local or remote endpoint for runtime requests.
- Keeps window QR capture conservative and favors clear manual guidance when the QR state is not ready.

## Attribution

- Inspired by Haocen2004's [bh3_login_simulation-memories](https://github.com/HonkaiScanner/bh3_login_simulation-memories).
- Some protocol-facing portions in this project are adapted from public upstream implementations. Keep upstream notices and licenses when redistributing derivative code.

## Development

### Prerequisites

- Go
- Node.js
- `wails` CLI
- PowerShell on Windows

### Install frontend dependencies

```powershell
cd frontend
npm install
cd ..
```

### Run in development

```powershell
wails dev
```

Notes:

- The GUI performs local Bilibili account login and talks to the configured endpoint for runtime requests.
- Fresh clones must build the frontend first because `main.go` embeds `frontend/dist`.
- Development package names and title stamps use `dev+<random>+<yyMMddHHmmss>`.

Frontend build prerequisite:

```powershell
cd frontend
npm install
npm run build
cd ..
```

### Development builds

Use the development build script if you want a local package to test.

```powershell
.\scripts\build-dev.ps1
```

## Release Build

Use the release script for packaged builds:

```powershell
.\scripts\build-release.ps1
```

What the release script does:

- builds with stripped symbol/debug metadata
- produces the packaged GUI binary in `build/bin`
- adds a release title stamp in the form `r<yyMMddHHmmss>`

Release behavior:

- GUI release is Windows-only
- game-window QR handling favors stable detection and manual guidance over simulated clicks inside the Unity client

## Repository Notes

- `.gitignore` intentionally excludes portable runtime data, build output, and generated bindings.
- `frontend/dist` still needs to be generated locally before `go test ./...`, `go build`, or `wails build` can compile the `main` package.

## License

Released under the [MIT License](LICENSE).
