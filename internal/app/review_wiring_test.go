package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/review"
)

// TestLiveHandCarriesGradesIntoReview closes the loop the whole app is built
// around: the coach grades the hero's decisions during play, and the review
// screen shows those frozen grades afterwards. An empty Grades map here means
// the review renders a hand with no verdicts — the feature silently absent.
func TestLiveHandCarriesGradesIntoReview(t *testing.T) {
	cfg := TableConfig{
		SmallBlind: 5, BigBlind: 10, Stack: 1000,
		Lineup: ClassroomLineup(), CoachMode: CoachFull, Speed: SpeedInstant,
	}
	var m tea.Model = NewTableScreen(cfg, DefaultPrefs(), profile.NewProfile())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Init()

	// Drive one key at a time and snapshot the archive the moment a hand
	// completes: grades are per-hand, so starting the next one clears them.
	var (
		rec engine.HandRecord
		ann review.HandAnnotations
		got bool
	)
	keys := []string{"c", "x", " ", "\r"}
	for i := 0; i < 400 && !got; i++ {
		m = driveKeys(m, keys[i%len(keys)])
		ts := m.(*TableScreen)
		if ts.hand == nil || ts.hand.Phase() != engine.PhaseComplete || len(ts.grades) == 0 {
			continue
		}
		if r, a, ok := ts.lastHandArchive(); ok && len(a.Grades) > 0 {
			rec, ann, got = r, a, true
		}
	}
	if !got {
		t.Fatal("no completed hand carried the coach's grades into the review archive")
	}

	// Every graded index must point at a hero action, and carry real content.
	for idx, g := range ann.Grades {
		if idx < 0 || idx >= len(rec.Events) {
			t.Fatalf("grade keyed at event %d, out of range (%d events)", idx, len(rec.Events))
		}
		e := rec.Events[idx]
		if e.Kind != engine.EvAction || e.Seat != heroSeat {
			t.Errorf("grade keyed at event %d, which is %v by seat %d — not a hero action",
				idx, e.Kind, e.Seat)
		}
		if g.Band == "" {
			t.Errorf("grade at event %d has no band", idx)
		}
		if strings.ToLower(g.Band) != g.Band && strings.ToUpper(g.Band[:1]) != g.Band[:1] {
			t.Errorf("grade band %q is not display-cased", g.Band)
		}
		if g.EVLossBB < 0 {
			t.Errorf("grade at event %d has negative EV loss %v", idx, g.EVLossBB)
		}
	}
}

// TestGradesNeverOutnumberHeroActions: a misalignment would attribute a
// verdict to an action the hero never took, which is worse than showing none.
func TestGradesNeverOutnumberHeroActions(t *testing.T) {
	cfg := TableConfig{
		SmallBlind: 5, BigBlind: 10, Stack: 1000,
		Lineup: ClassroomLineup(), CoachMode: CoachFull, Speed: SpeedInstant,
	}
	var m tea.Model = NewTableScreen(cfg, DefaultPrefs(), profile.NewProfile())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Init()

	for i := 0; i < 25; i++ {
		m = driveKeys(m, "c", "x", " ", "\r")
		ts := m.(*TableScreen)
		rec, ann, ok := ts.lastHandArchive()
		if !ok {
			continue
		}
		heroActions := 0
		for _, e := range rec.Events {
			if e.Kind == engine.EvAction && e.Seat == heroSeat {
				heroActions++
			}
		}
		if len(ann.Grades) > heroActions {
			t.Fatalf("iteration %d: %d grades for %d hero actions", i, len(ann.Grades), heroActions)
		}
	}
}
