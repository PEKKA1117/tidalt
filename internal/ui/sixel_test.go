package ui

import (
	"image"
	"image/color"
	"image/color/palette"
	"strconv"
	"strings"
	"testing"
)

// decodeSixel parses a payload produced by sixelEncode back into a pixel grid
// of palette indices, with -1 for a pixel no colour claimed. It exists so the
// encoder can be checked against what a terminal would actually paint rather
// than against the shape of its own output.
func decodeSixel(t *testing.T, s string, pxW, pxH int) [][]int {
	t.Helper()

	body, ok := strings.CutPrefix(s, "\x1bP0;1;0q")
	if !ok {
		t.Fatalf("payload does not start with a sixel DCS: %.20q", s)
	}
	body, ok = strings.CutSuffix(body, "\x1b\\")
	if !ok {
		t.Fatal("payload is not terminated by ST")
	}

	grid := make([][]int, pxH)
	for y := range grid {
		grid[y] = make([]int, pxW)
		for x := range grid[y] {
			grid[y][x] = -1
		}
	}

	var (
		top, x, cur int
		i           int
	)
	readInt := func() int {
		start := i
		for i < len(body) && body[i] >= '0' && body[i] <= '9' {
			i++
		}
		n, _ := strconv.Atoi(body[start:i])
		return n
	}
	paint := func(ch byte, run int) {
		mask := ch - sixelCharBase
		for range run {
			for bit := range sixelBandHeight {
				if mask&(1<<bit) == 0 {
					continue
				}
				if y := top + bit; y < pxH && x < pxW {
					grid[y][x] = cur
				}
			}
			x++
		}
	}

	for i < len(body) {
		switch c := body[i]; {
		case c == '"': // raster attributes — consumed and ignored
			i++
			for i < len(body) && (body[i] == ';' || (body[i] >= '0' && body[i] <= '9')) {
				i++
			}
		case c == '#':
			i++
			cur = readInt()
			if i < len(body) && body[i] == ';' {
				// A colour definition, not a selection; skip its parameters.
				for i < len(body) && (body[i] == ';' || (body[i] >= '0' && body[i] <= '9')) {
					i++
				}
			}
		case c == '$':
			x = 0
			i++
		case c == '-':
			x, top = 0, top+sixelBandHeight
			i++
		case c == '!':
			i++
			run := readInt()
			paint(body[i], run)
			i++
		case c >= sixelCharBase && c <= '~':
			paint(c, 1)
			i++
		default:
			t.Fatalf("unexpected byte %q at offset %d", c, i)
		}
	}
	return grid
}

// solidImage is a pxW×pxH image of one colour.
func solidImage(c color.Color, pxW, pxH int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, pxW, pxH))
	for y := range pxH {
		for x := range pxW {
			img.Set(x, y, c)
		}
	}
	return img
}

// TestSixelSolidRoundTrip asserts every pixel of a flat image is claimed by one
// colour, and that the colour decoded is the one the image was built from.
func TestSixelSolidRoundTrip(t *testing.T) {
	const w, h = 24, 12
	want := palette.Plan9[42]

	out := sixelEncode(solidImage(want, w, h), w, h)
	grid := decodeSixel(t, out, w, h)

	for y := range h {
		for x := range w {
			if grid[y][x] < 0 {
				t.Fatalf("pixel (%d,%d) was left unpainted", x, y)
			}
			if got := palette.Plan9[grid[y][x]]; got != want {
				t.Fatalf("pixel (%d,%d) decoded as %v, want %v", x, y, got, want)
			}
		}
	}
}

// TestSixelCoversEveryPixel asserts a height that is not a multiple of the band
// size still paints its last, partial band — the case where an off-by-one in
// the mask loop would silently clip the bottom of the cover.
func TestSixelCoversEveryPixel(t *testing.T) {
	const w, h = 8, 14 // 14 = two full bands plus two rows
	grid := decodeSixel(t, sixelEncode(solidImage(palette.Plan9[7], w, h), w, h), w, h)

	for y := range h {
		for x := range w {
			if grid[y][x] < 0 {
				t.Errorf("pixel (%d,%d) was left unpainted", x, y)
			}
		}
	}
}

