package components

import (
	"regexp"
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stripANSI removes SGR escape sequences so tests can inspect the plain
// cell grid of a styled render.
func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

// plateCases is the fixture set for the layout invariants: every state a
// seat plate can be in, including hostile inputs.
func plateCases() map[string]SeatPlateView {
	return map[string]SeatPlateView{
		"waiting": {
			Order: "5", Name: "Tara", Position: engine.PosSB, Read: "loose",
			Stack: 995, Width: 22,
		},
		"bet": {
			Order: "6", Name: "Nia", Position: engine.PosBB, Read: "sticky",
			Stack: 990, Bet: 10, Width: 22,
		},
		"raised": {
			Order: "2", Name: "Ivy", Position: engine.PosHJ, Read: "wild",
			Stack: 970, Bet: 30, Raised: true, Width: 22,
		},
		"folded": {
			Order: "3", Name: "Sam", Position: engine.PosCO, Read: "solid",
			Stack: 1000, Folded: true, Width: 22,
		},
		"to-act-hero": {
			Order: "4", Name: "YOU", Position: engine.PosBTN, Stack: 1000,
			ToAct: true, Hero: true, Dealer: true, Width: 22,
		},
		"all-in": {
			Order: "1", Name: "Cole", Position: engine.PosUTG, Read: "tight",
			Stack: 0, AllIn: true, Bet: 700, Width: 22,
		},
		"no-order": {
			Name: "Nia", Position: engine.PosBB, Read: "sticky",
			Stack: 990, Width: 22,
		},
		"long-name": {
			Order: "2", Name: "Bartholomew Kuznetsov III",
			Position: engine.PosHJ, Read: "wild", Stack: 970, Width: 22,
		},
		"long-read": {
			Order: "2", Name: "Ivy", Position: engine.PosHJ,
			Read: strings.Repeat("extremely-wild ", 5), Stack: 970, Width: 22,
		},
		"big-stack": {
			Order: "2", Name: "Ivy", Position: engine.PosHJ, Read: "wild",
			Stack: 1000000, Bet: 500000, Raised: true, Width: 22,
		},
		"narrow": {
			Order: "2", Name: "Ivy", Position: engine.PosHJ, Read: "wild",
			Stack: 970, Bet: 30, Raised: true, Width: 10,
		},
	}
}

// plateEdgeCases are the border configurations the ring geometry uses,
// plus the fully closed and fully open extremes.
func plateEdgeCases() map[string]PlateEdges {
	return map[string]PlateEdges{
		"closed":     {},
		"open-left":  {Left: true},
		"open-right": {Right: true},
		"open-top":   {Top: true},
		"open-bot":   {Bottom: true},
		"open-all":   {Top: true, Bottom: true, Left: true, Right: true},
	}
}

// TestSeatPlateFixedBlock: three rows, every row exactly the slot width,
// for every state, both hands and every border configuration - truncate,
// never wrap.
func TestSeatPlateFixedBlock(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		for name, v := range plateCases() {
			for _, hand := range []PlateHand{PlateLeftHand, PlateRightHand} {
				for edgeName, edges := range plateEdgeCases() {
					t.Run(name+"/"+edgeName, func(t *testing.T) {
						assertBlock(t, v.Render(hand, edges), v.Width, 3)
					})
				}
			}
		}
	})
}

func TestSeatPlateInlineOneRow(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		for name, v := range plateCases() {
			t.Run(name, func(t *testing.T) {
				assertBlock(t, v.RenderInline(), v.Width, 1)
			})
		}
	})
}

func TestSeatPlateStatusRowFixedWidth(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		for name, v := range plateCases() {
			t.Run(name, func(t *testing.T) {
				got := v.RenderStatus(30)
				if w := lipgloss.Width(got); w != 30 {
					t.Errorf("status row is %d cells, want 30: %q", w, got)
				}
				if strings.Contains(got, "\n") {
					t.Errorf("status row must never wrap: %q", got)
				}
			})
		}
	})
}

func TestSeatPlateDefaultWidth(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		v := SeatPlateView{Order: "1", Name: "Nia", Position: engine.PosBB, Stack: 990}
		assertBlock(t, v.Render(PlateLeftHand, PlateEdges{}), DefaultPlateWidth, 3)
	})
}

