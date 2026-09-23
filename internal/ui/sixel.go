package ui

import (
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// Sixel encodes an image as six-pixel-tall bands of printable characters. Each
// character carries one column of six pixels as a bitmask offset by '?', so a
// band is written once per colour it contains and the bands are overlaid with
// a carriage return ($) between them.
//
// Unlike the Kitty protocol there is no image id and no placement: the data is
// painted at the cursor and the terminal keeps no copy, so a redraw means
// re-sending the whole payload. That is why sixelEncode caches nothing itself —
// the reconcile in syncCover decides when a re-send is actually needed.
const (
	// sixelBandHeight is the number of pixel rows one sixel character spans.
	sixelBandHeight = 6
	// sixelCharBase offsets a six-bit mask into printable ASCII ('?' = 0).
	sixelCharBase = '?'
	// sixelMaxColors is the palette size. 256 is what every sixel terminal is
	// required to support; foot allows more but there is no gain here.
	sixelMaxColors = 256
	// sixelRunMin is the shortest run worth writing as !<count><char> rather
	// than repeating the character. "!4x" is four bytes, so runs of four break
	// even and only runs above that save anything.
	sixelRunMin = 4
)

// sixelEncode renders img as a sixel payload sized exactly pxW×pxH pixels,
// scaling to fit. It returns "" for a nil image or a degenerate size.
//
// The exact pixel size matters: a sixel image is drawn from the cursor
// downwards with no clipping, so one that is taller than the box reserved for
// it pushes the terminal into scrolling the frame.
func sixelEncode(img image.Image, pxW, pxH int) string {
	q := sixelQuantise(img, pxW, pxH)
	if q == nil {
		return ""
	}
	return sixelEncodeRows(q, 0, pxH)
}

// sixelQuantise scales img to pxW×pxH and reduces it to a palette, returning
// nil for input that cannot be drawn.
//
// The result is worth caching by the caller: quantising is the expensive half,
// and a repaint of part of the image re-encodes rows out of this same buffer
// rather than redoing the work. Slicing an already-quantised image also keeps
// the dither pattern continuous across a partial repaint, which re-dithering
// each slice would not.
func sixelQuantise(img image.Image, pxW, pxH int) *image.Paletted {
	if img == nil || pxW <= 0 || pxH <= 0 {
		return nil
	}
	// Scale first, then quantise: dithering a full-size image and sampling it
	// down would scatter the dither pattern into noise.
	scaled := image.NewRGBA(image.Rect(0, 0, pxW, pxH))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), img, img.Bounds(), xdraw.Over, nil)

	quantised := image.NewPaletted(scaled.Bounds(), palette.Plan9)
	draw.FloydSteinberg.Draw(quantised, quantised.Bounds(), scaled, image.Point{})
	return quantised
}

// sixelEncodeRows encodes pixel rows [y0, y1) of an already-quantised image as
// a self-contained payload, to be drawn at the screen row those pixels belong
// to. Encoding a slice rather than the whole image is what makes repairing a
// few damaged rows cheap.
func sixelEncodeRows(quantised *image.Paletted, y0, y1 int) string {
	if quantised == nil {
		return ""
	}
	pxW := quantised.Bounds().Dx()
	y0, y1 = max(y0, 0), min(y1, quantised.Bounds().Dy())
	if pxW <= 0 || y1 <= y0 {
		return ""
	}
	pxH := y1 - y0

	var sb strings.Builder
	// DCS: aspect ratio 1:1, background left untouched, then the raster
	// attributes that declare the true pixel size.
	sb.WriteString("\x1bP0;1;0q")
	fmt.Fprintf(&sb, "\"1;1;%d;%d", pxW, pxH)

	used := sixelPaletteUsed(quantised, y0, y1)
	for _, idx := range used {
		r, g, b := sixelColorPercent(quantised.Palette[idx])
		fmt.Fprintf(&sb, "#%d;2;%d;%d;%d", idx, r, g, b)
	}

	writeSixelBands(&sb, quantised, used, y0, pxW, pxH)

	sb.WriteString("\x1b\\")
	return sb.String()
}

// sixelPaletteUsed lists, in ascending order, the palette indices the image
// actually uses. Declaring only those keeps the header proportional to the
// picture rather than always paying for 256 colour definitions.
func sixelPaletteUsed(img *image.Paletted, y0, y1 int) []int {
	var seen [sixelMaxColors]bool
	for y := y0; y < y1; y++ {
		for x := range img.Bounds().Dx() {
			seen[img.ColorIndexAt(x, y)] = true
		}
	}
	used := make([]int, 0, sixelMaxColors)
	for idx, ok := range seen {
		if ok && idx < len(img.Palette) {
			used = append(used, idx)
		}
	}
	return used
}