// TestSixelBandSeparators asserts one band separator between bands and none
// trailing: a stray '-' at the end drops the cursor an extra six pixels and
// scrolls the frame.
func TestSixelBandSeparators(t *testing.T) {
	const w = 8
	for _, tc := range []struct{ h, want int }{
		{h: 6, want: 0},
		{h: 12, want: 1},
		{h: 18, want: 2},
		{h: 13, want: 2},
	} {
		out := sixelEncode(solidImage(palette.Plan9[7], w, tc.h), w, tc.h)
		if got := strings.Count(out, "-"); got != tc.want {
			t.Errorf("height %d: %d band separators, want %d", tc.h, got, tc.want)
		}
	}
}

// TestSixelDeclaresRasterSize asserts the declared size matches what was asked
// for. A terminal trusts this to reserve space, so a mismatch is what scrolls
// the frame.
func TestSixelDeclaresRasterSize(t *testing.T) {
	out := sixelEncode(solidImage(palette.Plan9[7], 20, 10), 20, 10)
	if !strings.Contains(out, `"1;1;20;10`) {
		t.Errorf("raster attributes missing or wrong in %.60q", out)
	}
}

// TestSixelRunLength asserts repeats collapse only when doing so is shorter:
// "!4x" costs four bytes, so three repeats must stay literal.
func TestSixelRunLength(t *testing.T) {
	var sb strings.Builder
	writeSixelRunLength(&sb, []byte("aaabbbbc"))
	if got, want := sb.String(), "aaa!4bc"; got != want {
		t.Errorf("run-length output = %q, want %q", got, want)
	}
}

// TestSixelDeclaresOnlyUsedColours asserts the header carries the palette the
// picture needs rather than all 256 entries.
func TestSixelDeclaresOnlyUsedColours(t *testing.T) {
	out := sixelEncode(solidImage(palette.Plan9[42], 12, 6), 12, 6)
	if got := strings.Count(out, ";2;"); got != 1 {
		t.Errorf("%d colour definitions for a flat image, want 1", got)
	}
}

// TestSixelRejectsDegenerateInput asserts nothing is emitted when there is
// nothing to draw, so a caller cannot write a half-formed DCS to the terminal.
func TestSixelRejectsDegenerateInput(t *testing.T) {
	for _, tc := range []struct {
		name     string
		img      image.Image
		pxW, pxH int
	}{
		{name: "nil image", img: nil, pxW: 10, pxH: 10},
		{name: "zero width", img: solidImage(color.Black, 4, 4), pxW: 0, pxH: 10},
		{name: "negative height", img: solidImage(color.Black, 4, 4), pxW: 10, pxH: -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sixelEncode(tc.img, tc.pxW, tc.pxH); got != "" {
				t.Errorf("got %.40q, want empty", got)
			}
		})
	}
}

// TestDamagedRows covers the line comparison the repair is driven by.
func TestDamagedRows(t *testing.T) {
	for _, tc := range []struct {
		name       string
		drawn, cur []string
		want       [][2]int
	}{
		{name: "identical", drawn: []string{"a", "b", "c"}, cur: []string{"a", "b", "c"}, want: nil},
		{name: "one row", drawn: []string{"a", "b", "c"}, cur: []string{"a", "X", "c"}, want: [][2]int{{1, 2}}},
		{name: "contiguous run", drawn: []string{"a", "b", "c", "d"}, cur: []string{"a", "X", "Y", "d"}, want: [][2]int{{1, 3}}},
		{name: "two runs", drawn: []string{"a", "b", "c", "d"}, cur: []string{"X", "b", "Y", "d"}, want: [][2]int{{0, 1}, {2, 3}}},
		{name: "no history", drawn: nil, cur: []string{"a", "b"}, want: [][2]int{{0, 2}}},
		{name: "resized box", drawn: []string{"a"}, cur: []string{"a", "b"}, want: [][2]int{{0, 2}}},
		{name: "nothing rendered", drawn: []string{"a"}, cur: nil, want: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := damagedRows(tc.drawn, tc.cur)
			if len(got) != len(tc.want) {
				t.Fatalf("damagedRows = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("damagedRows = %v, want %v", got, tc.want)
				}
			}
		})
	}
}
