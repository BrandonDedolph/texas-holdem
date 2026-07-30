package ai

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
)

// Rationale is the typed reasoning a Decision emitted while deciding.
// The facts are appended by the strategy branches that actually consumed
// them — never reconstructed afterwards — which is the contract that keeps
// coach output truthful (DESIGN.md §1 principle 3). Facts are ordered by
// salience; the coach renders the top few.
type Rationale struct {
	Facts []Fact
}

// Add appends facts in order.
func (r *Rationale) Add(facts ...Fact) { r.Facts = append(r.Facts, facts...) }

// Has reports whether the rationale contains a fact of the given kind.
func (r Rationale) Has(k FactKind) bool {
	_, ok := r.Get(k)
	return ok
}

// Get returns the first fact of the given kind.
func (r Rationale) Get(k FactKind) (Fact, bool) {
	for _, f := range r.Facts {
		if f.Kind() == k {
			return f, true
		}
	}
	return nil, false
}

// FactKind discriminates Fact implementations without reflection.
type FactKind uint8

// Fact kinds, one per concrete fact type.
const (
	FactPotOdds FactKind = iota
	FactEquity
	FactOuts
	FactPosition
	FactRange
	FactChart
	FactClass
	FactArchetype
	FactSizing
	FactInitiative
)

var factKindNames = [...]string{
	"pot-odds", "equity", "outs", "position", "range",
	"chart", "class", "archetype", "sizing", "initiative",
}

// String is the lowercase kind name ("pot-odds").
func (k FactKind) String() string {
	if int(k) >= len(factKindNames) {
		return "?"
	}
	return factKindNames[k]
}

// Fact is one typed unit of reasoning. The set of implementations is closed
// (the unexported method seals it to this package): the coach's explanation
// templates are written against exactly these types, and an open set would
// reintroduce the "explanation drifts from the decision" failure mode.
type Fact interface {
	Kind() FactKind
	fact()
}

// PotOddsFact records the price of a call: the strategy compared its equity
// against Required = ToCall / (Pot + ToCall).
type PotOddsFact struct {
	ToCall   engine.Chips
	Pot      engine.Chips // pot before the call — what the hero stands to win
	Required float64      // break-even equity, 0..1
}

// EquityFact records a hand-vs-range equity the decision consumed. Method
// says how it was obtained: "exact" (full enumeration), "sampled"
// (deterministic Monte Carlo), "chart" (preflop chart-percentile proxy), or
// "implied-odds credit" (a stated adjustment on top of another EquityFact).
type EquityFact struct {
	Equity float64
	Method string
}

// OutsFact records the outs inventory the decision consumed, computed
// against the same perceived range the equity used.
type OutsFact struct {
	Report equity.OutsReport
}

// PositionFact records the hero's position and whether the hero acts after
// the primary opponent postflop.
type PositionFact struct {
	Pos        engine.Position
	InPosition bool
}

// RangeFact records the range the strategy put an opponent on.
type RangeFact struct {
	Seat    engine.Seat
	Range   equity.Range
	Summary string // "≈15% range (opened from UTG)" — pre-rendered for the coach
}

// ChartFact records a preflop chart lookup: which chart, and whether the
// hero's hand was in it (after personality scaling).
type ChartFact struct {
	Chart   string // "BTN open", "vs 3-bet", ...
	InRange bool
}

// ClassFact records the postflop classification of the hero's hand.
//
// Design note: design-learning.md §4.5 declared this as wrapping
// rulebased.Classification, but ai cannot import rulebased (rulebased
// imports ai). The classification enums therefore live here — see HandClass
// and DrawClass below — and rulebased builds on them, keeping a single
// definition. The OutsReport the learning doc bundled into the
// classification travels as its own OutsFact.
type ClassFact struct {
	Made HandClass
	Draw DrawClass
}

// ArchetypeFact records that an opponent's read changed the decision —
// "station: don't bluff" is the canonical example.
type ArchetypeFact struct {
	Seat engine.Seat
	Key  string // archetype key, "" when the read is behavioral only
	Note string
}

// SizingFact records why a bet or raise is the size it is.
type SizingFact struct {
	FractionOfPot float64
	Purpose       string // "value", "semi-bluff", "deny equity", "open", "bluff", ...
}

// InitiativeFact records who is driving the betting: true when the hero
// made the last aggressive action, which is what gates the c-bet branch.
type InitiativeFact struct {
	HeroIsAggressor bool
}

// Kind implements Fact.
func (PotOddsFact) Kind() FactKind { return FactPotOdds }

// Kind implements Fact.
func (EquityFact) Kind() FactKind { return FactEquity }

// Kind implements Fact.
func (OutsFact) Kind() FactKind { return FactOuts }

// Kind implements Fact.
func (PositionFact) Kind() FactKind { return FactPosition }

// Kind implements Fact.
func (RangeFact) Kind() FactKind { return FactRange }

// Kind implements Fact.
func (ChartFact) Kind() FactKind { return FactChart }

// Kind implements Fact.
func (ClassFact) Kind() FactKind { return FactClass }

// Kind implements Fact.
func (ArchetypeFact) Kind() FactKind { return FactArchetype }

// Kind implements Fact.
func (SizingFact) Kind() FactKind { return FactSizing }

// Kind implements Fact.
func (InitiativeFact) Kind() FactKind { return FactInitiative }

func (PotOddsFact) fact()    {}
func (EquityFact) fact()     {}
func (OutsFact) fact()       {}
func (PositionFact) fact()   {}
func (RangeFact) fact()      {}
func (ChartFact) fact()      {}
func (ClassFact) fact()      {}
func (ArchetypeFact) fact()  {}
func (SizingFact) fact()     {}
func (InitiativeFact) fact() {}

// HandClass is the made-hand strength ladder the postflop strategy reasons
// in. It is deliberately coarser than eval.Category: the teaching
// vocabulary is "top pair", not "one pair, kicker pattern 7".
type HandClass int

// Made-hand classes, weakest to strongest.
const (
	Air        HandClass = iota // no pair of our own (board-only pairs count as Air)
	WeakPair                    // bottom pair, underpairs, other weak pairs
	MiddlePair                  // second-best board rank paired (or a pocket pair between top and second)
	TopPair                     // top board rank paired with a hole card
	Overpair                    // pocket pair above the board
	TwoPair                     // two pair using at least one hole card
	TripsPlus                   // trips, straights, flushes and up
)

var handClassNames = [...]string{
	"air", "weak pair", "middle pair", "top pair", "overpair", "two pair", "trips+",
}

// String is the lowercase teaching name ("top pair").
func (h HandClass) String() string {
	if h < 0 || int(h) >= len(handClassNames) {
		return "?"
	}
	return handClassNames[h]
}

// DrawClass is the draw-strength ladder. Order encodes strength:
// a combo draw beats a flush draw beats an OESD beats a gutshot.
type DrawClass int

// Draw classes, weakest to strongest.
const (
	NoDraw    DrawClass = iota
	Gutshot             // one straight-completing rank (~4 outs)
	OESD                // open-ended or double-gutted (~8 outs)
	FlushDraw           // four to a flush using a hero card (~9 outs)
	ComboDraw           // flush draw plus a straight draw (~12+ outs)
)

var drawClassNames = [...]string{
	"no draw", "gutshot", "open-ended straight draw", "flush draw", "combo draw",
}

// String is the lowercase teaching name ("combo draw").
func (d DrawClass) String() string {
	if d < 0 || int(d) >= len(drawClassNames) {
		return "?"
	}
	return drawClassNames[d]
}
