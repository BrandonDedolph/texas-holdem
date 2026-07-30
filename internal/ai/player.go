// Package ai defines the strategic layer's contracts: the Player that
// occupies a seat, the Strategy that decides, the typed Rationale facts a
// decision emits while deciding, and the Personality parameter blocks that
// turn one baseline strategy into a table of distinct opponents.
//
// Two of the app's three non-negotiable principles (docs/DESIGN.md §1) live
// here structurally:
//
//   - One source of strategic truth. There is exactly one rule-based
//     Strategy implementation (internal/ai/rulebased); archetypes are
//     Personality values over it, never separate bots. The coach is the TAG
//     preset running the same code the opponents run.
//   - The strategy explains itself. Strategy.Decide returns typed facts
//     gathered as part of deciding — the coach renders those facts and
//     nothing else, so it can never cite a number the decision did not
//     actually consume.
package ai

import "github.com/BrandonDedolph/texas-holdem/internal/engine"

// Player is a seat-occupying agent. Act receives an engine.PlayerView — the
// seat's complete information set with opponents' hole cards physically
// absent — and nothing else. That single-parameter signature is the no-cheat
// property: there is no side channel through which an implementation could
// see cards the view does not contain.
type Player interface {
	Act(v *engine.PlayerView) engine.Action
	Name() string
}

// Lineup wires Players to seats for a hand or a session. A nil entry (or an
// absent seat) is the human player, who acts through the UI instead.
type Lineup map[engine.Seat]Player

// Player returns the agent seated at seat, if any.
func (l Lineup) Player(seat engine.Seat) (Player, bool) {
	p, ok := l[seat]
	if !ok || p == nil {
		return nil, false
	}
	return p, true
}
