package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Benehiko/tidalt/v4/internal/tidal"
)

// followModel is a client on the Queue with its cursor on row 0 while the
// parent plays row 2 ("Three"); the last key was pressed at t0 and the
// follow idle time is 5s.
func followModel(t0 time.Time) Model {
	tracks, _ := playlistFixture()
	m := newSmokeModel()
	m.clientMode = true
	m.section = SecQueue
	m.tracks = tracks
	m.tracksOrder = tracks
	m.currentTrack = &tidal.Track{ID: tracks[2].ID}
	m.followIdle = 5 * time.Second
	m.lastKeyAt = t0
	return m
}

// TestFollowPlaying covers docs/ui.md "Following the playing track": when the
// cursor moves onto the playing track, and every case in which it stays put.
func TestFollowPlaying(t *testing.T) {
	t0 := time.Now()
	cases := []struct {
		name  string
		idle  time.Duration
		setup func(*Model)
		want  int
	}{
		{name: "idle time not yet over", idle: 4 * time.Second, want: 0},
		{name: "idle time over", idle: 5 * time.Second, want: 2},
		{name: "long idle", idle: time.Hour, want: 2},
		{name: "standalone instance too", idle: time.Minute, want: 2, setup: func(m *Model) { m.clientMode = false }},
		{name: "first of duplicate entries", idle: time.Minute, want: 1, setup: func(m *Model) {
			m.tracks = append(m.tracks[:1:1], m.tracks[2], m.tracks[1], m.tracks[2])
		}},
		{name: "cursor already on a copy", idle: time.Minute, want: 3, setup: func(m *Model) {
			m.tracks = append(m.tracks, m.tracks[2])
			m.cursor = 3
		}},
		{name: "off", idle: time.Minute, want: 0, setup: func(m *Model) { m.followIdle = 0 }},
		{name: "another page", idle: time.Minute, want: 0, setup: func(m *Model) { m.section = SecFavSongs }},
		{name: "overlay open", idle: time.Minute, want: 0, setup: func(m *Model) { m.overlay = OverlayHelp }},
		{name: "find prompt open", idle: time.Minute, want: 0, setup: func(m *Model) { m.findActive = true }},
		{name: "chord half typed", idle: time.Minute, want: 0, setup: func(m *Model) { m.pendingKeys = []string{"g"} }},
		{name: "artist view open", idle: time.Minute, want: 0, setup: func(m *Model) { m.showArtist = true }},
		{name: "nothing playing", idle: time.Minute, want: 0, setup: func(m *Model) { m.currentTrack = nil }},
		{name: "playing track not queued", idle: time.Minute, want: 0, setup: func(m *Model) {
			m.currentTrack = &tidal.Track{ID: 999}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := followModel(t0)
			if tc.setup != nil {
				tc.setup(&m)
			}
			m.followPlaying(t0.Add(tc.idle))
			if m.cursor != tc.want {
				t.Fatalf("cursor = %d, want %d", m.cursor, tc.want)
			}
		})
	}
}

// TestFollowPlayingThroughUpdate drives the follow through Update: a key
// press restarts the idle time, so the tick right after it leaves the cursor
// where the key put it, and a tick after the idle time moves it.
func TestFollowPlayingThroughUpdate(t *testing.T) {
	m := followModel(time.Now().Add(-time.Minute))
	update := func(msg tea.Msg) {
		t.Helper()
		next, _ := m.Update(msg)
		nm, ok := next.(Model)
		if !ok {
			t.Fatalf("Update returned %T, want Model", next)
		}
		m = nm
	}

	update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursor != 1 {
		t.Fatalf("down moved the cursor to %d, want 1", m.cursor)
	}
	update(tickMsg(time.Now()))
	if m.cursor != 1 {
		t.Fatalf("tick right after a key moved the cursor to %d", m.cursor)
	}

	m.lastKeyAt = time.Now().Add(-time.Minute)
	update(tickMsg(time.Now()))
	if m.cursor != 2 {
		t.Fatalf("idle tick left the cursor on %d, want the playing row 2", m.cursor)
	}
}
