package eval

import "github.com/BrandonDedolph/texas-holdem/internal/engine"

// noStraight marks a rank mask that contains no straight.
const noStraight = 0xFF

// straightTop maps a 13-bit rank mask to the top rank of the best straight
// within it, or noStraight. 8 KB, built in init() in microseconds — the only
// table the evaluator uses. Indexing it with a single suit's mask detects
// straight flushes; indexing it with the combined mask detects straights.
var straightTop [1 << engine.NumRanks]uint8

func init() {
	// The wheel is the one straight the sliding-window check below cannot
	// see, because its Ace plays low.
	const wheel = 1<<engine.Ace | 1<<engine.Five | 1<<engine.Four |
		1<<engine.Three | 1<<engine.Two

	for mask := 0; mask < len(straightTop); mask++ {
		best := uint8(noStraight)
		if mask&wheel == wheel {
			best = uint8(engine.Five)
		}
		// Ascending scan so the highest straight in the mask wins,
		// overwriting the wheel if both are present.
		for hi := engine.Six; hi <= engine.Ace; hi++ {
			run := (uint16(1)<<5 - 1) << (hi - 4)
			if uint16(mask)&run == run {
				best = uint8(hi)
			}
		}
		straightTop[mask] = best
	}
}
