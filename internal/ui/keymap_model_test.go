package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Benehiko/tidalt/v4/internal/player"
	"github.com/Benehiko/tidalt/v4/internal/tidal"
)

// keyMsg builds the tea.KeyMsg whose String() is s, for the keys these tests
// press.
func keyMsg(s string) tea.KeyMsg {
	named := map[string]tea.KeyType{
		keyEnter:  tea.KeyEnter,
		keyEsc:    tea.KeyEsc,
		keyUp:     tea.KeyUp,
		keyDown:   tea.KeyDown,
		keyLeft:   tea.KeyLeft,
		keyRight:  tea.KeyRight,
		" ":       tea.KeySpace,
		"ctrl+c":  tea.KeyCtrlC,
		"ctrl+s":  tea.KeyCtrlS,
		"ctrl+z":  tea.KeyCtrlZ,
		"ctrl+f":  tea.KeyCtrlF,
		"ctrl+b":  tea.KeyCtrlB,
		"ctrl+@":  tea.KeyCtrlAt,
		"pgdown":  tea.KeyPgDown,
		"pgup":    tea.KeyPgUp,
		"home":    tea.KeyHome,
		"end":     tea.KeyEnd,
		"tab":     tea.KeyTab,
		"bksp":    tea.KeyBackspace,
		"ctrl+h":  tea.KeyCtrlH,
		"shift+t": tea.KeyShiftTab,
	}
	if kt, ok := named[s]; ok {
		return tea.KeyMsg{Type: kt}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// press feeds keys through handleKey and returns the model plus the last
// command produced.
func press(t *testing.T, m Model, keys ...string) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range keys {
		msg := keyMsg(k)
		if got := msg.String(); got != k && k != "bksp" && k != "shift+t" {
			t.Fatalf("keyMsg(%q).String() = %q", k, got)
		}
		var next tea.Model
		next, cmd = m.handleKey(msg)
		m = asModel(t, next)
	}
	return m, cmd
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func spotifyModel(t *testing.T) Model {
	t.Helper()
	m := newSmokeModel()
	m.keymap = mustPreset(t, PresetSpotifyPlayer)
	return m
}

func TestSpotifyNextPreviousTrack(t *testing.T) {
	m := spotifyModel(t)
	m, _ = press(t, m, "n")
	if m.cursor != 1 {
		t.Fatalf("n: cursor = %d, want 1 (next track)", m.cursor)
	}
	m, _ = press(t, m, "p")
	if m.cursor != 0 {
		t.Fatalf("p: cursor = %d, want 0 (previous track)", m.cursor)
	}
}

func TestSpotifyPageChords(t *testing.T) {
	cases := []struct {
		keys []string
		want Section
	}{
		{[]string{"g", "y"}, SecFavSongs},
		{[]string{"g", "r"}, SecHistory},
		{[]string{"g", "m"}, SecMixes},
		{[]string{"g", "s"}, SecSearch},
		{[]string{"u", "p"}, SecPlaylists},
		{[]string{"u", "a"}, SecFavArtists},
		{[]string{"u", "A"}, SecFavAlbums},
		{[]string{"T"}, SecSettings},
	}
	for _, tc := range cases {
		m := spotifyModel(t)
		m.section = SecMixes
		if tc.want == SecMixes {
			m.section = SecQueue
		}
		m, _ = press(t, m, tc.keys...)
		if m.section != tc.want {
			t.Errorf("%v: section = %v, want %v", tc.keys, m.section, tc.want)
		}
		if len(m.pendingKeys) != 0 {
			t.Errorf("%v: pending chord %v left over", tc.keys, m.pendingKeys)
		}
	}

	m := spotifyModel(t)
	m.section = SecFavSongs
	m, _ = press(t, m, "z")
	if m.section != SecQueue {
		t.Fatalf("z: section = %v, want Queue", m.section)
	}
}

// TestChordWorksFromSidebar: page chords are global, so they work while the
// sidebar holds focus too.
func TestChordWorksFromSidebar(t *testing.T) {
	m := spotifyModel(t)
	m.focusMain = false
	m, _ = press(t, m, "g", "y")
	if m.section != SecFavSongs || !m.focusMain {
		t.Fatalf("g y from sidebar: section=%v focusMain=%v", m.section, m.focusMain)
	}
}

func TestSpotifyPendingChordAndInvalidChord(t *testing.T) {
	m := spotifyModel(t)
	m.width, m.height = 160, 40
	m, _ = press(t, m, "g")
	if strings.Join(m.pendingKeys, " ") != "g" {
		t.Fatalf("after g: pending = %v, want [g]", m.pendingKeys)
	}
	footer := stripANSI(m.footerKeyBar(m.activeTheme(), m.width))
	for _, want := range []string{"g-", "y Favorite songs", "a Actions on selected track"} {
		if !strings.Contains(footer, want) {
			t.Errorf("pending-chord footer %q is missing %q", footer, want)
		}
	}
	m, _ = press(t, m, "x")
	if len(m.pendingKeys) != 0 {
		t.Fatalf("g x: pending = %v, want cleared", m.pendingKeys)
	}
	if m.cursor != 0 || m.section != SecQueue {
		t.Fatalf("g x should do nothing, cursor=%d section=%v", m.cursor, m.section)
	}
	m, _ = press(t, m, "n")
	if m.cursor != 1 {
		t.Fatalf("n after an invalid chord: cursor = %d, want 1", m.cursor)
	}
}

func TestEscCancelsPendingChord(t *testing.T) {
	m := spotifyModel(t)
	m, _ = press(t, m, "g", keyEsc)
	if len(m.pendingKeys) != 0 {
		t.Fatalf("esc: pending = %v, want cleared", m.pendingKeys)
	}
	if !m.focusMain {
		t.Fatal("esc that cancels a chord should not also leave the main pane")
	}
}

func TestSpotifyListMotions(t *testing.T) {
	m := spotifyModel(t)
	m, _ = press(t, m, "G")
	if m.cursor != 2 {
		t.Fatalf("G: cursor = %d, want 2", m.cursor)
	}
	m, _ = press(t, m, "g", "g")
	if m.cursor != 0 {
		t.Fatalf("gg: cursor = %d, want 0", m.cursor)
	}
	m, _ = press(t, m, "end")
	if m.cursor != 2 {
		t.Fatalf("end: cursor = %d, want 2", m.cursor)
	}
	m, _ = press(t, m, "home")
	if m.cursor != 0 {
		t.Fatalf("home: cursor = %d, want 0", m.cursor)
	}
	m, _ = press(t, m, "/", "k", "i", "n", "g", keyEnter)
	if m.cursor != 2 {
		t.Fatalf("/king: cursor = %d, want 2", m.cursor)
	}
}

func TestPageSelect(t *testing.T) {
	m := spotifyModel(t)
	m.height = 20
	many := make([]tidal.Track, 0, 100)
	for i := range 100 {
		many = append(many, tidal.Track{ID: i + 1, Title: "t"})
	}
	m.tracks = many
	m, _ = press(t, m, "pgdown")
	if m.cursor <= 1 || m.cursor >= 100 {
		t.Fatalf("pgdown: cursor = %d, want a page further", m.cursor)
	}
	page := m.cursor
	m, _ = press(t, m, "ctrl+f")
	if m.cursor != 2*page {
		t.Fatalf("ctrl+f: cursor = %d, want %d", m.cursor, 2*page)
	}
	m, _ = press(t, m, "pgup", "ctrl+b", "ctrl+b")
	if m.cursor != 0 {
		t.Fatalf("paging up past the top: cursor = %d, want 0", m.cursor)
	}
	m, _ = press(t, m, "G", "pgdown")
	if m.cursor != 99 {
		t.Fatalf("paging down past the bottom: cursor = %d, want 99", m.cursor)
	}
}

func TestSpotifyVolumeAndMute(t *testing.T) {
	m := spotifyModel(t)
	m.player = player.NewPlayer()
	m, _ = press(t, m, "+")
	if m.volume != 85 {
		t.Fatalf("+: volume = %v, want 85", m.volume)
	}
	m, _ = press(t, m, "-", "-")
	if m.volume != 75 {
		t.Fatalf("- -: volume = %v, want 75", m.volume)
	}
	m, _ = press(t, m, "_")
	if m.volume != 0 {
		t.Fatalf("_: volume = %v, want 0", m.volume)
	}
	m, _ = press(t, m, "_")
	if m.volume != 75 {
		t.Fatalf("_ again: volume = %v, want 75 restored", m.volume)
	}
}

func TestSpotifyShuffle(t *testing.T) {
	m := spotifyModel(t)
	m, _ = press(t, m, "ctrl+s")
	if m.shuffleMode == ShuffleOff {
		t.Fatal("C-s should cycle shuffle on")
	}
	m, _ = press(t, m, "s")
	if m.shuffleMode == ShuffleOff {
		t.Fatal("s is not shuffle in spotify-player")
	}
}

func TestSpotifyAddSelectedToQueue(t *testing.T) {
	m := spotifyModel(t)
	m.section = SecFavSongs
	m.favSongs = []tidal.Track{{ID: 42, Title: "Bulls In The Bronx"}}
	before := len(m.tracks)
	m, _ = press(t, m, "Z")
	if len(m.tracks) != before+1 || m.tracks[len(m.tracks)-1].ID != 42 {
		t.Fatalf("Z: queue = %d tracks, want %d ending with 42", len(m.tracks), before+1)
	}
}

func TestSpotifyActionsPopups(t *testing.T) {
	m := spotifyModel(t)
	m, _ = press(t, m, "g", "a")
	if m.overlay != OverlayActionSheet || m.sheetTrack == nil || m.sheetTrack.ID != 1 {
		t.Fatalf("g a: overlay = %v, sheet = %v; want actions on the selected track", m.overlay, m.sheetTrack)
	}

	m = spotifyModel(t)
	playing := m.tracks[2]
	m.currentTrack = &playing
	m, _ = press(t, m, "a")
	if m.overlay != OverlayActionSheet || m.sheetTrack == nil || m.sheetTrack.ID != 3 {
		t.Fatalf("a: overlay = %v, sheet = %v; want actions on the playing track", m.overlay, m.sheetTrack)
	}
}

func TestHelpOverlay(t *testing.T) {
	for _, preset := range []string{PresetTidalt, PresetSpotifyPlayer} {
		m := newSmokeModel()
		m.keymap = mustPreset(t, preset)
		m.width, m.height = 120, 50
		m, _ = press(t, m, "?")
		if m.overlay != OverlayHelp {
			t.Fatalf("%s: ? should open the help overlay, got %v", preset, m.overlay)
		}
		view := stripANSI(m.View())
		for _, want := range []string{"Next track", "Resume/pause", "Quit"} {
			if !strings.Contains(view, want) {
				t.Errorf("%s: help overlay is missing %q", preset, want)
			}
		}
		m, _ = press(t, m, keyEsc)
		if m.overlay != OverlayNone {
			t.Fatalf("%s: esc should close help", preset)
		}
	}
}

func TestHelpOverlayShowsPresetKeys(t *testing.T) {
	m := spotifyModel(t)
	m.width, m.height = 120, 50
	rows := stripANSI(strings.Join(m.helpRows(), "\n"))
	for _, want := range []string{"C-s", "g y", "u p"} {
		if !strings.Contains(rows, want) {
			t.Errorf("spotify help is missing key %q", want)
		}
	}
}

// TestTidaltPresetIsDefault: a model built without a keymap keeps the
// historical bindings.
func TestTidaltPresetIsDefault(t *testing.T) {
	m := newSmokeModel()
	m, _ = press(t, m, ".")
	if m.cursor != 1 {
		t.Fatalf(".: cursor = %d, want 1", m.cursor)
	}
	m, _ = press(t, m, "n")
	if m.cursor != 1 {
		t.Fatalf("n without a find query should not move, cursor = %d", m.cursor)
	}
	theme := m.themeName
	m, _ = press(t, m, "t")
	if m.themeName == theme {
		t.Fatal("t should cycle the theme in the tidalt preset")
	}
	m, cmd := press(t, m, "q")
	if !isQuit(cmd) {
		t.Fatal("q should quit")
	}
	_ = m
}

func TestSpotifyQuit(t *testing.T) {
	m := spotifyModel(t)
	if _, cmd := press(t, m, "q"); !isQuit(cmd) {
		t.Fatal("q should quit")
	}
	if _, cmd := press(t, m, "ctrl+c"); !isQuit(cmd) {
		t.Fatal("ctrl+c should quit")
	}
}

// TestTextOverlaysSwallowShortcuts: typing into the Spotify-import URL input
// must not quit or cycle the theme.
func TestTextOverlaysSwallowShortcuts(t *testing.T) {
	m := newSmokeModel()
	m.openImportSpotify()
	theme := m.themeName
	m, cmd := press(t, m, "q", "t", "d")
	if isQuit(cmd) {
		t.Fatal("q in the import input must not quit")
	}
	if m.themeName != theme {
		t.Fatal("t in the import input must not cycle the theme")
	}
	if m.overlay != OverlayImportSpotify || m.importInput.Value() != "qtd" {
		t.Fatalf("overlay = %v, input = %q; want the import input to hold qtd", m.overlay, m.importInput.Value())
	}
	if _, cmd := press(t, m, "ctrl+c"); !isQuit(cmd) {
		t.Fatal("ctrl+c should still quit")
	}
}

func TestFooterReflectsKeymap(t *testing.T) {
	m := spotifyModel(t)
	m.width, m.height = 160, 40
	footer := stripANSI(m.footerKeyBar(m.activeTheme(), m.width))
	for _, want := range []string{"g a Actions", "? Help"} {
		if !strings.Contains(footer, want) {
			t.Errorf("spotify footer %q is missing %q", footer, want)
		}
	}
	m = newSmokeModel()
	m.width, m.height = 160, 40
	footer = stripANSI(m.footerKeyBar(m.activeTheme(), m.width))
	for _, want := range []string{"o Actions", "? Help"} {
		if !strings.Contains(footer, want) {
			t.Errorf("tidalt footer %q is missing %q", footer, want)
		}
	}
}

// TestUserOverrideAppliesToModel: a custom binding from keymap.toml drives the
// model like a preset one.
func TestUserOverrideAppliesToModel(t *testing.T) {
	km, err := ParseKeymap([]byte("preset = \"spotify-player\"\n[[keymaps]]\ncommand = \"NextTrack\"\nkey_sequence = \"g n\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	m := newSmokeModel()
	m.keymap = km
	m, _ = press(t, m, "g", "n")
	if m.cursor != 1 {
		t.Fatalf("g n: cursor = %d, want 1", m.cursor)
	}
}
