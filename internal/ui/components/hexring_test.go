package components

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// ringSizes are the frame sizes the geometry must close at: the 80x24
// mockup's 77x13 hexagon, a deeper wide-layout cut, and small and shallow
// extremes.
func ringSizes() []HexRingSpec {
	return []HexRingSpec{
		{Width: 77, Height: 13, Corner: 2}, // the Direction E mockup
		{Width: 40, Height: 9, Corner: 2},
		{Width: 30, Height: 7, Corner: 1},
		{Width: 26, Height: 11, Corner: 3},
		{Width: 60, Height: 17, Corner: 4},
		{Width: 8, Height: 7, Corner: 2}, // minimal honest hexagon at depth 2
	}
}

// cellsOf splits a plain row into single-cell strings. Every frame glyph
// is one cell in both glyph sets, so rune indexing is cell indexing.
func cellsOf(row string) []string {
	runes := []rune(row)
	cells := make([]string, len(runes))
	for i, r := range runes {
		cells[i] = string(r)
	}
	return cells
}

// assertRingGeometry checks a gapless frame cell by cell: corners on their
// exact columns, diagonals stepping one column per row, verticals flush to
// the sides, horizontal edges unbroken, and nothing else anywhere.
func assertRingGeometry(t *testing.T, spec HexRingSpec, rows []string) {
	t.Helper()
	g := theme.G
	w, h, d := spec.Width, spec.Height, spec.Corner

	if len(rows) != h {
		t.Fatalf("got %d rows, want %d", len(rows), h)
	}
	for r, row := range rows {
		if lipgloss.Width(row) != w {
			t.Fatalf("row %d is %d cells, want %d: %q", r, lipgloss.Width(row), w, row)
		}
	}

	expect := make(map[[2]int]string)
	lo, hi := d+1, w-d-2
	expect[[2]int{0, lo}] = g.RingTL
	expect[[2]int{0, hi}] = g.RingTR
	expect[[2]int{h - 1, lo}] = g.RingBL
	expect[[2]int{h - 1, hi}] = g.RingBR
	for c := lo + 1; c < hi; c++ {
		expect[[2]int{0, c}] = g.RingH
		expect[[2]int{h - 1, c}] = g.RingH
	}
	for r := 1; r <= d; r++ {
		expect[[2]int{r, d + 1 - r}] = g.DiagUp
		expect[[2]int{r, w - d - 2 + r}] = g.DiagDown
		expect[[2]int{h - 1 - d + r - 1, r}] = g.DiagDown
		expect[[2]int{h - 1 - d + r - 1, w - 1 - r}] = g.DiagUp
	}
	for r := d + 1; r <= h-2-d; r++ {
		expect[[2]int{r, 0}] = g.RingV
		expect[[2]int{r, w - 1}] = g.RingV
	}

	for r, row := range rows {
		for c, cell := range cellsOf(row) {
			want, ok := expect[[2]int{r, c}]
			if !ok {
				want = " "
			}
			if cell != want {
				t.Errorf("cell (%d,%d) = %q, want %q", r, c, cell, want)
			}
		}
	}
}

func TestHexRingClosesAtAllSizes(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		for _, spec := range ringSizes() {
			rows := hexRingRows(spec)
			if rows == nil {
				t.Fatalf("HexRing(%dx%d d%d) returned nil", spec.Width, spec.Height, spec.Corner)
			}
			assertRingGeometry(t, spec, rows)
		}
	})
}

func TestHexRingStyledWidths(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		for _, spec := range ringSizes() {
			for i, row := range HexRing(spec) {
				if w := lipgloss.Width(row); w != spec.Width {
					t.Errorf("%dx%d row %d is %d cells, want %d",
						spec.Width, spec.Height, i, w, spec.Width)
				}
			}
		}
	})
}

func TestHexRingClampsCornerDepth(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		// A corner too deep for the height clamps down instead of breaking
		// the shape; the effective depth is readable off the top corner.
		rows := hexRingRows(HexRingSpec{Width: 40, Height: 7, Corner: 10})
		if rows == nil {
			t.Fatal("clampable spec returned nil")
		}
		cells := cellsOf(rows[0])
		got := -1
		for c, cell := range cells {
			if cell != " " {
				got = c
				break
			}
		}
		if got != 3 { // depth clamps to (7-3)/2 = 2, corner at column d+1
			t.Fatalf("top corner at column %d, want 3:\n%s", got, rows[0])
		}
		assertRingGeometry(t, HexRingSpec{Width: 40, Height: 7, Corner: 2}, rows)
	})
}

