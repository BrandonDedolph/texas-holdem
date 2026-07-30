package components

import (
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
)

// RingGap is an opening cut into the hexagon frame so a seat plate can
// fuse into it. For the top and bottom edges Start/Len are columns; for
// the vertical sides they are rows. The ring leaves the gap blank; the
// layout overlays the plate, whose open edge supplies the fusion
// junctions (see SeatPlateView and PlateEdges).
type RingGap struct {
	Start, Len int
}

// HexRingSpec describes the hexagon frame of the Direction E table
// (docs/table-redesign-pitch-2.md): a horizontal top and bottom edge,
// four slope-1 diagonal corner cuts of Corner rows each, and two vertical
// sides, with gaps where seat plates inset.
//
// The geometry, for width W, height H and corner depth d (0-indexed):
//
//	row 0:            top edge from column d+1 to W-d-2, rounded corners
//	rows 1..d:        the upper diagonals, stepping one column per row
//	rows d+1..H-2-d:  the verticals at columns 0 and W-1
//	rows H-1-d..H-2:  the lower diagonals, stepping back in
//	row H-1:          bottom edge from column d+1 to W-d-2
type HexRingSpec struct {
	Width, Height int
	Corner        int // diagonal rows per corner cut; 0 means 2, the mockup depth

	TopGap, BottomGap   RingGap   // openings in the horizontal edges (columns)
	LeftGaps, RightGaps []RingGap // openings in the verticals (rows)
}

// defaultRingCorner is the corner depth of the 80x24 mockups.
const defaultRingCorner = 2

// HexRing draws the hexagon frame as Height rows of exactly Width display
// cells, interior blank, styled with the ring color. The layout overlays
// seat plates into the gaps and content into the middle. Corner depth is
// clamped down until the shape fits; a frame too small to close (under
// 6x5) returns nil.
func HexRing(spec HexRingSpec) []string {
	rows := hexRingRows(spec)
	if rows == nil {
		return nil
	}
	th := theme.Current
	styled := make([]string, len(rows))
	for i, r := range rows {
		styled[i] = th.RingFrame.Render(r)
	}
	return styled
}

// hexRingRows builds the unstyled frame. Split from HexRing so the
// geometry tests can inspect cells without parsing escape sequences.
func hexRingRows(spec HexRingSpec) []string {
	g := theme.G
	w, h := spec.Width, spec.Height
	d := spec.Corner
	if d <= 0 {
		d = defaultRingCorner
	}
	// Clamp the corner depth until the top edge keeps two corner cells
	// (w >= 2d+4) and at least one vertical row survives (h >= 2d+3).
	for d > 1 && (w < 2*d+4 || h < 2*d+3) {
		d--
	}
	if w < 2*d+4 || h < 2*d+3 {
		return nil
	}

	rows := make([]string, h)
	rows[0] = ringEdgeRow(w, d, g.RingTL, g.RingTR, spec.TopGap)
	rows[h-1] = ringEdgeRow(w, d, g.RingBL, g.RingBR, spec.BottomGap)

	for r := 1; r <= d; r++ {
		cells := blankCells(w)
		cells[d+1-r] = g.DiagUp
		cells[w-d-2+r] = g.DiagDown
		rows[r] = strings.Join(cells, "")

		rb := h - 1 - d + r - 1 // mirrored lower diagonal row
		cells = blankCells(w)
		cells[r] = g.DiagDown
		cells[w-1-r] = g.DiagUp
		rows[rb] = strings.Join(cells, "")
	}

	for r := d + 1; r <= h-2-d; r++ {
		cells := blankCells(w)
		if !inRingGap(spec.LeftGaps, r) {
			cells[0] = g.RingV
		}
		if !inRingGap(spec.RightGaps, r) {
			cells[w-1] = g.RingV
		}
		rows[r] = strings.Join(cells, "")
	}
	return rows
}

// ringEdgeRow draws a horizontal edge row: corners at columns d+1 and
// w-d-2, horizontal fill between, and the gap (clamped to the edge span)
// blanked out.
func ringEdgeRow(w, d int, left, right string, gap RingGap) string {
	g := theme.G
	cells := blankCells(w)
	lo, hi := d+1, w-d-2
	cells[lo] = left
	cells[hi] = right
	for c := lo + 1; c < hi; c++ {
		cells[c] = g.RingH
	}
	for c := gap.Start; c < gap.Start+gap.Len; c++ {
		if c >= lo && c <= hi {
			cells[c] = " "
		}
	}
	return strings.Join(cells, "")
}

func blankCells(w int) []string {
	cells := make([]string, w)
	for i := range cells {
		cells[i] = " "
	}
	return cells
}

func inRingGap(gaps []RingGap, row int) bool {
	for _, g := range gaps {
		if row >= g.Start && row < g.Start+g.Len {
			return true
		}
	}
	return false
}
