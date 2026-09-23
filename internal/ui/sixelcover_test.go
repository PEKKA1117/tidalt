package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"
)

const sixelDCS = "\x1bP0;1;0q"

// sixelCoverModel returns a Queue-section model that draws with sixel, wired to
// an in-memory writer and a fixed cell size so no terminal is needed.
func sixelCoverModel(t *testing.T) (Model, *bytes.Buffer) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			img.Set(x, y, color.RGBA{200, 120, 60, 255})
		}
	}
	buf := &bytes.Buffer{}
	m := newSmokeModel()
	m.width, m.height = 120, 40
	m.section = SecQueue
	m.cursor = 1
	m.coverImage = img
	m.coverCacheKey = "cover-a"
	m.gfxMode = coverSixel
	m.ttyOut = buf
	m.kitty = &kittyState{}
	m.cellPx = func() (int, int, bool) { return 8, 16, true }
	return m, buf
}

// frame mimics one BubbleTea cycle in the order the runtime performs it: the
// view is built (which records the lines lying over the cover box), the
// renderer would write it, and only then does the reconcile run.
func frame(m *Model) {
	_ = m.View()
	m.syncCover()
}

// TestSixelCoverDrawsWithoutViewChange mirrors the Kitty regression: the
// payload has to reach the terminal on its own, never through the View string,
// which BubbleTea truncates and diffs.
func TestSixelCoverDrawsWithoutViewChange(t *testing.T) {
	m, buf := sixelCoverModel(t)

	frame(&m)
	if !strings.Contains(buf.String(), sixelDCS) {
		t.Fatal("no sixel payload written to the TTY on first sync")
	}
	if strings.Contains(m.View(), sixelDCS) {
		t.Error("sixel payload leaked into the View string; the renderer will mangle it")
	}

	buf.Reset()
	frame(&m)
	if buf.Len() != 0 {
		t.Errorf("an unchanged frame still wrote %d bytes", buf.Len())
	}
}

// TestSixelCoverSizedToItsBox asserts the payload declares exactly the pixel
// size of the cell box it is drawn into. A taller image is not clipped by the
// terminal — it scrolls the frame — so this is the difference between a cover
// and a corrupted screen.
func TestSixelCoverSizedToItsBox(t *testing.T) {
	m, buf := sixelCoverModel(t)
	_, _, panelW, imgRows, ok := m.coverBoxRect()
	if !ok {
		t.Fatal("no cover box in the Queue section")
	}

	frame(&m)

	want := fmt.Sprintf(`"1;1;%d;%d`, panelW*8, imgRows*16)
	if !strings.Contains(buf.String(), want) {
		t.Errorf("payload does not declare %s for a %dx%d cell box", want, panelW, imgRows)
	}
}

// TestSixelCoverClearsOldBoxOnMove asserts a moved cover erases where it was
// before drawing where it now is. Sixel pixels are not removed by the text
// frame, which reserves the box as blank cells that BubbleTea then skips as
// unchanged, so a missing clear strands a copy on screen.
func TestSixelCoverClearsOldBoxOnMove(t *testing.T) {
	m, buf := sixelCoverModel(t)
	frame(&m)
	col, row, panelW, _, _ := m.coverBoxRect()

	// Narrow the terminal: the cover box moves left.
	buf.Reset()
	m.width = 100
	frame(&m)

	out := buf.String()
	oldHome := fmt.Sprintf("\x1b[%d;%dH", row, col)
	if !strings.Contains(out, oldHome) {
		t.Errorf("no cursor move to the old box at row %d col %d", row, col)
	}
	if !strings.Contains(out, strings.Repeat(" ", panelW)) {
		t.Errorf("old box was not painted over with %d spaces", panelW)
	}
	if !strings.Contains(out, sixelDCS) {
		t.Error("cover was cleared but not redrawn")
	}
	if strings.Index(out, sixelDCS) < strings.Index(out, oldHome) {
		t.Error("redraw was written before the clear; the clear would erase it")
	}
}

