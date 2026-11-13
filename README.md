# OSSU
## The Open Source Screenshot Utility

Inspired by UOSU, the Ultima Online Screenshot Utility. Independent software, not an official UOSU update. No original UOSU code or artwork is included. MIT licensed.

## Status

Milestone 1 is a small Windows x64 console-first Go screenshot utility. It captures the **visible primary display**, at native pixel dimensions, to PNG. No GUI, tray, service, autostart, installer, or persisted settings. The previous C# implementation was deliberately removed; this implementation starts from the clean Go reset at `156b448`, not from that source.

No uploads, accounts, telemetry, networking features, game hooks, game injection, or bundled UOSU assets. Screenshots stay in the selected output directory; OSSU does not publish them. Desktop pixels and filenames can contain private information: choose the folder and sharing policy accordingly.

Windows can redirect or cloud-sync the Pictures Known Folder (for example through OneDrive). OSSU performs no uploads, but Windows/third-party folder synchronization is outside its control. For strictly local retention, select a verified nonsynced folder with `-out`; do not assume the default Pictures folder is nonsynced.

Project repository: https://github.com/suttonwilliamd/ossu.

## Build and run

Requires Windows x64, Windows 10 version 1703 or newer, and Go 1.25 or newer. Use an interactive, unlocked desktop session. No third-party Go modules, C compiler, administrator privileges, or installation are required.

From the repository root in PowerShell:

```powershell
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
$env:CGO_ENABLED = '0'
go build -trimpath -o .\build\ossu.exe .
.\build\ossu.exe -help
.\build\ossu.exe
```

The last command **waits**, registering `Ctrl+Shift+F9` globally with Windows `RegisterHotKey` and `MOD_NOREPEAT`. Press the shortcut to capture; holding it does not repeatedly capture. Ctrl+C stops and unregisters it. A registration conflict exits nonzero with an actionable error; there is no fallback shortcut. A running capture and PNG write finish serially; presses received while busy are discarded, not replayed as a batch.

```powershell
# One immediate capture; no hotkey registration.
.\build\ossu.exe -once -out 'C:\Screenshots'

# Explicit alternative global shortcut; waits without taking a screenshot.
.\build\ossu.exe -hotkey 'Ctrl+Alt+F10' -out 'C:\Screenshots'

# Three captures; first after 2s, then wait 2s after each completed save.
.\build\ossu.exe -interval 2s -count 3 -out 'C:\Screenshots'
```

### Flags and defaults

| Flag | Default | Contract |
| --- | --- | --- |
| `-out folder` | Current user's Windows Known Folder Pictures + `OSSU` | Folder is created only when saving a capture. Relative paths resolve from the working directory. If Known Folder resolution fails, specify `-out`; OSSU does not silently choose another folder. |
| `-hotkey shortcut` | `Ctrl+Shift+F9` | Only in default hotkey mode. Case-insensitive `Ctrl`, `Alt`, `Shift`, plus one letter A–Z, digit 0–9, or F1–F11. At least Ctrl or Alt is required. Duplicate/unknown modifiers, F12 (debugger-reserved), Windows-key combinations (OS-reserved), and other keys are rejected. OS/app conflicts can still prevent registration. |
| `-once` | `false` | One immediate capture and exit, without a hotkey. Cannot combine with `-interval`, `-count`, or an explicit `-hotkey`. |
| `-interval duration` | Disabled | Opt-in repetition; Go durations such as `500ms`, `5s`, `1m`; minimum `100ms`. First capture after one full interval. Each subsequent delay starts **after** the previous capture/write completes, so slow storage cannot accumulate a timer queue. Cannot combine with an explicit `-hotkey`. |
| `-count n` | Unlimited in interval mode | Explicit values must be positive and require `-interval`. Counts successful saves; any failure stops with a nonzero exit rather than pretending the batch completed. |
| `-help` | — | Print usage and exit. |

No automatic captures occur without `-once` or `-interval`. Ctrl+C stops idle hotkey waiting within approximately 50ms plus scheduling, or interrupts a timer immediately. Pixel conversion and PNG writes check cancellation; an active synchronous Windows/filesystem call must return before cleanup can complete. Ctrl+C is a clean exit; invalid arguments, registration, capture, and file errors are nonzero.

Successful saves log the full path and pixel dimensions. Names resemble `OSSU_2026-11-13_12-34-56.123456789.png`, using local capture time. Same-clock and concurrent-process collisions receive `_0001`, `_0002`, etc. Every file uses Windows exclusive `CREATE_NEW`; no existing file is overwritten. After 10,000 collisions for one timestamp, the capture fails explicitly. A failed PNG write marks only the newly created file for deletion by its still-owned handle, avoiding path-replacement deletion races. Cleanup errors are reported; an abruptly killed process, power loss, or failed cleanup can leave an incomplete file. This is not an atomic-publication or crash-durability guarantee.

## Capture boundaries and limitations

