package app

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/trainer"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
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

// TestTrainerMenuResumesLastQuiz: the quiz menu remembers the last quiz
// practised — the cursor survives the end of a session and the rebuild of
// the screen model — instead of resetting to the first row every time
// (docs/ui-review.md F3). The memory lives on the profile, so a rebuilt
// screen must be given the same profile the app holds; a rebuild from a
// fresh profile is a different player and correctly starts at the top.
func TestTrainerMenuResumesLastQuiz(t *testing.T) {

	tr := testTrainerScreen(t, 80, 24)
	if tr.list.Cursor() != 0 {
		t.Fatalf("nothing practised yet: cursor %d, want 0", tr.list.Cursor())
	}
	tr.list.cursor = int(trainer.QuizEquity)
	tr.handleAction(ActSelect)
	if tr.state != trainerAsking {
		t.Fatal("selecting a quiz should start it")
	}
	tr.handleAction(ActBack) // end the quiz, back to the menu
	if got := tr.list.Cursor(); got != int(trainer.QuizEquity) {
		t.Errorf("after a session the cursor reset to %d, want the equity row %d",
			got, int(trainer.QuizEquity))
	}

	// A rebuilt screen model (a later navigation constructs Trainer anew
	// from the app's profile) starts on the same quiz.
	fresh := NewTrainer(tr.prof, DefaultPrefs())
	fresh.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if got := fresh.list.Cursor(); got != int(trainer.QuizEquity) {
		t.Errorf("rebuilt trainer cursor %d, want the last practised quiz %d",
			got, int(trainer.QuizEquity))
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

// TestTrainerRemembersQuizAcrossRestarts: the trainer should open on what
// the player is actually practising. An in-memory-only memory resets every
// launch, which is exactly when it matters.
func TestTrainerRemembersQuizAcrossRestarts(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	prof := profile.NewProfile()

	tr := NewTrainer(prof, DefaultPrefs())
	tr.begin(trainer.QuizOuts)
	if prof.LastQuiz != trainer.QuizOuts.String() {
		t.Fatalf("LastQuiz = %q, want %q", prof.LastQuiz, trainer.QuizOuts.String())
	}

	// A brand new screen from the same persisted profile resumes there.
	again := NewTrainer(prof, DefaultPrefs())
	if got := again.list.cursor; got != int(trainer.QuizOuts) {
		t.Errorf("cursor = %d, want %d (the last quiz practised)", got, int(trainer.QuizOuts))
	}

	// An unknown value (hand-edited profile) must not crash or mis-select.
	prof.LastQuiz = "no-such-quiz"
	if got := NewTrainer(prof, DefaultPrefs()).list.cursor; got != 0 {
		t.Errorf("unknown LastQuiz should fall back to the first quiz, got cursor %d", got)
	}
}

// TestTrainerMenuShowsSkillLadder (docs/ui-review.md §5.6): the menu shows
// the selected kind's ladder — what every level drills, the current level
// marked, and the real gate-stat progress toward the next unlock.
func TestTrainerMenuShowsSkillLadder(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	prof := profile.NewProfile()
	// Gate skill mid-climb at level 2 (0-based Level 1): 72% over 14.
	prof.DrillStats[trainer.QuizOuts.String()] = profile.SkillStat{
		EMA: 0.72, Attempts: 14, Level: 1,
	}
	tr := NewTrainer(prof, DefaultPrefs())
	tr.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	tr.list.cursor = int(trainer.QuizOuts)

	view := squash(tr.View())
	if !strings.Contains(view, "SKILL LADDER") {
		t.Fatalf("menu missing the ladder; view: %q", view)
	}
	for l := 1; l <= trainer.MaxLevel; l++ {
		if blurb := trainer.LevelBlurb(trainer.QuizOuts, l); !strings.Contains(view, blurb) {
			t.Errorf("ladder missing level %d blurb %q", l, blurb)
		}
	}
	// The completed level is checked off and the current one is marked.
	if !strings.Contains(view, theme.G.Good+" 1") {
		t.Errorf("level 1 should be checked off; view: %q", view)
	}
	if !strings.Contains(view, "> 2") {
		t.Errorf("level 2 should carry the current marker; view: %q", view)
	}
	// The unlock line quotes the enforced thresholds and the live stat.
	if !strings.Contains(view, "level 3 unlocks at 80% over 20") {
		t.Errorf("ladder missing the unlock rule; view: %q", view)
	}
	if !strings.Contains(view, "now 72% over 14") {
		t.Errorf("ladder missing the live progress; view: %q", view)
	}

	// On the Back row the ladder yields, but the layout must not reflow.
	before := lipgloss.Height(tr.View())
	tr.list.cursor = len(tr.list.rows) - 1
	after := tr.View()
	if strings.Contains(after, "SKILL LADDER") {
		t.Error("the Back row has no ladder to show")
	}
	if got := lipgloss.Height(after); got != before {
		t.Errorf("hiding the ladder changed the height %d -> %d", before, got)
	}
}

// TestTrainerSummaryShowsUnlockProgress: a finished session that did not
// level up still says how far the next unlock is — the ladder's promise
// carried through to the moment the player is deciding whether to go again.
func TestTrainerSummaryShowsUnlockProgress(t *testing.T) {
	tr := testTrainerScreen(t, 80, 24)
	tr.begin(trainer.QuizOuts)
	for tr.state != trainerSummary {
		if tr.state == trainerAsking {
			// Deliberately wrong: no level-up, so the progress line shows.
			tr.input = "999"
			tr.handleAction(ActSelect)
		} else {
			tr.handleAction(ActContinue)
		}
	}
	view := squash(tr.View())
	if !strings.Contains(view, "unlocks at 80% over 20") {
		t.Errorf("summary should state the next unlock and progress; view: %q", view)
	}
}
