# Windows manual verification

Use this checklist on a Windows amd64 desktop with the native app and Apex Legends installed. These checks require the game window and display hardware; browser tests do not verify native alpha blending, click-through, global hotkeys, or Raw Input.

## Overlay and input

- [ ] Open Apex Legends in borderless windowed mode and launch Recoil Practice.
- [ ] In edit mode, place the arrows around the intended crosshair alignment point. Resize and move the settings window, then check arrow spacing, size, opacity, and timeline settings.
- [ ] Press F8 to enter practice mode. Confirm the window becomes click-through and stays above the game, and that the arrows remain aligned with the same screen point.
- [ ] With Apex Legends in the foreground, hold left-click. Confirm the right arrow starts immediately, the left arrow begins at 800 ms, and the right arrow returns at 1,330 ms.
- [ ] Release left-click during a run. Confirm the timeline resets and later phases do not appear after release.
- [ ] Hold left-click through the full 2,210 ms pattern. Confirm the progress stays at the end, the active arrow turns off, and it does not loop. Release and click again to start a new run.
- [ ] Alt-tab away during a run. Confirm focus loss cancels it. Return to Apex Legends and verify a fresh click starts a run.
- [ ] Press F9 during a run. Confirm input is disabled and the run resets. Press F9 again, release any held click, then press again to verify a fresh click starts the pattern.
- [ ] Press F8 to return to edit mode. Confirm controls can be used and Preview pattern runs without Apex Legends in the foreground.

## Voice and saved settings

- [ ] Enable Voice cues and preview a pattern. Confirm embedded `right` and `left` clips play offline.
- [ ] At the default 150 ms lead, compare each spoken direction with its visual transition. Try 0 ms and 350 ms to confirm the adjustable lead is applied.
- [ ] Move the window to another position and enter practice mode. Exit through the app close control, relaunch, and confirm the saved position and settings are restored.
- [ ] If multiple monitors are available, save a position on a secondary display, relaunch, and verify it restores sensibly. Then disconnect or change the display arrangement and confirm the window remains reachable.

## Display scaling

- [ ] Check the overlay at the display's normal DPI scaling and at another Windows scaling level, if available. Confirm arrows, timeline, labels, and input controls remain legible and positioned correctly.
- [ ] Resize to the minimum supported app window size and test the largest arrow size and timeline offset. Confirm the timeline does not cover controls.
