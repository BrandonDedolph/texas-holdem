package rulebased

import (
	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// Classification is the postflop summary the strategy reasons over: what
// the hero has made and what the hero is drawing to.
//
// Design note: design-learning.md §4.3 bundled an equity.OutsReport into
// this struct. It is kept separate here because Classify is also the inner
// loop of range perception (called per combo, ~1300 times per street), and
// an outs report costs a full range scan. The strategy computes the
// OutsReport once per decision, against the same perceived range the
// equity uses, and emits it as its own ai.OutsFact.
type Classification struct {
	Made ai.HandClass
	Draw ai.DrawClass
	Rank eval.HandRank
}

// Classify summarizes hole cards on a board (3–5 cards).
func Classify(hole [2]engine.Card, board []engine.Card) Classification {
	rank := eval.EvalHoldem(hole, board)
	return Classification{
		Made: madeClass(hole, board, rank),
		Draw: drawClass(hole, board, rank),
		Rank: rank,
	}
}

// madeClass maps a full HandRank down to the teaching ladder. The board
// participation rules matter: a pair (or two pair) sitting entirely on the
// board is everyone's hand, so the hero's class is what the hero's own
// cards add — Air if they add nothing.
func madeClass(hole [2]engine.Card, board []engine.Card, rank eval.HandRank) ai.HandClass {
	cat := rank.Category()
	r1, r2 := hole[0].Rank(), hole[1].Rank()
	pocket := r1 == r2

	top, second := topTwoBoardRanks(board)

	switch {
	case cat >= eval.ThreeOfAKind:
		return ai.TripsPlus

	case cat == eval.TwoPair:
		// Two pair counts only when a hole card is part of it; a paired
		// board plus a lone board pair the hero merely plays is not "two
		// pair" in any sense worth betting on.
		hiPair, loPair := rankKicker(rank, 0), rankKicker(rank, 1)
		if pocket && (r1 == hiPair || r1 == loPair) {
			// Pocket pair over/under a board pair: plays like the pocket
			// pair it is, not like a made two pair.
			switch {
			case r1 > top:
				return ai.Overpair
			case r1 > second:
				return ai.MiddlePair
			default:
				return ai.WeakPair
			}
		}
		if r1 == hiPair || r1 == loPair || r2 == hiPair || r2 == loPair {
			return ai.TwoPair
		}
		return ai.Air

	case cat == eval.OnePair:
		pair := rankKicker(rank, 0)
		if pocket {
			switch {
			case r1 > top:
				return ai.Overpair
			case r1 > second:
				return ai.MiddlePair
			default:
				return ai.WeakPair
			}
		}
		if pair != r1 && pair != r2 {
			return ai.Air // the pair is entirely on the board
		}
		switch pair {
		case top:
			return ai.TopPair
		case second:
			return ai.MiddlePair
		default:
			return ai.WeakPair
		}

	default:
		return ai.Air
	}
}

// rankKicker extracts kicker field i of a HandRank using the documented
// layout (eval/rank.go: category in bits 23..20, k1 in 19..16, 4 bits per
// field). TestRankKickerLayout pins the layout.
func rankKicker(r eval.HandRank, i uint) engine.Rank {
	return engine.Rank(uint32(r) >> (16 - 4*i) & 0xF)
}

// topTwoBoardRanks returns the highest and second-highest distinct board
// ranks (second == top when the board has only one distinct rank).
func topTwoBoardRanks(board []engine.Card) (top, second engine.Rank) {
	hasTop, hasSecond := false, false
	for _, c := range board {
		r := c.Rank()
		switch {
		case !hasTop:
			top, hasTop = r, true
		case r > top:
			second, hasSecond = top, true
			top = r
		case r == top:
			// duplicate of top: not a distinct second rank
		case !hasSecond || r > second:
			second, hasSecond = r, true
		}
	}
	if !hasSecond {
		second = top
	}
	return top, second
}

// drawClass inventories the hero's draws on a flop or turn. It is
// deliberately range-free and cheap (bitmask arithmetic only) because
// perception runs it per combo; the definitions match equity.Outs' draw
// classifier, and TestDrawClassMatchesOuts pins the agreement.
func drawClass(hole [2]engine.Card, board []engine.Card, rank eval.HandRank) ai.DrawClass {
	if len(board) != 3 && len(board) != 4 {
		return ai.NoDraw // no cards to come preflop or on the river
	}
	if rank.Category() >= eval.Straight {
		return ai.NoDraw // already there; nothing worth calling a draw
	}

	flush := hasFlushDraw(hole, board)
	straight := straightDrawClass(hole, board)

	switch {
	case flush && straight != ai.NoDraw:
		return ai.ComboDraw
	case flush:
		return ai.FlushDraw
	default:
		return straight
	}
}

// hasFlushDraw reports four to a flush using at least one hero card. A
// four-flush entirely on the board is everyone's draw, not the hero's.
func hasFlushDraw(hole [2]engine.Card, board []engine.Card) bool {
	var count [engine.NumSuits]int
	var heroHas [engine.NumSuits]bool
	for _, c := range hole {
		count[c.Suit()]++
		heroHas[c.Suit()] = true
	}
	for _, c := range board {
		count[c.Suit()]++
	}
	for s := engine.Suit(0); s < engine.NumSuits; s++ {
		if heroHas[s] && count[s] == 4 {
			return true
		}
	}
	return false
}

// straightDrawClass counts straight-completing ranks that need a hero card:
// two or more completing ranks is open-ended (or double-gutted — same eight
// outs, same lesson), one is a gutshot. This matches equity.Outs.
func straightDrawClass(hole [2]engine.Card, board []engine.Card) ai.DrawClass {
	var handBoard, boardOnly uint16
	for _, c := range hole {
		handBoard |= 1 << c.Rank()
	}
	for _, c := range board {
		handBoard |= 1 << c.Rank()
		boardOnly |= 1 << c.Rank()
	}
	completing := 0
	for r := engine.Rank(0); r < engine.NumRanks; r++ {
		bit := uint16(1) << r
		if straightInMask(handBoard|bit) && !straightInMask(boardOnly|bit) {
			completing++
		}
	}
	switch {
	case completing >= 2:
		return ai.OESD
	case completing == 1:
		return ai.Gutshot
	default:
		return ai.NoDraw
	}
}

// straightInMask reports five consecutive ranks or the wheel in a rank mask.
func straightInMask(m uint16) bool {
	if m&(m>>1)&(m>>2)&(m>>3)&(m>>4) != 0 {
		return true
	}
	const wheel = 1<<engine.Ace | 1<<engine.Five | 1<<engine.Four | 1<<engine.Three | 1<<engine.Two
	return m&wheel == wheel
}

// boardDry is the c-bet dryness test (design-learning.md §4.3): no flush
// draw possible, unpaired, and at most one high (9+) or connected card —
// the boards where one bet takes it down often enough that c-betting air
// is profitable, which is exactly the lesson the c-bet branch teaches.
func boardDry(board []engine.Card) bool {
	var suits [engine.NumSuits]int
	var ranks [engine.NumRanks]int
	high := 0
	for _, c := range board {
		suits[c.Suit()]++
		ranks[c.Rank()]++
		if c.Rank() >= engine.Nine {
			high++
		}
	}
	for _, n := range suits {
		if n >= 2 {
			return false // flush draw possible
		}
	}
	for _, n := range ranks {
		if n >= 2 {
			return false // paired
		}
	}
	if high > 1 {
		return false
	}
	// Connected: any two board cards within two ranks of each other put
	// straight draws in both ranges.
	for i := 0; i < len(board); i++ {
		for j := i + 1; j < len(board); j++ {
			d := int(board[i].Rank()) - int(board[j].Rank())
			if d < 0 {
				d = -d
			}
			if d <= 2 {
				return false
			}
		}
	}
	return true
}
