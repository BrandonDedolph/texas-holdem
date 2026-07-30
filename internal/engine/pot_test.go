package engine

import (
	"reflect"
	"testing"
)

// TestBuildPotsWorkedExample is design-engine.md §3.3 as a literal test:
// blinds 1/2, A = 25 all-in, B = 80, C = 60 all-in, D = 2 folded.
func TestBuildPotsWorkedExample(t *testing.T) {
	var committed [MaxSeats]Chips
	committed[0] = 25 // A
	committed[1] = 80 // B
	committed[2] = 2  // D (folded big blind)
	committed[3] = 60 // C
	folded := NewSeatSet(2)
	live := NewSeatSet(0, 1, 3)

	pots, refund := BuildPots(committed, folded, live)

	wantRefund := [MaxSeats]Chips{1: 20} // B's uncalled 20 comes back
	if refund != wantRefund {
		t.Fatalf("refund = %v, want %v", refund, wantRefund)
	}
	want := []Pot{
		{Amount: 77, Eligible: NewSeatSet(0, 1, 3)}, // main: 25+25+25 + D's dead 2
		{Amount: 70, Eligible: NewSeatSet(1, 3)},    // side: 35+35
	}
	if !reflect.DeepEqual(pots, want) {
		t.Fatalf("pots = %+v, want %+v", pots, want)
	}
	// Chips conserve: 77 + 70 + 20 refund = 167 = 25+80+60+2.
	if pots[0].Amount+pots[1].Amount+refund[1] != 167 {
		t.Fatal("worked example does not conserve chips")
	}
}

// workedExampleHand plays §3.3 as a real hand: seat 0 = A (button, 25),
// seat 1 = B (SB, 80), seat 2 = D (BB, folds), seat 3 = C (UTG, 60).
func workedExampleHand(t *testing.T, eval Evaluator) *Hand {
	t.Helper()
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0,
		Stacks: map[Seat]Chips{0: 25, 1: 80, 2: 50, 3: 60},
		Holes: map[Seat][2]Card{
			0: Holes("2c 2d"), // A
			1: Holes("5c 5d"), // B
			3: Holes("Ac Ad"), // C
		},
		Seed: 33,
		Eval: eval,
	})
	mustApply(t, h, Raise{3, 60}) // C open-shoves
	mustApply(t, h, Call{0})      // A calls all-in for 25
	mustApply(t, h, Raise{1, 80}) // B: short all-in reraise (full would be 118)
	mustApply(t, h, Fold{2})      // D folds the big blind
	if h.Phase() != PhaseComplete {
		t.Fatalf("phase = %v, want complete via run-out", h.Phase())
	}
	return h
}

