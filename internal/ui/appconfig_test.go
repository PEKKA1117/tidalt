package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAppConfigDefaults(t *testing.T) {
	cfg, err := ParseAppConfig([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Layout != (Layout{}) {
		t.Fatalf("empty config: layout = %+v, want the zero (tidalt) layout", cfg.Layout)
	}
}

func TestParseAppConfigSpotifyPreset(t *testing.T) {
	cfg, err := ParseAppConfig([]byte("[layout]\npreset = \"spotify-player\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := Layout{PlaybackTop: true, HideSidebar: true, TrackTable: true}
	if cfg.Layout != want {
		t.Fatalf("spotify-player layout = %+v, want %+v", cfg.Layout, want)
	}
}

func TestParseAppConfigOverrides(t *testing.T) {
	cfg, err := ParseAppConfig([]byte(`
[layout]
preset = "spotify-player"
playback_window_position = "Bottom"
sidebar = true
`))
	if err != nil {
		t.Fatal(err)
	}
	want := Layout{TrackTable: true}
	if cfg.Layout != want {
		t.Fatalf("layout = %+v, want %+v", cfg.Layout, want)
	}

	cfg, err = ParseAppConfig([]byte("[layout]\nplayback_window_position = \"top\"\ntrack_table = true\nsidebar = false\n"))
	if err != nil {
		t.Fatal(err)
	}
	want = Layout{PlaybackTop: true, HideSidebar: true, TrackTable: true}
	if cfg.Layout != want {
		t.Fatalf("layout = %+v, want %+v", cfg.Layout, want)
	}
}

func TestParseAppConfigErrors(t *testing.T) {
	cases := map[string]string{
		"unknown preset":   "[layout]\npreset = \"winamp\"",
		"unknown position": "[layout]\nplayback_window_position = \"Left\"",
		"unknown setting":  "[layout]\nsidebar_width = 3",
		"wrong type":       "[layout]\nsidebar = \"no\"",
		"bad toml":         "[layout",
	}
	for name, cfg := range cases {
		if _, err := ParseAppConfig([]byte(cfg)); err == nil {
			t.Errorf("%s: ParseAppConfig should fail", name)
		}
	}
}

func TestLoadAppConfig(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadAppConfig(filepath.Join(dir, "missing.toml"))
	if err != nil || cfg.Layout != (Layout{}) {
		t.Fatalf("missing file: %+v, %v; want defaults and no error", cfg, err)
	}

	path := filepath.Join(dir, "app.toml")
	if err := os.WriteFile(path, []byte("[layout]\nsidebar = false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadAppConfig(path)
	if err != nil || !cfg.Layout.HideSidebar {
		t.Fatalf("sidebar = false: %+v, %v", cfg, err)
	}

	if err := os.WriteFile(path, []byte("[layout]\npreset = \"winamp\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadAppConfig(path)
	if err == nil {
		t.Fatal("a broken file should report an error")
	}
	if cfg.Layout != (Layout{}) {
		t.Fatalf("a broken file should fall back to defaults, got %+v", cfg.Layout)
	}
}

func TestAppConfigPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got := AppConfigPath(); got != "/xdg/tidalt/app.toml" {
		t.Fatalf("AppConfigPath() = %q", got)
	}
}
