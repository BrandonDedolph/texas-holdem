package trainer

// The trainer's whole claim to trustworthiness is that generated answers
// are computed, never authored. These tests hold that claim by regenerating
// many items and re-deriving every stated answer from eval/equity/coach —
// a trainer that teaches a wrong fact is worse than no trainer.

import (
	"fmt"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/coach"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

// forEachGeneratedItem generates sessions of every kind at every level over
// several seeds and hands each item to fn with a description for failures.
func forEachGeneratedItem(t *testing.T, kind QuizKind, fn func(t *testing.T, it Item, desc string)) {
	t.Helper()
	for level := 0; level < MaxLevel; level++ {
		for seed := int64(1); seed <= 3; seed++ {
			s := NewSessionSeeded(kind, profAtLevel(kind, level), seed)
			for i := range s.Items {
				desc := fmt.Sprintf("%s L%d seed %d item %d (%s)", kind, level+1, seed, i, s.Items[i].SkillTag)
				fn(t, s.Items[i], desc)
			}
		}
	}
}

// TestRankingsAnswersVerified: every rankings item carries a WinnerFact,
// and the fact machinery recomputes the winner with eval and compares it to
// the stated answer.
func TestRankingsAnswersVerified(t *testing.T) {
	forEachGeneratedItem(t, QuizRankings, func(t *testing.T, it Item, desc string) {
		if it.Drill.Fact == nil {
			t.Fatalf("%s: rankings item without a verification fact", desc)
		}
		if err := it.Drill.Fact.Verify(&it.Drill); err != nil {
			t.Errorf("%s: %v", desc, err)
		}
	})
}

// TestOutsAnswersVerified recomputes each item's clean-out count from its
// recorded inputs with equity.Outs and compares it to the stated answer,
// which must also be exact (tolerance 0) — outs are counted, not estimated.
func TestOutsAnswersVerified(t *testing.T) {
	forEachGeneratedItem(t, QuizOuts, func(t *testing.T, it Item, desc string) {
		num, ok := it.Drill.Answer.(tutorial.NumericAnswer)
		if !ok {
			t.Fatalf("%s: outs item answer is %T, want NumericAnswer", desc, it.Drill.Answer)
		}
		if num.Tolerance != 0 {
			t.Errorf("%s: outs must be exact, tolerance %v", desc, num.Tolerance)
		}
		report := recomputeOuts(t, it)
		if float64(report.Count) != num.Value {
			t.Errorf("%s: stated %v outs, recomputed %d", desc, num.Value, report.Count)
		}
	})
}

// TestEquityAnswersVerified: every equity item carries an EquityFact whose
// Verify recomputes the exact equity; the stated (rounded) value must land
// within a point of it.
func TestEquityAnswersVerified(t *testing.T) {
	forEachGeneratedItem(t, QuizEquity, func(t *testing.T, it Item, desc string) {
		if it.Drill.Fact == nil {
			t.Fatalf("%s: equity item without a verification fact", desc)
		}
		if err := it.Drill.Fact.Verify(&it.Drill); err != nil {
			t.Errorf("%s: %v", desc, err)
		}
		num := it.Drill.Answer.(tutorial.NumericAnswer)
		if num.Tolerance != equityTolerance {
			t.Errorf("%s: tolerance %v, want %v", desc, num.Tolerance, float64(equityTolerance))
		}
	})
}

// TestSpotAnswersVerified: the drill's marked choice is the coach's own
// pick, choices align with the grading options, and grading the pick with
// the real grader returns Best — the quiz's "correct" is anchored to the
// same coach that grades live play.
func TestSpotAnswersVerified(t *testing.T) {
	grader := coach.New(nil, 0)
	forEachGeneratedItem(t, QuizSpots, func(t *testing.T, it Item, desc string) {
		if it.Spot == nil {
			t.Fatalf("%s: spot item without grading context", desc)
		}
		choice := it.Drill.Answer.(tutorial.ChoiceAnswer)
		if len(choice.Choices) != len(it.Spot.Options) {
			t.Fatalf("%s: %d choices but %d options", desc, len(choice.Choices), len(it.Spot.Options))
		}
		pick := it.Spot.Options[choice.Correct]
		if pick.Type() != it.Spot.Advice.Decision.Action.Type() {
			t.Errorf("%s: marked choice %v is not the coach's pick %v",
				desc, pick.Type(), it.Spot.Advice.Decision.Action.Type())
		}
		if g := grader.GradeAction(it.Spot.Advice, pick); g.Grade != coach.GradeBest {
			t.Errorf("%s: the coach's own pick graded %v", desc, g.Grade)
		}
	})
}

// recomputeOuts re-derives an outs item's report from its recorded inputs:
// a single stated combo for levels 1–2, a parsed range spec for level 3.
func recomputeOuts(t *testing.T, it Item) equity.OutsReport {
	t.Helper()
	hero := engine.Holes(it.hero)
	board := engine.MustCards(it.board)
	var vs equity.Range
	if it.villainIsRange {
		r, err := equity.ParseRange(it.villain)
		if err != nil {
			t.Fatalf("recorded range %q does not parse: %v", it.villain, err)
		}
		vs = r
	} else {
		vs = singleRange(engine.Holes(it.villain))
	}
	return equity.Outs(hero, board, &vs)
}

// TestRankingsTrapsHold: the level-3 rankings tags must actually produce
// the traps they claim, verified by direct recomputation — a "split trap"
// that is not a split teaches exactly the wrong lesson.
func TestRankingsTrapsHold(t *testing.T) {
	rng := newRNG(21)
	for i := 0; i < 40; i++ {
		it := genRankings(rng, tagRankSplit)
		board := engine.MustCards(it.board)
		lr := eval.EvalHoldem(engine.Holes(it.hero), board)
		rr := eval.EvalHoldem(engine.Holes(it.villain), board)
		if lr != rr {
			t.Fatalf("split item %d: hands do not tie (%v vs %v)", i, lr, rr)
		}
		if it.Drill.Answer.(tutorial.ChoiceAnswer).Correct != 2 {
			t.Fatalf("split item %d: answer is not the split choice", i)
		}
	}
	for i := 0; i < 40; i++ {
		it := genRankings(rng, tagRankCounterfeit)
		board := engine.MustCards(it.board)
		if !anyHandCounterfeited(engine.Holes(it.hero), engine.Holes(it.villain), board) {
			t.Fatalf("counterfeit item %d: no hand is actually counterfeited", i)
		}
	}
	for i := 0; i < 40; i++ {
		it := genRankings(rng, tagRankKicker)
		board := engine.MustCards(it.board)
		lr := eval.EvalHoldem(engine.Holes(it.hero), board)
		rr := eval.EvalHoldem(engine.Holes(it.villain), board)
		if lr.Category() != rr.Category() || lr == rr {
			t.Fatalf("kicker item %d: not a same-category fight (%v vs %v)", i, lr, rr)
		}
	}
}

// anyHandCounterfeited re-derives the counterfeit predicate independently:
// some hand paired with its hole cards, yet its best five use neither.
func anyHandCounterfeited(a, b [2]engine.Card, board []engine.Card) bool {
	check := func(hole [2]engine.Card) bool {
		_, best := eval.Best5(hole, board)
		for _, c := range best {
			if c == hole[0] || c == hole[1] {
				return false
			}
		}
		if hole[0].Rank() == hole[1].Rank() {
			return true
		}
		for _, bc := range board {
			if bc.Rank() == hole[0].Rank() || bc.Rank() == hole[1].Rank() {
				return true
			}
		}
		return false
	}
	return check(a) || check(b)
}

// TestOutsTaintedProduced: level-3 outs items really contain tainted outs,
// and the stated answer counts only the clean ones.
func TestOutsTaintedProduced(t *testing.T) {
	rng := newRNG(31)
	for i := 0; i < 30; i++ {
		it := genOuts(rng, tagOutsTainted)
		report := recomputeOuts(t, it)
		if len(report.Tainted) == 0 {
			t.Fatalf("tainted item %d has no tainted outs", i)
		}
		want := float64(report.Count)
		if got := it.Drill.Answer.(tutorial.NumericAnswer).Value; got != want {
			t.Fatalf("tainted item %d: answer %v, clean count %v", i, got, want)
		}
	}
}

// TestRejectionSamplingTerminates: the constructive-plus-rejection
// generators for the trap tags must land their predicates comfortably
// within budget. Generating dozens of the rarest shapes quickly IS the
// test — exhaustion would panic, and slowness would show in test time.
func TestRejectionSamplingTerminates(t *testing.T) {
	rng := newRNG(41)
	for i := 0; i < 30; i++ {
		genRankings(rng, tagRankSplit)
		genRankings(rng, tagRankCounterfeit)
		genOuts(rng, tagOutsTainted)
		genOuts(rng, tagOutsCombo)
	}
}

// TestNamedRangesParse: a typo in a named range spec must fail loudly here,
// not at generation time.
func TestNamedRangesParse(t *testing.T) {
	for _, nr := range namedRanges {
		r, err := equity.ParseRange(nr.spec)
		if err != nil {
			t.Errorf("%s: %v", nr.name, err)
			continue
		}
		if r.CountCombos() < 30 {
			t.Errorf("%s: only %.0f combos — too thin to estimate against", nr.name, r.CountCombos())
		}
	}
}
