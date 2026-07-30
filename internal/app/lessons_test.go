package app

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// newLessonsScreen builds the curriculum screen over a throwaway profile
// (writes redirected to a temp dir) with the given lessons pre-completed,
// sized to w x h.
func newLessonsScreen(t *testing.T, w, h int, done ...string) (*Lessons, *profile.Profile) {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	p := profile.NewProfile()
	for _, id := range done {
		p.CompleteLesson(id)
	}
	l := NewLessons(p, DefaultPrefs())
	l.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return l, p
}

// squash collapses all whitespace to single spaces after stripping ANSI, so
// tests can look for prose fragments across wrap points.
func squash(s string) string {
	return strings.Join(strings.Fields(stripANSI(s)), " ")
}

// TestCurriculumListShowsEveryLesson: all registered lessons render, in
// curriculum order, with the progress count.
func TestCurriculumListShowsEveryLesson(t *testing.T) {
	lessons := tutorial.All()
	if len(lessons) != 13 {
		t.Fatalf("registry has %d lessons, the curriculum ships 13", len(lessons))
	}
	l, _ := newLessonsScreen(t, 80, 24)
	view := squash(l.View())
	for _, lesson := range lessons {
		title := lesson.Title
		if len(title) > 16 {
			title = title[:16] // rows truncate; the head of the title must survive
		}
		if !strings.Contains(view, title) {
			t.Errorf("list missing lesson %q", lesson.Title)
		}
	}
	if !strings.Contains(view, "0 of 13 complete") {
		t.Error("list missing the progress count")
	}

	// Every locked lesson carries its tag — including the longest-titled
	// row, which once lost the tag to a hidden style padding.
	for i, lesson := range lessons {
		row := stripANSI(l.lessonRow(i, lesson))
		if got := strings.Contains(row, "locked"); got != !lesson.Unlocked(l.prof) {
			t.Errorf("row %q: locked tag mismatch (lesson %s)", row, lesson.ID)
		}
	}
}

// TestLockedLessonExplainsAndGates: a locked lesson cannot be entered, its
// detail line says exactly which prerequisite to finish, and completing
// that prerequisite unlocks it.
func TestLockedLessonExplainsAndGates(t *testing.T) {
	l, p := newLessonsScreen(t, 80, 24)
	lessons := l.reg.All()

	// Find the first locked lesson (lesson 2 on a fresh profile).
	locked := -1
	for i, lesson := range lessons {
		if !lesson.Unlocked(p) {
			locked = i
			break
		}
	}
	if locked < 0 {
		t.Fatal("a fresh profile should have locked lessons")
	}
	l.cursor = locked
	lesson := lessons[locked]
	preTitle := lesson.Prerequisites[0]
	if pre, ok := l.reg.Get(preTitle); ok {
		preTitle = pre.Title
	}

	view := squash(l.View())
	if !strings.Contains(view, "finish "+preTitle+" first") {
		t.Fatalf("locked detail must say why; view: %q", view)
	}
	if !strings.Contains(view, "locked") {
		t.Error("locked row should carry the locked tag")
	}

	l.handleAction(ActSelect)
	if l.view != nil {
		t.Fatal("a locked lesson must not open")
	}

	// Completing the prerequisites unlocks it.
	for _, id := range lesson.Prerequisites {
		p.CompleteLesson(id)
	}
	l.handleAction(ActSelect)
	if l.view == nil {
		t.Fatal("lesson should open once its prerequisites are complete")
	}
}

// TestLessonsResumeAtFirstIncomplete: the cursor starts on the first
// incomplete lesson, and on the first lesson when everything is done.
func TestLessonsResumeAtFirstIncomplete(t *testing.T) {
	first := tutorial.All()[0]
	second := tutorial.All()[1]

	l, _ := newLessonsScreen(t, 80, 24, first.ID)
	if got := l.reg.All()[l.cursor].ID; got != second.ID {
		t.Errorf("resume cursor on %q, want %q", got, second.ID)
	}

	var all []string
	for _, lesson := range tutorial.All() {
		all = append(all, lesson.ID)
	}
	l, _ = newLessonsScreen(t, 80, 24, all...)
	if l.cursor != 0 {
		t.Errorf("finished curriculum should rest the cursor at the top, got %d", l.cursor)
	}
}

