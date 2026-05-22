// Spotify_Updater.exe
// Server-side update tool with terminal progress.
//
// Behavior (single unified flow, runs the same on first install and
// every subsequent update):
//
//   1. Stop any running Spotify so its files unlock.
//   2. Ensure %APPDATA%\Spotify       -> <root>\Spotify       junction.
//      Ensure %LOCALAPPDATA%\Spotify  -> <root>\SpotifyData   junction.
//      On first run, if a real Spotify install already exists at
//      %APPDATA%\Spotify, EnsureJunction migrates it into the portable
//      folder before swapping in the junction.
//   3. Download a fresh SpotifySetup.exe with a progress bar.
//   4. Run the installer. Because of the junction, the installer writes
//      into <root>\Spotify directly, so no manual relocation is needed.
//      The installer auto-launches Spotify when it finishes.
//   5. Kill the installer-launched Spotify so the install dir unlocks.
//   6. Re-export HKCU\SOFTWARE\Classes\spotify and scrub autostart
//      entries the installer may have re-added.
//
// This replaces the older "open Spotify and wait 120 s for its
// self-updater" approach, which was unreliable: Spotify's built-in
// updater does not always run on each launch, so most update cycles
// did nothing and just re-opened the app.

package main

import (
	"fmt"
	"os"
	"spotify-portable/common"
	"time"
)

func main() {
	p := common.Resolve()

	fmt.Println("=========================================")
	fmt.Println("  Spotify Portable - Updater")
	fmt.Println("=========================================")
	fmt.Println()

	if err := run(p); err != nil {
		die(err)
	}

	fmt.Println()
	fmt.Println("[+] Update complete. Closing in 5s...")
	time.Sleep(5 * time.Second)
}

func run(p common.Paths) error {
	// Make sure every portable folder we care about exists before any
	// step touches it.
	for _, d := range []string{p.SpotifyDir, p.DataDir, p.StateDir, p.InstallerDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	fmt.Println("[1/6] Stopping any running Spotify...")
	common.KillSpotify()
	time.Sleep(1 * time.Second)

	// Junctions go up before the installer runs so the installer's
	// writes to %APPDATA%\Spotify and %LOCALAPPDATA%\Spotify land in
	// our portable folders. EnsureJunction handles three cases:
	//   - link missing               -> create junction
	//   - link is real folder, empty -> migrate then swap
	//   - link is already a junction -> leave alone
	fmt.Println("[2/6] Ensuring portable junctions...")
	if err := common.EnsureJunction(p.SpotifyDir, p.RoamingSpotify); err != nil {
		return fmt.Errorf("install junction: %w", err)
	}
	if err := common.EnsureJunction(p.DataDir, p.LocalAppSpotify); err != nil {
		return fmt.Errorf("data junction: %w", err)
	}

	fmt.Println("[3/6] Downloading Spotify installer...")
	if common.FileExists(p.InstallerExe) {
		_ = os.Remove(p.InstallerExe)
	}
	if err := common.DownloadFile(common.SpotifySetupURL, p.InstallerExe, "Download"); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	// SpotifySetup.exe shows its own GUI progress dialog, so we just
	// run it and wait for it to exit. It auto-launches Spotify when
	// finished; we kill that in the next step. WaitForRoamingSpotify
	// is a safety net for the rare case where SpotifySetup.exe returns
	// before Spotify.exe is fully on disk.
	fmt.Println("[4/6] Running installer (this can take 30-60s)...")
	_ = common.RunInstaller(p.InstallerExe)
	if err := common.WaitForRoamingSpotify(p, 120*time.Second); err != nil {
		return err
	}

	fmt.Println("[5/6] Stopping installer-launched Spotify...")
	common.KillSpotify()
	time.Sleep(2 * time.Second)
	// Second sweep covers child processes that respawned during the
	// first taskkill (Spotify likes to relaunch itself once on close).
	common.KillSpotify()

	fmt.Println("[6/6] Refreshing registry export and disabling autostart...")
	_ = common.RegExport(`HKEY_CURRENT_USER\SOFTWARE\Classes\spotify`, p.RegFile)
	_ = common.RegDeleteRun("Spotify")
	_ = common.RegDeleteRun("SpotifyWebHelper")

	common.WriteTimestamp(p.LastUpdate)

	if !common.FileExists(p.SpotifyExe) {
		return fmt.Errorf("Spotify.exe missing after install at %s", p.SpotifyExe)
	}
	return nil
}

func die(err error) {
	fmt.Println()
	fmt.Println("[!] Error:", err)
	fmt.Println("Closing in 10s...")
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
