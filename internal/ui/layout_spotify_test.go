package ui

import (
	"strings"
	"testing"

	"github.com/Benehiko/tidalt/v4/internal/tidal"
)

// playingModel is a smoke model with the first track playing, sized so the
// queue's cover panel is shown.
func playingModel(layout Layout) Model {
	m := newSmokeModel()
	m.layout = layout
	m.width, m.height = 160, 40
	tr := m.tracks[0]
	m.currentTrack = &tr
	m.isPlaying = true
	m.duration = 78
	for i := range m.tracks {
		m.tracks[i].Album.Title = "Collide With The Sky"
	}
	return m
}

func viewLines(m Model) []string {
	return strings.Split(stripANSI(m.View()), "\n")
}

// lineOf returns the index of the first view line containing s, or -1.
func lineOf(lines []string, s string) int {
	for i, l := range lines {
		if strings.Contains(l, s) {
			return i
		}
	}
	return -1
}

func TestPlaybackWindowPosition(t *testing.T) {
	for _, tc := range []struct {
		name   string
		layout Layout
		top    bool
	}{
		{"bottom", Layout{}, false},
		{"top", Layout{PlaybackTop: true}, true},
	} {
		m := playingModel(tc.layout)
		lines := viewLines(m)
		if len(lines) != m.height {
			t.Errorf("%s: view has %d lines, want %d", tc.name, len(lines), m.height)
		}
		// The time readout only appears in the playback window.
		at := lineOf(lines, "0:00 / 1:18")
		if at < 0 {
			t.Fatalf("%s: playback window not found", tc.name)
		}
		if tc.top && at >= nowBarH {
			t.Errorf("%s: playback window at line %d, want within the first %d", tc.name, at, nowBarH)
		}
		if !tc.top && at < m.height-nowBarH-footerH {
			t.Errorf("%s: playback window at line %d, want at the bottom", tc.name, at)
		}
		if !strings.Contains(lines[len(lines)-1], "Quit") {
			t.Errorf("%s: last line %q should be the key bar", tc.name, lines[len(lines)-1])
		}
	}
}

// TestPlaybackTopKeepsHeightWhenIdle: with nothing playing, the top playback
// window keeps its full height so the panes below don't jump.
func TestPlaybackTopKeepsHeightWhenIdle(t *testing.T) {
	m := playingModel(Layout{PlaybackTop: true})
	m.currentTrack = nil
	m.isPlaying = false
	lines := viewLines(m)
	if len(lines) != m.height {
		t.Fatalf("idle view has %d lines, want %d", len(lines), m.height)
	}
	if at := lineOf(lines, "QUEUE"); at != nowBarH {
		t.Fatalf("queue panel title at line %d, want %d (just below the playback window)", at, nowBarH)
	}
}

func TestCoverBoxFollowsPlaybackWindow(t *testing.T) {
	bottom, top := playingModel(Layout{}), playingModel(Layout{PlaybackTop: true})
	colB, rowBottom, wB, hB, ok := bottom.coverBoxRect()
	if !ok {
		t.Fatal("cover box should be shown at 160x40")
	}
	colT, rowTop, wT, hT, ok := top.coverBoxRect()
	if !ok {
		t.Fatal("cover box should be shown at 160x40 with the playback window on top")
	}
	if colT != colB || wT != wB || hT != hB {
		t.Fatalf("only the row should move: bottom (%d,%d,%d) vs top (%d,%d,%d)", colB, wB, hB, colT, wT, hT)
	}
	if rowTop != rowBottom+nowBarH {
		t.Fatalf("cover row with the playback window on top = %d, want %d", rowTop, rowBottom+nowBarH)
	}
}

