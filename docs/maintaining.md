# Maintainer notes

These notes are for persons who change or release the program. The [README](../README.md) tells you how to use and build it.

## Test

Build the program first. The Go tests use the built frontend.

```powershell
go vet ./...
go test ./...
cd frontend
npm test
```

- The frontend tests use Google Chrome.
- Two Go tests use real windows and Raw Input. Set `RECOIL_NATIVE_TEST=1` to include them.
- The automatic tests cannot examine the overlay in the game.

## Release

1. Set `productVersion` in `wails.json` and the version text in `frontend/src/main.js`.
2. Make a tag with the same version and push it:

```powershell
git tag v0.2.0
git push origin v0.2.0
```

GitHub Actions builds `recoil-overlay.exe` and attaches it to a new release.

## Design

- The settings window is a Wails (WebView2) window.
- Go draws the overlay into a separate layered window. Windows lets mouse clicks go through this window to the game.
- A separate window is necessary because Windows cannot make the transparent Wails window a layered window.
- A Raw Input listener reads the left mouse button and the four bound keys. It discards all other keys.
- The voice cues are in the program file. An internet connection is not necessary.
