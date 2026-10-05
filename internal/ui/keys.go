package ui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Benehiko/tidalt/v4/internal/player"
	"github.com/Benehiko/tidalt/v4/internal/tidal"
)

// Frequently-compared key strings, hoisted to constants (goconst).
const (
	keyEsc   = "esc"
	keyUp    = "up"
	keyDown  = "down"
	keyEnter = "enter"
	keyLeft  = "left"
	keyRight = "right"
)

// keys returns the active keymap; models built without one (tests, the
// daemon before config load) use the tidalt preset.
func (m *Model) keys() *Keymap {
	if m.keymap == nil {
		return defaultKeymap
	}
	return m.keymap
}

// handleKey is the top-level key dispatcher. Order of precedence:
//  1. ctrl+c always quits
//  2. an open in-list find prompt, a text-input overlay (command palette,
//     Spotify import) or a focused search input owns every other key
//  3. any other overlay, after the few global actions that work over it
//  4. key sequences: a pending chord prefix waits for its next key; a
//     global action (pages, transport, theme, device, help, quit) runs
//     straight away
//  5. sidebar navigation (when the sidebar holds focus)
//  6. the active section's main-pane handler, which resolves list and track
//     actions through the keymap for anything it does not handle itself
func (m Model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	if key == "ctrl+c" {
		cmd := m.quit()
		return m, cmd
	}

	if m.findActive && m.overlay == OverlayNone {
		return m.updateFind(k)
	}

	if m.overlay != OverlayNone {
		if !m.overlayTakesText() {
			if act, ok := m.keys().Lookup(key); ok && overlayGlobal(act) {
				return m.runAction(act)
			}
		}
		return m.updateOverlay(k)
	}

	// A focused search input consumes typing; Enter and the arrows still
	// reach the section handler (search nav / submit).
	if m.searchInput.Focused() {
		switch key {
		case keyEsc:
			m.searchInput.Blur()
			return m, nil
		case keyEnter, keyUp, keyDown:
			return m.routeKey(k)
		default:
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(k)
			return m, cmd
		}
	}

	if len(m.pendingKeys) > 0 && key == keyEsc {
		m.pendingKeys = nil
		return m, nil
	}
	seq := append(slices.Clone(m.pendingKeys), key)
	name := strings.Join(seq, " ")
	if m.keys().IsPrefix(name) {
		m.pendingKeys = seq
		return m, nil
	}
	m.pendingKeys = nil
	act, bound := m.keys().Lookup(name)
	if len(seq) > 1 {
		if !bound {
			return m, nil // an unknown chord is dropped, as in vim
		}
		// Route the completed chord as one key whose String() is the whole
		// sequence, so section handlers resolve it like any single key.
		k = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
	}
	if bound && m.isGlobalAction(act, name) {
		return m.runAction(act)
	}
	return m.routeKey(k)
}

// routeKey hands a key to the sidebar or the active section.
func (m Model) routeKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.focusMain {
		return m.updateSidebar(k)
	}
	return m.updateSection(k)
}

// overlayTakesText reports whether the open overlay is a text input that must
// receive every printable key.
func (m *Model) overlayTakesText() bool {
	return m.overlay == OverlayCommandPalette || m.overlay == OverlayImportSpotify
}

// overlayGlobal lists the actions that still work while a (non-text) overlay
// is open.
func overlayGlobal(a Action) bool {
	switch a {
	case ActQuit, ActOpenCommandPalette, ActCycleTheme:
		return true
	default:
		return false
	}
}

// structuralKeys move the cursor or focus in the sidebar and sections. A
// playback action bound to one of them (the tidalt preset's Space, ← and →)
// only runs where the section handler lets it through, never globally.
var structuralKeys = map[string]bool{
	keyUp: true, keyDown: true, keyLeft: true, keyRight: true,
	keyEnter: true, keyEsc: true, " ": true,
	"h": true, "j": true, "k": true, "l": true,
}

// isGlobalAction reports whether act, bound to seq, runs regardless of which
// pane has focus. List and track actions depend on the selection, so they are
// left to the section handlers.
func (m *Model) isGlobalAction(act Action, seq string) bool {
	switch act {
	case ActCycleTheme:
		return m.section != SecSettings // the theme picker cycles its own cursor
	case ActSwitchDevice:
		return m.section != SecSearch
	default:
	}
	switch actionIndex[act].group {
	case groupPages, groupApp:
		return true
	case groupPlayback:
		return !structuralKeys[seq]
	default:
		return false
	}
}

