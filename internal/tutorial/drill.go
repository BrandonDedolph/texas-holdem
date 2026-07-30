package tutorial

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
)

// Drill is one checked question. The explanation is shown after answering,
// right or wrong — a wrong answer teaches more than a right one, and drills
// retry freely either way.
type Drill struct {
	Prompt  string
	Visual  *Visual // cards/board the question is about; may be nil
	Answer  Answer
	Explain string

	// Fact, when non-nil, is the machine-checkable poker claim behind the
	// stated answer. The content test verifies every Fact against
	// eval/equity, so a drill can never teach a wrong number — the design's
	// "a wrong pot-odds figure is the most damaging possible bug" applied to
	// the curriculum. Purely conceptual drills (rules, vocabulary) have no
	// Fact.
	Fact Fact
}

// Answer checks a learner's input. Implementations are forgiving about
// formatting ("2", "31", "31%") but never about substance.
type Answer interface {
	Check(input string) bool
}

// ChoiceAnswer is a multiple-choice answer: Correct indexes Choices.
// Check accepts the 1-based choice number or the choice text itself
// (case-insensitive).
type ChoiceAnswer struct {
	Choices []string
	Correct int
}

// Check implements Answer.
func (a ChoiceAnswer) Check(input string) bool {
	in := strings.TrimSpace(input)
	if n, err := strconv.Atoi(in); err == nil {
		return n-1 == a.Correct
	}
	for i, c := range a.Choices {
		if strings.EqualFold(in, c) {
			return i == a.Correct
		}
	}
	return false
}

// NumericAnswer is a number with a tolerance. Outs are exact (Tolerance 0);
// equity estimates allow ±5 — the goal is table-speed arithmetic, not a
// calculator. Check accepts an optional trailing "%".
type NumericAnswer struct {
	Value     float64
	Tolerance float64
}

// Check implements Answer.
func (a NumericAnswer) Check(input string) bool {
	in := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(input), "%"))
	v, err := strconv.ParseFloat(in, 64)
	if err != nil {
		return false
	}
	return math.Abs(v-a.Value) <= a.Tolerance
}

// OrderAnswer asks the learner to sequence Items: Correct[k] is the index
// into Items of the k-th item in the right order. Check accepts 1-based
// positions separated by spaces or commas ("2 1 3").
type OrderAnswer struct {
	Items   []string
	Correct []int
}

// Check implements Answer.
func (a OrderAnswer) Check(input string) bool {
	fields := strings.FieldsFunc(input, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\t'
	})
	if len(fields) != len(a.Correct) {
		return false
	}
	for k, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n-1 != a.Correct[k] {
			return false
		}
	}
	return true
}

// --- Facts --------------------------------------------------------------------
//
// A Fact recomputes a drill's correct answer from eval/equity and errors if
// the authored answer disagrees. Card inputs are code strings ("As Ks 2h"),
// per the repo convention that content never spells raw card values.

// Fact is a verifiable poker claim backing a drill's answer.
type Fact interface {
	Verify(d *Drill) error
}

// WinnerFact claims which of several hands wins on a completed board. Hands
// align with the drill's ChoiceAnswer.Choices; Split, if ≥ 0, is the choice
// index meaning "split pot". The winner (or the split) must be the choice
// the answer marks correct.
type WinnerFact struct {
	Board string
	Hands []string // hole cards per choice; "" for non-hand choices (e.g. "Split")
	Split int      // choice index for a split pot; -1 when absent
}

// Verify implements Fact.
func (f WinnerFact) Verify(d *Drill) error {
	choice, ok := d.Answer.(ChoiceAnswer)
	if !ok {
		return fmt.Errorf("WinnerFact needs a ChoiceAnswer, drill has %T", d.Answer)
	}
	board, err := engine.ParseCards(f.Board)
	if err != nil {
		return err
	}
	best, bestIdx, ties := eval.HandRank(0), -1, 0
	for i, hs := range f.Hands {
		if hs == "" {
			continue
		}
		cards, err := engine.ParseCards(hs)
		if err != nil {
			return err
		}
		if len(cards) != 2 {
			return fmt.Errorf("hand %d %q is not two cards", i, hs)
		}
		r := eval.EvalHoldem([2]engine.Card{cards[0], cards[1]}, board)
		switch {
		case bestIdx < 0 || r > best:
			best, bestIdx, ties = r, i, 1
		case r == best:
			ties++
		}
	}
	want := bestIdx
	if ties > 1 {
		if f.Split < 0 {
			return fmt.Errorf("hands tie but the fact declares no split choice")
		}
		want = f.Split
	}
	if choice.Correct != want {
		return fmt.Errorf("recomputed winner is choice %d, drill says %d", want, choice.Correct)
	}
	return nil
}

// RankOrderFact claims the strength order of several hands. Hands align with
// the drill's OrderAnswer.Items; each is either five cards (ranked with
// Eval5, Board must be empty) or two hole cards ranked on Board. The
// recomputed strongest-first order must equal OrderAnswer.Correct; ties are
// an authoring error.
type RankOrderFact struct {
	Board string
	Hands []string
}