// sixelColorPercent converts a colour to the 0-100 per-channel scale sixel
// uses, rounding to nearest so full-scale channels survive the trip.
func sixelColorPercent(c color.Color) (r, g, b int) {
	r16, g16, b16, _ := c.RGBA()
	scale := func(v uint32) int { return int((v*100 + 0x7fff) / 0xffff) }
	return scale(r16), scale(g16), scale(b16)
}

// writeSixelBands emits the image body: one pass per colour per six-row band,
// separated by $ (return to the band's first column) and terminated by - (drop
// to the next band). Colours absent from a band are skipped entirely.
func writeSixelBands(sb *strings.Builder, img *image.Paletted, used []int, y0, pxW, pxH int) {
	row := make([]byte, pxW)
	for top := 0; top < pxH; top += sixelBandHeight {
		written := false
		for _, idx := range used {
			if !sixelBandRow(img, row, idx, y0+top, pxW, min(top+sixelBandHeight, pxH)-top) {
				continue
			}
			if written {
				sb.WriteByte('$') // overlay the next colour on the same band
			}
			fmt.Fprintf(sb, "#%d", idx)
			writeSixelRunLength(sb, row)
			written = true
		}
		if top+sixelBandHeight < pxH {
			sb.WriteByte('-')
		}
	}
}

// sixelBandRow fills row with the six-bit column masks for one colour across
// one band, and reports whether the colour appears there at all.
func sixelBandRow(img *image.Paletted, row []byte, idx, top, pxW, rows int) bool {
	found := false
	for x := range pxW {
		var mask byte
		for bit := range sixelBandHeight {
			if bit >= rows {
				break
			}
			if int(img.ColorIndexAt(x, top+bit)) == idx {
				mask |= 1 << bit
			}
		}
		row[x] = sixelCharBase + mask
		if mask != 0 {
			found = true
		}
	}
	return found
}

// writeSixelRunLength writes row, collapsing repeats into !<count><char>.
// Album art is full of flat regions, so this is most of the payload saving.
func writeSixelRunLength(sb *strings.Builder, row []byte) {
	for i := 0; i < len(row); {
		j := i
		for j < len(row) && row[j] == row[i] {
			j++
		}
		run := j - i
		if run >= sixelRunMin {
			fmt.Fprintf(sb, "!%d%c", run, row[i])
		} else {
			for range run {
				sb.WriteByte(row[i])
			}
		}
		i = j
	}
}

// sixelDrawAt returns the escape that paints payload with its top-left corner
// at the absolute 1-indexed cell (row, col). The cursor is saved and restored
// around it — DECSC/DECRC carry the graphic attributes too — so the text frame
// underneath is left exactly as it was.
func sixelDrawAt(col, row int, payload string) string {
	if payload == "" {
		return ""
	}
	return fmt.Sprintf("\x1b7\x1b[%d;%dH%s\x1b8", row, col, payload)
}

// sixelClearBox returns the escape that erases a cols×rows cell box at the
// absolute 1-indexed cell (row, col).
//
// Sixel has no delete: the pixels are simply on the screen. Nor does the text
// frame clean up after it, because the box is reserved as unchanged blank
// cells and BubbleTea skips lines that did not change. So the box is painted
// over with spaces, after resetting the attributes so the blanks match the
// reserved cells rather than whatever colour was last set.
func sixelClearBox(col, row, cols, rows int) string {
	if cols <= 0 || rows <= 0 {
		return ""
	}
	blank := strings.Repeat(" ", cols)
	var sb strings.Builder
	sb.WriteString("\x1b7\x1b[0m")
	for i := range rows {
		fmt.Fprintf(&sb, "\x1b[%d;%dH%s", row+i, col, blank)
	}
	sb.WriteString("\x1b8")
	return sb.String()
}

// damagedRows returns the half-open ranges of box rows whose frame lines
// changed between drawn and current, which is precisely the set BubbleTea
// rewrote and therefore the set whose sixel pixels are gone.
//
// A length mismatch means the box was resized or has no history, and the whole
// thing is reported as damaged.
func damagedRows(drawn, current []string) [][2]int {
	if len(drawn) != len(current) {
		if len(current) == 0 {
			return nil
		}
		return [][2]int{{0, len(current)}}
	}
	var out [][2]int
	for i := 0; i < len(current); {
		if drawn[i] == current[i] {
			i++
			continue
		}
		start := i
		for i < len(current) && drawn[i] != current[i] {
			i++
		}
		out = append(out, [2]int{start, i})
	}
	return out
}
