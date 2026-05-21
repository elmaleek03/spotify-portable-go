// Spotify_Updater.exe
// Server-side update tool with terminal progress.
//
// Behavior:
//   - On first run (no Spotify\Spotify.exe yet): downloads SpotifySetup.exe
//     with a progress bar, runs the installer, copies the resulting install
//     into the portable Spotify\ folder, builds the junctions, exports the
//     registry key.
//   - On subsequent runs: ensures junctions, opens Spotify so its built-in
//     auto-updater can self-update, sleeps 120 seconds with a countdown
//     bar, then kills every Spotify process and exits (the terminal closes
//     with it).

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"spotify-portable/common"
	"time"
)

const sleepSeconds = 120

func main() {
	p := common.Resolve()

	fmt.Println("=========================================")
	fmt.Println("  Spotify Portable - Updater")
	fmt.Println("=========================================")
	fmt.Println()

	// Keep portable folders ready before anything else touches them.
	if err := os.MkdirAll(p.SpotifyDir, 0755); err != nil {
		die(err)
	}
	if err := os.MkdirAll(p.DataDir, 0755); err != nil {
		die(err)
	}
	if err := os.MkdirAll(p.StateDir, 0755); err != nil {
		die(err)
	}

	if !common.FileExists(p.SpotifyExe) {
		if err := firstTimeSetup(p); err != nil {
			die(err)
		}
		fmt.Println()
		fmt.Println("[+] Setup complete. Closing in 5s...")
		time.Sleep(5 * time.Second)
		return
	}

	// Subsequent runs: trigger Spotify's own self-updater by launching it.
	if err := refreshExisting(p); err != nil {
		die(err)
	}
}

func firstTimeSetup(p common.Paths) error {
	fmt.Println("[1/6] First run detected. Stopping any Spotify processes...")
	common.KillSpotify()
	time.Sleep(1 * time.Second)

	fmt.Println("[2/6] Cleaning %APPDATA%\\Spotify ...")
	if common.FileExists(p.RoamingSpotify) {
		_ = common.RemoveAll(p.RoamingSpotify)
	}

	fmt.Println("[3/6] Downloading Spotify installer...")
	if err := os.MkdirAll(p.InstallerDir, 0755); err != nil {
		return err
	}
	if common.FileExists(p.InstallerExe) {
		_ = os.Remove(p.InstallerExe)
	}
	if err := common.DownloadFile(common.SpotifySetupURL, p.InstallerExe, "Download"); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	fmt.Println("[4/6] Running installer. This can take up to a minute...")
	// The Spotify installer auto-launches Spotify when it finishes, so we
	// kill those processes after RunInstaller returns so the install dir
	// is unlocked and we can relocate it.
	_ = common.RunInstaller(p.InstallerExe)
	if err := common.WaitForRoamingSpotify(p, 90*time.Second); err != nil {
		return err
	}
	common.KillSpotify()
	time.Sleep(2 * time.Second)

	fmt.Println("[5/6] Building portable folder + junctions...")
	if err := relocateInstall(p); err != nil {
		return err
	}
	if err := common.EnsureJunction(p.DataDir, p.LocalAppSpotify); err != nil {
		return fmt.Errorf("data junction: %w", err)
	}

	fmt.Println("[6/6] Exporting registry and disabling autostart...")
	_ = common.RegExport(`HKEY_CURRENT_USER\SOFTWARE\Classes\spotify`, p.RegFile)
	_ = common.RegDeleteRun("Spotify")
	_ = common.RegDeleteRun("SpotifyWebHelper")

	common.WriteTimestamp(p.LastUpdate)
	if !common.FileExists(p.SpotifyExe) {
		return fmt.Errorf("Spotify.exe still missing after setup")
	}
	return nil
}

// relocateInstall copies %APPDATA%\Spotify into <root>\Spotify and
// replaces the original with a junction pointing back.
func relocateInstall(p common.Paths) error {
	src := p.RoamingSpotify
	if !common.FileExists(src) {
		return fmt.Errorf("install source missing: %s", src)
	}

	// Count bytes for the progress bar.
	var total int64
	_ = filepath.Walk(src, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	pw := common.NewProgress("Copying ", total)

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(p.SpotifyDir, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, info.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer out.Close()
		buf := make([]byte, 256*1024)
		for {
			n, rerr := in.Read(buf)
			if n > 0 {
				if _, werr := out.Write(buf[:n]); werr != nil {
					return werr
				}
				pw.Current += int64(n)
				pw.Draw(false)
			}
			if rerr != nil {
				if errors.Is(rerr, io.EOF) {
					return nil
				}
				return rerr
			}
		}
	})
	pw.Done()
	if err != nil {
		return err
	}

	if err := common.RemoveAll(src); err != nil {
		return err
	}
	return common.MakeJunction(p.SpotifyDir, src)
}

func refreshExisting(p common.Paths) error {
	fmt.Println("[1/4] Ensuring junctions...")
	if err := common.EnsureJunction(p.SpotifyDir, p.RoamingSpotify); err != nil {
		return fmt.Errorf("install junction: %w", err)
	}
	if err := common.EnsureJunction(p.DataDir, p.LocalAppSpotify); err != nil {
		return fmt.Errorf("data junction: %w", err)
	}
	common.EnsureRegistry(p)

	fmt.Println("[2/4] Stopping any running Spotify...")
	common.KillSpotify()
	time.Sleep(2 * time.Second)

	fmt.Printf("[3/4] Launching Spotify to self-update (%ds window)...\n", sleepSeconds)
	if err := common.LaunchSpotify(p); err != nil {
		return fmt.Errorf("launch: %w", err)
	}
	countdown(sleepSeconds)

	fmt.Println()
	fmt.Println("[4/4] Closing all Spotify processes...")
	common.KillSpotify()
	time.Sleep(1 * time.Second)
	common.KillSpotify()

	common.WriteTimestamp(p.LastUpdate)

	// Re-export the registry in case Spotify rewrote it during this session.
	_ = common.RegExport(`HKEY_CURRENT_USER\SOFTWARE\Classes\spotify`, p.RegFile)
	_ = common.RegDeleteRun("Spotify")
	_ = common.RegDeleteRun("SpotifyWebHelper")

	fmt.Println()
	fmt.Println("[+] Update window finished. Closing...")
	time.Sleep(2 * time.Second)
	return nil
}

func countdown(seconds int) {
	pw := common.NewProgress("Updating", int64(seconds))
	for i := 1; i <= seconds; i++ {
		time.Sleep(1 * time.Second)
		pw.Current = int64(i)
		pw.Draw(true)
	}
	pw.Done()
}

func die(err error) {
	fmt.Println()
	fmt.Println("[!] Error:", err)
	fmt.Println("Closing in 10s...")
	time.Sleep(10 * time.Second)
	os.Exit(1)
}
