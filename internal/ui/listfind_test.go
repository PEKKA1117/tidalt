package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Benehiko/tidalt/v4/internal/tidal"
)

// pressKeys feeds a sequence of key strings through handleKey. Single runes
// are sent as KeyRunes; the named keys below map to their special types.
func pressKeys(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case keyEnter:
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case keyEsc:
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "backspace":
			msg = tea.KeyMsg{Type: tea.KeyBackspace}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, _ := m.handleKey(msg)
		m = asModel(t, next)
	}
	return m
}

// typeKeys splits s into single-rune key presses.
func typeKeys(s string) []string {
	out := make([]string, 0, len(s))
	for _, r := range s {
		out = append(out, string(r))
	}
	return out
}

func artistsModel() Model {
	m := newSmokeModel()
	m.section = SecFavArtists
	m.favArtists = []tidal.Artist{
		{ID: 1, Name: "Pierce The Veil"},
		{ID: 2, Name: "YOASOBI"},
		{ID: 3, Name: "Ado"},
		{ID: 4, Name: "Kenshi Yonezu"},
		{ID: 5, Name: "Adore Delano"},
	}
	return m
}

// TestFindInArtistList is the regression for the reported bug: "/" then
// "ado" in the artist list must land on Ado.
func TestFindInArtistList(t *testing.T) {
	m := artistsModel()
	m = pressKeys(t, m, "/")
	m = pressKeys(t, m, typeKeys("ado")...)
	if got := m.favArtists[m.cursor].Name; got != "Ado" {
		t.Fatalf("after /ado cursor on %q, want Ado", got)
	}
	m = pressKeys(t, m, keyEnter)
	if m.findActive {
		t.Fatal("Enter should close the find prompt")
	}
	if got := m.favArtists[m.cursor].Name; got != "Ado" {
		t.Fatalf("after Enter cursor on %q, want Ado", got)
	}
}

// TestFindTypingIsNotTreatedAsCommands: letters that are global shortcuts
// (q quits, t cycles theme, d opens devices) must be literal in the prompt.
func TestFindTypingIsNotTreatedAsCommands(t *testing.T) {
	m := artistsModel()
	theme := m.themeName
	m = pressKeys(t, m, "/", "t", "d", "q")
	if m.overlay != OverlayNone {
		t.Fatalf("typing in find opened overlay %v", m.overlay)
	}
	if m.themeName != theme {
		t.Fatal("typing t in find cycled the theme")
	}
	if got := m.findInput.Value(); got != "tdq" {
		t.Fatalf("find input = %q, want tdq", got)
	}
}

func TestFindEscRestoresCursor(t *testing.T) {
	m := artistsModel()
	m.cursor = 1
	m = pressKeys(t, m, "/")
	m = pressKeys(t, m, typeKeys("ken")...)
	if m.cursor != 3 {
		t.Fatalf("incremental find cursor = %d, want 3", m.cursor)
	}
	m = pressKeys(t, m, keyEsc)
	if m.findActive {
		t.Fatal("Esc should close the find prompt")
	}
	if m.cursor != 1 {
		t.Fatalf("Esc cursor = %d, want restored 1", m.cursor)
	}
	if !m.focusMain {
		t.Fatal("Esc in the find prompt must not also leave the main pane")
	}
}

func TestFindNextPrevWrap(t *testing.T) {
	m := artistsModel()
	m = pressKeys(t, m, "/")
	m = pressKeys(t, m, typeKeys("ado")...)
	m = pressKeys(t, m, keyEnter)
	if m.cursor != 2 {
		t.Fatalf("first match = %d, want 2 (Ado)", m.cursor)
	}
	m = pressKeys(t, m, "n")
	if m.cursor != 4 {
		t.Fatalf("n = %d, want 4 (Adore Delano)", m.cursor)
	}
	m = pressKeys(t, m, "n")
	if m.cursor != 2 {
		t.Fatalf("n should wrap to 2, got %d", m.cursor)
	}
	m = pressKeys(t, m, "N")
	if m.cursor != 4 {
		t.Fatalf("N should wrap back to 4, got %d", m.cursor)
	}
}

func TestFindNoMatchKeepsCursor(t *testing.T) {
	m := artistsModel()
	m.cursor = 1
	m = pressKeys(t, m, "/")
	m = pressKeys(t, m, typeKeys("zzz")...)
	m = pressKeys(t, m, keyEnter)
	if m.cursor != 1 {
		t.Fatalf("no-match cursor = %d, want 1", m.cursor)
	}
}

