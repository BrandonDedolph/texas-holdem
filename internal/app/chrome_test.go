package app

import (
	"regexp"
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/trainer"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// THE CHROME RULE (render.go) as a test: every content screen shares one
// five-part grammar — header row ending "? help", a full-width rule under
// it, an exact-height body, and a footer row ending "? help" — and no
// content screen draws a rounded border, which is reserved for overlays.
// One test asserts the invariant for the whole family, so a screen cannot
// quietly rejoin the old bordered look.

// chromeScreens builds every content screen in every interaction state
// worth probing, over a mid-journey profile so status slots are populated.
func chromeScreens(t *testing.T) map[string]tea.Model {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	prof := profile.NewProfile()
	prof.CompleteLesson("hand-rankings")
	prof.SessionLog = append(prof.SessionLog, profile.SessionSummary{Hands: 12})
	prof.DrillStats[trainer.QuizOuts.String()] = profile.SkillStat{EMA: 0.7, Attempts: 9, Level: 1}
	prefs := DefaultPrefs()

	lessonOpen := NewLessons(prof, prefs)
	if !lessonOpen.Open("hand-rankings") {
		t.Fatal("hand-rankings should open on this profile")
	}

	quiz := NewTrainer(prof, prefs)
	quiz.seed = func() int64 { return 42 }
	quiz.begin(trainer.QuizOuts)

	feedback := NewTrainer(prof, prefs)
	feedback.seed = func() int64 { return 42 }
	feedback.begin(trainer.QuizOuts)
	feedback.input = "9"
	feedback.handleAction(ActSelect)

	summary := NewTrainer(prof, prefs)
	summary.seed = func() int64 { return 42 }
	summary.begin(trainer.QuizOuts)
	for summary.state != trainerSummary {
		if summary.state == trainerAsking {
			summary.input = "9"
			summary.handleAction(ActSelect)
		} else {
			summary.handleAction(ActContinue)
		}
	}

	return map[string]tea.Model{
		"main-menu":        NewMainMenu(prof),
		"game-setup":       NewGameSetup(prof, prefs),
		"settings":         NewSettings(prof, prefs),
		"quick-reference":  NewQuickReference(),
		"lessons-list":     NewLessons(prof, prefs),
		"lesson-open":      lessonOpen,
		"trainer-menu":     NewTrainer(prof, prefs),
		"trainer-quiz":     quiz,
		"trainer-feedback": feedback,
		"trainer-summary":  summary,
	}
}

// borderRunRE matches a rounded-border top edge of meaningful length: a
// screen or box frame, but not a card — the table's mini card tops out at
// three dashes and the lesson/trainer study card at five, while any real
// frame spans tens of cells.
var borderRunRE = regexp.MustCompile(`╭─{8,}`)

func TestContentScreensShareTheChrome(t *testing.T) {
	for name, m := range chromeScreens(t) {
		for _, bp := range breakpoints {
			m.Update(tea.WindowSizeMsg{Width: bp.w, Height: bp.h})
			view := stripANSI(m.View())
			lines := strings.Split(view, "\n")

			if len(lines) != bp.h {
				t.Errorf("%s %dx%d: %d rows, want exactly %d", name, bp.w, bp.h, len(lines), bp.h)
				continue
			}
			for i, l := range lines {
				if got := lipgloss.Width(l); got > bp.w {
					t.Errorf("%s %dx%d: row %d is %d cells wide", name, bp.w, bp.h, i, got)
				}
			}

			// Row 1: header, title on the left edge, "? help" on the right.
			header := strings.TrimRight(lines[0], " ")
			if !strings.HasSuffix(header, "? help") {
				t.Errorf("%s %dx%d: header %q must end with \"? help\"", name, bp.w, bp.h, header)
			}
			if strings.HasPrefix(lines[0], "  ") {
				t.Errorf("%s %dx%d: title must sit on the left edge, got %q", name, bp.w, bp.h, lines[0])
			}

			// Row 2: a full-width rule.
			if lines[1] != strings.Repeat(theme.G.RuleH, bp.w) {
				t.Errorf("%s %dx%d: row 2 must be a full-width rule, got %q", name, bp.w, bp.h, lines[1])
			}

			// Last row: the key legend, ending "? help".
			footer := strings.TrimRight(lines[len(lines)-1], " ")
			if !strings.HasSuffix(footer, "? help") {
				t.Errorf("%s %dx%d: footer %q must end with \"? help\"", name, bp.w, bp.h, footer)
			}
			if strings.TrimSpace(footer) == "? help" {
				t.Errorf("%s %dx%d: footer carries no legend beyond \"? help\"", name, bp.w, bp.h)
			}

			// No rounded borders: frames belong to overlays only.
			if borderRunRE.MatchString(view) {
				t.Errorf("%s %dx%d: content screen draws a border frame:\n%s", name, bp.w, bp.h, view)
			}
		}
	}
}

// TestHelpOverlayKeepsTheBorder is the other half of the border rule: the
// help sheet is an overlay, and the rounded frame is exactly what marks it
// as one. If this fails together with the test above, someone inverted the
// chrome rule rather than breaking it.
func TestHelpOverlayKeepsTheBorder(t *testing.T) {
	m := NewMainMenu(profile.NewProfile())
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if view := stripANSI(m.View()); !borderRunRE.MatchString(view) {
		t.Errorf("help overlay should render inside a rounded border:\n%s", view)
	}
}
