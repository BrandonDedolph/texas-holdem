// Package coach turns the strategy layer into a teacher. It recommends an
// action for the hero's seat, renders the recommendation from the typed
// facts the decision consumed, grades the hero's chosen action the moment
// it is committed, and surfaces once-per-player teachable moments.
//
// Two of the app's non-negotiable principles (docs/DESIGN.md §1) are
// structural in this package:
//
//   - Grade the decision, not the outcome. GradeAction's only inputs are
//     the frozen Advice (computed from the hero's PlayerView at decision
//     time) and the action taken. There is no parameter through which a
//     runout, a showdown, or an opponent's hole card could reach the
//     grader, so a correct call that loses grades byte-identically to the
//     same call when it wins. The anti-resulting test pins this.
//
//   - One source of strategic truth. The coach's brain is
//     rulebased.NewStrategy(ai.Archetypes["coach"], seed) — the identical
//     code the table's bots run, under the TAG parameter block (the
//     "coach" preset is pinned equal to "tag" by test). There is no second
//     strategy for the coach to disagree with.
package coach

import (
	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/ai/rulebased"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
)

// NumberChip is one labelled figure the coach panel renders as a chip:
// {"Pot odds", "2.2:1 (31%)"}, {"Your equity", "~34%"}, {"Outs", "8"}.
// Values are pre-formatted here so the TUI renders and never computes.
type NumberChip struct {
	Label, Value string
}

// Advice is the coach's frozen recommendation for one decision point. It is
// captured the moment the hero's turn begins and never recomputed: the
// advice the hero saw is the advice grading uses, which keeps the audit
// trail airtight even though a recompute would be identical (the strategy
// is deterministic per seed).
type Advice struct {
	// Decision is the full strategy output: chosen action, every scored
	// candidate, and the typed rationale. Grading reads Candidates; the
	// explanation was rendered from Rationale and nothing else.
	Decision ai.Decision

	Headline string       // imperative, ≤40 chars: "Raise to 90"
	Body     string       // 2–4 sentences, one idea each
	Numbers  []NumberChip // the hard numbers, out of the prose

	// Digest is the compact snapshot of what the hero knew, taken at
	// advise time. GradeAction copies it into the GradedDecision so the
	// review screen can show the spot without replaying the hand — and so
	// the grader never needs to be handed a view after the fact.
	Digest ViewDigest
}

// ForMode returns the displayable copy of the advice for a coach verbosity
// mode (DESIGN.md §2.9): Full shows everything; Mistakes shows the numbers
// but withholds the opinion (Headline and Body blank) until after the hero
// acts; Off shows nothing.
//
// Decision and Digest are retained in every mode — grading needs them, and
// grades are recorded even at Off. Renderers must draw only Headline, Body
// and Numbers.
func (a Advice) ForMode(mode string) Advice {
	switch mode {
	case profile.CoachOff:
		a.Headline, a.Body, a.Numbers = "", "", nil
	case profile.CoachMistakes:
		a.Headline, a.Body = "", ""
	}
	return a
}

// Coach advises, grades, and teaches from the hero's seat.
type Coach struct {
	strat *rulebased.Strategy
	prof  *profile.Profile

	// seen is the fire-once fallback for teachable moments when no profile
	// is attached (trainer sessions, tests). With a profile, moments
	// persist forever via profile.MomentsSeen instead.
	seen map[string]bool
}

// New builds a Coach. The strategy is the one rule-based implementation
// under the "coach" archetype — the same code, the same dials, the same
// seed-determinism the opponents have. prof may be nil (moments then
// fire once per Coach instead of once per player, and grades are not
// recorded); seed makes advice reproducible for replay and tests.
func New(prof *profile.Profile, seed int64) *Coach {
	return &Coach{
		strat: rulebased.NewStrategy(ai.Archetypes["coach"], seed),
		prof:  prof,
		seen:  map[string]bool{},
	}
}

// SetRead attributes an archetype to an opponent's seat. The coach reads
// bots as what they actually are — the app knows the lineup, and the hero
// is told the read, so this is the lesson rather than cheating. Reads
// change perception (range assignment, bluff warnings) only; they cannot
// reveal cards the view does not contain.
func (c *Coach) SetRead(seat engine.Seat, p ai.Personality) { c.strat.SetRead(seat, p) }

// Mode returns the persisted coach verbosity ("full" | "mistakes" | "off"),
// defaulting to full when no profile is attached.
func (c *Coach) Mode() string {
	if c.prof == nil || c.prof.CoachMode == "" {
		return profile.CoachFull
	}
	return c.prof.CoachMode
}

// Advise computes the coach's recommendation for the hero's decision. v
// must be the acting hero's view (v.Legal non-empty). The result is frozen:
// callers capture it when the turn begins and pass the same value to
// GradeAction when the hero commits.
func (c *Coach) Advise(v *engine.PlayerView) Advice {
	d := c.strat.Decide(v)
	headline, body, chips := Explain(d, v)
	return Advice{
		Decision: d,
		Headline: headline,
		Body:     body,
		Numbers:  chips,
		Digest:   NewViewDigest(v),
	}
}