// runAction performs a bound action. List actions are handled by
// updateListMotion, which needs the active list; here they are no-ops.
func (m Model) runAction(act Action) (tea.Model, tea.Cmd) {
	switch act {
	case ActQuit:
		cmd := m.quit()
		return m, cmd
	case ActOpenCommandPalette:
		if m.overlay != OverlayCommandPalette {
			m.openCommandPalette()
		}
	case ActOpenCommandHelp:
		m.overlay = OverlayHelp
		m.helpScroll = 0
	case ActCycleTheme:
		m.cycleTheme()
	case ActSwitchDevice:
		m.openDeviceSelect()
	case ActSwitchTheme:
		return m.selectSection(SecSettings)
	case ActQueue:
		return m.selectSection(SecQueue)
	case ActLikedTrackPage:
		return m.selectSection(SecFavSongs)
	case ActRecentlyPlayedTrackPage:
		return m.selectSection(SecHistory)
	case ActMixesPage:
		return m.selectSection(SecMixes)
	case ActSearchPage:
		return m.selectSection(SecSearch)
	case ActBrowseUserPlaylists:
		return m.selectSection(SecPlaylists)
	case ActBrowseUserFollowedArtists:
		return m.selectSection(SecFavArtists)
	case ActBrowseUserSavedAlbums:
		return m.selectSection(SecFavAlbums)
	default:
		return m.runTrackAction(act)
	}
	return m, nil
}

func (m *Model) quit() tea.Cmd {
	if m.currentTrack != nil {
		_ = m.store.SaveLastPosition(m.currPos)
	}
	if m.player != nil {
		m.player.Close()
	}
	m.store.Close()
	return tea.Quit
}

// openDeviceSelect populates the device list and raises the device overlay.
func (m *Model) openDeviceSelect() {
	devs, err := player.ListDevices()
	if err != nil {
		m.errText = err.Error()
		return
	}
	m.devices = devs
	m.overlay = OverlayDeviceSelect
	m.cursor = 0
	for i, d := range devs {
		if d.HWName == m.currentDevice {
			m.cursor = i
			break
		}
	}
}

// cycleTheme advances to the next palette in paletteOrder and commits it.
func (m *Model) cycleTheme() {
	i := 0
	for j, name := range paletteOrder {
		if name == m.themeName {
			i = j
			break
		}
	}
	next := paletteOrder[(i+1)%len(paletteOrder)]
	m.applyTheme(next)
}

// applyTheme commits a palette by name and persists it.
func (m *Model) applyTheme(name string) {
	m.themeName = name
	m.palette = resolvePalette(name)
	m.theme = m.palette.Theme()
	m.previewPalette = nil
	m.rebuildProgress()
	_ = m.store.SaveTheme(name)
}

// rebuildProgress rebuilds the progress bar with the active theme's gradient at
// the current width.
func (m *Model) rebuildProgress() {
	t := m.activeTheme()
	barWidth := max(m.width-22, 10)
	m.progress = progressWithTheme(t, barWidth)
}

// updateSidebar handles navigation while the sidebar holds focus.
func (m Model) updateSidebar(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case keyUp, "k":
		if m.sidebarCursor > 0 {
			m.sidebarCursor--
		}
	case keyDown, "j":
		if m.sidebarCursor < len(navSections)-1 {
			m.sidebarCursor++
		}
	case keyEnter, "l", keyRight, " ":
		return m.selectSection(navSections[m.sidebarCursor])
	case "/":
		return m.selectSection(SecSearch)
	}
	return m, nil
}

// selectSection switches to a section, moves focus to the main pane, and fires
// any data-load command the section needs.
func (m Model) selectSection(sec Section) (tea.Model, tea.Cmd) {
	m.section = sec
	m.showArtist = false
	m.focusMain = true
	m.sidebarCursor = navIndexOf(sec)
	m.cursor = 0
	if sec == SecPlaylists {
		m.detailFocus = false
	}
	if sec == SecSettings {
		m.enterSettings()
	}

	switch sec {
	case SecSearch:
		m.searchInput.Focus()
	default:
		m.searchInput.Blur()
	}
	cmd := m.loadSection(sec)
	return m, tea.Batch(cmd, m.syncQueueCover())
}

