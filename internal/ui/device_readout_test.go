package ui

import "testing"

// Device strings used by the readout cases below.
const (
	readoutHW     = "hw:2,0"
	readoutPlug   = "plughw:2,0"
	readoutShared = "default"
	readoutOther  = "hw:4,0"
)

// The readout is the only feedback a user gets that picking a device did
// anything, so it has to distinguish three situations that all present as
// "currentDevice and activeDevice disagree": a switch in flight, a plughw:
// downgrade that will never reach the requested device, and a player that is
// not running at all.
func TestDeviceReadout(t *testing.T) {
	cases := []struct {
		name          string
		currentDevice string
		activeDevice  string
		isPlaying     bool
		want          string
	}{
		{
			name:          "nothing selected and nothing open",
			currentDevice: "",
			activeDevice:  "",
			isPlaying:     false,
			want:          "auto",
		},
		{
			name:          "selected but nothing opened yet",
			currentDevice: readoutShared,
			activeDevice:  "",
			isPlaying:     false,
			want:          readoutShared,
		},
		{
			name:          "playing on the selected device",
			currentDevice: readoutHW,
			activeDevice:  readoutHW,
			isPlaying:     true,
			want:          readoutHW,
		},
		{
			name:          "switch to shared in flight",
			currentDevice: readoutShared,
			activeDevice:  readoutHW,
			isPlaying:     true,
			want:          readoutHW + " → " + readoutShared,
		},
		{
			name:          "switch back to a hw device",
			currentDevice: readoutOther,
			activeDevice:  readoutShared,
			isPlaying:     true,
			want:          readoutShared + " → " + readoutOther,
		},
		{
			// The plug fallback is permanent for that device: promising a
			// switch that is not coming would be worse than saying nothing.
			name:          "plughw downgrade is not a switch",
			currentDevice: readoutHW,
			activeDevice:  readoutPlug,
			isPlaying:     true,
			want:          readoutPlug,
		},
		{
			// Stopped: the arrow would imply something is in progress.
			name:          "not playing, so no switch is under way",
			currentDevice: readoutShared,
			activeDevice:  readoutHW,
			isPlaying:     false,
			want:          readoutHW,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newSmokeModel()
			m.currentDevice = c.currentDevice
			m.activeDevice = c.activeDevice
			m.isPlaying = c.isPlaying
			if got := m.deviceReadout(); got != c.want {
				t.Errorf("deviceReadout() = %q, want %q", got, c.want)
			}
		})
	}
}