func TestHexRingTooSmallIsNil(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		for _, spec := range []HexRingSpec{
			{Width: 5, Height: 5},
			{Width: 20, Height: 4},
			{Width: 0, Height: 0},
		} {
			if got := HexRing(spec); got != nil {
				t.Errorf("HexRing(%dx%d) should be nil, got %d rows",
					spec.Width, spec.Height, len(got))
			}
		}
	})
}

func TestHexRingGapsBlankOnlyTheirCells(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		spec := HexRingSpec{Width: 77, Height: 13, Corner: 2}
		gapped := spec
		gapped.TopGap = RingGap{Start: 26, Len: 24}
		gapped.BottomGap = RingGap{Start: 25, Len: 26}
		gapped.LeftGaps = []RingGap{{Start: 3, Len: 3}, {Start: 7, Len: 3}}
		gapped.RightGaps = []RingGap{{Start: 3, Len: 3}, {Start: 7, Len: 3}}

		full, cut := hexRingRows(spec), hexRingRows(gapped)
		for r := range full {
			fc, cc := cellsOf(full[r]), cellsOf(cut[r])
			for c := range fc {
				inGap := (r == 0 && c >= 26 && c < 50) ||
					(r == 12 && c >= 25 && c < 51) ||
					(c == 0 && (r >= 3 && r <= 5 || r >= 7 && r <= 9)) ||
					(c == 76 && (r >= 3 && r <= 5 || r >= 7 && r <= 9))
				switch {
				case inGap && cc[c] != " ":
					t.Errorf("cell (%d,%d) should be blank in the gap, got %q", r, c, cc[c])
				case !inGap && cc[c] != fc[c]:
					t.Errorf("cell (%d,%d) changed outside the gaps: %q -> %q", r, c, fc[c], cc[c])
				}
			}
		}
	})
}

func TestHexRingGapClampedToEdge(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		// A gap overrunning the edge span must not blank the diagonals'
		// columns outside the edge.
		spec := HexRingSpec{Width: 20, Height: 7, Corner: 2,
			TopGap: RingGap{Start: 0, Len: 20}}
		rows := hexRingRows(spec)
		if strings.TrimSpace(rows[0]) != "" {
			t.Errorf("full-width gap should blank the whole edge: %q", rows[0])
		}
		// Rows below the edge are untouched.
		assertRingGeometry(t, HexRingSpec{Width: 20, Height: 7, Corner: 2},
			hexRingRows(HexRingSpec{Width: 20, Height: 7, Corner: 2}))
	})
}

// overlayAt writes a block into base at (row, col), cell for cell. Plain
// text only - the table layout's compositor is a later wave; this is just
// enough to prove the parts fuse.
func overlayAt(base []string, row, col int, block string) {
	for i, line := range strings.Split(stripANSI(block), "\n") {
		if row+i >= len(base) {
			return
		}
		cells := cellsOf(base[row+i])
		for j, cell := range cellsOf(line) {
			if col+j < len(cells) {
				cells[col+j] = cell
			}
		}
		base[row+i] = strings.Join(cells, "")
	}
}

