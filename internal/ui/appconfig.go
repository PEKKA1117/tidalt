package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Layout is the screen arrangement. The zero value is tidalt's own layout:
// sidebar on the left, playback window at the bottom, plain track rows.
type Layout struct {
	PlaybackTop bool // playback window above the panes instead of below
	HideSidebar bool // no sidebar; pages are reached by key or palette
	TrackTable  bool // track lists as a table: # · Title · Artist · Album · Time
	// StartLibrary opens tidalt on the Library page instead of the Queue.
	StartLibrary bool
}

// startSection is the page tidalt opens on.
func startSection(l Layout) Section {
	if l.StartLibrary {
		return SecLibrary
	}
	return SecQueue
}

// layoutPresets are the values `[layout] preset` accepts.
var layoutPresets = map[string]Layout{
	PresetTidalt:        {},
	PresetSpotifyPlayer: {PlaybackTop: true, HideSidebar: true, TrackTable: true, StartLibrary: true},
}

// AppConfig is app.toml: settings that are not key bindings.
type AppConfig struct {
	// Preset is the layout preset's name. keymap.toml falls back to the
	// same key preset when it names none, so one line here sets both.
	Preset string
	Layout Layout
	// FollowIdle is how long the keyboard must be idle on the Queue before
	// its cursor moves back onto the playing track; 0 turns that off.
	FollowIdle time.Duration
}

// defaultFollowIdle is `[queue] follow_idle_sec` when app.toml leaves it unset.
const defaultFollowIdle = 10 * time.Second

// defaultAppConfig is the configuration without an app.toml.
func defaultAppConfig() AppConfig {
	return AppConfig{Preset: PresetTidalt, FollowIdle: defaultFollowIdle}
}

// appConfigFile is the on-disk shape of app.toml. Pointer fields tell "unset"
// (keep the preset's value) from an explicit false.
type appConfigFile struct {
	Layout struct {
		Preset                 string `toml:"preset"`
		PlaybackWindowPosition string `toml:"playback_window_position"`
		Sidebar                *bool  `toml:"sidebar"`
		TrackTable             *bool  `toml:"track_table"`
		StartPage              string `toml:"start_page"`
	} `toml:"layout"`
	Queue struct {
		FollowIdleSec *int `toml:"follow_idle_sec"`
	} `toml:"queue"`
}

// ParseAppConfig reads app.toml contents: the layout preset (tidalt when
// unset), then any individual layout settings on top of it.
func ParseAppConfig(data []byte) (AppConfig, error) {
	var f appConfigFile
	md, err := toml.Decode(string(data), &f)
	if err != nil {
		return AppConfig{}, err
	}
	if und := md.Undecoded(); len(und) > 0 {
		return AppConfig{}, fmt.Errorf("unknown setting %q", und[0].String())
	}
	l := f.Layout
	if l.Preset == "" {
		l.Preset = PresetTidalt
	}
	layout, ok := layoutPresets[l.Preset]
	if !ok {
		return AppConfig{}, fmt.Errorf("unknown layout preset %q (want %q or %q)", l.Preset, PresetTidalt, PresetSpotifyPlayer)
	}
	switch strings.ToLower(l.PlaybackWindowPosition) {
	case "":
	case "top":
		layout.PlaybackTop = true
	case "bottom":
		layout.PlaybackTop = false
	default:
		return AppConfig{}, fmt.Errorf("playback_window_position = %q: want \"Top\" or \"Bottom\"", l.PlaybackWindowPosition)
	}
	if l.Sidebar != nil {
		layout.HideSidebar = !*l.Sidebar
	}
	if l.TrackTable != nil {
		layout.TrackTable = *l.TrackTable
	}
	switch strings.ToLower(l.StartPage) {
	case "":
	case "queue":
		layout.StartLibrary = false
	case "library":
		layout.StartLibrary = true
	default:
		return AppConfig{}, fmt.Errorf("start_page = %q: want \"Queue\" or \"Library\"", l.StartPage)
	}
	followIdle := defaultFollowIdle
	if sec := f.Queue.FollowIdleSec; sec != nil {
		if *sec < 0 {
			return AppConfig{}, fmt.Errorf("follow_idle_sec = %d: want 0 (off) or more seconds", *sec)
		}
		followIdle = time.Duration(*sec) * time.Second
	}
	return AppConfig{Preset: l.Preset, Layout: layout, FollowIdle: followIdle}, nil
}

// AppConfigPath is where tidalt looks for app.toml:
// $XDG_CONFIG_HOME/tidalt/app.toml, usually ~/.config/tidalt/app.toml.
func AppConfigPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "tidalt", "app.toml")
}

// LoadAppConfig reads app.toml. A missing file means the defaults; a broken
// one returns the defaults too, alongside the error.
func LoadAppConfig(path string) (AppConfig, error) {
	if path == "" {
		return defaultAppConfig(), nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // G304: the path is tidalt's own config file, chosen by the user
	if errors.Is(err, fs.ErrNotExist) {
		return defaultAppConfig(), nil
	}
	if err != nil {
		return defaultAppConfig(), err
	}
	cfg, err := ParseAppConfig(data)
	if err != nil {
		return defaultAppConfig(), fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}
