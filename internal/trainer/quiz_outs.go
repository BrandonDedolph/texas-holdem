package trainer

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

// genOuts generates one outs-counting item: hero plus board against a
// stated villain holding (levels 1–2) or a stated small range (level 3,
// where tainted outs need a range that can improve past the hero). The
// answer is equity.Outs' clean count, exactly — the same function the
// coach quotes, so the quiz and the coach can never disagree.
func genOuts(rng *rand.Rand, tag string) Item {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if tag == tagOutsTainted {
			it, ok := genOutsTainted(rng)
			if ok {
				return it
			}
			continue
		}
		shape, ok := drawShapeFor(rng, tag)
		if !ok {
			continue
		}
		vs := singleRange(shape.villain)
		report := equity.Outs(shape.hero, shape.board, &vs)
		if !outsShapeOK(tag, report) {
			continue
		}
		return outsItem(tag, shape.hero, shape.board, report,
			villainShown(shape.villain, shape.board),
			engine.CardsString(shape.villain[:]), false)
	}
	panic("trainer: outs generation exhausted attempts for " + tag)
}

// drawShape is a constructed hero-draw-versus-made-hand deal. The outs and
// equity generators share it: outs asks for the count, equity asks what the
// count is worth.
type drawShape struct {
	hero, villain [2]engine.Card
	board         []engine.Card
}

// drawShapeFor builds the tag's textbook draw. Constructive with a random
// skin (suits, ranks) rather than fully random, because pure rejection off
// random deals would make "pure nine-out flush draw" a needle in a haystack;
// the caller still verifies the resulting OutsReport before accepting.
func drawShapeFor(rng *rand.Rand, tag string) (drawShape, bool) {
	switch tag {
	case tagOutsFlush:
		return flushShape(rng)
	case tagOutsOESD:
		return oesdShape(rng)
	default:
		return comboShape(rng)
	}
}

// outsShapeOK is the per-tag predicate on the *computed* report — the
// construction proposes, the arithmetic disposes.
func outsShapeOK(tag string, report equity.OutsReport) bool {
	_, flush := report.ByDraw[equity.FlushDraw]
	_, oesd := report.ByDraw[equity.OESD]
	if len(report.Tainted) != 0 {
		return false
	}
	switch tag {
	case tagOutsFlush:
		return flush && !oesd && report.Count == 9
	case tagOutsOESD:
		return oesd && !flush && report.Count == 8
	default: // combo draw: full flush plus the non-flush straight cards
		return flush && oesd && report.Count == 15
	}
}

// flushShape: hero holds two of a suit, the flop has exactly two more, and
// the villain has top pair — the canonical nine-out spot.
func flushShape(rng *rand.Rand) (drawShape, bool) {
	f := engine.Suit(rng.IntN(engine.NumSuits))
	top := engine.Ten + engine.Rank(rng.IntN(int(engine.Ace-engine.Ten+1)))

	// Five distinct ranks below top: two for the hero, two board cards,
	// one villain kicker.
	below := make([]engine.Rank, 0, int(top))
	for r := engine.Two; r < top; r++ {
		below = append(below, r)
	}
	rng.Shuffle(len(below), func(i, j int) { below[i], below[j] = below[j], below[i] })
	h1, h2, b1, b2, k := below[0], below[1], below[2], below[3], below[4]

	var used engine.CardSet
	var s drawShape
	s.hero = [2]engine.Card{engine.MakeCard(h1, f), engine.MakeCard(h2, f)}
	s.board = []engine.Card{
		engine.MakeCard(b1, f),
		engine.MakeCard(b2, f),
		engine.MakeCard(top, offSuit(rng, f)),
	}
	s.villain = [2]engine.Card{
		engine.MakeCard(top, offSuit(rng, f)),
		engine.MakeCard(k, offSuit(rng, f)),
	}
	return s, claimAll(&used, s)
}

