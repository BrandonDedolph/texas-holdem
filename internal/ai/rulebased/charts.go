// Package rulebased is the one rule-based strategy: the single source of
// strategic truth that powers every opponent archetype and the coach
// (DESIGN.md §1 principle 2). Preflop it is chart-driven; postflop it is a
// small classify-then-decide skeleton whose branches map 1:1 to lessons.
// Every branch appends the typed ai.Fact values it actually consumed, which
// is what lets the coach explain the decision truthfully.
package rulebased

import (
	"fmt"
	"math"
	"sort"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
)

// Chart is a preflop range written as ordered range-grammar terms,
// STRONGEST FIRST. The order is load-bearing: Personality.RangeScale
// widens or narrows a chart by taking a prefix of its combos, so a chart
// whose strong hands are not at the front would give a nit a garbage range.
type Chart struct {
	Name  string
	Terms []string

	combos []equity.Combo // expansion in chart order, deduplicated; built by initCharts
}

// Range returns the chart at full weight.
func (c *Chart) Range() equity.Range {
	var r equity.Range
	for _, cb := range c.combos {
		r.W[cb] = 1
	}
	return r
}

// Scaled returns the chart widened or narrowed by scale: the first
// scale × len combos of the chart, in chart (strength) order. Scales above
// 1 extend past the chart into chenOrder — the global strongest-first
// ordering of all 1326 combos — skipping combos already present, so a LAG's
// range is always a strict superset of the printed chart and a nit's a
// strict subset (design-learning.md §4.3).
func (c *Chart) Scaled(scale float64) equity.Range {
	if scale <= 0 {
		return equity.Range{}
	}
	target := int(math.Round(scale * float64(len(c.combos))))
	if target < 1 {
		target = 1
	}
	var r equity.Range
	n := 0
	for _, cb := range c.combos {
		if n >= target {
			break
		}
		r.W[cb] = 1
		n++
	}
	for _, cb := range chenOrder {
		if n >= target {
			break
		}
		if r.W[cb] == 0 {
			r.W[cb] = 1
			n++
		}
	}
	return r
}

// --- Raise-first-in charts --------------------------------------------------
//
// Standard 6-max opening ranges, written strongest-first. Approximate
// targets (design-learning.md §4.3): UTG ~15%, HJ ~18%, CO ~26%, BTN ~42%,
// SB ~40% (raise-or-fold — the baseline NEVER open-limps, because the coach
// must never recommend it). TestChartTargets pins the bands.

var rfiUTG = &Chart{Name: "UTG open", Terms: []string{
	"AA", "KK", "QQ", "JJ", "TT",
	"AKs", "AKo", "AQs", "99", "AJs", "KQs", "AQo",
	"88", "ATs", "KJs", "77", "AJo", "QJs", "KTs", "JTs",
	"66", "QTs", "55", "T9s", "44", "98s", "33", "22",
	"ATo", "KQo", "A9s", "87s",
}}

var rfiHJ = &Chart{Name: "HJ open", Terms: append(append([]string{}, rfiUTG.Terms...),
	"A8s", "KJo", "QJo", "76s", "65s", "J9s", "T8s", "A5s",
)}

var rfiCO = &Chart{Name: "CO open", Terms: append(append([]string{}, rfiHJ.Terms...),
	"A7s", "A6s", "A4s", "A3s", "A2s", "K9s", "Q9s", "54s",
	"97s", "86s", "75s", "A9o", "KTo", "QTo", "JTo", "K8s", "Q8s",
)}

var rfiBTN = &Chart{Name: "BTN open", Terms: append(append([]string{}, rfiCO.Terms...),
	"A8o", "A7o", "A6o", "A5o", "A4o", "A3o", "A2o",
	"K7s", "K6s", "K5s", "K4s", "K3s", "K2s",
	"Q7s", "Q6s", "Q5s", "J8s", "J7s", "T7s", "96s", "85s",
	"64s", "53s", "43s",
	"K9o", "Q9o", "J9o", "T9o", "98o",
)}

// SB opens raise-or-fold with a range just under the button's: position is
// worst postflop, so the bottom of the button range is trimmed.
var rfiSB = &Chart{Name: "SB open", Terms: rfiBTN.Terms[:len(rfiBTN.Terms)-3]}

// RFI maps positions to raise-first-in charts. The big blind has no RFI —
// it can only check its option or raise over limps.
var RFI = map[engine.Position]*Chart{
	engine.PosUTG: rfiUTG,
	engine.PosHJ:  rfiHJ,
	engine.PosCO:  rfiCO,
	engine.PosBTN: rfiBTN,
	engine.PosSB:  rfiSB,
}

// --- Facing an open -----------------------------------------------------------

