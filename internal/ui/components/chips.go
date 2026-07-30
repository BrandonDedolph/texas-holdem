package components

import (
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
)

// MaxPileChips is the tallest pile ChipScale will aim for. Piles exist to
// make a ratio visible at a glance ("call 2 to win 3"), and past a handful
// of chips the eye counts instead of compares.
const MaxPileChips = 5

// chipPileHardCap bounds a pile drawn with a caller-supplied scale, so a
// degenerate scale can never flood a row with chips.
const chipPileHardCap = 2 * MaxPileChips

// ChipScale picks one chips-per-glyph scale for a set of amounts that must
// be commensurable (the pot and the call price). It is the largest common
// divisor of the amounts, raised if needed so the tallest pile stays
// within MaxPileChips: 45 and 30 share a scale of 15 (piles of 3 and 2),
// 900 and 600 a scale of 300 (again 3 and 2), while coprime amounts fall
// back to ceil(max/MaxPileChips). Zero and negative amounts are ignored;
// the scale is always at least 1.
func ChipScale(amounts ...engine.Chips) engine.Chips {
	var g, max engine.Chips
	for _, a := range amounts {
		if a <= 0 {
			continue
		}
		g = gcd(g, a)
		if a > max {
			max = a
		}
	}
	if max <= 0 {
		return 1
	}
	if min := ceilDiv(max, MaxPileChips); g < min {
		g = min
	}
	return g
}

// ChipPile renders a proportional chip pile for an amount at a given
// scale (chips per glyph), e.g. "ooo 45". It is a pure function of amount
// and scale; the caller supplies one shared scale (see ChipScale) so the
// pot and call piles are commensurable. Any positive amount draws at
// least one chip; a non-positive amount renders nothing.
func ChipPile(amount, scale engine.Chips) string {
	if amount <= 0 {
		return ""
	}
	if scale <= 0 {
		scale = 1
	}
	n := ceilDiv(amount, scale)
	if n > chipPileHardCap {
		n = chipPileHardCap
	}
	g := theme.G
	th := theme.Current
	return th.SeatBet.Render(strings.Repeat(g.Chip, int(n))) + " " +
		th.SeatBet.Render(amount.String())
}

func gcd(a, b engine.Chips) engine.Chips {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func ceilDiv(a, b engine.Chips) engine.Chips {
	return (a + b - 1) / b
}
