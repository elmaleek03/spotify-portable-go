//go:build windows

package common

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// detached makes a child run with no console window and detached from us.
func detached() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

// KillSpotify terminates every Spotify-related process.
func KillSpotify() {
	names := []string{
		"Spotify.exe", "SpotifyWebHelper.exe", "SpotifyMigrator.exe",
		"SpotifySetup.exe", "SpotifyStartupTask.exe",
	}
	for _, n := range names {
		cmd := exec.Command("taskkill", "/F", "/IM", n, "/T")
		cmd.SysProcAttr = detached()
		_ = cmd.Run()
	}
}

// IsReparsePoint reports whether p is a junction or symlink.
func IsReparsePoint(p string) bool {
	pPtr, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return false
	}
	attrs, err := syscall.GetFileAttributes(pPtr)
	if err != nil {
		return false
	}
	return attrs&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

// RemoveAll deletes a path. For reparse points it removes only the link.
func RemoveAll(p string) error {
	if IsReparsePoint(p) {
		return os.Remove(p)
	}
	if fi, err := os.Lstat(p); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			return os.Remove(p)
		}
	}
	return os.RemoveAll(p)
}

// MakeJunction creates a directory junction at link pointing to target.
// Uses cmd's mklink because it works without admin and without symlink
// developer-mode privileges.
func MakeJunction(target, link string) error {
	cmd := exec.Command("cmd", "/C", "mklink", "/J", link, target)
	cmd.SysProcAttr = detached()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, string(out))
	}
	return nil
}

// EnsureJunction guarantees `link` is a junction pointing at `target`,
// creating `target` first if it doesn't exist. If `link` already exists as
// a real directory it is moved out of the way (contents copied into target
// when target is empty, then the original removed).
func EnsureJunction(target, link string) error {
	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}
	if IsReparsePoint(link) {
		// Already a reparse point. Trust it.
		return nil
	}
	if FileExists(link) {
		// A real folder is in the way. If our portable target is empty,
		// migrate the data into it; otherwise just remove the stray dir.
		if isDirEmpty(target) {
			_ = copyDirContents(link, target)
		}
		if err := RemoveAll(link); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		return err
	}
	return MakeJunction(target, link)
}

// CleanDir removes every entry inside p but leaves p itself in place.
// If p does not exist it is created. Reparse points found inside p are
// unlinked rather than walked, so a junction inside the dir does not cause
// us to wipe the link's real target. Errors on individual entries are
// collected; the first one is returned but cleanup continues for the rest.
func CleanDir(p string) error {
	if p == "" {
		return nil
	}
	if !FileExists(p) {
		return os.MkdirAll(p, 0755)
	}
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	names, err := f.Readdirnames(-1)
	f.Close()
	if err != nil {
		return err
	}
	var firstErr error
	for _, n := range names {
		if rmErr := RemoveAll(filepath.Join(p, n)); rmErr != nil && firstErr == nil {
			firstErr = rmErr
		}
	}
	return firstErr
}

func isDirEmpty(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return true
	}
	defer f.Close()
	names, _ := f.Readdirnames(1)
	return len(names) == 0
}

func copyDirContents(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

// EnsureRegistry imports the saved .reg file when present.
func EnsureRegistry(p Paths) {
	if !FileExists(p.RegFile) {
		return
	}
	cmd := exec.Command("reg", "import", p.RegFile)
	cmd.SysProcAttr = detached()
	_ = cmd.Run()
}

func RegExport(key, file string) error {
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	cmd := exec.Command("reg", "export", key, file, "/y")
	cmd.SysProcAttr = detached()
	return cmd.Run()
}

func RegDeleteRun(name string) error {
	cmd := exec.Command("reg", "delete",
		`HKEY_CURRENT_USER\SOFTWARE\Microsoft\Windows\CurrentVersion\Run`,
		"/V", name, "/F")
	cmd.SysProcAttr = detached()
	return cmd.Run()
}

// LaunchSpotify starts Spotify and detaches.
//
// On modern Spotify (>= 1.2.x) SpotifyLauncher.exe is *intended* to be the
// bootstrap: it does an integrity check + self-update probe, then hatches
// Spotify.exe. In practice, on the portable layout used here, the launcher
// hangs indefinitely after a single white flash and never spawns
// Spotify.exe -- it tries to talk to the OS-installed `Spotify Installer`
// service / `SpotifyStartupTask.exe` which do not exist on a portable copy,
// and stalls waiting on them. Spotify.exe by itself starts cleanly: it
// reads the same SpotifyData/ CEF state, finds Apps\xpui.spa + login.spa
// in its own directory, and shows the UI within ~1 s.
//
// So: prefer Spotify.exe, only fall back to SpotifyLauncher.exe if the
// main binary is missing (incomplete install). DETACHED_PROCESS keeps the
// child fully unhooked from our launcher so we can exit immediately.
//
// Do NOT set HideWindow:true here. SysProcAttr.HideWindow translates to
// STARTF_USESHOWWINDOW + wShowWindow=SW_HIDE in the child's STARTUPINFO,
// and Spotify reads nCmdShow from there to decide how to bring up its
// main window. With HideWindow set, Spotify boots tray-only -- the
// process is fully alive but the window stays minimized to the tray
// until the user right-clicks the tray icon and picks "Open Spotify".
// Leaving SysProcAttr.HideWindow at its zero value lets the child use
// SW_SHOWDEFAULT and open normally.
func LaunchSpotify(p Paths) error {
	target := p.SpotifyExe
	if !FileExists(target) {
		target = p.LauncherExe
	}
	cmd := exec.Command(target)
	cmd.Dir = filepath.Dir(target)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000008, // DETACHED_PROCESS
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// WaitForRoamingSpotify blocks until %APPDATA%\Spotify\Spotify.exe shows
// up (used after running SpotifySetup.exe).
func WaitForRoamingSpotify(p Paths, d time.Duration) error {
	deadline := time.Now().Add(d)
	target := filepath.Join(p.RoamingSpotify, "Spotify.exe")
	for time.Now().Before(deadline) {
		if FileExists(target) {
			time.Sleep(2 * time.Second)
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("timed out waiting for installer to populate %%APPDATA%%\\Spotify")
}