// threeBetVsEarly is the 3-bet range against UTG/HJ opens: value-weighted
// (JJ+/AK) plus the two textbook suited-ace bluffs.
var threeBetVsEarly = &Chart{Name: "3-bet vs early open", Terms: []string{
	"AA", "KK", "QQ", "JJ", "AKs", "AKo", "A5s", "A4s",
}}

// threeBetVsLate widens against CO/BTN/SB opens, whose ranges are weaker.
var threeBetVsLate = &Chart{Name: "3-bet vs late open", Terms: []string{
	"AA", "KK", "QQ", "JJ", "TT", "AKs", "AKo", "AQs", "AQo", "KQs", "A5s", "A4s",
}}

// callVsOpen is the cold-call range in position (not the blinds): pairs for
// set value plus playable suited broadways and connectors.
var callVsOpen = &Chart{Name: "call vs open", Terms: []string{
	"TT", "99", "88", "77", "66", "55", "44", "33", "22",
	"AQs", "AJs", "ATs", "KQs", "AQo", "QJs", "KJs", "JTs", "T9s", "98s",
}}

// bbDefend is the big blind's calling range facing one open. It is the
// widest chart in the file because the BB already has one blind invested
// and closes the preflop action — the pot-odds discount is the lesson.
var bbDefend = &Chart{Name: "BB defend", Terms: []string{
	"TT", "99", "88", "AQs", "AJs", "AQo", "KQs", "ATs", "KJs", "QJs",
	"77", "66", "JTs", "KTs", "QTs", "AJo", "ATo", "KQo",
	"A9s", "A8s", "A7s", "A6s", "A5s", "A4s", "A3s", "A2s",
	"55", "44", "33", "22", "T9s", "98s", "87s", "76s", "65s", "54s",
	"K9s", "Q9s", "J9s", "T8s", "97s", "86s", "75s", "64s", "53s",
	"KJo", "QJo", "JTo", "T9o", "98o",
}}

// sbDefend is the small blind's flat range: tight, because the SB plays
// every postflop street out of position with money behind the action.
var sbDefend = &Chart{Name: "SB defend", Terms: []string{
	"TT", "99", "88", "77", "66", "55", "AQs", "AJs", "ATs", "KQs", "AQo", "QJs", "JTs",
}}

// --- Facing a 3-bet -------------------------------------------------------------

// fourBet is the raise range facing a 3-bet: premiums only at baseline.
var fourBet = &Chart{Name: "4-bet", Terms: []string{
	"AA", "KK", "QQ", "AKs", "AKo",
}}

// vsThreeBetCall continues facing a 3-bet without reopening the pot.
var vsThreeBetCall = &Chart{Name: "call vs 3-bet", Terms: []string{
	"JJ", "TT", "99", "AQs", "AJs", "KQs", "AQo", "ATs",
}}

// ThreeBet returns the 3-bet chart against an open from openerPos.
func ThreeBet(openerPos engine.Position) *Chart {
	if openerPos == engine.PosUTG || openerPos == engine.PosHJ {
		return threeBetVsEarly
	}
	return threeBetVsLate
}

// DefendCall returns the flat-call chart for heroPos facing an open.
func DefendCall(heroPos engine.Position) *Chart {
	switch heroPos {
	case engine.PosBB:
		return bbDefend
	case engine.PosSB:
		return sbDefend
	default:
		return callVsOpen
	}
}

// FourBet returns the raise range facing a 3-bet.
func FourBet() *Chart { return fourBet }

// VsThreeBetCall returns the continue range facing a 3-bet.
func VsThreeBetCall() *Chart { return vsThreeBetCall }

// allCharts enumerates every chart for init and for TestChartsParse.
func allCharts() []*Chart {
	return []*Chart{
		rfiUTG, rfiHJ, rfiCO, rfiBTN, rfiSB,
		threeBetVsEarly, threeBetVsLate, callVsOpen, bbDefend, sbDefend,
		fourBet, vsThreeBetCall,
	}
}

// chenOrder is every one of the 1326 combos ordered strongest-first by Chen
// formula score (class-level, ties broken deterministically: pairs before
// suited before offsuit, then by ranks). It is the widening fallback for
// RangeScale > 1 and the source of chenPrefix ranges — one global strength
// ordering instead of five hand-tuned "wider" chart sets.
var chenOrder []equity.Combo

// chenPct[c] is combo c's strength percentile in chenOrder: 0 for the
// strongest combo, approaching 1 for the weakest. It feeds the preflop
// ScoreBB proxy — an ordering, never quoted as an equity.
var chenPct [equity.NumCombos]float64

