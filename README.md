# OSSU
## The Open Source Screenshot Utility
Inspired by UOSU, the Ultima Online Screenshot Utility.

An independent Windows utility preserving the classic workflow: folder, hotkey, repeated captures and named files. No uploads, accounts, telemetry or game injection. **Early preview**, not an official UOSU update.

## Use
Run OSSU.exe on Windows 10/11 with .NET Framework 4.8. No administrator privileges or installer required. The executable is unsigned.
- Ctrl+Shift+F9: selected target (active window by default).
- Ctrl+Shift+F10: all monitors.
- Pick alternate Ctrl+Shift+F1–F12 combinations and Apply.
- Client area excludes window borders/titlebar.
- PNG default; JPEG adjustable quality; BMP available.
- Timestamp or numbered filenames; no overwrites.
- Capture in 3s lets you switch to your game.
- Auto shutter is opt-in, OFF at startup, 5–3600 seconds. Errors stop it.
- Close hides to tray; Exit in tray menu quits.
- Preview the last 30 captures from this run.

Settings: %LOCALAPPDATA%\OSSU\settings.xml. Images: Pictures\OSSU by default. This may be OneDrive-synced; choose a local folder if unwanted.

## Limitations
Copies visible screen pixels. Covering windows will appear. Minimized windows, protected content, secure desktop, HDR tone mapping and exclusive-fullscreen games are not guaranteed; use windowed/borderless mode. No pointer capture. Auto shutter follows the foreground window: stop before opening private content. Mixed-DPI monitor moves may require restarting OSSU.

## Build
Run Windows PowerShell: .\build.ps1 -Test
Uses the Windows .NET Framework C# compiler without downloaded dependencies. Output: build\OSSU.exe.

13 synthetic tests passed locally. Actual WinForms layout visually reviewed at 200% DPI. **Real game capture, native hotkeys, tray lifecycle and multi-monitor operation remain manual acceptance tests**, not verified claims.
See docs/research.md. MIT applies to this independently authored code only.
