package rulebased

import (
	"reflect"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// TestRationaleTruthfulness sweeps the classroom mix through full seeded
// hands and asserts, at every single decision point, the invariants the
// coach's honesty rests on:
//
//   - every Decision carries at least one fact — no branch is silent
//   - a decision facing a bet consumed a PotOddsFact
//   - every postflop decision consumed an equity and a range fact
//   - candidates are non-empty and the chosen action is among them
func TestRationaleTruthfulness(t *testing.T) {
	if testing.Short() {
		t.Skip("full-hand simulation")
	}
	keys := []string{"nit", "tag", "lag", "station", "maniac", "tag"}
	bots := make(map[engine.Seat]*AI, 6)
	for s := engine.Seat(0); s < 6; s++ {
		bots[s] = New(keys[s], personality(t, keys[s]), int64(s)*31+5)
	}

	table := engine.NewTable(cfg6)
	table.Eval = eval.Standard{}
	for seat, b := range bots {
		if err := table.Sit(seat, b.Name(), 1000); err != nil {
			t.Fatalf("sit: %v", err)
		}
	}

	checked := 0
	for i := 0; i < 20; i++ {
		h, err := table.StartHand(engine.NewDeck(uint64(90000 + i*13)))
		if err != nil {
			t.Fatalf("hand %d: %v", i, err)
		}
		for h.Phase() == engine.PhaseBetting && h.CurrentSeat() != engine.NoSeat {
			seat := h.CurrentSeat()
			v := h.View(seat)
			d := bots[seat].Strategy().Decide(v)
			checked++

			if len(d.Rationale.Facts) == 0 {
				t.Fatalf("hand %d %s: decision with an empty rationale", i, v.Street)
			}
			if len(d.Candidates) == 0 {
				t.Fatalf("hand %d %s: decision with no candidates", i, v.Street)
			}
			found := false
			for _, sc := range d.Candidates {
				if sameAction(sc.Action, d.Action) {
					found = true
				}
			}
			if !found {
				t.Fatalf("hand %d %s: chosen %v not among candidates", i, v.Street, d.Action.Type())
			}
			if v.ToCall > 0 && v.Street != engine.Preflop && !d.Rationale.Has(ai.FactPotOdds) {
				t.Fatalf("hand %d %s: facing a bet without a pot-odds fact", i, v.Street)
			}
			if v.ToCall > 0 && v.Street == engine.Preflop && countRaises(v) > 0 && !d.Rationale.Has(ai.FactPotOdds) {
				t.Fatalf("hand %d preflop: facing a raise without a pot-odds fact", i)
			}
			if v.Street != engine.Preflop {
				if !d.Rationale.Has(ai.FactEquity) {
					t.Fatalf("hand %d %s: postflop decision without an equity fact", i, v.Street)
				}
				if !d.Rationale.Has(ai.FactRange) {
					t.Fatalf("hand %d %s: postflop decision without a range fact", i, v.Street)
				}
				if !d.Rationale.Has(ai.FactClass) {
					t.Fatalf("hand %d %s: postflop decision without a class fact", i, v.Street)
				}
			} else if !d.Rationale.Has(ai.FactChart) && !d.Rationale.Has(ai.FactPotOdds) {
				t.Fatalf("hand %d preflop: decision cites neither chart nor pot odds", i)
			}

			if err := h.Apply(d.Action); err != nil {
				t.Fatalf("hand %d: illegal %v: %v", i, d.Action.Type(), err)
			}
		}
		if err := table.FinishHand(h); err != nil {
			t.Fatalf("finish: %v", err)
		}
		for seat := range bots {
			if missing := 1000 - table.Stack(seat); missing > 0 {
				if err := table.Rebuy(seat, missing); err != nil {
					t.Fatalf("rebuy: %v", err)
				}
			}
		}
	}
	if checked < 100 {
		t.Fatalf("only %d decisions checked; simulation too small to mean anything", checked)
	}
	t.Logf("verified rationale invariants over %d decisions", checked)
}

func countRaises(v *engine.PlayerView) int {
	n := 0
	for _, e := range v.History {
		if e.Kind == engine.EvAction && e.Street == engine.Preflop &&
			(e.Action == engine.ActionRaise || e.Action == engine.ActionBet) {
			n++
		}
	}
	return n
}

// TestDeterminism: the same seed and the same view produce the identical
// Decision — action, chosen size, candidates, scores, and facts — across
// separately constructed strategies AND separately constructed (identical)
// hands. This is what makes replays exact and grading auditable.
func TestDeterminism(t *testing.T) {
	build := func() []*engine.PlayerView {
		return []*engine.PlayerView{
			unopenedSpot(t, engine.PosCO, engine.Holes("Ad Jd"), 77),
			bbFacingBTNOpenSpot(t, engine.Holes("Ts 9s"), 78),
			flushDrawFacingHalfPot(t, 79),
			bustedComboRiverSpot(t, 80),
		}
	}
	a, b := build(), build()
	for i := range a {
		for _, key := range []string{"tag", "maniac"} {
			d1 := NewStrategy(personality(t, key), 1234).Decide(a[i])
			d2 := NewStrategy(personality(t, key), 1234).Decide(b[i])
			if !reflect.DeepEqual(d1, d2) {
				t.Errorf("%s spot %d: identical views produced different decisions:\n%+v\nvs\n%+v",
					key, i, d1.Action, d2.Action)
			}
		}
	}
}

// TestSeedChangesMixing: a different strategy seed may change a mixed
// decision (the river bluff), proving the seed actually reaches the
// mixing — determinism without variety would mean the RNG is dead code.
func TestSeedChangesMixing(t *testing.T) {
	seen := map[engine.ActionType]bool{}
	for seed := int64(0); seed < 40; seed++ {
		v := bustedComboRiverSpot(t, 5)
		d := NewStrategy(ai.Baseline(), seed).Decide(v)
		seen[d.Action.Type()] = true
	}
	if len(seen) < 2 {
		t.Errorf("40 seeds produced only %v; mixing is not wired to the seed", seen)
	}
}

// TestNoCheat: two hands identical in everything the hero may legally see
// — but with different opponent hole cards — produce byte-identical
// decisions. Together with the engine's guarantee that PlayerView carries
// no opponent cards, this is the "bots provably cannot cheat" claim.
func TestNoCheat(t *testing.T) {
	build := func(villain [2]engine.Card) *engine.PlayerView {
		h, err := engine.NewHandFromSetup(engine.HandSetup{
			Config: cfg6, Button: 0,
			Stacks: map[engine.Seat]engine.Chips{0: 1000, 1: 1000},
			Holes: map[engine.Seat][2]engine.Card{
				0: engine.Holes("Qs Js"),
				1: villain,
			},
			Board: engine.MustCards("Ts 9s 2h"),
			Seed:  55,
			Eval:  eval.Standard{},
		})
		if err != nil {
			t.Fatalf("setup: %v", err)
		}
		mustApply(t, h, engine.Raise{S: 0, To: 25})
		mustApply(t, h, engine.Call{S: 1})
		mustApply(t, h, engine.Bet{S: 1, Amount: 30})
		return h.View(0)
	}

	// Villain holds the stone nuts in one world and air in the other; the
	// hero's information set is identical, so the decision must be too.
	nuts := build(engine.Holes("Ah Kh"))
	air := build(engine.Holes("3d 2c"))

	for _, key := range []string{"tag", "station", "maniac"} {
		d1 := NewStrategy(personality(t, key), 9).Decide(nuts)
		d2 := NewStrategy(personality(t, key), 9).Decide(air)
		if !reflect.DeepEqual(d1, d2) {
			t.Errorf("%s: decision depends on opponent hole cards — the bot is cheating", key)
		}
	}

	// And the structural half: the view itself contains no card outside
	// Hole ∪ Board (the engine promises this; trust but verify here, since
	// this package's honesty claim rests on it).
	allowed := engine.NewCardSet(append([]engine.Card{nuts.Hole[0], nuts.Hole[1]}, nuts.Board...)...)
	for _, e := range nuts.History {
		for _, c := range e.Cards {
			if !allowed.Has(c) {
				t.Fatalf("PlayerView history leaks card %s", c.Code())
			}
		}
	}
}
