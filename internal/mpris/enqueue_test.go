package mpris

import (
	"testing"
	"time"
)

// TestEnqueueEmitsEvent asserts the private D-Bus method turns a client's
// queue addition into the event the UI loop consumes, carrying the position
// flag intact.
func TestEnqueueEmitsEvent(t *testing.T) {
	for _, tc := range []struct {
		name string
		next bool
	}{
		{name: "append", next: false},
		{name: "play next", next: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ch := make(chan Event, 1)
			app := &tidalApp{ch: ch}

			if dErr := app.Enqueue(`{"id":42}`, tc.next); dErr != nil {
				t.Fatalf("Enqueue returned %v", dErr)
			}

			select {
			case ev := <-ch:
				if ev.Cmd != CmdEnqueue {
					t.Errorf("Cmd = %v, want CmdEnqueue", ev.Cmd)
				}
				if ev.TrackJSON != `{"id":42}` {
					t.Errorf("TrackJSON = %q, want the payload unchanged", ev.TrackJSON)
				}
				if ev.EnqueueNext != tc.next {
					t.Errorf("EnqueueNext = %v, want %v", ev.EnqueueNext, tc.next)
				}
			default:
				t.Fatal("no event was queued")
			}
		})
	}
}

// TestEnqueueDropsWhenNobodyListens asserts a full channel cannot wedge the
// D-Bus call, matching the other methods on this interface.
func TestEnqueueDropsWhenNobodyListens(t *testing.T) {
	ch := make(chan Event) // unbuffered, no reader
	app := &tidalApp{ch: ch}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = app.Enqueue(`{"id":1}`, false)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Enqueue blocked on an unread channel")
	}
}
