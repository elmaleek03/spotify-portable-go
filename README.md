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
| `Launch_Spotify.exe`    | Clients  | None (GUI subsystem)  | Restores junctions + registry, wipes user/cache state, launches Spotify, exits.|
| `Spotify_Updater.exe`   | Admin    | Console + progress    | First-time install, then a 120 s self-update window for later runs.          |

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
   - download `SpotifySetup.exe`,
   - run the installer,
   - kill the auto-launched Spotify so the install dir unlocks,
   - copy the install into `Spotify/`,
   - replace `%APPDATA%\Spotify` with a junction to `Spotify/`,
   - create `SpotifyData/` and replace `%LOCALAPPDATA%\Spotify` with a
     junction to it (migrating any existing data if `SpotifyData/` is empty),
   - export `HKCU\SOFTWARE\Classes\spotify` to `_Reg\Spotify.reg`,
   - delete Spotify's autostart `Run` entries.

When it closes, the portable install is ready.

### 2. Daily client use

Users double-click `Launch_Spotify.exe`. Nothing else.

No console window appears. The launcher kills any running Spotify, rebuilds
both junctions, **wipes `Spotify\Users\` and `SpotifyData/` so every session
starts clean (no login, no offline cache, no logs)**, imports the registry,
starts Spotify, and exits.

The install binaries (`Spotify.exe` + resources) are preserved across
sessions so updates persist even though user data is wiped.

### 3. Periodic updates (admin)

Run `Spotify_Updater.exe` again on the admin schedule of your choice (manual,
Task Scheduler, on logon, whatever). It will:

1. Ensure both junctions still exist.
2. Kill any running Spotify.
3. Launch Spotify so its built-in auto-updater downloads the new version
   into `Spotify/`.
4. Show a 120 second countdown bar (`[==========    ] 60% Updating 72/120`).
5. Kill every Spotify process when the timer hits zero.
6. Refresh the registry export and exit.

That's the entire update workflow. The terminal closes with the updater.

## Command-line behaviour

Both binaries take zero arguments. Behaviour is fully driven by which
binary you run and the state of the project root.

`Spotify_Updater.exe`:

- If `Spotify/Spotify.exe` is missing -> first-time setup mode.
- Otherwise -> launch + 120 s window + kill all Spotify processes.

`Launch_Spotify.exe`:

- If `Spotify/Spotify.exe` is missing -> exit 1 silently (admin must run the
  updater first).
- Otherwise -> kill any running Spotify, ensure junctions, wipe
  `Spotify\Users\` and `SpotifyData/` for a clean session, import reg, start
  Spotify, exit.

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
    progress.go                 \r progress bar with smoothed speed (also drives the 120s countdown)
    winutil.go                  taskkill, mklink /J, reg import/export, EnsureJunction with empty-target migration
    download.go                 HTTP download with progress + installer runner
  cmd/
    launcher/main.go            silent client launcher
    launcher/app.manifest       Win32 manifest (DPI / UTF-8 / asInvoker)
    updater/main.go             first-time setup + 120s update window
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
  auto-launches Spotify when finished; the updater kills that auto-launched
  process before relocating the install.
- The 120 second update window is a fixed wait. If your network is slow you
  can extend it by editing the `sleepSeconds` constant in
  `_src/cmd/updater/main.go` and rebuilding.
- First run migrates anything currently in `%APPDATA%\Spotify` /
  `%LOCALAPPDATA%\Spotify` into `Spotify/` / `SpotifyData/` if the portable
  folders are empty, so an existing logged-in user does not have to log in
  again on the portable copy (until the next clean-session launch wipes it).
- Unlike Discord, the install dir (`Spotify/`) is NOT wiped on launch
  because Spotify's executable lives there; only the `Users\` subfolder
  (login token + offline tracks) is wiped. `SpotifyData/` (cache) is wiped
  in full.

## License

MIT. See `LICENSE`.

## Credits

Companion project to `discord-portable-go`. Same workflow as two specialized
executables, adapted for Spotify's `%APPDATA%`-based install layout.
