package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Benehiko/tidalt/v4/internal/tidal"
)

// recordHistory prepends a track to the Recently Played list (de-duplicated,
// most-recent first, capped).
func (m *Model) recordHistory(t tidal.Track) {
	const maxHistory = 100
	out := make([]tidal.Track, 0, len(m.history)+1)
	out = append(out, t)
	for i := range m.history {
		if m.history[i].ID == t.ID {
			continue
		}
		out = append(out, m.history[i])
		if len(out) >= maxHistory {
			break
		}
	}
	m.history = out
	_ = m.store.SaveHistory(m.history)
}

// --- Playlists (index + detail) ---

func (m Model) updatePlaylists(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.detailFocus {
		return m.updatePlaylistDetail(k)
	}
	switch k.String() {
	case "h", keyLeft:
		m.focusMain = false
		return m, nil
	case keyUp, "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case keyDown, "j":
		if m.cursor < len(m.playlists)-1 {
			m.cursor++
		}
	case keyEnter, "l", keyRight:
		if m.cursor >= 0 && m.cursor < len(m.playlists) {
			pl := m.playlists[m.cursor]
			m.openPlaylist = &pl
			cmd := m.loadPlaylistDetail(pl)
			return m, cmd
		}
	}
	return m, nil
}

func (m Model) updatePlaylistDetail(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "h", keyLeft:
		m.detailFocus = false
		return m, nil
	case keyUp, "k":
		if m.detailCursor > 0 {
			m.detailCursor--
		}
		return m, nil
	case keyDown, "j":
		if m.detailCursor < len(m.detailTracks)-1 {
			m.detailCursor++
		}
		return m, nil
	case keyEnter:
		// Load the whole playlist into the queue, tracking its origin, and play
		// from the selected track.
		return m.playPlaylistFrom(m.detailCursor)
	}
	return m.commonKeys(k)
}

func (m *Model) loadPlaylistDetail(pl tidal.Playlist) tea.Cmd {
	uuid := pl.UUID
	title := pl.Title
	return func() tea.Msg {
		tracks, err := m.client.GetPlaylistTracks(m.ctx, uuid)
		if err != nil {
			return errMsg(err)
		}
		return playlistDetailMsg{uuid: uuid, title: title, tracks: tracks}
	}
}

// playPlaylistFrom loads the open playlist's tracks into the queue (tracking
// its origin for the hybrid model) and plays from index i.
func (m Model) playPlaylistFrom(i int) (tea.Model, tea.Cmd) {
	if len(m.detailTracks) == 0 || m.openPlaylist == nil {
		return m, nil
	}
	m.loadQueueFromPlaylist(m.detailTracks, *m.openPlaylist)
	if i < 0 || i >= len(m.tracks) {
		i = 0
	}
	m.cursor = i
	m.jumpToQueue()
	m.sidebarCursor = navIndexOf(SecQueue)
	track := m.tracks[i]
	_ = m.store.CacheTrack(track.ID, track)
	cmd := m.playTrackCmd(track)
	return m, cmd
}

// --- Favorite artists ---

func (m Model) updateFavArtists(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "h", keyLeft:
		m.focusMain = false
	case keyUp, "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case keyDown, "j":
		if m.cursor < len(m.favArtists)-1 {
			m.cursor++
		}
	case keyEnter:
		if m.cursor >= 0 && m.cursor < len(m.favArtists) {
			a := m.favArtists[m.cursor]
			return m.openArtistByID(a.ID, a.Name)
		}
	}
	return m, nil
}

// openArtistByID opens the artist drill-down for an explicit artist.
func (m Model) openArtistByID(id int, name string) (tea.Model, tea.Cmd) {
	if id == 0 {
		return m, nil
	}
	m.prevSection = m.section
	m.artistLoading = true
	m.showArtist = true
	return m, func() tea.Msg {
		albums, err := m.client.GetArtistAlbums(m.ctx, id)
		if err != nil {
			return errMsg(err)
		}
		return artistAlbumsMsg{artistID: id, artistName: name, albums: albums}
	}
}

// --- Favorite albums ---

func (m Model) updateFavAlbums(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "h", keyLeft:
		m.focusMain = false
	case keyUp, "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case keyDown, "j":
		if m.cursor < len(m.favAlbums)-1 {
			m.cursor++
		}
	case keyEnter:
		if m.cursor >= 0 && m.cursor < len(m.favAlbums) {
			cmd := m.openAlbum(m.favAlbums[m.cursor].ID)
			return m, cmd
		}
	}
	return m, nil
}

// --- History ---

func (m Model) updateHistory(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "h", keyLeft:
		m.focusMain = false
		return m, nil
	case keyUp, "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case keyDown, "j":
		if m.cursor < len(m.history)-1 {
			m.cursor++
		}
		return m, nil
	case keyEnter:
		cmd := m.playListIntoQueue(m.history, m.cursor)
		return m, cmd
	}
	return m.commonKeys(k)
}

// --- Render panes ---

