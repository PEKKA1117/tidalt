package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustPreset(t *testing.T, name string) *Keymap {
	t.Helper()
	km, err := Preset(name)
	if err != nil {
		t.Fatalf("Preset(%q): %v", name, err)
	}
	return km
}

func TestPresetBindings(t *testing.T) {
	cases := []struct {
		preset string
		seq    string
		want   Action
	}{
		// tidalt: the historical bindings, unchanged.
		{PresetTidalt, " ", ActResumePause},
		{PresetTidalt, ">", ActNextTrack},
		{PresetTidalt, ".", ActNextTrack},
		{PresetTidalt, "<", ActPreviousTrack},
		{PresetTidalt, "left", ActSeekBackward},
		{PresetTidalt, "right", ActSeekForward},
		{PresetTidalt, "9", ActVolumeDown},
		{PresetTidalt, "0", ActVolumeUp},
		{PresetTidalt, "s", ActShuffle},
		{PresetTidalt, "S", ActSaveQueueAsPlaylist},
		{PresetTidalt, "o", ActShowActionsOnSelectedItem},
		{PresetTidalt, "r", ActGoToRadio},
		{PresetTidalt, "f", ActToggleLiked},
		{PresetTidalt, "a", ActGoToArtist},
		{PresetTidalt, "c", ActCopyLink},
		{PresetTidalt, "t", ActCycleTheme},
		{PresetTidalt, "d", ActSwitchDevice},
		{PresetTidalt, ":", ActOpenCommandPalette},
		{PresetTidalt, "ctrl+p", ActOpenCommandPalette},
		{PresetTidalt, "q", ActQuit},
		{PresetTidalt, "/", ActSearch},
		{PresetTidalt, "n", ActFindNext},
		{PresetTidalt, "N", ActFindPrevious},
		{PresetTidalt, "g g", ActSelectFirst},
		{PresetTidalt, "G", ActSelectLast},
		{PresetTidalt, "?", ActOpenCommandHelp},

		// spotify-player: its documented defaults.
		{PresetSpotifyPlayer, "n", ActNextTrack},
		{PresetSpotifyPlayer, "p", ActPreviousTrack},
		{PresetSpotifyPlayer, " ", ActResumePause},
		{PresetSpotifyPlayer, "ctrl+s", ActShuffle},
		{PresetSpotifyPlayer, "+", ActVolumeUp},
		{PresetSpotifyPlayer, "-", ActVolumeDown},
		{PresetSpotifyPlayer, "_", ActMute},
		{PresetSpotifyPlayer, "^", ActSeekStart},
		{PresetSpotifyPlayer, ">", ActSeekForward},
		{PresetSpotifyPlayer, "<", ActSeekBackward},
		{PresetSpotifyPlayer, "q", ActQuit},
		{PresetSpotifyPlayer, "g g", ActSelectFirst},
		{PresetSpotifyPlayer, "home", ActSelectFirst},
		{PresetSpotifyPlayer, "G", ActSelectLast},
		{PresetSpotifyPlayer, "end", ActSelectLast},
		{PresetSpotifyPlayer, "pgdown", ActPageSelectNext},
		{PresetSpotifyPlayer, "ctrl+f", ActPageSelectNext},
		{PresetSpotifyPlayer, "pgup", ActPageSelectPrevious},
		{PresetSpotifyPlayer, "ctrl+b", ActPageSelectPrevious},
		{PresetSpotifyPlayer, "g a", ActShowActionsOnSelectedItem},
		{PresetSpotifyPlayer, "ctrl+@", ActShowActionsOnSelectedItem},
		{PresetSpotifyPlayer, "a", ActShowActionsOnCurrentTrack},
		{PresetSpotifyPlayer, "Z", ActAddSelectedItemToQueue},
		{PresetSpotifyPlayer, "ctrl+z", ActAddSelectedItemToQueue},
		{PresetSpotifyPlayer, "T", ActSwitchTheme},
		{PresetSpotifyPlayer, "D", ActSwitchDevice},
		{PresetSpotifyPlayer, "/", ActSearch},
		{PresetSpotifyPlayer, "u p", ActBrowseUserPlaylists},
		{PresetSpotifyPlayer, "u a", ActBrowseUserFollowedArtists},
		{PresetSpotifyPlayer, "u A", ActBrowseUserSavedAlbums},
		{PresetSpotifyPlayer, "g y", ActLikedTrackPage},
		{PresetSpotifyPlayer, "g r", ActRecentlyPlayedTrackPage},
		{PresetSpotifyPlayer, "g s", ActSearchPage},
		{PresetSpotifyPlayer, "g m", ActMixesPage},
		{PresetSpotifyPlayer, "z", ActQueue},
		{PresetSpotifyPlayer, "?", ActOpenCommandHelp},
		{PresetSpotifyPlayer, "ctrl+h", ActOpenCommandHelp},
		{PresetSpotifyPlayer, "N", ActSaveQueueAsPlaylist},
		{PresetSpotifyPlayer, ":", ActOpenCommandPalette},
	}
	for _, tc := range cases {
		km := mustPreset(t, tc.preset)
		got, ok := km.Lookup(tc.seq)
		if !ok || got != tc.want {
			t.Errorf("%s: Lookup(%q) = %q, %v; want %q", tc.preset, tc.seq, got, ok, tc.want)
		}
	}
}

