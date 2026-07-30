package coach

import (
	"fmt"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
)

// Grade is the five-band decision quality scale (design-learning.md §5.2).
// Bands come from EV loss over the decision's scored candidates, so two
// near-equal actions both grade well — euchre's binary matched/didn't is
// exactly what this replaces, because poker has too many close spots for a
// binary right/wrong.
type Grade int

// Grades, best to worst.
const (
	GradeBest       Grade = iota // matched the coach action (sizing within 25%)
	GradeGood                    // different action, ScoreBB within 0.25bb of best
	GradeInaccuracy              // EV loss 0.25–1bb
	GradeMistake                 // EV loss 1–3bb
	GradeBlunder                 // EV loss ≥ 3bb
)

var gradeNames = [...]string{"best", "good", "inaccuracy", "mistake", "blunder"}

// String is the lowercase band name ("inaccuracy") — also the key
// profile.GradeTotals counts under.
func (g Grade) String() string {
	if g < 0 || int(g) >= len(gradeNames) {
		return "?"
	}
	return gradeNames[g]
}

// GoodOrBetter reports whether the decision counts toward the accuracy
// statistic. Best and Good both count: close spots must never be marked
// wrong on coin flips.
func (g Grade) GoodOrBetter() bool { return g <= GradeGood }

// EV-loss band boundaries, in big blinds, and the sizing tolerance for a
// "matched" action. Package constants rather than magic numbers because the
// review screen's legend quotes them.
const (
	evGoodBB    = 0.25 // below this a different action is still Good
	evMistakeBB = 1.0  // Inaccuracy becomes Mistake
	evBlunderBB = 3.0  // Mistake becomes Blunder
	sizingTol   = 0.25 // matched sizing may differ from the pick by ±25%
)

// ViewDigest is a compact snapshot of the hero's information set at
// decision time — everything the review screen needs to show the spot,
// and nothing the hero could not see. It is built by Advise and copied
// verbatim into the GradedDecision; the grader itself never receives a
// view after the action is taken, which is half of the anti-resulting
// guarantee (the other half is that GradeAction has no outcome parameter).
type ViewDigest struct {
	Street engine.Street
	Hole   string // "9s 8s"
	Board  string // "Ks 7s 2h", "" preflop
	Pot    engine.Chips
	ToCall engine.Chips
	Blinds engine.Blinds
}

// NewViewDigest snapshots a view.
func NewViewDigest(v *engine.PlayerView) ViewDigest {
	return ViewDigest{
		Street: v.Street,
		Hole:   engine.CardsString(v.Hole[:]),
		Board:  engine.CardsString(v.Board),
		Pot:    v.Pot,
		ToCall: v.ToCall,
		Blinds: v.Blinds,
	}
}

// GradedDecision is one hero decision, graded at the moment it was
// committed and immutable afterwards. Nothing in it can encode the hand's
// outcome: every field derives from the frozen Advice and the taken action.
type GradedDecision struct {
	// HandID is assigned by the recorder when the decision is persisted
	// alongside its HandRecord (DESIGN.md §2.6); the grader leaves it
	// empty because PlayerView carries no hand identity.
	HandID string

	Street engine.Street
	Taken  engine.Action

	// Best is the highest-scoring candidate — the counterfactual EV loss
	// is measured against. Note it may differ from the coach's chosen
	// action: the branch logic picks over the raw proxy scores, and the
	// pick is the recommendation while Best anchors the loss arithmetic.
	Best ai.ScoredAction

	// EVLossBB is Best.ScoreBB − Score(Taken), clamped ≥ 0, in big
	// blinds. Scores are the strategy's EV proxy (see ai.ScoredAction):
	// comparable within this one decision, never summed across sessions
	// as if they were solver output.
	EVLossBB float64

	Grade Grade

	// Body is the explanation with the counterfactual ("Raise to 60 was
	// my pick, but calling is fine here too."). Rendered here, from
	// decision-time inputs only, so it is as outcome-proof as the grade.
	Body string

	// ViewDigest is what the hero knew, for the review screen.
	ViewDigest ViewDigest
}

// GradeAction grades the hero's committed action against the frozen
// advice. It is called synchronously the moment the hero acts — before the
// engine deals another card — and the result is never mutated.
//
// The signature is the anti-resulting principle made structural: the
// grader receives the advice (computed from the hero's view when the turn
// began) and the action, and nothing else. Opponents' cards and future
// streets are not merely ignored — they are unreachable.
//
// The grade is recorded to the profile's lifetime totals in every coach
// mode, including Off: silence is a display choice, not an amnesty.
func (c *Coach) GradeAction(adv Advice, taken engine.Action) GradedDecision {
	cands := adv.Decision.Candidates
	best := ai.Best(cands) // panics on empty — a candidate-less Decision is a strategy bug

	matched := matchesPick(taken, adv.Decision.Action)

	evLoss := best.ScoreBB - scoreFor(taken, cands)
	if evLoss < 0 {
		evLoss = 0
	}
	if matched {
		// Following the coach costs nothing, by definition. The branch logic
		// that picks the recommendation and the proxy that scores candidates
		// can disagree by a hair, and where they do the recommendation wins:
		// the app must never tell a player they leaked EV by doing exactly
		// what it advised. This keeps the grade and the review's EV ledger
		// telling the same story — a session of perfect obedience reads as
		// zero leak, which is what a learner following instructions has
		// every right to expect.
		evLoss = 0
	}

	grade := gradeFor(matched, evLoss)

	g := GradedDecision{
		Street:     adv.Digest.Street,
		Taken:      taken,
		Best:       best,
		EVLossBB:   evLoss,
		Grade:      grade,
		Body:       gradeBody(adv.Decision.Action, taken, grade, evLoss),
		ViewDigest: adv.Digest,
	}
	if c != nil && c.prof != nil {
		c.prof.RecordGrade(grade.String())
	}
	return g
}

