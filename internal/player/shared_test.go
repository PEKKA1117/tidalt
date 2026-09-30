package player

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
)

// Device strings shared by the tests in this package. They are constants
// rather than literals so the classification tables, the memoisation test, and
// the AudioPath test all speak about the same two devices.
const (
	testHWDevice   = "hw:2,0"
	testPlugDevice = "plughw:2,0"
)

// The shared PCM must never be classified as bit-perfect, and must never be
// confused with a hw: device. Everything downstream — the reservation skip, the
// resample flag, the UI badge — keys off this classification.
func TestDeviceClassification(t *testing.T) {
	cases := []struct {
		device string
		shared bool
		plug   bool
	}{
		{testHWDevice, false, false},
		{"hw:0,0", false, false},
		{testPlugDevice, false, true},
		{"default", true, true},
		{"default:CARD=S9Pro", true, true},
		// A card whose name merely starts with the same letters is not the
		// shared PCM; only the exact name or a `default:` parameter list is.
		{"defaultish", false, false},
	}
	for _, c := range cases {
		if got := IsSharedDevice(c.device); got != c.shared {
			t.Errorf("IsSharedDevice(%q) = %v, want %v", c.device, got, c.shared)
		}
		if got := isPlugDevice(c.device); got != c.plug {
			t.Errorf("isPlugDevice(%q) = %v, want %v", c.device, got, c.plug)
		}
	}
}

// A hw: device must state resample=0 so the bit-perfect contract is explicit
// rather than inherited, and the shared PCM needs a deeper buffer because the
// sound server's own scheduling sits underneath ours.
func TestAlsaTuning(t *testing.T) {
	cases := []struct {
		device        string
		allowResample int
		bufferPeriods int
	}{
		{testHWDevice, 0, 4},
		{testPlugDevice, 1, 4},
		{"default", 1, 8},
	}
	for _, c := range cases {
		resample, periods := alsaTuning(c.device)
		if resample != c.allowResample || periods != c.bufferPeriods {
			t.Errorf("alsaTuning(%q) = (%d, %d), want (%d, %d)",
				c.device, resample, periods, c.allowResample, c.bufferPeriods)
		}
	}
}

// The shared PCM is played through the sound server that ReserveDevice1 would
// ask to step aside, so the handshake must not run at all. A cancelled context
// proves it: reserveALSADevice observes ctx and would fail, so a clean return
// here means the D-Bus path was never entered.
func TestSharedDeviceSkipsReservation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	release, err := reserveUnlessShared(ctx, sharedDevice)
	if err != nil {
		t.Fatalf("reserveUnlessShared(cancelled ctx, %q) = %v, want no error", sharedDevice, err)
	}
	if release == nil {
		t.Fatal("reserveUnlessShared returned a nil release func")
	}
	release() // must be safe to call, and must not panic on a no-op reservation.
}

// An exclusive device still has to yield a card number. Without this the
// shared-mode exemption could swallow a genuinely malformed device string.
func TestReserveUnlessSharedRejectsUnparseableDevice(t *testing.T) {
	release, err := reserveUnlessShared(context.Background(), "not-a-device")
	if err == nil {
		t.Error("reserveUnlessShared accepted a device with no card number")
	}
	if release != nil {
		t.Error("reserveUnlessShared returned a release func alongside an error")
	}
}

// The picker must offer shared mode, and must offer it last so the
// auto-detected DAC keeps the top slot.
func TestListDevicesOffersSharedLast(t *testing.T) {
	devices, err := ListDevices()
	if err != nil {
		t.Skipf("no ALSA devices readable in this environment: %v", err)
	}
	if len(devices) == 0 {
		t.Fatal("ListDevices returned nothing; the shared entry is unconditional")
	}
	last := devices[len(devices)-1]
	if !last.Shared || last.HWName != sharedDevice {
		t.Errorf("last device = %+v, want the shared entry (%q)", last, sharedDevice)
	}
	for _, d := range devices[:len(devices)-1] {
		if d.Shared {
			t.Errorf("hardware device %q is flagged shared", d.HWName)
		}
	}
}

