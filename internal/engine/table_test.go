package engine

import (
	"errors"
	"testing"
)

func newTestTable() *Table {
	t := NewTable(TableConfig{
		SmallBlind: 1, BigBlind: 2,
		MinBuyIn: 40, MaxBuyIn: 200,
		Seats: MaxSeats,
	})
	t.Eval = hashEval
	return t
}

func sitN(t *testing.T, tbl *Table, n int, buyIn Chips) {
	t.Helper()
	for s := Seat(0); int(s) < n; s++ {
		if err := tbl.Sit(s, "p", buyIn); err != nil {
			t.Fatalf("Sit(%d): %v", s, err)
		}
	}
}

// playWalk starts a hand from a deterministic deck and folds it out.
func playWalk(t *testing.T, tbl *Table, seed uint64) *Hand {
	t.Helper()
	h, err := tbl.StartHand(NewDeck(seed))
	if err != nil {
		t.Fatalf("StartHand: %v", err)
	}
	foldOut(t, h)
	if err := tbl.FinishHand(h); err != nil {
		t.Fatalf("FinishHand: %v", err)
	}
	return h
}

func TestSitValidation(t *testing.T) {
	tbl := newTestTable()
	if err := tbl.Sit(0, "a", 100); err != nil {
		t.Fatalf("Sit: %v", err)
	}
	if err := tbl.Sit(0, "b", 100); !errors.Is(err, ErrInvalidSeat) {
		t.Fatalf("double-sit: %v, want ErrInvalidSeat", err)
	}
	if err := tbl.Sit(1, "b", 39); !errors.Is(err, ErrInvalidBuyIn) {
		t.Fatalf("below min buy-in: %v, want ErrInvalidBuyIn", err)
	}
	if err := tbl.Sit(1, "b", 201); !errors.Is(err, ErrInvalidBuyIn) {
		t.Fatalf("above max buy-in: %v, want ErrInvalidBuyIn", err)
	}
	if err := tbl.Sit(Seat(7), "b", 100); !errors.Is(err, ErrInvalidSeat) {
		t.Fatalf("bad seat: %v, want ErrInvalidSeat", err)
	}
	if got := tbl.Status(0); got != SeatActive {
		t.Fatalf("status = %v, want active before the first hand", got)
	}
}

func TestStartHandNeedsTwoPlayers(t *testing.T) {
	tbl := newTestTable()
	sitN(t, tbl, 1, 100)
	if _, err := tbl.StartHand(NewDeck(1)); !errors.Is(err, ErrNotEnoughPlayers) {
		t.Fatalf("StartHand with one player: %v, want ErrNotEnoughPlayers", err)
	}
}

func TestStartFinishFlowAndButtonRotation(t *testing.T) {
	tbl := newTestTable()
	sitN(t, tbl, 3, 100)

	h, err := tbl.StartHand(NewDeck(1))
	if err != nil {
		t.Fatalf("StartHand: %v", err)
	}
	if tbl.Button() != 0 {
		t.Fatalf("first-hand button = %d, want 0", tbl.Button())
	}
	// Mid-hand, everything seat-related is locked.
	if _, err := tbl.StartHand(NewDeck(2)); !errors.Is(err, ErrHandInProgress) {
		t.Fatalf("second StartHand: %v, want ErrHandInProgress", err)
	}
	if err := tbl.Sit(4, "late", 100); !errors.Is(err, ErrHandInProgress) {
		t.Fatalf("Sit mid-hand: %v, want ErrHandInProgress", err)
	}
	if err := tbl.Rebuy(0, 10); !errors.Is(err, ErrHandInProgress) {
		t.Fatalf("Rebuy mid-hand: %v, want ErrHandInProgress", err)
	}
	if err := tbl.SitOut(1); !errors.Is(err, ErrHandInProgress) {
		t.Fatalf("SitOut mid-hand: %v, want ErrHandInProgress", err)
	}
	if err := tbl.FinishHand(h); !errors.Is(err, ErrHandInProgress) {
		t.Fatalf("FinishHand before completion: %v, want ErrHandInProgress", err)
	}

	foldOut(t, h) // walk: BB (seat 2) collects the SB's blind
	if err := tbl.FinishHand(h); err != nil {
		t.Fatalf("FinishHand: %v", err)
	}
	if tbl.Stack(1) != 99 || tbl.Stack(2) != 101 || tbl.Stack(0) != 100 {
		t.Fatalf("stacks after walk = %d/%d/%d, want 100/99/101",
			tbl.Stack(0), tbl.Stack(1), tbl.Stack(2))
	}
	if tbl.Button() != 1 {
		t.Fatalf("button after hand = %d, want 1", tbl.Button())
	}
	if tbl.HandsPlayed() != 1 {
		t.Fatalf("HandsPlayed = %d, want 1", tbl.HandsPlayed())
	}
	// Finishing the same hand twice is an error.
	if err := tbl.FinishHand(h); err == nil {
		t.Fatal("FinishHand accepted a hand that is not live")
	}
}

