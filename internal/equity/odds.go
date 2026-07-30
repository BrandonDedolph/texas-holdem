package equity

import (
	"fmt"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// PotOdds returns the equity required to break even on a call:
// toCall / (pot + toCall). It is deliberately defined as *required equity*
// rather than a ratio, because this is the number that gets compared
// against hand equity everywhere — the coach's core comparison is
// "your equity ~34% vs the 31% you need". The ratio form exists only for
// display; see OddsRatio.
//
// pot is the pot before the hero's call (what the hero stands to win).
func PotOdds(toCall, pot engine.Chips) float64 {
	if toCall <= 0 {
		return 0
	}
	if pot < 0 {
		pot = 0
	}
	return float64(toCall) / float64(pot+toCall)
}

// OddsRatio returns pot:toCall reduced to lowest integer terms — the
// "you're getting 3 to 1" display form players actually say. Display only:
// every decision compares equities, never ratios. Non-integer spots reduce
// exactly (calling 20 into 45 is 9:4); the display layer may round further.
func OddsRatio(toCall, pot engine.Chips) (int, int) {
	if toCall <= 0 {
		return 0, 0
	}
	if pot < 0 {
		pot = 0
	}
	a, b := int64(pot), int64(toCall)
	for d := b; d != 0; {
		a, d = d, a%d
	}
	// a is now gcd(pot, toCall); it is nonzero because toCall > 0.
	return int(int64(pot) / a), int(int64(toCall) / a)
}

// RequiredEquityText renders the pot-odds lesson sentence the coach and the
// tutorial quote verbatim: "risk 50 to win 150 → need 25%".
func RequiredEquityText(toCall, pot engine.Chips) string {
	pct := int(PotOdds(toCall, pot)*100 + 0.5)
	return fmt.Sprintf("risk %s to win %s → need %d%%", toCall, pot, pct)
}
