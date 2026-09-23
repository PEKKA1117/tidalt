package ui

import (
	"testing"

	"github.com/Benehiko/tidalt/v4/internal/mpris"
	"github.com/Benehiko/tidalt/v4/internal/tidal"
)

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
	updated, _ := m.Update(stale)
	got := asModel(t, updated)

	if len(got.tracks) != len(tracks) {
		t.Fatalf("parent state shrank the queue to %d tracks, want %d", len(got.tracks), len(tracks))
	}
}
