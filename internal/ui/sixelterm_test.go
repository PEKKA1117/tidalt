package ui

import "testing"

// TestDA1HasSixel covers the replies real terminals give, and the parsing trap
// that matters: 4 must be matched as a whole parameter, not as a digit inside
// one.
func TestDA1HasSixel(t *testing.T) {
	for _, tc := range []struct {
		name string
		resp string
		want bool
	}{
		{name: "foot", resp: "\x1b[?62;4;22;28;52c", want: true},
		{name: "xterm with sixel", resp: "\x1b[?63;1;2;4;6;9;15;22c", want: true},
		{name: "sixel only capability", resp: "\x1b[?4c", want: true},
		{name: "vt100 no sixel", resp: "\x1b[?1;2c", want: false},
		{name: "4 inside a larger number", resp: "\x1b[?64;42;14c", want: false},
		{name: "no reply", resp: "", want: false},
		{name: "truncated, no terminator", resp: "\x1b[?62;4;22", want: false},
		{name: "unrelated output", resp: "hello\n", want: false},
		{name: "reply preceded by noise", resp: "x\x1b[?62;4c", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := da1HasSixel(tc.resp); got != tc.want {
				t.Errorf("da1HasSixel(%q) = %v, want %v", tc.resp, got, tc.want)
			}
		})
	}
}
