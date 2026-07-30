package rulebased

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// Test scaffolding: spots are built through the real engine
// (NewHandFromSetup + applied actions), never hand-assembled PlayerViews,
// so every tested view is one the engine could actually produce.

const (
	sb = 5
	bb = 10
)

var cfg6 = engine.TableConfig{SmallBlind: sb, BigBlind: bb, Seats: 6}

// sixSeatStacks builds the standard 100bb six-handed stack map.
func sixSeatStacks() map[engine.Seat]engine.Chips {
	m := make(map[engine.Seat]engine.Chips, 6)
	for s := engine.Seat(0); s < 6; s++ {
		m[s] = 1000
	}
	return m
}

// heroSeatFor maps a position onto the seat that holds it with the button
// on seat 0: SB=1, BB=2, UTG=3, HJ=4, CO=5, BTN=0.
func heroSeatFor(pos engine.Position) engine.Seat {
	switch pos {
	case engine.PosSB:
		return 1
	case engine.PosBB:
		return 2
	case engine.PosUTG:
		return 3
	case engine.PosHJ:
		return 4
	case engine.PosCO:
		return 5
	default:
		return 0
	}
}

// unopenedSpot deals a six-handed hand and folds everyone in front of the
// hero, leaving the hero first-in at the given position (UTG/HJ/CO/BTN/SB).
func unopenedSpot(t *testing.T, pos engine.Position, hole [2]engine.Card, seed uint64) *engine.PlayerView {
	t.Helper()
	hero := heroSeatFor(pos)
	h, err := engine.NewHandFromSetup(engine.HandSetup{
		Config: cfg6, Button: 0,
		Stacks: sixSeatStacks(),
		Holes:  map[engine.Seat][2]engine.Card{hero: hole},
		Seed:   seed,
		Eval:   eval.Standard{},
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	for h.CurrentSeat() != hero {
		mustApply(t, h, engine.Fold{S: h.CurrentSeat()})
	}
	return h.View(hero)
}

// bbFacingBTNOpenSpot deals a hand where it folds to the button, the
// button opens to 25 (2.5bb), the small blind folds, and the hero in the
// big blind faces the open.
func bbFacingBTNOpenSpot(t *testing.T, hole [2]engine.Card, seed uint64) *engine.PlayerView {
	t.Helper()
	hero := engine.Seat(2) // BB with button on 0
	h, err := engine.NewHandFromSetup(engine.HandSetup{
		Config: cfg6, Button: 0,
		Stacks: sixSeatStacks(),
		Holes:  map[engine.Seat][2]engine.Card{hero: hole},
		Seed:   seed,
		Eval:   eval.Standard{},
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	mustApply(t, h, engine.Fold{S: 3})
	mustApply(t, h, engine.Fold{S: 4})
	mustApply(t, h, engine.Fold{S: 5})
	mustApply(t, h, engine.Raise{S: 0, To: 25})
	mustApply(t, h, engine.Fold{S: 1})
	if h.CurrentSeat() != hero {
		t.Fatalf("expected hero to act, current seat %d", h.CurrentSeat())
	}
	return h.View(hero)
}

// headsUpHand builds a heads-up hand (seats 0 and 1, button/SB on 0) with
// the given hero hole cards on seat 0, a scripted board prefix, and villain
// cards from the seed.
func headsUpHand(t *testing.T, heroHole [2]engine.Card, board []engine.Card, seed uint64) *engine.Hand {
	t.Helper()
	h, err := engine.NewHandFromSetup(engine.HandSetup{
		Config: engine.TableConfig{SmallBlind: sb, BigBlind: bb, Seats: 6},
		Button: 0,
		Stacks: map[engine.Seat]engine.Chips{0: 1000, 1: 1000},
		Holes:  map[engine.Seat][2]engine.Card{0: heroHole},
		Board:  board,
		Seed:   seed,
		Eval:   eval.Standard{},
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	return h
}

// flushDrawFacingHalfPot: hero on the button with a bare flush draw, the
// caller leads half pot on the flop. Required equity 25%.
func flushDrawFacingHalfPot(t *testing.T, seed uint64) *engine.PlayerView {
	t.Helper()
	h := headsUpHand(t, engine.Holes("9s 8s"), engine.MustCards("Ks 7s 2h"), seed)
	mustApply(t, h, engine.Raise{S: 0, To: 25})
	mustApply(t, h, engine.Call{S: 1})
	mustApply(t, h, engine.Bet{S: 1, Amount: 25})
	if h.CurrentSeat() != 0 {
		t.Fatalf("expected hero to act, current seat %d", h.CurrentSeat())
	}
	return h.View(0)
}

// bustedComboRiverSpot: the hero bet a combo draw on the flop, bricked the
// turn and river, and the villain has checked the river over to the hero —
// the canonical "do I turn this into a bluff?" spot.
func bustedComboRiverSpot(t *testing.T, seed uint64) *engine.PlayerView {
	t.Helper()
	h := headsUpHand(t, engine.Holes("9s 8s"), engine.MustCards("Ks 7s 6h 2d 2c"), seed)
	mustApply(t, h, engine.Raise{S: 0, To: 25})
	mustApply(t, h, engine.Call{S: 1})
	// Flop: check, hero bets the combo draw, call.
	mustApply(t, h, engine.Check{S: 1})
	mustApply(t, h, engine.Bet{S: 0, Amount: 25})
	mustApply(t, h, engine.Call{S: 1})
	// Turn: checks through.
	mustApply(t, h, engine.Check{S: 1})
	mustApply(t, h, engine.Check{S: 0})
	// River: villain checks, hero to act with busted draw.
	mustApply(t, h, engine.Check{S: 1})
	if h.CurrentSeat() != 0 || h.Street() != engine.River {
		t.Fatalf("expected hero on the river, seat %d street %v", h.CurrentSeat(), h.Street())
	}
	return h.View(0)
}

func mustApply(t *testing.T, h *engine.Hand, a engine.Action) {
	t.Helper()
	if err := h.Apply(a); err != nil {
		t.Fatalf("apply %v seat %d: %v", a.Type(), a.Seat(), err)
	}
}

func personality(t *testing.T, key string) ai.Personality {
	t.Helper()
	p, ok := ai.Archetype(key)
	if !ok {
		t.Fatalf("unknown archetype %q", key)
	}
	return p
}

// tally counts a seat's voluntary decisions during a sim.
type tally struct {
	folds, checks, calls, aggr int
	decisions                  int
}

func (c *tally) note(a engine.Action) {
	c.decisions++
	switch a.Type() {
	case engine.ActionFold:
		c.folds++
	case engine.ActionCheck:
		c.checks++
	case engine.ActionCall:
		c.calls++
	default:
		c.aggr++
	}
}

// vpip is the share of decisions that voluntarily put chips in.
func (c tally) vpipShare() float64 {
	if c.decisions == 0 {
		return 0
	}
	return float64(c.calls+c.aggr) / float64(c.decisions)
}

// aggrShare is the share of decisions that bet or raised.
func (c tally) aggrShare() float64 {
	if c.decisions == 0 {
		return 0
	}
	return float64(c.aggr) / float64(c.decisions)
}

// playHands runs full hands through the real engine with the given bots in
// every seat, tallying each seat's decisions. Every applied action is a
// Decision from the one strategy, so this doubles as a legality fuzz: an
// illegal chosen action fails the test immediately.
func playHands(t *testing.T, bots map[engine.Seat]*AI, hands int, deckSeed uint64) map[engine.Seat]*tally {
	t.Helper()
	table := engine.NewTable(cfg6)
	table.Eval = eval.Standard{}
	for seat, b := range bots {
		if err := table.Sit(seat, b.Name(), 1000); err != nil {
			t.Fatalf("sit %d: %v", seat, err)
		}
	}
	tallies := make(map[engine.Seat]*tally, len(bots))
	for seat := range bots {
		tallies[seat] = &tally{}
	}

	for i := 0; i < hands; i++ {
		h, err := table.StartHand(engine.NewDeck(deckSeed + uint64(i)*7919))
		if err != nil {
			t.Fatalf("hand %d: %v", i, err)
		}
		for h.Phase() == engine.PhaseBetting && h.CurrentSeat() != engine.NoSeat {
			seat := h.CurrentSeat()
			d := bots[seat].Strategy().Decide(h.View(seat))
			tallies[seat].note(d.Action)
			if err := h.Apply(d.Action); err != nil {
				t.Fatalf("hand %d: bot %s chose illegal %v: %v", i, bots[seat].Name(), d.Action.Type(), err)
			}
		}
		if err := table.FinishHand(h); err != nil {
			t.Fatalf("finish %d: %v", i, err)
		}
		// Top busted stacks back up so the lineup never shrinks.
		for seat := range bots {
			if missing := 1000 - table.Stack(seat); missing > 0 {
				if err := table.Rebuy(seat, missing); err != nil {
					t.Fatalf("rebuy %d: %v", seat, err)
				}
			}
		}
	}
	return tallies
}
