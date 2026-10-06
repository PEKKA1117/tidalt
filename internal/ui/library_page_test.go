package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Benehiko/tidalt/v4/internal/tidal"
)

// plRoadTrip is the first playlist in libraryModel.
const plRoadTrip = "Road trip"

// libKey builds a key message for the Library tests, including shift+tab,
// which keyMsg does not spell the same way.
func libKey(s string) tea.KeyMsg {
	if s == keyShiftTab {
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	}
	return keyMsg(s)
}

// libPress feeds keys through handleKey and returns the model and the last
// command.
func libPress(t *testing.T, m Model, keys ...string) (Model, tea.Cmd) {
	t.Helper()
	var cmd tea.Cmd
	for _, k := range keys {
		msg := libKey(k)
		if got := msg.String(); got != k {
			t.Fatalf("libKey(%q).String() = %q", k, got)
		}
		var next tea.Model
		next, cmd = m.handleKey(msg)
		m = asModel(t, next)
	}
	return m, cmd
}

// libraryModel is a smoke model on the Library page with all three lists
// loaded.
func libraryModel() Model {
	m := newSmokeModel()
	m.width, m.height = 120, 30
	m.playlists = []tidal.Playlist{
		{UUID: "p1", Title: plRoadTrip, NumberOfTracks: 42},
		{UUID: "p2", Title: "Focus", NumberOfTracks: 7},
		{UUID: "p3", Title: "Workout", NumberOfTracks: 12},
	}
	m.favArtists = []tidal.Artist{
		{ID: 11, Name: "Bonobo"},
		{ID: 12, Name: "Floating Points"},
	}
	m.favAlbums = []tidal.Album{
		{ID: 21, Title: "Migration", ReleaseDate: "2017-01-13", NumberOfTracks: 12},
		{ID: 22, Title: "Promises", ReleaseDate: "2021-03-26", NumberOfTracks: 9},
		{ID: 23, Title: "Crush", ReleaseDate: "2019-10-18", NumberOfTracks: 13},
	}
	m.section = SecLibrary
	return m
}

func TestLibraryPageKeyOpensLibrary(t *testing.T) {
	for _, preset := range []string{PresetTidalt, PresetSpotifyPlayer} {
		t.Run(preset, func(t *testing.T) {
			m := newSmokeModel()
			m.keymap = mustPreset(t, preset)
			m, _ = libPress(t, m, "g", "l")
			if m.section != SecLibrary {
				t.Fatalf("g l: section = %v, want SecLibrary", m.section)
			}
			if !m.focusMain {
				t.Error("g l should focus the main pane")
			}
			if n := len(m.pageHistory); n == 0 || m.pageHistory[n-1] != SecQueue {
				t.Errorf("g l should record the page left, history = %v", m.pageHistory)
			}
		})
	}
}

func TestLibraryFocusCycles(t *testing.T) {
	cases := []struct {
		name string
		keys []string
		want int
	}{
		{"opens on playlists", nil, libPlaylists},
		{keyTab, []string{keyTab}, libArtists},
		{"tab tab", []string{keyTab, keyTab}, libAlbums},
		{"tab wraps", []string{keyTab, keyTab, keyTab}, libPlaylists},
		{"l", []string{"l"}, libArtists},
		{keyRight, []string{keyRight}, libArtists},
		{"shift+tab wraps", []string{keyShiftTab}, libAlbums},
		{"h wraps", []string{"h"}, libAlbums},
		{"left wraps", []string{keyLeft}, libAlbums},
		{"h after tab", []string{keyTab, "h"}, libPlaylists},
	}
	for _, preset := range []string{PresetTidalt, PresetSpotifyPlayer} {
		for _, c := range cases {
			t.Run(preset+"/"+c.name, func(t *testing.T) {
				m := libraryModel()
				m.keymap = mustPreset(t, preset)
				m, _ = libPress(t, m, c.keys...)
				if m.libFocus != c.want {
					t.Errorf("libFocus = %d, want %d", m.libFocus, c.want)
				}
				if m.section != SecLibrary || !m.focusMain {
					t.Errorf("focus keys must stay on the Library main pane (section %v, focusMain %v)", m.section, m.focusMain)
				}
			})
		}
	}
}

