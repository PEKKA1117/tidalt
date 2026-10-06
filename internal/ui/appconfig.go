package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Layout is the screen arrangement. The zero value is tidalt's own layout:
// sidebar on the left, playback window at the bottom, plain track rows.
type Layout struct {
	PlaybackTop bool // playback window above the panes instead of below
	HideSidebar bool // no sidebar; pages are reached by key or palette
	TrackTable  bool // track lists as a table: # · Title · Artist · Album · Time
}

// layoutPresets are the values `[layout] preset` accepts.
var layoutPresets = map[string]Layout{
	PresetTidalt:        {},
	PresetSpotifyPlayer: {PlaybackTop: true, HideSidebar: true, TrackTable: true},
}

// AppConfig is app.toml: settings that are not key bindings.
type AppConfig struct {
	Layout Layout
}

// appConfigFile is the on-disk shape of app.toml. Pointer fields tell "unset"
// (keep the preset's value) from an explicit false.
type appConfigFile struct {
	Layout struct {
		Preset                 string `toml:"preset"`
		PlaybackWindowPosition string `toml:"playback_window_position"`
		Sidebar                *bool  `toml:"sidebar"`
		TrackTable             *bool  `toml:"track_table"`
	} `toml:"layout"`
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
	return AppConfig{Layout: layout}, nil
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
		return AppConfig{}, nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // G304: the path is tidalt's own config file, chosen by the user
	if errors.Is(err, fs.ErrNotExist) {
		return AppConfig{}, nil
	}
	if err != nil {
		return AppConfig{}, err
	}
	cfg, err := ParseAppConfig(data)
	if err != nil {
		return AppConfig{}, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}
