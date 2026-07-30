package coach

import (
	"math"
	"strconv"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
)

// PricePiles is the chip-pile rendering of a call price (table redesign,
// Direction H grafted into E): the pot and the call drawn as two small
// piles whose heights are the odds, with the plain-language ratio beside
// them — ●●● 45 / ●● 30 / "call 2 to win 3 — need 40%". This function
// returns only the pieces; the caller owns glyphs, colour, and layout.
type PricePiles struct {
	PotPile  int     // chip glyphs to draw for the pot
	CallPile int     // chip glyphs to draw for the call
	Phrase   string  // "call 2 to win 3 — need 40%"
	Required float64 // break-even equity, 0..1 (equity.PotOdds)
}

// maxPile is the tallest pile PilePrice draws.
//
// How the scale was chosen: the pile encodes the pot:call *ratio*, never
// the chip count, so 45:30 and 900:600 both reduce to ●●●/●● and read
// identically — the whole point of the graft. The common teaching prices
// all reduce small (a pot-sized bet is 2:1, half-pot 3:1, third-pot 4:1;
// facing a standard open it is 3:2), so piles up to 6 render the exact
// reduced ratio, covering 5:1 through 1:1 and 5:2/5:3-style odd sizes.
// Beyond that a human cannot count glyphs at a glance anyway, so awkward
// chip counts (47 into 30 does not reduce) fall back to an approximate
// pile scaled to 5 high while the Phrase carries the exact price.
const maxPile = 6

// PilePrice computes the chip-pile price of a call. pot is the pot before
// the call — what the hero stands to win — matching equity.PotOdds. Pure
// and rendering-free: piles are counts, the phrase is plain text.
//
// The phrase quotes the exact reduced ratio when it is small enough to say
// out loud ("call 2 to win 3"), and normalizes to one otherwise ("call 1
// to win 1.6"), mirroring equity.OddsToOne's display policy. The required
// percentage is always exact (rounded to whole points, the app-wide rule).
func PilePrice(toCall, pot engine.Chips) PricePiles {
	if toCall <= 0 {
		return PricePiles{Phrase: "no bet to call"}
	}
	if pot < 0 {
		pot = 0
	}
	req := equity.PotOdds(toCall, pot)
	need := " — need " + PctText(req*100)

	potN, callN := equity.OddsRatio(toCall, pot)
	if potN <= maxPile && callN <= maxPile {
		return PricePiles{
			PotPile:  potN,
			CallPile: callN,
			Phrase:   "call " + strconv.Itoa(callN) + " to win " + strconv.Itoa(potN) + need,
			Required: req,
		}
	}

	// Ratio too gnarly to draw exactly: scale the larger pile to 5 and the
	// other proportionally (at least 1 — a nonzero amount never vanishes).
	big := pot
	if toCall > big {
		big = toCall
	}
	scale := func(amount engine.Chips) int {
		if amount <= 0 {
			return 0
		}
		n := int(math.Round(5 * float64(amount) / float64(big)))
		if n < 1 {
			n = 1
		}
		return n
	}
	return PricePiles{
		PotPile:  scale(pot),
		CallPile: scale(toCall),
		Phrase: "call 1 to win " + strconv.FormatFloat(
			math.Round(10*float64(pot)/float64(toCall))/10, 'f', -1, 64) + need,
		Required: req,
	}
}
