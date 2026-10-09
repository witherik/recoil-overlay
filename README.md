# Recoil Practice

Recoil Practice is a Windows overlay for practicing the strafe rhythm of Apex Legends weapons. It shows two arrows and an expected timing timeline for the weapon you pick, and scores your own strafes against it. It does not automate game input.

## Run

Open `build/bin/recoil-overlay.exe` on Windows 10/11 (64-bit), with the Microsoft Edge WebView2 runtime installed. The overlay appears straight away, centered on the Apex window, so you can adjust it while looking at the real thing. Try **Preview pattern**, use **Move overlay** if the position needs adjusting, then press **F8** to practice. Use Apex in borderless windowed mode. F8 returns to the editable controls.

The repository also contains old upstream superglide binaries; use **recoil-overlay.exe**, not those files. Recoil Practice needs no separate input server or administrator privileges.

## Pattern and controls

Pick a weapon from the large menu at the top of the settings; where it has more than one firing mode, the modes appear as buttons beside it. There are 15 weapons and 20 patterns:

| Weapon | Mode | Pattern (ms) | Total |
| --- | --- | --- | ---: |
| Flatline | | L600 R500 L1100 R610 | 2.81 s |
| HAVOC | Normal | wait 350, R370 L350 R630 L1150 | 2.85 s |
| HAVOC | Turbocharged | R370 L350 R630 L1150 | 2.50 s |
| Hemlok | | L1310 R660 L650 R660 L390 | 3.67 s |
| Nemesis | Charged | R330 L660 R330 L660 R330 L330 | 2.64 s |
| R-301 | | R800 L530 R880 | 2.21 s |
| Alternator | Default | L600 R700 L800 R710 | 2.81 s |
| Alternator | Double Tap | L450 R525 L600 R530 | 2.11 s |
| C.A.R. | | L470 R380 L250 R330 L320 | 1.75 s |
| Prowler | Burst | L1280 R1050 L410 | 2.74 s |
| Prowler | Auto | L990 R660 L380 R530 | 2.56 s |
| R-99 | | R390 L270 R290 L220 R280 | 1.45 s |
| Volt | | R750 L420 R250 L750 | 2.17 s |
| Devotion | Normal | R500 L1190 R1450 L530 | 3.67 s |
| Devotion | Turbocharged | R270 L1080 R1400 L760 | 3.51 s |
| L-STAR | | L300 R2400 | 2.70 s |
| Rampage | Normal | L620 R1230 L6100 | 7.95 s |
| Rampage | Charged | L450 R940 L4600 | 5.99 s |
| Spitfire | | L430 R670 L1110 R1440 L1330 R460 | 5.44 s |
| RE-45 | | wait 400, L500 R500 L500 R500 | 2.40 s |

"wait" is a neutral phase: fire without strafing, as when the HAVOC spins up. It is drawn grey, has no voice line and is not scored. Any input is fine during a wait: if you are already strafing the right way when it ends, that switch counts as on time. Where the source lists consecutive steps in the same direction (burst weapons), they are merged into one phase.

The native Windows input listener uses Raw Input for left mouse button down/up events and for the five bound keys (two strafe keys, three practice keys); every other keystroke is discarded unread. The keys are only read, never swallowed, so the game still receives them. Hold left-click while Apex Legends is the foreground window to start the pattern. Releasing cancels and resets it. The pattern ends after its last phase and does not loop; release and press again for another run.

