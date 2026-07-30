// Package eval ranks poker hands.
//
// The evaluator is a direct bitmask + rank-histogram implementation
// (docs/design-engine.md §2): one pass builds per-suit rank masks and a rank
// histogram, then the categories are checked in strength order with explicit
// code for each. No lookup table beyond an 8 KB straight table, no
// allocations, and the result — HandRank — is self-describing, which the
// coach layer depends on.
package eval

import (
	"fmt"
	"math/bits"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// Card aliases engine.Card. eval defines no card semantics of its own; the
// alias only keeps evaluator signatures concise.
type Card = engine.Card

// Eval5 ranks exactly 5 cards. Used by lessons ("rank these five") and as the
// cross-check oracle in tests. Cards must be distinct; duplicates yield an
// undefined (but non-panicking) result.
func Eval5(cards [5]Card) HandRank { return evalCards(cards[:]) }

// Eval7 ranks the best 5-card hand within exactly 7 cards. This is the hot
// path for the equity code: it takes its input by value and never allocates.
func Eval7(cards [7]Card) HandRank { return evalCards(cards[:]) }

// EvalHoldem ranks hole cards against a 3-, 4-, or 5-card board. The coach
// calls this on every street; 5- and 6-card evaluation reuses the same core.
// It panics on any other board size — that is a caller bug, not a game state.
func EvalHoldem(hole [2]Card, board []Card) HandRank {
	if len(board) < 3 || len(board) > 5 {
		panic(fmt.Sprintf("eval: EvalHoldem needs a 3-, 4-, or 5-card board, got %d", len(board)))
	}
	var buf [7]Card
	buf[0], buf[1] = hole[0], hole[1]
	copy(buf[2:], board)
	return evalCards(buf[:2+len(board)])
}

// Best5 ranks like EvalHoldem but additionally reports WHICH five cards form
// the best hand, so the TUI and review screen can highlight them. It takes
// the slower 21-combination path via Eval5; it is called once per showdown,
// never in equity loops. The five cards come back in descending rank order.
func Best5(hole [2]Card, board []Card) (HandRank, [5]Card) {
	if len(board) < 3 || len(board) > 5 {
		panic(fmt.Sprintf("eval: Best5 needs a 3-, 4-, or 5-card board, got %d", len(board)))
	}
	var buf [7]Card
	buf[0], buf[1] = hole[0], hole[1]
	n := 2 + copy(buf[2:], board)
	all := buf[:n]

	// Sort descending so each candidate combination — and therefore the
	// returned hand — reads high card first. Card order sorts by rank
	// because the encoding is rank-major.
	for i := 1; i < n; i++ {
		for j := i; j > 0 && all[j] > all[j-1]; j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}

	var (
		best      HandRank // zero is below every real hand
		bestCards [5]Card
	)
	// Enumerate 5-subsets as bitmasks — n ≤ 7, so at most 128 mask tests.
	// Clarity wins over index gymnastics on a once-per-showdown path.
	for m := 0; m < 1<<n; m++ {
		if bits.OnesCount(uint(m)) != 5 {
			continue
		}
		var hand [5]Card
		k := 0
		for i := 0; i < n; i++ {
			if m>>i&1 == 1 {
				hand[k] = all[i]
				k++
			}
		}
		if r := Eval5(hand); r > best {
			best, bestCards = r, hand
		}
	}
	return best, bestCards
}

// evalCards is the shared core: it ranks the best five-card hand within 5, 6,
// or 7 cards. One pass builds per-suit rank masks and a rank histogram; the
// categories are then checked in strength order, returning at the first hit.
func evalCards(cards []Card) HandRank {
	var suits [engine.NumSuits]uint16
	var counts [engine.NumRanks]uint8
	for _, c := range cards {
		suits[c.Suit()] |= 1 << c.Rank()
		counts[c.Rank()]++
	}

	// Flush shortcut. With at most 7 cards, 5+ suited cards leave at most 2
	// off-suit ones, which makes quads (needs 3 off-suit cards of one rank)
	// and a full house (the suited cards are all distinct ranks, so no pair
	// remains for trips to pair with) impossible — nothing that outranks a
	// flush can coexist with one except the same-suit straight flush. So
	// this path is self-contained; the oracle test proves it exhaustively.
	for _, sm := range suits {
		if bits.OnesCount16(sm) >= 5 {
			if top := straightTop[sm]; top != noStraight {
				return pack(StraightFlush, engine.Rank(top), 0, 0, 0, 0)
			}
			k1, k2, k3, k4, k5 := top5(sm)
			return pack(Flush, k1, k2, k3, k4, k5)
		}
	}

	rankMask := suits[0] | suits[1] | suits[2] | suits[3]

	// One descending scan of the histogram finds every made group. With 7
	// cards there can be at most one quad, two trips, or three pairs; the
	// scan order guarantees trip1 > trip2 and pair1 > pair2.
	const none = -1
	quad, trip1, trip2, pair1, pair2 := none, none, none, none, none
	for r := int(engine.Ace); r >= 0; r-- {
		switch counts[r] {
		case 4:
			quad = r
		case 3:
			if trip1 == none {
				trip1 = r
			} else if trip2 == none {
				trip2 = r
			}
		case 2:
			if pair1 == none {
				pair1 = r
			} else if pair2 == none {
				pair2 = r
			}
		}
	}

	if quad != none {
		kicker := top1(rankMask &^ (1 << quad))
		return pack(FourOfAKind, engine.Rank(quad), kicker, 0, 0, 0)
	}
	if trip1 != none && (trip2 != none || pair1 != none) {
		// A second trips outranks any pair only if it is higher; with 7
		// cards both can exist (e.g. 333 22 AA), so take the max.
		pairRank := trip2
		if pair1 > pairRank {
			pairRank = pair1
		}
		return pack(FullHouse, engine.Rank(trip1), engine.Rank(pairRank), 0, 0, 0)
	}
	if top := straightTop[rankMask]; top != noStraight {
		return pack(Straight, engine.Rank(top), 0, 0, 0, 0)
	}
	if trip1 != none {
		rest := rankMask &^ (1 << trip1)
		k1 := popTop(&rest)
		k2 := popTop(&rest)
		return pack(ThreeOfAKind, engine.Rank(trip1), k1, k2, 0, 0)
	}
	if pair2 != none {
		// A possible third pair is not lost: its rank stays in rankMask and
		// competes for the kicker like any other card.
		kicker := top1(rankMask &^ (1 << pair1) &^ (1 << pair2))
		return pack(TwoPair, engine.Rank(pair1), engine.Rank(pair2), kicker, 0, 0)
	}
	if pair1 != none {
		rest := rankMask &^ (1 << pair1)
		k1 := popTop(&rest)
		k2 := popTop(&rest)
		k3 := popTop(&rest)
		return pack(OnePair, engine.Rank(pair1), k1, k2, k3, 0)
	}
	k1, k2, k3, k4, k5 := top5(rankMask)
	return pack(HighCard, k1, k2, k3, k4, k5)
}

// top1 returns the highest rank set in a non-empty rank mask.
func top1(m uint16) engine.Rank { return engine.Rank(bits.Len16(m) - 1) }

// popTop removes and returns the highest rank set in a non-empty rank mask.
func popTop(m *uint16) engine.Rank {
	r := top1(*m)
	*m &^= 1 << r
	return r
}

// top5 returns the five highest ranks set in a mask with at least five bits.
func top5(m uint16) (k1, k2, k3, k4, k5 engine.Rank) {
	k1 = popTop(&m)
	k2 = popTop(&m)
	k3 = popTop(&m)
	k4 = popTop(&m)
	k5 = popTop(&m)
	return
}
