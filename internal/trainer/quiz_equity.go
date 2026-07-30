package trainer

import (
	"fmt"
	"math"
	"math/rand/v2"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

// namedRanges are the level-3 villain ranges, stated by name and spec so
// the player is estimating against something they could reconstruct. The
// specs are data, but nothing about the *answer* is: the equity is computed
// against the parsed range, and a test parses every spec.
var namedRanges = []struct{ name, spec string }{
	{"a tight opening range", "77+, ATs+, KQs, AJo+, KQo"},
	{"a button opening range", "22+, A2s+, K9s+, Q9s+, J9s+, T8s+, 97s+, 87s, 76s, A8o+, KTo+, QTo+, JTo"},
	{"a 3-bet range", "TT+, AQs+, AQo+, A5s"},
}

// equityTolerance and the half-credit band (equityHalfMargin, trainer.go)
// implement the design's scoring: within ±5 points scores full, ±10 half.
const equityTolerance = 5

// genEquity generates one equity-estimation item. Every item is a drawing
// spot on purpose: the skill being taught is converting outs to equity at
// table speed, so the answer screen shows the rule-of-4 (flop) or rule-of-2
// (turn) arithmetic beside the exact number — the gap between the estimate
// and the truth is the calibration lesson.
func genEquity(rng *rand.Rand, tag string) Item {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		switch tag {
		case tagEquityRange:
			if it, ok := genEquityRange(rng); ok {
				return it
			}
		default:
			if it, ok := genEquityHand(rng, tag); ok {
				return it
			}
		}
	}
	panic("trainer: equity generation exhausted attempts for " + tag)
}

// genEquityHand builds a hand-versus-shown-hand estimate on the flop (rule
// of 4) or the turn (rule of 2), reusing the outs quiz's draw shapes.
func genEquityHand(rng *rand.Rand, tag string) (Item, bool) {
	shapes := []string{tagOutsFlush, tagOutsOESD, tagOutsCombo}
	shape, ok := drawShapeFor(rng, shapes[rng.IntN(len(shapes))])
	if !ok {
		return Item{}, false
	}
	board := shape.board
	if tag == tagEquityTurn {
		used := engine.NewCardSet(append(append(shape.hero[:], shape.board...), shape.villain[:]...)...)
		board = append(append([]engine.Card{}, shape.board...), randCard(rng, &used))
	}
	vs := singleRange(shape.villain)
	report := equity.Outs(shape.hero, board, &vs)
	if report.Count < 4 {
		return Item{}, false // the turn completed the draw, or the shape degraded
	}
	res := equity.HandVsHand(shape.hero, shape.villain, board, equity.Options{})
	pct := res.Equity * 100
	if pct < 8 || pct > 60 {
		return Item{}, false
	}

	heroStr := engine.CardsString(shape.hero[:])
	villainStr := engine.CardsString(shape.villain[:])
	boardStr := engine.CardsString(board)
	prompt := "Villain shows " + villainStr + " (" +
		eval.EvalHoldem(shape.villain, board).Describe() + "). Estimate your equity (%)."

	return Item{
		Drill: tutorial.Drill{
			Prompt: prompt,
			Visual: &tutorial.Visual{Board: &tutorial.VisualBoard{
				Board: board, Hole: shape.hero, ShowHole: true,
			}},
			Answer:  tutorial.NumericAnswer{Value: math.Round(pct), Tolerance: equityTolerance},
			Explain: equityExplain(report, res, len(board)),
			Fact:    tutorial.EquityFact{Hero: heroStr, Villain: villainStr, Board: boardStr, Within: 1},
		},
		SkillTag: tag,
		hero:     heroStr,
		villain:  villainStr,
		board:    boardStr,
	}, true
}

// genEquityRange builds the level-3 estimate: the hero's draw against a
// named preflop range on a flop.
func genEquityRange(rng *rand.Rand) (Item, bool) {
	shapes := []string{tagOutsFlush, tagOutsOESD, tagOutsCombo}
	shape, ok := drawShapeFor(rng, shapes[rng.IntN(len(shapes))])
	if !ok {
		return Item{}, false
	}
	named := namedRanges[rng.IntN(len(namedRanges))]
	vs, err := equity.ParseRange(named.spec)
	if err != nil {
		// A broken spec is a programmer error; the spec test pins it, and
		// panicking here beats an infinite rejection loop.
		panic("trainer: bad named range " + named.name + ": " + err.Error())
	}
	live := vs.Without(append(append([]engine.Card{}, shape.hero[:]...), shape.board...)...)
	if live.CountCombos() < 20 {
		return Item{}, false
	}
	report := equity.Outs(shape.hero, shape.board, &vs)
	if report.Discounted < 3 {
		return Item{}, false // the range must have the hero drawing, not ahead
	}
	res := equity.HandVsRange(shape.hero, vs, shape.board, equity.Options{})
	pct := res.Equity * 100
	if pct < 8 || pct > 65 {
		return Item{}, false
	}

	heroStr := engine.CardsString(shape.hero[:])
	boardStr := engine.CardsString(shape.board)
	prompt := "Villain has " + named.name + " (" + named.spec + "). Estimate your equity (%)."

	return Item{
		Drill: tutorial.Drill{
			Prompt: prompt,
			Visual: &tutorial.Visual{Board: &tutorial.VisualBoard{
				Board: shape.board, Hole: shape.hero, ShowHole: true,
			}},
			Answer:  tutorial.NumericAnswer{Value: math.Round(pct), Tolerance: equityTolerance},
			Explain: equityExplain(report, res, len(shape.board)),
			Fact:    tutorial.EquityFact{Hero: heroStr, VillainRange: named.spec, Board: boardStr, Within: 1},
		},
		SkillTag:       tagEquityRange,
		hero:           heroStr,
		villain:        named.spec,
		board:          boardStr,
		villainIsRange: true,
	}, true
}

// equityExplain places the rule-of-4 arithmetic beside the exact number —
// the whole point of the drill is closing the gap between the two. Every
// figure comes from the OutsReport and the equity Result; nothing is
// authored.
func equityExplain(report equity.OutsReport, res equity.Result, boardLen int) string {
	var rule string
	if boardLen == 3 {
		rule = fmt.Sprintf("Rule of 4: %s outs x 4 ~ %.0f%%.", trimFloat(report.Discounted), report.RuleOf4)
	} else {
		rule = fmt.Sprintf("Rule of 2: %s outs x 2 ~ %.0f%%.", trimFloat(report.Discounted), report.RuleOf2)
	}
	return rule + fmt.Sprintf("\nExact: %.1f%% (win %.1f%%, tie %.1f%%).",
		res.Equity*100, res.Win*100, res.Tie*100)
}
