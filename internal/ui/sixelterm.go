package ui

import (
	"os"
	"slices"
	"strings"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// da1Timeout bounds the wait for a Primary Device Attributes reply. A terminal
// that supports the query answers immediately; one that does not answers never,
// and this is how long startup pays to find that out.
const da1Timeout = 150 * time.Millisecond

// SixelSupported reports whether the terminal can draw sixel graphics, by
// asking it: DA1 (ESC [ c) is answered with a list of capability numbers, and 4
// is sixel.
//
// This has to be a query rather than a table of terminal names. foot, xterm,
// mlterm, contour and WezTerm all speak sixel, several of them only when
// configured to (foot's tweak.sixel can turn it off), and TERM says nothing
// about any of that. The cost is one round trip at startup, before BubbleTea
// takes over the terminal.
func SixelSupported() bool {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return false
	}
	resp, err := queryTerminal("\x1b[c")
	if err != nil {
		return false
	}
	return da1HasSixel(resp)
}

// queryTerminal writes a control sequence to the controlling terminal and
// returns whatever it answers within da1Timeout. The terminal is put in raw
// mode for the exchange so the reply is not echoed or held back for a newline,
// and is restored before returning however that turns out.
func queryTerminal(query string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", err
	}
	defer func() { _ = tty.Close() }()

	fd := int(tty.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return "", err
	}
	defer func() { _ = term.Restore(fd, state) }()

	if _, err := tty.WriteString(query); err != nil {
		return "", err
	}
	if err := tty.SetReadDeadline(time.Now().Add(da1Timeout)); err != nil {
		return "", err
	}

	// One read is enough: a DA1 reply is a couple of dozen bytes and arrives in
	// a single burst. A short read on timeout still returns what did arrive.
	buf := make([]byte, 64)
	n, err := tty.Read(buf)
	if n == 0 && err != nil {
		return "", err
	}
	return string(buf[:n]), nil
}

// da1HasSixel reports whether a DA1 reply advertises sixel support. The reply
// is ESC [ ? <n> ; <n> ; … c, and capability 4 is sixel — foot answers
// ESC [ ? 62 ; 4 ; 22 ; 28 ; 52 c. Matching is on whole parameters so that 4
// is not found inside 42 or 14.
func da1HasSixel(resp string) bool {
	_, body, ok := strings.Cut(resp, "\x1b[?")
	if !ok {
		return false
	}
	params, _, ok := strings.Cut(body, "c")
	if !ok {
		return false
	}
	return slices.Contains(strings.Split(params, ";"), "4")
}

// cellPixelSize returns the pixel dimensions of one terminal cell, derived from
// the window size the kernel reports.
//
// Sixel is sized in pixels while the layout is in cells, so this is what ties
// the two together. It is read fresh on every draw rather than cached: changing
// the font size changes the cell size without changing the cell grid, and a
// stale value would size the image wrongly and scroll the frame. ok is false
// when the terminal does not report pixel dimensions, in which case there is no
// honest way to size an image and the caller must not draw one.
func cellPixelSize() (w, h int, ok bool) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 || ws.Row == 0 || ws.Xpixel == 0 || ws.Ypixel == 0 {
		return 0, 0, false
	}
	return int(ws.Xpixel) / int(ws.Col), int(ws.Ypixel) / int(ws.Row), true
}
