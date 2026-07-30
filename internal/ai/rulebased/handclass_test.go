package rulebased

import (
	"math/rand/v2"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

func classify(t *testing.T, hole, board string) Classification {
	t.Helper()
	return Classify(engine.Holes(hole), engine.MustCards(board))
}

func TestMadeClass(t *testing.T) {
	cases := []struct {
		hole, board string
		want        ai.HandClass
	}{
		{"As Kd", "Qh 7s 2c", ai.Air},
		{"As Kd", "Ah 7s 2c", ai.TopPair},
		{"Ks Qd", "Ah Ks 2c", ai.MiddlePair},
		{"7s 6d", "Ah Ks 7c", ai.WeakPair},
		{"Qs Qd", "Jh 7s 2c", ai.Overpair},
		{"5s 5d", "Ah Ks 2c", ai.WeakPair},   // underpair
		{"9s 9d", "Ah 7s 2c", ai.MiddlePair}, // between top and second
		{"As 7d", "Ah 7s 2c", ai.TwoPair},
		{"As Kd", "Ah Kc 2c", ai.TwoPair},
		{"7s 7d", "Ah 7c 2c", ai.TripsPlus},
		{"As Kd", "Qh Js Tc", ai.TripsPlus}, // straight
		{"As Kd", "7h 7s 2c", ai.Air},       // board pair only
		{"As Kd", "7h 7s 2c 2d", ai.Air},    // board two pair only
		{"Qs Qd", "7h 7s 2c", ai.Overpair},  // pocket pair over a paired board
	}
	for _, c := range cases {
		got := classify(t, c.hole, c.board).Made
		if got != c.want {
			t.Errorf("Classify(%s | %s).Made = %v, want %v", c.hole, c.board, got, c.want)
		}
	}
}

func TestDrawClass(t *testing.T) {
	cases := []struct {
		hole, board string
		want        ai.DrawClass
	}{
		{"As Ks", "Qs 7s 2h", ai.FlushDraw},
		{"9s 8s", "Ks 7s 6h", ai.ComboDraw},
		{"9d 8d", "Kh 7s 6h", ai.OESD},
		{"9d 8d", "Kh 6s 5h", ai.Gutshot}, // needs exactly a 7
		{"As Kd", "Qh 7s 2c", ai.NoDraw},
		{"As Ks", "Qh Js Tc", ai.NoDraw}, // already a straight
		{"Ad Kd", "Qh 7s 2c 3d", ai.NoDraw},
	}
	for _, c := range cases {
		got := classify(t, c.hole, c.board).Draw
		if got != c.want {
			t.Errorf("Classify(%s | %s).Draw = %v, want %v", c.hole, c.board, got, c.want)
		}
	}
}

// TestDrawClassMatchesOuts pins the agreement between the cheap per-combo
// draw classifier and equity.Outs' draw inventory over random flops: the
// coach's vocabulary and the strategy's must be the same vocabulary
// (the quiz and the bot may never disagree about what a flush draw is).
func TestDrawClassMatchesOuts(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 7))
	for trial := 0; trial < 150; trial++ {
		perm := rng.Perm(engine.NumCards)
		hole := [2]engine.Card{engine.Card(perm[0]), engine.Card(perm[1])}
		board := []engine.Card{engine.Card(perm[2]), engine.Card(perm[3]), engine.Card(perm[4])}

		if eval.EvalHoldem(hole, board).Category() >= eval.Straight {
			continue // drawClass reports NoDraw for made straights+; Outs still inventories
		}
		got := Classify(hole, board).Draw
		report := equity.Outs(hole, board, nil)
		_, flush := report.ByDraw[equity.FlushDraw]
		_, oesd := report.ByDraw[equity.OESD]
		_, gut := report.ByDraw[equity.Gutshot]

		var want ai.DrawClass
		switch {
		case flush && (oesd || gut):
			want = ai.ComboDraw
		case flush:
			want = ai.FlushDraw
		case oesd:
			want = ai.OESD
		case gut:
			want = ai.Gutshot
		default:
			want = ai.NoDraw
		}
		if got != want {
			t.Errorf("%v%v | %v: drawClass %v, equity.Outs implies %v",
				hole[0], hole[1], engine.CardsString(board), got, want)
		}
	}
}

// TestRankKickerLayout pins the HandRank bit layout rankKicker depends on.
func TestRankKickerLayout(t *testing.T) {
	r := eval.EvalHoldem(engine.Holes("As Ad"), engine.MustCards("Ks 7c 2h"))
	if got := rankKicker(r, 0); got != engine.Ace {
		t.Fatalf("k1 of a pair of aces = %v, want Ace — HandRank layout changed", got)
	}
}

func TestBoardDry(t *testing.T) {
	dry := [][]string{{"Kh 7s 2c"}, {"Ah 8s 3d"}}
	wet := [][]string{{"Kh 7h 2h"}, {"Qh Js 9c"}, {"7h 7s 2c"}, {"9h 8s 2c"}, {"Ah Ks 2c"}}
	for _, b := range dry {
		if !boardDry(engine.MustCards(b[0])) {
			t.Errorf("board %s should be dry", b[0])
		}
	}
	for _, b := range wet {
		if boardDry(engine.MustCards(b[0])) {
			t.Errorf("board %s should be wet", b[0])
		}
	}
}
