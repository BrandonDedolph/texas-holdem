package rulebased

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// TestFlushDrawRightPriceNeverFolds pins the pot-odds lesson's central
// threshold: a flush draw getting 3:1 (needs 25%) has the equity to
// continue, so NO personality — not even the nit demanding a 10% premium —
// ever folds it. Mixed decisions may raise it; they may never fold it.
func TestFlushDrawRightPriceNeverFolds(t *testing.T) {
	for _, key := range []string{"nit", "tag", "lag", "station", "maniac", "coach"} {
		p := personality(t, key)
		for seed := int64(0); seed < 10; seed++ {
			strat := NewStrategy(p, seed)
			v := flushDrawFacingHalfPot(t, uint64(seed))
			d := strat.Decide(v)
			if d.Action.Type() == engine.ActionFold {
				t.Fatalf("%s (seed %d) folded a flush draw getting 3:1", key, seed)
			}
			if !d.Rationale.Has(ai.FactPotOdds) {
				t.Fatalf("%s facing a bet decided without a pot-odds fact", key)
			}
			if !d.Rationale.Has(ai.FactOuts) {
				t.Fatalf("%s on a draw decided without an outs fact", key)
			}
		}
	}
}

// TestStationNeverBluffsRiver: the station's halved bluff frequency lands
// below the bluff floor, so it NEVER turns a busted draw into a river bet
// — across any seed. This is the archetype's teaching claim, executable.
func TestStationNeverBluffsRiver(t *testing.T) {
	p := personality(t, "station")
	for seed := int64(0); seed < 60; seed++ {
		strat := NewStrategy(p, seed)
		v := bustedComboRiverSpot(t, 5)
		if d := strat.Decide(v); d.Action.Type() == engine.ActionBet {
			t.Fatalf("station (seed %d) bluffed the river with a busted draw", seed)
		}
	}
}

// TestBaselineMixesRiverBluffs: the TAG baseline bluffs busted combo draws
// at a real frequency — sometimes yes, sometimes no across seeds. If this
// fails, either the mixing or the busted-draw detection is broken.
func TestBaselineMixesRiverBluffs(t *testing.T) {
	bets, checks := 0, 0
	for seed := int64(0); seed < 60; seed++ {
		strat := NewStrategy(ai.Baseline(), seed)
		v := bustedComboRiverSpot(t, 5)
		switch strat.Decide(v).Action.Type() {
		case engine.ActionBet:
			bets++
		case engine.ActionCheck:
			checks++
		}
	}
	if bets == 0 || checks == 0 {
		t.Fatalf("TAG river bluff mix degenerate: %d bets, %d checks over 60 seeds", bets, checks)
	}
}

// TestNeverBluffIntoStation is the "stop bluffing seat 4" lesson: the SAME
// baseline that mixes river bluffs never fires one when the opponent is
// read as a calling station (CallDown < 0.8), and the decision cites the
// read as an ArchetypeFact — the coach can say why.
func TestNeverBluffIntoStation(t *testing.T) {
	station := personality(t, "station")
	cited := false
	for seed := int64(0); seed < 60; seed++ {
		strat := NewStrategy(ai.Baseline(), seed)
		strat.SetRead(1, station)
		v := bustedComboRiverSpot(t, 5)
		d := strat.Decide(v)
		if d.Action.Type() == engine.ActionBet {
			t.Fatalf("baseline (seed %d) bluffed into a known calling station", seed)
		}
		if f, ok := d.Rationale.Get(ai.FactArchetype); ok {
			cited = true
			af := f.(ai.ArchetypeFact)
			if af.Seat != 1 || af.Key != "station" {
				t.Fatalf("archetype fact cites seat %d key %q, want seat 1 station", af.Seat, af.Key)
			}
		}
	}
	if !cited {
		t.Error("killing the bluff never produced an ArchetypeFact; the coach cannot explain it")
	}
}

// TestValueBetsTopPair: the baseline value-bets top pair when checked to —
// "beginner honesty", no slowplay.
func TestValueBetsTopPair(t *testing.T) {
	h := headsUpHand(t, engine.Holes("As Kd"), engine.MustCards("Ah 7s 2c"), 3)
	mustApply(t, h, engine.Raise{S: 0, To: 25})
	mustApply(t, h, engine.Call{S: 1})
	mustApply(t, h, engine.Check{S: 1})
	v := h.View(0)

	strat := NewStrategy(ai.Baseline(), 7)
	d := strat.Decide(v)
	if d.Action.Type() != engine.ActionBet {
		t.Fatalf("TAG with top pair top kicker checked %v, want bet", d.Action.Type())
	}
	f, ok := d.Rationale.Get(ai.FactSizing)
	if !ok {
		t.Fatal("value bet decided without a sizing fact")
	}
	if sf := f.(ai.SizingFact); sf.Purpose != "value" {
		t.Errorf("sizing purpose %q, want value", sf.Purpose)
	}
	cf, _ := d.Rationale.Get(ai.FactClass)
	if cf.(ai.ClassFact).Made != ai.TopPair {
		t.Errorf("classified as %v, want top pair", cf.(ai.ClassFact).Made)
	}
}

// TestPostflopBetsAreHuman: every bet amount lands on the big-blind grid
// (or is exactly all-in) — no 137-into-400 computer bets.
func TestPostflopBetsAreHuman(t *testing.T) {
	h := headsUpHand(t, engine.Holes("As Kd"), engine.MustCards("Ah 7s 2c"), 3)
	mustApply(t, h, engine.Raise{S: 0, To: 25})
	mustApply(t, h, engine.Call{S: 1})
	mustApply(t, h, engine.Check{S: 1})
	v := h.View(0)

	opt, _ := v.Legal.Find(engine.ActionBet)
	for _, key := range []string{"nit", "tag", "lag", "station", "maniac"} {
		d := NewStrategy(personality(t, key), 1).Decide(v)
		for _, sc := range d.Candidates {
			amount, sized := amountOf(sc.Action)
			if !sized {
				continue
			}
			// Every discretized size is on the human grid (idempotent
			// under roundHuman) unless legality clamped it to a bound.
			if amount != roundHuman(amount, bb) && amount != opt.Min && amount != opt.Max {
				t.Errorf("%s candidate %v %d is off the human grid", key, sc.Action.Type(), amount)
			}
		}
	}
}

// TestCheapPriceBeatsThinEquity: facing a tiny bet with a weak holding the
// call threshold logic keys off pot odds, and the decision consumed a
// PotOddsFact either way.
func TestFacingBetAlwaysCitesPotOdds(t *testing.T) {
	for seed := uint64(1); seed <= 5; seed++ {
		v := flushDrawFacingHalfPot(t, seed)
		for _, key := range []string{"nit", "tag", "station"} {
			d := NewStrategy(personality(t, key), int64(seed)).Decide(v)
			if !d.Rationale.Has(ai.FactPotOdds) {
				t.Errorf("%s facing a bet has no pot-odds fact", key)
			}
			if !d.Rationale.Has(ai.FactEquity) {
				t.Errorf("%s facing a bet has no equity fact", key)
			}
			if !d.Rationale.Has(ai.FactRange) {
				t.Errorf("%s facing a bet has no range fact", key)
			}
		}
	}
}
