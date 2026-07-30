package coach

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// Test scaffolding: every spot is built through the real engine
// (NewHandFromSetup + applied actions), never a hand-assembled PlayerView,
// so every graded view is one the engine could actually produce.

const (
	sb = 5
	bb = 10
)

var cfg6 = engine.TableConfig{SmallBlind: sb, BigBlind: bb, Seats: 6}

func mustApply(t *testing.T, h *engine.Hand, a engine.Action) {
	t.Helper()
	if err := h.Apply(a); err != nil {
		t.Fatalf("apply %v seat %d: %v", a.Type(), a.Seat(), err)
	}
}

// huHand builds a heads-up hand (seats 0 and 1, button on 0 — so seat 0
// acts first preflop and last postflop) with both holes and a scripted
// board prefix, then applies the given actions.
func huHand(t *testing.T, hero, villain, board string, stack engine.Chips, seed uint64, actions ...engine.Action) *engine.Hand {
	t.Helper()
	h, err := engine.NewHandFromSetup(engine.HandSetup{
		Config: cfg6,
		Button: 0,
		Stacks: map[engine.Seat]engine.Chips{0: stack, 1: stack},
		Holes: map[engine.Seat][2]engine.Card{
			0: engine.Holes(hero),
			1: engine.Holes(villain),
		},
		Board: engine.MustCards(board),
		Seed:  seed,
		Eval:  eval.Standard{},
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	for _, a := range actions {
		mustApply(t, h, a)
	}
	return h
}

// flushDrawFacingBet is the canonical pot-odds teaching spot: hero on the
// button with a bare flush draw (9s8s on Ks7s2h), villain leads `bet` into
// the 50-chip preflop pot. With bet=25 the price is 25 into 75 → 25%
// required, comfortably beaten by a flush draw's ~35%: calling is correct.
func flushDrawFacingBet(t *testing.T, board string, villain string, bet engine.Chips, seed uint64) *engine.Hand {
	t.Helper()
	return huHand(t, "9s 8s", villain, board, 1000, seed,
		engine.Raise{S: 0, To: 25},
		engine.Call{S: 1},
		engine.Bet{S: 1, Amount: bet},
	)
}

// heroView returns the acting hero's view, failing if it is not the hero's
// turn (a mis-built spot should die loudly, not produce a Legal-less view).
func heroView(t *testing.T, h *engine.Hand, hero engine.Seat) *engine.PlayerView {
	t.Helper()
	if h.CurrentSeat() != hero {
		t.Fatalf("expected seat %d to act, current seat %d", hero, h.CurrentSeat())
	}
	return h.View(hero)
}

// checkDown checks both heads-up seats through the remaining streets until
// the hand completes, and returns the result.
func checkDown(t *testing.T, h *engine.Hand) *engine.HandResult {
	t.Helper()
	for h.Phase() == engine.PhaseBetting && h.CurrentSeat() != engine.NoSeat {
		mustApply(t, h, engine.Check{S: h.CurrentSeat()})
	}
	res, ok := h.Result()
	if !ok {
		t.Fatalf("hand did not complete (phase %v)", h.Phase())
	}
	return res
}

// sixHand builds a six-handed 100bb hand with the button on seat 0 and the
// given scripted holes, then applies the actions. Positions with button on
// 0: SB=1, BB=2, UTG=3, HJ=4, CO=5, BTN=0.
func sixHand(t *testing.T, holes map[engine.Seat][2]engine.Card, seed uint64, actions ...engine.Action) *engine.Hand {
	t.Helper()
	stacks := make(map[engine.Seat]engine.Chips, 6)
	for s := engine.Seat(0); s < 6; s++ {
		stacks[s] = 1000
	}
	h, err := engine.NewHandFromSetup(engine.HandSetup{
		Config: cfg6,
		Button: 0,
		Stacks: stacks,
		Holes:  holes,
		Seed:   seed,
		Eval:   eval.Standard{},
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	for _, a := range actions {
		mustApply(t, h, a)
	}
	return h
}

// bustedComboRiverSpot: the hero bet a combo draw on the flop, bricked out,
// and the villain checked the river to the hero — the canonical "do I turn
// this into a bluff?" spot. Hero seat 0 to act on the river.
func bustedComboRiverSpot(t *testing.T, seed uint64) *engine.Hand {
	t.Helper()
	return huHand(t, "9s 8s", "Ad Qd", "Ks 7s 6h 2d 2c", 1000, seed,
		engine.Raise{S: 0, To: 25},
		engine.Call{S: 1},
		// Flop: check, hero bets the combo draw, call.
		engine.Check{S: 1},
		engine.Bet{S: 0, Amount: 25},
		engine.Call{S: 1},
		// Turn: checks through.
		engine.Check{S: 1},
		engine.Check{S: 0},
		// River: villain checks, hero to act with the busted draw.
		engine.Check{S: 1},
	)
}
