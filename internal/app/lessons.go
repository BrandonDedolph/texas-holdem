package app

import (
	"strconv"
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"

	// The curriculum registers itself at init time (the euchre registry
	// pattern); importing content here is what puts the 13 lessons in
	// tutorial.DefaultRegistry when the app links this screen.
	_ "github.com/BrandonDedolph/texas-holdem/internal/tutorial/content"

	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// Lessons is the curriculum screen (docs/design-tui.md §1.1, ScreenLessons):
// the 13-lesson list with completion checkmarks and prerequisite gating, and
// — once a lesson is opened — the in-lesson section stepper in
// lesson_view.go.
//
// The list and the open lesson share one screen model rather than two
// Screens because the lesson view has no life of its own: it always returns
// to the list, and the list is where progress (checkmarks, unlocks) is
// re-read after every completion.
type Lessons struct {
	prof  *profile.Profile
	prefs *Prefs // reserved for pacing preferences; scripted hands run synchronously today
	reg   *tutorial.Registry

	cursor int
	view   *lessonView // non-nil while a lesson is open

	help          helpOverlay
	width, height int
}

// NewLessons builds the curriculum screen over the default registry, with
// the cursor on the first incomplete lesson — the "resume where you left
// off" behaviour a returning learner expects (design-learning.md §8.1).
func NewLessons(prof *profile.Profile, prefs *Prefs) *Lessons {
	return newLessonsWithRegistry(tutorial.DefaultRegistry, prof, prefs)
}

// newLessonsWithRegistry exists so tests can drive small curricula without
// touching the global registry (which panics on duplicate registration).
func newLessonsWithRegistry(reg *tutorial.Registry, prof *profile.Profile, prefs *Prefs) *Lessons {
	l := &Lessons{prof: prof, prefs: prefs, reg: reg}
	l.cursor = l.resumeIndex()
	return l
}

// resumeIndex is the list index of the first incomplete lesson (in Order),
// or 0 when the whole curriculum is done.
func (l *Lessons) resumeIndex() int {
	next := l.reg.Next(l.prof)
	if next == nil {
		return 0
	}
	for i, lesson := range l.reg.All() {
		if lesson.ID == next.ID {
			return i
		}
	}
	return 0
}

// Open opens a lesson by ID, honouring prerequisite gating: a locked lesson
// does not open and the list's detail line already states why. It reports
// whether the lesson opened. App routing calls this for a LessonRequest
// payload; the list's enter key goes through the same gate.
func (l *Lessons) Open(id string) bool {
	lesson, ok := l.reg.Get(id)
	if !ok || !lesson.Unlocked(l.prof) {
		return false
	}
	for i, cand := range l.reg.All() {
		if cand.ID == id {
			l.cursor = i
		}
	}
	l.view = newLessonView(lesson, l.prof)
	return true
}

// Init implements tea.Model.
func (l *Lessons) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (l *Lessons) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		l.width, l.height = msg.Width, msg.Height
	case tea.KeyMsg:
		cmd, _ := l.help.dispatch(l.keymap(), msg, l.handleAction)
		return l, cmd
	}
	return l, nil
}

// keymap returns the bindings live in the current state — the list's, or the
// open lesson's state-dependent set. The help overlay and the legend tests
// read the same value, so keys-that-act and keys-on-the-sheet cannot drift.
func (l *Lessons) keymap() KeyMap {
	if l.view != nil {
		return l.view.keymap()
	}
	return lessonListKeys
}

// handleAction performs one key action, delegating to the open lesson view
// when there is one. The view never emits navigation commands itself — it
// only reports whether it wants to close, which keeps every screen
// transition in this one place.
func (l *Lessons) handleAction(a KeyAction) (tea.Cmd, bool) {
	if l.view != nil {
		outcome, handled := l.view.handleAction(a)
		if outcome == lessonClose {
			// Re-read progress: the closed lesson may have completed, which
			// changes checkmarks, unlocks, and the resume position.
			l.view = nil
		}
		return nil, handled
	}

	lessons := l.reg.All()
	n := len(lessons)
	switch a {
	case ActUp:
		if n > 0 {
			l.cursor = (l.cursor - 1 + n) % n
		}
		return nil, true
	case ActDown:
		if n > 0 {
			l.cursor = (l.cursor + 1) % n
		}
		return nil, true
	case ActSelect:
		if n > 0 {
			// Open goes through the same prerequisite gate as LessonRequest
			// routing; on a locked lesson it declines and the detail line
			// (which already reads "Locked · finish X first") is the answer.
			l.Open(lessons[l.cursor].ID)
		}
		return nil, true
	case ActBack:
		return Navigate(ScreenMainMenu), true
	}
	return nil, false
}