// loadSection returns the command to (re)load a section's data, or nil when the
// section reuses already-loaded data. Favorites/playlists loaders arrive in
// later steps; for now only the always-available sections are wired.
func (m *Model) loadSection(sec Section) tea.Cmd {
	switch sec {
	case SecMixes:
		if len(m.mixes) > 0 {
			return nil
		}
		return func() tea.Msg {
			mixes, err := m.client.GetMixes(m.ctx)
			if err != nil {
				return errMsg(err)
			}
			return mixesMsg(mixes)
		}
	case SecPlaylists:
		if len(m.playlists) > 0 {
			return nil
		}
		return func() tea.Msg {
			pls, err := m.client.GetUserPlaylists(m.ctx)
			if err != nil {
				return errMsg(err)
			}
			return playlistsMsg(pls)
		}
	case SecFavArtists:
		if len(m.favArtists) > 0 {
			return nil
		}
		return func() tea.Msg {
			artists, err := m.client.GetFavoriteArtists(m.ctx, 200)
			if err != nil {
				return errMsg(err)
			}
			return favArtistsMsg(artists)
		}
	case SecFavAlbums:
		if len(m.favAlbums) > 0 {
			return nil
		}
		return func() tea.Msg {
			albums, err := m.client.GetFavoriteAlbums(m.ctx, 200)
			if err != nil {
				return errMsg(err)
			}
			return favAlbumsMsg(albums)
		}
	default:
		// SecHistory uses the in-memory m.history; others reuse loaded data.
		return nil
	}
}

// updateSection routes keys to the active section's handler.
func (m Model) updateSection(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Esc backs out one level: album → artist albums → close artist → sidebar.
	if k.String() == keyEsc {
		switch {
		case m.showArtist && m.artistAlbum != nil:
			m.artistAlbum = nil
			m.artistAlbumTracks = nil
			m.artistAlbumCursor = 0
		case m.showArtist:
			m.showArtist = false
		case m.section == SecPlaylists && m.detailFocus:
			m.detailFocus = false
		default:
			m.focusMain = false
		}
		return m, nil
	}

	if next, cmd, handled := m.updateListMotion(k); handled {
		return next, cmd
	}

	if m.showArtist {
		return m.updateArtist(k)
	}

	switch m.section {
	case SecSearch:
		return m.updateSearchKeys(k)
	case SecFavSongs:
		return m.updateFavSongs(k)
	case SecPlaylists:
		return m.updatePlaylists(k)
	case SecFavArtists:
		return m.updateFavArtists(k)
	case SecFavAlbums:
		return m.updateFavAlbums(k)
	case SecHistory:
		return m.updateHistory(k)
	case SecSettings:
		return m.updateSettings(k)
	default:
		// Queue, Now Playing, Mixes.
		if k.String() == "h" || (k.String() == keyLeft && m.currentTrack == nil) {
			m.focusMain = false
			return m, nil
		}
		return m.updateListKeys(k)
	}
}

// updateListKeys handles the common track-list sections (Queue, Favorites songs,
// Now Playing): cursor movement, playback, and track actions.
func (m Model) updateListKeys(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case keyUp, "k":
		if m.cursor > 0 {
			m.cursor--
		}
		cmd := m.syncQueueCover()
		return m, cmd
	case keyDown, "j":
		maxIdx := m.currentListLen()
		if m.cursor < maxIdx-1 {
			m.cursor++
		}
		cmd := m.syncQueueCover()
		return m, cmd
	case "x":
		if m.section == SecQueue {
			removeCmd := m.removeFromQueue(m.cursor)
			cmd := m.syncQueueCover()
			return m, tea.Batch(removeCmd, cmd)
		}
		return m, nil
	case "C":
		if m.section == SecQueue {
			m.clearQueue()
			cmd := m.syncQueueCover()
			return m, cmd
		}
		return m, nil
	case keyEnter:
		if m.section == SecMixes && len(m.mixes) > 0 {
			mix := m.mixes[m.cursor]
			return m, func() tea.Msg {
				tracks, err := m.client.GetMixTracks(m.ctx, mix.ID)
				if err != nil {
					return errMsg(err)
				}
				return tracksMsg(tracks)
			}
		}
		if t := m.selectedTrack(); t != nil {
			_ = m.store.CacheTrack(t.ID, *t)
			cmd := m.playTrackCmd(*t)
			return m, cmd
		}
		return m, nil
	}
	return m.commonKeys(k)
}

