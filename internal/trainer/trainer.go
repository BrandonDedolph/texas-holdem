// Package trainer is the standalone drill mode (design-learning.md §9):
// four quiz kinds — hand rankings, outs counting, equity estimation, and
// full decision spots — assembled into short, scored sessions.
//
// The package's one load-bearing idea is generation, not authoring. Every
// item is produced from a random deal and its stated answer is *computed*:
// rankings by eval, outs and equity by the equity package, spot answers by
// the same coach that grades live play. The pool is therefore infinite and
// can never teach a wrong fact — the verification tests regenerate the
// answers independently to hold that promise, because a trainer that
// teaches a wrong fact is worse than no trainer.
//
// Items reuse tutorial.Drill so the curriculum and the trainer share one
// renderer; difficulty is tracked per skill tag in the profile (EMA α=0.3,
// level unlock at ≥80% over ≥20 items — see internal/profile).
package trainer

import (
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/BrandonDedolph/texas-holdem/internal/coach"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

// QuizKind identifies one of the four drill families.
type QuizKind int

// Quiz kinds, in menu order.
const (
	QuizRankings QuizKind = iota // which of two showdown hands wins
	QuizOuts                     // count clean outs exactly
	QuizEquity                   // estimate equity within tolerance
	QuizSpots                    // fold/call/raise, graded by the coach
	NumQuizKinds = 4
)

var quizNames = [NumQuizKinds]string{"rankings", "outs", "equity", "spots"}
var quizTitles = [NumQuizKinds]string{
	"Rankings Speed Quiz", "Outs Quiz", "Equity Estimate", "Spot Quiz",
}

// String is the stable lowercase key ("outs") — also the profile tag the
// kind's level progression is recorded under (see kindTag).
func (k QuizKind) String() string {
	if k < 0 || int(k) >= len(quizNames) {
		return "?"
	}
	return quizNames[k]
}

// Title is the display name ("Outs Quiz").
func (k QuizKind) Title() string {
	if k < 0 || int(k) >= len(quizTitles) {
		return "?"
	}
	return quizTitles[k]
}

// SessionItems is the length of one session: long enough to move the EMA,
// short enough to finish in a few minutes.
const SessionItems = 10

// maxAttempts bounds every rejection-sampling loop in the generators. The
// trap predicates hit within a few dozen draws in practice (the termination
// test pins this); exhausting the budget means a generator bug, and
// panicking loudly beats quietly serving an item whose trap predicate does
// not hold.
const maxAttempts = 100_000

// Item is one generated question. Drill reuses the tutorial type so one
// renderer draws curriculum drills and trainer items alike; SkillTag names
// the sub-skill for the profile's EMA ("outs.flush"). Spot is set only for
// QuizSpots items, whose grading runs through coach.GradeAction rather than
// the drill's literal answer check.
type Item struct {
	Drill    tutorial.Drill
	SkillTag string
	Spot     *Spot

	// Generation record for the verification tests: the question's inputs
	// as card codes (or a range spec), so a test can recompute the stated
	// answer without reaching into the generator.
	hero, villain, board string
	villainIsRange       bool
}

// Outcome is the result of answering one item.
type Outcome struct {
	Correct bool
	Points  float64 // 1 full, 0.5 for a near-miss equity guess, 0 otherwise
	Explain string  // the item's explanation, shown right or wrong
	Detail  string  // spot items: the coach's grade sentence
	// Grade is meaningful only for spot items (Detail non-empty); other
	// kinds leave it at the zero value.
	Grade     coach.Grade
	LeveledUp bool // this answer completed the kind's level unlock
}

// equityHalfMargin is the half-credit band for equity estimates: within the
// drill's own tolerance (±5) scores full, within ±10 scores half. The half
// point keeps a near miss from feeling like a zero, but the EMA records it
// as a miss — close is not yet calibrated, and calibration is the skill.
const equityHalfMargin = 10

// Session is one drill run: a fixed slate of generated items plus the
// running score. Scoring state is exported for the screen; the profile and
// grader stay internal.
type Session struct {
	Kind  QuizKind
	Level int // 1-based difficulty the items were generated at
	Timed bool
	Items []Item

	Answered   int
	Correct    int
	Score      float64
	Streak     int
	BestStreak int
	LeveledUp  bool // the kind's next level unlocked during this session

	prof   *profile.Profile
	grader *coach.Coach
	pos    int
}

// NewSession builds a session at the profile's current level for the kind,
// with item selection weighted toward the player's weak tags.
func NewSession(kind QuizKind, prof *profile.Profile) *Session {
	return NewSessionSeeded(kind, prof, time.Now().UnixNano())
}

// NewSessionSeeded is NewSession with an explicit seed: the same seed and
// profile always generate the identical slate, which is what makes the
// generators testable and a session reproducible.
func NewSessionSeeded(kind QuizKind, prof *profile.Profile, seed int64) *Session {
	rng := newRNG(seed)
	level := levelFor(prof, kind)
	s := &Session{
		Kind:  kind,
		Level: level,
		Timed: kind == QuizRankings,
		Items: make([]Item, 0, SessionItems),
		prof:  prof,
		// The grader needs no profile: trainer grades feed DrillStats via
		// RecordDrill, not the lifetime GradeTotals of live play.
		grader: coach.New(nil, seed),
	}
	pool := tagPool(kind, level)
	for i := 0; i < SessionItems; i++ {
		tag := pickTag(rng, prof, pool)
		s.Items = append(s.Items, generateItem(kind, rng, tag))
	}
	return s
}

// newRNG seeds a rand/v2 PCG from one int64. The salt is the same golden
// ratio constant the engine's deck uses, so seed 0 is as good as any other.
func newRNG(seed int64) *rand.Rand {
	return rand.New(rand.NewPCG(uint64(seed), 0x9e3779b97f4a7c15))
}

// generateItem dispatches to the kind's generator. The tag selects the
// sub-skill (and with it the difficulty shape) within the kind.
func generateItem(kind QuizKind, rng *rand.Rand, tag string) Item {
	switch kind {
	case QuizRankings:
		return genRankings(rng, tag)
	case QuizOuts:
		return genOuts(rng, tag)
	case QuizEquity:
		return genEquity(rng, tag)
	default:
		return genSpots(rng, tag)
	}
}

// Current returns the item awaiting an answer, or nil when the session is
// done.
func (s *Session) Current() *Item {
	if s.pos >= len(s.Items) {
		return nil
	}
	return &s.Items[s.pos]
}

// Done reports whether every item has been answered.
func (s *Session) Done() bool { return s.pos >= len(s.Items) }

// Accuracy is the fraction of answered items that were correct, 0..1.
func (s *Session) Accuracy() float64 {
	if s.Answered == 0 {
		return 0
	}
	return float64(s.Correct) / float64(s.Answered)
}

// Answer checks the input against the current item, updates the score and
// the profile's skill stats, and advances to the next item. Answers are
// recorded as they happen, so an abandoned session still counts — progress
// is earned per answer, not per completed session.
func (s *Session) Answer(input string) Outcome {
	it := s.Current()
	if it == nil {
		return Outcome{}
	}
	var out Outcome
	switch {
	case it.Spot != nil:
		out = s.answerSpot(it, input)
	case s.Kind == QuizEquity:
		out = answerEquity(it, input)
	default:
		if it.Drill.Answer.Check(input) {
			out.Correct, out.Points = true, 1
		}
	}
	out.Explain = it.Drill.Explain

	s.Answered++
	s.Score += out.Points
	if out.Correct {
		s.Correct++
		s.Streak++
		if s.Streak > s.BestStreak {
			s.BestStreak = s.Streak
		}
	} else {
		s.Streak = 0
	}
	s.record(it, out.Correct, &out)
	s.pos++
	return out
}

// answerSpot grades a spot answer with coach.GradeAction against the frozen
// advice. GradeBest and GradeGood both count as correct — close spots must
// never be marked wrong on a coin flip (design-learning.md §9).
func (s *Session) answerSpot(it *Item, input string) Outcome {
	choice, ok := it.Drill.Answer.(tutorial.ChoiceAnswer)
	if !ok {
		return Outcome{}
	}
	idx := choiceIndex(input, choice.Choices)
	if idx < 0 || idx >= len(it.Spot.Options) {
		return Outcome{}
	}
	g := s.grader.GradeAction(it.Spot.Advice, it.Spot.Options[idx])
	out := Outcome{Detail: g.Body, Grade: g.Grade}
	if g.Grade.GoodOrBetter() {
		out.Correct, out.Points = true, 1
	}
	return out
}

// answerEquity scores an equity estimate: within the drill's tolerance is
// full credit, within equityHalfMargin is half credit (still recorded as a
// miss — see equityHalfMargin).
func answerEquity(it *Item, input string) Outcome {
	num, ok := it.Drill.Answer.(tutorial.NumericAnswer)
	if !ok {
		return Outcome{}
	}
	v, err := parseNumber(input)
	if err != nil {
		return Outcome{}
	}
	diff := math.Abs(v - num.Value)
	switch {
	case diff <= num.Tolerance:
		return Outcome{Correct: true, Points: 1}
	case diff <= equityHalfMargin:
		return Outcome{Points: 0.5}
	default:
		return Outcome{}
	}
}

// record feeds one answer into the profile: the fine-grained tag drives the
// weakness weighting of future sessions, and the kind's gate tag drives the
// level unlock. Both boundaries of the unlock are the profile's (≥80% EMA
// over ≥20 answers at the current level).
func (s *Session) record(it *Item, correct bool, out *Outcome) {
	if s.prof == nil {
		return
	}
	s.prof.RecordDrill(it.SkillTag, correct)
	stat := s.prof.RecordDrill(kindTag(s.Kind), correct)
	if s.Level < MaxLevel && stat.UnlockReady() && s.prof.AdvanceSkill(kindTag(s.Kind)) {
		s.LeveledUp, out.LeveledUp = true, true
	}
}

// parseNumber parses a numeric answer, forgiving a trailing "%".
func parseNumber(input string) (float64, error) {
	in := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(input), "%"))
	return strconv.ParseFloat(in, 64)
}