func TestRebuyLimits(t *testing.T) {
	tbl := newTestTable()
	sitN(t, tbl, 2, 100)
	if err := tbl.Rebuy(0, 101); !errors.Is(err, ErrInvalidBuyIn) {
		t.Fatalf("rebuy past max: %v, want ErrInvalidBuyIn", err)
	}
	if err := tbl.Rebuy(0, 100); err != nil {
		t.Fatalf("rebuy to max: %v", err)
	}
	if tbl.Stack(0) != 200 {
		t.Fatalf("stack = %d, want 200", tbl.Stack(0))
	}
	if err := tbl.Rebuy(3, 50); !errors.Is(err, ErrInvalidSeat) {
		t.Fatalf("rebuy on empty seat: %v, want ErrInvalidSeat", err)
	}
}

func TestHeadsUpButtonPostsSBAndAlternates(t *testing.T) {
	tbl := newTestTable()
	sitN(t, tbl, 2, 100)

	h := playWalk(t, tbl, 1)
	if h.Button() != 0 {
		t.Fatalf("hand 1 button = %d, want 0", h.Button())
	}
	// Heads-up the button posts the small blind.
	if e := h.Events()[0]; e.Kind != EvPostBlind || e.Seat != 0 || e.Amount != 1 {
		t.Fatalf("first blind event = %+v, want button posting 1", e)
	}

	h = playWalk(t, tbl, 2)
	if h.Button() != 1 {
		t.Fatalf("hand 2 button = %d, want 1 (alternating)", h.Button())
	}
	if e := h.Events()[0]; e.Seat != 1 || e.Amount != 1 {
		t.Fatalf("hand 2 first blind = %+v, want seat 1 posting 1", e)
	}
}

func TestBustedSeatIsNotDealt(t *testing.T) {
	tbl := newTestTable()
	sitN(t, tbl, 2, 40)

	h, err := tbl.StartHand(NewScriptedDeck(MustCards("2c 2d Ac Ad"), 1))
	if err != nil {
		t.Fatalf("StartHand: %v", err)
	}
	// Deal order heads-up: seat 1 (left of button) first, button last —
	// so seat 1 holds 2c2d and the button holds AcAd.
	tbl.Eval = nil // ensure the hand carried its own evaluator copy
	winner := holeEval(map[Card]uint32{MustCard("2c"): 1, MustCard("Ac"): 9})
	h.eval = winner // white-box: pin the stub for this scripted showdown

	mustApply(t, h, Raise{0, 40}) // button shoves
	mustApply(t, h, Call{1})      // all-in call, run-out
	if err := tbl.FinishHand(h); err != nil {
		t.Fatalf("FinishHand: %v", err)
	}
	if tbl.Stack(0) != 80 || tbl.Stack(1) != 0 {
		t.Fatalf("stacks = %d/%d, want 80/0", tbl.Stack(0), tbl.Stack(1))
	}
	// The busted seat is flagged by its zero stack and is not dealt in.
	if _, err := tbl.StartHand(NewDeck(3)); !errors.Is(err, ErrNotEnoughPlayers) {
		t.Fatalf("StartHand with a busted seat: %v, want ErrNotEnoughPlayers", err)
	}
	// A rebuy restores it.
	if err := tbl.Rebuy(1, 40); err != nil {
		t.Fatalf("Rebuy: %v", err)
	}
	tbl.Eval = hashEval
	if _, err := tbl.StartHand(NewDeck(4)); err != nil {
		t.Fatalf("StartHand after rebuy: %v", err)
	}
}

