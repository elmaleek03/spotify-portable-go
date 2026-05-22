<div align="center">
  <img src="assets/logo.png" alt="Spotify Portable" width="128" height="128" />

  <h1>Spotify Portable (Go)</h1>

  <p>
    <strong>One-folder, virtual-disk-friendly Spotify for kiosks.</strong><br/>
    Silent client launcher + admin updater, written in Go.
  </p>
</div>

---

A two-binary, fully portable Spotify setup for kiosks and virtual disks that
reset to a clean state on reboot. The whole client is kept in one folder so
admin updates and end-user launches stay separate, predictable, and silent.

## Why

Standard Spotify installs scatter files across `%APPDATA%\Spotify` (binaries
+ per-user login + offline cache) and `%LOCALAPPDATA%\Spotify` (browser
storage + logs). On a kiosk PC where the system drive resets to a clean
state on reboot, this makes the install vanish along with logins, settings,
and cache every restart. This project keeps both folders inside the portable
project root and links them in via NTFS junctions every time the launcher
runs, so a reboot does not reset Spotify state.

## What you get

| File                    | Audience | Window                | Purpose                                                                      |
| ----------------------- | -------- | --------------------- | ---------------------------------------------------------------------------- |
| `Launch_Spotify.exe`    | Clients  | None (GUI subsystem)  | Restores junctions + registry, wipes login state, launches Spotify, exits.|
| `Spotify_Updater.exe`   | Admin    | Console + progress    | Downloads SpotifySetup.exe and runs it through the portable junctions, every run. |

The two binaries are designed to be the only thing anyone runs.
`Launch_Spotify.exe` is intentionally silent (no console, no prompts, no
update logic) so end users get a one-click experience.

## Folder layout (after first run)

```
spotify-portable-go/
  Launch_Spotify.exe      <- silent launcher (clients run this)
  Spotify_Updater.exe     <- terminal updater (admin runs this)
  Spotify/                <- portable install dir (junction target, has Spotify.exe)
  SpotifyData/            <- portable Local cache (browser storage, logs)
  _Installer/             <- cached SpotifySetup.exe
  _Reg/                   <- exported HKCU\SOFTWARE\Classes\spotify
  _state/                 <- last_update.txt timestamp
  _src/                   <- Go source (common pkg + cmd/launcher + cmd/updater)
  build.bat               <- rebuild both binaries
```

Only the two `.exe` files, `_src/`, `build.bat`, `LICENSE`, `.gitignore`, and
this `README.md` are tracked in git. Everything else is generated at first
run and ignored by `.gitignore`.

## Junctions used