func TestLibraryCursorPerColumn(t *testing.T) {
	m := libraryModel()
	m, _ = libPress(t, m, "j", "j", "j", "j") // clamps at the last playlist
	if m.libCursor[libPlaylists] != 2 {
		t.Errorf("playlists cursor = %d, want 2", m.libCursor[libPlaylists])
	}
	m, _ = libPress(t, m, keyTab, "j")
	if m.libCursor[libArtists] != 1 || m.libCursor[libPlaylists] != 2 || m.libCursor[libAlbums] != 0 {
		t.Errorf("j on artists moved the wrong cursor: %v", m.libCursor)
	}
	m, _ = libPress(t, m, "k", "k")
	if m.libCursor[libArtists] != 0 {
		t.Errorf("artists cursor = %d, want 0", m.libCursor[libArtists])
	}
	m, _ = libPress(t, m, keyTab, "G")
	if m.libCursor[libAlbums] != 2 {
		t.Errorf("G on albums: cursor = %d, want 2", m.libCursor[libAlbums])
	}
	m, _ = libPress(t, m, "g", "g")
	if m.libCursor[libAlbums] != 0 {
		t.Errorf("gg on albums: cursor = %d, want 0", m.libCursor[libAlbums])
	}
	if m.libCursor[libPlaylists] != 2 {
		t.Errorf("motion on albums disturbed playlists cursor: %v", m.libCursor)
	}
}

func TestLibraryFindInFocusedColumn(t *testing.T) {
	m := libraryModel()
	m, _ = libPress(t, m, keyTab, keyTab)
	m = pressKeys(t, m, append(append([]string{"/"}, typeKeys("crush")...), keyEnter)...)
	if m.libCursor[libAlbums] != 2 {
		t.Errorf("find 'crush' in albums: cursor = %d, want 2", m.libCursor[libAlbums])
	}
	if m.libCursor[libPlaylists] != 0 || m.libCursor[libArtists] != 0 {
		t.Errorf("find moved another column: %v", m.libCursor)
	}
}

func TestLibraryRemembersFocusAndCursors(t *testing.T) {
	m := libraryModel()
	m.pageHistory = []Section{SecQueue}
	m, _ = libPress(t, m, keyTab, "j") // artists, cursor 1
	m, _ = libPress(t, m, "z")         // leave for the Queue
	if m.section != SecQueue {
		t.Fatalf("z: section = %v, want SecQueue", m.section)
	}
	m, _ = libPress(t, m, "g", "l")
	if m.section != SecLibrary {
		t.Fatalf("g l: section = %v", m.section)
	}
	if m.libFocus != libArtists {
		t.Errorf("libFocus after return = %d, want artists", m.libFocus)
	}
	if m.libCursor[libArtists] != 1 {
		t.Errorf("artists cursor after return = %d, want 1", m.libCursor[libArtists])
	}
}

func TestLibraryEnterPlaylist(t *testing.T) {
	m := libraryModel()
	m, _ = libPress(t, m, "j")
	m, cmd := libPress(t, m, keyEnter)
	if m.section != SecPlaylists {
		t.Fatalf("Enter on a playlist: section = %v, want SecPlaylists", m.section)
	}
	if m.openPlaylist == nil || m.openPlaylist.UUID != "p2" {
		t.Fatalf("openPlaylist = %+v, want p2", m.openPlaylist)
	}
	if m.cursor != 1 {
		t.Errorf("playlists index cursor = %d, want 1", m.cursor)
	}
	if cmd == nil {
		t.Error("Enter on a playlist should load its tracks")
	}
	next, _ := m.Update(playlistDetailMsg{uuid: "p2", title: "Focus", tracks: m.tracks})
	m = asModel(t, next)
	if !m.detailFocus {
		t.Error("the playlist's tracks should be focused once loaded")
	}
	m, _ = libPress(t, m, "backspace")
	if m.section != SecLibrary {
		t.Errorf("Backspace: section = %v, want SecLibrary", m.section)
	}
	if m.libCursor[libPlaylists] != 1 {
		t.Errorf("playlists cursor after Backspace = %d, want 1", m.libCursor[libPlaylists])
	}
}