func TestSitInWaitsForTheBigBlind(t *testing.T) {
	tbl := newTestTable()
	sitN(t, tbl, 3, 100)

	playWalk(t, tbl, 1) // button 0 → 1
	if err := tbl.SitOut(1); err != nil {
		t.Fatalf("SitOut: %v", err)
	}

	// Hand 2: seats {0,2}, button seat 1 is gone → button lands on 2.
	h := playWalk(t, tbl, 2)
	if h.Button() != 2 || h.DealtIn() != NewSeatSet(0, 2) {
		t.Fatalf("hand 2: button %d dealtIn %v, want 2 and {0,2}", h.Button(), h.DealtIn())
	}

	if err := tbl.SitIn(1); err != nil {
		t.Fatalf("SitIn: %v", err)
	}
	if tbl.Status(1) != SeatWaitingForBB {
		t.Fatalf("status = %v, want waiting-for-bb", tbl.Status(1))
	}

	// Hand 3: button 0 (heads-up 0 vs 2, BB is 2). Seat 1 would be the SB,
	// not the BB, so it keeps waiting.
	h = playWalk(t, tbl, 3)
	if h.Button() != 0 || h.DealtIn().Has(1) {
		t.Fatalf("hand 3: button %d dealtIn %v — seat 1 must still wait", h.Button(), h.DealtIn())
	}

	// Hand 4: button 2, so the BB lands exactly on seat 1 → dealt in.
	h = playWalk(t, tbl, 4)
	if h.Button() != 2 {
		t.Fatalf("hand 4 button = %d, want 2", h.Button())
	}
	if !h.DealtIn().Has(1) {
		t.Fatal("hand 4: seat 1 was due for the big blind and must be dealt in")
	}
	if h.Position(1) != PosBB {
		t.Fatalf("seat 1 position = %v, want BB", h.Position(1))
	}
	if tbl.Status(1) != SeatActive {
		t.Fatalf("status = %v, want active after being dealt in", tbl.Status(1))
	}
}

func TestNewPlayerAfterFirstHandWaitsForBB(t *testing.T) {
	tbl := newTestTable()
	sitN(t, tbl, 2, 100)
	playWalk(t, tbl, 1)

	if err := tbl.Sit(4, "late", 100); err != nil {
		t.Fatalf("Sit: %v", err)
	}
	if got := tbl.Status(4); got != SeatWaitingForBB {
		t.Fatalf("late sitter status = %v, want waiting-for-bb", got)
	}
}

func TestLeaveCashesOut(t *testing.T) {
	tbl := newTestTable()
	sitN(t, tbl, 3, 100)
	if err := tbl.Leave(1); err != nil {
		t.Fatalf("Leave: %v", err)
	}
	if tbl.Status(1) != SeatEmpty || tbl.Stack(1) != 0 || tbl.Name(1) != "" {
		t.Fatal("Leave did not clear the seat")
	}
	if err := tbl.Leave(1); !errors.Is(err, ErrInvalidSeat) {
		t.Fatalf("Leave empty seat: %v, want ErrInvalidSeat", err)
	}
}

func TestHandPositionDerivedFromButton(t *testing.T) {
	h := mustSetup(t, HandSetup{
		Config: cfg12(),
		Button: 3,
		Stacks: map[Seat]Chips{0: 100, 1: 100, 2: 100, 3: 100, 4: 100, 5: 100},
		Seed:   1,
	})
	want := map[Seat]Position{
		3: PosBTN, 4: PosSB, 5: PosBB, 0: PosUTG, 1: PosHJ, 2: PosCO,
	}
	for seat, pos := range want {
		if got := h.Position(seat); got != pos {
			t.Errorf("Position(%d) = %v, want %v", seat, got, pos)
		}
	}
}

func TestNewTablePanicsOnBadConfig(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewTable accepted zero blinds")
		}
	}()
	NewTable(TableConfig{Seats: 6})
}
