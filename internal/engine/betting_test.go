package engine

import (
	"math/rand/v2"
	"reflect"
	"testing"
)

func assertOptions(t *testing.T, h *Hand, want ActionOptions) {
	t.Helper()
	got := h.LegalActions()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("seat %d LegalActions = %+v, want %+v", h.CurrentSeat(), got, want)
	}
}

func TestMinRaiseLadder(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0,
		Stacks: map[Seat]Chips{0: 1000, 1: 1000, 2: 1000},
		Seed:   1,
	})

	// Button opens: min raise-to is BB + BB = 4 (the blind is the first
	// "full raise" the arithmetic sees).
	assertOptions(t, h, ActionOptions{
		{Type: ActionFold},
		{Type: ActionCall, Min: 2, Max: 2},
		{Type: ActionRaise, Min: 4, Max: 1000},
	})
	mustApply(t, h, Raise{0, 7}) // full raise, size 5

	// SB: min re-raise is 7 + 5 = 12; all-in ceiling is stack + posted 1.
	assertOptions(t, h, ActionOptions{
		{Type: ActionFold},
		{Type: ActionCall, Min: 6, Max: 6},
		{Type: ActionRaise, Min: 12, Max: 1000},
	})
	mustApply(t, h, Raise{1, 12}) // exactly the minimum, size 5

	// BB: min is 12 + 5 = 17.
	assertOptions(t, h, ActionOptions{
		{Type: ActionFold},
		{Type: ActionCall, Min: 10, Max: 10},
		{Type: ActionRaise, Min: 17, Max: 1000},
	})
	mustApply(t, h, Raise{2, 30}) // over-raise, size 18

	// Button again: a full raise reopened action; min is 30 + 18 = 48.
	assertOptions(t, h, ActionOptions{
		{Type: ActionFold},
		{Type: ActionCall, Min: 23, Max: 23},
		{Type: ActionRaise, Min: 48, Max: 1000},
	})
}

// TestReopeningInequality is the incomplete-raise rule as a table: a seat may
// raise iff ActedAtBet[seat] < LastFullRaiseTo.
func TestReopeningInequality(t *testing.T) {
	setup := func(t *testing.T) *Hand {
		h := mustSetup(t, HandSetup{
			Config: cfg12(),
			Button: 0, // seat 0 short-stacked to make the incomplete shove
			Stacks: map[Seat]Chips{0: 15, 1: 500, 2: 500, 3: 500},
			Seed:   2,
		})
		mustApply(t, h, Raise{3, 10}) // UTG: full raise, size 8, raise-to 10
		// Button's only raise is an all-in 15 < 10+8: incomplete.
		assertOptions(t, h, ActionOptions{
			{Type: ActionFold},
			{Type: ActionCall, Min: 10, Max: 10},
			{Type: ActionRaise, Min: 15, Max: 15},
		})
		mustApply(t, h, Raise{0, 15})
		// White-box: the price moved, the full-raise trackers did not.
		if h.bet.CurrentBet != 15 || h.bet.LastFullRaiseTo != 10 || h.bet.LastFullRaiseSz != 8 {
			t.Fatalf("betting state after incomplete raise = %+v", h.bet)
		}
		return h
	}

	t.Run("seats that never acted may still raise", func(t *testing.T) {
		h := setup(t)
		// SB posted but posting is not acting: full raising rights, priced
		// from the all-in level: min = 15 + 8 = 23.
		if h.CurrentSeat() != 1 {
			t.Fatalf("current = %d, want SB", h.CurrentSeat())
		}
		opt, ok := h.LegalActions().Find(ActionRaise)
		if !ok || opt.Min != 23 || opt.Max != 500 {
			t.Fatalf("SB raise option = %+v ok=%v, want Min 23 Max 500", opt, ok)
		}
		mustApply(t, h, Fold{1})
		// Same for the BB.
		opt, ok = h.LegalActions().Find(ActionRaise)
		if !ok || opt.Min != 23 {
			t.Fatalf("BB raise option = %+v ok=%v, want Min 23", opt, ok)
		}
	})

	t.Run("the original raiser may not reraise", func(t *testing.T) {
		h := setup(t)
		mustApply(t, h, Fold{1})
		mustApply(t, h, Call{2}) // BB just calls the 15
		// UTG acted at bet 10 and 10 < 10 is false: fold or call only.
		if h.CurrentSeat() != 3 {
			t.Fatalf("current = %d, want UTG", h.CurrentSeat())
		}
		assertOptions(t, h, ActionOptions{
			{Type: ActionFold},
			{Type: ActionCall, Min: 5, Max: 5},
		})
	})

	t.Run("a full raise restores raising rights", func(t *testing.T) {
		h := setup(t)
		mustApply(t, h, Fold{1})
		mustApply(t, h, Raise{2, 23}) // BB completes a full raise, size 8
		// UTG acted at 10 and 10 < 23: rights restored, min = 23 + 8 = 31.
		opt, ok := h.LegalActions().Find(ActionRaise)
		if !ok || opt.Min != 31 {
			t.Fatalf("UTG raise option = %+v ok=%v, want Min 31", opt, ok)
		}
	})
}

