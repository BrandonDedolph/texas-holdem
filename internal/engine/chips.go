package engine

import "strconv"

// Chips is a chip count. Whole chips only — blinds are small integers, so
// there is no fractional bookkeeping anywhere in the engine.
//
// Signed, so that per-hand net results (won − invested) are representable
// directly. The invariant sum(stacks) + sum(contributions) == constant holds
// after every applied action.
type Chips int64

// String renders a chip count with thousands separators: "1,240".
func (c Chips) String() string {
	n := int64(c)
	neg := n < 0
	if neg {
		n = -n
	}
	digits := strconv.FormatInt(n, 10)

	lead := len(digits) % 3
	if lead == 0 {
		lead = 3
	}
	out := digits[:lead]
	for i := lead; i < len(digits); i += 3 {
		out += "," + digits[i:i+3]
	}
	if neg {
		return "-" + out
	}
	return out
}

// Blinds are the table's forced bets.
type Blinds struct {
	Small, Big Chips
}

// BB renders a chip count in big blinds to one decimal place: "12.5bb".
// Coaching text speaks in big blinds; the table speaks in chips.
func (b Blinds) BB(c Chips) string {
	if b.Big <= 0 {
		return c.String()
	}
	tenths := (int64(c)*10 + int64(b.Big)/2) / int64(b.Big)
	whole, frac := tenths/10, tenths%10
	if frac < 0 {
		frac = -frac
	}
	if frac == 0 {
		return strconv.FormatInt(whole, 10) + "bb"
	}
	return strconv.FormatInt(whole, 10) + "." + strconv.FormatInt(frac, 10) + "bb"
}
