package equity

import (
	"fmt"
	"sort"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// DrawType names a draw for the coach's vocabulary and the ByDraw report.
type DrawType int

// Draw types.
const (
	FlushDraw     DrawType = iota // four to a flush using a hero card
	OESD                          // open-ended (or double-gutted) straight draw: two completing ranks
	Gutshot                       // one completing rank
	TwoOvercards                  // both hole cards above the board
	PairToTrips                   // pocket pair drawing to a set
	PairToTwoPair                 // board pair + kicker drawing to two pair
	BackdoorFlush                 // three to a flush on the flop; no next-card outs
)

var drawNames = [...]string{
	"flush draw", "open-ended straight draw", "gutshot", "two overcards",
	"pair to trips", "pair to two pair", "backdoor flush draw",
}

func (d DrawType) String() string {
	if d < 0 || int(d) >= len(drawNames) {
		return "?"
	}
	return drawNames[d]
}

// OutsReport is the outs answer the coach and the quiz both quote.
type OutsReport struct {
	Clean      []Card              // improve hero to ahead of the range's majority
	Tainted    []Card              // improve hero, but a real slice of the range improves past him
	ByDraw     map[DrawType][]Card // per-draw candidate cards, unfiltered
	Count      int                 // len(Clean)
	Discounted float64             // Clean + 0.5×Tainted — what the coach quotes as "~N outs"
	RuleOf4    float64             // Discounted × 4: the flop teaching estimate
	RuleOf2    float64             // Discounted × 2: the turn teaching estimate
}

// taintFraction is the slice of the reference range that may still beat
// the hero after an out before that out counts as tainted rather than
// clean. Strictly "any combo at all" would let freak holdings (the 2♥ that
// gives a lone 22 quads under a hero flush) taint every textbook out —
// the flush draw must count 9 — while a card that leaves a fifth of the
// range beating you (the 8♣ that completes your straight and his flush)
// deserves the coach's half-out discount.
const taintFraction = 0.2

// Outs reports which next cards help the hero, against a reference range.
// vsRange defaults to "top pair or better" when nil — the standard beginner
// frame. The board must be a flop or a turn; there is no next card on the
// river.
//
// The definition is mechanical so the quiz and the coach can never
// disagree. With "ahead" meaning "beats the weighted majority of the
// range", a candidate card c is:
//
//   - an out when the hero was NOT ahead, c improves the hero's own hand
//     rank, and the hero is ahead after c;
//   - tainted (rather than clean) when, additionally, more than
//     taintFraction of the range still beats the hero after c — the card
//     that completes your straight and his flush improves you into a hand
//     that is still losing real money;
//   - nothing otherwise. A card that improves the hero without taking the
//     lead is not an out, and if the hero is already ahead there are no
//     outs to count.
func Outs(hero [2]Card, board []Card, vsRange *Range) OutsReport {
	if len(board) != 3 && len(board) != 4 {
		panic(fmt.Sprintf("equity: Outs needs a flop or turn board, got %d cards", len(board)))
	}
	checkQuery(board, hero)
	dead := engine.NewCardSet(append([]Card{hero[0], hero[1]}, board...)...)

	var ref Range
	if vsRange != nil {
		ref = vsRange.Without(append([]Card{hero[0], hero[1]}, board...)...)
	} else {
		ref = topPairOrBetter(board, dead)
	}
	combos := ref.Combos()

	report := OutsReport{ByDraw: classifyDraws(hero, board, dead)}
	if len(combos) == 0 {
		return report
	}

	heroNow := eval.EvalHoldem(hero, board)
	var beatNow, totalNow float64
	for _, wc := range combos {
		cc := comboCards[wc.Combo]
		w := float64(wc.Weight)
		if heroNow > eval.EvalHoldem([2]Card{cc[0], cc[1]}, board) {
			beatNow += w
		}
		totalNow += w
	}
	aheadNow := beatNow > totalNow/2
	if aheadNow {
		// No outs when already ahead; the draw inventory still reports.
		report.RuleOf4, report.RuleOf2 = 0, 0
		return report
	}

	next := make([]Card, len(board)+1)
	copy(next, board)
	for c := Card(0); c < engine.NumCards; c++ {
		if dead.Has(c) {
			continue
		}
		next[len(board)] = c
		heroAfter := eval.EvalHoldem(hero, next)
		if heroAfter <= heroNow {
			continue
		}
		var beatA, loseA, totalA float64
		cm := engine.NewCardSet(c)
		for _, wc := range combos {
			if comboMasks[wc.Combo]&cm != 0 {
				continue
			}
			cc := comboCards[wc.Combo]
			va := eval.EvalHoldem([2]Card{cc[0], cc[1]}, next)
			w := float64(wc.Weight)
			if heroAfter > va {
				beatA += w
			} else if va > heroAfter {
				loseA += w
			}
			totalA += w
		}
		if totalA == 0 || beatA <= totalA/2 {
			continue // improved, but not to ahead: not an out
		}
		if loseA > taintFraction*totalA {
			report.Tainted = append(report.Tainted, c)
		} else {
			report.Clean = append(report.Clean, c)
		}
	}

	report.Count = len(report.Clean)
	report.Discounted = float64(report.Count) + 0.5*float64(len(report.Tainted))
	report.RuleOf4 = report.Discounted * 4
	report.RuleOf2 = report.Discounted * 2
	return report
}

// topPairOrBetter builds the default reference range: every non-dead combo
// whose current hand on this board is at least a pair of the top board
// rank. That includes overpairs, two pair, and everything above, which is
// the standard beginner frame for "what am I drawing against?".
func topPairOrBetter(board []Card, dead engine.CardSet) Range {
	top := engine.Rank(0)
	for _, c := range board {
		if c.Rank() > top {
			top = c.Rank()
		}
	}
	threshold := topPairThreshold(top)
	var r Range
	for i := 0; i < NumCombos; i++ {
		if comboMasks[i]&dead != 0 {
			continue
		}
		cc := comboCards[i]
		if eval.EvalHoldem([2]Card{cc[0], cc[1]}, board) >= threshold {
			r.W[i] = 1
		}
	}
	return r
}

// topPairThreshold is the weakest HandRank that counts as "top pair": a
// pair of the top board rank with zero kickers. It is assembled from the
// HandRank layout documented in eval/rank.go (category in bits 23..20, k1
// in bits 19..16); TestTopPairThreshold pins that layout.
func topPairThreshold(top engine.Rank) eval.HandRank {
	return eval.HandRank(uint32(eval.OnePair)<<20 | uint32(top)<<16)
}

// classifyDraws inventories the hero's draws mechanically, range-free. The
// card lists are candidates for each named draw; clean/tainted status is
// the range comparison's job, not this one's.
func classifyDraws(hero [2]Card, board []Card, dead engine.CardSet) map[DrawType][]Card {
	byDraw := make(map[DrawType][]Card)
	heroNow := eval.EvalHoldem(hero, board)
	add := func(d DrawType, cards []Card) {
		if len(cards) > 0 {
			sort.Slice(cards, func(i, j int) bool { return cards[i] < cards[j] })
			byDraw[d] = cards
		}
	}

	// Flush draws: four of a suit between hand and board, at least one from
	// the hand (a four-flush entirely on the board is everyone's draw, not
	// the hero's). Three of a suit on the flop is the backdoor note, which
	// has no next-card outs and is reported without cards.
	var suitCount [engine.NumSuits]int
	var heroSuit [engine.NumSuits]bool
	for _, c := range hero {
		suitCount[c.Suit()]++
		heroSuit[c.Suit()] = true
	}
	for _, c := range board {
		suitCount[c.Suit()]++
	}
	for s := engine.Suit(0); s < engine.NumSuits; s++ {
		if !heroSuit[s] {
			continue
		}
		switch suitCount[s] {
		case 4:
			var outs []Card
			for r := engine.Rank(0); r < engine.NumRanks; r++ {
				if c := engine.MakeCard(r, s); !dead.Has(c) {
					outs = append(outs, c)
				}
			}
			add(FlushDraw, outs)
		case 3:
			if len(board) == 3 {
				byDraw[BackdoorFlush] = nil
			}
		}
	}

	// Straight draws: a rank completes when hand+board+rank contains a
	// straight that the board+rank alone does not (so the straight uses a
	// hero card). Two completing ranks is open-ended (or double-gutted —
	// same eight outs, same lesson), one is a gutshot.
	if heroNow.Category() < eval.Straight {
		var handBoard, boardOnly uint16
		for _, c := range hero {
			handBoard |= 1 << c.Rank()
		}
		for _, c := range board {
			handBoard |= 1 << c.Rank()
			boardOnly |= 1 << c.Rank()
		}
		var completing []engine.Rank
		for r := engine.Rank(0); r < engine.NumRanks; r++ {
			bit := uint16(1) << r
			if hasStraightMask(handBoard|bit) && !hasStraightMask(boardOnly|bit) {
				completing = append(completing, r)
			}
		}
		if len(completing) > 0 {
			var outs []Card
			for _, r := range completing {
				for s := engine.Suit(0); s < engine.NumSuits; s++ {
					if c := engine.MakeCard(r, s); !dead.Has(c) {
						outs = append(outs, c)
					}
				}
			}
			if len(completing) >= 2 {
				add(OESD, outs)
			} else {
				add(Gutshot, outs)
			}
		}
	}

	topBoard := engine.Rank(0)
	for _, c := range board {
		if c.Rank() > topBoard {
			topBoard = c.Rank()
		}
	}
	r1, r2 := hero[0].Rank(), hero[1].Rank()

	// Two overcards: an unpaired high-card hand whose both ranks beat the
	// board; pairing either would be top pair.
	if heroNow.Category() == eval.HighCard && r1 != r2 && r1 > topBoard && r2 > topBoard {
		var outs []Card
		for _, r := range []engine.Rank{r1, r2} {
			for s := engine.Suit(0); s < engine.NumSuits; s++ {
				if c := engine.MakeCard(r, s); !dead.Has(c) {
					outs = append(outs, c)
				}
			}
		}
		add(TwoOvercards, outs)
	}

	if heroNow.Category() == eval.OnePair {
		switch {
		case r1 == r2:
			// Pocket pair under an unpaired board: two cards to a set.
			var outs []Card
			for s := engine.Suit(0); s < engine.NumSuits; s++ {
				if c := engine.MakeCard(r1, s); !dead.Has(c) {
					outs = append(outs, c)
				}
			}
			add(PairToTrips, outs)
		default:
			// One hole card paired the board; the other's three siblings
			// make two pair. If the pair is entirely on the board (neither
			// hole card matches), there is no two-pair draw to name.
			kicker := engine.Rank(engine.NumRanks)
			for _, c := range board {
				if c.Rank() == r1 {
					kicker = r2
				} else if c.Rank() == r2 {
					kicker = r1
				}
			}
			if kicker < engine.NumRanks {
				var outs []Card
				for s := engine.Suit(0); s < engine.NumSuits; s++ {
					if c := engine.MakeCard(kicker, s); !dead.Has(c) {
						outs = append(outs, c)
					}
				}
				add(PairToTwoPair, outs)
			}
		}
	}
	return byDraw
}

// hasStraightMask reports whether a rank mask contains five consecutive
// ranks or the wheel (A-5).
func hasStraightMask(m uint16) bool {
	if m&(m>>1)&(m>>2)&(m>>3)&(m>>4) != 0 {
		return true
	}
	const wheel = 1<<engine.Ace | 1<<engine.Five | 1<<engine.Four | 1<<engine.Three | 1<<engine.Two
	return m&wheel == wheel
}