func TestLibraryEnterArtist(t *testing.T) {
	m := libraryModel()
	m, cmd := libPress(t, m, keyTab, "j", keyEnter)
	if !m.showArtist {
		t.Fatal("Enter on an artist should open the artist view")
	}
	if cmd == nil {
		t.Error("Enter on an artist should load its albums")
	}
	if m.prevSection != SecLibrary {
		t.Errorf("prevSection = %v, want SecLibrary", m.prevSection)
	}
	m, _ = libPress(t, m, "backspace")
	if m.showArtist || m.section != SecLibrary {
		t.Errorf("Backspace: showArtist %v section %v, want the Library", m.showArtist, m.section)
	}
}

func TestLibraryEnterAlbum(t *testing.T) {
	m := libraryModel()
	m, cmd := libPress(t, m, keyShiftTab, "j", keyEnter)
	if cmd == nil {
		t.Fatal("Enter on an album should load its tracks")
	}
	if m.section != SecLibrary {
		t.Errorf("section before the tracks arrive = %v, want SecLibrary", m.section)
	}
	next, _ := m.Update(tracksMsg(m.tracks))
	m = asModel(t, next)
	if m.section != SecQueue {
		t.Fatalf("album tracks: section = %v, want SecQueue", m.section)
	}
	m, _ = libPress(t, m, "backspace")
	if m.section != SecLibrary {
		t.Errorf("Backspace: section = %v, want SecLibrary", m.section)
	}
	if m.libFocus != libAlbums || m.libCursor[libAlbums] != 1 {
		t.Errorf("after Backspace focus %d cursor %v, want albums/1", m.libFocus, m.libCursor)
	}
}

func TestLibraryActiveList(t *testing.T) {
	m := libraryModel()
	cases := []struct {
		focus int
		first string
		n     int
	}{
		{libPlaylists, plRoadTrip, 3},
		{libArtists, "Bonobo", 2},
		{libAlbums, "Migration", 3},
	}
	for _, c := range cases {
		m.libFocus = c.focus
		labels, cur := m.activeList()
		if len(labels) != c.n || labels[0] != c.first {
			t.Errorf("focus %d: labels = %v", c.focus, labels)
		}
		if cur != &m.libCursor[c.focus] {
			t.Errorf("focus %d: cursor pointer is not the column's cursor", c.focus)
		}
	}
}

func TestLibrarySelectedTrackIsCurrent(t *testing.T) {
	m := libraryModel()
	if got := m.selectedTrack(); got != nil {
		t.Errorf("nothing playing: selectedTrack = %+v, want nil", got)
	}
	tr := m.tracks[2]
	m.currentTrack = &tr
	if got := m.selectedTrack(); got == nil || got.ID != tr.ID {
		t.Errorf("selectedTrack = %+v, want the playing track", got)
	}
}

func TestLibraryLoadsMissingLists(t *testing.T) {
	m := libraryModel()
	if cmd := m.loadSection(SecLibrary); cmd != nil {
		t.Error("all lists loaded: loadSection(SecLibrary) should be nil")
	}
	for _, clear := range []func(*Model){
		func(m *Model) { m.playlists = nil },
		func(m *Model) { m.favArtists = nil },
		func(m *Model) { m.favAlbums = nil },
	} {
		mm := libraryModel()
		clear(&mm)
		if cmd := mm.loadSection(SecLibrary); cmd == nil {
			t.Error("a missing list should be loaded")
		}
	}
}

