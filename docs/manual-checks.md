# Manual checklist

The automatic tests cannot examine the overlay in the game. Do these checks on a Windows computer with Apex Legends before a release. The key names are the default keys.

## Preparation

1. Start Apex Legends in the borderless window mode.
2. Start Recoil Practice.

## Overlay

- [ ] The overlay shows on the crosshair. Its status line reads `EDIT MODE / F8 TO PRACTICE`.
- [ ] Mouse clicks go through the overlay.
- [ ] Change each slider and each number field. The overlay shows each change.
- [ ] Change **Colors**. The settings window and the overlay change color.
- [ ] Set the timeline spacing to a negative value. The timeline moves above the arrows. The status line moves below the arrows. The arrows stay on the crosshair.
- [ ] Select a different weapon and a different fire mode. The timeline shows the new pattern.
- [ ] Click **Hide**. The overlay goes out of view. Click **Show**. The overlay comes back.
- [ ] Hide the overlay. Click **Preview pattern**. The overlay comes back and shows the pattern.
- [ ] Click **Move**. Drag the overlay. Click **Done**. The overlay stays in its new position.
- [ ] Click **Center**. The overlay goes back to the crosshair.

## Settings window

- [ ] Click **–** in the title bar. The settings window goes to the taskbar and the overlay goes out of view.
- [ ] Click the program in the taskbar. The settings window and the overlay come back.
- [ ] You cannot change the size of the settings window. All controls show without a scroll bar.
- [ ] Move the settings window. Close the program and start it again. The window position and the settings are the same.
- [ ] If you have a second monitor, do the last check on that monitor. Then disconnect the monitor and start the program. The window shows on the first monitor.

## Practice mode

- [ ] Push F8. The settings window goes to the taskbar. The overlay stays above the game.
- [ ] Select the R-301. Hold the left mouse button. The right arrow comes on immediately. The left arrow comes on at 800 ms. The right arrow comes on again at 1,330 ms.
- [ ] Hold the button for the full pattern. The playhead stops at the end and the pattern does not start again.
- [ ] Release the button during a pattern. The pattern stops.
- [ ] Push Alt+Tab during a pattern. The pattern stops. Go back to the game. A new click starts a pattern.
- [ ] Push F9 during a pattern. The pattern stops and the status line reads `DISABLED`. Push F9 again. Only a new click starts a pattern.
- [ ] Select the HAVOC in the Normal mode. The timeline starts with a grey phase of 350 ms. No arrow comes on and no voice cue sounds in this phase.
- [ ] Push F8. The settings window comes back and the overlay stays in view.
- [ ] Push F8. Click the program in the taskbar. The practice mode stops and the settings window comes back.

## Score

- [ ] Hold the left mouse button and strafe with A and D. The lower bar shows your strafes in the two colors. It stays grey when you hold no key or both keys.
- [ ] Release the button. The timeline shows a time error below each direction change and the total deviation at the top.

## Keys

- [ ] Give a strafe key a different key. The program reads the new key and ignores the old key.
- [ ] Start to change a key and push Escape. The key does not change.
- [ ] Give **Start practice** a different key. The new key starts the practice mode but does not stop it. The old key stops it.
- [ ] Hold the new key. The program changes mode only one time.
- [ ] Try to give a practice key the key of a strafe key. The program refuses the key.

## Voice

- [ ] Click **Preview pattern** with each of the three sounds. Each sound plays without an internet connection.
- [ ] Change **Sound**. The lead time changes to 60 ms, 125 ms or 0 ms. A lead time that you typed stays the same.
- [ ] Set the lead time to 0 ms and then to 350 ms. The cue comes at the direction change and then 350 ms before it.

## Errors and display scale

- [ ] If an error message shows, click it. The message goes out of view.
- [ ] Do the overlay checks again at a different Windows display scale. The arrows, the timeline and the text have the correct position and size.
