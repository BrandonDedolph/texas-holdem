package coach

import (
	"math"
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// TestSetFlopFigure re-derives "flops a set about 1 in 8.5" by brute force:
// enumerate every C(50,3) flop for a pocket pair and count those containing
// one of the two remaining cards of the pair rank. Only then is the quoted
// string checked, so the constant and the enumeration cannot drift apart.
func TestSetFlopFigure(t *testing.T) {
	hole := engine.Holes("7h 7c")
	dead := engine.NewCardSet(hole[0], hole[1])
	var deck []engine.Card
	for c := engine.Card(0); c < engine.NumCards; c++ {
		if !dead.Has(c) {
			deck = append(deck, c)
		}
	}
	hits, total := 0, 0
	for i := 0; i < len(deck); i++ {
		for j := i + 1; j < len(deck); j++ {
			for k := j + 1; k < len(deck); k++ {
				total++
				if deck[i].Rank() == engine.Seven || deck[j].Rank() == engine.Seven ||
					deck[k].Rank() == engine.Seven {
					hits++
				}
			}
		}
	}
	if total != 19600 {
		t.Fatalf("flop count = %d, want C(50,3) = 19600", total)
	}
	if hits != 2304 { // 19600 − C(48,3): flops containing a seven
		t.Fatalf("set flops = %d, want 2304", hits)
	}
	p := float64(hits) / float64(total)
	if oneIn := 1 / p; math.Abs(oneIn-8.5) > 0.05 {
		t.Fatalf("set odds 1 in %.3f — the quoted 8.5 would be false", oneIn)
	}

	line := PotentialLine(hole, nil)
	if !strings.Contains(line, "flops a set about 1 in 8.5") {
		t.Errorf("77 line %q does not quote the derived set odds", line)
	}
}

// TestPreflopPotentialLines pins the full preflop wording per hand shape.
// Figures, all over the C(50,3) = 19,600 flops:
//
//	77 overcard:  1 − C(22,3)/C(50,3) = 1 − 1540/19600  = 92.1%
//	AA overpair:  C(48,3)/C(50,3)     = 17296/19600     = 88.2%
//	QQ overpair:  C(40,3)/C(50,3)     = 9880/19600      = 50.4%
//	flush draw:   C(11,2)×39/C(50,3)  = 2145/19600      = 10.9% ≈ 1 in 9
//	pairs up:     1 − C(44,3)/C(50,3) = 6356/19600      = 32.4% ≈ 1 in 3
//	87 open-ender: enumerated (TestOESDFlopProb), ≈ 10.2% ≈ 1 in 10
func TestPreflopPotentialLines(t *testing.T) {
	cases := []struct{ hole, want string }{
		{"7h 7c", "flops a set about 1 in 8.5 — unimproved, it is often second best: an overcard flops 92% of the time"},
		{"Ah Ac", "flops a set about 1 in 8.5 — unimproved, it is still an overpair on 88% of flops"},
		{"Qh Qc", "flops a set about 1 in 8.5 — unimproved, it is still an overpair on 50% of flops"},
		{"8s 7s", "flops a flush draw about 1 in 9 and an open-ended straight draw about 1 in 10"},
		{"As Ks", "flops a flush draw about 1 in 9 — and a pair about 1 in 3 flops"},
		{"Th 9c", "flops an open-ended straight draw about 1 in 10 and a pair about 1 in 3 flops"},
		{"Ah Qc", "hits a pair about 1 in 3 flops — big offsuit hands play one-pair pots, where the kicker decides"},
		{"7h 2c", "hits a pair about 1 in 3 flops — and not much else"},
	}
	for _, c := range cases {
		if got := PotentialLine(engine.Holes(c.hole), nil); got != c.want {
			t.Errorf("PotentialLine(%s) =\n  %q\nwant\n  %q", c.hole, got, c.want)
		}
	}
}

// TestOESDFlopProb sanity-bounds the enumerated open-ender figure for the
// max-stretch connector: the folklore range is "about 1 in 10", and the
// mechanical definition (two or more completing ranks, double-gutters
// included, flopped straights excluded) must land near it. It also pins
// suit-independence: 87s and 87o flop the same straight draws.
func TestOESDFlopProb(t *testing.T) {
	p := oesdFlopProb(engine.Eight, engine.Seven)
	if p < 0.08 || p > 0.12 {
		t.Errorf("87 open-ender flop probability = %.4f, want ≈ 0.10", p)
	}
	// AK cannot flop an open-ender (only QJT-gutshots and made straights).
	if p := oesdFlopProb(engine.Ace, engine.King); p != 0 {
		t.Errorf("AK open-ender flop probability = %.4f, want 0", p)
	}
}

// TestPostflopPotentialLines pins the made-hand-plus-draw wording. The out
// counts are the live cards equity.Outs inventories: four nines for 77's
// gutshot on 8JT; nine spades for the flush draw; 15 distinct cards for
// the turned flush draw + open-ender combo (9 + 8 − 2 shared).
func TestPostflopPotentialLines(t *testing.T) {
	cases := []struct{ hole, board, want string }{
		{"7h 7c", "8h Jc Ts",
			"pair of sevens with a gutshot — a gutshot is 4 outs"},
		{"9s 8s", "Ks 7s 2h",
			"high card, king with a flush draw — a flush draw is 9 outs"},
		{"9s 8s", "Ks 7s 2h 6d",
			"high card, king with a flush draw plus an open-ended straight draw — 15 outs between them"},
		{"Ah Kc", "Qs 8d 3c",
			"high card, ace — pairing either card makes top pair (6 outs)"},
		{"Ah Qc", "Qs 8d 3c",
			"pair of queens — pairing your kicker makes two pair (3 outs)"},
		{"7h 7c", "2h Jc Ts",
			"pair of sevens — the 2 remaining sevens make a set"},
		{"Ah 4h", "Kh 8d 3c",
			"high card, ace — only a backdoor flush draw, needing both remaining cards"},
		{"Ah Ac", "Ad Jc Ts Td 4d",
			"full house, aces full of tens — the board is complete, no more cards to come"},
	}
	for _, c := range cases {
		got := PotentialLine(engine.Holes(c.hole), engine.MustCards(c.board))
		if got != c.want {
			t.Errorf("PotentialLine(%s | %s) =\n  %q\nwant\n  %q", c.hole, c.board, got, c.want)
		}
	}
}

// TestPotentialLineRejectsMalformedBoards pins the equity-package stance:
// a two-card board is never a game state.
func TestPotentialLineRejectsMalformedBoards(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("PotentialLine accepted a 2-card board")
		}
	}()
	PotentialLine(engine.Holes("7h 7c"), engine.MustCards("8h Jc"))
}