func TestHiddenSidebar(t *testing.T) {
	m := playingModel(Layout{HideSidebar: true})
	sidebarW, mainW := m.layoutDims()
	if sidebarW != 0 || mainW != m.width {
		t.Fatalf("layoutDims = %d, %d; want 0, %d", sidebarW, mainW, m.width)
	}
	if strings.Contains(stripANSI(m.View()), "LIBRARY") {
		t.Fatal("the sidebar should not be drawn")
	}
	// h / Esc would hand focus to the sidebar; with none, focus stays put.
	m, _ = press(t, m, "h", "j")
	if !m.focusMain || m.cursor != 1 {
		t.Fatalf("after h j: focusMain=%v cursor=%d; want focus kept on the list", m.focusMain, m.cursor)
	}
	m, _ = press(t, m, keyEsc, "j")
	if !m.focusMain || m.cursor != 2 {
		t.Fatalf("after esc j: focusMain=%v cursor=%d; want focus kept on the list", m.focusMain, m.cursor)
	}
}

func TestTrackTable(t *testing.T) {
	m := playingModel(Layout{TrackTable: true})
	view := stripANSI(m.View())
	for _, want := range []string{"#", "TITLE", "ARTIST", "ALBUM", "Collide With The Sky", "Pierce The Veil"} {
		if !strings.Contains(view, want) {
			t.Errorf("track table is missing %q", want)
		}
	}

	// The column header stays put while the list scrolls.
	many := make([]tidal.Track, 0, 200)
	for i := range 200 {
		many = append(many, tidal.Track{ID: i + 1, Title: "Track", Duration: 60})
	}
	many[150].Title = "Needle"
	m.tracks = many
	m.cursor = 150
	lines := viewLines(m)
	header := lineOf(lines, "TITLE")
	needle := lineOf(lines, "Needle")
	if header < 0 || needle < 0 || header >= needle {
		t.Fatalf("header at %d, cursor row at %d; want the header above the scrolled list", header, needle)
	}
}

func TestTrackTableOffByDefault(t *testing.T) {
	m := playingModel(Layout{})
	if strings.Contains(stripANSI(m.View()), "ALBUM") {
		t.Fatal("the default layout should not draw the table header")
	}
}

func TestRenderTrackTableRowColumns(t *testing.T) {
	m := playingModel(Layout{TrackTable: true})
	th := m.activeTheme()
	tr := m.tracks[1]
	row := stripANSI(renderTrackTableRow(th, tr, rowOpts{index: 2, width: 100, duration: tr.Duration}))
	header := stripANSI(renderTrackTableHeader(th, 100))
	if w := len([]rune(row)); w != 100 {
		t.Fatalf("row width = %d, want 100: %q", w, row)
	}
	// Each column starts where its header does.
	for _, col := range []struct{ head, cell string }{
		{"TITLE", "Hell Above"},
		{"ARTIST", "Pierce The Veil"},
		{"ALBUM", "Collide With The Sky"},
	} {
		h := strings.Index(header, col.head)
		c := strings.Index(row, col.cell)
		if h < 0 || c < 0 || h != c {
			t.Errorf("%s column: header at %d, cell at %d\n%s\n%s", col.head, h, c, header, row)
		}
	}
	if !strings.HasSuffix(strings.TrimRight(row, " "), "3:32") {
		t.Errorf("duration should be right-aligned: %q", row)
	}
}

func TestPreviousPage(t *testing.T) {
	m := spotifyModel(t)
	m, _ = press(t, m, "g", "y")
	m, _ = press(t, m, "u", "p")
	if m.section != SecPlaylists {
		t.Fatalf("section = %v, want Playlists", m.section)
	}
	m, _ = press(t, m, "backspace")
	if m.section != SecFavSongs {
		t.Fatalf("backspace: section = %v, want Favorite songs", m.section)
	}
	m, _ = press(t, m, "ctrl+q")
	if m.section != SecQueue {
		t.Fatalf("C-q: section = %v, want Queue", m.section)
	}
	m, _ = press(t, m, "backspace")
	if m.section != SecQueue {
		t.Fatalf("backspace with no history: section = %v, want Queue", m.section)
	}
}

