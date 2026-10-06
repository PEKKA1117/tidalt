package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The Library page (SecLibrary, `g l`) shows playlists, favorite artists and
// favorite albums side by side, as spotify-player's Library page does. Spec:
// docs/layout.md "Library page".

// Library page columns, left to right.
const (
	libPlaylists = iota
	libArtists
	libAlbums
	libColumns
)

// Library column titles.
const (
	titlePlaylists = "PLAYLISTS"
	titleArtists   = "ARTISTS"
	titleAlbums    = "ALBUMS"
)

// libNarrowW is the page width below which only the focused column is shown.
const libNarrowW = 60

// libraryColumnWidths splits the page width 40/20/40 between the Playlists,
// Artists and Albums columns; Albums absorbs the rounding.
func libraryColumnWidths(w int) (playlists, artists, albums int) {
	playlists = w * 40 / 100
	artists = w * 20 / 100
	return playlists, artists, w - playlists - artists
}

// libraryLabels returns the findable labels of a Library column.
func (m *Model) libraryLabels(col int) []string {
	var labels []string
	switch col {
	case libPlaylists:
		labels = make([]string, len(m.playlists))
		for i := range m.playlists {
			labels[i] = m.playlists[i].Title
		}
	case libArtists:
		labels = make([]string, len(m.favArtists))
		for i := range m.favArtists {
			labels[i] = m.favArtists[i].Name
		}
	case libAlbums:
		labels = make([]string, len(m.favAlbums))
		for i := range m.favAlbums {
			labels[i] = m.favAlbums[i].Title
		}
	}
	return labels
}

// libraryColumnLen is the number of entries in a Library column.
func (m *Model) libraryColumnLen(col int) int {
	switch col {
	case libPlaylists:
		return len(m.playlists)
	case libArtists:
		return len(m.favArtists)
	case libAlbums:
		return len(m.favAlbums)
	default:
		return 0
	}
}

// loadLibrary batches the loaders of whichever Library lists are not loaded
// yet; nil when all three are.
func (m *Model) loadLibrary() tea.Cmd {
	var cmds []tea.Cmd
	for _, sec := range []Section{SecPlaylists, SecFavArtists, SecFavAlbums} {
		if cmd := m.loadSection(sec); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// updateLibrary handles the Library page's keys: focus moves between the
// columns (wrapping), j/k move the focused column's cursor, and Enter opens
// the entry under it. Anything else resolves through the keymap as a
// playback or track action, which acts on the playing track here.
func (m Model) updateLibrary(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	col := m.libFocus
	switch k.String() {
	case keyTab, "l", keyRight:
		m.libFocus = (col + 1) % libColumns
		return m, nil
	case keyShiftTab, "h", keyLeft:
		m.libFocus = (col + libColumns - 1) % libColumns
		return m, nil
	case keyUp, "k":
		if m.libCursor[col] > 0 {
			m.libCursor[col]--
		}
		return m, nil
	case keyDown, "j":
		if m.libCursor[col] < m.libraryColumnLen(col)-1 {
			m.libCursor[col]++
		}
		return m, nil
	case keyEnter:
		return m.openLibraryEntry()
	}
	return m.commonKeys(k)
}

// openLibraryEntry opens the entry under the focused column's cursor: a
// playlist on the Playlists page with its tracks focused, an artist in the
// artist view, an album's tracks in the queue.
func (m Model) openLibraryEntry() (tea.Model, tea.Cmd) {
	col := m.libFocus
	i := m.libCursor[col]
	if i < 0 || i >= m.libraryColumnLen(col) {
		return m, nil
	}
	switch col {
	case libPlaylists:
		pl := m.playlists[i]
		next, cmd := m.selectSection(SecPlaylists)
		nm, ok := next.(Model)
		if !ok {
			return next, cmd
		}
		nm.cursor = i
		nm.openPlaylist = &pl
		return nm, tea.Batch(cmd, nm.loadPlaylistDetail(pl))
	case libArtists:
		a := m.favArtists[i]
		return m.openArtistByID(a.ID, a.Name)
	default:
		cmd := m.openAlbum(m.favAlbums[i].ID)
		return m, cmd
	}
}

// renderLibraryPane draws the Library page: the three columns side by side,
// or only the focused one, full width, below libNarrowW.
func (m *Model) renderLibraryPane(t Theme, w, h int) string {
	if w < libNarrowW {
		return m.renderLibraryColumn(t, m.libFocus, w, h)
	}
	pw, aw, alw := libraryColumnWidths(w)
	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderLibraryColumn(t, libPlaylists, pw, h),
		m.renderLibraryColumn(t, libArtists, aw, h),
		m.renderLibraryColumn(t, libAlbums, alw, h),
	)
}

// renderLibraryColumn draws one Library column as a list panel; the focused
// column gets the accent border and the selection band.
func (m *Model) renderLibraryColumn(t Theme, col, w, h int) string {
	focused := m.focusMain && col == m.libFocus
	cursor := m.libCursor[col]
	innerW := max(w-2, 1)
	switch col {
	case libPlaylists:
		rows := playlistRows(t, m.playlists, cursor, focused, innerW)
		return renderListPanel(t, titlePlaylists, focused, rows, cursor*2, w, h)
	case libArtists:
		rows := artistRows(t, m.favArtists, cursor, focused, innerW)
		return renderListPanel(t, titleArtists, focused, rows, cursor, w, h)
	default:
		rows := albumRows(t, m.favAlbums, cursor, focused, innerW)
		return renderListPanel(t, titleAlbums, focused, rows, cursor, w, h)
	}
}