// TestSpotifyPresetDropsTidaltOnlyKeys: keys spotify-player gives another
// meaning (or none) must not keep their tidalt binding underneath.
func TestSpotifyPresetDropsTidaltOnlyKeys(t *testing.T) {
	km := mustPreset(t, PresetSpotifyPlayer)
	for _, seq := range []string{".", "9", "0", "s", "o", "t", "d", "left", "right"} {
		if a, ok := km.Lookup(seq); ok {
			t.Errorf("spotify-player: %q is bound to %q, want unbound", seq, a)
		}
	}
}

func TestUnknownPreset(t *testing.T) {
	if _, err := Preset("emacs"); err == nil {
		t.Fatal("Preset(emacs) should fail")
	}
}

func TestIsPrefix(t *testing.T) {
	km := mustPreset(t, PresetSpotifyPlayer)
	for _, seq := range []string{"g", "u"} {
		if !km.IsPrefix(seq) {
			t.Errorf("IsPrefix(%q) = false, want true", seq)
		}
	}
	for _, seq := range []string{"n", "g y", "x"} {
		if km.IsPrefix(seq) {
			t.Errorf("IsPrefix(%q) = true, want false", seq)
		}
	}
}

func TestNormalizeKeySequence(t *testing.T) {
	cases := map[string]string{
		"q":         "q",
		"C-s":       "ctrl+s",
		"ctrl+s":    "ctrl+s",
		"M-enter":   "alt+enter",
		"space":     " ",
		"C-space":   "ctrl+@",
		"page_down": "pgdown",
		"page_up":   "pgup",
		"backtab":   "shift+tab",
		"g a":       "g a",
		"u  A":      "u A",
		"C-c C-x /": "ctrl+c ctrl+x /",
		"esc":       "esc",
		"backspace": "backspace",
	}
	for in, want := range cases {
		got, err := normalizeKeySequence(in)
		if err != nil || got != want {
			t.Errorf("normalizeKeySequence(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "   ", "C-", "M-", "C-ab"} {
		if got, err := normalizeKeySequence(bad); err == nil {
			t.Errorf("normalizeKeySequence(%q) = %q, want error", bad, got)
		}
	}
}

func TestKeyLabel(t *testing.T) {
	cases := map[string]string{
		" ":         "Space",
		"ctrl+s":    "C-s",
		"ctrl+@":    "C-Space",
		"alt+enter": "M-↵",
		"enter":     "↵",
		"g a":       "g a",
		"left":      "←",
		"right":     "→",
		"pgdown":    "PgDn",
		"q":         "q",
	}
	for in, want := range cases {
		if got := keyLabel(in); got != want {
			t.Errorf("keyLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseKeymapDefaultsToTidalt(t *testing.T) {
	km, err := ParseKeymap([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := km.Lookup("."); a != ActNextTrack {
		t.Fatalf("empty config: '.' = %q, want NextTrack (tidalt preset)", a)
	}
}

func TestParseKeymapOverrides(t *testing.T) {
	cfg := `
preset = "spotify-player"

[[keymaps]]
command = "NextTrack"
key_sequence = "g n"

[[keymaps]]
command = "ResumePause"
key_sequence = "M-enter"

[[keymaps]]
command = "None"
key_sequence = "q"

[[keymaps]]
command = "ToggleLiked"
key_sequence = "n"
`
	km, err := ParseKeymap([]byte(cfg))
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]Action{
		"g n":       ActNextTrack,
		"alt+enter": ActResumePause,
		" ":         ActResumePause, // preset binding kept
		"n":         ActToggleLiked, // rebinding replaces the preset's meaning
		"p":         ActPreviousTrack,
	}
	for seq, want := range checks {
		if got, ok := km.Lookup(seq); !ok || got != want {
			t.Errorf("Lookup(%q) = %q, %v; want %q", seq, got, ok, want)
		}
	}
	if a, ok := km.Lookup("q"); ok {
		t.Errorf("q should be unbound by command = \"None\", got %q", a)
	}
	// Keys lists every sequence bound to an action, preset order first.
	if got := km.Keys(ActNextTrack); strings.Join(got, ",") != "g n" {
		t.Errorf("Keys(NextTrack) = %v, want [g n]", got)
	}
}

func TestParseKeymapErrors(t *testing.T) {
	cases := map[string]string{
		"unknown preset":  `preset = "emacs"`,
		"unknown command": "[[keymaps]]\ncommand = \"Explode\"\nkey_sequence = \"x\"",
		"bad key":         "[[keymaps]]\ncommand = \"Quit\"\nkey_sequence = \"C-\"",
		"missing key":     "[[keymaps]]\ncommand = \"Quit\"",
		"prefix conflict": "preset = \"spotify-player\"\n[[keymaps]]\ncommand = \"Quit\"\nkey_sequence = \"g\"",
		"longer conflict": "[[keymaps]]\ncommand = \"Quit\"\nkey_sequence = \"q x\"",
		"bad toml":        "preset = ",
	}
	for name, cfg := range cases {
		if _, err := ParseKeymap([]byte(cfg)); err == nil {
			t.Errorf("%s: ParseKeymap should fail", name)
		}
	}
}

func TestLoadKeymapFile(t *testing.T) {
	dir := t.TempDir()

	km, err := LoadKeymap(filepath.Join(dir, "missing.toml"), PresetTidalt)
	if err != nil {
		t.Fatalf("missing file should fall back to the default preset, got %v", err)
	}
	if a, _ := km.Lookup("."); a != ActNextTrack {
		t.Fatalf("missing file: '.' = %q, want NextTrack", a)
	}

	path := filepath.Join(dir, "keymap.toml")
	if err := os.WriteFile(path, []byte(`preset = "spotify-player"`), 0o600); err != nil {
		t.Fatal(err)
	}
	km, err = LoadKeymap(path, PresetTidalt)
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := km.Lookup("n"); a != ActNextTrack {
		t.Fatalf("spotify file: 'n' = %q, want NextTrack", a)
	}

	if err := os.WriteFile(path, []byte(`preset = "emacs"`), 0o600); err != nil {
		t.Fatal(err)
	}
	km, err = LoadKeymap(path, PresetTidalt)
	if err == nil {
		t.Fatal("invalid file should report an error")
	}
	if km == nil {
		t.Fatal("invalid file should still return the default keymap")
	}
}

func TestKeymapPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got := KeymapPath(); got != "/xdg/tidalt/keymap.toml" {
		t.Fatalf("KeymapPath() = %q", got)
	}
}

// TestKeymapFollowsLayoutPreset: when keymap.toml names no preset, the keys
// follow app.toml's layout preset, so one line there switches both.
func TestKeymapFollowsLayoutPreset(t *testing.T) {
	dir := t.TempDir()

	km, err := LoadKeymap(filepath.Join(dir, "missing.toml"), PresetSpotifyPlayer)
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := km.Lookup("n"); a != ActNextTrack {
		t.Fatalf("no keymap.toml, spotify-player layout: n = %q, want NextTrack", a)
	}

	path := filepath.Join(dir, "keymap.toml")
	override := "[[keymaps]]\ncommand = \"Quit\"\nkey_sequence = \"Q\"\n"
	if err := os.WriteFile(path, []byte(override), 0o600); err != nil {
		t.Fatal(err)
	}
	km, err = LoadKeymap(path, PresetSpotifyPlayer)
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := km.Lookup("n"); a != ActNextTrack {
		t.Fatalf("keymap.toml without preset: n = %q, want spotify-player's NextTrack", a)
	}
	if a, _ := km.Lookup("Q"); a != ActQuit {
		t.Fatalf("override not applied: Q = %q", a)
	}

	// An explicit preset in keymap.toml wins over the layout's.
	if err := os.WriteFile(path, []byte("preset = \"tidalt\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	km, err = LoadKeymap(path, PresetSpotifyPlayer)
	if err != nil {
		t.Fatal(err)
	}
	if a, _ := km.Lookup("."); a != ActNextTrack {
		t.Fatalf("explicit tidalt preset: . = %q, want NextTrack", a)
	}

	// A broken keymap.toml still falls back to the layout's preset.
	if err := os.WriteFile(path, []byte("preset = \"emacs\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	km, err = LoadKeymap(path, PresetSpotifyPlayer)
	if err == nil {
		t.Fatal("broken file should report an error")
	}
	if a, _ := km.Lookup("n"); a != ActNextTrack {
		t.Fatalf("broken file, spotify-player layout: n = %q, want NextTrack", a)
	}
}