// commonKeys resolves a key the section handler did not use through the
// keymap and runs the playback or track action bound to it.
func (m Model) commonKeys(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	act, ok := m.keys().Lookup(k.String())
	if !ok {
		return m, nil
	}
	switch actionIndex[act].group {
	case groupPlayback, groupTrack:
		return m.runTrackAction(act)
	default:
		return m, nil
	}
}

// runTrackAction performs a playback or track action.
func (m Model) runTrackAction(act Action) (tea.Model, tea.Cmd) {
	switch act {
	case ActResumePause:
		return m.togglePlay()
	case ActSeekBackward:
		m.seekTo(m.currPos - 10)
	case ActSeekForward:
		m.seekTo(m.currPos + 10)
	case ActSeekStart:
		m.seekTo(0)
	case ActVolumeDown:
		m.setVolume(m.volume - 5)
	case ActVolumeUp:
		m.setVolume(m.volume + 5)
	case ActMute:
		m.toggleMute()
	case ActShuffle:
		cmd := m.cycleShuffle()
		return m, cmd
	case ActSaveQueueAsPlaylist:
		return m.saveQueueAsNew()
	case ActNextTrack:
		return m.skipNext()
	case ActPreviousTrack:
		return m.skipPrev()
	case ActShowActionsOnSelectedItem:
		if t := m.selectedTrack(); t != nil {
			m.openActionSheet(*t)
		}
	case ActShowActionsOnCurrentTrack:
		if m.currentTrack != nil {
			m.openActionSheet(*m.currentTrack)
		}
	case ActAddSelectedItemToQueue:
		if t := m.selectedTrack(); t != nil {
			track := *t
			cmd := m.enqueueEnd(track)
			return m, cmd
		}
	case ActGoToRadio:
		if t := m.selectedTrack(); t != nil {
			cmd := m.radioFrom(*t)
			return m, cmd
		}
	case ActToggleLiked:
		if t := m.selectedTrack(); t != nil {
			cmd := m.toggleFavorite(*t)
			return m, cmd
		}
	case ActGoToArtist:
		return m.openArtistFor(m.selectedTrack())
	case ActCopyLink:
		return m.copyLink()
	default:
	}
	return m, nil
}

// seekTo seeks the playing track to pos seconds.
func (m *Model) seekTo(pos float64) {
	if m.clientMode || m.player == nil || m.currentTrack == nil {
		return
	}
	if err := m.player.Seek(max(pos, 0)); err != nil {
		m.errText = err.Error()
	}
}

// toggleMute drops the volume to 0, or restores the level it had before.
func (m *Model) toggleMute() {
	if m.volume > 0 {
		m.preMuteVolume = m.volume
		m.setVolume(0)
		return
	}
	restore := m.preMuteVolume
	if restore <= 0 {
		restore = 50
	}
	m.setVolume(restore)
}

// --- shared action helpers ---

func (m *Model) setVolume(v float64) {
	if m.clientMode {
		return
	}
	m.volume = max(min(v, 100), 0)
	_ = m.player.SetVolume(m.volume)
	_ = m.store.SaveVolume(m.volume)
}

// cycleShuffle advances to the next shuffle mode. In client mode the parent
// owns the queue and its order, so the change is forwarded to it and nothing
// is reordered locally: the next parentStateMsg brings back both the new mode
// and the order the parent will actually play. Reshuffling here as well would
// leave the two instances showing different queues.
func (m *Model) cycleShuffle() tea.Cmd {
	var next ShuffleMode
	switch m.shuffleMode {
	case ShuffleOff:
		next = ShuffleFisherYates
	case ShuffleFisherYates:
		next = ShuffleRandom
	default:
		next = ShuffleOff
	}
	if m.clientMode {
		mc, mode := m.mprisClient, next.String()
		return func() tea.Msg {
			if err := mc.SendShuffle(mode); err != nil {
				return errMsg(err)
			}
			return nil
		}
	}
	m.setShuffle(next)
	return nil
}

