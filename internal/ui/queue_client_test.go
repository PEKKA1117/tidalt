package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Benehiko/tidalt/v4/internal/mpris"
	"github.com/Benehiko/tidalt/v4/internal/tidal"
)

// addedTitle names the track the enqueue tests push onto a queue.
const addedTitle = "Added"

// playlistFixture is a three-track playlist plus its Tidal metadata.
func playlistFixture() ([]tidal.Track, tidal.Playlist) {
	tracks := []tidal.Track{
		{ID: 11, Title: "One"},
		{ID: 22, Title: "Two"},
		{ID: 33, Title: "Three"},
	}
	return tracks, tidal.Playlist{UUID: "uuid-1", Title: "Night Drive"}
}

// TestLoadQueueFromPlaylistMarksLocalInClientMode asserts a client-mode
// playlist load flags the queue as local, which is what makes doPlayTrack hand
// the parent the whole playlist instead of a single track ID.
func TestLoadQueueFromPlaylistMarksLocalInClientMode(t *testing.T) {
	tracks, pl := playlistFixture()

	m := newSmokeModel()
	m.clientMode = true
	m.loadQueueFromPlaylist(tracks, pl)

	if len(m.tracks) != len(tracks) {
		t.Fatalf("queue holds %d tracks, want %d", len(m.tracks), len(tracks))
	}
	if !m.localPlaylist {
		t.Error("localPlaylist is false; the parent would only receive the selected track")
	}

	// Standalone mode is the player itself, so there is nothing to hand over.
	s := newSmokeModel()
	s.loadQueueFromPlaylist(tracks, pl)
	if s.localPlaylist {
		t.Error("localPlaylist set outside client mode")
	}
}

// TestLoadListIntoQueueMarksLocalInClientMode covers the same fix on the
// ad-hoc list path (Favorite Songs, Recently Played, artist albums).
func TestLoadListIntoQueueMarksLocalInClientMode(t *testing.T) {
	tracks, _ := playlistFixture()

	m := newSmokeModel()
	m.clientMode = true
	if !m.loadListIntoQueue(tracks, 1) {
		t.Fatal("loadListIntoQueue rejected an in-range index")
	}
	if len(m.tracks) != len(tracks) || m.cursor != 1 {
		t.Fatalf("queue = %d tracks, cursor = %d; want %d and 1", len(m.tracks), m.cursor, len(tracks))
	}
	if !m.localPlaylist {
		t.Error("localPlaylist is false; the parent would only receive the selected track")
	}

	// An out-of-range index must leave the existing queue alone.
	before := len(m.tracks)
	if m.loadListIntoQueue(tracks, len(tracks)) {
		t.Error("loadListIntoQueue accepted an out-of-range index")
	}
	if len(m.tracks) != before {
		t.Errorf("queue changed on a rejected load: %d tracks, want %d", len(m.tracks), before)
	}
}

// TestParentStateKeepsFreshlyLoadedPlaylist reproduces the reported bug: right
// after loading a playlist the client polls the parent, whose queue still
// holds only the track it was told to play. That state must not shrink the
// local queue before the playlist has been handed over.
func TestParentStateKeepsFreshlyLoadedPlaylist(t *testing.T) {
	tracks, pl := playlistFixture()

	m := newSmokeModel()
	m.clientMode = true
	m.loadQueueFromPlaylist(tracks, pl)

	stale := parentStateMsg(mpris.PlayerState{
		PlaylistJSON: mpris.MarshalTracks([]tidal.Track{{ID: 22, Title: "Two"}}),
	})
	got := asModel(t, must(m.Update(stale)))

	if len(got.tracks) != len(tracks) {
		t.Fatalf("parent state shrank the queue to %d tracks, want %d", len(got.tracks), len(tracks))
	}
}

