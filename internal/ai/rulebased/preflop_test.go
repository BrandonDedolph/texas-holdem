package rulebased

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// openPositions are the positions with a raise-first-in decision.
var openPositions = []engine.Position{
	engine.PosUTG, engine.PosHJ, engine.PosCO, engine.PosBTN, engine.PosSB,
}

// classReps is one representative combo per 169-grid hand class, spades
// and hearts, for exhaustive-but-cheap preflop sweeps.
func classReps() [][2]engine.Card {
	var out [][2]engine.Card
	for hi := engine.Rank(0); hi < engine.NumRanks; hi++ {
		out = append(out, [2]engine.Card{engine.MakeCard(hi, engine.Spades), engine.MakeCard(hi, engine.Hearts)})
		for lo := engine.Rank(0); lo < hi; lo++ {
			out = append(out,
				[2]engine.Card{engine.MakeCard(hi, engine.Spades), engine.MakeCard(lo, engine.Spades)},
				[2]engine.Card{engine.MakeCard(hi, engine.Spades), engine.MakeCard(lo, engine.Hearts)})
		}
	}
	return out
}

// TestBaselineNeverOpenLimps is the "coach must never recommend a limp"
// rule made a test: first-in at every position with every hand class, no
// personality at or above the aggression floor ever calls. Limping is
// reserved for sub-floor personalities (the station) by construction.
func TestBaselineNeverOpenLimps(t *testing.T) {
	reps := classReps()
	for _, key := range []string{"nit", "tag", "lag", "maniac", "coach"} {
		strat := NewStrategy(personality(t, key), 11)
		for _, pos := range openPositions {
			for i, hole := range reps {
				v := unopenedSpot(t, pos, hole, uint64(1000+i))
				d := strat.Decide(v)
				if d.Action.Type() == engine.ActionCall {
					t.Fatalf("%s open-limped %v %v first-in at %s", key, hole[0], hole[1], pos)
				}
			}
		}
	}
}

// TestPreflopSpotDecisions pins the individual open decisions the design
// calls out, through the full strategy rather than the raw charts.
func TestPreflopSpotDecisions(t *testing.T) {
	tag := NewStrategy(ai.Baseline(), 7)

	for _, pos := range openPositions {
		v := unopenedSpot(t, pos, engine.Holes("As Ah"), 42)
		if d := tag.Decide(v); d.Action.Type() != engine.ActionRaise {
			t.Errorf("TAG with AA first-in at %s chose %v, want raise", pos, d.Action.Type())
		}
		v = unopenedSpot(t, pos, engine.Holes("7s 2h"), 42)
		if d := tag.Decide(v); d.Action.Type() != engine.ActionFold {
			t.Errorf("TAG with 72o first-in at %s chose %v, want fold", pos, d.Action.Type())
		}
	}

	v := unopenedSpot(t, engine.PosBTN, engine.Holes("As 5s"), 42)
	if d := tag.Decide(v); d.Action.Type() != engine.ActionRaise {
		t.Errorf("TAG with A5s on the BTN chose %v, want raise", d.Action.Type())
	}
	v = unopenedSpot(t, engine.PosUTG, engine.Holes("As 5s"), 42)
	if d := tag.Decide(v); d.Action.Type() != engine.ActionFold {
		t.Errorf("TAG with A5s UTG chose %v, want fold", d.Action.Type())
	}
}

// TestOpenSizeIsHuman pins the open sizing: 2.5bb standard, in whole
// chips, rounded to the blind grid.
func TestOpenSizeIsHuman(t *testing.T) {
	tag := NewStrategy(ai.Baseline(), 7)
	v := unopenedSpot(t, engine.PosCO, engine.Holes("As Ks"), 42)
	d := tag.Decide(v)
	r, ok := d.Action.(engine.Raise)
	if !ok {
		t.Fatalf("expected raise, got %v", d.Action.Type())
	}
	if r.To != 25 {
		t.Errorf("open size %d, want 25 (2.5bb)", r.To)
	}
	if r.To != roundHuman(r.To, bb) {
		t.Errorf("open size %d is off the human chip grid", r.To)
	}
}

// TestFacingOpenCitesTheNumbers: defending the big blind is the pot-odds
// lesson, so the decision must have consumed pot odds, a range read, and
// a chart lookup — whatever the action taken.
func TestFacingOpenCitesTheNumbers(t *testing.T) {
	tag := NewStrategy(ai.Baseline(), 7)
	for i, hole := range [][2]engine.Card{
		engine.Holes("As Ah"), // 3-bets
		engine.Holes("9s 8s"), // defends
		engine.Holes("7s 2h"), // folds
	} {
		v := bbFacingBTNOpenSpot(t, hole, uint64(50+i))
		d := tag.Decide(v)
		for _, k := range []ai.FactKind{ai.FactPotOdds, ai.FactRange, ai.FactChart, ai.FactPosition} {
			if !d.Rationale.Has(k) {
				t.Errorf("BB defend with %v %v: rationale missing %v fact", hole[0], hole[1], k)
			}
		}
	}
}

// TestFacingOpenBranches pins the three-way split: premium 3-bets, decent
// suited hand calls, trash folds.
func TestFacingOpenBranches(t *testing.T) {
	tag := NewStrategy(ai.Baseline(), 7)

	if d := tag.Decide(bbFacingBTNOpenSpot(t, engine.Holes("As Ah"), 1)); d.Action.Type() != engine.ActionRaise {
		t.Errorf("AA in BB vs BTN open chose %v, want 3-bet", d.Action.Type())
	}
	if d := tag.Decide(bbFacingBTNOpenSpot(t, engine.Holes("9s 8s"), 2)); d.Action.Type() != engine.ActionCall {
		t.Errorf("98s in BB vs BTN open chose %v, want call", d.Action.Type())
	}
	if d := tag.Decide(bbFacingBTNOpenSpot(t, engine.Holes("7s 2h"), 3)); d.Action.Type() != engine.ActionFold {
		t.Errorf("72o in BB vs BTN open chose %v, want fold", d.Action.Type())
	}
}

// TestStationLimps: the one personality below the aggression floor does
// limp — the archetype's defining leak exists and is visible.
func TestStationLimps(t *testing.T) {
	station := NewStrategy(personality(t, "station"), 3)
	limped := false
	for i, hole := range classReps() {
		v := unopenedSpot(t, engine.PosBTN, hole, uint64(2000+i))
		if station.Decide(v).Action.Type() == engine.ActionCall {
			limped = true
			break
		}
	}
	if !limped {
		t.Error("station never limped first-in on the button; the archetype's leak is missing")
	}
}

// TestCandidatesCoverLegalClasses: every Decision carries a scored
// candidate for each legal action class, including multiple raise sizes,
// and the chosen action is always among them.
func TestCandidatesCoverLegalClasses(t *testing.T) {
	tag := NewStrategy(ai.Baseline(), 7)
	v := bbFacingBTNOpenSpot(t, engine.Holes("As Ks"), 9)
	d := tag.Decide(v)

	byType := map[engine.ActionType]int{}
	for _, sc := range d.Candidates {
		byType[sc.Action.Type()]++
	}
	if byType[engine.ActionFold] != 1 || byType[engine.ActionCall] != 1 {
		t.Errorf("candidates missing fold/call: %v", byType)
	}
	if byType[engine.ActionRaise] < 2 {
		t.Errorf("want at least two discretized raise sizes, got %d", byType[engine.ActionRaise])
	}
	found := false
	for _, sc := range d.Candidates {
		if sameAction(sc.Action, d.Action) {
			found = true
		}
	}
	if !found {
		t.Error("chosen action not among candidates")
	}
}
