// Package profile persists the player's learning progress and settings.
//
// One package owns everything the app remembers between runs: lesson
// completion, teachable moments already shown, drill skill levels, lifetime
// grade counts, the bankroll, and user settings (coach verbosity, table
// defaults). There is deliberately no separate settings package — see
// docs/DESIGN.md §2.10.
//
// The profile itself is one small human-readable JSON file so a curious
// player can open it and read their own progress. Hand histories grow
// without bound, so they live apart as append-only JSONL (see store.go)
// and the profile stays a fast, small read at startup.
package profile

import (
	"time"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// CurrentVersion is the profile schema version this build reads and writes.
// Load switches on the stored Version to migrate old files; a version it
// does not recognize is preserved untouched (see Store.Load), never
// overwritten by guesswork.
const CurrentVersion = 1

// Coach verbosity modes, persisted as strings so the profile file stays
// readable. The canonical vocabulary is DESIGN.md §2.9: the learning doc's
// "grade-only" is the same mode as CoachMistakes.
const (
	CoachFull     = "full"     // advice shown before acting
	CoachMistakes = "mistakes" // numbers visible, recommendation revealed after acting
	CoachOff      = "off"      // no live coaching; post-hand review still works
)

// Default table stakes and bankroll. Provisional until game setup exposes
// stake selection: 5/10 blinds, 100bb buy-in, bankroll of ten buy-ins.
const (
	DefaultSmallBlind engine.Chips = 5
	DefaultBigBlind   engine.Chips = 10
	DefaultStack      engine.Chips = 1_000
	DefaultBankroll   engine.Chips = 10_000
)

// Profile is everything persisted about the player except hand histories.
// The zero value is not usable directly (its maps are nil); construct with
// NewProfile or obtain one from Store.Load, which never returns nil.
type Profile struct {
	Version     int                  `json:"version"`
	CreatedAt   time.Time            `json:"created_at"`
	Bankroll    engine.Chips         `json:"bankroll"`     // session-persistent chips
	LessonsDone map[string]time.Time `json:"lessons_done"` // lesson ID -> completion time
	MomentsSeen map[string]time.Time `json:"moments_seen"` // moment ID -> first shown
	DrillStats  map[string]SkillStat `json:"drill_stats"`  // trainer SkillTag -> stat

	// GradeTotals counts lifetime graded decisions per grade band, keyed by
	// the grade's string name ("best", "good", "inaccuracy", ...).
	// TODO(wire-types): key by coach.Grade's canonical name once
	// internal/coach lands; profile stays decoupled until then.
	GradeTotals map[string]int `json:"grade_totals"`

	SessionLog []SessionSummary `json:"session_log"`

	// LastQuiz is the trainer quiz kind last started, so the trainer opens on
	// what the player is actually practising instead of resetting to the top
	// every run. Empty means "never started one".
	LastQuiz string `json:"last_quiz"`

	// UnlockAllLessons lifts every lesson's prerequisite gate (Settings ->
	// "Lesson locks: All open"). Off by default: the lockstep curriculum is
	// the pedagogy. Completion tracking is unaffected — only the gates lift.
	UnlockAllLessons bool `json:"unlock_all_lessons"`

	CoachMode     string      `json:"coach_mode"` // CoachFull | CoachMistakes | CoachOff
	TableDefaults TableConfig `json:"table_defaults"`
	Display       Display     `json:"display"`
	// store is the Store that loaded this profile, so Save writes back to
	// the same place. Unexported and JSON-invisible; nil means "resolve the
	// default user directory on first Save".
	store *Store
}

// SessionSummary is one play session's headline numbers. Decision accuracy
// and net result are tracked side by side on purpose: the stats screen plots
// them diverging, and that divergence is the variance lesson.
type SessionSummary struct {
	Start    time.Time `json:"start"`
	Hands    int       `json:"hands"`
	NetBB    float64   `json:"net_bb"`     // chips result in big blinds
	Accuracy float64   `json:"accuracy"`   // fraction of decisions graded good or better, 0..1
	EVLossBB float64   `json:"ev_loss_bb"` // summed decision EV leaks
}

// TableConfig is the last-used game setup, restored as the default next run.
type TableConfig struct {
	Lineup     []string     `json:"lineup"` // opponent archetype keys, e.g. "nit", "tag"
	SmallBlind engine.Chips `json:"small_blind"`
	BigBlind   engine.Chips `json:"big_blind"`
	Stack      engine.Chips `json:"stack"` // buy-in stack per seat
}

// Display holds terminal presentation preferences.
//
// The values are stored as strings rather than the app layer's enums so the
// profile stays decoupled from internal/app (which imports profile, not the
// other way round) and so a hand-edited profile.json reads sensibly. Unknown
// values fall back to the default rather than erroring — a typo in a settings
// file should not stop the game from starting.
type Display struct {
	Speed      string `json:"speed"`      // learn | normal | fast | instant
	Deck       string `json:"deck"`       // four-color | two-color
	ASCII      bool   `json:"ascii"`      // force the ASCII glyph set
	Background string `json:"background"` // auto | dark | light
}

// Display defaults. Learn speed and the four-color deck are the learning-tool
// defaults: unlimited reading time, and every suit its own ink because flush
// blindness is the classic beginner leak.
const (
	SpeedLearn   = "learn"
	DeckFourCol  = "four-color"
	DeckTwoCol   = "two-color"
	BackgroundAu = "auto"
)

// DefaultDisplay returns the first-run presentation preferences.
func DefaultDisplay() Display {
	return Display{Speed: SpeedLearn, Deck: DeckTwoCol, Background: BackgroundAu}
}

// normalize fills blank fields with their defaults, so a profile written by an
// older version (or edited by hand) is always usable.
func (d *Display) normalize() {
	if d.Speed == "" {
		d.Speed = SpeedLearn
	}
	if d.Deck == "" {
		d.Deck = DeckTwoCol
	}
	if d.Background == "" {
		d.Background = BackgroundAu
	}
}

// DefaultTableConfig is the "classroom mix" (DESIGN.md §2.11): hero plus one
// of each archetype, at the default stakes.
func DefaultTableConfig() TableConfig {
	return TableConfig{
		Lineup:     []string{"nit", "tag", "lag", "station", "maniac"},
		SmallBlind: DefaultSmallBlind,
		BigBlind:   DefaultBigBlind,
		Stack:      DefaultStack,
	}
}

// NewProfile returns a fresh first-run profile with every map allocated, so
// callers may index into it without nil checks.
func NewProfile() *Profile {
	return &Profile{
		Version:       CurrentVersion,
		CreatedAt:     time.Now().UTC(),
		Bankroll:      DefaultBankroll,
		LessonsDone:   map[string]time.Time{},
		MomentsSeen:   map[string]time.Time{},
		DrillStats:    map[string]SkillStat{},
		GradeTotals:   map[string]int{},
		CoachMode:     CoachFull,
		TableDefaults: DefaultTableConfig(),
		Display:       DefaultDisplay(),
	}
}

// ensureMaps allocates any nil maps. Load calls this so a hand-edited or
// older profile file can never hand out a profile that panics on first use.
func (p *Profile) ensureMaps() {
	if p.LessonsDone == nil {
		p.LessonsDone = map[string]time.Time{}
	}
	if p.MomentsSeen == nil {
		p.MomentsSeen = map[string]time.Time{}
	}
	if p.DrillStats == nil {
		p.DrillStats = map[string]SkillStat{}
	}
	if p.GradeTotals == nil {
		p.GradeTotals = map[string]int{}
	}
	p.Display.normalize()
}

// CompleteLesson records a lesson as done. Re-completing refreshes the
// timestamp; the curriculum treats any entry as "done".
func (p *Profile) CompleteLesson(id string) {
	p.ensureMaps()
	p.LessonsDone[id] = time.Now().UTC()
}

// MarkMomentSeen records the first time a teachable moment fired. Moments
// are once-per-player, so a repeat call keeps the original timestamp.
func (p *Profile) MarkMomentSeen(id string) {
	p.ensureMaps()
	if _, seen := p.MomentsSeen[id]; !seen {
		p.MomentsSeen[id] = time.Now().UTC()
	}
}

// RecordGrade increments the lifetime count for one grade band.
// TODO(wire-types): accept coach.Grade once internal/coach lands.
func (p *Profile) RecordGrade(grade string) {
	p.ensureMaps()
	p.GradeTotals[grade]++
}

// --- Drill skill tracking -------------------------------------------------
//
// The trainer tracks per-skill accuracy as an exponential moving average and
// gates level unlocks on it (design-learning.md §9). The arithmetic lives
// here, next to the persisted state, so the trainer and any future stats
// screen cannot disagree about what the numbers mean.

const (
	// emaAlpha weights the newest answer at 30%. High enough that ~10 recent
	// answers dominate the average (recovery from a bad streak is fast),
	// low enough that one lucky guess cannot swing a level unlock.
	emaAlpha = 0.3

	// UnlockEMA and UnlockAttempts gate level progression: at least 80%
	// rolling accuracy over at least 20 answers at the current level.
	// Exported so the trainer's ladder can state the rule it enforces —
	// a displayed threshold must be the enforced threshold.
	UnlockEMA      = 0.8
	UnlockAttempts = 20
)

// SkillStat is one skill tag's progress at its current difficulty level.
// Attempts and EMA describe the current level only — they reset on level-up,
// because an average earned at an easier level says nothing about this one.
type SkillStat struct {
	EMA      float64 `json:"ema"`      // rolling accuracy, 0..1
	Attempts int     `json:"attempts"` // answers at the current level
	Level    int     `json:"level"`    // 0-based difficulty level
}

// Record folds one drill answer into the stat and returns the updated value.
// The first answer at a level seeds the EMA directly rather than blending
// with an arbitrary prior — otherwise a player would start every level
// "wrong" and grind against a phantom zero.
func (s SkillStat) Record(correct bool) SkillStat {
	x := 0.0
	if correct {
		x = 1.0
	}
	if s.Attempts == 0 {
		s.EMA = x
	} else {
		s.EMA = (1-emaAlpha)*s.EMA + emaAlpha*x
	}
	s.Attempts++
	return s
}

// UnlockReady reports whether this skill has earned the next level:
// EMA ≥ 80% over ≥ 20 attempts at the current level. Both boundaries are
// inclusive — exactly 80% over exactly 20 answers unlocks.
func (s SkillStat) UnlockReady() bool {
	return s.Attempts >= UnlockAttempts && s.EMA >= UnlockEMA
}

// RecordDrill records one answer for a skill tag and returns the updated
// stat. Safe on a zero-value map.
func (p *Profile) RecordDrill(tag string, correct bool) SkillStat {
	p.ensureMaps()
	s := p.DrillStats[tag].Record(correct)
	p.DrillStats[tag] = s
	return s
}

// AdvanceSkill promotes a skill to the next level if it has earned it,
// resetting EMA and attempts for the new level. Returns whether it advanced.
func (p *Profile) AdvanceSkill(tag string) bool {
	p.ensureMaps()
	s := p.DrillStats[tag]
	if !s.UnlockReady() {
		return false
	}
	p.DrillStats[tag] = SkillStat{Level: s.Level + 1}
	return true
}
