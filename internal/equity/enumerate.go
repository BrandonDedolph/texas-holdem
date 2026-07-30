package equity

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// This file holds the exact backends. They enumerate every possible runout
// (and, for range queries, every villain combo) and tally weighted
// outcomes. Exact is the preferred backend everywhere it fits the budget:
// a learner re-running the same spot must see the same number, and no
// confidence interval beats "this is the answer".

// liveCards returns the deck minus a dead set, in ascending card order.
func liveCards(dead engine.CardSet) []Card {
	return (engine.FullDeck() &^ dead).Cards()
}

// forEachRunout enumerates every k-card completion drawn from live, in
// lexicographic order. The runout slice is reused across calls; fn must not
// retain it. mask is the runout as a CardSet for O(1) conflict tests.
func forEachRunout(live []Card, k int, fn func(runout []Card, mask engine.CardSet)) {
	if k == 0 {
		fn(nil, 0)
		return
	}
	var buf [5]Card
	var rec func(start, depth int, mask engine.CardSet)
	rec = func(start, depth int, mask engine.CardSet) {
		if depth == k {
			fn(buf[:k], mask)
			return
		}
		for i := start; i <= len(live)-(k-depth); i++ {
			buf[depth] = live[i]
			rec(i+1, depth+1, mask.Add(live[i]))
		}
	}
	rec(0, 0, 0)
}

// exactHandVsCombos enumerates hero vs a set of weighted villain combos over
// all runouts. Combos must already be free of conflicts with hero and
// board; combos conflicting with a particular runout are skipped pair-wise.
//
// The hero's rank is evaluated once per runout and reused across all
// villain combos, so the cost is boards × (1 + combos) evals rather than
// the boards × combos × 2 the design budgeted — this is what makes "flop
// hand vs full range" comfortably exact.
func exactHandVsCombos(hero [2]Card, combos []WeightedCombo, board []Card) tally {
	dead := engine.NewCardSet(append([]Card{hero[0], hero[1]}, board...)...)
	if len(combos) == 1 {
		// A single known villain: exclude its cards from the runout deck
		// outright. Preflop this shrinks C(50,5) to C(48,5).
		dead |= comboMasks[combos[0].Combo]
	}
	live := liveCards(dead)
	need := 5 - len(board)

	var hbuf, vbuf [7]Card
	hbuf[0], hbuf[1] = hero[0], hero[1]
	copy(hbuf[2:], board)
	copy(vbuf[2:], board)

	var t tally
	forEachRunout(live, need, func(run []Card, rm engine.CardSet) {
		copy(hbuf[2+len(board):], run)
		copy(vbuf[2+len(board):], run)
		heroRank := eval.Eval7(hbuf)
		for _, wc := range combos {
			if comboMasks[wc.Combo]&rm != 0 {
				continue
			}
			cc := comboCards[wc.Combo]
			vbuf[0], vbuf[1] = cc[0], cc[1]
			vr := eval.Eval7(vbuf)
			t.addOutcome(heroRank > vr, heroRank < vr, float64(wc.Weight))
		}
	})
	return t
}

// exactMultiway enumerates every non-conflicting villain combo tuple and
// every runout. Cost is Π|ranges| × boards × (villains+1) evals; the
// planner only routes here when that fits the budget.
func exactMultiway(hero [2]Card, lists [][]WeightedCombo, board []Card) tally {
	nv := len(lists)
	dead := engine.NewCardSet(append([]Card{hero[0], hero[1]}, board...)...)
	need := 5 - len(board)

	chosen := make([][2]Card, nv)
	var buf [7]Card
	copy(buf[2:], board)

	var t tally
	var rec func(i int, mask engine.CardSet, w float64)
	rec = func(i int, mask engine.CardSet, w float64) {
		if i == nv {
			live := liveCards(mask)
			forEachRunout(live, need, func(run []Card, _ engine.CardSet) {
				copy(buf[2+len(board):], run)
				buf[0], buf[1] = hero[0], hero[1]
				heroRank := eval.Eval7(buf)
				best := eval.HandRank(0)
				atBest := 0
				for v := 0; v < nv; v++ {
					buf[0], buf[1] = chosen[v][0], chosen[v][1]
					r := eval.Eval7(buf)
					if r > best {
						best, atBest = r, 1
					} else if r == best {
						atBest++
					}
				}
				switch {
				case heroRank > best:
					t.win += w
					t.share += w
				case heroRank == best:
					t.tie += w
					t.share += w / float64(1+atBest)
				default:
					t.lose += w
				}
				t.total += w
			})
			return
		}
		for _, wc := range lists[i] {
			m := comboMasks[wc.Combo]
			if m&mask != 0 {
				continue
			}
			chosen[i] = comboCards[wc.Combo]
			rec(i+1, mask|m, w*float64(wc.Weight))
		}
	}
	rec(0, dead, 1)
	return t
}
