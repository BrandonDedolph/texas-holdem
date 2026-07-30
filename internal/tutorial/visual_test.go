package tutorial

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// sampleVisuals covers every visual type once.
func sampleVisuals(t *testing.T) map[string]*Visual {
	t.Helper()
	rng, err := equity.ParseRange("22+, A2s+, KTs+, AJo+")
	if err != nil {
		t.Fatal(err)
	}
	return map[string]*Visual{
		"board": {
			Board: &VisualBoard{
				Board:    engine.MustCards("Ah 9d 6c 3s 2d"),
				Hole:     engine.Holes("Ac Kd"),
				ShowHole: true,
				ShowBest: true,
			},
			Caption: "a board with the best five annotated",
		},
		"range-grid": {
			RangeGrid: &VisualRangeGrid{Range: rng, Title: "A sample range"},
		},
		"table-seats": {
			TableSeats: &VisualTableSeats{
				Highlight: []engine.Position{engine.PosBTN},
				Notes: map[engine.Position]string{
					engine.PosUTG: "first to act preflop",
					engine.PosBTN: "best seat",
				},
			},
		},
		"hand-ladder": {
			HandLadder: &VisualHandLadder{Highlight: 1},
		},
		"pot-odds": {
			PotOdds: &VisualPotOdds{Pot: 45, ToCall: 20},
		},
	}
}

// TestVisualsRenderWithin80Cols asserts every visual type produces non-empty
// output that fits the app's 80-column floor.
func TestVisualsRenderWithin80Cols(t *testing.T) {
	for name, v := range sampleVisuals(t) {
		t.Run(name, func(t *testing.T) {
			out := v.Render(80)
			if strings.TrimSpace(out) == "" {
				t.Fatal("render is empty")
			}
			for i, line := range strings.Split(out, "\n") {
				if w := lipgloss.Width(line); w > 80 {
					t.Errorf("line %d is %d cells wide: %q", i, w, line)
				}
			}
		})
	}
}

func TestEmptyVisualRendersNothing(t *testing.T) {
	if out := (&Visual{}).Render(80); out != "" {
		t.Errorf("empty visual rendered %q", out)
	}
}

// TestLadderExamplesMatchTheirTiers verifies every hand-ladder example
// actually evaluates to the tier it illustrates — the ladder is lesson 1's
// backbone, and a misfiled example would teach a wrong ranking.
func TestLadderExamplesMatchTheirTiers(t *testing.T) {
	rungs := LadderRungs()
	if len(rungs) != 10 {
		t.Fatalf("ladder has %d rungs, want 10", len(rungs))
	}
	var prev eval.HandRank
	for i, r := range rungs {
		cards := engine.MustCards(r.Cards)
		if len(cards) != 5 {
			t.Fatalf("rung %q has %d cards", r.Name, len(cards))
		}
		rank := eval.Eval5([5]engine.Card{cards[0], cards[1], cards[2], cards[3], cards[4]})
		if rank.Category() != r.Category {
			t.Errorf("rung %q evaluates to %s, want %s", r.Name, rank.Category(), r.Category)
		}
		if i > 0 && rank >= prev {
			t.Errorf("rung %q does not rank below %q", r.Name, rungs[i-1].Name)
		}
		prev = rank
	}
	// The top rung must be the ace-high straight flush by name.
	first := engine.MustCards(rungs[0].Cards)
	if s := eval.Eval5([5]engine.Card{first[0], first[1], first[2], first[3], first[4]}).String(); s != "Royal Flush" {
		t.Errorf("top rung describes as %q, want Royal Flush", s)
	}
}

// TestRangeGridCellLabels pins the chart layout: pairs on the diagonal,
// suited above, offsuit below.
func TestRangeGridCellLabels(t *testing.T) {
	cases := []struct {
		row, col int
		want     string
	}{
		{0, 0, "AA"},
		{0, 1, "AKs"},
		{1, 0, "AKo"},
		{12, 12, "22"},
		{0, 12, "A2s"},
		{12, 0, "A2o"},
		// The ten's cells carry the wide "10" labels — the owner's decision.
		{4, 4, "1010"},
		{0, 4, "A10s"},
		{4, 0, "A10o"},
		{4, 5, "109s"},
		{5, 4, "109o"},
	}
	for _, tc := range cases {
		if got := cellLabel(tc.row, tc.col); got != tc.want {
			t.Errorf("cellLabel(%d, %d) = %q, want %q", tc.row, tc.col, got, tc.want)
		}
	}
}
