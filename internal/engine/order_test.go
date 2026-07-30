package engine

import (
	"reflect"
	"testing"
)

// sixHandOrder builds a full six-handed hand at blinds 1/2 with the button
// on the given seat. Holes are seeded — these tests never reach a showdown
// decision that depends on cards.
func sixHandOrder(t *testing.T, button Seat) *Hand {
	t.Helper()
	stacks := map[Seat]Chips{}
	for s := Seat(0); s < MaxSeats; s++ {
		stacks[s] = 200
	}
	return mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: button,
		Stacks: stacks,
		Seed:   7,
		Eval:   hashEval,
	})
}

// callToFlop limps every seat to the flop: callers to the big blind, then
// the big blind checks its option.
func callToFlop(t *testing.T, h *Hand) {
	t.Helper()
	for h.Street() == Preflop && h.Phase() == PhaseBetting {
		s := h.CurrentSeat()
		if h.ToCall(s) > 0 {
			mustApply(t, h, Call{s})
		} else {
			mustApply(t, h, Check{s})
		}
	}
	if h.Street() != Flop {
		t.Fatalf("expected flop, on %v", h.Street())
	}
}

// TestStreetOrderRenumbersAtTheFlop is the feature's reason to exist: the
// same six plates carry one order preflop (blinds last) and another
// postflop (blinds first), and the switch happens exactly at the flop.
func TestStreetOrderRenumbersAtTheFlop(t *testing.T) {
	h := sixHandOrder(t, 0) // SB=1 BB=2 UTG=3 HJ=4 CO=5 BTN=0

	preflop := []Seat{3, 4, 5, 0, 1, 2} // UTG first, blinds last
	if got := h.StreetOrder(); !reflect.DeepEqual(got, preflop) {
		t.Fatalf("preflop StreetOrder = %v, want %v", got, preflop)
	}

	callToFlop(t, h)

	postflop := []Seat{1, 2, 3, 4, 5, 0} // blinds first, button last
	if got := h.StreetOrder(); !reflect.DeepEqual(got, postflop) {
		t.Fatalf("flop StreetOrder = %v, want %v", got, postflop)
	}

	// The order must be a property of the street, not of who has acted:
	// mid-round, after two checks, the stamped order is unchanged.
	mustApply(t, h, Check{1})
	mustApply(t, h, Check{2})
	if got := h.StreetOrder(); !reflect.DeepEqual(got, postflop) {
		t.Fatalf("mid-round StreetOrder = %v, want %v", got, postflop)
	}
}

// TestStreetOrderHeadsUp pins the heads-up special case: the button is the
// small blind, first to act preflop and last postflop.
func TestStreetOrderHeadsUp(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 3,
		Stacks: map[Seat]Chips{3: 200, 5: 200},
		Seed:   7,
		Eval:   hashEval,
	})
	if sb, bb := h.SmallBlindSeat(), h.BigBlindSeat(); sb != 3 || bb != 5 {
		t.Fatalf("heads-up blinds SB=%d BB=%d, want SB=3 BB=5", sb, bb)
	}
	if got, want := h.StreetOrder(), []Seat{3, 5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("heads-up preflop order = %v, want %v", got, want)
	}
	mustApply(t, h, Call{3})
	mustApply(t, h, Check{5})
	if got, want := h.StreetOrder(), []Seat{5, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("heads-up flop order = %v, want %v", got, want)
	}
}

// TestStreetOrderKeepsFoldedSeats: a fold vacates a turn, not a position —
// the folded seat keeps its slot in the order so its plate can be stamped ✗
// without every other digit shifting.
func TestStreetOrderKeepsFoldedSeats(t *testing.T) {
	h := sixHandOrder(t, 0)
	mustApply(t, h, Fold{3}) // UTG folds
	want := []Seat{3, 4, 5, 0, 1, 2}
	if got := h.StreetOrder(); !reflect.DeepEqual(got, want) {
		t.Fatalf("StreetOrder after UTG fold = %v, want %v", got, want)
	}
	if h.Live().Has(3) {
		t.Fatal("seat 3 should be folded")
	}
}

// TestStreetOrderSkipsSeatsNotDealtIn: sitting-out seats are not part of
// the hand and hold no position in the order.
func TestStreetOrderSkipsSeatsNotDealtIn(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0,
		Stacks: map[Seat]Chips{0: 200, 2: 200, 4: 200}, // seats 1, 3, 5 sit out
		Seed:   7,
		Eval:   hashEval,
	})
	// Three-handed with button 0: SB=2, BB=4, and the button opens preflop.
	if sb, bb := h.SmallBlindSeat(), h.BigBlindSeat(); sb != 2 || bb != 4 {
		t.Fatalf("blinds SB=%d BB=%d, want SB=2 BB=4", sb, bb)
	}
	if got, want := h.StreetOrder(), []Seat{0, 2, 4}; !reflect.DeepEqual(got, want) {
		t.Fatalf("preflop order = %v, want %v", got, want)
	}
	mustApply(t, h, Call{0})
	mustApply(t, h, Call{2})
	mustApply(t, h, Check{4})
	if got, want := h.StreetOrder(), []Seat{2, 4, 0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("flop order = %v, want %v", got, want)
	}
}

// TestNeighbours pins LeftOf/RightOf: left is clockwise (acts after you),
// right is counter-clockwise, and both skip seats not dealt in.
func TestNeighbours(t *testing.T) {
	full := sixHandOrder(t, 0)
	if l, r := full.LeftOf(0), full.RightOf(0); l != 1 || r != 5 {
		t.Errorf("full ring neighbours of 0 = left %d, right %d; want 1, 5", l, r)
	}
	if l, r := full.LeftOf(5), full.RightOf(5); l != 0 || r != 4 {
		t.Errorf("full ring neighbours of 5 = left %d, right %d; want 0, 4", l, r)
	}

	sparse := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 0,
		Stacks: map[Seat]Chips{0: 200, 2: 200, 5: 200},
		Seed:   7,
		Eval:   hashEval,
	})
	if l, r := sparse.LeftOf(0), sparse.RightOf(0); l != 2 || r != 5 {
		t.Errorf("sparse neighbours of 0 = left %d, right %d; want 2, 5", l, r)
	}

	hu := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 1,
		Stacks: map[Seat]Chips{1: 200, 4: 200},
		Seed:   7,
		Eval:   hashEval,
	})
	// Heads-up your one opponent is both neighbours.
	if l, r := hu.LeftOf(1), hu.RightOf(1); l != 4 || r != 4 {
		t.Errorf("heads-up neighbours of 1 = left %d, right %d; want 4, 4", l, r)
	}
}

// TestSeatSetPrev pins Prev as Next's mirror, wrap included.
func TestSeatSetPrev(t *testing.T) {
	set := NewSeatSet(1, 3, 4)
	cases := []struct{ before, want Seat }{
		{0, 4}, // wraps backwards
		{1, 4},
		{3, 1},
		{4, 3},
		{5, 4},
	}
	for _, c := range cases {
		if got := set.Prev(c.before); got != c.want {
			t.Errorf("Prev(%d) = %d, want %d", c.before, got, c.want)
		}
	}
	if got := SeatSet(0).Prev(2); got != NoSeat {
		t.Errorf("empty set Prev = %d, want NoSeat", got)
	}
}