// oesdShape: hero's connectors plus two board cards form a four-straight,
// the third board card is low and disconnected, and the villain has top
// pair — the canonical eight-out spot. Suits are spread so no flush draw
// contaminates the count.
func oesdShape(rng *rand.Rand) (drawShape, bool) {
	// r..r+3 is the four-run; r-1 and r+4 complete it, so r sits in
	// Four..Ten (room below for the low card, room above for the top out).
	r := engine.Four + engine.Rank(rng.IntN(int(engine.Ten-engine.Four+1)))
	low := engine.Two + engine.Rank(rng.IntN(int(r-engine.Two-1))) // Two..r-2

	// The villain's kicker must neither block a straight out (r-1, r+4)
	// nor touch a rank already in play.
	kick, ok := pickRank(rng, r-1, r, r+1, r+2, r+3, r+4, low)
	if !ok {
		return drawShape{}, false
	}

	// Hero and board suits all distinct where it matters: no suit can
	// reach four cards, so the only draw is the straight.
	perm := rng.Perm(engine.NumSuits)
	hs1, hs2 := engine.Suit(perm[0]), engine.Suit(perm[1])
	bs := rng.Perm(engine.NumSuits)

	var used engine.CardSet
	var s drawShape
	s.hero = [2]engine.Card{engine.MakeCard(r, hs1), engine.MakeCard(r+1, hs2)}
	s.board = []engine.Card{
		engine.MakeCard(r+2, engine.Suit(bs[0])),
		engine.MakeCard(r+3, engine.Suit(bs[1])),
		engine.MakeCard(low, engine.Suit(bs[2])),
	}
	s.villain = [2]engine.Card{
		engine.MakeCard(r+3, engine.Suit(bs[2])),
		engine.MakeCard(kick, engine.Suit(bs[1])),
	}
	return s, claimAll(&used, s)
}

// comboShape: suited connectors with a two-flush, four-straight board —
// nine flush outs plus six straight outs, the fifteen-out monster the
// "rule of 4" lesson is built around.
func comboShape(rng *rand.Rand) (drawShape, bool) {
	f := engine.Suit(rng.IntN(engine.NumSuits))
	r := engine.Four + engine.Rank(rng.IntN(int(engine.Ten-engine.Four+1)))
	low := engine.Two + engine.Rank(rng.IntN(int(r-engine.Two-1)))

	// The villain's kicker must not block an out: not a straight
	// completer, not a rank already in play.
	kick, ok := pickRank(rng, r-1, r, r+1, r+2, r+3, r+4, low)
	if !ok {
		return drawShape{}, false
	}

	var used engine.CardSet
	var s drawShape
	s.hero = [2]engine.Card{engine.MakeCard(r, f), engine.MakeCard(r+1, f)}
	s.board = []engine.Card{
		engine.MakeCard(r+2, f),
		engine.MakeCard(r+3, f),
		engine.MakeCard(low, offSuit(rng, f)),
	}
	s.villain = [2]engine.Card{
		engine.MakeCard(r+3, offSuit(rng, f)),
		engine.MakeCard(kick, offSuit(rng, f)),
	}
	return s, claimAll(&used, s)
}

// genOutsTainted builds the level-3 spot: an open-ender on a two-tone board
// against a stated three-hand range whose flush draw turns the suited
// straight cards into tainted outs — "count clean outs only" made concrete.
// The shape mirrors equity's own tainted-outs test case.
func genOutsTainted(rng *rand.Rand) (Item, bool) {
	f := engine.Suit(rng.IntN(engine.NumSuits))
	// Board r+2 / r-1 in the flush suit plus a low card: the hero's r,r+1
	// bridge them into a four-run completed by r-2 or r+3.
	r := engine.Five + engine.Rank(rng.IntN(int(engine.Jack-engine.Five+1)))
	low := engine.Two + engine.Rank(rng.IntN(int(r-engine.Two-2))) // Two..r-3

	var used engine.CardSet
	hero := [2]engine.Card{
		engine.MakeCard(r+1, offSuit(rng, f)),
		engine.MakeCard(r, offSuit(rng, f)),
	}
	board := []engine.Card{
		engine.MakeCard(r+2, f),
		engine.MakeCard(r-1, f),
		engine.MakeCard(low, offSuit(rng, f)),
	}
	for _, c := range append(hero[:], board...) {
		if !take(&used, c) {
			return Item{}, false
		}
	}

	// The stated range: two top-pair hands plus a suited ace — the flush
	// draw that taints the hero's suited straight outs. Card collisions
	// between range combos and the deal fall out via take() below, which
	// fails the attempt and lets the caller resample a fresh skin.
	g1 := offSuit(rng, f)
	xf, ok := pickRank(rng, low, r-1, r-2, r, r+1, r+2, r+3, engine.Ace)
	if !ok {
		return Item{}, false
	}
	combos := [][2]engine.Card{
		{engine.MakeCard(r+2, g1), engine.MakeCard(r+1, g1)},
		{engine.MakeCard(r+3, offSuit(rng, f)), engine.MakeCard(r+2, offSuit(rng, f))},
		{engine.MakeCard(engine.Ace, f), engine.MakeCard(xf, f)},
	}
	specs := make([]string, len(combos))
	for i, c := range combos {
		if c[0] == c[1] || !take(&used, c[0]) || !take(&used, c[1]) {
			return Item{}, false
		}
		specs[i] = c[0].Code() + c[1].Code()
	}
	spec := strings.Join(specs, ", ")

	vs, err := equity.ParseRange(spec)
	if err != nil {
		return Item{}, false
	}
	report := equity.Outs(hero, board, &vs)
	if report.Count < 4 || len(report.Tainted) == 0 {
		return Item{}, false
	}
	prompt := "Villain's range: " + spec + ". Count only your CLEAN outs."
	return outsItem(tagOutsTainted, hero, board, report, prompt,
		spec, true), true
}