func (m Model) togglePlay() (tea.Model, tea.Cmd) {
	if m.clientMode {
		mc := m.mprisClient
		return m, func() tea.Msg {
			if err := mc.SendPlayPause(); err != nil {
				return errMsg(err)
			}
			return nil
		}
	}
	if m.currentTrack == nil {
		if t := m.selectedTrack(); t != nil {
			_ = m.store.CacheTrack(t.ID, *t)
			cmd := m.playTrackCmd(*t)
			return m, cmd
		}
		return m, nil
	}
	_ = m.player.Pause()
	// Read the state back rather than assuming the flip landed where we
	// expected: the player can force itself back to paused on its own (a
	// failed ALSA reacquire on resume), and toggling blindly from a stale
	// belief inverts play/pause for the rest of the track.
	m.isPlaying = !m.player.IsPaused()
	m.pushState()
	if m.isPlaying {
		return m, tea.Batch(m.ensureBarsTicking()...)
	}
	return m, nil
}

func (m Model) skipNext() (tea.Model, tea.Cmd) {
	if len(m.tracks) == 0 {
		return m, nil
	}
	m.shufflePlayed = append(m.shufflePlayed, m.cursor)
	next := m.nextIndex()
	if next < 0 {
		return m, nil
	}
	m.advancing = true
	m.cursor = next
	track := m.tracks[next]
	m.currPos = 0
	m.duration = 0
	_ = m.store.CacheTrack(track.ID, track)
	cmd := m.playNextTrackCmd(track)
	return m, cmd
}

func (m Model) skipPrev() (tea.Model, tea.Cmd) {
	if len(m.tracks) == 0 {
		return m, nil
	}
	prev := m.prevIndex()
	if prev < 0 {
		return m, nil
	}
	m.advancing = false
	m.cursor = prev
	track := m.tracks[prev]
	m.currPos = 0
	m.duration = 0
	_ = m.store.CacheTrack(track.ID, track)
	cmd := m.playTrackCmd(track)
	return m, cmd
}

func (m *Model) radioFrom(t tidal.Track) tea.Cmd {
	id := t.ID
	m.pendingQueueSource = "radio" // tracksMsg marks the resulting queue unsaved
	return func() tea.Msg {
		tracks, err := m.client.GetTrackRadio(m.ctx, id)
		if err != nil {
			return errMsg(err)
		}
		return tracksMsg(tracks)
	}
}

func (m *Model) toggleFavorite(t tidal.Track) tea.Cmd {
	id := t.ID
	isFav := m.favorites[id]
	return func() tea.Msg {
		var err error
		if isFav {
			err = m.client.RemoveFavorite(m.ctx, id)
		} else {
			err = m.client.AddFavorite(m.ctx, id)
		}
		if err != nil {
			return errMsg(err)
		}
		return favoriteMsg{trackID: id, added: !isFav}
	}
}

func (m Model) copyLink() (tea.Model, tea.Cmd) {
	if t := m.selectedTrack(); t != nil {
		return m.copyTrackLink(*t)
	}
	return m, nil
}

// copyTrackLink copies a track's Tidal URL to the clipboard and flashes a
// confirmation in the status line.
func (m Model) copyTrackLink(t tidal.Track) (tea.Model, tea.Cmd) {
	link := fmt.Sprintf("https://tidal.com/track/%d", t.ID)
	if err := clipboard.WriteAll(link); err != nil {
		m.errText = err.Error()
		return m, nil
	}
	m.errText = "Copied link to clipboard"
	return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return clearErrMsg{} })
}

// openAlbum loads an album's tracks into the queue.
func (m *Model) openAlbum(albumID int) tea.Cmd {
	if albumID == 0 {
		return nil
	}
	id := strconv.Itoa(albumID)
	return func() tea.Msg {
		tracks, err := m.client.GetAlbumTracks(m.ctx, id)
		if err != nil {
			return errMsg(err)
		}
		return tracksMsg(tracks)
	}
}

// openArtistFor opens the transient artist drill-down for a track's artist.
func (m Model) openArtistFor(t *tidal.Track) (tea.Model, tea.Cmd) {
	if t == nil {
		t = m.currentTrack
	}
	if t == nil || t.Artist.ID == 0 {
		return m, nil
	}
	artistID := t.Artist.ID
	artistName := t.Artist.Name
	m.prevSection = m.section
	m.artistLoading = true
	m.showArtist = true
	return m, func() tea.Msg {
		albums, err := m.client.GetArtistAlbums(m.ctx, artistID)
		if err != nil {
			return errMsg(err)
		}
		return artistAlbumsMsg{artistID: artistID, artistName: artistName, albums: albums}
	}
}