// chenPrefix returns the top frac (0..1] of all 1326 combos by Chen order —
// the coarse "they could have anything decent" ranges perception falls back
// to for limps and blind checks.
func chenPrefix(frac float64) equity.Range {
	if frac > 1 {
		frac = 1
	}
	n := int(math.Round(frac * float64(len(chenOrder))))
	var r equity.Range
	for _, cb := range chenOrder[:n] {
		r.W[cb] = 1
	}
	return r
}

// chenScore is the Chen formula for a 169-grid hand class — a standard,
// teachable preflop strength heuristic. It is used only to ORDER hands
// (for chart widening), never quoted as an equity.
func chenScore(hi, lo engine.Rank, suited bool) float64 {
	pts := func(r engine.Rank) float64 {
		switch r {
		case engine.Ace:
			return 10
		case engine.King:
			return 8
		case engine.Queen:
			return 7
		case engine.Jack:
			return 6
		default:
			return (float64(r) + 2) / 2
		}
	}
	s := pts(hi)
	if hi == lo {
		s *= 2
		if s < 5 {
			s = 5
		}
		return s
	}
	if suited {
		s += 2
	}
	gap := int(hi) - int(lo) - 1
	switch {
	case gap == 1:
		s -= 1
	case gap == 2:
		s -= 2
	case gap == 3:
		s -= 4
	case gap >= 4:
		s -= 5
	}
	if gap <= 1 && hi < engine.Queen {
		s++
	}
	return s
}

func init() {
	initChenOrder()
	initCharts()
}

func initChenOrder() {
	type class struct {
		hi, lo engine.Rank
		suited bool
		pair   bool
		score  float64
	}
	classes := make([]class, 0, 169)
	for hi := engine.Rank(0); hi < engine.NumRanks; hi++ {
		classes = append(classes, class{hi: hi, lo: hi, pair: true, score: chenScore(hi, hi, false)})
		for lo := engine.Rank(0); lo < hi; lo++ {
			classes = append(classes, class{hi: hi, lo: lo, suited: true, score: chenScore(hi, lo, true)})
			classes = append(classes, class{hi: hi, lo: lo, score: chenScore(hi, lo, false)})
		}
	}
	sort.SliceStable(classes, func(i, j int) bool {
		a, b := classes[i], classes[j]
		if a.score != b.score {
			return a.score > b.score
		}
		if a.pair != b.pair {
			return a.pair
		}
		if a.suited != b.suited {
			return a.suited
		}
		if a.hi != b.hi {
			return a.hi > b.hi
		}
		return a.lo > b.lo
	})
	chenOrder = make([]equity.Combo, 0, equity.NumCombos)
	for _, cl := range classes {
		chenOrder = append(chenOrder, classCombos(cl.hi, cl.lo, cl.suited)...)
	}
	if len(chenOrder) != equity.NumCombos {
		panic(fmt.Sprintf("rulebased: chen order has %d combos, want %d", len(chenOrder), equity.NumCombos))
	}
	for i, cb := range chenOrder {
		chenPct[cb] = float64(i) / float64(equity.NumCombos)
	}
}

// classCombos expands one 169-grid class into combos, deterministically.
func classCombos(hi, lo engine.Rank, suited bool) []equity.Combo {
	var out []equity.Combo
	if hi == lo {
		for s1 := engine.Suit(0); s1 < engine.NumSuits; s1++ {
			for s2 := s1 + 1; s2 < engine.NumSuits; s2++ {
				out = append(out, equity.MakeCombo(engine.MakeCard(hi, s1), engine.MakeCard(hi, s2)))
			}
		}
		return out
	}
	for s1 := engine.Suit(0); s1 < engine.NumSuits; s1++ {
		for s2 := engine.Suit(0); s2 < engine.NumSuits; s2++ {
			if suited != (s1 == s2) {
				continue
			}
			out = append(out, equity.MakeCombo(engine.MakeCard(hi, s1), engine.MakeCard(lo, s2)))
		}
	}
	return out
}

// initCharts expands every chart's terms into its ordered, deduplicated
// combo list. A term that fails to parse is a programmer error in package
// data, so it panics at init — TestChartsParse pins this at build time.
func initCharts() {
	for _, c := range allCharts() {
		if len(c.combos) > 0 {
			continue // shared-slice charts (SB) may alias an already-built one
		}
		seen := make(map[equity.Combo]bool, 256)
		for _, term := range c.Terms {
			r, err := equity.ParseRange(term)
			if err != nil {
				panic(fmt.Sprintf("rulebased: chart %q term %q: %v", c.Name, term, err))
			}
			for _, wc := range r.Combos() {
				if !seen[wc.Combo] {
					seen[wc.Combo] = true
					c.combos = append(c.combos, wc.Combo)
				}
			}
		}
	}
}
