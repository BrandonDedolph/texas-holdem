package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
)

// driveKeys feeds keys to the model and returns it, running any command the
// update produced so pacing ticks and villain actions resolve. SpeedInstant
// makes this synchronous.
func driveKeys(m tea.Model, keys ...string) tea.Model {
	for _, k := range keys {
		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)})
		for cmd != nil {
			msg := cmd()
			if msg == nil {
				break
			}
			m, cmd = m.Update(msg)
		}
	}
	return m
}

func newLiveTable(t *testing.T, coach CoachMode) tea.Model {
	t.Helper()
	cfg := TableConfig{
		SmallBlind: 5, BigBlind: 10, Stack: 1000,
		Lineup: ClassroomLineup(), CoachMode: coach, Speed: SpeedInstant,
	}
	var m tea.Model = NewTableScreen(cfg, DefaultPrefs())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Init()
	return m
}

// TestTablePlaysHandsAgainstRealAI folds the hero through many hands against
// the real strategy engine. It is a smoke test for the whole stack composing:
// engine + eval + equity + ai + the table screen. Anything that panics,
// deadlocks, or renders a broken frame shows up here.
func TestTablePlaysHandsAgainstRealAI(t *testing.T) {
	m := newLiveTable(t, CoachFull)
	for i := 0; i < 40; i++ {
		// Fold when it is our turn, otherwise advance the pacing/hand loop.
		m = driveKeys(m, "f", " ", "\r")
		view := m.View()
		if strings.TrimSpace(view) == "" {
			t.Fatalf("iteration %d: table rendered an empty frame", i)
		}
		if lines := strings.Count(view, "\n"); lines > 40 {
			t.Fatalf("iteration %d: frame grew to %d lines, layout is reflowing", i, lines)
		}
	}
}

// TestTableSurvivesAggression drives the hero into raising rather than
// folding, so the AI faces real pressure and the sizing path is exercised.
func TestTableSurvivesAggression(t *testing.T) {
	m := newLiveTable(t, CoachMistakes)
	for i := 0; i < 30; i++ {
		// r/b opens sizing, 2 picks the half-pot preset, enter confirms;
		// x checks and c calls cover the spots where raising is illegal.
		m = driveKeys(m, "r", "2", "\r", "b", "2", "\r", "x", "c", " ", "\r")
		if strings.TrimSpace(m.View()) == "" {
			t.Fatalf("iteration %d: table rendered an empty frame", i)
		}
	}
}

// TestEveryArchetypeSeatsAndPlays builds a table of each archetype in turn and
// plays hands, so a bad preset key or a personality that produces illegal
// actions is caught here rather than in front of a learner.
func TestEveryArchetypeSeatsAndPlays(t *testing.T) {
	for _, a := range Archetypes() {
		t.Run(a.Key, func(t *testing.T) {
			cfg := TableConfig{
				SmallBlind: 5, BigBlind: 10, Stack: 1000,
				Lineup: []string{a.Key, a.Key, a.Key, a.Key, a.Key},
				Speed:  SpeedInstant,
			}
			var m tea.Model = NewTableScreen(cfg, DefaultPrefs())
			m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
			m.Init()
			for i := 0; i < 15; i++ {
				m = driveKeys(m, "c", "x", " ", "\r")
			}
			if strings.TrimSpace(m.View()) == "" {
				t.Fatal("table rendered an empty frame")
			}
		})
	}
}

// TestUnknownArchetypeFallsBackToBaseline: a stale lineup in a saved profile
// must seat a sensible opponent rather than breaking the table.
func TestUnknownArchetypeFallsBackToBaseline(t *testing.T) {
	opp := newArchetypeOpponent("no-such-style", "Ghost", 1)
	if opp == nil {
		t.Fatal("unknown archetype produced a nil opponent")
	}
	if opp.Name() != "Ghost" {
		t.Errorf("Name = %q, want %q", opp.Name(), "Ghost")
	}
	type personalitied interface{ Personality() ai.Personality }
	p, ok := opp.(personalitied)
	if !ok {
		t.Skip("opponent does not expose its personality")
	}
	if got, want := p.Personality().Key, ai.Baseline().Key; got != want {
		t.Errorf("fallback personality = %q, want the baseline %q", got, want)
	}
}

// TestSetupArchetypesMatchTheAIRegistry pins the single-source-of-truth
// property: the setup screen's list is a projection of the ai presets, so a
// label can never drift from the behaviour it names.
func TestSetupArchetypesMatchTheAIRegistry(t *testing.T) {
	for _, a := range Archetypes() {
		p, ok := ai.Archetype(a.Key)
		if !ok {
			t.Errorf("setup offers %q, which the ai registry does not define", a.Key)
			continue
		}
		if a.Label != p.Label || a.Blurb != p.Blurb {
			t.Errorf("%q drifted from the registry:\n got  %q / %q\n want %q / %q",
				a.Key, a.Label, a.Blurb, p.Label, p.Blurb)
		}
	}
	if _, ok := ai.Archetype("coach"); !ok {
		t.Error("the coach preset should exist in the registry")
	}
	for _, a := range Archetypes() {
		if a.Key == "coach" {
			t.Error("the coach preset is an advisor, not a seat to play against")
		}
	}
}
