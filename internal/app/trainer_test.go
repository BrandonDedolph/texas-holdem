package app

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/trainer"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// testTrainerScreen builds a Trainer with a deterministic session seed.
func testTrainerScreen(t *testing.T, w, h int) *Trainer {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	tr := NewTrainer(profile.NewProfile(), DefaultPrefs())
	tr.seed = func() int64 { return 42 }
	tr.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return tr
}

// answerCorrectly submits the current item's correct answer through the
// screen's own action path.
func answerCorrectly(t *testing.T, tr *Trainer) {
	t.Helper()
	it := tr.currentItem()
	if it == nil {
		t.Fatal("no current item to answer")
	}
	switch a := it.Drill.Answer.(type) {
	case tutorial.ChoiceAnswer:
		tr.handleAction(trainerChoiceActs[a.Correct])
	case tutorial.NumericAnswer:
		tr.input = trainerScore(a.Value)
		tr.handleAction(ActSelect)
	default:
		t.Fatalf("unexpected answer type %T", it.Drill.Answer)
	}
	if tr.state != trainerFeedback {
		t.Fatal("submitting an answer must reach the feedback state")
	}
}

// trainerStateBuilds constructs the screen in each interaction state, so
// the keymap and legend tests can probe every state's vocabulary.
func trainerStateBuilds(t *testing.T, w, h int) map[string]func() *Trainer {
	return map[string]func() *Trainer{
		"menu": func() *Trainer { return testTrainerScreen(t, w, h) },
		"asking-choice": func() *Trainer {
			tr := testTrainerScreen(t, w, h)
			tr.begin(trainer.QuizRankings)
			return tr
		},
		"asking-numeric": func() *Trainer {
			tr := testTrainerScreen(t, w, h)
			tr.begin(trainer.QuizOuts)
			return tr
		},
		"feedback": func() *Trainer {
			tr := testTrainerScreen(t, w, h)
			tr.begin(trainer.QuizRankings)
			answerCorrectly(t, tr)
			return tr
		},
		"summary": func() *Trainer {
			tr := testTrainerScreen(t, w, h)
			tr.begin(trainer.QuizRankings)
			for tr.state != trainerSummary {
				if tr.state == trainerAsking {
					answerCorrectly(t, tr)
				} else {
					tr.handleAction(ActContinue)
				}
			}
			return tr
		},
	}
}

