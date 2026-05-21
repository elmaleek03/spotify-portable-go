package common

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	SpotifySetupURL = "https://download.scdn.co/SpotifySetup.exe"
	UserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// DownloadFile fetches url to dst, drawing a progress bar labeled `label`.
func DownloadFile(url, dst, label string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Cache-Control", "no-cache")

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("HTTP %s", resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()

	pw := NewProgress(label, resp.ContentLength)
	_, err = io.Copy(f, &CountReader{R: resp.Body, P: pw})
	pw.Done()
	return err
}

// RunInstaller runs SpotifySetup.exe and waits for it to exit.
//
// Spotify's installer has no documented silent flag (unlike Discord's `-s`).
// It does run mostly unattended on its own: it shows a brief progress
// dialog, installs into %APPDATA%\Spotify, then auto-launches Spotify.
// We let it complete, then KillSpotify takes care of the auto-launched
// process so the updater can relocate the install.
func RunInstaller(path string) error {
	cmd := exec.Command(path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// WriteTimestamp writes the current UTC time to a state file.
func WriteTimestamp(path string) {
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	_ = os.WriteFile(path, []byte(time.Now().UTC().Format(time.RFC3339)), 0644)
}
