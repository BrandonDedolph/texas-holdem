// Package equity computes the numbers the coach quotes: hand-vs-range
// equity, outs, and pot odds (docs/design-learning.md §3).
//
// Everything here is either exact enumeration or deterministic, seeded
// sampling. That is a teaching requirement, not a performance choice: a
// learner who replays the same spot must see the same number, and the quiz
// and the coach must never disagree about what an "out" is.
package equity

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// Card aliases engine.Card, like the eval package does: equity defines no
// card semantics of its own, the alias only keeps signatures concise.
type Card = engine.Card

// NumCombos is the number of unordered two-card starting hands: C(52,2).
const NumCombos = 1326

// Combo is one of the 1326 unordered two-card starting hands, encoded as an
// index into the canonical C(52,2) ordering: for cards lo < hi (numeric Card
// order), the index is hi*(hi-1)/2 + lo. The encoding exists so a Range can
// be a flat weight array rather than a map.
type Combo uint16

// MakeCombo builds a Combo from two distinct cards, in either order.
func MakeCombo(a, b Card) Combo {
	if a == b {
		panic("equity: MakeCombo with two identical cards")
	}
	if a > b {
		a, b = b, a
	}
	return Combo(uint16(b)*(uint16(b)-1)/2 + uint16(a))
}

// comboCards and comboMasks invert the Combo encoding once, at init, because
// every equity loop needs them and the arithmetic inverse (a square root) is
// not worth doing 1326 times per query. comboCards is high card first.
var (
	comboCards [NumCombos][2]Card
	comboMasks [NumCombos]engine.CardSet
)

func init() {
	for hi := Card(1); hi < engine.NumCards; hi++ {
		for lo := Card(0); lo < hi; lo++ {
			c := MakeCombo(lo, hi)
			comboCards[c] = [2]Card{hi, lo}
			comboMasks[c] = engine.NewCardSet(hi, lo)
		}
	}
}

// Cards returns the combo's two cards, high card first.
func (c Combo) Cards() [2]Card { return comboCards[c] }

// Mask returns the combo's two cards as a CardSet, for conflict tests.
func (c Combo) Mask() engine.CardSet { return comboMasks[c] }

// String is the exact-combo notation the range grammar accepts: "AhKh".
func (c Combo) String() string {
	cc := comboCards[c]
	return cc[0].Code() + cc[1].Code()
}

// WeightedCombo is one nonzero entry of a Range.
type WeightedCombo struct {
	Combo  Combo
	Weight float32
}

// Range is a weighted set of starting-hand combos. Weight 0 = not in range,
// 1 = always, fractional weights supported ("flats AQo half the time").
//
// The representation is deliberately 1326 combo weights and not a 169-cell
// grid: card removal ("he can't have AA when I hold an ace") only works at
// combo granularity. The grid is a view — see Grid.
type Range struct {
	W [NumCombos]float32
}

// Combos returns the nonzero entries in ascending Combo order. The order is
// deterministic so that everything downstream (enumeration, sampling,
// serialization) is deterministic.
func (r Range) Combos() []WeightedCombo {
	out := make([]WeightedCombo, 0, 64)
	for i, w := range r.W {
		if w > 0 {
			out = append(out, WeightedCombo{Combo(i), w})
		}
	}
	return out
}

// Without returns the range with every combo containing a dead card removed.
// This is the card-removal operation the whole package exists for: callers
// pass the hero's hole cards and the board.
func (r Range) Without(dead ...Card) Range {
	m := engine.NewCardSet(dead...)
	for i := range r.W {
		if r.W[i] > 0 && comboMasks[i]&m != 0 {
			r.W[i] = 0
		}
	}
	return r
}

// CountCombos returns the weighted combo count (a full range is 1326).
func (r Range) CountCombos() float64 {
	var sum float64
	for _, w := range r.W {
		sum += float64(w)
	}
	return sum
}

// Percent returns the range's share of all 1326 combos, 0–100. This is the
// number a range chart quotes ("a 15% opening range").
func (r Range) Percent() float64 {
	return r.CountCombos() / NumCombos * 100
}

// Contains reports whether the exact holding is in the range at any weight.
func (r Range) Contains(hole [2]Card) bool {
	return r.W[MakeCombo(hole[0], hole[1])] > 0
}

// Grid is the 13×13 range-chart view: row and column 0 are Aces, 12 are
// Twos. The diagonal is pairs, above the diagonal (col > row) is suited,
// below is offsuit — the layout every poker chart uses. Each cell is the
// mean weight of its member combos, so a full cell reads 1 and a cell
// halved by card removal reads 0.5.
type Grid [13][13]float32

// Grid returns the chart view of the range. It is a view for the UI and the
// lesson content only; computation always happens on the combo weights.
func (r Range) Grid() Grid {
	var g Grid
	for row := 0; row < 13; row++ {
		for col := 0; col < 13; col++ {
			r1 := engine.Rank(12 - row)
			r2 := engine.Rank(12 - col)
			var combos []Combo
			switch {
			case row == col:
				combos = pairCombos(r1)
			case row < col: // upper right: suited, first rank is the row
				combos = suitedCombos(r1, r2)
			default: // lower left: offsuit, first rank is the column
				combos = offsuitCombos(r2, r1)
			}
			var sum float32
			for _, c := range combos {
				sum += r.W[c]
			}
			g[row][col] = sum / float32(len(combos))
		}
	}
	return g
}

// pairCombos returns the 6 combos of a pocket pair.
func pairCombos(r engine.Rank) []Combo {
	out := make([]Combo, 0, 6)
	for s1 := engine.Suit(0); s1 < engine.NumSuits; s1++ {
		for s2 := s1 + 1; s2 < engine.NumSuits; s2++ {
			out = append(out, MakeCombo(engine.MakeCard(r, s1), engine.MakeCard(r, s2)))
		}
	}
	return out
}

// suitedCombos returns the 4 suited combos of two distinct ranks.
func suitedCombos(hi, lo engine.Rank) []Combo {
	out := make([]Combo, 0, 4)
	for s := engine.Suit(0); s < engine.NumSuits; s++ {
		out = append(out, MakeCombo(engine.MakeCard(hi, s), engine.MakeCard(lo, s)))
	}
	return out
}

// offsuitCombos returns the 12 offsuit combos of two distinct ranks.
func offsuitCombos(hi, lo engine.Rank) []Combo {
	out := make([]Combo, 0, 12)
	for s1 := engine.Suit(0); s1 < engine.NumSuits; s1++ {
		for s2 := engine.Suit(0); s2 < engine.NumSuits; s2++ {
			if s1 == s2 {
				continue
			}
			out = append(out, MakeCombo(engine.MakeCard(hi, s1), engine.MakeCard(lo, s2)))
		}
	}
	return out
}
