package player

import "strings"

// sharedDevice is the ALSA PCM used by the non-exclusive ("shared") output
// mode. It is whatever the system has configured as `default`: on a normal
// desktop that resolves to the PipeWire or PulseAudio ALSA plugin, and on a
// bare ALSA box to dmix. Either way the sound server keeps ownership of the
// card, so every other application keeps playing while tidalt does.
//
// `default` is deliberately preferred over naming the `pipewire` PCM directly.
// The latter only exists when pipewire-alsa is installed and would have to be
// probed for; `default` is guaranteed to resolve on any ALSA installation.
const sharedDevice = "default"

// sharedDeviceInfo is the picker entry for shared mode. It is appended to the
// hardware devices rather than prepended, so the auto-detected DAC keeps the
// top slot and the exclusive path stays the default experience.
func sharedDeviceInfo() DeviceInfo {
	return DeviceInfo{
		HWName:   sharedDevice,
		CardName: "Shared",
		LongName: "Shared — system mixer (PipeWire/PulseAudio)",
		Shared:   true,
	}
}

// IsSharedDevice reports whether device is the shared, non-exclusive PCM.
// Exported because the UI labels this path differently from the plughw:
// downgrade: both forfeit bit-perfect output, but one is a deliberate choice
// and the other is a device refusing to cooperate.
// Shared devices skip the org.freedesktop.ReserveDevice1 handshake entirely:
// the point of this mode is to leave the sound server holding the card, and
// asking it to release the device would defeat that.
func IsSharedDevice(device string) bool {
	return device == sharedDevice || strings.HasPrefix(device, sharedDevice+":")
}

// isPlugDevice reports whether device routes through ALSA's plug layer, either
// because it is the shared PCM or because the plughw: fallback engaged. Such a
// path may resample, remix, or reformat, so it is never bit-perfect.
func isPlugDevice(device string) bool {
	return IsSharedDevice(device) || strings.HasPrefix(device, "plughw:")
}

// alsaTuning returns the PCM parameters that depend on which kind of device is
// being opened.
//
// allowResample is passed to snd_pcm_hw_params_set_rate_resample. On a raw hw:
// device it is set to 0 to make the bit-perfect contract explicit — there is no
// plug layer to resample, so this is a no-op in practice, but it stops the
// claim from resting on an implicit ALSA default. Plug-layer devices need it on
// to do the job they exist for.
//
// bufferPeriods scales the ring buffer. Four periods (~93 ms at 44.1 kHz) is
// right for a USB DAC being fed directly. The shared PCM adds the sound
// server's own graph scheduling underneath ours, and four periods leaves too
// little slack for it, so shared mode gets eight.
func alsaTuning(device string) (allowResample, bufferPeriods int) {
	switch {
	case IsSharedDevice(device):
		return 1, 8
	case isPlugDevice(device):
		return 1, 4
	default:
		return 0, 4
	}
}
