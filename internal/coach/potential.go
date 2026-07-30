package coach

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// PotentialLine is the one-line "what your hand can become" note the table
// shows under the hero's hole cards (table-redesign-pitch.md §2.4). Preflop
// it describes the starting hand's shape — what it flops, and how often;
// postflop it names the made hand and the draw beside it, with the draw's
// out count.
//
// Every number is computed, not recited: the preflop probabilities are
// closed-form counts over the C(50,3) = 19,600 possible flops (or a direct
// enumeration of them, for straight draws), and the postflop out counts are
// the live cards equity.Outs inventories per draw. The board must be empty,
// a flop, a turn, or a river; anything else is a caller bug and panics, the
// equity package's stance.
func PotentialLine(hole [2]engine.Card, board []engine.Card) string {
	switch len(board) {
	case 0:
		return preflopPotential(hole)
	case 3, 4:
		return postflopPotential(hole, board)
	case 5:
		made := strings.ToLower(eval.EvalHoldem(hole, board).String())
		return made + " — the board is complete, no more cards to come"
	default:
		panic(fmt.Sprintf("coach: PotentialLine needs 0, 3, 4, or 5 board cards, got %d", len(board)))
	}
}

// flops50 is C(50,3): the number of distinct flops once the hero's two hole
// cards are known.
var flops50 = choose3(50)

// choose3 is C(n,3) as a float64.
func choose3(n int) float64 { return float64(n) * float64(n-1) * float64(n-2) / 6 }

// preflopPotential classifies the starting hand's shape and quotes what it
// flops. Derivations, all over the C(50,3) flops:
//
//   - pocket pair → set: 1 − C(48,3)/C(50,3) ≈ 11.76%, "about 1 in 8.5"
//     (2 of the pair rank remain among the 50 unseen cards);
//   - pocket pair, overcard on the flop: 1 − C(50−a,3)/C(50,3) with a = 4 ×
//     (ranks above the pair); overpair flop: C(4b,3)/C(50,3) with b = ranks
//     below;
//   - suited → flush draw (exactly two more of the suit): C(11,2)×39 /
//     C(50,3) ≈ 10.94%, "about 1 in 9";
//   - unpaired → pairs a hole rank: 1 − C(44,3)/C(50,3) ≈ 32.4%, "about
//     1 in 3" (6 cards of the two hole ranks remain);
//   - open-ended straight draw: direct enumeration (oesdFlopProb).
func preflopPotential(hole [2]engine.Card) string {
	hi, lo := hole[0].Rank(), hole[1].Rank()
	if hi < lo {
		hi, lo = lo, hi
	}

	if hi == lo { // pocket pair
		setP := 1 - choose3(48)/flops50
		set := "flops a set about 1 in " + oneInText(setP)
		above := 4 * int(engine.Ace-hi)
		overcardP := 1 - choose3(50-above)/flops50
		if overcardP > 0.5 {
			return set + " — unimproved, it is often second best: an overcard flops " +
				PctText(overcardP*100) + " of the time"
		}
		overpairP := choose3(4*int(hi-engine.Two)) / flops50
		return set + " — unimproved, it is still an overpair on " +
			PctText(overpairP*100) + " of flops"
	}

	suited := hole[0].Suit() == hole[1].Suit()
	flushDrawP := 55 * 39 / flops50 // C(11,2) × C(39,1) / C(50,3)
	pairP := 1 - choose3(44)/flops50
	oesdP := oesdFlopProb(hi, lo)

	// A straight-draw clause is only worth a beginner's attention when the
	// shape actually delivers it: max-stretch connectors flop an open-ender
	// about 1 in 10, while one- and two-gappers fall off fast. 5% is the
	// cut between "part of this hand's plan" and "a happy accident".
	const oesdWorthQuoting = 0.05

	switch {
	case suited && oesdP >= oesdWorthQuoting:
		return "flops a flush draw about 1 in " + oneInText(flushDrawP) +
			" and an open-ended straight draw about 1 in " + oneInText(oesdP)
	case suited:
		return "flops a flush draw about 1 in " + oneInText(flushDrawP) +
			" — and a pair about 1 in " + oneInText(pairP) + " flops"
	case oesdP >= oesdWorthQuoting:
		return "flops an open-ended straight draw about 1 in " + oneInText(oesdP) +
			" and a pair about 1 in " + oneInText(pairP) + " flops"
	case lo >= engine.Ten:
		// Big offsuit cards: the shape's real risk is domination — both
		// players pair the same top card and the kicker settles it.
		return "hits a pair about 1 in " + oneInText(pairP) +
			" flops — big offsuit hands play one-pair pots, where the kicker decides"
	default:
		return "hits a pair about 1 in " + oneInText(pairP) + " flops — and not much else"
	}
}

