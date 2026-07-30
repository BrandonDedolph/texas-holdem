package equity

import (
	"math/rand/v2"
	"sort"
	"time"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// This file holds the seeded Monte Carlo backends — the fallback, never the
// default (design-learning.md §3.3). Every sampler is driven by an explicit
// seed (derived from the query inputs when the caller passes 0), so a
// replayed hand shows identical numbers.

// pcgStream is the fixed second PCG seed word; the query seed supplies the
// first. Fixed so that one int64 fully determines the stream.
const pcgStream = 0x9E3779B97F4A7C15

// mcDeadlineStride is how many samples run between wall-clock checks. The
// planner already cut the sample count to fit the deadline; this guard only
// fires when the cost model was wildly wrong, trading determinism for not
// freezing the UI.
const mcDeadlineStride = 1024

// cumWeights builds a cumulative weight table for weighted combo picks.
func cumWeights(combos []WeightedCombo) []float64 {
	cum := make([]float64, len(combos))
	sum := 0.0
	for i, wc := range combos {
		sum += float64(wc.Weight)
		cum[i] = sum
	}
	return cum
}

// pickCombo draws a combo index proportionally to weight.
func pickCombo(rng *rand.Rand, cum []float64) int {
	x := rng.Float64() * cum[len(cum)-1]
	i := sort.SearchFloat64s(cum, x)
	if i == len(cum) { // x landed exactly on the total; clamp
		i--
	}
	return i
}

// sampleRunout fills run with `need` distinct cards drawn from live that
// avoid the used mask, by rejection — need is at most 5, so rejection is
// cheaper than reshuffling.
func sampleRunout(rng *rand.Rand, live []Card, used engine.CardSet, run []Card) {
	for k := range run {
		for {
			c := live[rng.IntN(len(live))]
			if !used.Has(c) {
				used = used.Add(c)
				run[k] = c
				break
			}
		}
	}
}

// mcHandVsCombos samples hero vs a weighted combo set: pick a combo by
// weight, complete the board, evaluate. Combos are drawn before the runout,
// which matches the exact backend's pair weighting because every combo has
// the same number of compatible runouts.
func mcHandVsCombos(hero [2]Card, combos []WeightedCombo, board []Card, samples int, deadline time.Duration, seed int64) (tally, int) {
	rng := rand.New(rand.NewPCG(uint64(seed), pcgStream))
	cum := cumWeights(combos)
	dead := engine.NewCardSet(append([]Card{hero[0], hero[1]}, board...)...)
	if len(combos) == 1 {
		dead |= comboMasks[combos[0].Combo]
	}
	live := liveCards(dead)
	need := 5 - len(board)
	run := make([]Card, need)

	var hbuf, vbuf [7]Card
	hbuf[0], hbuf[1] = hero[0], hero[1]
	copy(hbuf[2:], board)
	copy(vbuf[2:], board)

	var t tally
	start := time.Now()
	done := 0
	for ; done < samples; done++ {
		if deadline > 0 && done%mcDeadlineStride == 0 && time.Since(start) > deadline {
			break
		}
		wc := combos[pickCombo(rng, cum)]
		cc := comboCards[wc.Combo]
		sampleRunout(rng, live, comboMasks[wc.Combo], run)
		copy(hbuf[2+len(board):], run)
		copy(vbuf[2+len(board):], run)
		vbuf[0], vbuf[1] = cc[0], cc[1]
		hr, vr := eval.Eval7(hbuf), eval.Eval7(vbuf)
		t.addOutcome(hr > vr, hr < vr, 1)
	}
	return t, done
}

// mcMultiway samples a full villain lineup per trial: each villain's combo
// is drawn independently by weight and the whole tuple is rejected on any
// card conflict, which keeps the sampled distribution proportional to the
// product of weights — the same joint the exact backend enumerates.
func mcMultiway(hero [2]Card, lists [][]WeightedCombo, board []Card, samples int, deadline time.Duration, seed int64) (tally, int) {
	rng := rand.New(rand.NewPCG(uint64(seed), pcgStream))
	nv := len(lists)
	cums := make([][]float64, nv)
	for i, l := range lists {
		cums[i] = cumWeights(l)
	}
	dead := engine.NewCardSet(append([]Card{hero[0], hero[1]}, board...)...)
	live := liveCards(dead)
	need := 5 - len(board)
	run := make([]Card, need)
	chosen := make([][2]Card, nv)

	var buf [7]Card
	copy(buf[2:], board)

	var t tally
	start := time.Now()
	done := 0
	// Attempts are capped so mutually exclusive ranges (every tuple
	// conflicts) terminate; the caller then sees a zero-total Result.
	for attempts := 0; done < samples && attempts < samples*50; attempts++ {
		if deadline > 0 && attempts%mcDeadlineStride == 0 && time.Since(start) > deadline {
			break
		}
		mask := dead
		ok := true
		for i := 0; i < nv; i++ {
			wc := lists[i][pickCombo(rng, cums[i])]
			m := comboMasks[wc.Combo]
			if m&mask != 0 {
				ok = false
				break
			}
			mask |= m
			chosen[i] = comboCards[wc.Combo]
		}
		if !ok {
			continue
		}
		sampleRunout(rng, live, mask&^dead, run)
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
			t.win++
			t.share++
		case heroRank == best:
			t.tie++
			t.share += 1 / float64(1+atBest)
		default:
			t.lose++
		}
		t.total++
		done++
	}
	return t, done
}

// mcRangeVsRange samples a combo from each range independently by weight,
// rejecting conflicting pairs (same joint distribution as the exact
// backend), then completes the board.
func mcRangeVsRange(aCombos, bCombos []WeightedCombo, board []Card, samples int, deadline time.Duration, seed int64) (tally, int) {
	rng := rand.New(rand.NewPCG(uint64(seed), pcgStream))
	aCum, bCum := cumWeights(aCombos), cumWeights(bCombos)
	boardMask := engine.NewCardSet(board...)
	live := liveCards(boardMask)
	need := 5 - len(board)
	run := make([]Card, need)

	var abuf, bbuf [7]Card
	copy(abuf[2:], board)
	copy(bbuf[2:], board)

	var t tally
	start := time.Now()
	done := 0
	for attempts := 0; done < samples && attempts < samples*50; attempts++ {
		if deadline > 0 && attempts%mcDeadlineStride == 0 && time.Since(start) > deadline {
			break
		}
		wa := aCombos[pickCombo(rng, aCum)]
		wb := bCombos[pickCombo(rng, bCum)]
		am, bm := comboMasks[wa.Combo], comboMasks[wb.Combo]
		if am&bm != 0 {
			continue
		}
		sampleRunout(rng, live, am|bm, run)
		ac, bc := comboCards[wa.Combo], comboCards[wb.Combo]
		abuf[0], abuf[1] = ac[0], ac[1]
		bbuf[0], bbuf[1] = bc[0], bc[1]
		copy(abuf[2+len(board):], run)
		copy(bbuf[2+len(board):], run)
		ar, br := eval.Eval7(abuf), eval.Eval7(bbuf)
		t.addOutcome(ar > br, ar < br, 1)
		done++
	}
	return t, done
}