func TestGotoTopBottom(t *testing.T) {
	m := artistsModel()
	m.cursor = 2
	m = pressKeys(t, m, "G")
	if m.cursor != len(m.favArtists)-1 {
		t.Fatalf("G cursor = %d, want %d", m.cursor, len(m.favArtists)-1)
	}
	m = pressKeys(t, m, "g")
	if m.cursor != len(m.favArtists)-1 {
		t.Fatal("a single g must not move the cursor")
	}
	m = pressKeys(t, m, "g")
	if m.cursor != 0 {
		t.Fatalf("gg cursor = %d, want 0", m.cursor)
	}
}

// TestFindMatchesTrackArtist: in track lists the query matches the title or
// the artist name.
func TestFindMatchesTrackArtist(t *testing.T) {
	m := newSmokeModel()
	m.section = SecFavSongs
	m.favSongs = []tidal.Track{
		{ID: 1, Title: "Idol", Artist: tidal.Artist{Name: "YOASOBI"}},
		{ID: 2, Title: "Usseewa", Artist: tidal.Artist{Name: "Ado"}},
	}
	m = pressKeys(t, m, "/")
	m = pressKeys(t, m, typeKeys("ado")...)
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want 1 (artist Ado)", m.cursor)
	}
}

// TestFindPerContext covers each list the find/motion keys must reach.
func TestFindPerContext(t *testing.T) {
	tracks := []tidal.Track{
		{ID: 1, Title: "Alpha"}, {ID: 2, Title: "Beta"}, {ID: 3, Title: "Gamma"},
	}
	cases := []struct {
		name   string
		setup  func(*Model)
		cursor func(*Model) int
	}{
		{"queue", func(m *Model) { m.section = SecQueue; m.tracks = tracks }, func(m *Model) int { return m.cursor }},
		{"history", func(m *Model) { m.section = SecHistory; m.history = tracks }, func(m *Model) int { return m.cursor }},
		{"mixes", func(m *Model) {
			m.section = SecMixes
			m.mixes = []tidal.Mix{{Title: "Alpha"}, {Title: "Beta"}, {Title: "Gamma"}}
		}, func(m *Model) int { return m.cursor }},
		{"albums", func(m *Model) {
			m.section = SecFavAlbums
			m.favAlbums = []tidal.Album{{Title: "Alpha"}, {Title: "Beta"}, {Title: "Gamma"}}
		}, func(m *Model) int { return m.cursor }},
		{"playlists", func(m *Model) {
			m.section = SecPlaylists
			m.playlists = []tidal.Playlist{{Title: "Alpha"}, {Title: "Beta"}, {Title: "Gamma"}}
		}, func(m *Model) int { return m.cursor }},
		{"playlist detail", func(m *Model) {
			m.section = SecPlaylists
			m.detailFocus = true
			m.detailTracks = tracks
		}, func(m *Model) int { return m.detailCursor }},
		{"artist albums", func(m *Model) {
			m.showArtist = true
			m.artistAlbums = []tidal.Album{{Title: "Beta"}, {Title: "Gamma"}}
		}, func(m *Model) int { return m.artistCursor }},
		{"artist album tracks", func(m *Model) {
			m.showArtist = true
			m.artistAlbum = &tidal.Album{Title: "X"}
			m.artistAlbumTracks = tracks
		}, func(m *Model) int { return m.artistAlbumCursor }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newSmokeModel()
			tc.setup(&m)
			m = pressKeys(t, m, "/")
			m = pressKeys(t, m, typeKeys("gam")...)
			m = pressKeys(t, m, keyEnter)
			want := 2
			if tc.name == "artist albums" {
				want = 3 // two synthetic rows precede the albums
			}
			if got := tc.cursor(&m); got != want {
				t.Fatalf("find cursor = %d, want %d", got, want)
			}
			m = pressKeys(t, m, "g", "g")
			if got := tc.cursor(&m); got != 0 {
				t.Fatalf("gg cursor = %d, want 0", got)
			}
			m = pressKeys(t, m, "G")
			if got := tc.cursor(&m); got != want {
				t.Fatalf("G cursor = %d, want %d", got, want)
			}
		})
	}
}

func TestFindPromptRenders(t *testing.T) {
	m := artistsModel()
	m.width, m.height = 100, 30
	m = pressKeys(t, m, "/")
	m = pressKeys(t, m, typeKeys("ado")...)
	if v := stripANSI(m.View()); !strings.Contains(v, "/ado") {
		t.Fatalf("view does not show the find prompt:\n%s", v)
	}
}
