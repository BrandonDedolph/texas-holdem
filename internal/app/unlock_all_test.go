package app

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
	tea "github.com/charmbracelet/bubbletea"
)

// The "Lesson locks: All open" setting (docs/ui-review.md §5.4): lockstep
// stays the default, and the setting lifts every prerequisite gate — in the
// list, in Open's gating, and across restarts via the profile.

// lastLockedLesson returns the highest-order lesson a fresh profile cannot
// enter — the deepest gate the setting must lift.
func lastLockedLesson(t *testing.T, prof *profile.Profile) *tutorial.Lesson {
	t.Helper()
	all := tutorial.All()
	for i := len(all) - 1; i >= 0; i-- {
		if !all[i].Unlocked(prof) {
			return all[i]
		}
	}
	t.Fatal("a fresh profile should have locked lessons")
	return nil
}

// TestLessonLocksDefaultOn: lockstep is the default — a fresh profile has
// gates, and the list says so.
func TestLessonLocksDefaultOn(t *testing.T) {
	prof := profile.NewProfile()
	if prof.UnlockAllLessons {
		t.Fatal("UnlockAllLessons must default to false: the lockstep curriculum is the pedagogy")
	}
	l := NewLessons(prof, DefaultPrefs())
	l.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	deep := lastLockedLesson(t, prof)
	if l.Open(deep.ID) {
		t.Fatalf("locked lesson %s must not open by default", deep.ID)
	}
	if !strings.Contains(squash(l.View()), "locked") {
		t.Error("the default list should carry locked tags")
	}
}

// TestUnlockAllLiftsEveryGate: with the setting on, every previously locked
// lesson opens and no row reads locked — completion state is untouched.
func TestUnlockAllLiftsEveryGate(t *testing.T) {
	prof := profile.NewProfile()
	prof.UnlockAllLessons = true
	l := NewLessons(prof, DefaultPrefs())
	l.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	if view := squash(l.View()); strings.Contains(view, "locked") {
		t.Errorf("no row should read locked with every gate lifted; view: %q", view)
	}
	for _, lesson := range tutorial.All() {
		if !l.Open(lesson.ID) {
			t.Errorf("lesson %s should open with all locks lifted", lesson.ID)
		}
		l.view = nil // close without completing
		if lesson.Completed(prof) {
			t.Errorf("opening %s must not mark it complete", lesson.ID)
		}
	}
}

// TestUnlockAllRoundTripsThroughSettings: cycling the Settings row flips
// the profile field, persists it, and a reload from disk still has it —
// then cycling back restores the default.
func TestUnlockAllRoundTripsThroughSettings(t *testing.T) {
	dir := t.TempDir()
	store := profile.StoreAt(dir)
	prof, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}

	s := NewSettings(prof, DefaultPrefs())
	s.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if got := s.list.rows[settingLocks].Value(); got != "In order" {
		t.Fatalf("Lesson locks should default to In order, got %q", got)
	}

	s.list.cursor = settingLocks
	s.handleAction(ActRight)
	if !prof.UnlockAllLessons {
		t.Fatal("cycling Lesson locks to All open must set the profile field")
	}

	reloaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.UnlockAllLessons {
		t.Error("UnlockAllLessons must survive a save/load round-trip")
	}

	s.handleAction(ActRight) // wraps back to In order
	if prof.UnlockAllLessons {
		t.Error("cycling back must restore the lockstep default")
	}
	if reloaded, err = store.Load(); err != nil || reloaded.UnlockAllLessons {
		t.Errorf("restored default must persist too (err %v)", err)
	}
}