// TestEnqueueForwardsInClientMode asserts a client does not edit its own queue
// copy: the addition is handed to the parent, which owns the queue and decides
// where the track lands relative to what it is playing.
func TestEnqueueForwardsInClientMode(t *testing.T) {
	added := tidal.Track{ID: 99, Title: addedTitle}

	for _, tc := range []struct {
		name string
		call func(m *Model) tea.Cmd
	}{
		{name: "add to queue", call: func(m *Model) tea.Cmd { return m.enqueueEnd(added) }},
		{name: "play next", call: func(m *Model) tea.Cmd { return m.enqueueNext(added) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newSmokeModel()
			m.clientMode = true
			before := len(m.tracks)

			cmd := tc.call(&m)

			if cmd == nil {
				t.Error("no command returned; the parent never hears about the addition")
			}
			if len(m.tracks) != before {
				t.Errorf("local queue changed to %d tracks; the parent's copy is authoritative", len(m.tracks))
			}
			if m.queueDirty {
				t.Error("queueDirty set on a client edit the parent has not applied yet")
			}
		})
	}
}

// TestEnqueueEditsQueueOutsideClientMode pins the standalone behaviour the
// client-mode branch must not disturb.
func TestEnqueueEditsQueueOutsideClientMode(t *testing.T) {
	added := tidal.Track{ID: 99, Title: addedTitle}

	m := newSmokeModel()
	m.cursor = 0
	if cmd := m.enqueueEnd(added); cmd != nil {
		t.Error("a standalone player should not return a command; it is the parent")
	}
	if got := m.tracks[len(m.tracks)-1].ID; got != added.ID {
		t.Errorf("last queued track = %d, want %d", got, added.ID)
	}

	n := newSmokeModel()
	n.cursor = 1
	n.enqueueNext(added)
	if got := n.tracks[2].ID; got != added.ID {
		t.Errorf("track after the cursor = %d, want %d", got, added.ID)
	}
}

// TestParentAppliesClientEnqueue drives the parent's side of the D-Bus call:
// the forwarded track must reach the queue, positioned against the parent's
// own cursor, without the parent being in client mode itself.
func TestParentAppliesClientEnqueue(t *testing.T) {
	added := tidal.Track{ID: 99, Title: addedTitle}
	payload := mpris.MarshalTracks(added)

	t.Run("play next", func(t *testing.T) {
		m := newSmokeModel()
		m.cursor = 1
		got := asModel(t, must(m.Update(mprisMsg(mpris.Event{
			Cmd: mpris.CmdEnqueue, TrackJSON: payload, EnqueueNext: true,
		}))))

		if len(got.tracks) != 4 {
			t.Fatalf("queue holds %d tracks, want 4", len(got.tracks))
		}
		if got.tracks[2].ID != added.ID {
			t.Errorf("track after the cursor = %d, want %d", got.tracks[2].ID, added.ID)
		}
	})

	t.Run("append", func(t *testing.T) {
		m := newSmokeModel()
		got := asModel(t, must(m.Update(mprisMsg(mpris.Event{
			Cmd: mpris.CmdEnqueue, TrackJSON: payload,
		}))))

		if got.tracks[len(got.tracks)-1].ID != added.ID {
			t.Errorf("last queued track = %d, want %d", got.tracks[len(got.tracks)-1].ID, added.ID)
		}
	})

	// A malformed payload must be reported, not panic or corrupt the queue.
	t.Run("invalid payload", func(t *testing.T) {
		m := newSmokeModel()
		before := len(m.tracks)
		got := asModel(t, must(m.Update(mprisMsg(mpris.Event{
			Cmd: mpris.CmdEnqueue, TrackJSON: "not json",
		}))))

		if len(got.tracks) != before {
			t.Errorf("queue changed to %d tracks on a bad payload, want %d", len(got.tracks), before)
		}
		if got.errText == "" {
			t.Error("no error surfaced for an undecodable track")
		}
	})
}

// must drops the command half of an Update so the model can be asserted on.
func must(m tea.Model, _ tea.Cmd) tea.Model { return m }