// renderPlaylistsPane draws the playlist index (left) and the open playlist's
// detail (right) side by side.
func (m *Model) renderPlaylistsPane(t Theme, w, h int) string {
	idxW := min(max(w/3, 22), 30)
	detailW := w - idxW

	idxRows := playlistRows(t, m.playlists, m.cursor, !m.detailFocus, idxW-2)
	indexPanel := renderListPanel(t, "PLAYLISTS", !m.detailFocus, idxRows, m.cursor*2, idxW, h)

	title := "SELECT A PLAYLIST"
	if m.openPlaylist != nil {
		title = strings.ToUpper(m.playlistName)
	}
	detailPanel := m.renderTrackList(t, trackList{
		title:   title,
		tracks:  m.detailTracks,
		cursor:  m.detailCursor,
		focused: m.detailFocus,
		index:   true,
		empty:   "Press → to open a playlist.",
	}, detailW, h)

	return lipgloss.JoinHorizontal(lipgloss.Top, indexPanel, detailPanel)
}

// updateFavSongs handles the Favorite Songs section. Enter loads the favorites
// into the queue and plays from the selected track.
func (m Model) updateFavSongs(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "h", keyLeft:
		m.focusMain = false
		return m, nil
	case keyUp, "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil
	case keyDown, "j":
		if m.cursor < len(m.favSongs)-1 {
			m.cursor++
		}
		return m, nil
	case keyEnter:
		cmd := m.playListIntoQueue(m.favSongs, m.cursor)
		return m, cmd
	}
	return m.commonKeys(k)
}

// renderFavSongsPane lists the favorite songs.
func (m *Model) renderFavSongsPane(t Theme, w, h int) string {
	return m.renderTrackList(t, trackList{
		title:   "SONGS",
		tracks:  m.favSongs,
		cursor:  m.cursor,
		focused: m.focusMain,
		artist:  true,
		allFav:  true,
		empty:   "No favorite songs.",
	}, w, h)
}

// renderFavArtistsPane lists favorited artists.
func (m *Model) renderFavArtistsPane(t Theme, w, h int) string {
	rows := artistRows(t, m.favArtists, m.cursor, m.focusMain, max(w-2, 1))
	return renderListPanel(t, "ARTISTS", m.focusMain, rows, m.cursor, w, h)
}

// renderFavAlbumsPane lists favorited albums.
func (m *Model) renderFavAlbumsPane(t Theme, w, h int) string {
	rows := albumRows(t, m.favAlbums, m.cursor, m.focusMain, max(w-2, 1))
	return renderListPanel(t, "ALBUMS", m.focusMain, rows, m.cursor, w, h)
}

// renderHistoryPane lists recently played tracks.
func (m *Model) renderHistoryPane(t Theme, w, h int) string {
	return m.renderTrackList(t, trackList{
		title:   "RECENTLY PLAYED",
		tracks:  m.history,
		cursor:  m.cursor,
		focused: m.focusMain,
		artist:  true,
		empty:   "Nothing played yet this session.",
	}, w, h)
}

// listRowStyle returns the cursor marker and name style for a list row: the
// accent "›" marker when the row is under the cursor of the focused list.
func listRowStyle(t Theme, on bool) (cur string, style lipgloss.Style) {
	if on {
		return t.RowPlaying.Render("› "), t.RowPlaying
	}
	return "  ", t.Row
}

// selBand wraps a row under the focused cursor in the selection band.
func selBand(t Theme, line string, on bool, innerW int) string {
	if on {
		return t.RowSel.Width(innerW).Render(stripANSI(line))
	}
	return line
}

// playlistRows builds the playlist list: two lines per playlist (title, then
// its track count). sel draws the selection band on the cursor row; the
// cursor row's first line is at index cursor*2.
func playlistRows(t Theme, pls []tidal.Playlist, cursor int, sel bool, innerW int) []string {
	rows := make([]string, 0, 2*len(pls))
	for i := range pls {
		on := sel && i == cursor
		cur, style := listRowStyle(t, on)
		name := truncateStr(cur+style.Render(pls[i].Title), innerW)
		rows = append(rows, selBand(t, name, on, innerW),
			t.RowFaint.Render(fmt.Sprintf("    %d tracks", pls[i].NumberOfTracks)))
	}
	if len(rows) == 0 {
		rows = append(rows, t.RowDim.Render("No playlists."))
	}
	return rows
}

// artistRows builds the favorite-artists list, one row per artist.
func artistRows(t Theme, artists []tidal.Artist, cursor int, sel bool, innerW int) []string {
	rows := make([]string, 0, len(artists))
	for i := range artists {
		on := sel && i == cursor
		cur, style := listRowStyle(t, on)
		line := truncateStr(cur+t.RowDim.Render("◎ ")+style.Render(artists[i].Name), innerW)
		rows = append(rows, selBand(t, line, on, innerW))
	}
	if len(rows) == 0 {
		rows = append(rows, t.RowDim.Render("No favorite artists."))
	}
	return rows
}

// albumRows builds the favorite-albums list: title, release year and track
// count on one row per album.
func albumRows(t Theme, albums []tidal.Album, cursor int, sel bool, innerW int) []string {
	rows := make([]string, 0, len(albums))
	for i := range albums {
		a := albums[i]
		year := ""
		if len(a.ReleaseDate) >= 4 {
			year = " (" + a.ReleaseDate[:4] + ")"
		}
		on := sel && i == cursor
		cur, style := listRowStyle(t, on)
		meta := t.RowFaint.Render(year + " · " + strconv.Itoa(a.NumberOfTracks) + " tracks")
		line := truncateStr(cur+t.RowDim.Render("⊞ ")+style.Render(a.Title)+meta, innerW)
		rows = append(rows, selBand(t, line, on, innerW))
	}
	if len(rows) == 0 {
		rows = append(rows, t.RowDim.Render("No favorite albums."))
	}
	return rows
}
