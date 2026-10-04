# Recoil Practice

Recoil Practice is a Windows overlay for practicing a visual strafe rhythm with the Apex Legends R-301. It shows two arrows and an expected timing timeline. It does not automate game input.

## Run

Open `build/bin/recoil-overlay.exe` on Windows 10/11 (64-bit), with the Microsoft Edge WebView2 runtime installed. Try **Preview pattern**, then press **F8**: the overlay appears centered on the Apex window. Use **Move overlay** if it needs adjusting. Use Apex in borderless windowed mode. F8 returns to the editable controls.

The repository also contains old upstream superglide binaries; use **recoil-overlay.exe**, not those files. Recoil Practice needs no separate input server or administrator privileges.

## Pattern and controls

The built-in pattern is fixed at:

| Direction | Duration |
| --- | ---: |
| Right | 800 ms |
| Left | 530 ms |
| Right | 880 ms |

The native Windows input listener uses Raw Input for left mouse button down/up events and for the two strafe keys; every other keystroke is discarded unread. Hold left-click while Apex Legends is the foreground window to start the pattern. Releasing cancels and resets it. The pattern ends after 2.21 seconds and does not loop; release and press again for another run.

- **F8** switches between edit mode and practice mode. Practice mode makes the overlay click-through; press F8 again to edit.
- **Move overlay** shows the overlay with a backdrop so you can drag it into place; **Done moving** hides it again. The position is saved as an offset from the middle of the Apex window (or of the current monitor when Apex is not running), so it follows resolution changes. The first run starts centered, and **Center** resets it. Use this to correct the placement on mixed-DPI setups.
- **Hide timeline while shooting** keeps the timeline visible between sprays but removes it from the overlay while left-click is held.
- **Your strafes** are drawn in a second bar under the expected one, on the same time axis: mint while you strafe right, coral for left, grey for neutral (no key, or both held). The **Strafe keys** buttons set which two keyboard keys are read (A and D by default): click one, then press the key; Escape cancels.
- **Score:** when a spray ends, it stays on the timeline until the next one. Each expected switch shows how early (−) or late (+) you made it, or MISS, and the header shows the average. Sprays shorter than 250 ms are ignored, and a switch is only judged if you kept firing long enough to make it.
- **F9** disables or enables practice input. Re-enabling requires a fresh click (release, then press).
- The game must have the foreground title **Apex Legends** for a held click to start a run.
- **Preview pattern** runs from edit mode without Apex Legends. Use it to inspect the arrows and timeline.
- Optional voice cues are embedded in the app, so playback works offline. The clips are short so they fit before a direction change: `left` is about 77 ms and `right` about 95 ms, synthesized with Microsoft David Desktop at its fastest rate, trimmed to start within 1 ms and normalised in volume. Voice lead defaults to 150 ms and can be adjusted from 0 to 350 ms.

Arrow spacing, arrow size, opacity, arrow and timeline visibility, timeline placement, voice, and voice lead are editable in the Wails settings window. Settings are stored at `%APPDATA%\RecoilPractice\settings.json`. The settings window position is saved when entering practice mode and when exiting through the app's close control.

## Build requirements

- Windows amd64
- Node.js 20 or newer
- Wails CLI 2.10.1
- Go 1.23, either installed on `PATH` or available at `.tools\go1.23\go\bin\go.exe`

The build script uses the portable Go SDK path when present and falls back to system Go otherwise. It installs frontend dependencies from the lockfile, builds the frontend, runs Go tests, and builds the Windows amd64 app:

```powershell
.\scripts\build.ps1
```

To run the app through Wails during development:

```powershell
.\scripts\build.ps1 -Dev
```

The build expects `wails.exe` at `.tools\gopath\bin\wails.exe` or `wails` on `PATH`.

## Checks

From the repository root:

```powershell
go test ./...
go vet ./...
```

The optional native Windows registration smoke test registers and unregisters the mouse Raw Input device and F8/F9 hotkeys. It does not inject input. Run it only on Windows amd64, with F8/F9 available:

```powershell
$env:RECOIL_NATIVE_TEST = '1'
go test . -run TestNativeInputLifecycle -count=1
Remove-Item Env:RECOIL_NATIVE_TEST
```

The frontend Playwright checks run headlessly using an installed Chrome channel:

```powershell
cd frontend
npm ci
npm test
```

Browser layout and frontend checks have been verified. The native desktop/game presentation has not been visually checked in this environment; use the [manual verification checklist](docs/manual-checks.md) on a Windows desktop.

## Rendering status

The Wails window provides the editable settings UI. During practice that window is hidden, and Go draws the arrows and timeline into a per-pixel alpha bitmap in a separate native layered window with its own saved position. The settings window stays where you left it. Windows layered-window flags provide click-through. A separate native window is required because the transparent Wails window is created with `WS_EX_NOREDIRECTIONBITMAP`, which Windows will not combine with `WS_EX_LAYERED`. If the input listener or native renderer fails, the app attempts to restore edit mode and shows an error.

Automated Windows tests verify input/hotkey registration cleanup, alpha-bitmap submission, and showing/hiding the practice overlay over a hidden test-owned window. The canvas is tested at 100%, 150%, and 200% scale. These tests do not establish in-game display compatibility.

This first version has only the R-301 preset and an expected-direction timeline. Shot markers, automatic weapon detection, and additional weapon profiles are future work. A click is an intent to shoot; the overlay cannot detect reloads, ammunition or recoil reset state. Use uninterrupted sprays and release between attempts.

## Attribution and license

This fork builds on [AlexKimmel/superglide-overlay](https://github.com/AlexKimmel/superglide-overlay). The original MIT license is retained in [LICENSE](LICENSE). R-301 durations were transcribed from the [Apex recoil strafe trainer](https://www.apexrecoilstrafing.com/) on October 4, 2026; they are trainer presets, not independently measured game telemetry. The input architecture uses the documented Windows Raw Input APIs; no source from the GPL input-overlay project was copied.