// Verify implements Fact.
func (f RankOrderFact) Verify(d *Drill) error {
	order, ok := d.Answer.(OrderAnswer)
	if !ok {
		return fmt.Errorf("RankOrderFact needs an OrderAnswer, drill has %T", d.Answer)
	}
	if len(order.Correct) != len(f.Hands) {
		return fmt.Errorf("fact has %d hands, answer orders %d", len(f.Hands), len(order.Correct))
	}
	board, err := engine.ParseCards(f.Board)
	if err != nil {
		return err
	}
	ranks := make([]eval.HandRank, len(f.Hands))
	for i, hs := range f.Hands {
		cards, err := engine.ParseCards(hs)
		if err != nil {
			return err
		}
		switch len(cards) {
		case 5:
			if len(board) != 0 {
				return fmt.Errorf("hand %d is five cards but the fact also has a board", i)
			}
			ranks[i] = eval.Eval5([5]engine.Card{cards[0], cards[1], cards[2], cards[3], cards[4]})
		case 2:
			ranks[i] = eval.EvalHoldem([2]engine.Card{cards[0], cards[1]}, board)
		default:
			return fmt.Errorf("hand %d %q is neither five cards nor a hole pair", i, hs)
		}
	}
	for k, idx := range order.Correct {
		if idx < 0 || idx >= len(ranks) {
			return fmt.Errorf("answer position %d indexes hand %d, out of range", k, idx)
		}
		if k > 0 && ranks[order.Correct[k-1]] <= ranks[idx] {
			return fmt.Errorf("answer puts hand %d ahead of hand %d but it does not beat it",
				order.Correct[k-1], idx)
		}
	}
	return nil
}

// OutsFact claims the number of clean outs for a hand on a flop or turn,
// against the default "top pair or better" reference range — exactly what
// equity.Outs computes, so the drill and the coach can never disagree.
// The drill's NumericAnswer must equal the recomputed count exactly.
type OutsFact struct {
	Hero  string
	Board string
}

// Verify implements Fact.
func (f OutsFact) Verify(d *Drill) error {
	num, ok := d.Answer.(NumericAnswer)
	if !ok {
		return fmt.Errorf("OutsFact needs a NumericAnswer, drill has %T", d.Answer)
	}
	report := equity.Outs(engine.Holes(f.Hero), engine.MustCards(f.Board), nil)
	if num.Value != float64(report.Count) {
		return fmt.Errorf("recomputed %d clean outs, drill says %g", report.Count, num.Value)
	}
	if num.Tolerance != 0 {
		return fmt.Errorf("outs are exact; drill allows ±%g", num.Tolerance)
	}
	return nil
}

// PotOddsFact claims the required equity, in percent, to call Call into a
// pot of Pot (the pot before the hero's call, bet included). The same
// arithmetic prices a bluff: a bet of Call into Pot must succeed
// Call/(Pot+Call) of the time. The drill's NumericAnswer must match the
// recomputed percentage within one point (authors quote whole percents).
type PotOddsFact struct {
	Pot, Call engine.Chips
}

// Verify implements Fact.
func (f PotOddsFact) Verify(d *Drill) error {
	num, ok := d.Answer.(NumericAnswer)
	if !ok {
		return fmt.Errorf("PotOddsFact needs a NumericAnswer, drill has %T", d.Answer)
	}
	pct := equity.PotOdds(f.Call, f.Pot) * 100
	if math.Abs(num.Value-pct) > 1 {
		return fmt.Errorf("recomputed %.1f%%, drill says %g", pct, num.Value)
	}
	return nil
}

// EquityFact claims the hero's equity, in percent, against a known hand or a
// range on a board (empty board = preflop). The drill's NumericAnswer must
// be within Within points of the exact number (default 3 — authored values
// are rounded, but a stated 36 against a true 28 is a teaching bug).
type EquityFact struct {
	Hero         string
	Villain      string // exact hand, or
	VillainRange string // a range spec; exactly one of the two is set
	Board        string
	Within       float64
}

// Verify implements Fact.
func (f EquityFact) Verify(d *Drill) error {
	num, ok := d.Answer.(NumericAnswer)
	if !ok {
		return fmt.Errorf("EquityFact needs a NumericAnswer, drill has %T", d.Answer)
	}
	var board []engine.Card
	if f.Board != "" {
		board = engine.MustCards(f.Board)
	}
	var res equity.Result
	switch {
	case f.Villain != "" && f.VillainRange == "":
		res = equity.HandVsHand(engine.Holes(f.Hero), engine.Holes(f.Villain), board, equity.Options{})
	case f.VillainRange != "" && f.Villain == "":
		r, err := equity.ParseRange(f.VillainRange)
		if err != nil {
			return err
		}
		res = equity.HandVsRange(engine.Holes(f.Hero), r, board, equity.Options{})
	default:
		return fmt.Errorf("EquityFact needs exactly one of Villain and VillainRange")
	}
	within := f.Within
	if within == 0 {
		within = 3
	}
	pct := res.Equity * 100
	if math.Abs(num.Value-pct) > within {
		return fmt.Errorf("recomputed %.1f%% equity, drill says %g (±%g)", pct, num.Value, within)
	}
	return nil
}

// RangeFact claims whether a hand is inside a range, for chart-reading
// drills. InChoice and OutChoice are the ChoiceAnswer indices meaning "yes,
// it's in the range" and "no, it isn't"; the recomputed membership must pick
// the choice the answer marks correct.
type RangeFact struct {
	Spec      string // range grammar, e.g. the lesson's BTN opening range
	Hand      string // hole cards, e.g. "As 5s"
	InChoice  int
	OutChoice int
}

// Verify implements Fact.
func (f RangeFact) Verify(d *Drill) error {
	choice, ok := d.Answer.(ChoiceAnswer)
	if !ok {
		return fmt.Errorf("RangeFact needs a ChoiceAnswer, drill has %T", d.Answer)
	}
	r, err := equity.ParseRange(f.Spec)
	if err != nil {
		return err
	}
	want := f.OutChoice
	if r.Contains(engine.Holes(f.Hand)) {
		want = f.InChoice
	}
	if choice.Correct != want {
		return fmt.Errorf("range membership picks choice %d, drill says %d", want, choice.Correct)
	}
	return nil
}
