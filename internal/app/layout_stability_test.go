package app

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Layout stability (docs/design-tui.md §8.1): anchors hold their exact
// (row, col) and the total view height never changes as state mutates, at
// every breakpoint. posOf/assertAnchorsStable are euchre's helpers, ported.

// pos is the row/column of a marker within a rendered view.
type pos struct{ row, col int }

// posOf returns the row and column of the first occurrence of marker, or
// {-1,-1} if it never appears. Column is measured in runes so multibyte
// glyphs don't skew it.
func posOf(view, marker string) pos {
	for i, line := range strings.Split(view, "\n") {
		if c := strings.Index(line, marker); c >= 0 {
			return pos{i, len([]rune(line[:c]))}
		}
	}
	return pos{-1, -1}
}

// assertAnchorsStable renders the view in each mutated state and checks
// that every anchor stays at its baseline position and the total height
// never changes. Mutations run in map order (deliberately arbitrary — the
// invariant must hold from any state to any state).
func assertAnchorsStable(t *testing.T, view func() string, anchors []string, mutate map[string]func()) {
	t.Helper()

	base := view()
	baseHeight := lipgloss.Height(base)
	want := map[string]pos{}
	for _, a := range anchors {
		p := posOf(base, a)
		if p.row < 0 {
			t.Fatalf("anchor %q not found in baseline view:\n%s", a, base)
		}
		want[a] = p
	}

	for name, fn := range mutate {
		fn()
		v := view()
		for _, a := range anchors {
			if got := posOf(v, a); got != want[a] {
				t.Errorf("state %q: anchor %q moved from %+v to %+v", name, a, want[a], got)
			}
		}
		if got := lipgloss.Height(v); got != baseHeight {
			t.Errorf("state %q: total view height changed from %d to %d", name, baseHeight, got)
		}
	}
}

// breakpoints are the three supported sizes every stability test runs at.
var breakpoints = []struct{ w, h int }{
	{80, 24}, {104, 30}, {60, 20},
}

// sized sends a WindowSizeMsg and asserts the rendered view fills the
// terminal height exactly — the outermost stability guarantee.
func sized(t *testing.T, m tea.Model, w, h int) {
	t.Helper()
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	if got := lipgloss.Height(m.View()); got != h {
		t.Fatalf("view height %d at %dx%d, want exactly %d", got, w, h, h)
	}
}

func TestMainMenuLayoutStable(t *testing.T) {
	// A mid-journey profile so the status column is populated: the detail
	// rows and right-aligned statuses must not make the menu reflow.
	prof := profile.NewProfile()
	for _, id := range []string{"hand-rankings", "hand-flow"} {
		prof.CompleteLesson(id)
	}
	prof.SessionLog = append(prof.SessionLog, profile.SessionSummary{Hands: 38})
	prof.DrillStats["outs"] = profile.SkillStat{EMA: 0.55, Attempts: 12}

	for _, bp := range breakpoints {
		m := NewMainMenu(prof)
		sized(t, m, bp.w, bp.h)

		assertAnchorsStable(t, m.View,
			[]string{"TEXAS HOLD'EM", "Play", "Quit"},
			map[string]func(){
				"cursor to lessons":  func() { m.cursor = menuLessons },
				"cursor to quit":     func() { m.cursor = menuQuit },
				"wrap back to top":   func() { m.handleAction(ActDown) },
				"cursor up wraps":    func() { m.handleAction(ActUp) },
				"back to first row":  func() { m.cursor = menuPlay },
				"long detail row":    func() { m.cursor = menuQuickRef }, // longest detail line
				"short detail row":   func() { m.cursor = menuQuit },
				"first row again ok": func() { m.cursor = menuPlay },
			})
	}
}

func TestGameSetupLayoutStable(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	for _, bp := range breakpoints {
		g := NewGameSetup(profile.NewProfile(), DefaultPrefs())
		sized(t, g, bp.w, bp.h)

		assertAnchorsStable(t, g.View,
			[]string{"Game Setup", "Start Game", "Blinds", "Back"},
			map[string]func(){
				"cursor moves":       func() { g.handleAction(ActDown) },
				"cycle blinds":       func() { g.list.cursor = setupBlinds; g.handleAction(ActRight) },
				"cycle stack":        func() { g.list.cursor = setupStack; g.handleAction(ActRight) },
				"cycle lineup long":  func() { g.list.cursor = setupLineup; g.handleAction(ActRight) },
				"cycle lineup again": func() { g.handleAction(ActRight) },
				"cycle coach":        func() { g.list.cursor = setupCoach; g.handleAction(ActRight) },
				"cycle speed":        func() { g.list.cursor = setupSpeed; g.handleAction(ActRight) },
				"back to start row":  func() { g.list.cursor = setupStart },
			})
	}
}

func TestSettingsLayoutStable(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	// Settings changes hit theme globals; restore them so other tests see
	// the defaults regardless of run order.
	defer func() {
		theme.SetDeckMode(theme.FourColor)
		theme.G = theme.Unicode()
	}()

	for _, bp := range breakpoints {
		s := NewSettings(profile.NewProfile(), DefaultPrefs())
		sized(t, s, bp.w, bp.h)

		assertAnchorsStable(t, s.View,
			[]string{"Settings", "Theme", "Coach", "Back"},
			map[string]func(){
				"cursor moves":     func() { s.handleAction(ActDown) },
				"cycle deck":       func() { s.list.cursor = settingDeck; s.handleAction(ActRight) },
				"deck back":        func() { s.handleAction(ActLeft) },
				"cycle coach mode": func() { s.list.cursor = settingCoach; s.handleAction(ActRight) },
				"cycle speed":      func() { s.list.cursor = settingSpeed; s.handleAction(ActRight) },
				"top row again":    func() { s.list.cursor = settingTheme },
			})
	}
}