// outsItem packages a verified outs question.
func outsItem(tag string, hero [2]engine.Card, board []engine.Card,
	report equity.OutsReport, prompt, villain string, isRange bool) Item {

	return Item{
		Drill: tutorial.Drill{
			Prompt: prompt,
			Visual: &tutorial.Visual{Board: &tutorial.VisualBoard{
				Board: board, Hole: hero, ShowHole: true,
			}},
			Answer:  tutorial.NumericAnswer{Value: float64(report.Count), Tolerance: 0},
			Explain: outsExplain(report, len(board)),
		},
		SkillTag:       tag,
		hero:           engine.CardsString(hero[:]),
		villain:        villain,
		board:          engine.CardsString(board),
		villainIsRange: isRange,
	}
}

// villainShown phrases a stated exact holding, with its made hand named by
// the evaluator so the description can never misfile the hand.
func villainShown(villain [2]engine.Card, board []engine.Card) string {
	return "Villain shows " + engine.CardsString(villain[:]) + " (" +
		eval.EvalHoldem(villain, board).Describe() + "). How many clean outs do you have?"
}

// outsExplain renders the answer from the report and nothing else: the out
// cards, the tainted discount when present, and the rule-of-4/2 arithmetic.
func outsExplain(report equity.OutsReport, boardLen int) string {
	lines := []string{
		fmt.Sprintf("Clean outs (%d): %s.", report.Count, engine.CardsString(report.Clean)),
	}
	if len(report.Tainted) > 0 {
		lines = append(lines, fmt.Sprintf("Tainted (count half): %s - improve villain too.",
			engine.CardsString(report.Tainted)))
	}
	if boardLen == 3 {
		lines = append(lines, fmt.Sprintf("Rule of 4: %s outs x 4 ~ %.0f%% by the river.",
			trimFloat(report.Discounted), report.RuleOf4))
	} else {
		lines = append(lines, fmt.Sprintf("Rule of 2: %s outs x 2 ~ %.0f%% on the river.",
			trimFloat(report.Discounted), report.RuleOf2))
	}
	return strings.Join(lines, "\n")
}

// singleRange is the one-combo range of an exactly known villain holding.
func singleRange(v [2]engine.Card) equity.Range {
	var r equity.Range
	r.W[equity.MakeCombo(v[0], v[1])] = 1
	return r
}

// offSuit returns a random suit other than f.
func offSuit(rng *rand.Rand, f engine.Suit) engine.Suit {
	for {
		s := engine.Suit(rng.IntN(engine.NumSuits))
		if s != f {
			return s
		}
	}
}

// pickRank returns a random rank not among avoid, reporting failure when
// every rank is excluded (the caller resamples the whole shape).
func pickRank(rng *rand.Rand, avoid ...engine.Rank) (engine.Rank, bool) {
	var pool []engine.Rank
next:
	for r := engine.Two; r < engine.NumRanks; r++ {
		for _, a := range avoid {
			if r == a {
				continue next
			}
		}
		pool = append(pool, r)
	}
	if len(pool) == 0 {
		return 0, false
	}
	return pool[rng.IntN(len(pool))], true
}

// claimAll marks a shape's nine cards used, reporting false on any
// collision — a collision means the random skin repeated a card and the
// shape must be resampled.
func claimAll(used *engine.CardSet, s drawShape) bool {
	for _, c := range append(append(s.hero[:], s.board...), s.villain[:]...) {
		if !take(used, c) {
			return false
		}
	}
	return true
}

// trimFloat renders a float without trailing zeros ("7.5", "9").
func trimFloat(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }
