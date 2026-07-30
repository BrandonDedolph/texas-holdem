package tutorial

import (
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
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
		"showdown": {
			Showdown: &VisualShowdown{
				Board: engine.MustCards("Qs Jh 7d 4c 2h"),
				Hands: []ShownHand{
					{Label: "Player A", Hole: engine.Holes("Ac Kd")},
					{Label: "Player B", Hole: engine.Holes("7h 8h")},
				},
			},
			Caption: "a labelled head-to-head",
		},
		"showdown-preflop": {
			Showdown: &VisualShowdown{
				Hands: []ShownHand{
					{Label: "You", Hole: engine.Holes("Kh Jc")},
					{Label: "The raiser", Hole: engine.Holes("Ad Qs")},
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

// TestShowdownLabelsEveryHand: the head-to-head visual names whose cards
// are whose — an unlabelled comparison teaches nothing — and does so in
// both the full and the compact form.
func TestShowdownLabelsEveryHand(t *testing.T) {
	vis := sampleVisuals(t)["showdown"]
	for name, out := range map[string]string{
		"full":    vis.Render(80),
		"compact": vis.RenderCompact(60),
	} {
		for _, label := range []string{"Player A", "Player B"} {
			if !strings.Contains(out, label) {
				t.Errorf("%s: showdown missing label %q:\n%s", name, label, out)
			}
		}
	}
	// A preflop matchup (no board) still renders both labelled hands.
	pre := sampleVisuals(t)["showdown-preflop"]
	out := pre.Render(80)
	if !strings.Contains(out, "You") || !strings.Contains(out, "The raiser") {
		t.Errorf("preflop showdown missing labels:\n%s", out)
	}
	if strings.Contains(out, "Board") {
		t.Errorf("preflop showdown should not draw a board:\n%s", out)
	}
}

// TestHandLadderDrawsCards: every rung of the ladder draws its five example
// cards as mini cards — ten tiers, fifty card frames, each tier's name on
// its middle row. The ladder was once an inline list of codes; this test is
// the guard against it regressing to one. (The full-size study card's own
// geometry tests live with it in internal/ui/components.)
func TestHandLadderDrawsCards(t *testing.T) {
	out := (&VisualHandLadder{}).Render(80)
	plain := stripANSI(out)
	rungs := LadderRungs()
	if got, want := strings.Count(plain, theme.G.CardTL), 5*len(rungs); got != want {
		t.Errorf("ladder draws %d card frames, want %d (5 per rung)", got, want)
	}
	lines := strings.Split(plain, "\n")
	if got, want := len(lines), 3*len(rungs); got != want {
		t.Errorf("ladder is %d rows, want %d (3 per rung)", got, want)
	}
	for i, r := range rungs {
		mid := lines[3*i+1]
		if !strings.Contains(mid, r.Name) {
			t.Errorf("rung %d middle row %q missing tier name %q", i+1, mid, r.Name)
		}
		wantNum := strconv.Itoa(i+1) + "."
		if !strings.Contains(mid, wantNum) {
			t.Errorf("rung %d middle row %q missing its number %q", i+1, mid, wantNum)
		}
		// The example hand appears as drawn card faces, not inline codes: the
		// suit rides its own glyph inside a frame on the same three rows.
		for _, c := range engine.MustCards(r.Cards) {
			face := c.Rank().Symbol() + theme.SuitGlyph(c.Suit())
			band := strings.Join(lines[3*i:3*i+3], "\n")
			if !strings.Contains(band, face) {
				t.Errorf("rung %d band missing drawn face %q", i+1, face)
			}
		}
	}
}

// TestColorizeCards: card codes render through the inline card (suit glyph,
// same cell width), range-spec combos split into their two cards, and
// hand-class notation is left alone.
func TestColorizeCards(t *testing.T) {
	base := theme.Current.Body
	spade := theme.G.SuitSpade
	club := theme.G.SuitClub

	out := stripANSI(ColorizeCards("the As completes it", base))
	if want := "the A" + spade + " completes it"; out != want {
		t.Errorf("As: got %q, want %q", out, want)
	}
	if got := stripANSI(ColorizeCards("holding 10c here", base)); got != "holding 10"+club+" here" {
		t.Errorf("10c: got %q", got)
	}
	// A range-spec combo is two concatenated cards.
	if got := stripANSI(ColorizeCards("range: 9c8c only", base)); got != "range: 9"+club+"8"+club+" only" {
		t.Errorf("combo: got %q", got)
	}
	// Hand-class notation must not be mistaken for cards.
	for _, s := range []string{"open A5s here", "fold K9s+ and 108s+", "72o never plays"} {
		if got := stripANSI(ColorizeCards(s, base)); got != s {
			t.Errorf("notation %q changed to %q", s, got)
		}
	}
	// Width is preserved: wrapping computed on the plain text stays valid.
	for _, s := range []string{"As Kd 10h", "the 4c completes his flush"} {
		if got, want := lipgloss.Width(ColorizeCards(s, base)), lipgloss.Width(s); got != want {
			t.Errorf("%q: colorized width %d, plain %d", s, got, want)
		}
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