// postflopPotential names the made hand plus the best draw beside it. The
// draw inventory and its live-card counts come from equity.Outs (the same
// machinery the strategy consumes), so the quoted counts are actual
// undealt cards, not folklore.
func postflopPotential(hole [2]engine.Card, board []engine.Card) string {
	made := strings.ToLower(eval.EvalHoldem(hole, board).String())
	byDraw := equity.Outs(hole, board, nil).ByDraw

	fd := byDraw[equity.FlushDraw]
	oesd := byDraw[equity.OESD]
	gut := byDraw[equity.Gutshot]
	switch {
	case len(fd) > 0 && len(oesd) > 0:
		n := unionCount(fd, oesd)
		return made + " with a flush draw plus an open-ended straight draw — " +
			strconv.Itoa(n) + " outs between them"
	case len(fd) > 0 && len(gut) > 0:
		n := unionCount(fd, gut)
		return made + " with a flush draw plus a gutshot — " +
			strconv.Itoa(n) + " outs between them"
	case len(fd) > 0:
		return made + " with a flush draw — a flush draw is " + strconv.Itoa(len(fd)) + " outs"
	case len(oesd) > 0:
		return made + " with an open-ended straight draw — an open-ender is " +
			strconv.Itoa(len(oesd)) + " outs"
	case len(gut) > 0:
		return made + " with a gutshot — a gutshot is " + strconv.Itoa(len(gut)) + " outs"
	}
	if ov := byDraw[equity.TwoOvercards]; len(ov) > 0 {
		return made + " — pairing either card makes top pair (" + strconv.Itoa(len(ov)) + " outs)"
	}
	if trips := byDraw[equity.PairToTrips]; len(trips) > 0 {
		return made + " — the " + strconv.Itoa(len(trips)) + " remaining " +
			strings.ToLower(hole[0].Rank().Plural()) + " make a set"
	}
	if tp := byDraw[equity.PairToTwoPair]; len(tp) > 0 {
		return made + " — pairing your kicker makes two pair (" + strconv.Itoa(len(tp)) + " outs)"
	}
	if _, ok := byDraw[equity.BackdoorFlush]; ok {
		return made + " — only a backdoor flush draw, needing both remaining cards"
	}
	return made + " — no draw; the hand plays as it stands"
}

// oneInText renders a probability as the "1 in N" figure beginners actually
// remember. Below 10 it keeps a half step (a set is "1 in 8.5", not "1 in
// 9"); above, a whole number — the half would be false precision.
func oneInText(p float64) string {
	n := 1 / p
	if n < 10 {
		return strconv.FormatFloat(math.Round(n*2)/2, 'f', -1, 64)
	}
	return strconv.Itoa(int(math.Round(n)))
}

// unionCount counts the distinct cards across draw out-lists (a combo
// draw's two lists overlap: the straight-completing card of the flush suit
// is in both and must count once).
func unionCount(lists ...[]engine.Card) int {
	var set engine.CardSet
	for _, l := range lists {
		set |= engine.NewCardSet(l...)
	}
	return set.Count()
}

// oesdCache memoizes oesdFlopProb per rank pair — suits do not enter a
// straight count, so hi<<4|lo is a complete key.
var (
	oesdMu    sync.Mutex
	oesdCache = map[int]float64{}
)

// oesdFlopProb is the probability that the flop gives an unpaired hand an
// open-ended (or double-gutted) straight draw without flopping a straight
// outright, by direct enumeration of all C(50,3) flops. Only ranks matter,
// so the enumeration runs over the 50 unseen cards of one canonical suit
// assignment. "Open-ended" is the same mechanical definition equity's draw
// classifier uses: two or more ranks complete a straight that uses a hole
// card (equity.classifyDraws — the mask test is duplicated here because
// that helper is unexported).
func oesdFlopProb(hi, lo engine.Rank) float64 {
	key := int(hi)<<4 | int(lo)
	oesdMu.Lock()
	defer oesdMu.Unlock()
	if p, ok := oesdCache[key]; ok {
		return p
	}

	// The 50 unseen cards: two ranks appear 3 times, the rest 4.
	var deck []engine.Rank
	for r := engine.Rank(0); r < engine.NumRanks; r++ {
		n := 4
		if r == hi || r == lo {
			n = 3
		}
		for i := 0; i < n; i++ {
			deck = append(deck, r)
		}
	}
	holeMask := uint16(1)<<hi | uint16(1)<<lo

	hits, total := 0, 0
	for i := 0; i < len(deck); i++ {
		for j := i + 1; j < len(deck); j++ {
			for k := j + 1; k < len(deck); k++ {
				total++
				boardMask := uint16(1)<<deck[i] | uint16(1)<<deck[j] | uint16(1)<<deck[k]
				full := holeMask | boardMask
				if hasStraightRankMask(full) {
					continue // flopped a straight — that is not a draw
				}
				completing := 0
				for r := engine.Rank(0); r < engine.NumRanks; r++ {
					bit := uint16(1) << r
					if hasStraightRankMask(full|bit) && !hasStraightRankMask(boardMask|bit) {
						completing++
					}
				}
				if completing >= 2 {
					hits++
				}
			}
		}
	}
	p := float64(hits) / float64(total)
	oesdCache[key] = p
	return p
}

// hasStraightRankMask reports whether a rank mask contains five consecutive
// ranks or the wheel — a duplicate of equity's unexported hasStraightMask,
// kept byte-for-byte in step with it.
func hasStraightRankMask(m uint16) bool {
	if m&(m>>1)&(m>>2)&(m>>3)&(m>>4) != 0 {
		return true
	}
	const wheel = 1<<engine.Ace | 1<<engine.Five | 1<<engine.Four | 1<<engine.Three | 1<<engine.Two
	return m&wheel == wheel
}