// TestLessonsCursorAdvancesAfterCompletion: closing a lesson that was just
// completed lands the cursor on the newly unlocked lesson, not the one just
// finished — while closing a revisited (already-complete) lesson leaves the
// cursor where it was (docs/ui-review.md F3).
func TestLessonsCursorAdvancesAfterCompletion(t *testing.T) {
	l, p := newLessonsScreen(t, 80, 24)
	lessons := l.reg.All()
	if l.cursor != 0 {
		t.Fatalf("fresh curriculum should start on lesson 1, cursor %d", l.cursor)
	}

	if !l.Open(lessons[0].ID) {
		t.Fatal("lesson 1 should open on a fresh profile")
	}
	// Completion is recorded on the profile (in real flow by the lesson
	// view's Progress.Record); the list re-reads the profile on close.
	p.CompleteLesson(lessons[0].ID)
	l.handleAction(ActBack)
	if l.view != nil {
		t.Fatal("esc should close the lesson view")
	}
	if got := lessons[l.cursor].ID; got != lessons[1].ID {
		t.Errorf("after completing lesson 1 the cursor is on %q, want %q", got, lessons[1].ID)
	}

	// Revisiting the completed lesson and closing it must not yank the
	// cursor forward.
	if !l.Open(lessons[0].ID) {
		t.Fatal("a completed lesson should reopen")
	}
	l.handleAction(ActBack)
	if l.cursor != 0 {
		t.Errorf("closing a revisited lesson moved the cursor to %d, want 0", l.cursor)
	}
}

// TestCompletedLessonShowsCheckmark: completion state renders as the good
// glyph and counts in the subtitle.
func TestCompletedLessonShowsCheckmark(t *testing.T) {
	first := tutorial.All()[0]
	l, _ := newLessonsScreen(t, 80, 24, first.ID)
	view := stripANSI(l.View())
	if !strings.Contains(view, theme.G.Good) {
		t.Error("completed lesson should render the good glyph")
	}
	if !strings.Contains(squash(view), "1 of 13 complete") {
		t.Error("progress count should reflect the completion")
	}
}

// TestLessonsListLayoutStable: the list holds its anchors and height at all
// three breakpoints while the cursor moves across locked and done rows.
func TestLessonsListLayoutStable(t *testing.T) {
	first := tutorial.All()[0]
	for _, bp := range breakpoints {
		l, _ := newLessonsScreen(t, bp.w, bp.h, first.ID)
		sized(t, l, bp.w, bp.h)

		assertAnchorsStable(t, l.View,
			[]string{"Lessons", "1. ", "13. "},
			map[string]func(){
				"cursor down":        func() { l.handleAction(ActDown) },
				"cursor to last":     func() { l.cursor = len(l.reg.All()) - 1 },
				"cursor wraps down":  func() { l.handleAction(ActDown) },
				"cursor wraps up":    func() { l.handleAction(ActUp) },
				"cursor to locked":   func() { l.cursor = 4 },
				"cursor back to top": func() { l.cursor = 0 },
			})
	}
}

// TestLessonListKeymap: the list handles exactly what it documents, ignores
// what it does not, and the help overlay renders the whole vocabulary.
func TestLessonListKeymap(t *testing.T) {
	l, _ := newLessonsScreen(t, 80, 24)
	for _, b := range l.keymap() {
		if b.Action == ActHelp {
			continue
		}
		fresh, _ := newLessonsScreen(t, 80, 24)
		if _, handled := fresh.handleAction(b.Action); !handled {
			t.Errorf("list: keymap documents %q (%s) but handleAction ignores it", b.Label, b.Help)
		}
	}

	for _, k := range []string{"z", "x", "f", "r", "b", "1", "9", "tab", "left", "right", "backspace"} {
		if l.keymap().Contains(k) {
			continue
		}
		before := l.View()
		_, cmd := l.Update(key(k))
		if cmd != nil {
			t.Errorf("list: undocumented key %q produced a command", k)
		}
		if l.View() != before {
			t.Errorf("list: undocumented key %q changed the view", k)
		}
	}

	l.Update(key("?"))
	view := l.View()
	for _, b := range lessonListKeys {
		if !strings.Contains(view, b.Label) || !strings.Contains(view, b.Help) {
			t.Errorf("help overlay missing %q / %q", b.Label, b.Help)
		}
	}
	if !strings.Contains(view, "Press any key to close") {
		t.Error("help overlay must say how to close")
	}
}

// TestLessonKeymapKeysAreUnique: within each lesson keymap a key resolves
// to exactly one action (the dispatch premise, mirrored from keymap.go's
// own test for the maps defined in the lesson files).
func TestLessonKeymapKeysAreUnique(t *testing.T) {
	maps := map[string]KeyMap{
		"lessonListKeys":         lessonListKeys,
		"lessonSectionKeys":      lessonSectionKeys,
		"lessonDrillKeys":        lessonDrillKeys,
		"lessonScriptPauseKeys":  lessonScriptPauseKeys,
		"lessonScriptActKeys":    lessonScriptActKeys,
		"lessonScriptSizingKeys": lessonScriptSizingKeys,
		"lessonSplashKeys":       lessonSplashKeys,
	}
	for name, km := range maps {
		seen := map[string]bool{}
		for _, b := range km {
			for _, k := range b.Keys {
				if seen[k] {
					t.Errorf("%s: key %q bound twice", name, k)
				}
				seen[k] = true
			}
		}
	}
}