// TestTrainerKeymapKeysAreUnique: within each trainer keymap a key resolves
// to exactly one action (the dispatch premise, mirrored from
// keybind_legend_test.go for the maps this file owns).
func TestTrainerKeymapKeysAreUnique(t *testing.T) {
	maps := map[string]KeyMap{
		"trainerMenuKeys":     trainerMenuKeys,
		"trainerChoiceKeys2":  trainerChoiceKeys[2],
		"trainerChoiceKeys3":  trainerChoiceKeys[3],
		"trainerChoiceKeys4":  trainerChoiceKeys[4],
		"trainerNumericKeys":  trainerNumericKeys,
		"trainerFeedbackKeys": trainerFeedbackKeys,
		"trainerSummaryKeys":  trainerSummaryKeys,
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

// TestTrainerDocumentedActionsHandled: every action a state's keymap
// documents is implemented by handleAction in that state. Fresh screen per
// binding, because performing an action may change state (and keymap).
func TestTrainerDocumentedActionsHandled(t *testing.T) {
	for name, build := range trainerStateBuilds(t, 80, 24) {
		for _, b := range build().keymap() {
			if b.Action == ActHelp {
				continue // consumed by the shared help dispatch
			}
			tr := build()
			if _, handled := tr.handleAction(b.Action); !handled {
				t.Errorf("%s: keymap documents %q (%s) but handleAction ignores it",
					name, b.Label, b.Help)
			}
		}
	}
}

// TestTrainerUndocumentedKeysIgnored: a key outside the state's keymap
// changes nothing — same view, no command.
func TestTrainerUndocumentedKeysIgnored(t *testing.T) {
	probe := map[string]bool{
		"z": true, "x": true, "f": true, "r": true, "b": true, "c": true,
		"a": true, "v": true, "tab": true, "pgdown": true, "f1": true,
	}
	for _, km := range []KeyMap{
		trainerMenuKeys, trainerChoiceKeys[4], trainerNumericKeys,
		trainerFeedbackKeys, trainerSummaryKeys,
	} {
		for _, b := range km {
			for _, k := range b.Keys {
				probe[k] = true
			}
		}
	}

	for name, build := range trainerStateBuilds(t, 80, 24) {
		tr := build()
		km := tr.keymap()
		for k := range probe {
			if km.Contains(k) {
				continue
			}
			before := tr.View()
			_, cmd := tr.Update(key(k))
			if cmd != nil {
				t.Errorf("%s: undocumented key %q produced a command", name, k)
			}
			if after := tr.View(); after != before {
				t.Errorf("%s: undocumented key %q changed the view", name, k)
			}
		}
	}
}

// TestTrainerHelpOverlayDocumentsKeymap: the "?" sheet renders every
// binding of the active state, and any key closes it back to the exact
// previous view.
func TestTrainerHelpOverlayDocumentsKeymap(t *testing.T) {
	for name, build := range trainerStateBuilds(t, 80, 24) {
		tr := build()
		base := tr.View()
		km := tr.keymap()

		tr.Update(key("?"))
		view := tr.View()
		if view == base {
			t.Errorf("%s: \"?\" should open the help overlay", name)
			continue
		}
		for _, b := range km {
			if !strings.Contains(view, b.Label) {
				t.Errorf("%s help: missing keycap %q", name, b.Label)
			}
			if !strings.Contains(view, b.Help) {
				t.Errorf("%s help: missing description %q", name, b.Help)
			}
		}
		tr.Update(key("z"))
		if tr.View() != base {
			t.Errorf("%s: closing help must restore the exact previous view", name)
		}
	}
}

// TestTrainerKeymapTracksState: the keymap follows the interaction state,
// including the per-item choice count.
func TestTrainerKeymapTracksState(t *testing.T) {
	tr := testTrainerScreen(t, 80, 24)
	same := func(a, b KeyMap) bool { return len(a) == len(b) && &a[0] == &b[0] }

	if !same(tr.keymap(), trainerMenuKeys) {
		t.Error("menu state: keymap must be the menu set")
	}
	tr.begin(trainer.QuizRankings)
	n := len(tr.currentItem().Drill.Answer.(tutorial.ChoiceAnswer).Choices)
	if !same(tr.keymap(), trainerChoiceKeys[n]) {
		t.Errorf("choice question: keymap must be the %d-choice set", n)
	}
	answerCorrectly(t, tr)
	if !same(tr.keymap(), trainerFeedbackKeys) {
		t.Error("feedback state: keymap must be the feedback set")
	}
	tr.endQuiz()
	tr.begin(trainer.QuizOuts)
	if !same(tr.keymap(), trainerNumericKeys) {
		t.Error("numeric question: keymap must be the numeric set")
	}
}

// TestTrainerMenuLayoutStable: menu chrome holds position as the cursor
// moves, at every breakpoint.
func TestTrainerMenuLayoutStable(t *testing.T) {
	for _, bp := range breakpoints {
		tr := testTrainerScreen(t, bp.w, bp.h)
		sized(t, tr, bp.w, bp.h)
		assertAnchorsStable(t, tr.View,
			[]string{"Trainer", "Rankings Speed Quiz", "Back"},
			map[string]func(){
				"cursor down":        func() { tr.handleAction(ActDown) },
				"cursor to back":     func() { tr.list.cursor = 4 },
				"cursor wraps":       func() { tr.handleAction(ActDown) },
				"cursor up":          func() { tr.handleAction(ActUp) },
				"first row again":    func() { tr.list.cursor = 0 },
				"long detail (spot)": func() { tr.list.cursor = 3 },
			})
	}
}

// TestTrainerQuizLayoutStable: the quiz chrome — status labels, panel
// anchor — holds its exact position through typing, answering, feedback and
// the next question, at every breakpoint. This is the screen's version of
// the table's load-bearing stability test.
func TestTrainerQuizLayoutStable(t *testing.T) {
	for _, bp := range breakpoints {
		tr := testTrainerScreen(t, bp.w, bp.h)
		tr.begin(trainer.QuizOuts)
		sized(t, tr, bp.w, bp.h)

		assertAnchorsStable(t, tr.View,
			[]string{"Question", "Streak", "Score"},
			map[string]func(){
				"type a digit": func() {
					if tr.state == trainerAsking {
						tr.handleAction(ActPreset1)
					}
				},
				"delete digit": func() {
					if tr.state == trainerAsking {
						tr.handleAction(ActBackspace)
					}
				},
				"submit answer": func() {
					if tr.state == trainerAsking {
						tr.input = "9"
						tr.handleAction(ActSelect)
					}
				},
				"next question": func() {
					if tr.state == trainerFeedback {
						tr.handleAction(ActContinue)
					}
				},
			})
	}
}

// TestTrainerQuizFitsCompact: at the 60x20 floor every state renders at
// exactly the terminal height with no line overflowing the width — an
// overflowing line would wrap and break every anchor below it.
func TestTrainerQuizFitsCompact(t *testing.T) {
	for name, build := range trainerStateBuilds(t, 60, 20) {
		tr := build()
		view := tr.View()
		if got := lipgloss.Height(view); got != 20 {
			t.Errorf("%s: view height %d at 60x20, want 20", name, got)
		}
		for i, line := range strings.Split(view, "\n") {
			if w := lipgloss.Width(line); w > 60 {
				t.Errorf("%s: line %d is %d cells wide, terminal is 60", name, i, w)
			}
		}
	}
}

// TestTrainerFlow: a full session through the screen's own action path
// lands on the summary, and "again" starts a fresh session of the same
// kind.
func TestTrainerFlow(t *testing.T) {
	tr := testTrainerScreen(t, 80, 24)
	tr.begin(trainer.QuizRankings)
	for tr.state != trainerSummary {
		if tr.state == trainerAsking {
			answerCorrectly(t, tr)
		} else {
			tr.handleAction(ActContinue)
		}
	}
	view := stripANSI(tr.View())
	if !strings.Contains(view, "Session Complete") {
		t.Error("summary view missing its title")
	}

	tr.handleAction(ActSelect)
	if tr.state != trainerAsking || tr.session.Answered != 0 {
		t.Error("enter on the summary must start a fresh session")
	}

	tr.handleAction(ActBack)
	if tr.state != trainerMenu || tr.session != nil {
		t.Error("esc must end the quiz and return to the menu")
	}
}

// TestTrainerTimerTicks: the clock advances only while a timed session is
// live — rankings ticks, outs does not, and the menu never does.
func TestTrainerTimerTicks(t *testing.T) {
	tr := testTrainerScreen(t, 80, 24)
	if cmd := tr.begin(trainer.QuizOuts); cmd != nil {
		t.Error("untimed quiz must not start the clock")
	}
	tr.Update(trainerTickMsg{})
	if tr.elapsed != 0 {
		t.Error("untimed quiz counted a tick")
	}
	tr.endQuiz()

	if cmd := tr.begin(trainer.QuizRankings); cmd == nil {
		t.Error("timed quiz must start the clock")
	}
	_, cmd := tr.Update(trainerTickMsg{})
	if tr.elapsed != 1 || cmd == nil {
		t.Errorf("timed tick: elapsed=%d cmd=%v, want 1 and a re-arm", tr.elapsed, cmd)
	}
	tr.endQuiz()
	if _, cmd := tr.Update(trainerTickMsg{}); cmd != nil {
		t.Error("clock must stop once the session ends")
	}
}
