package app

import (
	"strconv"
	"strings"
	"time"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/trainer"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MainMenu is the entry screen: Play / Lessons / Trainer / Quick Reference /
// Settings / Quit.
//
// The menu is the one place the whole product is visible at once, so its
// rows carry state (docs/ui-review.md F3): each row shows a quiet
// right-aligned status derived from the profile — the next lesson, the last
// session's size, the weakest drill — and the cursor starts on the door the
// profile says is the player's path, not unconditionally on Play.
type MainMenu struct {
	// confirmQuit is set by the first esc; the second one actually quits.
	confirmQuit bool

	prof *profile.Profile
	reg  *tutorial.Registry

	cursor int
	help   helpOverlay
	width  int
	height int
}

// Main menu rows, in list order. Indexed by the constants below so
// handleSelect cannot drift from the row slice.
const (
	menuPlay = iota
	menuLessons
	menuTrainer
	menuQuickRef
	menuSettings
	menuQuit
	menuRowCount
)

// NewMainMenu builds the main menu over the player's profile; statuses are
// re-read from it on every render, so returning from a lesson or a session
// refreshes the rows without any event plumbing.
func NewMainMenu(prof *profile.Profile) *MainMenu {
	m := &MainMenu{prof: prof, reg: tutorial.DefaultRegistry}
	m.cursor = m.defaultCursor()
	return m
}

// defaultCursor is the menu's one act of guidance: where the cursor starts.
// The rule is "resume the most recent thread": a profile whose latest
// activity is a lesson (including the empty profile, whose next activity is
// lesson 1) starts on Lessons; a profile whose latest activity is a table
// session starts on Play; a finished curriculum always starts on Play — the
// lessons themselves say the table is where learning continues.
func (m *MainMenu) defaultCursor() int {
	if m.prof == nil {
		return menuPlay
	}
	if m.reg.Next(m.prof) == nil {
		return menuPlay // curriculum complete: the table is the next teacher
	}
	var lastLesson, lastPlay time.Time
	for _, ts := range m.prof.LessonsDone {
		if ts.After(lastLesson) {
			lastLesson = ts
		}
	}
	for _, s := range m.prof.SessionLog {
		if s.Start.After(lastPlay) {
			lastPlay = s.Start
		}
	}
	if lastPlay.After(lastLesson) {
		return menuPlay
	}
	return menuLessons
}

// menuRow is one rendered menu row: the door's name, a quiet right-aligned
// status carrying real progress (blank when there is none — silence, not a
// nag), and the one-line description shown in the detail slot while the row
// is selected.
type menuRow struct {
	label  string
	status string
	detail string
}

// rows builds the six rows with statuses read live from the profile. Every
// status states a fact the profile actually holds; a fresh profile gets
// "start here" on Lessons and blank statuses elsewhere.
func (m *MainMenu) rows() []menuRow {
	rows := []menuRow{
		{label: "Play", detail: "6-max cash game against five AI opponents"},
		{label: "Lessons", detail: "Guided lessons from rankings to bet sizing"},
		{label: "Trainer", detail: "Drills: rank hands, count outs, judge equity"},
		{label: "Quick Reference", detail: "Rankings, positions, pot odds and glossary"},
		{label: "Settings", detail: "Theme, deck colors, speed, coach verbosity"},
		{label: "Quit", detail: "Leave the table"},
	}
	if m.prof == nil {
		return rows
	}
	rows[menuPlay].status = m.playStatus()
	rows[menuLessons].status, rows[menuLessons].detail = m.lessonsState(rows[menuLessons].detail)
	rows[menuTrainer].status = m.trainerStatus()
	return rows
}

// playStatus reports the last logged session's size. Blank until a session
// has been logged — nothing is claimed that never happened.
func (m *MainMenu) playStatus() string {
	log := m.prof.SessionLog
	if len(log) == 0 {
		return ""
	}
	hands := log[len(log)-1].Hands
	label := strconv.Itoa(hands) + " hands"
	if hands == 1 {
		label = "1 hand"
	}
	return "last session " + theme.G.Dot + " " + label
}

// lessonsState is the Lessons row's status and detail: "start here" on a
// fresh profile, the next incomplete lesson while the curriculum is under
// way, and a completion statement once it is done. The count lives in the
// detail line, phrased as progress, never as failure.
func (m *MainMenu) lessonsState(freshDetail string) (status, detail string) {
	lessons := m.reg.All()
	total := strconv.Itoa(len(lessons))
	done := 0
	for _, l := range lessons {
		if l.Completed(m.prof) {
			done++
		}
	}
	next := m.reg.Next(m.prof)
	dot := " " + theme.G.Dot + " "
	switch {
	case next == nil:
		return "all " + total + " complete",
			"Every lesson done" + dot + "revisit any, in any order"
	case done == 0:
		return "start here" + dot + total + " lessons", freshDetail
	default:
		return "next: " + strconv.Itoa(next.Order) + ". " + next.Title,
			strconv.Itoa(done) + " of " + total + " lessons complete"
	}
}

// trainerStatus names the drilled quiz whose gate skill is furthest below
// the 80% mastery bar — the one worth practising next. Quiet when nothing
// has been drilled yet, and quiet again when every practised skill is at or
// above the bar: no news is the reward.
func (m *MainMenu) trainerStatus() string {
	weakest := trainer.QuizKind(-1)
	worst := 1.0
	for kind := trainer.QuizKind(0); kind < trainer.NumQuizKinds; kind++ {
		s := m.prof.DrillStats[kind.String()]
		if s.Attempts == 0 {
			continue // never drilled (or a fresh level): nothing to report
		}
		if s.EMA < worst {
			worst, weakest = s.EMA, kind
		}
	}
	if weakest < 0 || worst >= 0.8 {
		return ""
	}
	return "weakest: " + weakest.Title()
}

// Init implements tea.Model.
func (m *MainMenu) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *MainMenu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		cmd, _ := m.help.dispatch(m.keymap(), msg, m.handleAction)
		return m, cmd
	}
	return m, nil
}

