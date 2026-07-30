// Package engine_test holds the end-to-end tests that exercise the engine
// against the real hand evaluator.
//
// These live in an external test package because internal/eval imports
// internal/engine, so the engine's own package tests cannot import it back.
// From out here the edge runs eval → engine as designed and there is no cycle.
package engine_test

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// playOut applies a scripted list of actions, failing the test on the first
// rejection. Each action is checked against LegalActions first, so a script
// that drifts out of sync with the engine reports which action was illegal
// rather than a bare error.
func playOut(t *testing.T, h *engine.Hand, actions ...engine.Action) {
	t.Helper()
	for i, a := range actions {
		if got := h.CurrentSeat(); got != a.Seat() {
			t.Fatalf("action %d (%T): seat to act is %d, script says %d", i, a, got, a.Seat())
		}
		if err := h.Validate(a); err != nil {
			t.Fatalf("action %d (%T seat %d): %v (legal: %v)", i, a, a.Seat(), err, h.LegalActions())
		}
		if err := h.Apply(a); err != nil {
			t.Fatalf("action %d (%T seat %d): Apply: %v", i, a, a.Seat(), err)
		}
	}
}

// TestShowdownPicksTheBestHand plays a scripted three-handed hand to showdown
// and checks the real evaluator awards the pot to the best five cards — the
// vertical slice through engine + eval.
func TestShowdownPicksTheBestHand(t *testing.T) {
	// Board pairs the deuce and puts three spades out.
	//   seat 0: Ah Kh -> ace-king high, no pair
	//   seat 1: Qs 9s -> flush, queen high  (the winner)
	//   seat 2: 2c 2d -> quad deuces? no: 2c 2d + 2h on board = trip deuces
	h, err := engine.NewHandFromSetup(engine.HandSetup{
		Config: engine.TableConfig{SmallBlind: 5, BigBlind: 10},
		Button: 0,
		Stacks: map[engine.Seat]engine.Chips{0: 1000, 1: 1000, 2: 1000},
		Holes: map[engine.Seat][2]engine.Card{
			0: engine.Holes("Ah Kh"),
			1: engine.Holes("Qs 9s"),
			2: engine.Holes("2c 2d"),
		},
		Board: engine.MustCards("2h 7s 4s 8s Jd"),
		Seed:  42,
		Eval:  eval.Evaluator,
	})
	if err != nil {
		t.Fatalf("NewHandFromSetup: %v", err)
	}

	// Everyone limps/checks to showdown. Button (seat 0) acts first three-handed
	// preflop; the big blind (seat 2) has the option.
	playOut(t, h,
		engine.Call{S: 0}, engine.Call{S: 1}, engine.Check{S: 2}, // preflop
		engine.Check{S: 1}, engine.Check{S: 2}, engine.Check{S: 0}, // flop
		engine.Check{S: 1}, engine.Check{S: 2}, engine.Check{S: 0}, // turn
		engine.Check{S: 1}, engine.Check{S: 2}, engine.Check{S: 0}, // river
	)

	res, ok := h.Result()
	if !ok {
		t.Fatalf("hand did not complete; phase = %v", h.Phase())
	}
	if len(res.Awards) != 1 {
		t.Fatalf("got %d pot awards, want 1 (%+v)", len(res.Awards), res.Awards)
	}

	award := res.Awards[0]
	if len(award.Winners) != 1 || award.Winners[0] != 1 {
		t.Fatalf("winners = %v, want [1] (queen-high flush)", award.Winners)
	}
	if got, want := eval.Rank(award.Rank).Describe(), "Flush, Queen high (Q-9-8-7-4)"; got != want {
		t.Errorf("winning hand described as %q, want %q", got, want)
	}

	// Three limpers at 10 apiece.
	if want := engine.Chips(30); award.Amount != want {
		t.Errorf("pot = %v, want %v", award.Amount, want)
	}
	// Net always sums to zero, and the winner is up two other players' blinds.
	var sum engine.Chips
	for _, n := range res.Net {
		sum += n
	}
	if sum != 0 {
		t.Errorf("Net sums to %v, want 0 (%v)", sum, res.Net)
	}
	if want := engine.Chips(20); res.Net[1] != want {
		t.Errorf("winner net = %v, want %v", res.Net[1], want)
	}
}