// Feedback returns the grade line to show for a verbosity mode: Full always
// shows it, Mistakes shows it only when the decision actually missed
// (numbers are the scoreboard; opinion is reserved for leaks), Off shows
// nothing. The grade itself was recorded regardless.
func (g GradedDecision) Feedback(mode string) string {
	switch mode {
	case profile.CoachOff:
		return ""
	case profile.CoachMistakes:
		if g.Grade.GoodOrBetter() {
			return ""
		}
	}
	return g.Body
}

// gradeFor maps a (matched, EV loss) pair onto the band. Taking the
// coach's own pick is Best by definition — the recommendation is the
// standard of play, so following it can never be graded down, even where
// the raw proxy ranks another candidate a hair higher.
func gradeFor(matched bool, evLossBB float64) Grade {
	switch {
	case matched:
		return GradeBest
	case evLossBB < evGoodBB:
		return GradeGood
	case evLossBB < evMistakeBB:
		return GradeInaccuracy
	case evLossBB < evBlunderBB:
		return GradeMistake
	default:
		return GradeBlunder
	}
}

// matchesPick reports whether the taken action is the coach's pick: same
// action type, and for sized actions an amount within ±25% of the pick.
// Raising to 70 when the coach said 60 is the same decision; raising to
// 200 is not.
func matchesPick(taken, pick engine.Action) bool {
	if taken.Type() != pick.Type() {
		return false
	}
	ta, sized := actionAmount(taken)
	if !sized {
		return true
	}
	pa, _ := actionAmount(pick)
	diff := float64(ta - pa)
	if diff < 0 {
		diff = -diff
	}
	return diff <= sizingTol*float64(pa)
}

// scoreFor returns the taken action's ScoreBB: its candidate of the same
// type with the nearest amount. Every legal action class was scored by the
// strategy, so this is a lookup; the defensive fallback (an action type
// somehow absent from the slate) grades against the weakest candidate —
// pessimistic beats a panic in a live game.
func scoreFor(taken engine.Action, cands []ai.ScoredAction) float64 {
	ta, _ := actionAmount(taken)
	bestDiff := engine.Chips(-1)
	var score float64
	found := false
	for _, c := range cands {
		if c.Action.Type() != taken.Type() {
			continue
		}
		ca, _ := actionAmount(c.Action)
		diff := ca - ta
		if diff < 0 {
			diff = -diff
		}
		if !found || diff < bestDiff {
			found, bestDiff, score = true, diff, c.ScoreBB
		}
	}
	if found {
		return score
	}
	score = cands[0].ScoreBB
	for _, c := range cands[1:] {
		if c.ScoreBB < score {
			score = c.ScoreBB
		}
	}
	return score
}

// gradeBody renders the explanation with the counterfactual. Everything
// it cites — the pick, the taken action, the EV band — existed at decision
// time, so the text is byte-identical no matter how the hand ends.
func gradeBody(pick, taken engine.Action, grade Grade, evLossBB float64) string {
	switch grade {
	case GradeBest:
		return actionPhrase(pick) + " was my pick too."
	case GradeGood:
		return actionPhrase(pick) + " was my pick, but " + actionGerund(taken) + " is fine here too."
	case GradeInaccuracy:
		return fmt.Sprintf("An inaccuracy — %s gives up about %.1fbb; %s was better.",
			actionGerund(taken), evLossBB, actionPhrase(pick))
	case GradeMistake:
		return fmt.Sprintf("A mistake — %s gives up about %.1fbb; %s was the play.",
			actionGerund(taken), evLossBB, actionPhrase(pick))
	default:
		return fmt.Sprintf("A blunder — %s gives up about %.1fbb; %s was the play.",
			actionGerund(taken), evLossBB, actionPhrase(pick))
	}
}

// actionPhrase is the imperative form: "Raise to 60", "Call", "Fold".
func actionPhrase(a engine.Action) string {
	switch a.Type() {
	case engine.ActionFold:
		return "Fold"
	case engine.ActionCheck:
		return "Check"
	case engine.ActionCall:
		return "Call"
	case engine.ActionBet:
		amt, _ := actionAmount(a)
		return "Bet " + amt.String()
	default:
		amt, _ := actionAmount(a)
		return "Raise to " + amt.String()
	}
}

// actionGerund is the in-sentence form: "raising to 60", "calling".
func actionGerund(a engine.Action) string {
	switch a.Type() {
	case engine.ActionFold:
		return "folding"
	case engine.ActionCheck:
		return "checking"
	case engine.ActionCall:
		return "calling"
	case engine.ActionBet:
		amt, _ := actionAmount(a)
		return "betting " + amt.String()
	default:
		amt, _ := actionAmount(a)
		return "raising to " + amt.String()
	}
}

// actionAmount extracts the street-total amount of a sized action.
// sized is false for fold/check/call, whose identity carries no amount.
func actionAmount(a engine.Action) (amt engine.Chips, sized bool) {
	switch v := a.(type) {
	case engine.Bet:
		return v.Amount, true
	case engine.Raise:
		return v.To, true
	}
	return 0, false
}
