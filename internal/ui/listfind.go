package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Benehiko/tidalt/v4/internal/tidal"
)

// activeList returns the searchable labels of the list the main pane is
// showing, plus a pointer to that list's cursor. It returns (nil, nil) for
// contexts that are not a plain list (Search, Settings).
func (m *Model) activeList() (labels []string, cursor *int) {
	if m.showArtist {
		if m.artistAlbum != nil {
			return trackLabels(m.artistAlbumTracks), &m.artistAlbumCursor
		}
		// The two synthetic quick-play rows precede the albums (see updateArtist).
		labels = []string{"Play all tracks", "Top tracks"}
		for i := range m.artistAlbums {
			labels = append(labels, m.artistAlbums[i].Title)
		}
		return labels, &m.artistCursor
	}
	switch m.section {
	case SecSearch, SecSettings:
		return nil, nil
	case SecFavSongs:
		return trackLabels(m.favSongs), &m.cursor
	case SecHistory:
		return trackLabels(m.history), &m.cursor
	case SecFavArtists:
		labels := make([]string, len(m.favArtists))
		for i := range m.favArtists {
			labels[i] = m.favArtists[i].Name
		}
		return labels, &m.cursor
	case SecFavAlbums:
		labels := make([]string, len(m.favAlbums))
		for i := range m.favAlbums {
			labels[i] = m.favAlbums[i].Title
		}
		return labels, &m.cursor
	case SecPlaylists:
		if m.detailFocus {
			return trackLabels(m.detailTracks), &m.detailCursor
		}
		labels := make([]string, len(m.playlists))
		for i := range m.playlists {
			labels[i] = m.playlists[i].Title
		}
		return labels, &m.cursor
	case SecMixes:
		labels := make([]string, len(m.mixes))
		for i := range m.mixes {
			labels[i] = m.mixes[i].Title + " " + m.mixes[i].SubTitle
		}
		return labels, &m.cursor
	default:
		// Queue, Now Playing.
		return trackLabels(m.tracks), &m.cursor
	}
}

// trackLabels makes tracks findable by title or artist name.
func trackLabels(tracks []tidal.Track) []string {
	labels := make([]string, len(tracks))
	for i := range tracks {
		labels[i] = tracks[i].Title + " " + tracks[i].Artist.Name
	}
	return labels
}

// findMatch returns the index of the first label containing query
// (case-insensitive), scanning from start in direction dir (+1 or -1) and
// wrapping around the list. It returns -1 when nothing matches.
func findMatch(labels []string, query string, start, dir int) int {
	n := len(labels)
	q := strings.ToLower(query)
	if n == 0 || q == "" {
		return -1
	}
	for step := range n {
		i := ((start+dir*step)%n + n) % n
		if strings.Contains(strings.ToLower(labels[i]), q) {
			return i
		}
	}
	return -1
}

// updateListMotion handles the list actions shared by every list context:
// find (/), repeat find (n/N), top/bottom (gg/G) and paging, as bound in the
// keymap. handled is false for any other key (and for non-list contexts), so
// the caller falls through to the section handler.
func (m Model) updateListMotion(k tea.KeyMsg) (_ tea.Model, _ tea.Cmd, handled bool) {
	labels, cursor := m.activeList()
	if cursor == nil {
		return m, nil, false
	}
	act, ok := m.keys().Lookup(k.String())
	if !ok {
		return m, nil, false
	}

	switch act {
	case ActSearch:
		m.findInput = textinput.New()
		m.findInput.Prompt = "/"
		m.findInput.Focus()
		m.findActive = true
		m.findOrigin = *cursor
		return m, nil, true
	case ActFindNext, ActFindPrevious:
		if m.findQuery == "" {
			return m, nil, true
		}
		dir := 1
		if act == ActFindPrevious {
			dir = -1
		}
		if i := findMatch(labels, m.findQuery, *cursor+dir, dir); i >= 0 {
			*cursor = i
		}
	case ActSelectFirst:
		*cursor = 0
	case ActSelectLast:
		if len(labels) > 0 {
			*cursor = len(labels) - 1
		}
	case ActPageSelectNext:
		*cursor = max(min(*cursor+m.pageSize(), len(labels)-1), 0)
	case ActPageSelectPrevious:
		*cursor = max(*cursor-m.pageSize(), 0)
	default:
		return m, nil, false
	}
	cmd := m.syncQueueCover()
	return m, cmd, true
}

// pageSize is how far PageSelectNext/Previous move: the rows a list panel
// shows.
func (m *Model) pageSize() int {
	return max(m.bodyHeight()-2, 1)
}

// updateFind handles keys while the find prompt is open. Typing moves the
// cursor to the first match at or after where the find started; Enter keeps
// it, Esc puts the cursor back.
func (m Model) updateFind(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	labels, cursor := m.activeList()
	if cursor == nil {
		m.findActive = false
		return m, nil
	}
	switch k.String() {
	case keyEsc:
		m.findActive = false
		*cursor = m.findOrigin
		cmd := m.syncQueueCover()
		return m, cmd
	case keyEnter:
		m.findActive = false
		if q := m.findInput.Value(); q != "" {
			m.findQuery = q
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.findInput, cmd = m.findInput.Update(k)
	*cursor = m.findOrigin
	if i := findMatch(labels, m.findInput.Value(), m.findOrigin, 1); i >= 0 {
		*cursor = i
	}
	coverCmd := m.syncQueueCover()
	return m, tea.Batch(cmd, coverCmd)
}
