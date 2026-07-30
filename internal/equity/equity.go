package equity

import (
	"fmt"
	"math"
	"time"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// Result is the answer to an equity query.
type Result struct {
	Win, Tie, Lose float64 // fractions; Win + Tie + Lose == 1
	// Equity is the number the coach quotes: the hero's expected share of
	// the pot. Heads-up it is Win + Tie/2; multiway, ties split by the
	// number of tied players.
	Equity  float64
	Samples int  // Monte Carlo sample count; 0 when exact
	Exact   bool // true when computed by full enumeration
}

// Options tunes an equity query. The zero value is the production default.
type Options struct {
	// MaxExactEvals is the enumeration budget in hand evaluations; above
	// it the query falls back to Monte Carlo. See DefaultMaxExactEvals.
	MaxExactEvals int
	// MCSamples is the Monte Carlo sample count. The default 10k gives
	// about ±1% at 95% confidence — more precision than the coach ever
	// quotes (it says "~31%").
	MCSamples int
	// Seed drives the sampler. 0 derives a seed from the query's inputs,
	// so a replayed hand shows identical numbers without the caller
	// having to thread a seed through.
	Seed int64
	// Deadline is a hard time budget. It is honored by *planning*, not by
	// aborting: the exact/sampled decision and the sample count are cut to
	// fit the budget using a conservative per-eval cost model, so the same
	// inputs still produce the same Result on any machine. A wall-clock
	// guard backstops the model and only sacrifices determinism if the
	// estimate was wildly wrong.
	Deadline time.Duration
}

const (
	// DefaultMaxExactEvals is the exact-enumeration budget. The design
	// (design-learning.md §3.3) set 2M assuming ~100ns per eval; the real
	// evaluator benchmarks at ~32ns/op with zero allocations, so the
	// budget is widened to 4M (~130ms worst case, and the worst case —
	// preflop hand vs hand at 3.4M evals — is not a per-frame path).
	// Exact beats sampled for a teaching tool, so every path that now
	// fits is taken exactly:
	//   - preflop hand vs hand (C(48,5) boards): exact, was "table only"
	//   - flop hand vs a full 1326-combo range: exact, was "borderline"
	DefaultMaxExactEvals = 4_000_000

	// DefaultMCSamples is the Monte Carlo default per §3.2.
	DefaultMCSamples = 10_000

	// nsPerEval is the planning cost model for one Eval7 including loop
	// overhead. Measured ~32ns; padded so Deadline planning errs toward
	// finishing early.
	nsPerEval = 60

	// nsPerSampleBase is the per-sample Monte Carlo overhead (RNG, combo
	// pick, runout sampling) on top of the evals themselves.
	nsPerSampleBase = 500
)

func (o Options) withDefaults() Options {
	if o.MaxExactEvals <= 0 {
		o.MaxExactEvals = DefaultMaxExactEvals
	}
	if o.MCSamples <= 0 {
		o.MCSamples = DefaultMCSamples
	}
	return o
}

// plan decides exact vs sampled for a query whose exact cost is exactEvals
// hand evaluations and whose Monte Carlo cost is evalsPerSample per sample.
// samples is 0 when exact.
func (o Options) plan(exactEvals, evalsPerSample float64) (exact bool, samples int) {
	exact = exactEvals <= float64(o.MaxExactEvals)
	if exact && o.Deadline > 0 && time.Duration(exactEvals*nsPerEval) > o.Deadline {
		exact = false
	}
	if exact {
		return true, 0
	}
	samples = o.MCSamples
	if o.Deadline > 0 {
		nsPer := evalsPerSample*nsPerEval + nsPerSampleBase
		if cap := int(float64(o.Deadline) / nsPer); cap < samples {
			samples = max(cap, 1)
		}
	}
	return false, samples
}

// HandVsHand computes hero's equity against one known villain holding.
//
// Preflop this exact-enumerates all C(48,5) = 1,712,304 boards (~0.11s at
// the measured eval speed) rather than sampling — the design routed this to
// a precomputed table, which does not exist yet.
// TODO(preflop-table): a generated 169×169 table (design-learning.md §3.3)
// would make preflop lookups instant; until then exactness wins over speed.
func HandVsHand(hero, villain [2]Card, board []Card, opt Options) Result {
	checkQuery(board, hero, villain)
	opt = opt.withDefaults()
	combos := []WeightedCombo{{MakeCombo(villain[0], villain[1]), 1}}

	exact, samples := opt.plan(handVsCombosEvals(1, len(board)), 2)
	if exact {
		return exactHandVsCombos(hero, combos, board).result(true, 0)
	}
	if opt.Seed == 0 {
		opt.Seed = deriveSeed('h', [][2]Card{hero, villain}, board, nil)
	}
	t, n := mcHandVsCombos(hero, combos, board, samples, opt.Deadline, opt.Seed)
	return t.result(false, n)
}

// HandVsRange computes hero's equity against a weighted range. Combos that
// conflict with the hero's cards or the board are removed first; each
// surviving (combo, runout) pair is weighted by the combo weight, which is
// the standard definition of range equity.
func HandVsRange(hero [2]Card, villain Range, board []Card, opt Options) Result {
	checkQuery(board, hero)
	opt = opt.withDefaults()
	live := villain.Without(append([]Card{hero[0], hero[1]}, board...)...)
	combos := live.Combos()
	if len(combos) == 0 {
		return Result{Exact: true}
	}

	exact, samples := opt.plan(handVsCombosEvals(len(combos), len(board)), 2)
	if exact {
		return exactHandVsCombos(hero, combos, board).result(true, 0)
	}
	if opt.Seed == 0 {
		opt.Seed = deriveSeed('r', [][2]Card{hero}, board, []Range{live})
	}
	t, n := mcHandVsCombos(hero, combos, board, samples, opt.Deadline, opt.Seed)
	return t.result(false, n)
}

// HandVsRanges computes hero's equity in a multiway pot, one range per
// villain. Equity is the expected pot share (ties split N ways). Exact
// enumeration is used when the combo-tuple × runout product fits the
// budget; otherwise seeded Monte Carlo.
func HandVsRanges(hero [2]Card, villains []Range, board []Card, opt Options) Result {
	if len(villains) == 0 {
		panic("equity: HandVsRanges with no villains")
	}
	if len(villains) == 1 {
		return HandVsRange(hero, villains[0], board, opt)
	}
	checkQuery(board, hero)
	opt = opt.withDefaults()

	dead := append([]Card{hero[0], hero[1]}, board...)
	lists := make([][]WeightedCombo, len(villains))
	ranges := make([]Range, len(villains))
	tuples := 1.0
	for i, v := range villains {
		ranges[i] = v.Without(dead...)
		lists[i] = ranges[i].Combos()
		if len(lists[i]) == 0 {
			return Result{Exact: true}
		}
		tuples *= float64(len(lists[i]))
	}

	nv := len(villains)
	need := 5 - len(board)
	boards := comb(52-2-len(board)-2*nv, need)
	evals := tuples * boards * float64(nv+1)
	exact, samples := opt.plan(evals, float64(nv+1))
	if exact {
		return exactMultiway(hero, lists, board).result(true, 0)
	}
	if opt.Seed == 0 {
		opt.Seed = deriveSeed('m', [][2]Card{hero}, board, ranges)
	}
	t, n := mcMultiway(hero, lists, board, samples, opt.Deadline, opt.Seed)
	return t.result(false, n)
}

// RangeVsRange computes range A's equity against range B — a lessons-only
// query ("how does a button opening range fare against a big-blind defend?").
func RangeVsRange(a, b Range, board []Card, opt Options) Result {
	checkQuery(board)
	opt = opt.withDefaults()
	aCombos := a.Without(board...).Combos()
	bCombos := b.Without(board...).Combos()
	if len(aCombos) == 0 || len(bCombos) == 0 {
		return Result{Exact: true}
	}

	need := 5 - len(board)
	boards := comb(52-2-len(board), need)
	evals := float64(len(aCombos)) * boards * float64(1+len(bCombos))
	exact, samples := opt.plan(evals, 2)
	if exact {
		var t tally
		scratch := make([]WeightedCombo, 0, len(bCombos))
		for _, wa := range aCombos {
			am := comboMasks[wa.Combo]
			scratch = scratch[:0]
			for _, wb := range bCombos {
				if comboMasks[wb.Combo]&am == 0 {
					scratch = append(scratch, wb)
				}
			}
			if len(scratch) == 0 {
				continue
			}
			sub := exactHandVsCombos(comboCards[wa.Combo], scratch, board)
			t.add(sub, float64(wa.Weight))
		}
		return t.result(true, 0)
	}
	if opt.Seed == 0 {
		opt.Seed = deriveSeed('v', nil, board, []Range{a.Without(board...), b.Without(board...)})
	}
	t, n := mcRangeVsRange(aCombos, bCombos, board, samples, opt.Deadline, opt.Seed)
	return t.result(false, n)
}

// tally is the raw weighted outcome accumulator shared by the exact and
// Monte Carlo backends. share is the hero's pot share (a tie among k
// players contributes w/k), so Equity falls out of one division.
type tally struct {
	win, tie, lose float64
	share          float64
	total          float64
}

func (t *tally) addOutcome(heroBeats, heroLoses bool, w float64) {
	switch {
	case heroBeats:
		t.win += w
		t.share += w
	case heroLoses:
		t.lose += w
	default:
		t.tie += w
		t.share += w / 2
	}
	t.total += w
}

// add merges a sub-tally scaled by w (used to weight a per-combo tally by
// that combo's range weight).
func (t *tally) add(sub tally, w float64) {
	t.win += sub.win * w
	t.tie += sub.tie * w
	t.lose += sub.lose * w
	t.share += sub.share * w
	t.total += sub.total * w
}

func (t tally) result(exact bool, samples int) Result {
	if t.total == 0 {
		return Result{Exact: exact}
	}
	return Result{
		Win:     t.win / t.total,
		Tie:     t.tie / t.total,
		Lose:    t.lose / t.total,
		Equity:  t.share / t.total,
		Samples: samples,
		Exact:   exact,
	}
}

// handVsCombosEvals estimates the exact-enumeration cost of a hand-vs-combos
// query in hand evaluations. A single known villain excludes its cards from
// the runout deck; a range leaves them in (each combo skips conflicting
// runouts pair-wise).
func handVsCombosEvals(nCombos, boardLen int) float64 {
	need := 5 - boardLen
	if nCombos == 1 {
		return comb(52-4-boardLen, need) * 2
	}
	return comb(52-2-boardLen, need) * float64(1+nCombos)
}

// comb is C(n, k) as a float64 — estimates only, so overflow-free.
func comb(n, k int) float64 {
	if k < 0 || k > n {
		return 0
	}
	out := 1.0
	for i := 0; i < k; i++ {
		out = out * float64(n-i) / float64(i+1)
	}
	return math.Round(out)
}

// checkQuery panics on caller bugs, matching the eval package's stance: a
// malformed query is never a game state, so failing loudly beats quietly
// teaching a number computed from an impossible deal.
func checkQuery(board []Card, holes ...[2]Card) {
	switch len(board) {
	case 0, 3, 4, 5:
	default:
		panic(fmt.Sprintf("equity: board must have 0, 3, 4, or 5 cards, got %d", len(board)))
	}
	var seen engine.CardSet
	note := func(c Card) {
		if !c.Valid() {
			panic(fmt.Sprintf("equity: invalid card %d", c))
		}
		if seen.Has(c) {
			panic(fmt.Sprintf("equity: duplicate card %s in query", c.Code()))
		}
		seen = seen.Add(c)
	}
	for _, h := range holes {
		note(h[0])
		note(h[1])
	}
	for _, c := range board {
		note(c)
	}
}

// deriveSeed hashes a query's inputs (FNV-1a) so that Seed 0 still samples
// deterministically: the same replayed spot always shows the same number.
func deriveSeed(tag byte, holes [][2]Card, board []Card, ranges []Range) int64 {
	h := uint64(14695981039346656037)
	put := func(b byte) {
		h ^= uint64(b)
		h *= 1099511628211
	}
	put(tag)
	for _, hole := range holes {
		put(byte(hole[0]))
		put(byte(hole[1]))
	}
	put(0xFF)
	for _, c := range board {
		put(byte(c))
	}
	put(0xFE)
	for _, r := range ranges {
		for i, w := range r.W {
			if w > 0 {
				put(byte(i))
				put(byte(i >> 8))
				b := math.Float32bits(w)
				put(byte(b))
				put(byte(b >> 8))
				put(byte(b >> 16))
				put(byte(b >> 24))
			}
		}
		put(0xFD)
	}
	if h == 0 { // 0 means "derive", so never return it
		h = 1
	}
	return int64(h)
}