// choiceIndex resolves an answer to a choice index: the 1-based number, or
// the choice text itself (case-insensitive). Returns -1 when unrecognized.
func choiceIndex(input string, choices []string) int {
	in := strings.TrimSpace(input)
	if n, err := strconv.Atoi(in); err == nil {
		if n >= 1 && n <= len(choices) {
			return n - 1
		}
		return -1
	}
	for i, c := range choices {
		if strings.EqualFold(in, c) {
			return i
		}
	}
	return -1
}

// --- deal helpers shared by the generators ------------------------------------

// deal returns n distinct cards from a fresh seeded shuffle.
func deal(rng *rand.Rand, n int) []engine.Card {
	cards := make([]engine.Card, engine.NumCards)
	for i := range cards {
		cards[i] = engine.Card(i)
	}
	rng.Shuffle(len(cards), func(i, j int) { cards[i], cards[j] = cards[j], cards[i] })
	return cards[:n]
}

// randCard draws a uniformly random card outside used and marks it used.
func randCard(rng *rand.Rand, used *engine.CardSet) engine.Card {
	for {
		c := engine.Card(rng.IntN(engine.NumCards))
		if !used.Has(c) {
			*used = used.Add(c)
			return c
		}
	}
}

// take claims a specific card; it reports false (without claiming) when the
// card is already used, so constructive generators can fall back to
// resampling instead of building an impossible deal.
func take(used *engine.CardSet, c engine.Card) bool {
	if used.Has(c) {
		return false
	}
	*used = used.Add(c)
	return true
}
