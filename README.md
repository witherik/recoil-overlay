# Recoil Practice

Recoil Practice is a Windows overlay for Apex Legends. It shows the strafe pattern of a weapon while you shoot. Then it compares your strafes with the pattern and gives a score.

The program only reads the mouse and the keyboard. It does not send input to the game.

https://github.com/user-attachments/assets/5512f863-05c0-4c45-9105-68176445ffe5

![The settings window of Recoil Practice](docs/images/settings.png)

## Download

1. Download `recoil-overlay.exe` from the [latest release](../../releases/latest).
2. Start the program. No installation is necessary.

The program operates on Windows 10 and Windows 11 (64-bit). The Microsoft Edge WebView2 Runtime is necessary.

**Note:** The file has no code signature. Windows SmartScreen can show a warning when you start the program.

## Use

1. Set Apex Legends to the borderless window mode.
2. Start Recoil Practice. The overlay shows on the crosshair.
3. Select a weapon. If the weapon has more than one fire mode, select a fire mode.
4. Click **Preview pattern** to see the pattern.
5. Push **F8** to start the practice mode.
6. Hold the left mouse button in Apex Legends. Strafe in the direction of the bright arrow.
7. Release the mouse button. The timeline shows your result.
8. Push **F8** again to go back to the settings window.

| Key | Function |
| --- | --- |
| F8 | Starts and stops the practice mode. |
| A, D | The strafe keys that the program reads. |

You can change all keys in the **Keys** card.

### Result

- The top bar of the timeline shows the pattern. The bar below it shows your strafes.
- Each direction change shows your time error in milliseconds. A minus sign means too early. A plus sign means too late.
- `MISS` means that you did not change direction.
- The total deviation is the sum of the time errors. A smaller number is better.
- The program ignores a spray that is shorter than 250 ms.

### Settings

| Card | Contents |
| --- | --- |
| Arrows | The distance between the arrows and their size. |
| Timeline | The position and the width of the timeline. A negative position puts the timeline above the arrows. |
| Overlay | The colors, the opacity and the position. **Move** lets you drag the overlay. **Center** puts it back on the crosshair. |
| Voice | A voice cue or a tone before each direction change, and its lead time. |
| Keys | The strafe keys and the practice keys. |

**Reset defaults** sets all settings to their initial values. The program saves the settings in `%APPDATA%\RecoilPractice\settings.json`.

### Limits

- You must select the weapon manually.
- The program does not know the ammunition quantity or the reload status.
- The title of the game window must be `Apex Legends`.
- The program contains 20 patterns for 15 weapons. The patterns come from a community trainer. They are not game measurements.

The program reads only the left mouse button and the four keys in the **Keys** card. It discards all other keys. The voice cues are in the program file. An internet connection is not necessary.

## Build

These items are necessary:

- Windows (amd64)
- Go 1.23
- Node.js 20 or later
- Wails CLI 2.10.1

Install the Wails CLI:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.1
```

Build the program:

```powershell
.\scripts\build.ps1
```

The script installs the frontend packages, does the Go tests and builds `build\bin\recoil-overlay.exe`. Add `-Dev` to start the program in the Wails development mode.

## Credits

- This project is a fork of [superglide-overlay](https://github.com/AlexKimmel/superglide-overlay) by AlexKimmel (MIT License).
- The weapon patterns and the weapon icons come from the [Apex recoil strafe trainer](https://www.apexrecoilstrafing.com/).
- [Wails](https://wails.io/) supplies the window and the build tools.

Apex Legends is a trademark of Electronic Arts Inc. This project has no relation to Electronic Arts or Respawn Entertainment. The weapon icons in `frontend/src/assets/weapons` are their property, and the MIT License does not apply to them.

## Maintenance

The tests, the release procedure and the design are in the [maintainer notes](docs/maintaining.md).

## License

[MIT](LICENSE)