// updateArtist handles the transient artist drill-down sub-view.
func (m Model) updateArtist(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	// When an album is open, navigate its track list instead of the album list.
	if m.artistAlbum != nil {
		return m.updateArtistAlbum(k)
	}
	switch k.String() {
	case keyUp, "k":
		if m.artistCursor > 0 {
			m.artistCursor--
		}
		return m, nil
	case keyDown, "j":
		if m.artistCursor < len(m.artistAlbums)+2-1 {
			m.artistCursor++
		}
		return m, nil
	case keyEnter:
		artistID := m.artistID
		switch m.artistCursor {
		case 0: // ▶ Play all tracks
			m.artistLoading = true
			return m, func() tea.Msg {
				tracks, err := m.client.GetArtistAllTracks(m.ctx, artistID)
				if err != nil {
					return errMsg(err)
				}
				return tracksMsg(tracks)
			}
		case 1: // ★ Top tracks
			return m, func() tea.Msg {
				tracks, err := m.client.GetArtistTopTracks(m.ctx, artistID, 100)
				if err != nil {
					return errMsg(err)
				}
				return tracksMsg(tracks)
			}
		default:
			// Open the album to view its tracks (does not load the queue yet).
			if idx := m.artistCursor - 2; idx >= 0 && idx < len(m.artistAlbums) {
				album := m.artistAlbums[idx]
				albumID := strconv.Itoa(album.ID)
				m.artistLoading = true
				return m, func() tea.Msg {
					tracks, err := m.client.GetAlbumTracks(m.ctx, albumID)
					if err != nil {
						return errMsg(err)
					}
					return artistAlbumTracksMsg{album: album, tracks: tracks}
				}
			}
		}
		return m, nil
	}
	return m.commonKeys(k)
}

// updateArtistAlbum navigates the track list of an album opened inside the
// artist drill-down. Esc backs out to the album list; Enter loads the album
// into the queue and plays from the selected track.
func (m Model) updateArtistAlbum(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case keyEsc:
		m.artistAlbum = nil
		m.artistAlbumTracks = nil
		m.artistAlbumCursor = 0
		return m, nil
	case keyUp, "k":
		if m.artistAlbumCursor > 0 {
			m.artistAlbumCursor--
		}
		return m, nil
	case keyDown, "j":
		if m.artistAlbumCursor < len(m.artistAlbumTracks)-1 {
			m.artistAlbumCursor++
		}
		return m, nil
	case keyEnter:
		tracks := m.artistAlbumTracks
		i := m.artistAlbumCursor
		m.showArtist = false
		m.artistAlbum = nil
		m.section = SecQueue
		m.sidebarCursor = navIndexOf(SecQueue)
		cmd := m.playListIntoQueue(tracks, i)
		return m, cmd
	}
	return m.commonKeys(k)
}

// --- selection helpers ---

// selectedTrack returns the track under the cursor in the active context, or
// the current track as a fallback. Returns nil when nothing is selectable.
func (m *Model) selectedTrack() *tidal.Track {
	switch {
	case m.section == SecSearch:
		return m.selectedTrackForSearch()
	case m.section == SecFavSongs && len(m.favSongs) > 0 && m.cursor < len(m.favSongs):
		t := m.favSongs[m.cursor]
		return &t
	case m.section == SecHistory && len(m.history) > 0 && m.cursor < len(m.history):
		t := m.history[m.cursor]
		return &t
	case m.section == SecPlaylists && m.detailFocus && len(m.detailTracks) > 0 && m.detailCursor < len(m.detailTracks):
		t := m.detailTracks[m.detailCursor]
		return &t
	case len(m.tracks) > 0 && m.cursor >= 0 && m.cursor < len(m.tracks):
		t := m.tracks[m.cursor]
		return &t
	case m.currentTrack != nil:
		return m.currentTrack
	default:
		return nil
	}
}

// currentListLen returns the length of the list the main cursor indexes for the
// active section.
func (m *Model) currentListLen() int {
	switch m.section {
	case SecMixes:
		return len(m.mixes)
	default:
		return len(m.tracks)
	}
}
