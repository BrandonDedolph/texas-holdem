package ai

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// TestArchetypePresets pins the parameter table from design-learning.md
// §4.6 at the invariant level: which dial deviates in which direction is
// each archetype's identity, and a drive-by "tuning" commit that flips one
// should fail loudly.
func TestArchetypePresets(t *testing.T) {
	for _, key := range append(append([]string{}, ArchetypeKeys...), "coach") {
		p, ok := Archetype(key)
		if !ok {
			t.Fatalf("archetype %q missing", key)
		}
		if p.Key != key || p.Label == "" || p.Blurb == "" {
			t.Errorf("archetype %q metadata incomplete: %+v", key, p)
		}
	}

	tag := Archetypes["tag"]
	if tag.RangeScale != 1 || tag.Aggression != 1 || tag.BluffFreq != 1 || tag.CallDown != 1 || tag.CBetFreq != 1 {
		t.Errorf("TAG is not the all-ones baseline: %+v", tag)
	}

	// The coach IS the TAG game — same dials, different name. If these
	// diverge the app teaches contradictions.
	coach := Archetypes["coach"]
	if coach.RangeScale != tag.RangeScale || coach.Aggression != tag.Aggression ||
		coach.BluffFreq != tag.BluffFreq || coach.CallDown != tag.CallDown ||
		coach.CBetFreq != tag.CBetFreq || coach.Patience != tag.Patience {
		t.Errorf("coach dials diverge from TAG: %+v vs %+v", coach, tag)
	}

	nit, lag, station, maniac := Archetypes["nit"], Archetypes["lag"], Archetypes["station"], Archetypes["maniac"]
	if !(nit.RangeScale < tag.RangeScale && tag.RangeScale < lag.RangeScale && lag.RangeScale < maniac.RangeScale) {
		t.Error("RangeScale ordering nit < tag < lag < maniac broken")
	}
	if !(station.Aggression < nit.Aggression && nit.Aggression < tag.Aggression &&
		tag.Aggression < lag.Aggression && lag.Aggression < maniac.Aggression) {
		t.Error("Aggression ordering station < nit < tag < lag < maniac broken")
	}
	if station.CallDown >= 0.8 {
		t.Error("station CallDown must be below 0.8 — the never-bluff-a-station rule keys off it")
	}
	if nit.CallDown <= 1 {
		t.Error("nit CallDown must demand a premium (> 1)")
	}
	if station.BluffFreq >= nit.BluffFreq {
		t.Error("station must bluff less than the nit")
	}
}

func TestRationaleHasGet(t *testing.T) {
	var r Rationale
	if r.Has(FactPotOdds) {
		t.Error("empty rationale reports a pot-odds fact")
	}
	r.Add(PotOddsFact{ToCall: 50, Pot: 150, Required: 0.25}, EquityFact{Equity: 0.31, Method: "exact"})
	if !r.Has(FactPotOdds) || !r.Has(FactEquity) || r.Has(FactOuts) {
		t.Errorf("Has misreports: %+v", r)
	}
	f, ok := r.Get(FactPotOdds)
	if !ok {
		t.Fatal("Get(FactPotOdds) missed")
	}
	if po := f.(PotOddsFact); po.Required != 0.25 {
		t.Errorf("fact round-trip lost data: %+v", po)
	}
}

// TestFactKinds pins the Kind() of every fact type — the coach's template
// dispatch depends on the mapping.
func TestFactKinds(t *testing.T) {
	cases := []struct {
		f    Fact
		want FactKind
	}{
		{PotOddsFact{}, FactPotOdds},
		{EquityFact{}, FactEquity},
		{OutsFact{}, FactOuts},
		{PositionFact{}, FactPosition},
		{RangeFact{}, FactRange},
		{ChartFact{}, FactChart},
		{ClassFact{}, FactClass},
		{ArchetypeFact{}, FactArchetype},
		{SizingFact{}, FactSizing},
		{InitiativeFact{}, FactInitiative},
	}
	for _, c := range cases {
		if c.f.Kind() != c.want {
			t.Errorf("%T.Kind() = %v, want %v", c.f, c.f.Kind(), c.want)
		}
	}
}

func TestBest(t *testing.T) {
	cands := []ScoredAction{
		{Action: engine.Fold{S: 0}, ScoreBB: 0},
		{Action: engine.Call{S: 0}, ScoreBB: 1.4},
		{Action: engine.Raise{S: 0, To: 60}, ScoreBB: 0.9},
	}
	if b := Best(cands); b.Action.Type() != engine.ActionCall {
		t.Errorf("Best picked %v, want call", b.Action.Type())
	}
}

func TestLineup(t *testing.T) {
	l := Lineup{1: nil}
	if _, ok := l.Player(1); ok {
		t.Error("nil entry reported as a player")
	}
	if _, ok := l.Player(0); ok {
		t.Error("absent seat reported as a player")
	}
}

// TestClassOrdering pins the strength ladders the postflop thresholds
// compare against (Made >= TopPair, Draw >= OESD, ...).
func TestClassOrdering(t *testing.T) {
	if !(Air < WeakPair && WeakPair < MiddlePair && MiddlePair < TopPair &&
		TopPair < Overpair && Overpair < TwoPair && TwoPair < TripsPlus) {
		t.Error("HandClass ordering broken")
	}
	if !(NoDraw < Gutshot && Gutshot < OESD && OESD < FlushDraw && FlushDraw < ComboDraw) {
		t.Error("DrawClass ordering broken")
	}
}