func TestLibraryRender(t *testing.T) {
	titles := []string{titlePlaylists, titleArtists, titleAlbums}
	cases := []struct {
		name  string
		w     int
		focus int
		want  []string
	}{
		{"wide", 100, libPlaylists, titles},
		{"exactly 60", 60, libArtists, titles},
		{"narrow playlists", 59, libPlaylists, []string{titlePlaylists}},
		{"narrow artists", 40, libArtists, []string{titleArtists}},
		{"narrow albums", 40, libAlbums, []string{titleAlbums}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := libraryModel()
			m.libFocus = c.focus
			out := stripANSI(m.renderLibraryPane(m.theme, c.w, 12))
			for _, title := range titles {
				want := strings.Contains(strings.Join(c.want, " "), title)
				if got := strings.Contains(out, " "+title+" "); got != want {
					t.Errorf("title %s shown = %v, want %v\n%s", title, got, want, out)
				}
			}
			for i, ln := range strings.Split(out, "\n") {
				if lw := len([]rune(ln)); lw != c.w {
					t.Errorf("line %d width %d, want %d", i, lw, c.w)
				}
			}
		})
	}

	m := libraryModel()
	out := stripANSI(m.renderLibraryPane(m.theme, 100, 12))
	for _, s := range []string{plRoadTrip, "42 tracks", "◎ Bonobo", "⊞ Migration (2017)"} {
		if !strings.Contains(out, s) {
			t.Errorf("wide render missing %q\n%s", s, out)
		}
	}
}

func TestLibraryColumnWidths(t *testing.T) {
	cases := []struct{ w, p, a, al int }{
		{100, 40, 20, 40},
		{101, 40, 20, 41},
		{63, 25, 12, 26},
	}
	for _, c := range cases {
		p, a, al := libraryColumnWidths(c.w)
		if p != c.p || a != c.a || al != c.al {
			t.Errorf("libraryColumnWidths(%d) = %d,%d,%d want %d,%d,%d", c.w, p, a, al, c.p, c.a, c.al)
		}
	}
}

func TestLibraryEmptyColumns(t *testing.T) {
	m := libraryModel()
	m.playlists, m.favArtists, m.favAlbums = nil, nil, nil
	out := stripANSI(m.renderLibraryPane(m.theme, 120, 10))
	for _, s := range []string{"No playlists.", "No favorite artists.", "No favorite albums."} {
		if !strings.Contains(out, s) {
			t.Errorf("empty Library missing %q\n%s", s, out)
		}
	}
}

func TestLibraryViewInBothLayouts(t *testing.T) {
	for _, layout := range []Layout{{}, {HideSidebar: true, PlaybackTop: true, TrackTable: true}} {
		m := libraryModel()
		m.layout = layout
		for _, w := range []int{30, 70, 160} {
			m.width = w
			out := stripANSI(m.View())
			if !strings.Contains(out, titlePlaylists) {
				t.Errorf("layout %+v width %d: Library view lacks PLAYLISTS", layout, w)
			}
		}
	}
}

func TestPaletteHasGoToLibrary(t *testing.T) {
	for _, it := range allPaletteItems() {
		if it.label != "Go to Library" {
			continue
		}
		m := newSmokeModel()
		next, _ := it.run(m)
		if asModel(t, next).section != SecLibrary {
			t.Error("Go to Library should open the Library")
		}
		return
	}
	t.Error(`command palette has no "Go to Library"`)
}

func TestLibraryHelpListsAction(t *testing.T) {
	info, ok := actionIndex[ActLibraryPage]
	if !ok || info.group != groupPages || info.desc != "Library" {
		t.Errorf("ActLibraryPage action info = %+v, %v", info, ok)
	}
}
