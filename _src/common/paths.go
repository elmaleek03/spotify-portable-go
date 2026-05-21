package common

import (
	"os"
	"path/filepath"
)

// Paths bundles every filesystem location the launcher and updater need.
// All portable paths are resolved relative to the executable's own directory
// so the whole folder stays movable across drives.
type Paths struct {
	Root         string // folder containing the exe
	SpotifyDir   string // <root>\Spotify       (install dir, has Spotify.exe)
	DataDir      string // <root>\SpotifyData   (LocalAppData cache + storage)
	SpotifyExe   string // <root>\Spotify\Spotify.exe
	UsersDir     string // <root>\Spotify\Users (per-user login + prefs)
	InstallerDir string // <root>\_Installer
	InstallerExe string // <root>\_Installer\SpotifySetup.exe
	RegDir       string // <root>\_Reg
	RegFile      string // <root>\_Reg\Spotify.reg
	StateDir     string // <root>\_state
	LastUpdate   string // <root>\_state\last_update.txt
	ConfigFile   string // <root>\config.ini

	RoamingSpotify  string // %APPDATA%\Spotify       (install junction target)
	LocalAppSpotify string // %LOCALAPPDATA%\Spotify  (cache junction target)
}

func Resolve() Paths {
	exe, err := os.Executable()
	root := ""
	if err == nil {
		root = filepath.Dir(exe)
	} else {
		root, _ = os.Getwd()
	}
	p := Paths{Root: root}
	p.SpotifyDir = filepath.Join(root, "Spotify")
	p.DataDir = filepath.Join(root, "SpotifyData")
	p.SpotifyExe = filepath.Join(p.SpotifyDir, "Spotify.exe")
	p.UsersDir = filepath.Join(p.SpotifyDir, "Users")
	p.InstallerDir = filepath.Join(root, "_Installer")
	p.InstallerExe = filepath.Join(p.InstallerDir, "SpotifySetup.exe")
	p.RegDir = filepath.Join(root, "_Reg")
	p.RegFile = filepath.Join(p.RegDir, "Spotify.reg")
	p.StateDir = filepath.Join(root, "_state")
	p.LastUpdate = filepath.Join(p.StateDir, "last_update.txt")
	p.ConfigFile = filepath.Join(root, "config.ini")

	p.RoamingSpotify = filepath.Join(os.Getenv("APPDATA"), "Spotify")
	p.LocalAppSpotify = filepath.Join(os.Getenv("LOCALAPPDATA"), "Spotify")
	return p
}

func FileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
