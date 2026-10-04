package ui

import (
	"testing"

	"github.com/Benehiko/tidalt/v4/internal/mpris"
	"github.com/Benehiko/tidalt/v4/internal/tidal"
)

// numberedTracks returns n tracks with IDs 1..n, enough for a shuffle to be
// distinguishable from the original order.
func numberedTracks(n int) []tidal.Track {
	tracks := make([]tidal.Track, n)
	for i := range tracks {
		tracks[i] = tidal.Track{ID: i + 1}
	}
	return tracks
}

// trackIDs flattens a queue to its ID sequence for comparison in failures.
func trackIDs(tracks []tidal.Track) []int {
	ids := make([]int, len(tracks))
	for i := range tracks {
		ids[i] = tracks[i].ID
	}
	return ids
}

// TestParentStateKeepsParentShuffleOrder reproduces the client re-shuffling the
// parent's already-shuffled queue: a client in Shuffle mode must show exactly
// the order the parent sent, and keep showing it across repeated polls instead
// of reshuffling on every tick.
func TestParentStateKeepsParentShuffleOrder(t *testing.T) {
	parent := newSmokeModel()
	parent.tracksOrder = numberedTracks(30)
	parent.shuffleMode = ShuffleFisherYates
	parent.applyShuffle()
	// Change the parent's queue after the client last synced, which is what
	// sends the client down its list-replacement path.
	parent.enqueueEnd(tidal.Track{ID: 99})
	ps := parentStateMsg(mpris.PlayerState{
		PlaylistJSON: mpris.MarshalTracks(parent.tracks),
		ShuffleMode:  parent.shuffleMode.String(),
	})

	c := newSmokeModel()
	c.clientMode = true
	c.shuffleMode = ShuffleFisherYates // synced from an earlier poll

	for tick := range 4 {
		c = asModel(t, must(c.Update(ps)))
		if !sameTrackIDs(c.tracks, parent.tracks) {
			t.Fatalf("tick %d: client order %v, parent plays %v", tick, trackIDs(c.tracks), trackIDs(parent.tracks))
		}
	}
	if c.shuffleMode != ShuffleFisherYates {
		t.Errorf("client shuffle mode = %v, want the parent's Shuffle", c.shuffleMode)
	}
}

// TestParentStatePicksUpReshuffle covers a reshuffle on the parent that keeps
// the queue length and the first track: comparing only those would miss it.
func TestParentStatePicksUpReshuffle(t *testing.T) {
	c := newSmokeModel()
	c.clientMode = true
	c.tracks = []tidal.Track{{ID: 1}, {ID: 2}, {ID: 3}}
	c.tracksOrder = c.tracks

	reshuffled := []tidal.Track{{ID: 1}, {ID: 3}, {ID: 2}}
	got := asModel(t, must(c.Update(parentStateMsg(mpris.PlayerState{
		PlaylistJSON: mpris.MarshalTracks(reshuffled),
		ShuffleMode:  ShuffleFisherYates.String(),
	}))))

	if !sameTrackIDs(got.tracks, reshuffled) {
		t.Errorf("client order %v, want the parent's %v", trackIDs(got.tracks), trackIDs(reshuffled))
	}
}

// TestCycleShuffleForwardsInClientMode asserts a client's shuffle key goes to
// the parent and leaves the local queue, cursor, and mode alone until the
// parent's state comes back.
func TestCycleShuffleForwardsInClientMode(t *testing.T) {
	m := newSmokeModel()
	m.clientMode = true
	m.cursor = 2
	before := append([]tidal.Track(nil), m.tracks...)

	cmd := m.cycleShuffle()

	if cmd == nil {
		t.Fatal("no command returned; the parent never hears about the shuffle")
	}
	if m.shuffleMode != ShuffleOff {
		t.Errorf("client mode changed to %v locally; the parent's mode is authoritative", m.shuffleMode)
	}
	if !sameTrackIDs(m.tracks, before) {
		t.Errorf("client queue reordered to %v locally", trackIDs(m.tracks))
	}
	if m.cursor != 2 {
		t.Errorf("cursor moved to %d, want it left at 2", m.cursor)
	}
}

// TestParentAppliesClientShuffle drives the parent's side of SetShuffle: the
// mode changes, the queue keeps the same tracks, and the cursor stays on the
// playing track so auto-advance continues from it.
func TestParentAppliesClientShuffle(t *testing.T) {
	for _, mode := range []ShuffleMode{ShuffleFisherYates, ShuffleRandom, ShuffleOff} {
		t.Run(mode.String(), func(t *testing.T) {
			m := newSmokeModel()
			m.tracksOrder = numberedTracks(20)
			m.applyShuffle()
			m.cursor = 7
			playing := m.tracks[7]
			m.currentTrack = &playing

			got := asModel(t, must(m.Update(mprisMsg(mpris.Event{
				Cmd: mpris.CmdSetShuffle, ShuffleMode: mode.String(),
			}))))

			if got.shuffleMode != mode {
				t.Errorf("shuffle mode = %v, want %v", got.shuffleMode, mode)
			}
			if len(got.tracks) != 20 {
				t.Fatalf("queue holds %d tracks, want 20", len(got.tracks))
			}
			if got.tracks[got.cursor].ID != playing.ID {
				t.Errorf("cursor on track %d, want the playing track %d", got.tracks[got.cursor].ID, playing.ID)
			}
			if mode == ShuffleFisherYates && got.cursor != 0 {
				t.Errorf("cursor = %d; the playing track should lead the shuffled queue", got.cursor)
			}
		})
	}
}

// TestClientHandsOverUnshuffledPlaylist asserts the playlist a client sends is
// its original order, not a shuffled display order the parent would then treat
// as original, and that the start index points into that same list.
func TestClientHandsOverUnshuffledPlaylist(t *testing.T) {
	m := newSmokeModel()
	m.clientMode = true
	m.tracksOrder = []tidal.Track{{ID: 1}, {ID: 2}, {ID: 3}}
	m.tracks = []tidal.Track{{ID: 3}, {ID: 1}, {ID: 2}}

	tracksJSON, idx := m.playlistHandover(3)

	if want := mpris.MarshalTracks(m.tracksOrder); tracksJSON != want {
		t.Errorf("sent %s, want the unshuffled order %s", tracksJSON, want)
	}
	if idx != 2 {
		t.Errorf("start index = %d, want 2 (track 3 in the sent list)", idx)
	}
}