// View implements tea.Model.
func (l *Lessons) View() string {
	w, h := fallbackSize(l.width, l.height)
	if l.help.open {
		title := "Lessons"
		if l.view != nil {
			title = l.view.lesson.Title
		}
		return renderHelp(title, l.keymap(), w, h)
	}
	if l.view != nil {
		return l.view.render(w, h)
	}
	return l.renderList(w, h)
}

// lessonListWidth is the fixed inner width of the curriculum list; rows are
// padded to it so cursor movement can never shift a column.
const lessonListWidth = 46

// renderList draws the curriculum: title, progress, the 13 rows, and a
// detail line for the selected lesson. The compact breakpoint drops the
// subtitle and spacer rows so all 13 lessons still fit a 60x20 frame.
func (l *Lessons) renderList(w, h int) string {
	th := theme.Current
	lessons := l.reg.All()
	compact := layoutFor(w, h) == LayoutCompact
	dot := " " + theme.G.Dot + " "

	lines := []string{th.Title.Render("Lessons")}
	if !compact {
		done := 0
		for _, lesson := range lessons {
			if lesson.Completed(l.prof) {
				done++
			}
		}
		lines = append(lines,
			th.Subtitle.Render(strconv.Itoa(done)+" of "+strconv.Itoa(len(lessons))+" complete"),
			"")
	}
	for i, lesson := range lessons {
		lines = append(lines, l.lessonRow(i, lesson))
	}
	if !compact {
		lines = append(lines, "")
	}
	lines = append(lines, th.Help.Render(padRow(l.detailLine(), lessonListWidth)))
	lines = append(lines, th.Help.Render(padRow(
		"up/down move"+dot+"enter open"+dot+"esc back"+dot+"? help", lessonListWidth)))
	return frame(w, h, strings.Join(lines, "\n"))
}

// lessonRow renders one curriculum row at exactly lessonListWidth cells:
// cursor marker, completion mark, number and title, and a right-aligned
// "locked" tag. Locked rows stay selectable — the requirement is that a
// locked lesson explains itself, which needs the cursor to reach it.
func (l *Lessons) lessonRow(i int, lesson *tutorial.Lesson) string {
	th := theme.Current
	done := lesson.Completed(l.prof)
	locked := !lesson.Unlocked(l.prof)

	// The MenuItem styles carry a built-in left padding for whole-row
	// rendering (formList's convention); this row is composed from styled
	// segments, so the padding is stripped or every segment would grow the
	// row by two cells.
	rowStyle := th.MenuItem.UnsetPaddingLeft()
	switch {
	case locked:
		rowStyle = th.MenuItemDisabled.UnsetPaddingLeft()
	case i == l.cursor:
		rowStyle = th.MenuItemSelected.UnsetPaddingLeft()
	}

	cursor := "  "
	if i == l.cursor {
		cursor = "> "
	}
	mark, markStyle := theme.G.Dot, th.Help
	if done {
		mark, markStyle = theme.G.Good, th.GradeGood
	}

	left := rowStyle.Render(cursor) + markStyle.Render(mark) +
		rowStyle.Render(" "+padRow(strconv.Itoa(lesson.Order)+". ", 4)+lesson.Title)
	right := ""
	if locked {
		right = th.MenuItemDisabled.UnsetPaddingLeft().Render("locked")
	}
	return rowLR(lessonListWidth, left, right)
}

// detailLine is the one-line description of the selected lesson: its goal,
// or — for a locked lesson — exactly why it is locked and what to do about
// it ("finish Position first"). Never a bare disabled row.
func (l *Lessons) detailLine() string {
	lessons := l.reg.All()
	if len(lessons) == 0 {
		return ""
	}
	if l.cursor >= len(lessons) {
		l.cursor = len(lessons) - 1
	}
	lesson := lessons[l.cursor]
	dot := " " + theme.G.Dot + " "
	switch {
	case !lesson.Unlocked(l.prof):
		return "Locked" + dot + l.lockReason(lesson)
	case lesson.Completed(l.prof):
		return "Completed" + dot + lesson.Goal
	default:
		return lesson.Goal
	}
}

// lockReason names the incomplete prerequisites by their lesson titles:
// "finish Position first". IDs are the fallback for a prerequisite pointing
// at an unregistered lesson (a content bug the curriculum test catches).
func (l *Lessons) lockReason(lesson *tutorial.Lesson) string {
	var missing []string
	for _, id := range lesson.Prerequisites {
		if _, done := l.prof.LessonsDone[id]; done {
			continue
		}
		title := id
		if pre, ok := l.reg.Get(id); ok {
			title = pre.Title
		}
		missing = append(missing, title)
	}
	return "finish " + strings.Join(missing, " and ") + " first"
}

// lessonListKeys is the curriculum list's complete key vocabulary.
var lessonListKeys = KeyMap{
	bindUp, bindDown,
	Binding{ActSelect, []string{"enter", " "}, "enter/space", "open lesson"},
	backBinding("back to menu"),
	bindHelp,
}