// TestSeatPlateFusionJunctions pins the fusion chars: an open edge renders
// tee junctions where a closed edge renders rounded corners, so the plate
// welds seamlessly into a ring line running along that edge.
func TestSeatPlateFusionJunctions(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		g := theme.G
		v := plateCases()["bet"]

		rows := strings.Split(stripANSI(v.Render(PlateLeftHand, PlateEdges{Left: true})), "\n")
		for _, r := range []int{0, 2} {
			if !strings.HasPrefix(rows[r], g.RingTeeL) {
				t.Errorf("open-left row %d should start with %q: %q", r, g.RingTeeL, rows[r])
			}
		}
		if !strings.HasPrefix(rows[1], g.RingV) {
			t.Errorf("open-left row 1 should start with the ring vertical: %q", rows[1])
		}

		rows = strings.Split(stripANSI(v.Render(PlateRightHand, PlateEdges{Right: true})), "\n")
		for _, r := range []int{0, 2} {
			if !strings.HasSuffix(rows[r], g.RingTeeR) {
				t.Errorf("open-right row %d should end with %q: %q", r, g.RingTeeR, rows[r])
			}
		}

		rows = strings.Split(stripANSI(v.Render(PlateLeftHand, PlateEdges{})), "\n")
		if !strings.HasPrefix(rows[0], g.RingTL) || !strings.HasSuffix(rows[0], g.RingTR) {
			t.Errorf("closed plate should carry rounded top corners: %q", rows[0])
		}
		if !strings.HasPrefix(rows[2], g.RingBL) || !strings.HasSuffix(rows[2], g.RingBR) {
			t.Errorf("closed plate should carry rounded bottom corners: %q", rows[2])
		}

		// An open top renders the inset-into-an-edge form with outward tees.
		rows = strings.Split(stripANSI(v.Render(PlateLeftHand, PlateEdges{Top: true})), "\n")
		if !strings.HasPrefix(rows[0], g.RingTeeR) || !strings.HasSuffix(rows[0], g.RingTeeL) {
			t.Errorf("open-top row 0 should be the inset form %q...%q: %q",
				g.RingTeeR, g.RingTeeL, rows[0])
		}
	})
}

func TestSeatPlateContent(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		g := theme.G
		cases := plateCases()

		got := stripANSI(cases["folded"].Render(PlateLeftHand, PlateEdges{Left: true}))
		if strings.Contains(got, "3 Sam") {
			t.Errorf("folded plate must replace the order digit with the fold mark:\n%s", got)
		}
		if !strings.Contains(got, g.FoldMark+" Sam") {
			t.Errorf("folded plate title should carry the fold mark:\n%s", got)
		}
		if !strings.Contains(got, "folded") {
			t.Errorf("folded plate should read folded:\n%s", got)
		}
		if !strings.Contains(got, "1,000") {
			t.Errorf("folded seats keep their stack visible:\n%s", got)
		}

		got = stripANSI(cases["all-in"].Render(PlateRightHand, PlateEdges{Right: true}))
		if !strings.Contains(got, "ALL-IN") {
			t.Errorf("all-in plate should read ALL-IN in place of the stack:\n%s", got)
		}

		got = stripANSI(cases["raised"].Render(PlateRightHand, PlateEdges{Right: true}))
		if !strings.Contains(got, g.RaiseMark) {
			t.Errorf("raised bet should carry the raise marker:\n%s", got)
		}
		if !strings.Contains(got, g.Chip+" 30") {
			t.Errorf("bet should render as chips on the felt:\n%s", got)
		}

		got = stripANSI(cases["to-act-hero"].RenderInline())
		if !strings.Contains(got, strings.TrimSpace(g.ToAct)) {
			t.Errorf("to-act plate should carry the turn marker:\n%s", got)
		}
		if !strings.Contains(got, "1,000") {
			t.Errorf("hero inline plate should carry the stack on the rim:\n%s", got)
		}

		got = stripANSI(cases["waiting"].RenderStatus(30))
		if !strings.Contains(got, "995") {
			t.Errorf("status row should carry the stack:\n%s", got)
		}
	})
}

// TestSeatPlateNeverTruncatesStack: the name gives way before the stack
// does - reading stacks at all times is a habit the layout teaches.
func TestSeatPlateNeverTruncatesStack(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		v := plateCases()["long-name"]
		body := strings.Split(v.Render(PlateLeftHand, PlateEdges{Left: true}), "\n")[1]
		if !strings.Contains(body, "970") {
			t.Errorf("long name must not push the stack out of the body row: %q", body)
		}
		title := strings.Split(stripANSI(v.Render(PlateLeftHand, PlateEdges{Left: true})), "\n")[0]
		if !strings.Contains(title, theme.G.Ellipsis) {
			t.Errorf("long name should truncate with an ellipsis in the title: %q", title)
		}
	})
}