// TestShortAllInBetRights: the reopening inequality on an unbet street — a
// check is acting, so a sub-minimum all-in bet does not reopen the checker,
// while seats that have not yet acted keep full raising rights.
func TestShortAllInBetRights(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0, // BB seat 2 will have exactly 1 chip behind postflop
		Stacks: map[Seat]Chips{0: 100, 1: 100, 2: 3},
		Seed:   3,
	})
	mustApply(t, h, Call{0})
	mustApply(t, h, Call{1})
	mustApply(t, h, Check{2})
	mustApply(t, h, Check{1}) // flop checks through
	mustApply(t, h, Check{2})
	mustApply(t, h, Check{0})

	// Turn: SB checks, then the BB shoves its last chip — below the min bet.
	mustApply(t, h, Check{1})
	assertOptions(t, h, ActionOptions{
		{Type: ActionFold},
		{Type: ActionCheck},
		{Type: ActionBet, Min: 1, Max: 1}, // all-in below one BB
	})
	mustApply(t, h, Bet{2, 1})

	// Button has not acted this street: rights open, min = 1 + 2 (min bet).
	opt, ok := h.LegalActions().Find(ActionRaise)
	if !ok || opt.Min != 3 {
		t.Fatalf("button raise option = %+v ok=%v, want Min 3", opt, ok)
	}
	mustApply(t, h, Call{0})

	// The SB checked this street; the short all-in does not reopen it:
	// ActedAtBet 0 < LastFullRaiseTo 0 is false, so fold or call only.
	assertOptions(t, h, ActionOptions{
		{Type: ActionFold},
		{Type: ActionCall, Min: 1, Max: 1},
	})
	mustApply(t, h, Call{1})
	if h.Street() != River {
		t.Fatalf("street = %v, want river", h.Street())
	}
}

func TestFullAllInBetReopensChecker(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0, // BB seat 2 has exactly one BB behind postflop
		Stacks: map[Seat]Chips{0: 100, 1: 100, 2: 4},
		Seed:   4,
	})
	mustApply(t, h, Call{0})
	mustApply(t, h, Call{1})
	mustApply(t, h, Check{2})
	// Flop: SB checks, BB shoves 2 — exactly one big blind, a FULL bet.
	mustApply(t, h, Check{1})
	mustApply(t, h, Bet{2, 2})
	mustApply(t, h, Call{0})
	// The SB checked, but a full bet reopens it: raise available, min 2+2.
	opt, ok := h.LegalActions().Find(ActionRaise)
	if !ok || opt.Min != 4 {
		t.Fatalf("SB raise option = %+v ok=%v, want Min 4 after a full bet", opt, ok)
	}
}

func TestBBOption(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0,
		Stacks: map[Seat]Chips{0: 100, 1: 100, 2: 100},
		Seed:   5,
	})
	mustApply(t, h, Call{0}) // button limps
	mustApply(t, h, Call{1}) // SB completes

	// Everyone has matched the blind, but the BB posted without acting —
	// it still owes a decision: check the option or raise. No call option.
	if h.CurrentSeat() != 2 {
		t.Fatalf("current = %d, want BB", h.CurrentSeat())
	}
	assertOptions(t, h, ActionOptions{
		{Type: ActionFold},
		{Type: ActionCheck},
		{Type: ActionRaise, Min: 4, Max: 100},
	})

	// BB raises: the limpers must get to act again.
	mustApply(t, h, Raise{2, 8})
	if h.CurrentSeat() != 0 {
		t.Fatalf("after BB raise, current = %d, want 0", h.CurrentSeat())
	}
	mustApply(t, h, Fold{0})
	mustApply(t, h, Fold{1})
	if h.Phase() != PhaseComplete {
		t.Fatal("hand should end when both limpers fold to the BB raise")
	}
	assertNet(t, h, map[Seat]Chips{0: -2, 1: -2, 2: 4})
}

func TestBBCheckClosesPreflop(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0,
		Stacks: map[Seat]Chips{0: 100, 1: 100, 2: 100},
		Seed:   6,
	})
	mustApply(t, h, Call{0})
	mustApply(t, h, Call{1})
	mustApply(t, h, Check{2})
	if h.Street() != Flop {
		t.Fatalf("street = %v, want flop after the BB checks its option", h.Street())
	}
	if h.CurrentSeat() != 1 {
		t.Fatalf("flop opens with seat %d, want 1 (first live left of button)", h.CurrentSeat())
	}
}

