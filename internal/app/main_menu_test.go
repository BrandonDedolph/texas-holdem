package app

import (
	"strings"
	"testing"
	"time"

	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// newMenu builds a MainMenu over the given profile, sized to w x h.
func newMenu(prof *profile.Profile, w, h int) *MainMenu {
	m := NewMainMenu(prof)
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

// TestMainMenuFreshProfile: a brand-new profile starts the cursor on Lessons
// — the beginner's door — and the Lessons row says "start here" rather than
// phrasing zero progress as failure.
func TestMainMenuFreshProfile(t *testing.T) {
	m := newMenu(profile.NewProfile(), 80, 24)
	if m.cursor != menuLessons {
		t.Errorf("fresh profile cursor = %d, want menuLessons (%d)", m.cursor, menuLessons)
	}

	view := squash(m.View())
	want := "start here " + theme.G.Dot + " 13 lessons"
	if !strings.Contains(view, want) {
		t.Errorf("fresh Lessons row should read %q; view: %q", want, view)
	}
	if strings.Contains(view, "0 of 13") {
		t.Error("zero progress must not be phrased as \"0 of 13\"")
	}
	if strings.Contains(view, "last session") {
		t.Error("no session has been played; Play must not claim one")
	}
	if strings.Contains(view, "weakest") {
		t.Error("nothing has been drilled; Trainer must stay quiet")
	}
}

// TestMainMenuDefaultCursorResumes: the cursor starts on whichever thread —
// lessons or play — the profile's timestamps say was picked up last, and on
// Play once the curriculum is finished.
func TestMainMenuDefaultCursorResumes(t *testing.T) {
	first := tutorial.All()[0]

	// Latest activity is a lesson: resume on Lessons.
	p := profile.NewProfile()
	p.LessonsDone[first.ID] = time.Now().UTC()
	p.SessionLog = append(p.SessionLog,
		profile.SessionSummary{Start: time.Now().UTC().Add(-time.Hour), Hands: 20})
	if m := newMenu(p, 80, 24); m.cursor != menuLessons {
		t.Errorf("lesson finished last: cursor = %d, want menuLessons", m.cursor)
	}

	// Latest activity is a table session: resume on Play.
	p = profile.NewProfile()
	p.LessonsDone[first.ID] = time.Now().UTC().Add(-time.Hour)
	p.SessionLog = append(p.SessionLog,
		profile.SessionSummary{Start: time.Now().UTC(), Hands: 20})
	if m := newMenu(p, 80, 24); m.cursor != menuPlay {
		t.Errorf("session played last: cursor = %d, want menuPlay", m.cursor)
	}

	// Curriculum complete: the table is the next teacher, regardless of
	// which thread was touched last.
	p = profile.NewProfile()
	for _, l := range tutorial.All() {
		p.CompleteLesson(l.ID)
	}
	if m := newMenu(p, 80, 24); m.cursor != menuPlay {
		t.Errorf("curriculum done: cursor = %d, want menuPlay", m.cursor)
	}
}

// TestMainMenuRowsReflectProfile: the statuses state what the profile holds
// — lessons done and the next one up, the last session's hand count, the
// weakest drilled skill — and the detail slot captions the selected row.
func TestMainMenuRowsReflectProfile(t *testing.T) {
	lessons := tutorial.All()
	p := profile.NewProfile()
	p.CompleteLesson(lessons[0].ID)
	p.CompleteLesson(lessons[1].ID)
	p.SessionLog = append(p.SessionLog, profile.SessionSummary{Hands: 40})
	p.DrillStats["outs"] = profile.SkillStat{EMA: 0.55, Attempts: 12}
	p.DrillStats["rankings"] = profile.SkillStat{EMA: 0.92, Attempts: 25}

	m := newMenu(p, 80, 24)
	m.cursor = menuLessons
	view := squash(m.View())

	next := "next: 3. " + lessons[2].Title
	if len(next) > 40 {
		next = next[:40] // rows truncate; the head must survive
	}
	if !strings.Contains(view, next) {
		t.Errorf("Lessons row should point at the next lesson %q; view: %q", next, view)
	}
	if !strings.Contains(view, "2 of 13 lessons complete") {
		t.Errorf("detail slot should count progress; view: %q", view)
	}
	if !strings.Contains(view, "40 hands") {
		t.Errorf("Play row should carry the last session size; view: %q", view)
	}
	if !strings.Contains(view, "weakest: Outs Quiz") {
		t.Errorf("Trainer row should name the weakest drilled skill; view: %q", view)
	}
}

// TestMainMenuTrainerQuietWhenOnTrack: a drilled skill at or above the 80%
// mastery bar is not "weak" — the Trainer row stays silent instead of
// inventing something to say.
func TestMainMenuTrainerQuietWhenOnTrack(t *testing.T) {
	p := profile.NewProfile()
	p.DrillStats["outs"] = profile.SkillStat{EMA: 0.85, Attempts: 30}
	m := newMenu(p, 80, 24)
	if view := squash(m.View()); strings.Contains(view, "weakest") {
		t.Errorf("skills at the bar must not be labeled weakest; view: %q", view)
	}
}

// TestMainMenuAllLessonsComplete: a finished curriculum reads as an
// achievement and the cursor rests on Play.
func TestMainMenuAllLessonsComplete(t *testing.T) {
	p := profile.NewProfile()
	for _, l := range tutorial.All() {
		p.CompleteLesson(l.ID)
	}
	m := newMenu(p, 80, 24)
	view := squash(m.View())
	if !strings.Contains(view, "all 13 complete") {
		t.Errorf("Lessons row should state completion; view: %q", view)
	}
	if strings.Contains(view, "start here") {
		t.Error("a finished curriculum is not the place to start")
	}
}

// TestMainMenuSelectRoutesEveryRow: each row's enter lands on its screen —
// the cursor-indexed switch cannot drift from the row order.
func TestMainMenuSelectRoutesEveryRow(t *testing.T) {
	want := map[int]Screen{
		menuPlay:     ScreenGameSetup,
		menuLessons:  ScreenLessons,
		menuTrainer:  ScreenTrainer,
		menuQuickRef: ScreenQuickReference,
		menuSettings: ScreenSettings,
	}
	for row, screen := range want {
		m := newMenu(profile.NewProfile(), 80, 24)
		m.cursor = row
		cmd, handled := m.handleAction(ActSelect)
		if !handled || cmd == nil {
			t.Fatalf("row %d: select must produce a command", row)
		}
		msg, ok := cmd().(NavigateMsg)
		if !ok || msg.Screen != screen {
			t.Errorf("row %d routed to %v, want %v", row, msg.Screen, screen)
		}
	}

	m := newMenu(profile.NewProfile(), 80, 24)
	m.cursor = menuQuit
	cmd, _ := m.handleAction(ActSelect)
	if _, ok := cmd().(QuitMsg); !ok {
		t.Error("Quit row must emit QuitMsg")
	}
}
