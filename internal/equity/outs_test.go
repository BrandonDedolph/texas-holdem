package equity

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// TestOutsTextbookCounts pins the numbers every poker book quotes, against
// the default "top pair or better" reference range: flush draw 9, OESD 8,
// gutshot 4, combo draw 15. If the quiz and the coach are to agree, these
// must come out of the mechanical definition, not be special-cased.
func TestOutsTextbookCounts(t *testing.T) {
	cases := []struct {
		name  string
		hero  string
		board string
		clean int
		draw  DrawType
		nDraw int
	}{
		{"flush draw", "6h 5h", "Kh 9h 2c", 9, FlushDraw, 9},
		{"open-ended straight draw", "Jc Ts", "Qd 9s 2c", 8, OESD, 8},
		{"gutshot", "Jc Th", "Qd 8s 2c", 4, Gutshot, 4},
		{"combo draw", "Jh Th", "Qh 9h 2c", 15, FlushDraw, 9},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep := Outs(engine.Holes(tc.hero), engine.MustCards(tc.board), nil)
			if got := len(rep.Clean); got != tc.clean {
				t.Fatalf("Clean = %d (%s), want %d", got, engine.CardsString(rep.Clean), tc.clean)
			}
			if rep.Count != len(rep.Clean) {
				t.Fatalf("Count = %d, want len(Clean) = %d", rep.Count, len(rep.Clean))
			}
			if got := len(rep.ByDraw[tc.draw]); got != tc.nDraw {
				t.Fatalf("ByDraw[%v] = %d cards, want %d", tc.draw, got, tc.nDraw)
			}
		})
	}
}

// TestOutsComboDrawByDraw: the 15-out combo draw decomposes into a 9-card
// flush draw and an 8-card OESD (two cards belong to both).
func TestOutsComboDrawByDraw(t *testing.T) {
	rep := Outs(engine.Holes("Jh Th"), engine.MustCards("Qh 9h 2c"), nil)
	if got := len(rep.ByDraw[FlushDraw]); got != 9 {
		t.Fatalf("flush-draw cards = %d, want 9", got)
	}
	if got := len(rep.ByDraw[OESD]); got != 8 {
		t.Fatalf("OESD cards = %d, want 8", got)
	}
	if rep.Discounted != 15 {
		t.Fatalf("Discounted = %v, want 15", rep.Discounted)
	}
	if rep.RuleOf4 != 60 || rep.RuleOf2 != 30 {
		t.Fatalf("RuleOf4/RuleOf2 = %v/%v, want 60/30", rep.RuleOf4, rep.RuleOf2)
	}
}

// TestOutsTainted: against an explicit range holding a live flush draw, the
// card that completes the hero's straight AND the villain's flush is
// tainted, not clean — the discount the coach teaches.
func TestOutsTainted(t *testing.T) {
	vs := mustRange(t, "QsJs, KsQd, Ac5c")
	rep := Outs(engine.Holes("Jd Th"), engine.MustCards("Qc 9c 2s"), &vs)
	if got := len(rep.Clean); got != 6 {
		t.Fatalf("Clean = %d (%s), want 6", got, engine.CardsString(rep.Clean))
	}
	if got := len(rep.Tainted); got != 2 {
		t.Fatalf("Tainted = %d (%s), want 2 (the club K and club 8)", got, engine.CardsString(rep.Tainted))
	}
	for _, c := range rep.Tainted {
		if c.Suit() != engine.Clubs {
			t.Fatalf("tainted card %s is not a club", c.Code())
		}
	}
	if rep.Discounted != 7 { // 6 + 0.5×2
		t.Fatalf("Discounted = %v, want 7", rep.Discounted)
	}
}

// TestOutsWhenAhead: a hero already ahead of the range has no outs to
// count — outs are a catching-up concept.
func TestOutsWhenAhead(t *testing.T) {
	rep := Outs(engine.Holes("As Ad"), engine.MustCards("Kh 8c 2d"), nil)
	if len(rep.Clean) != 0 || len(rep.Tainted) != 0 || rep.Count != 0 {
		t.Fatalf("overpair ahead of top-pair range reported outs: %+v", rep)
	}
}

// TestDrawInventory: the ByDraw classification is hero-centric and works
// without any range comparison.
func TestDrawInventory(t *testing.T) {
	rep := Outs(engine.Holes("Ah Kd"), engine.MustCards("Qs 8h 3c"), nil)
	if got := len(rep.ByDraw[TwoOvercards]); got != 6 {
		t.Fatalf("two overcards = %d cards, want 6", got)
	}

	rep = Outs(engine.Holes("7s 7d"), engine.MustCards("Kh 8c 2d"), nil)
	if got := len(rep.ByDraw[PairToTrips]); got != 2 {
		t.Fatalf("pair to trips = %d cards, want 2", got)
	}

	rep = Outs(engine.Holes("Kd 9s"), engine.MustCards("Kh 8c 2d"), nil)
	if got := len(rep.ByDraw[PairToTwoPair]); got != 3 {
		t.Fatalf("pair to two pair = %d cards, want 3", got)
	}

	// Three to a flush on the flop is noted as a backdoor draw with no
	// next-card outs.
	rep = Outs(engine.Holes("Ah 5h"), engine.MustCards("Kh 8c 2d"), nil)
	if _, noted := rep.ByDraw[BackdoorFlush]; !noted {
		t.Fatal("backdoor flush draw not noted")
	}
	if len(rep.ByDraw[BackdoorFlush]) != 0 {
		t.Fatal("a backdoor draw has no next-card outs")
	}
}

// TestTopPairThreshold pins the HandRank layout that topPairThreshold
// depends on (eval/rank.go documents it; this test notices if it moves).
func TestTopPairThreshold(t *testing.T) {
	board := engine.MustCards("Kh 8c 2d")
	th := topPairThreshold(engine.King)
	if r := eval.EvalHoldem(engine.Holes("Kd 3c"), board); r < th {
		t.Fatal("top pair with the worst kicker should meet the threshold")
	}
	if r := eval.EvalHoldem(engine.Holes("8s 3s"), board); r >= th {
		t.Fatal("second pair should not meet the threshold")
	}
	if r := eval.EvalHoldem(engine.Holes("As Ad"), board); r < th {
		t.Fatal("an overpair should meet the threshold")
	}
	if r := eval.EvalHoldem(engine.Holes("8s 2s"), board); r < th {
		t.Fatal("two pair should meet the threshold")
	}
}