func TestHeadsUpBlindsAndOrder(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 3,
		Stacks: map[Seat]Chips{3: 100, 5: 100},
		Seed:   7,
	})
	// The button posts the small blind heads-up.
	if h.Committed(3) != 1 || h.Committed(5) != 2 {
		t.Fatalf("blinds = %d/%d, want button 1 / other 2", h.Committed(3), h.Committed(5))
	}
	if h.Position(3) != PosBTN || h.Position(5) != PosBB {
		t.Fatalf("positions = %v/%v, want BTN/BB", h.Position(3), h.Position(5))
	}
	// Button acts first preflop, last postflop.
	if h.CurrentSeat() != 3 {
		t.Fatalf("preflop first = %d, want button", h.CurrentSeat())
	}
	mustApply(t, h, Call{3})
	mustApply(t, h, Check{5})
	if h.CurrentSeat() != 5 {
		t.Fatalf("flop first = %d, want big blind", h.CurrentSeat())
	}
}

// TestLegalityDrivenFuzz drives thousands of random hands by picking
// uniformly from LegalActions (and uniformly within each option's sizing
// range) until completion, asserting chip conservation after every action
// and clean termination. This is the test that finds ToAct bookkeeping bugs
// no hand-written scenario anticipates.
func TestLegalityDrivenFuzz(t *testing.T) {
	const iterations = 5000
	for it := 0; it < iterations; it++ {
		rng := rand.New(rand.NewPCG(uint64(it), 0xfacade))

		perm := rng.Perm(MaxSeats)
		n := 2 + rng.IntN(MaxSeats-1)
		stacks := make(map[Seat]Chips, n)
		seats := make([]Seat, 0, n)
		for _, s := range perm[:n] {
			// A quarter of stacks are tiny to exercise short blinds,
			// sub-minimum shoves, and multiway side pots.
			var stack Chips
			if rng.IntN(4) == 0 {
				stack = 1 + Chips(rng.IntN(5))
			} else {
				stack = 1 + Chips(rng.IntN(300))
			}
			stacks[Seat(s)] = stack
			seats = append(seats, Seat(s))
		}
		button := seats[rng.IntN(len(seats))]

		h, err := NewHandFromSetup(HandSetup{
			Config: cfg12(),
			Button: button,
			Stacks: stacks,
			Seed:   uint64(it),
			Eval:   hashEval,
		})
		if err != nil {
			t.Fatalf("iter %d: setup failed: %v", it, err)
		}
		assertConserved(t, h)

		for steps := 0; h.Phase() != PhaseComplete; steps++ {
			if steps > 400 {
				t.Fatalf("iter %d: hand did not terminate", it)
			}
			cur := h.CurrentSeat()
			if !cur.Valid() {
				t.Fatalf("iter %d: betting phase with no seat to act", it)
			}
			opts := h.LegalActions()
			if len(opts) == 0 {
				t.Fatalf("iter %d: no legal actions while betting", it)
			}
			o := opts[rng.IntN(len(opts))]
			size := o.Min
			if o.Max > o.Min {
				size += Chips(rng.Int64N(int64(o.Max-o.Min) + 1))
			}
			var a Action
			switch o.Type {
			case ActionFold:
				a = Fold{cur}
			case ActionCheck:
				a = Check{cur}
			case ActionCall:
				a = Call{cur}
			case ActionBet:
				a = Bet{cur, size}
			case ActionRaise:
				a = Raise{cur, size}
			}
			if err := h.Apply(a); err != nil {
				t.Fatalf("iter %d: engine rejected its own legal action %s %d: %v",
					it, o.Type, size, err)
			}
			assertConserved(t, h)
		}

		if got := h.LegalActions(); len(got) != 0 {
			t.Fatalf("iter %d: LegalActions non-empty after completion", it)
		}
		if h.CurrentSeat() != NoSeat {
			t.Fatalf("iter %d: CurrentSeat = %d after completion", it, h.CurrentSeat())
		}
		res, ok := h.Result()
		if !ok {
			t.Fatalf("iter %d: no result on a complete hand", it)
		}
		var net Chips
		for s := Seat(0); s < MaxSeats; s++ {
			net += res.Net[s]
			if h.Stack(s)-h.startStacks[s] != res.Net[s] {
				t.Fatalf("iter %d: Net[%d] inconsistent with stack delta", it, s)
			}
		}
		if net != 0 {
			t.Fatalf("iter %d: Net sums to %d, want 0", it, net)
		}
	}
}
