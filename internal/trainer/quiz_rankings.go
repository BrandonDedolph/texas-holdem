package trainer

import (
	"math/rand/v2"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

// genRankings generates one rankings-speed item: two hole hands on a shared
// five-card board, "left, right, or split?". The correct answer is the
// eval comparison, never authored, and the drill carries a WinnerFact so
// the content-verification machinery can recompute it.
//
// Levels are trap shapes, produced by rejection sampling until the tag's
// predicate holds:
//
//	rank.category    the categories differ — the "obvious" read
//	rank.kicker      same category, different rank — kickers decide
//	rank.split       equal ranks — the board plays, or kickers counterfeit
//	rank.counterfeit a hand that "made" a pair whose best five ignore it
func genRankings(rng *rand.Rand, tag string) Item {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		var left, right [2]engine.Card
		var board []engine.Card
		if tag == tagRankCounterfeit {
			var ok bool
			left, right, board, ok = counterfeitDeal(rng)
			if !ok {
				continue
			}
		} else {
			cards := deal(rng, 9)
			left = [2]engine.Card{cards[0], cards[1]}
			right = [2]engine.Card{cards[2], cards[3]}
			board = cards[4:9]
		}

		lr := eval.EvalHoldem(left, board)
		rr := eval.EvalHoldem(right, board)
		switch tag {
		case tagRankCategory:
			if lr.Category() == rr.Category() {
				continue
			}
		case tagRankKicker:
			if lr.Category() != rr.Category() || lr == rr {
				continue
			}
		case tagRankSplit:
			if lr != rr {
				continue
			}
		case tagRankCounterfeit:
			if !counterfeited(left, board) && !counterfeited(right, board) {
				continue
			}
		}
		return rankingsItem(tag, left, right, board, lr, rr)
	}
	panic("trainer: rankings generation exhausted attempts for " + tag)
}

// counterfeitDeal constructs the classic counterfeit shape: a pocket pair
// under a double-paired board whose kicker also outranks it, so the pair
// the player "made" preflop never reaches their best five. Constructive
// rather than fully random because random deals hit this shape too rarely
// for a rejection loop to be honest about terminating.
func counterfeitDeal(rng *rand.Rand) (left, right [2]engine.Card, board []engine.Card, ok bool) {
	// A pocket pair Two..Nine leaves at least four higher ranks to build
	// the board from.
	pp := engine.Rank(rng.IntN(int(engine.Nine - engine.Two + 1)))
	higher := make([]engine.Rank, 0, int(engine.Ace-pp))
	for r := pp + 1; r <= engine.Ace; r++ {
		higher = append(higher, r)
	}
	rng.Shuffle(len(higher), func(i, j int) { higher[i], higher[j] = higher[j], higher[i] })
	pairA, pairB, kicker := higher[0], higher[1], higher[2]

	var used engine.CardSet
	suits := func() (a, b engine.Suit) {
		a = engine.Suit(rng.IntN(engine.NumSuits))
		for {
			b = engine.Suit(rng.IntN(engine.NumSuits))
			if b != a {
				return
			}
		}
	}
	s1, s2 := suits()
	left = [2]engine.Card{engine.MakeCard(pp, s1), engine.MakeCard(pp, s2)}
	if !take(&used, left[0]) || !take(&used, left[1]) {
		return left, right, nil, false
	}
	s1, s2 = suits()
	s3, s4 := suits()
	board = []engine.Card{
		engine.MakeCard(pairA, s1), engine.MakeCard(pairA, s2),
		engine.MakeCard(pairB, s3), engine.MakeCard(pairB, s4),
		engine.MakeCard(kicker, engine.Suit(rng.IntN(engine.NumSuits))),
	}
	for _, c := range board {
		if !take(&used, c) {
			return left, right, nil, false
		}
	}
	right = [2]engine.Card{randCard(rng, &used), randCard(rng, &used)}
	return left, right, board, true
}

// counterfeited reports whether a hand's made pair is dead weight: the hole
// cards paired something (a pocket pair, or a card pairing the board) yet
// neither hole card appears in the best five — the board plays instead.
func counterfeited(hole [2]engine.Card, board []engine.Card) bool {
	_, best := eval.Best5(hole, board)
	for _, c := range best {
		if c == hole[0] || c == hole[1] {
			return false
		}
	}
	if hole[0].Rank() == hole[1].Rank() {
		return true
	}
	for _, b := range board {
		if b.Rank() == hole[0].Rank() || b.Rank() == hole[1].Rank() {
			return true
		}
	}
	return false
}

// rankingsItem packages a verified rankings question. Split is always the
// third choice so the answer keys are stable across items.
func rankingsItem(tag string, left, right [2]engine.Card, board []engine.Card, lr, rr eval.HandRank) Item {
	leftStr := engine.CardsString(left[:])
	rightStr := engine.CardsString(right[:])
	boardStr := engine.CardsString(board)

	correct := 2
	verdict := "Split pot — both play " + lr.Describe() + "."
	switch {
	case lr > rr:
		correct, verdict = 0, "Left wins."
	case rr > lr:
		correct, verdict = 1, "Right wins."
	}
	explain := "Left makes " + lr.Describe() + "; Right makes " + rr.Describe() + ". " + verdict

	return Item{
		Drill: tutorial.Drill{
			Prompt: "Who wins at showdown?\nLeft: " + leftStr + "   Right: " + rightStr,
			Visual: &tutorial.Visual{Board: &tutorial.VisualBoard{Board: board}},
			Answer: tutorial.ChoiceAnswer{
				Choices: []string{"Left", "Right", "Split"},
				Correct: correct,
			},
			Explain: explain,
			Fact:    tutorial.WinnerFact{Board: boardStr, Hands: []string{leftStr, rightStr, ""}, Split: 2},
		},
		SkillTag: tag,
		hero:     leftStr,
		villain:  rightStr,
		board:    boardStr,
	}
}