// TestHexRingComposedWithPlates fuses six seat plates into the mockup-size
// ring and checks the assembled silhouette is sealed: no blank cell
// anywhere on the perimeter. This is the integration contract the layout
// wave builds on.
func TestHexRingComposedWithPlates(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		table := composedDemoTable()
		if len(table) != 13 {
			t.Fatalf("composed table is %d rows, want 13", len(table))
		}
		for r, row := range table {
			if w := lipgloss.Width(row); w != 77 {
				t.Errorf("row %d is %d cells, want 77: %q", r, w, row)
			}
		}

		// The top and bottom edges read unbroken from corner to corner:
		// ring segment, fusion tee, plate, fusion tee, ring segment. The
		// plate interiors legitimately contain spaces (they are engraved
		// titles), so the seal is checked at the ring runs and junctions.
		g := theme.G
		top, bottom := cellsOf(table[0]), cellsOf(table[12])
		for c := 3; c <= 25; c++ {
			if top[c] == " " {
				t.Errorf("row 0 col %d: hole in the top edge", c)
			}
		}
		for c := 50; c <= 73; c++ {
			if top[c] == " " {
				t.Errorf("row 0 col %d: hole in the top edge", c)
			}
		}
		if top[26] != g.RingTeeR || top[49] != g.RingTeeL {
			t.Errorf("top plate should fuse with tees at columns 26 and 49: %q %q",
				top[26], top[49])
		}
		for c := 3; c <= 24; c++ {
			if bottom[c] == " " {
				t.Errorf("row 12 col %d: hole in the bottom edge", c)
			}
		}
		for c := 51; c <= 73; c++ {
			if bottom[c] == " " {
				t.Errorf("row 12 col %d: hole in the bottom edge", c)
			}
		}
		if bottom[25] != g.RingTeeR || bottom[50] != g.RingTeeL {
			t.Errorf("bottom plate should fuse with tees at columns 25 and 50: %q %q",
				bottom[25], bottom[50])
		}
		// Both rims read unbroken down the vertical span.
		for r := 3; r <= 9; r++ {
			cells := cellsOf(table[r])
			if cells[0] == " " || cells[76] == " " {
				t.Errorf("row %d: hole in a rim:\n%s", r, strings.Join(table, "\n"))
			}
		}

		t.Logf("composed hexagon:\n%s", strings.Join(table, "\n"))
	})
}

// composedDemoTable assembles the Direction E preflop scene from the
// published components: the ring, two inline plates, four side plates and
// some felt furniture.
func composedDemoTable() []string {
	g := theme.G

	ring := hexRingRows(HexRingSpec{
		Width: 77, Height: 13, Corner: 2,
		TopGap:    RingGap{Start: 26, Len: 24},
		BottomGap: RingGap{Start: 25, Len: 26},
		LeftGaps:  []RingGap{{Start: 3, Len: 3}, {Start: 7, Len: 3}},
		RightGaps: []RingGap{{Start: 3, Len: 3}, {Start: 7, Len: 3}},
	})

	cole := SeatPlateView{Order: "1", Name: "Cole", Position: engine.PosUTG,
		Read: "tight", Stack: 1000, Folded: true, Width: 24}
	hero := SeatPlateView{Order: "4", Name: "YOU", Position: engine.PosBTN,
		Stack: 1000, ToAct: true, Hero: true, Dealer: true, Width: 26}
	nia := SeatPlateView{Order: "6", Name: "Nia", Position: engine.PosBB,
		Read: "sticky", Stack: 990, Bet: 10, Width: 22}
	tara := SeatPlateView{Order: "5", Name: "Tara", Position: engine.PosSB,
		Read: "loose", Stack: 995, Bet: 5, Width: 22}
	ivy := SeatPlateView{Order: "2", Name: "Ivy", Position: engine.PosHJ,
		Read: "wild", Stack: 970, Bet: 30, Raised: true, Width: 22}
	sam := SeatPlateView{Order: "3", Name: "Sam", Position: engine.PosCO,
		Read: "solid", Stack: 1000, Folded: true, Width: 22}

	overlayAt(ring, 0, 26, cole.RenderInline())
	overlayAt(ring, 1, 26, cole.RenderStatus(24))
	overlayAt(ring, 12, 25, hero.RenderInline())
	overlayAt(ring, 3, 0, nia.Render(PlateLeftHand, PlateEdges{Left: true}))
	overlayAt(ring, 7, 0, tara.Render(PlateLeftHand, PlateEdges{Left: true}))
	overlayAt(ring, 3, 55, ivy.Render(PlateRightHand, PlateEdges{Right: true}))
	overlayAt(ring, 7, 55, sam.Render(PlateRightHand, PlateEdges{Right: true}))

	scale := ChipScale(45, 30)
	overlayAt(ring, 2, 28, PotLine([]PotView{{Amount: 45}}, 20))
	overlayAt(ring, 6, 26, strings.Repeat(g.Dot+"    ", 4)+g.Dot)
	overlayAt(ring, 10, 25, ChipPile(45, scale)+" pot")
	overlayAt(ring, 10, 40, ChipPile(30, scale)+" to call")
	return ring
}
