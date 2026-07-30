package app

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
)

// Navigation payloads. Every NavigateMsg.Data value is one of the named
// structs in this file (or nil) — never a map or a bare primitive — so a
// screen receiving construction data can type-switch on exactly the shapes
// it understands (docs/design-tui.md §1.2).

// TableConfig is the payload GameSetup emits to construct a table session:
// stakes, buy-in, who is at the table, and how chatty the coach should be.
// It is the app-level sibling of engine.TableConfig, which carries only what
// the engine needs; this carries what a *session* needs.
type TableConfig struct {
	SmallBlind engine.Chips
	BigBlind   engine.Chips
	Stack      engine.Chips // starting stack (buy-in) per seat
	Lineup     []string     // opponent archetype keys, seats 1..5 in order
	CoachMode  CoachMode
	Speed      Speed
}

// StackBB is the buy-in expressed in big blinds, the unit poker players
// actually think in ("a 100bb cash game").
func (c TableConfig) StackBB() int {
	if c.BigBlind <= 0 {
		return 0
	}
	return int(c.Stack / c.BigBlind)
}

// ReviewRequest asks for the post-hand review screen. ReturnTo exists
// because HandReview has two parents — Table ("review last hand") and
// Lessons (worked examples) — and an explicit return target is simpler than
// a navigation stack for a graph this shallow (docs/design-tui.md §1.2).
type ReviewRequest struct {
	// TODO(wire-review): carry the engine.HandRecord + coach annotations
	// once internal/review lands; until then the request only routes.
	ReturnTo Screen
}

// LessonRequest opens the lessons screen at a specific lesson.
type LessonRequest struct {
	LessonID string
}

// --- Coach verbosity --------------------------------------------------------

// CoachMode is the coach's verbosity (DESIGN.md §2.9). The progression
// mirrors learning: first you want answers explained (Full), then to decide
// alone but be caught when wrong (Mistakes), then a clean game whose
// mistakes you review afterwards (Off — grades are still recorded).
type CoachMode int

// Coach verbosity modes, in cycle order.
const (
	CoachFull     CoachMode = iota // advice + reasoning before the hero acts
	CoachMistakes                  // numbers only before; opinion only on mistakes
	CoachOff                       // silent; post-hand review still works
)

var coachModeNames = [...]string{"Full", "Mistakes", "Off"}

// String is the display label ("Full").
func (m CoachMode) String() string {
	if int(m) < 0 || int(m) >= len(coachModeNames) {
		return "?"
	}
	return coachModeNames[m]
}

// ProfileKey is the persisted form ("full"), matching the profile package's
// string vocabulary so the two layers cannot drift.
func (m CoachMode) ProfileKey() string {
	switch m {
	case CoachMistakes:
		return profile.CoachMistakes
	case CoachOff:
		return profile.CoachOff
	default:
		return profile.CoachFull
	}
}

// CoachModeFromProfile maps a persisted key back to a CoachMode. Unknown
// keys (a hand-edited profile) fall back to Full — the safest mode for a
// learning tool is the one that explains itself.
func CoachModeFromProfile(key string) CoachMode {
	switch key {
	case profile.CoachMistakes:
		return CoachMistakes
	case profile.CoachOff:
		return CoachOff
	default:
		return CoachFull
	}
}

// --- Game speed --------------------------------------------------------------

// Speed is the pacing setting (docs/design-tui.md §6.3). It scales every
// animation and think-time constant; SpeedInstant short-circuits all delays,
// which is what makes rendering tests deterministic.
type Speed int

// Speeds, slowest first.
const (
	SpeedLearn   Speed = iota // pauses on new cards; longest think-times
	SpeedNormal               // no hard pauses; natural think-times
	SpeedFast                 // flat short think; animations at 2x
	SpeedInstant              // no delays at all (tests run here)
)

var speedNames = [...]string{"Learn", "Normal", "Fast", "Instant"}

// String is the display label ("Learn").
func (s Speed) String() string {
	if int(s) < 0 || int(s) >= len(speedNames) {
		return "?"
	}
	return speedNames[s]
}

// --- Opponent archetypes ------------------------------------------------------
//
// TODO(wire-ai): these keys and labels become ai package presets once
// internal/ai lands (design-learning.md §4.6). Until then they are plain
// data so game setup can choose a lineup today; the keys are the contract —
// they match profile.DefaultTableConfig and will match the ai registry.

// Archetype describes one opponent style for the setup screen.
type Archetype struct {
	Key   string // stable identifier persisted in profiles ("nit")
	Label string // display name ("Nit")
	Blurb string // one-line description for the setup screen
}

// Archetypes lists every opponent style, in teaching order (tightest to
// wildest) — the order itself sketches the tight/loose, passive/aggressive
// axes a beginner needs to internalize.
func Archetypes() []Archetype {
	return []Archetype{
		{Key: "nit", Label: "Nit", Blurb: "plays only premium hands; a big bet means it"},
		{Key: "tag", Label: "TAG", Blurb: "tight-aggressive; solid ranges, applies pressure"},
		{Key: "lag", Label: "LAG", Blurb: "loose-aggressive; wide ranges, relentless betting"},
		{Key: "station", Label: "Station", Blurb: "calling station; never folds, rarely raises"},
		{Key: "maniac", Label: "Maniac", Blurb: "raises everything; huge swings, thin values"},
	}
}

// ClassroomLineup is the default opponent mix (DESIGN.md §2.11): one of each
// archetype, so every hand demonstrates the full spectrum of styles.
func ClassroomLineup() []string {
	return []string{"nit", "tag", "lag", "station", "maniac"}
}

// LineupPreset is a named table composition the setup screen can cycle
// through. Presets rather than per-seat pickers: a beginner should choose a
// *kind of table*, not micro-manage five seats.
type LineupPreset struct {
	Label string
	Keys  []string // archetype keys, seats 1..5
}

// LineupPresets lists the selectable table mixes, classroom mix first (it is
// the default for new profiles).
func LineupPresets() []LineupPreset {
	return []LineupPreset{
		{Label: "Classroom mix", Keys: ClassroomLineup()},
		{Label: "Tight table", Keys: []string{"nit", "nit", "tag", "tag", "station"}},
		{Label: "Aggressive table", Keys: []string{"lag", "lag", "tag", "maniac", "maniac"}},
		{Label: "Loose-passive", Keys: []string{"station", "station", "station", "nit", "tag"}},
	}
}
