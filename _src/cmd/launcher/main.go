// Launch_Spotify.exe
// Fully silent, GUI-subsystem launcher intended for client machines.
// On every run it:
//   1. kills any running Spotify so its files unlock
//   2. ensures %APPDATA%\Spotify       -> <root>\Spotify        (junction)
//   3. ensures %LOCALAPPDATA%\Spotify  -> <root>\SpotifyData    (junction)
//   4. wipes <root>\Spotify\Users so every session starts logged-out
//   5. imports _Reg\Spotify.reg if present
//   6. launches Spotify.exe directly and exits
//
// SpotifyData/ is intentionally NOT wiped: it holds CEF / Chromium state
// that Spotify reads on startup (Default\ profile keys, Local State,
// browser caches). Clearing it leaves Spotify in a half-initialized state
// where it crashes during CEF init with the classic white-window-and-die.
// The actual login token + offline tracks live in Spotify\Users\, so
// wiping that folder alone is enough for a logged-out clean session on
// every launch.
//
// Note on entrypoint: we run Spotify.exe directly rather than
// SpotifyLauncher.exe. The launcher binary is designed for an installed
// Spotify and stalls indefinitely on a portable copy waiting for the
// "Spotify Installer" service / SpotifyStartupTask.exe that don't exist
// here -- the user-visible symptom is a brief white flash and nothing
// else. Spotify.exe boots cleanly on its own and reads the same CEF /
// Apps state from this folder.
//
// No console window, no prompts, no update logic. Use Spotify_Updater.exe
// to perform updates ahead of time on the server side.

package main

import (
	"os"
	"spotify-portable/common"
	"time"
)

func main() {
	p := common.Resolve()

	// Without Spotify.exe there is nothing we can launch. Refuse silently
	// instead of throwing UI at a kiosk client.
	if !common.FileExists(p.SpotifyExe) {
		os.Exit(1)
	}

	// Free any locks on the install / data dirs before we wipe them.
	common.KillSpotify()
	// Give Windows a moment to release file handles from the killed
	// processes; without this, CleanDir(Users) sometimes races a still-
	// closing Spotify.exe and leaves a stale token behind.
	time.Sleep(500 * time.Millisecond)

	// Restore portable links every launch in case the disk was reset.
	// Done before the wipe so EnsureJunction's "migrate when target is
	// empty" path cannot repopulate our portable folders from %APPDATA%
	// or %LOCALAPPDATA% after we clear them.
	_ = common.EnsureJunction(p.SpotifyDir, p.RoamingSpotify)
	_ = common.EnsureJunction(p.DataDir, p.LocalAppSpotify)

	// Mandatory clean session: wipe per-user login token + offline cache
	// only. CEF / Chromium state in SpotifyData/ is preserved so the
	// launcher can boot the app without crashing.
	_ = common.CleanDir(p.UsersDir)

	common.EnsureRegistry(p)

	if err := common.LaunchSpotify(p); err != nil {
		os.Exit(2)
	}
}