func (m *MainMenu) keymap() KeyMap { return mainMenuKeys }

func (m *MainMenu) handleAction(a KeyAction) (tea.Cmd, bool) {
	// Anything that is not esc means the player was not trying to leave, so
	// a later stray esc asks again rather than quitting on what feels to
	// them like a first press.
	if a != ActBack {
		m.confirmQuit = false
	}
	switch a {
	case ActUp:
		m.cursor = (m.cursor - 1 + menuRowCount) % menuRowCount
		return nil, true
	case ActDown:
		m.cursor = (m.cursor + 1) % menuRowCount
		return nil, true
	case ActSelect:
		return m.handleSelect(), true
	case ActBack:
		// Every other screen treats esc as "back", so a bare esc quitting the
		// app contradicts the muscle memory the rest of the product builds.
		// Ask once (docs/ui-review.md §5 q2).
		if m.confirmQuit {
			return Quit(), true
		}
		m.confirmQuit = true
		return nil, true
	}
	return nil, false
}

// handleSelect routes the selected row.
func (m *MainMenu) handleSelect() tea.Cmd {
	switch m.cursor {
	case menuPlay:
		return Navigate(ScreenGameSetup)
	case menuLessons:
		return Navigate(ScreenLessons)
	case menuTrainer:
		return Navigate(ScreenTrainer)
	case menuQuickRef:
		return Navigate(ScreenQuickReference)
	case menuSettings:
		return Navigate(ScreenSettings)
	case menuQuit:
		return Quit()
	}
	return nil
}

// mainMenuWidth is the fixed inner width of the menu box; every row is laid
// out to exactly this many cells so cursor moves and status changes can
// never shift a column.
const mainMenuWidth = 46

// renderList draws the six rows. The selected row's detail renders on the
// shell's status row (the chrome rule), not below the list — the old
// bottom-of-box slot read as a caption for whichever row happened to be
// last (the F3 complaint: "6-max cash game..." directly under "Quit").
func (m *MainMenu) renderList(rows []menuRow) string {
	th := theme.Current
	var b strings.Builder
	for i, r := range rows {
		// The MenuItem styles carry a built-in left padding for whole-row
		// rendering (formList's convention); these rows are composed from
		// styled segments, so the padding is stripped or each segment would
		// grow the row by two cells.
		style := th.MenuItem.UnsetPaddingLeft()
		cursor := "  "
		if i == m.cursor {
			style = th.MenuItemSelected.UnsetPaddingLeft()
			cursor = "> "
		}
		left := style.Render(cursor + r.label)
		right := ""
		if r.status != "" {
			right = th.Help.Render(r.status)
		}
		b.WriteString(rowLR(mainMenuWidth, left, right))
		if i < len(rows)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// View implements tea.Model.
func (m *MainMenu) View() string {
	w, h := fallbackSize(m.width, m.height)
	if m.help.open {
		return renderHelp("Main Menu", m.keymap(), w, h)
	}
	th := theme.Current

	rows := m.rows()
	subtitle := th.Subtitle.Render("Learn no-limit hold'em one decision at a time")
	content := lipgloss.PlaceHorizontal(mainMenuWidth, lipgloss.Center, subtitle) +
		"\n\n" + m.renderList(rows)

	return renderShell(w, h, shell{
		Title:      "TEXAS HOLD'EM",
		TitleExtra: renderSuitBanner(),
		Status:     m.statusRow(rows),
		Footer: "up/down move " + theme.G.Dot + " enter select " +
			theme.G.Dot + " esc quit",
	}, content)
}

// renderSuitBanner draws the four suits in their deck colors — a tiny,
// always-visible advertisement of the four-color deck the player will see
// on the table — for the header's title decoration.
func renderSuitBanner() string {
	g := theme.G
	return " " + theme.Current.Header.Render(g.Dot) + " " +
		theme.SuitStyle(engine.Spades).Render(g.SuitSpade) + " " +
		theme.SuitStyle(engine.Hearts).Render(g.SuitHeart) + " " +
		theme.SuitStyle(engine.Diamonds).Render(g.SuitDiamond) + " " +
		theme.SuitStyle(engine.Clubs).Render(g.SuitClub)
}

// statusRow is the selected row's detail, unless a quit confirmation is
// pending — an unanswered question outranks a description.
func (m *MainMenu) statusRow(rows []menuRow) string {
	if m.confirmQuit {
		return "press esc again to quit"
	}
	return rows[m.cursor].detail
}