// The claim that shared mode is non-exclusive is not something the pure unit
// tests above can settle: they only check classification. This one opens the
// real PCM twice at the same time. A hw: device answers the second open with
// -EBUSY — that is what exclusive means — so two concurrent handles surviving
// is direct evidence that the card was never claimed.
//
// It needs a working sound server and an audible-in-principle device, so it is
// opt-in: TIDALT_ALSA_SMOKE=1 go test ./internal/player/ -run SharedPCMIsNotExclusive
func TestSharedPCMIsNotExclusive(t *testing.T) {
	if os.Getenv("TIDALT_ALSA_SMOKE") != "1" {
		t.Skip("set TIDALT_ALSA_SMOKE=1 to open real ALSA devices")
	}

	ctx := context.Background()
	const (
		rate     = 44100
		channels = 2
		bits     = 16
	)

	first, err := openALSA(ctx, sharedDevice, channels, rate, bits)
	if err != nil {
		t.Fatalf("first open of %q: %v", sharedDevice, err)
	}
	defer closeALSA(first)

	second, err := openALSA(ctx, sharedDevice, channels, rate, bits)
	if err != nil {
		t.Fatalf("second concurrent open of %q: %v — the shared PCM is behaving exclusively", sharedDevice, err)
	}
	defer closeALSA(second)

	if first.bitPerfect || second.bitPerfect {
		t.Error("the shared PCM must never report bit-perfect output")
	}
	if first.rate != rate {
		t.Errorf("negotiated rate = %d, want %d", first.rate, rate)
	}
	t.Logf("shared PCM opened twice: device=%q rate=%d bytes/sample=%d period=%d buffer=%d",
		first.device, first.rate, first.bytesPerSample, first.periodSize, first.bufferSize)
}

// The counterpart to TestSharedPCMIsNotExclusive, and the check that the
// explicit snd_pcm_hw_params_set_rate_resample(..., 0) added for hw: devices
// did not break the exclusive path it describes. A hw: open must still
// succeed, must report bit-perfect, and must refuse a second concurrent open.
//
// The device is named explicitly rather than auto-detected, because opening a
// hw: endpoint takes the card away from the sound server: point it at an idle
// card, never at one currently playing.
//
//	TIDALT_ALSA_SMOKE_HW=hw:0,0 go test ./internal/player/ -run ExclusiveHWStillClaimsTheCard
func TestExclusiveHWStillClaimsTheCard(t *testing.T) {
	device := os.Getenv("TIDALT_ALSA_SMOKE_HW")
	if device == "" {
		t.Skip("set TIDALT_ALSA_SMOKE_HW=hw:N,0 (an idle card) to open a real hw: device")
	}

	ctx := context.Background()
	const (
		rate     = 44100
		channels = 2
		bits     = 16
	)

	first, err := openALSA(ctx, device, channels, rate, bits)
	if err != nil {
		t.Fatalf("open of %q: %v", device, err)
	}
	defer closeALSA(first)

	if !first.bitPerfect {
		t.Errorf("hw: device %q reported a downgraded path (%q)", device, first.device)
	}
	if first.rate != rate {
		t.Errorf("negotiated rate = %d, want %d — resample=0 should forbid a substitute rate", first.rate, rate)
	}

	second, err := openALSA(ctx, device, channels, rate, bits)
	if err == nil {
		closeALSA(second)
		t.Fatalf("a second concurrent open of %q succeeded; the hw: path is no longer exclusive", device)
	}
	t.Logf("exclusive as expected: device=%q rate=%d bytes/sample=%d period=%d buffer=%d; second open refused: %v",
		first.device, first.rate, first.bytesPerSample, first.periodSize, first.bufferSize, err)
}

// A device selection has to reach a track that is already playing. The counter
// is how it gets there: the playback loop compares it against the generation it
// started on, so every SetDevice must move it, including one that re-selects
// the device already in use — the loop, not SetDevice, decides that is a no-op.
func TestSetDeviceBumpsTheGeneration(t *testing.T) {
	p := NewPlayer()
	start := atomic.LoadUint64(&p.deviceGen)

	p.SetDevice(sharedDevice)
	afterFirst := atomic.LoadUint64(&p.deviceGen)
	if afterFirst == start {
		t.Fatal("SetDevice did not bump deviceGen; a playing track would never notice the change")
	}
	if got, err := p.getDevice(); err != nil || got != sharedDevice {
		t.Fatalf("getDevice() = (%q, %v), want (%q, nil)", got, err, sharedDevice)
	}

	p.SetDevice(sharedDevice)
	if atomic.LoadUint64(&p.deviceGen) == afterFirst {
		t.Error("re-selecting the same device must still bump deviceGen")
	}

	p.SetDevice(testHWDevice)
	if got, _ := p.getDevice(); got != testHWDevice {
		t.Errorf("getDevice() = %q, want %q", got, testHWDevice)
	}
}
