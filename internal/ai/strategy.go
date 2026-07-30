package ai

import "github.com/BrandonDedolph/texas-holdem/internal/engine"

// Strategy is the pluggable brain. It returns not just an action but the
// full scored candidate set and the typed reasoning — which is what makes
// the same object usable as an opponent (take Decision.Action) and as the
// coach (render Decision.Rationale, grade the hero's action against
// Decision.Candidates).
type Strategy interface {
	Decide(v *engine.PlayerView) Decision
}

// Decision is one resolved decision point.
type Decision struct {
	// Action is the chosen action, amount included. It always appears among
	// Candidates (same type, same amount), so grading the chosen action is a
	// lookup, never a recomputation.
	Action engine.Action

	// Candidates scores every legal action class: fold, check/call, and up
	// to three discretized raise sizes (small / standard / large from the
	// sizing table, clamped to the legal min/max), plus all-in when the
	// stack is shallow enough that the standard raise commits it anyway.
	// The coach grades the hero's action by its EV-loss band against the
	// best of these.
	Candidates []ScoredAction

	// Rationale holds the typed facts the decision actually consumed,
	// ordered by salience. Every branch of the strategy appends the facts
	// it used; the coach renders these and may not cite anything else.
	Rationale Rationale
}

// ScoredAction scores one candidate action.
//
// ScoreBB is an EV *proxy* in big blinds — NOT a solved expected value
// (that would require solving the game). It is a consistent, monotone
// heuristic assembled from the strategy's own arithmetic: pot share from
// the decision's cached equity, minus chips risked, plus a coarse
// fold-equity estimate for aggressive lines. Two properties are guaranteed
// and nothing more:
//
//   - Scores within one Decision are comparable — a higher score is the
//     strategy's better candidate, and EV-loss bands (best − taken) are
//     meaningful.
//   - The same view and seed always produce the same scores.
//
// Scores from different Decisions are NOT comparable; do not sum or
// average them across spots.
type ScoredAction struct {
	Action  engine.Action
	ScoreBB float64
}

// Best returns the highest-scoring candidate. It panics on an empty slice —
// a Decision without candidates is a strategy bug, never a game state.
func Best(candidates []ScoredAction) ScoredAction {
	if len(candidates) == 0 {
		panic("ai: Best on empty candidate list")
	}
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.ScoreBB > best.ScoreBB {
			best = c
		}
	}
	return best
}
