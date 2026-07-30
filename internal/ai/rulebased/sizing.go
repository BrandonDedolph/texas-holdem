package rulebased

import (
	"math"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// SizeTier discretizes the continuous no-limit sizing space into the three
// candidates every Decision scores (design-learning.md §4.1): small,
// standard, large. All-in appears as a fourth candidate when the stack is
// shallow enough that the standard raise commits it anyway.
type SizeTier int

// Size tiers.
const (
	SizeSmall SizeTier = iota
	SizeStandard
	SizeLarge
)

// Baseline sizing table (design-learning.md §4.3):
//
//   - opens 2.5bb + 1bb per limper (small 2.2bb, large 3.5bb)
//   - 3-bets 3× the open in position, 4× out of position
//   - c-bets 50% pot on dry boards, 66% on wet
//   - value bets 66–75% pot; river value 75%
//   - postflop raises ~2.8× the bet faced
//
// Everything funnels through roundHuman so bots never bet 137 into 400.

// openTo returns the raise-to size of an open (or isolation raise over
// limpers) for the given tier, in chips.
func openTo(tier SizeTier, bb engine.Chips, limpers int) engine.Chips {
	base := 2.5
	switch tier {
	case SizeSmall:
		base = 2.2
	case SizeLarge:
		base = 3.5
	}
	return engine.Chips(math.Round((base + float64(limpers)) * float64(bb)))
}

// threeBetTo returns the raise-to size facing a raise to openAmount:
// a multiple of the open, larger out of position because the price of
// playing the pot OOP is worse (design-learning.md §4.3).
func threeBetTo(tier SizeTier, openAmount engine.Chips, inPosition bool) engine.Chips {
	var mult float64
	switch tier {
	case SizeSmall:
		mult = 2.5
	case SizeStandard:
		mult = 3
	case SizeLarge:
		mult = 4
	}
	if !inPosition {
		mult++
	}
	return engine.Chips(math.Round(mult * float64(openAmount)))
}

// fourBetTo returns the raise-to size facing a 3-bet to threeBetAmount.
// 4-bets are smaller multiples than 3-bets — the pot is already bloated.
func fourBetTo(tier SizeTier, threeBetAmount engine.Chips) engine.Chips {
	var mult float64
	switch tier {
	case SizeSmall:
		mult = 2.1
	case SizeStandard:
		mult = 2.4
	case SizeLarge:
		mult = 2.8
	}
	return engine.Chips(math.Round(mult * float64(threeBetAmount)))
}

// betFraction returns the pot fraction for a postflop bet tier. The river's
// standard is bigger (0.75 vs 0.66) because there are no cards to come:
// river bets are pure value or pure bluff, and both want the fuller size.
func betFraction(tier SizeTier, street engine.Street) float64 {
	switch tier {
	case SizeSmall:
		return 0.5
	case SizeLarge:
		if street == engine.River {
			return 1.2
		}
		return 1.0
	default:
		if street == engine.River {
			return 0.75
		}
		return 0.66
	}
}

// betAmount returns the chip amount of a postflop bet as a pot fraction,
// human-rounded.
func betAmount(frac float64, pot, bb engine.Chips) engine.Chips {
	return roundHuman(engine.Chips(math.Round(frac*float64(pot))), bb)
}

// raiseTo returns the raise-to size facing a postflop bet: a multiple of
// the price, so the opponent's draw is charged relative to what they bet.
func raiseTo(tier SizeTier, facing engine.Chips) engine.Chips {
	var mult float64
	switch tier {
	case SizeSmall:
		mult = 2.2
	case SizeStandard:
		mult = 2.8
	case SizeLarge:
		mult = 3.5
	}
	return engine.Chips(math.Round(mult * float64(facing)))
}

// preferredTier is the tier a personality reaches for when it bets or
// raises: standard at baseline, large when Patience is gone. This is the
// only place Patience is consulted, so the maniac's oversizing is one dial.
func preferredTier(p float64) SizeTier {
	if p <= 0.4 {
		return SizeLarge
	}
	return SizeStandard
}

// roundHuman rounds a chip amount to a size a person would slide forward.
// A bot betting 137 into 400 reads as a computer instantly; 140 does not.
// The grid coarsens with the amount:
//
//   - under 10bb: half big blinds, so the idiomatic 2.5bb open survives
//   - 10bb to 25bb: whole big blinds
//   - over 25bb: five big blinds — nobody raises "to 137"
//
// The result is never rounded below one big blind.
func roundHuman(c, bb engine.Chips) engine.Chips {
	if bb <= 0 {
		return c
	}
	var step engine.Chips
	switch {
	case c < 10*bb:
		step = bb / 2
		if step < 1 {
			step = 1
		}
	case c < 25*bb:
		step = bb
	default:
		step = 5 * bb
	}
	rounded := (c + step/2) / step * step
	if rounded < bb {
		rounded = bb
	}
	return rounded
}

// clampSize clamps a desired amount into an option's legal [Min, Max],
// re-rounding when the clamp moved it — except onto the exact bounds,
// which are legal by definition (Max is usually all-in).
func clampSize(want engine.Chips, opt engine.ActionOption, bb engine.Chips) engine.Chips {
	if want <= opt.Min {
		return opt.Min
	}
	if want >= opt.Max {
		return opt.Max
	}
	r := roundHuman(want, bb)
	if r < opt.Min {
		return opt.Min
	}
	if r > opt.Max {
		return opt.Max
	}
	return r
}