func TestQuickReferenceLayoutStable(t *testing.T) {
	for _, bp := range breakpoints {
		q := NewQuickReference()
		sized(t, q, bp.w, bp.h)

		// The chrome — title, tab bar, footer — must hold position while
		// the panels (all different heights) swap underneath.
		assertAnchorsStable(t, q.View,
			[]string{"Quick Reference", "1 Rankings", "4 Glossary"},
			map[string]func(){
				"positions tab": func() { q.handleAction(ActTab2) },
				"pot odds tab":  func() { q.handleAction(ActTab3) },
				"glossary tab":  func() { q.handleAction(ActTab4) },
				"cycle right":   func() { q.handleAction(ActRight) }, // wraps to rankings
				"cycle left":    func() { q.handleAction(ActLeft) },  // back to glossary
				"rankings tab":  func() { q.handleAction(ActTab1) },
			})
	}
}

func TestComingSoonLayoutStable(t *testing.T) {
	for _, bp := range breakpoints {
		c := newComingSoon(ScreenTable)
		c.detail = tableConfigSummary(TableConfig{
			SmallBlind: 5, BigBlind: 10, Stack: 1000,
			Lineup: ClassroomLineup(), CoachMode: CoachFull, Speed: SpeedLearn,
		})
		sized(t, c, bp.w, bp.h)

		// A placeholder has no state to mutate; what matters is that its
		// frame fills the terminal exactly (checked by sized above) and the
		// title anchor exists.
		if posOf(c.View(), "Table").row < 0 {
			t.Errorf("%dx%d: placeholder title missing", bp.w, bp.h)
		}
	}
}

// TestTableLayoutStable is the load-bearing table invariant (§8.1): every
// anchor holds its exact position and the view height never changes across
// every state mutation, at all three breakpoints. Game state was reached
// through the engine (a flop checked to the hero); the mutations below are
// presentation-only fields — the sanctioned exception, since this test is
// about rendering, not rules.
func TestTableLayoutStable(t *testing.T) {
	anchorsFor := func(w, h int) []string {
		if layoutFor(w, h) == LayoutCompact {
			return []string{"#1", "YOU", "Board", "esc menu"}
		}
		return []string{"Hand #1", "YOU", "Nia", "? help", "esc menu"}
	}
	longLabel := strings.Repeat("raises to 999,999 ", 4)

	for _, bp := range breakpoints {
		m := buildTable(t, scenarioFlopChoosing(), bp.w, bp.h)
		sized(t, m, bp.w, bp.h)

		assertAnchorsStable(t, m.View, anchorsFor(bp.w, bp.h),
			map[string]func(){
				"villain action absurdly long": func() { m.lastAction[2] = longLabel },
				"villain action cleared":       func() { m.lastAction[2] = "" },
				"board hidden":                 func() { m.boardShown = 0 },
				"board full flop":              func() { m.boardShown = 3 },
				"hero grade line long":         func() { m.heroLine = longLabel },
				"hero grade withheld":          func() { m.heroGraded = false },
				"hero grade shown":             func() { m.heroGraded = true },
				"grade cleared":                func() { m.lastGrade = nil },
				"coach off":                    func() { m.coachMode = CoachOff },
				"coach mistakes":               func() { m.coachMode = CoachMistakes },
				"coach full":                   func() { m.coachMode = CoachFull },
				"bar waiting":                  func() { m.bar.Wait() },
				"bar choosing again":           func() { m.armBar() },
				"bar sizing":                   func() { m.bar.OpenSizing(engine.ActionBet) },
				"award text set":               func() { m.awardText = "MAIN 900 to Sam (two pair, nines and fives)" },
				"award text cleared":           func() { m.awardText = "" },
				"status very long":             func() { m.status = longLabel + longLabel },
				"hand done footer":             func() { m.handDone = true },
				"hand live footer":             func() { m.handDone = false },
			})
	}
}

// TestTableSidePotLineStable: the pot region switching from a single POT to
// MAIN + SIDE layers must not move anything else (the pot text itself is
// centered and changes, so it is deliberately not an anchor).
func TestTableSidePotLineStable(t *testing.T) {
	m := buildTable(t, scenarioSidePot(), 80, 24)
	sized(t, m, 80, 24)
	assertAnchorsStable(t, m.View,
		[]string{"Hand #1", "YOU", "Nia", "esc menu"},
		map[string]func(){
			"single pot":  func() { m.awardText = "POT 1,060" },
			"multi again": func() { m.awardText = "" },
		})
}

// TestQuickReferencePanelsFitCompact guards the content itself: every panel
// must fit the 60-column compact width without overflowing the box — an
// overflowing line would wrap and break every anchor below it.
func TestQuickReferencePanelsFitCompact(t *testing.T) {
	q := NewQuickReference()
	q.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	for tab := refTab(0); tab < tabCount; tab++ {
		q.active = tab
		view := q.View()
		if got := lipgloss.Height(view); got != 20 {
			t.Errorf("tab %v: view height %d at 60x20, want 20", tab, got)
		}
		for i, line := range strings.Split(view, "\n") {
			if w := lipgloss.Width(line); w > 60 {
				t.Errorf("tab %v: line %d is %d cells wide, terminal is 60", tab, i, w)
			}
		}
	}
}