| Symlink                   | Points to             | What lives there                       |
| ------------------------- | --------------------- | -------------------------------------- |
| `%APPDATA%\Spotify`       | `<root>\Spotify`      | Install (Spotify.exe + resources) and per-user `Users\` data |
| `%LOCALAPPDATA%\Spotify`  | `<root>\SpotifyData`  | Browser storage, cache, logs           |

Both junctions are rebuilt by every launch, so a wiped C: drive on the next
reboot is fine.

## Quick start

### 1. First-time setup (admin, once)

1. Drop `spotify-portable-go/` onto your portable volume.
2. Double-click `Spotify_Updater.exe`.
3. Wait for the progress bar. It will:
   - create `Spotify/` and `SpotifyData/` if missing,
   - replace `%APPDATA%\Spotify` with a junction to `Spotify/` and
     `%LOCALAPPDATA%\Spotify` with a junction to `SpotifyData/`
     (migrating any existing data first if the portable folders are empty),
   - download `SpotifySetup.exe`,
   - run the installer; because the junctions are already in place,
     `SpotifySetup.exe` writes straight into `Spotify/` and `SpotifyData/`,
   - kill the auto-launched Spotify so the install dir unlocks,
   - export `HKCU\SOFTWARE\Classes\spotify` to `_Reg\Spotify.reg`,
   - delete Spotify's autostart `Run` entries.

When it closes, the portable install is ready.

### 2. Daily client use

Users double-click `Launch_Spotify.exe`. Nothing else.

No console window appears. The launcher kills any running Spotify, rebuilds
both junctions, **wipes `Spotify\Users\` so every session starts logged-out
(no login, no offline tracks)**, imports the registry, starts Spotify via
`Spotify.exe`, and exits.

`SpotifyData/` (CEF / Chromium browser state) is intentionally preserved
across sessions because Spotify reads it on startup; wiping it leaves
Spotify crashing in CEF init with the classic white-window-then-exit. The
install binaries (`Spotify.exe` + `SpotifyLauncher.exe` + resources) are
also preserved so updates persist even though login state is wiped.

### 3. Periodic updates (admin)

Run `Spotify_Updater.exe` again on the admin schedule of your choice (manual,
Task Scheduler, on logon, whatever). The flow is the same as first-time
setup:

1. Stop any running Spotify.
2. Ensure both junctions still exist (`%APPDATA%\Spotify` -> `Spotify\`,
   `%LOCALAPPDATA%\Spotify` -> `SpotifyData\`).
3. Download a fresh `SpotifySetup.exe` with a progress bar.
4. Run the installer. Because the junctions are already in place, every
   file the installer writes to `%APPDATA%\Spotify` lands in the portable
   `Spotify\` folder.
5. Kill the installer-launched Spotify so the install dir unlocks.
6. Refresh the registry export and disable autostart entries.

That's the entire update workflow. The terminal closes with the updater.

## Command-line behaviour

Both binaries take zero arguments. Behaviour is fully driven by which
binary you run and the state of the project root.

`Spotify_Updater.exe`:

- Always: kill Spotify, ensure junctions, download `SpotifySetup.exe`,
  run it, kill the installer-launched Spotify, refresh the registry
  export. Same flow on first install and on every later run.

`Launch_Spotify.exe`:

- If `Spotify/Spotify.exe` is missing -> exit 1 silently (admin must run the
  updater first).
- Otherwise -> kill any running Spotify, ensure junctions, wipe
  `Spotify\Users\` for a logged-out clean session, import reg, start
  Spotify via `Spotify.exe`, exit.

## Building from source

Requirements: Go 1.21+ and the `rsrc` resource tool.

```bat
go install github.com/akavel/rsrc@latest
build.bat
```

`build.bat`:

1. Copies `_src/app.ico` into both `cmd/*` directories.
2. Generates `rsrc_amd64.syso` (icon + manifest) for each binary.
3. Builds `Launch_Spotify.exe` with `-H windowsgui` (no console).
4. Builds `Spotify_Updater.exe` as a console app.

Output binaries land in the project root. The launcher is ~3 MB, the updater
is ~5 MB.

## Source map

```
_src/
  go.mod
  app.ico                       canonical icon source
  common/
    paths.go                    portable path resolution + AppData targets
    console.go                  AllocConsole + ANSI VT for the GUI updater
    progress.go                 \r progress bar with smoothed speed + m:ss countdown formatter
    winutil.go                  taskkill, mklink /J, reg import/export, EnsureJunction with empty-target migration
    download.go                 HTTP download with progress + installer runner
  cmd/
    launcher/main.go            silent client launcher
    launcher/app.manifest       Win32 manifest (DPI / UTF-8 / asInvoker)
    updater/main.go             unified install/update flow (download SpotifySetup.exe + run via junction)
    updater/app.manifest        Win32 manifest
```

## Notes

- The launcher is GUI subsystem so antivirus / SmartScreen sees a clean
  windowless executable, no flashing console. Sign it if you ship widely.
- Junctions need NTFS. They do not work on FAT32 / exFAT volumes.
- The launcher does not require admin. Junctions are created with `mklink /J`
  which works for the current user.
- Spotify's installer (`SpotifySetup.exe`) has no documented silent flag
  unlike Discord's `-s`. It runs mostly unattended on its own and
  auto-launches Spotify when finished; the updater kills that
  auto-launched process at the end of every run.
- The updater downloads and runs the installer on every invocation
  rather than relying on Spotify's in-app self-updater. The portable
  junctions (`%APPDATA%\Spotify` -> `Spotify\`, `%LOCALAPPDATA%\Spotify`
  -> `SpotifyData\`) make `SpotifySetup.exe` write straight into the
  portable folders, so no manual relocation step is needed.
- First run migrates anything currently in `%APPDATA%\Spotify` /
  `%LOCALAPPDATA%\Spotify` into `Spotify/` / `SpotifyData/` if the portable
  folders are empty, so an existing logged-in user does not have to log in
  again on the portable copy (until the next clean-session launch wipes it).
- Unlike Discord, the install dir (`Spotify/`) is NOT wiped on launch
  because Spotify's executable lives there; only the `Users\` subfolder
  (login token + offline tracks) is wiped. `SpotifyData/` (CEF/Chromium
  state) is also preserved across sessions: Spotify reads it during CEF
  init and wiping it causes the classic white-window crash.
- The launcher entrypoint is `Spotify.exe` directly, not
  `SpotifyLauncher.exe`. The launcher binary is written for the standard
  installed copy of Spotify and tries to talk to the OS-level
  `Spotify Installer` service / `SpotifyStartupTask.exe` for self-update
  on startup. Neither exists on a portable copy, so `SpotifyLauncher.exe`
  shows a brief white flash and then stalls forever without ever
  spawning `Spotify.exe`. `Spotify.exe` on its own reads the same
  CEF/Chromium state from `SpotifyData/` and the same `Apps\xpui.spa` +
  `Apps\login.spa` from `Spotify/`, and shows the UI within ~1 second.

## License

MIT. See `LICENSE`.

## Credits

Companion project to `discord-portable-go`. Same workflow as two specialized
executables, adapted for Spotify's `%APPDATA%`-based install layout.