- Primary display only: the native Windows primary-monitor bounds under a per-monitor-v2 DPI context. No fixed game resolution, resizing, crop, window-client capture, or combined multi-monitor output.
- GDI `BitBlt` with `CAPTUREBLT` into a top-down 32-bit DIB captures the composed **visible desktop**, including overlapping windows and potentially sensitive content. It does not capture an obscured/minimized application's hidden pixels. Cursor rendering is not implemented.
- Protected/DRM content, capture-excluded windows, hardware overlays, and some accelerated surfaces can be black, omitted, or different. Successful API calls do not establish that every application allowed its pixels to be captured.
- No exclusive-fullscreen support or guarantee. Use desktop-composed/windowed or borderless applications; even these can restrict capture.
- No secure-desktop, lock-screen, other-session, or elevated-access bypass. Do not expect useful captures while locked or during UAC/secure-desktop transitions. Display mode/primary-monitor changes during capture can cause a failed or inconsistent frame.
- Output is ordinary opaque 8-bit RGB PNG, **not HDR**. No HDR metadata, wide-gamut/color-management pipeline, tone mapping, or color fidelity guarantee on HDR displays. GDI conversion may clip or alter colors.
- No JPEG, updater, publishing, background service, or network feature. Folder ACLs and Windows storage policy remain in effect; a folder must permit creation and deletion of OSSU's own failed writes.

## Verification

**SYNTHETIC** deterministic tests (no desktop capture or keyboard injection):

```powershell
go test -count=1 ./...
```

These cover mode/hotkey validation, reserved keys, simultaneous fixed-clock filename collisions/no overwrites, partial-write/cancellation cleanup preserving existing files, generated-image PNG pixels, and output errors. They do not prove native screenshot or hotkey behavior.

**Native Windows verification** must run the real executable separately: `-once`, a bounded `-interval 200ms -count 3` batch, PNG decoding against the actual primary display resolution, a real shortcut press/hold, a registration conflict, Ctrl+C and re-registration after exit, and an invalid output target. Keep desktop captures and smoke logs in system temp, not the repository or published artifacts. Native results and any inaccessible paths are recorded separately in the worker report; do not infer them from the synthetic suite.

Milestone 1 host evidence (Windows x64, Go 1.27.4): the synthetic suite passed; real `-once` and a three-capture `200ms` batch produced fully decoded opaque, nonuniform `3840×2160` PNGs matching an independent Windows display-mode query. Native conflict and file-as-output-folder cases exited 1 without false saves.

The initial console-cancellation failure's cause was **not directly proven**; inherited ignore-Ctrl+C state in the launcher is suspected. With the test launcher's Ctrl+C processing explicitly normalized, the unchanged initial executable stopped cleanly and released its hotkey (81ms hotkey / 12ms interval). Subsequently, OSSU was changed to explicitly enable Ctrl+C processing for its own process after installing Go's signal handler, without changing other processes or console input mode. The original, unnormalized smoke launcher then passed with that updated executable: hotkey/interval waiting stopped cleanly (50ms/4ms), the same shortcut re-registered after exit, and the Known Folder default resolved without capturing. These are native **programmatic console-control-event** results, not physical keyboard tests; they do not establish the initial failure's root cause.

**Actual keyboard shortcut press/hold is NOT VERIFIED: computer-helper access was denied.** Registration/conflict/shutdown evidence is not proof of key delivery or no-repeat behavior. Full native acceptance remains pending the manual shortcut check below. No independent review approval is claimed.

### Manual shortcut acceptance (William)

From the folder containing the tested `ossu.exe` artifact, use a nonsynced output folder:

```powershell
.\ossu.exe -out "$env:TEMP\OSSU-manual"
```

1. Wait for `Waiting for Ctrl+Shift+F9`; confirm no automatic captures.
2. Press and release Ctrl+Shift+F9 once. Wait for `Saved`; confirm exactly one new PNG in the output folder.
3. Press the chord again and hold F9 for a few seconds: expect one additional PNG, not repeated captures. Release F9, then repress the chord; expect one more PNG. Wait for each save so the busy-press discard rule does not confound the check.
4. Press Ctrl+C and confirm `Stopped.` and process exit. Restart the same command: it should register without a conflict. Press the shortcut once to confirm another save, then Ctrl+C to stop.

Keep these desktop screenshots private. Report the observed press/hold/repress and restart results; programmatic message delivery is not a substitute for this check.

## Milestone changelog

- Milestone 1: clean Go console implementation; primary-display PNG, DPI-aware native capture, Known Folder output, validated no-repeat global hotkey, once/bounded interval modes, cancellation, exclusive collision-safe files, and focused synthetic tests. No GUI/service or historical-source restoration.

## Research

Historical workflow research is preserved unchanged in [docs/research.md](docs/research.md). Its historical options and future-facing client-bounds discussion are not claims of implemented features. OSSU preserves the folder/hotkey/repeated-capture workflow, not UOSU UI or configuration compatibility. No affiliation with UOSU's developer or Ultima Online.
