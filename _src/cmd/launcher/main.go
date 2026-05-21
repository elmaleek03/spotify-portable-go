// Launch_Spotify.exe
// Fully silent, GUI-subsystem launcher intended for client machines.
// On every run it:
//   1. kills any running Spotify so its files unlock
//   2. ensures %APPDATA%\Spotify       -> <root>\Spotify        (junction)
//   3. ensures %LOCALAPPDATA%\Spotify  -> <root>\SpotifyData    (junction)
//   4. wipes <root>\Spotify\Users so every session starts logged-out
//   5. wipes <root>\SpotifyData so every session starts with a blank cache
//   6. imports _Reg\Spotify.reg if present
//   7. launches Spotify.exe and exits
//
// No console window, no prompts, no update logic. Use Spotify_Updater.exe
// to perform updates ahead of time on the server side.

package main

import (
	"os"
	"spotify-portable/common"
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

	// Restore portable links every launch in case the disk was reset.
	// Done before the wipe so EnsureJunction's "migrate when target is
	// empty" path cannot repopulate our portable folders from %APPDATA%
	// or %LOCALAPPDATA% after we clear them.
	_ = common.EnsureJunction(p.SpotifyDir, p.RoamingSpotify)
	_ = common.EnsureJunction(p.DataDir, p.LocalAppSpotify)

	// Mandatory clean session:
	//   - Spotify\Users  -> per-user login token + offline cache + prefs
	//   - SpotifyData    -> Local cache, browser storage, logs
	// Wiping both yields a logged-out, blank-cache Spotify on every launch
	// while leaving the install binaries (Spotify.exe + resources) intact.
	_ = common.CleanDir(p.UsersDir)
	_ = common.CleanDir(p.DataDir)

	common.EnsureRegistry(p)

	if err := common.LaunchSpotify(p); err != nil {
		os.Exit(2)
	}
}
