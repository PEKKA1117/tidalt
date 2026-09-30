package player

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// flacFixture renders a short silent FLAC with ffmpeg and returns its path.
// Silent rather than a tone because this test runs on a real desktop with real
// speakers; the playback loop is exercised either way.
func flacFixture(t *testing.T) string {
	t.Helper()

	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skipf("ffmpeg not available to build the fixture: %v", err)
	}

	path := filepath.Join(t.TempDir(), "fixture.flac")
	// The binary comes from LookPath and every argument is a literal, so the
	// only variable here is the resolved path to ffmpeg itself.
	//nolint:gosec // G204: no caller-controlled input reaches this command
	cmd := exec.CommandContext(t.Context(), ffmpeg, "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "anullsrc=r=44100:cl=stereo",
		"-t", "30", "-sample_fmt", "s16", path)
	if out, cerr := cmd.CombinedOutput(); cerr != nil {
		t.Fatalf("ffmpeg failed to build the fixture: %v\n%s", cerr, out)
	}
	return path
}

// waitForDevice polls AudioPath until it reports want, and fails if it never
// does. The switch is not instantaneous — the stream is torn down, the old
// device released, and the new one reserved and opened — so this cannot be a
// single read after a fixed sleep.
func waitForDevice(t *testing.T, p *Player, want string, within time.Duration) {
	t.Helper()

	deadline := time.Now().Add(within)
	var last string
	for time.Now().Before(deadline) {
		if device, _ := p.AudioPath(); device == want {
			return
		} else {
			last = device
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("device never became %q within %s; last reported %q", want, within, last)
}

// The end-to-end check for the thing the unit tests cannot reach: a device
// picked while a track is already playing must move that track, not sit in
// deviceOverride until some later Play(). This drives the real playback loop
// over a local HTTP stream, so it needs no Tidal session.
//
// TIDALT_ALSA_SMOKE_HW names an IDLE card — the switch takes it away from the
// sound server. Check /proc/asound/card*/pcm*p/sub0/status first.
//
//	TIDALT_ALSA_SMOKE=1 TIDALT_ALSA_SMOKE_HW=hw:0,0 \
//	  go test ./internal/player/ -run DeviceSwitchMovesAPlayingStream -v
func TestDeviceSwitchMovesAPlayingStream(t *testing.T) {
	if os.Getenv("TIDALT_ALSA_SMOKE") != "1" {
		t.Skip("set TIDALT_ALSA_SMOKE=1 to open real ALSA devices")
	}
	target := os.Getenv("TIDALT_ALSA_SMOKE_HW")
	if target == "" {
		t.Skip("set TIDALT_ALSA_SMOKE_HW=hw:N,0 (an idle card) to name the device to switch to")
	}

	fixture := flacFixture(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, fixture)
	}))
	defer srv.Close()

	p := NewPlayer()
	defer p.Close()
	if err := p.SetVolume(0); err != nil {
		t.Fatalf("SetVolume(0): %v", err)
	}

	p.SetDevice(sharedDevice)
	if _, err := p.Play(srv.URL); err != nil {
		t.Fatalf("Play on %q: %v", sharedDevice, err)
	}
	waitForDevice(t, p, sharedDevice, 5*time.Second)

	if _, bitPerfect := p.AudioPath(); bitPerfect {
		t.Error("playing on the shared PCM but reporting bit-perfect")
	}

	// The switch under test: change the device with the stream already running.
	p.SetDevice(target)
	waitForDevice(t, p, target, 10*time.Second)

	device, bitPerfect := p.AudioPath()
	if !bitPerfect {
		t.Errorf("after switching to %q, AudioPath reports a downgraded path (%q)", target, device)
	}
	pos, perr := p.GetPosition()
	if perr != nil {
		t.Errorf("GetPosition after the switch: %v", perr)
	} else if pos <= 0 {
		t.Errorf("position is %v after the switch; the stream should have resumed, not restarted at zero", pos)
	}

	// And back again, to prove the handover is not one-way.
	p.SetDevice(sharedDevice)
	waitForDevice(t, p, sharedDevice, 10*time.Second)

	t.Logf("switched shared → %s → shared with the stream running; position %.2fs", target, pos)
}