- **F8** switches between edit mode and practice mode. Practice mode hides the settings window and starts reading your clicks; press F8 again to edit.
- **Keys** sets the strafe keys and the three practice keys: click one, then press the key; Escape cancels. **Start practice** and **End practice** are both F8 by default, which makes it a toggle; give them different keys to have one key that only starts and one that only ends. **Disable / enable** is F9. A key already used by another action is refused. The names below use the defaults.
- **Overlay: Hide / Show** removes the overlay from the screen while you edit, or brings it back. In edit mode it is the same click-through window as in practice, in the same place, and it follows every setting as you change it. Practice mode always shows it, and the app starts with it shown.
- **Move overlay** gives the overlay a backdrop so you can drag it into place; **Done moving** makes it click-through again. The position is saved as an offset from the middle of the Apex window (or of the current monitor when Apex is not running), so it follows resolution changes. The first run starts centered, and **Center** resets it. Use this to correct the placement on mixed-DPI setups.
- **Timeline** has its own switch, a spacing below the arrows (up to 400 px) and a width (280 to 800 px). **Hide while shooting** keeps it visible between sprays but removes it from the overlay while left-click is held.
- **Colors** (in the Overlay card) picks the colour scheme for both the settings window and the overlay: Mint & coral (default), Violet & gold, Ember & cyan, or Blue & orange. The first colour of each pair is "right" on the overlay and the accent in the window; the second is "left". Blue & orange is the pair chosen to stay apart under colour blindness.
- **Reset defaults**, at the bottom of the window, puts every setting, key and the overlay position back to its default after a second click. The settings window keeps its place and size.
- Every slider has a number field beside it: type a value and press Enter or Tab. Values outside the range are clamped.
- **Your strafes** are drawn in a second bar under the expected one, on the same time axis: in the scheme's "right" colour while you strafe right, its "left" colour for left, grey for neutral (no key, or both held). The strafe keys read are A and D by default.
- **Score:** when a spray ends, it stays on the timeline until the next one. Each expected switch shows how early (−) or late (+) you made it, or MISS, and the header shows the total: the sum of those deviations, counting early and late alike. Sprays shorter than 250 ms are ignored, and a switch is only judged if you kept firing long enough to make it.
- **Opening cue** (off by default) also speaks the first strafe of a spray. It cannot be announced ahead of time, because the click is not predictable; with it off, the voice starts at the first change of direction. For weapons that begin with a wait, the strafe after the wait counts as the opening one.
- **F9** disables or enables practice input. Re-enabling requires a fresh click (release, then press).
- The game must have the foreground title **Apex Legends** for a held click to start a run.
- **Preview pattern** runs the pattern on the overlay from edit mode, without Apex Legends. It shows the overlay first if it was hidden.
- Optional voice cues are embedded in the app, so playback works offline. **Sound** picks one of three:
  - **Fast voice** (default): "left" and "right" compressed so each word starts within 1 ms and is effectively over by 75 ms. Synthesized with Microsoft David Desktop at its fastest rate, trimmed, time-compressed without changing pitch, and normalised in volume.
  - **Natural voice**: the same words at the normal speaking rate, about 250 ms each.
  - **Tones**: a 60 ms beep, low for left and an octave higher for right.
- **Lead** is how long before a direction change its cue starts, from 0 to 350 ms. It defaults to the length of the chosen sound (75, 250 or 60 ms), so the cue finishes just as the change arrives; a lead you have set yourself is kept when you switch sound.

The settings window groups everything into Arrows, Timeline, Overlay, Voice and Keys. Settings are stored at `%APPDATA%\RecoilPractice\settings.json`. The settings window position is saved when entering practice mode and when exiting through the app's close control. The **–** button in the title bar minimises the settings window to the taskbar. In edit mode the overlay preview goes with it and returns when you restore the window. Practice mode is unaffected, and leaving practice always brings the settings window back, even if it was minimised.

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

The optional native Windows registration smoke test registers and unregisters the mouse and keyboard Raw Input devices. It does not inject input. Run it only on Windows amd64:

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

The Wails window provides the editable settings UI. Go draws the arrows and timeline into a per-pixel alpha bitmap in a separate native layered window with its own saved position; edit mode shows that same window as its preview, and practice hides the settings window. The settings window stays where you left it. Windows layered-window flags provide click-through. A separate native window is required because the transparent Wails window is created with `WS_EX_NOREDIRECTIONBITMAP`, which Windows will not combine with `WS_EX_LAYERED`. If the input listener or native renderer fails, the app attempts to restore edit mode and shows an error.

Automated Windows tests verify input registration cleanup, alpha-bitmap submission, and showing/hiding the practice overlay over a hidden test-owned window. The canvas is tested at 100%, 150%, and 200% scale. These tests do not establish in-game display compatibility.

The weapon is chosen by hand: shot markers and automatic weapon detection are future work. A click is an intent to shoot; the overlay cannot detect reloads, ammunition or recoil reset state. Use uninterrupted sprays and release between attempts.

## Attribution and license

This fork builds on [AlexKimmel/superglide-overlay](https://github.com/AlexKimmel/superglide-overlay). The original MIT license is retained in [LICENSE](LICENSE). Weapon patterns were transcribed from the [Apex recoil strafe trainer](https://www.apexrecoilstrafing.com/) on October 4, 2026; they are trainer presets, not independently measured game telemetry. The weapon icons in `frontend/src/assets/weapons` were downloaded from the same site on October 9, 2026; they are Apex Legends artwork belonging to Electronic Arts and are not covered by this repository's MIT license. The input architecture uses the documented Windows Raw Input APIs; no source from the GPL input-overlay project was copied.
