package components

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// seatCases is the fixture set for the layout invariants: every state a
// seat block can be in, including hostile inputs (empty name, absurdly long
// name and action label).
func seatCases() map[string]SeatView {
	longAction := strings.Repeat("raises to 30 after tanking forever ", 10)
	return map[string]SeatView{
		"waiting": {
			Name: "Nia", Position: engine.PosHJ, Stack: 830, Width: 20,
		},
		"bet": {
			Name: "Nia", Position: engine.PosHJ, Stack: 830, Bet: 30,
			LastAction: "raises to 30", Width: 20,
		},
		"folded": {
			Name: "Cole", Position: engine.PosCO, Stack: 615, Folded: true,
			LastAction: "folds", Width: 20,
		},
		"dealer": {
			Name: "Ivy", Position: engine.PosBTN, Stack: 990, Dealer: true,
			Width: 20,
		},
		"to-act-hero": {
			Name: "YOU", Position: engine.PosBB, Stack: 900, ToAct: true,
			Hero: true, Width: 20,
		},
		"all-in": {
			Name: "Sam", Position: engine.PosSB, Stack: 0, AllIn: true,
			Bet: 285, LastAction: "all-in 285", Width: 20,
		},
		"showdown": {
			Name: "Nia", Position: engine.PosHJ, Stack: 740, ShowCards: true,
			Hole: engine.Holes("Ah Kh"), Width: 20,
		},
		"big-stack": {
			Name: "Sam", Position: engine.PosSB, Stack: 2395, Bet: 1000000,
			Width: 20,
		},
		"long-name": {
			Name: "Bartholomew Kuznetsov III", Position: engine.PosUTG,
			Stack: 1020, Width: 20,
		},
		"long-action": {
			Name: "Nia", Position: engine.PosHJ, Stack: 830,
			LastAction: longAction, Width: 20,
		},
		"empty-name": {
			Position: engine.PosUTG, Stack: 5, Width: 20,
		},
		"narrow": {
			Name: "Nia", Position: engine.PosBTN, Stack: 830, Dealer: true,
			Bet: 30, LastAction: "raises to 30", Width: 8,
		},
	}
}

// TestSeatRenderFixedBlock: three rows, every row exactly the slot width,
// for short and absurdly long content alike - truncate, never wrap.
func TestSeatRenderFixedBlock(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		for name, v := range seatCases() {
			t.Run(name, func(t *testing.T) {
				assertBlock(t, v.Render(), v.Width, 3)
			})
		}
	})
}

func TestSeatRenderCompactOneRow(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		for name, v := range seatCases() {
			t.Run(name, func(t *testing.T) {
				v.Width = 40
				assertBlock(t, v.RenderCompact(), v.Width, 1)
			})
		}
	})
}

func TestSeatRenderDefaultWidth(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		v := SeatView{Name: "Nia", Position: engine.PosHJ, Stack: 830}
		assertBlock(t, v.Render(), DefaultSeatWidth, 3)
	})
}

func TestSeatRenderContent(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		cases := seatCases()

		if got := cases["all-in"].Render(); !strings.Contains(got, "ALL-IN") {
			t.Errorf("all-in seat should read ALL-IN in place of the stack:\n%s", got)
		}
		if got := cases["waiting"].Render(); !strings.Contains(got, "830") {
			t.Errorf("stack must stay visible at all times:\n%s", got)
		}
		if got := cases["folded"].Render(); !strings.Contains(got, "615") {
			t.Errorf("folded seats keep their stack visible:\n%s", got)
		}
	})
}

func TestSeatRenderNeverTruncatesStack(t *testing.T) {
	// The name gives way before the stack does: reading stacks at all
	// times is a habit the layout is designed to teach.
	glyphSets(t, func(t *testing.T) {
		v := SeatView{
			Name:     "Bartholomew Kuznetsov III",
			Position: engine.PosUTG,
			Stack:    1020,
			Width:    20,
		}
		header := strings.Split(v.Render(), "\n")[0]
		if !strings.Contains(header, "1,020") {
			t.Errorf("long name truncated the stack out of row 1: %q", header)
		}
	})
}