func TestPreviousPageSkipsRepeats(t *testing.T) {
	m := spotifyModel(t)
	m, _ = press(t, m, "g", "y", "g", "y", "g", "y")
	m, _ = press(t, m, "backspace")
	if m.section != SecQueue {
		t.Fatalf("section = %v, want Queue (re-opening a page is not history)", m.section)
	}
}

func TestPreviousPageFromSidebarSelection(t *testing.T) {
	m := newSmokeModel()
	m.focusMain = false
	m.sidebarCursor = navIndexOf(SecFavSongs)
	m, _ = press(t, m, keyEnter)
	if m.section != SecFavSongs {
		t.Fatalf("section = %v, want Favorite songs", m.section)
	}
	m, _ = press(t, m, "backspace")
	if m.section != SecQueue {
		t.Fatalf("backspace: section = %v, want Queue", m.section)
	}
}

func TestPreviousPageClosesArtistView(t *testing.T) {
	m := newSmokeModel()
	m, _ = press(t, m, "a") // tidalt: go to the selected track's artist
	if !m.showArtist {
		t.Fatal("a should open the artist view")
	}
	m, _ = press(t, m, "backspace")
	if m.showArtist || m.section != SecQueue {
		t.Fatalf("backspace: showArtist=%v section=%v; want back on the Queue", m.showArtist, m.section)
	}
}

// TestTidaltPresetPageJumps: with the sidebar hidden, the tidalt preset needs
// page keys too; it gains spotify-player's, which collide with nothing.
func TestTidaltPresetPageJumps(t *testing.T) {
	cases := map[string]Section{
		"g y": SecFavSongs, "g r": SecHistory, "g m": SecMixes, "g s": SecSearch,
		"u p": SecPlaylists, "u a": SecFavArtists, "u A": SecFavAlbums,
	}
	for seq, want := range cases {
		m := newSmokeModel()
		m.section = SecMixes
		if want == SecMixes {
			m.section = SecQueue
		}
		m, _ = press(t, m, strings.Split(seq, " ")...)
		if m.section != want {
			t.Errorf("%s: section = %v, want %v", seq, m.section, want)
		}
	}
	m := newSmokeModel()
	m.section = SecFavSongs
	m, _ = press(t, m, "z")
	if m.section != SecQueue {
		t.Fatalf("z: section = %v, want Queue", m.section)
	}
}

func TestFooterWithoutSidebar(t *testing.T) {
	m := playingModel(Layout{HideSidebar: true})
	m.section = SecFavSongs
	footer := stripANSI(m.footerKeyBar(m.activeTheme(), m.width))
	if strings.Contains(footer, "Pane") {
		t.Errorf("footer %q offers switching to a sidebar that is not there", footer)
	}
	if !strings.Contains(footer, "⌫ Back") {
		t.Errorf("footer %q should offer PreviousPage", footer)
	}
}

// TestLeavingThemePickerDropsPreview: jumping away from the theme picker by
// page key or PreviousPage must not leave the previewed theme applied.
func TestLeavingThemePickerDropsPreview(t *testing.T) {
	for _, keys := range [][]string{{"g", "y"}, {"backspace"}} {
		m := newSmokeModel()
		m, _ = press(t, m, "g", "y") // some history for backspace
		m, _ = press(t, m, "z")
		m.focusMain = true
		nm, _ := m.selectSection(SecSettings)
		m = asModel(t, nm)
		m, _ = press(t, m, "j")
		if m.previewPalette == nil {
			t.Fatal("j in the theme picker should preview a theme")
		}
		m, _ = press(t, m, keys...)
		if m.section == SecSettings {
			t.Fatalf("%v should leave the theme picker", keys)
		}
		if m.previewPalette != nil {
			t.Errorf("%v: the previewed theme is still applied", keys)
		}
	}
}