// TestSplitPotDividesEvenly checks that a board-plays tie splits, using the
// real evaluator to establish the tie.
func TestSplitPotDividesEvenly(t *testing.T) {
	// The board is a queen-high straight; neither player's hole cards play.
	h, err := engine.NewHandFromSetup(engine.HandSetup{
		Config: engine.TableConfig{SmallBlind: 5, BigBlind: 10},
		Button: 0,
		Stacks: map[engine.Seat]engine.Chips{0: 1000, 1: 1000},
		Holes: map[engine.Seat][2]engine.Card{
			0: engine.Holes("2c 3d"),
			1: engine.Holes("2h 3s"),
		},
		Board: engine.MustCards("8c 9d Th Jc Qs"),
		Seed:  7,
		Eval:  eval.Evaluator,
	})
	if err != nil {
		t.Fatalf("NewHandFromSetup: %v", err)
	}

	// Heads-up the button posts the small blind and acts first preflop.
	playOut(t, h,
		engine.Call{S: 0}, engine.Check{S: 1},
		engine.Check{S: 1}, engine.Check{S: 0},
		engine.Check{S: 1}, engine.Check{S: 0},
		engine.Check{S: 1}, engine.Check{S: 0},
	)

	res, ok := h.Result()
	if !ok {
		t.Fatalf("hand did not complete; phase = %v", h.Phase())
	}
	award := res.Awards[0]
	if len(award.Winners) != 2 {
		t.Fatalf("winners = %v, want both seats (the board plays)", award.Winners)
	}
	if got, want := eval.Rank(award.Rank).Describe(), "Straight, Eight to Queen"; got != want {
		t.Errorf("winning hand described as %q, want %q", got, want)
	}
	// Both players put in 10; both get 10 back.
	for seat, net := range res.Net[:2] {
		if net != 0 {
			t.Errorf("seat %d net = %v, want 0 in a split pot", seat, net)
		}
	}
}

// TestTableDealsAndAwardsWithRealEvaluator drives the full Table lifecycle —
// seating, StartHand, play, FinishHand — and checks chips move correctly across
// the hand boundary.
func TestTableDealsAndAwardsWithRealEvaluator(t *testing.T) {
	tbl := engine.NewTable(engine.TableConfig{
		SmallBlind: 5, BigBlind: 10,
		MinBuyIn: 400, MaxBuyIn: 1000,
		Seats: 6,
	})
	tbl.Eval = eval.Evaluator
	names := []string{"You", "Nia", "Cole", "Ivy", "Tara", "Sam"}
	for i, name := range names {
		if err := tbl.Sit(engine.Seat(i), name, 1000); err != nil {
			t.Fatalf("Sit(%d): %v", i, err)
		}
	}

	const startingTotal = engine.Chips(6 * 1000)
	total := func() engine.Chips {
		var sum engine.Chips
		for i := range names {
			sum += tbl.Stack(engine.Seat(i))
		}
		return sum
	}

	// Play several hands with everyone folding to the big blind where possible,
	// otherwise taking the first legal action. The point is the lifecycle, not
	// the poker: chips must be conserved across hand boundaries and the button
	// must advance.
	prevButton := tbl.Button()
	for hand := 0; hand < 10; hand++ {
		h, err := tbl.StartHand(engine.NewDeck(uint64(hand)))
		if err != nil {
			t.Fatalf("hand %d: StartHand: %v", hand, err)
		}
		for h.CurrentSeat() != engine.NoSeat {
			opts := h.LegalActions()
			if len(opts) == 0 {
				t.Fatalf("hand %d: seat %d to act with no legal actions", hand, h.CurrentSeat())
			}
			seat := h.CurrentSeat()
			// Prefer checking, else fold — a cheap way to reach showdowns and
			// walks without modelling strategy.
			var a engine.Action = engine.Fold{S: seat}
			if _, ok := opts.Find(engine.ActionCheck); ok {
				a = engine.Check{S: seat}
			}
			if err := h.Apply(a); err != nil {
				t.Fatalf("hand %d: Apply(%T) seat %d: %v", hand, a, seat, err)
			}
		}
		if err := tbl.FinishHand(h); err != nil {
			t.Fatalf("hand %d: FinishHand: %v", hand, err)
		}
		if got := total(); got != startingTotal {
			t.Fatalf("hand %d: chips on the table = %v, want %v", hand, got, startingTotal)
		}
		if b := tbl.Button(); b == prevButton {
			t.Fatalf("hand %d: button did not move from seat %d", hand, prevButton)
		} else {
			prevButton = b
		}
	}
	if tbl.HandsPlayed() != 10 {
		t.Errorf("HandsPlayed = %d, want 10", tbl.HandsPlayed())
	}
}
