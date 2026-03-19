# HI3 Loader

Portable Wails desktop utility for local workflows.

## Overview

- Small Wails GUI with a local-only workflow.

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

### Development builds

Use the development build script if you want a local package to test.

```powershell
.\scripts\build-dev.ps1
```

Notes:

- The GUI entrypoint runs in helper-only mode. Sensitive login, token, dispatch, and scan flows are delegated to a short-lived local helper subprocess.
- Direct helper startup is intentionally blocked unless it is launched by the main program with a valid session token.
- Script and non-GUI entrypoints still keep in-process fallback for local development workflows.
- Fresh clones must build the frontend first because `main.go` embeds `frontend/dist`.

Frontend build prerequisite:

```powershell
cd frontend
npm install
npm run build
cd ..
```

### Script / fallback workflows

Some local scripts are still expected to run outside the GUI path. Example:

```powershell
go run ./scripts/manual_fetch_bili.go
```

Those paths keep development-friendly fallback behavior and are not treated as release-hardened entrypoints.

## Release Build

Use the release script instead of calling `wails build` manually:

```powershell
.\scripts\build-release.ps1
```

What the release script does:

- auto-detects whether `private_impl` is available
- builds with `-ldflags "-s -w"` to strip symbol/debug metadata
- produces the packaged GUI binary in `build/bin`

Release behavior:

- GUI release is helper-only
- if helper startup or helper authorization fails, sensitive GUI actions fail closed instead of falling back to in-process direct calls
- development scripts are not changed by this and may still use fallback logic

## Repository Notes

- `.gitignore` intentionally excludes portable runtime data, build output, generated bindings, etc..
- The public repository is expected to compile in stub mode when private files are absent, but `frontend/dist` still needs to be generated locally before `go test ./...`, `go build`, or `wails build` can compile the `main` package.
- Private local implementations are intentionally not tracked in the public repository.

## License

Released under the [MIT License](LICENSE).