func TestWorkedExampleShowdownCase1(t *testing.T) {
	// Case 1: C has the best hand overall → C wins side pot 70 and main 77.
	h := workedExampleHand(t, holeEval(map[Card]uint32{
		MustCard("2c"): 10, MustCard("5c"): 50, MustCard("Ac"): 100,
	}))

	assertNet(t, h, map[Seat]Chips{0: -25, 1: -60, 2: -2, 3: 87}) // C: 147-60
	res, _ := h.Result()
	if len(res.Awards) != 2 {
		t.Fatalf("awards = %+v, want 2 pots", res.Awards)
	}
	// Awarded last side pot first.
	side, main := res.Awards[0], res.Awards[1]
	if side.Pot != 1 || side.Amount != 70 || !reflect.DeepEqual(side.Winners, []Seat{3}) {
		t.Fatalf("side pot award = %+v", side)
	}
	if main.Pot != 0 || main.Amount != 77 || !reflect.DeepEqual(main.Winners, []Seat{3}) {
		t.Fatalf("main pot award = %+v", main)
	}
	// B's uncalled 20 was refunded, never entering any pot.
	found := false
	for _, e := range h.Events() {
		if e.Kind == EvRefundUncalled {
			if e.Seat != 1 || e.Amount != 20 {
				t.Fatalf("refund event = %+v, want seat 1 amount 20", e)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("no EvRefundUncalled event for B's uncalled 20")
	}
}

func TestWorkedExampleShowdownCase2(t *testing.T) {
	// Case 2: A and C tie for best, B worst. Side pot 70 → C alone (A is not
	// eligible). Main 77 splits A/C: 38 each, odd chip to whichever of A, C
	// sits first clockwise from the button — seat 3 (C), since the walk from
	// seat 1 reaches 3 before wrapping to 0.
	h := workedExampleHand(t, holeEval(map[Card]uint32{
		MustCard("2c"): 100, MustCard("5c"): 50, MustCard("Ac"): 100,
	}))

	res, _ := h.Result()
	side, main := res.Awards[0], res.Awards[1]
	if !reflect.DeepEqual(side.Winners, []Seat{3}) || side.Amount != 70 {
		t.Fatalf("side pot award = %+v, want C alone", side)
	}
	if !reflect.DeepEqual(main.Winners, []Seat{3, 0}) ||
		!reflect.DeepEqual(main.Shares, []Chips{39, 38}) {
		t.Fatalf("main pot award = %+v, want C 39 / A 38", main)
	}
	// A: 38−25 = +13; C: (70+39)−60 = +49.
	assertNet(t, h, map[Seat]Chips{0: 13, 1: -60, 2: -2, 3: 49})
}

func TestBuildPotsUncontestedRefund(t *testing.T) {
	// A bet 50, B folded after committing 20: A's uncalled 30 returns and A
	// can only win what was matched.
	var committed [MaxSeats]Chips
	committed[0] = 50
	committed[1] = 20
	pots, refund := BuildPots(committed, NewSeatSet(1), NewSeatSet(0))
	if refund[0] != 30 {
		t.Fatalf("refund = %d, want 30", refund[0])
	}
	want := []Pot{{Amount: 40, Eligible: NewSeatSet(0)}}
	if !reflect.DeepEqual(pots, want) {
		t.Fatalf("pots = %+v, want %+v", pots, want)
	}
}

func TestBuildPotsRefundBenchmarkIncludesFoldedMoney(t *testing.T) {
	// A folded after committing 70; B is live with 80, C all-in 60. B's
	// uncalled excess is measured against A's 70 — not C's 60 — because A's
	// dead 70 stays in the pot.
	var committed [MaxSeats]Chips
	committed[0] = 70
	committed[1] = 80
	committed[2] = 60
	pots, refund := BuildPots(committed, NewSeatSet(0), NewSeatSet(1, 2))
	if refund[1] != 10 {
		t.Fatalf("refund = %d, want 10", refund[1])
	}
	want := []Pot{
		{Amount: 180, Eligible: NewSeatSet(1, 2)}, // 60×3 (A's first 60 is dead money)
		{Amount: 20, Eligible: NewSeatSet(1)},     // A 10 + B 10 above C's level
	}
	if !reflect.DeepEqual(pots, want) {
		t.Fatalf("pots = %+v, want %+v", pots, want)
	}
}

func TestBuildPotsMergesEqualEligibility(t *testing.T) {
	// Equal live contributions at multiple "levels" collapse to one pot.
	var committed [MaxSeats]Chips
	committed[0], committed[1], committed[2] = 10, 10, 10
	pots, _ := BuildPots(committed, 0, NewSeatSet(0, 1, 2))
	if len(pots) != 1 || pots[0].Amount != 30 {
		t.Fatalf("pots = %+v, want one pot of 30", pots)
	}
}

func TestOddChipGoesClockwiseFromButton(t *testing.T) {
	p := Pot{Amount: 77, Eligible: NewSeatSet(0, 3)}
	a := awardPot(0, p, NewSeatSet(0, 3), 42, 0)
	if !reflect.DeepEqual(a.Winners, []Seat{3, 0}) {
		t.Fatalf("winners order = %v, want [3 0] (clockwise from seat 1)", a.Winners)
	}
	if !reflect.DeepEqual(a.Shares, []Chips{39, 38}) {
		t.Fatalf("shares = %v, want [39 38]", a.Shares)
	}

	// Three-way split of 10 with button between the winners.
	a = awardPot(0, Pot{Amount: 10}, NewSeatSet(0, 1, 2), 7, 2)
	if !reflect.DeepEqual(a.Winners, []Seat{0, 1, 2}) {
		t.Fatalf("winners order = %v, want [0 1 2]", a.Winners)
	}
	if !reflect.DeepEqual(a.Shares, []Chips{4, 3, 3}) {
		t.Fatalf("shares = %v, want [4 3 3]", a.Shares)
	}
	if a.Rank != 7 || a.Amount != 10 {
		t.Fatalf("award metadata = %+v", a)
	}
}

func TestThreeWayAllInLayering(t *testing.T) {
	// Distinct all-in depths with no folds: strict layering, each seat only
	// eligible up to its own level.
	var committed [MaxSeats]Chips
	committed[1], committed[3], committed[5] = 10, 40, 100
	live := NewSeatSet(1, 3, 5)
	pots, refund := BuildPots(committed, 0, live)
	if refund[5] != 60 {
		t.Fatalf("refund = %d, want 60 (nobody covered seat 5)", refund[5])
	}
	want := []Pot{
		{Amount: 30, Eligible: NewSeatSet(1, 3, 5)},
		{Amount: 60, Eligible: NewSeatSet(3, 5)},
	}
	if !reflect.DeepEqual(pots, want) {
		t.Fatalf("pots = %+v, want %+v", pots, want)
	}
}
