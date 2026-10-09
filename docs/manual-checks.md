# Windows manual verification

Use this checklist on a Windows amd64 desktop with the native app and Apex Legends installed. These checks require the game window and display hardware; browser tests do not verify native alpha blending, click-through, global hotkeys, or Raw Input.

## Overlay and input

- [ ] Open Apex Legends in borderless windowed mode and launch Recoil Practice.
- [ ] Confirm the overlay is already on screen over the crosshair, reads `EDIT MODE / F8 TO PRACTICE`, and lets clicks through to whatever is under it.
- [ ] In edit mode, change arrow spacing, size, opacity, timeline spacing and timeline width, by slider and by typing a value. Confirm the overlay follows each change.
- [ ] Open the weapon menu, pick another weapon and, where offered, another firing mode. Confirm the overlay timeline changes with it.
- [ ] Click **Hide** and confirm the overlay disappears; **Show** brings it back. Hide it, click **Preview pattern**, and confirm it returns and plays the pattern.
- [ ] Click **Move overlay**, drag the overlay by its backdrop, and click **Done moving**. Confirm it stays where you left it, without the backdrop, and is still there in practice mode. Click **Center** and confirm it returns to the crosshair.
- [ ] Press F8 to enter practice mode. Confirm the settings window hides, the overlay is click-through and stays above the game, and the arrows sit either side of the crosshair.
- [ ] With the R-301 selected and Apex Legends in the foreground, hold left-click. Confirm the right arrow starts immediately, the left arrow begins at 800 ms, and the right arrow returns at 1,330 ms.
- [ ] Select the HAVOC in Normal mode. Confirm the timeline starts with a grey 350 ms phase, neither arrow is lit and no word is spoken until the first strafe is due. Switch to Turbocharged and confirm the grey phase disappears.
- [ ] Release left-click during a run. Confirm the timeline resets and later phases do not appear after release.
- [ ] Hold left-click through the full 2,210 ms pattern. Confirm the progress stays at the end, the active arrow turns off, and it does not loop. Release and click again to start a new run.
- [ ] Alt-tab away during a run. Confirm focus loss cancels it. Return to Apex Legends and verify a fresh click starts a run.
- [ ] Press F9 during a run. Confirm input is disabled and the run resets. Press F9 again, release any held click, then press again to verify a fresh click starts the pattern.
- [ ] Press F8 to return to edit mode. Confirm the settings window returns, the overlay stays up, and Preview pattern runs without Apex Legends in the foreground.

## Strafe reading and score

- [ ] Hold left-click in Apex and strafe with A and D. Confirm the lower bar fills mint for right and coral for left as the playhead moves, and stays grey while neither or both keys are held.
- [ ] Release. Confirm the spray stays on the timeline with a signed deviation under each switch and the total deviation in the header, until the next click.
- [ ] Rebind a strafe key in edit mode, then confirm the new key is read and the old one is ignored. Press Escape while rebinding and confirm nothing changes.
- [ ] Rebind **Start practice** to another key. Confirm it enters practice but does not leave it, and that the old key still ends practice. Hold the new key down and confirm it does not flip modes repeatedly.
- [ ] Rebind **Disable / enable** and confirm the overlay status line names the new key. Try to bind a strafe key to a practice key and confirm it is refused.

## Voice and saved settings

- [ ] Enable Voice cues and preview a pattern. Confirm embedded `right` and `left` clips play offline.
- [ ] Switch **Sound** to Natural voice, then Tones, previewing each. Confirm the lead follows to 250 ms and 60 ms, and that a lead you typed yourself is kept on the next switch.
- [ ] At the default 75 ms lead, compare each spoken direction with its visual transition. Try 0 ms and 350 ms to confirm the adjustable lead is applied.
- [ ] Move the window to another position and enter practice mode. Exit through the app close control, relaunch, and confirm the saved position and settings are restored.
- [ ] If multiple monitors are available, save a position on a secondary display, relaunch, and verify it restores sensibly. Then disconnect or change the display arrangement and confirm the window remains reachable.

## Display scaling

- [ ] Check the overlay at the display's normal DPI scaling and at another Windows scaling level, if available. Confirm arrows, timeline, labels, and input controls remain legible and positioned correctly.
- [ ] Resize to the minimum supported app window size. Confirm every control is still reachable.