// TestSixelCoverClearsWhenHidden asserts leaving the Queue erases the image.
// The box is gone from the layout by then, so the clear can only work from the
// rect recorded when it was drawn.
func TestSixelCoverClearsWhenHidden(t *testing.T) {
	m, buf := sixelCoverModel(t)
	frame(&m)
	col, row, panelW, imgRows, _ := m.coverBoxRect()

	buf.Reset()
	m.section = SecSearch
	frame(&m)

	out := buf.String()
	for i := range imgRows {
		want := fmt.Sprintf("\x1b[%d;%dH%s", row+i, col, strings.Repeat(" ", panelW))
		if !strings.Contains(out, want) {
			t.Fatalf("row %d of the old box was not erased", row+i)
		}
	}

	buf.Reset()
	frame(&m)
	if buf.Len() != 0 {
		t.Errorf("clearing repeated on a later sync, wrote %d bytes", buf.Len())
	}
}

// TestSixelCoverSilentWithoutCellSize asserts nothing is drawn when the
// terminal will not say how big a cell is: the image could not be sized to its
// box, and guessing scrolls the frame.
func TestSixelCoverSilentWithoutCellSize(t *testing.T) {
	m, buf := sixelCoverModel(t)
	m.cellPx = func() (int, int, bool) { return 0, 0, false }

	frame(&m)

	if buf.Len() != 0 {
		t.Errorf("wrote %d bytes without knowing the cell size", buf.Len())
	}
}

// TestSixelCoverReusesQuantisedImage asserts the expensive half — scaling and
// dithering — is redone only when the cover or its pixel size changes, not on
// every repaint.
func TestSixelCoverReusesQuantisedImage(t *testing.T) {
	m, _ := sixelCoverModel(t)
	frame(&m)
	first := m.kitty.quant

	// A repaint of the same cover at the same size must reuse it.
	m.cursor = 2
	frame(&m)
	if m.kitty.quant != first {
		t.Error("cover was re-quantised for a repaint at the same size")
	}

	// A different box size must not.
	m.width = 100
	frame(&m)
	if m.kitty.quant == first {
		t.Error("quantised image was reused for a box of a different size")
	}
}

// TestSixelCoverRepairsOnlyDamagedRows is the regression for rows of the cover
// being eaten away while scrolling a list of tracks that share one cover.
//
// The renderer rewrites a frame line whenever any part of it changes, and the
// cover shares its lines with the track list, so moving the cursor overwrites
// the image in exactly those rows. Nothing about the cover itself changed, so
// the reconcile used to conclude the image was already on screen and write
// nothing — and the damage accumulated, one row per keypress.
func TestSixelCoverRepairsOnlyDamagedRows(t *testing.T) {
	m, buf := sixelCoverModel(t)
	frame(&m)
	full := buf.Len()
	col, row, _, _, ok := m.coverBoxRect()
	if !ok {
		t.Fatal("no cover box in the Queue section")
	}

	// Move the cursor: the two list rows involved are redrawn, and with them
	// the cover cells on those lines.
	buf.Reset()
	m.cursor = 2
	frame(&m)

	out := buf.String()
	if out == "" {
		t.Fatal("damaged rows were not repainted; they stay as terminal background")
	}
	if buf.Len() >= full {
		t.Errorf("repair wrote %d bytes against %d for the whole cover; it is not targeting the damage", buf.Len(), full)
	}
	if n := strings.Count(out, sixelDCS); n == 0 {
		t.Fatal("no sixel payload in the repair")
	}
	// The repair must be placed on the rows that changed, in the box's own
	// column — a payload drawn at the box's top-left would be the whole-cover
	// repaint this is meant to avoid.
	if !strings.Contains(out, fmt.Sprintf("\x1b[%d;%dH", row+1, col)) &&
		!strings.Contains(out, fmt.Sprintf("\x1b[%d;%dH", row+2, col)) {
		t.Errorf("repair was not drawn at the rows that changed (box starts at row %d)", row)
	}
}
