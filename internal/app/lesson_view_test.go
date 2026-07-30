package app

import (
	"strconv"
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
	"github.com/charmbracelet/lipgloss"
)

// openLessonScreen opens one lesson with its prerequisites pre-completed.
func openLessonScreen(t *testing.T, id string, w, h int) (*Lessons, *profile.Profile) {
	t.Helper()
	lesson, ok := tutorial.Get(id)
	if !ok {
		t.Fatalf("lesson %q not registered", id)
	}
	l, p := newLessonsScreen(t, w, h, lesson.Prerequisites...)
	if !l.Open(id) {
		t.Fatalf("lesson %q should open with prerequisites complete", id)
	}
	return l, p
}

// typedFor is the canonical keyboard input for a drill's correct answer,
// as the digits the drill screen accepts.
func typedFor(d *tutorial.Drill) string {
	switch a := d.Answer.(type) {
	case tutorial.ChoiceAnswer:
		return strconv.Itoa(a.Correct + 1)
	case tutorial.NumericAnswer:
		return strconv.FormatFloat(a.Value, 'f', -1, 64)
	case tutorial.OrderAnswer:
		var b strings.Builder
		for _, idx := range a.Correct {
			b.WriteString(strconv.Itoa(idx + 1))
		}
		return b.String()
	}
	return ""
}

// heroLine is the hero seat's scripted action list — the lesson's own
// recommended line, replay-validated by the content tests.
func heroLine(s *tutorial.ScriptedHand) []engine.Action {
	for _, ss := range s.Seats {
		if ss.Seat == s.Hero {
			return ss.Actions
		}
	}
	return nil
}

// sectionIndex finds the first section matching pred.
func sectionIndex(l *tutorial.Lesson, pred func(*tutorial.Section) bool) int {
	for i := range l.Sections {
		if pred(&l.Sections[i]) {
			return i
		}
	}
	return -1
}

// driveLessonToSplash steps an open lesson to its completion panel:
// sections advanced, drills answered correctly, scripted hands played on
// the lesson's own line. Every step renders, so a panic anywhere in any
// section's renderer fails the test.
func driveLessonToSplash(t *testing.T, l *Lessons) {
	t.Helper()
	v := l.view
	if v == nil {
		t.Fatal("no lesson open")
	}
	for i := 0; !v.splash; i++ {
		if i > 500 {
			t.Fatalf("lesson %q did not complete (stuck at section %d)", v.lesson.ID, v.idx)
		}
		if view := l.View(); lipgloss.Height(view) == 0 {
			t.Fatal("empty render")
		}
		sec := v.section()
		switch {
		case sec.Drill != nil:
			if v.drill.answered {
				if !v.drill.correct {
					t.Fatalf("lesson %q section %d: canonical answer judged wrong", v.lesson.ID, v.idx)
				}
				v.handleAction(ActSelect)
				continue
			}
			v.drill.typed = typedFor(sec.Drill)
			v.handleAction(ActSelect)
		case sec.Script != nil:
			r := v.script
			switch r.phase {
			case scriptIntro:
				v.handleAction(ActSelect)
			case scriptActing:
				line := heroLine(sec.Script)
				if r.decisions >= len(line) {
					t.Fatalf("lesson %q: hero line exhausted at decision %d", v.lesson.ID, r.decisions)
				}
				r.heroTry(line[r.decisions])
			case scriptDone:
				v.handleAction(ActSelect)
			}
		default:
			v.handleAction(ActSelect)
		}
	}
}

// TestEveryLessonOpensStepsAndCompletes drives the whole registry: every
// lesson opens, steps through every section without panicking, completes,
// and persists to the profile. New lessons are covered automatically.
func TestEveryLessonOpensStepsAndCompletes(t *testing.T) {
	for _, lesson := range tutorial.All() {
		lesson := lesson
		t.Run(lesson.ID, func(t *testing.T) {
			l, p := openLessonScreen(t, lesson.ID, 80, 24)
			driveLessonToSplash(t, l)

			if _, done := p.LessonsDone[lesson.ID]; !done {
				t.Fatal("completion must persist to the profile")
			}
			l.handleAction(ActSelect) // leave the splash
			if l.view != nil {
				t.Fatal("splash enter should return to the list")
			}
			if !strings.Contains(squash(l.View()), "of 13 complete") {
				t.Error("list should be back after completion")
			}
		})
	}
}

// TestDrillExplainsRightAndWrong: both a wrong and a right answer surface
// the explanation, the correct answer is accepted, and a wrong one retries
// freely.
func TestDrillExplainsRightAndWrong(t *testing.T) {
	l, _ := openLessonScreen(t, "pot-odds", 80, 24)
	v := l.view
	di := sectionIndex(v.lesson, func(s *tutorial.Section) bool { return s.Drill != nil })
	if di < 0 {
		t.Fatal("pot-odds has drills")
	}
	v.enterSection(di)
	d := v.section().Drill
	explain := squash(d.Explain)
	if len(explain) > 40 {
		explain = explain[:40]
	}

	// Wrong answer: explanation shows, drill stays open.
	v.drill.typed = "99"
	v.handleAction(ActSelect)
	if !v.drill.answered || v.drill.correct {
		t.Fatal("99 should be judged wrong")
	}
	view := squash(l.View())
	if !strings.Contains(view, "Not quite") {
		t.Error("wrong answer needs a verdict")
	}
	if !strings.Contains(view, explain) {
		t.Errorf("wrong answer must still show the explanation; view %q", view)
	}
	if v.prog.Passed(di) {
		t.Error("a wrong answer must not pass the drill")
	}

	// Retry: enter clears, the correct answer is accepted and explained.
	v.handleAction(ActSelect)
	if v.drill.answered {
		t.Fatal("enter after a wrong answer should clear it for a retry")
	}
	v.drill.typed = typedFor(d)
	v.handleAction(ActSelect)
	if !v.drill.correct {
		t.Fatal("the canonical answer must be accepted")
	}
	view = squash(l.View())
	if !strings.Contains(view, "Correct") || !strings.Contains(view, explain) {
		t.Error("a right answer shows the verdict and the explanation")
	}
	if !v.prog.Passed(di) {
		t.Error("a correct answer passes the drill")
	}
}

// TestDrillDigitKeysType: the digit key path (as dispatched actions) builds
// the buffer the checker reads.
func TestDrillDigitKeysType(t *testing.T) {
	l, _ := openLessonScreen(t, "pot-odds", 80, 24)
	v := l.view
	di := sectionIndex(v.lesson, func(s *tutorial.Section) bool { return s.Drill != nil })
	v.enterSection(di)

	l.Update(key("2"))
	l.Update(key("5"))
	if v.drill.typed != "25" {
		t.Fatalf("typed %q after pressing 2 5, want 25", v.drill.typed)
	}
	l.Update(key("backspace"))
	if v.drill.typed != "2" {
		t.Fatalf("backspace left %q, want 2", v.drill.typed)
	}
	l.Update(key("5"))
	l.Update(key("enter"))
	if !v.drill.answered || !v.drill.correct {
		t.Fatal("25 is the first pot-odds drill's answer")
	}
}

// TestScriptedHandStopsAndExpect: the scripted hand runs in the real
// engine, each Stop fires in order with its Teach on screen, Expect rejects
// any other action, free decisions are graded, and the hand completes into
// the debrief.
func TestScriptedHandStopsAndExpect(t *testing.T) {
	l, _ := openLessonScreen(t, "pot-odds", 80, 24)
	v := l.view
	si := sectionIndex(v.lesson, func(s *tutorial.Section) bool { return s.Script != nil })
	if si < 0 {
		t.Fatal("pot-odds has a scripted hand")
	}
	v.enterSection(si)
	r := v.script
	script := v.section().Script

	// Intro first; enter deals.
	if r.phase != scriptIntro {
		t.Fatal("scripted section starts on its intro")
	}
	v.handleAction(ActSelect)
	if r.phase != scriptActing || r.hand == nil {
		t.Fatal("enter should deal the hand into the live engine")
	}

	// Stop 0 is active: Teach on screen, Expect gating armed.
	if r.stop == nil || r.stop.AtDecision != 0 {
		t.Fatal("the first stop should be live at decision 0")
	}
	teach := squash(r.stop.Teach)
	if len(teach) > 40 {
		teach = teach[:40]
	}
	if !strings.Contains(squash(l.View()), teach) {
		t.Error("Stop.Teach must show in the coach box")
	}

	// A legal action that is not the expected one is rejected wholesale:
	// no event, no decision consumed, and the ask spelled out.
	events := len(r.hand.Events())
	r.heroTry(engine.Raise{S: script.Hero, To: 30})
	if len(r.hand.Events()) != events || r.decisions != 0 {
		t.Fatal("Expect must reject any other action")
	}
	if !strings.Contains(r.status, lessonActionText(r.stop.Expect)) {
		t.Errorf("rejection should name the ask; status %q", r.status)
	}

	// Play the lesson's own line to the end, watching the free decision
	// (decision 1, no stop) get graded like the table grades.
	line := heroLine(script)
	sawGrade := false
	for r.phase == scriptActing {
		if r.decisions >= len(line) {
			t.Fatal("hero line exhausted before the hand completed")
		}
		r.heroTry(line[r.decisions])
		if r.lastGrade != nil {
			sawGrade = true
		}
	}
	if !sawGrade {
		t.Error("ungated decisions must be graded with coach.GradeAction")
	}

	// Stops fired in order, exactly once each.
	var want []int
	for _, s := range script.Stops {
		want = append(want, s.AtDecision)
	}
	if len(r.stopsFired) != len(want) {
		t.Fatalf("stops fired %v, want %v", r.stopsFired, want)
	}
	for i := range want {
		if r.stopsFired[i] != want[i] {
			t.Fatalf("stops fired %v, want %v", r.stopsFired, want)
		}
	}

	// Debrief: the hand is complete in the engine and the debrief shows.
	if r.hand.Phase() != engine.PhaseComplete {
		t.Fatal("the scripted hand must complete in the engine")
	}
	view := squash(l.View())
	if !strings.Contains(view, "DEBRIEF") {
		t.Error("debrief box missing")
	}
	if r.resultLine == "" {
		t.Error("the result line should say who won")
	}

	// Continuing marks the section viewed.
	v.handleAction(ActSelect)
	if !v.prog.Viewed(si) {
		t.Error("finishing the hand views the scripted section")
	}
}

// TestScriptSizingConfirmsExpectedBet: an Expect'd bet is reachable through
// the real sizing flow — open sizing, type the amount, confirm — and a
// confirmed wrong size is rejected without consuming the decision.
func TestScriptSizingConfirmsExpectedBet(t *testing.T) {
	l, _ := openLessonScreen(t, "bet-sizing", 80, 24)
	v := l.view
	si := sectionIndex(v.lesson, func(s *tutorial.Section) bool { return s.Script != nil })
	v.enterSection(si)
	script := v.section().Script
	r := v.script
	l.Update(key("enter")) // deal

	// Walk to the first stop that expects a Bet.
	line := heroLine(script)
	var expect engine.Bet
	for r.phase == scriptActing {
		if r.stop != nil {
			if b, ok := r.stop.Expect.(engine.Bet); ok {
				expect = b
				break
			}
		}
		r.heroTry(line[r.decisions])
	}
	if expect.Amount == 0 {
		t.Fatal("bet-sizing scripts a Bet stop")
	}

	// Open sizing via the key path and confirm a wrong size: rejected.
	l.Update(key("b"))
	if r.bar.State() != ActionBarSizing {
		t.Fatal("b should open sizing")
	}
	r.bar.amount, r.bar.typed = expect.Amount+10, ""
	decisions := r.decisions
	l.Update(key("enter"))
	if r.decisions != decisions {
		t.Fatal("a wrong size must not pass an Expect stop")
	}
	if r.bar.State() != ActionBarChoosing {
		t.Fatal("rejection should drop back to choosing")
	}

	// Type the exact amount (leading 0 starts a typed value) and confirm.
	l.Update(key("b"))
	l.Update(key("0"))
	for _, d := range expect.Amount.String() {
		l.Update(key(string(d)))
	}
	l.Update(key("enter"))
	if r.decisions != decisions+1 {
		t.Fatalf("the expected bet of %s should be accepted", expect.Amount)
	}
}

// TestLessonViewLayoutStable: the lesson chrome holds position and the view
// height never changes across section stepping, drill answering and script
// play, at all three breakpoints.
func TestLessonViewLayoutStable(t *testing.T) {
	for _, bp := range breakpoints {
		l, _ := openLessonScreen(t, "pot-odds", bp.w, bp.h)
		v := l.view
		sized(t, l, bp.w, bp.h)
		di := sectionIndex(v.lesson, func(s *tutorial.Section) bool { return s.Drill != nil })

		assertAnchorsStable(t, l.View,
			[]string{"Lesson 7", "section "},
			map[string]func(){
				"next section":     func() { v.handleAction(ActRight) },
				"scroll down":      func() { v.handleAction(ActDown) },
				"scroll up":        func() { v.handleAction(ActUp) },
				"into the drill":   func() { v.enterSection(di) },
				"type digits":      func() { v.handleAction(ActPreset2) },
				"answer wrong":     func() { v.drill.typed = "99"; v.handleAction(ActSelect) },
				"clear for retry":  func() { v.handleAction(ActSelect) },
				"status very long": func() { v.status = strings.Repeat("status ", 30) },
				"back to first":    func() { v.enterSection(0) },
			})
	}
}

// TestScriptViewLayoutStable: the in-hand layout holds its anchors while
// the hero is prompted, rejected, sizes a bet, and acts.
func TestScriptViewLayoutStable(t *testing.T) {
	for _, bp := range breakpoints {
		l, _ := openLessonScreen(t, "pot-odds", bp.w, bp.h)
		v := l.view
		si := sectionIndex(v.lesson, func(s *tutorial.Section) bool { return s.Script != nil })
		v.enterSection(si)
		v.handleAction(ActSelect) // deal
		sized(t, l, bp.w, bp.h)
		r := v.script

		assertAnchorsStable(t, l.View,
			[]string{"Lesson 7", "Board"},
			map[string]func(){
				"rejected action": func() { r.heroTry(engine.Raise{S: 0, To: 30}) },
				"sizing open":     func() { v.handleAction(ActRaise) },
				"sizing nudge":    func() { v.handleAction(ActRight) },
				"sizing cancel":   func() { v.handleAction(ActBack) },
				"hero checks":     func() { r.heroTry(engine.Check{S: 0}) },
			})
	}
}

// TestLessonViewLinesFitCompact: every section of the visual-heavy lessons
// renders inside 60 columns at the compact breakpoint — an overflowing line
// would wrap and break every anchor below it.
func TestLessonViewLinesFitCompact(t *testing.T) {
	for _, id := range []string{"hand-rankings", "preflop-ranges", "pot-odds"} {
		l, _ := openLessonScreen(t, id, 60, 20)
		v := l.view
		for i := range v.lesson.Sections {
			v.enterSection(i)
			if v.script != nil {
				v.handleAction(ActSelect) // deal, so the table form renders too
			}
			view := l.View()
			if got := lipgloss.Height(view); got != 20 {
				t.Errorf("%s section %d: height %d at 60x20, want 20", id, i, got)
			}
			for n, line := range strings.Split(view, "\n") {
				if w := lipgloss.Width(line); w > 60 {
					t.Errorf("%s section %d: line %d is %d cells wide, terminal is 60", id, i, n, w)
				}
			}
		}
	}
}

// lessonStateBuilders builds each lesson-view state fresh, for the legend
// tests: dispatching a documented action must never be a no-op, and states
// mutate, so every probe gets its own screen.
func lessonStateBuilders() map[string]func(t *testing.T) *Lessons {
	drillIdx := func(l *tutorial.Lesson) int {
		return sectionIndex(l, func(s *tutorial.Section) bool { return s.Drill != nil })
	}
	scriptIdx := func(l *tutorial.Lesson) int {
		return sectionIndex(l, func(s *tutorial.Section) bool { return s.Script != nil })
	}
	return map[string]func(t *testing.T) *Lessons{
		"list": func(t *testing.T) *Lessons {
			l, _ := newLessonsScreen(t, 80, 24)
			return l
		},
		"section": func(t *testing.T) *Lessons {
			l, _ := openLessonScreen(t, "hand-rankings", 80, 24)
			return l
		},
		"drill": func(t *testing.T) *Lessons {
			l, _ := openLessonScreen(t, "pot-odds", 80, 24)
			l.view.enterSection(drillIdx(l.view.lesson))
			return l
		},
		"scriptIntro": func(t *testing.T) *Lessons {
			l, _ := openLessonScreen(t, "pot-odds", 80, 24)
			l.view.enterSection(scriptIdx(l.view.lesson))
			return l
		},
		"scriptActing": func(t *testing.T) *Lessons {
			l, _ := openLessonScreen(t, "pot-odds", 80, 24)
			l.view.enterSection(scriptIdx(l.view.lesson))
			l.view.handleAction(ActSelect)
			return l
		},
		"scriptSizing": func(t *testing.T) *Lessons {
			l, _ := openLessonScreen(t, "pot-odds", 80, 24)
			l.view.enterSection(scriptIdx(l.view.lesson))
			l.view.handleAction(ActSelect)
			l.view.handleAction(ActRaise) // BB facing a limp: raise is legal
			return l
		},
		"scriptDone": func(t *testing.T) *Lessons {
			l, _ := openLessonScreen(t, "pot-odds", 80, 24)
			v := l.view
			v.enterSection(scriptIdx(v.lesson))
			v.handleAction(ActSelect)
			line := heroLine(v.section().Script)
			for v.script.phase == scriptActing {
				v.script.heroTry(line[v.script.decisions])
			}
			return l
		},
		"splash": func(t *testing.T) *Lessons {
			l, _ := openLessonScreen(t, "hand-rankings", 80, 24)
			driveLessonToSplash(t, l)
			return l
		},
	}
}

// TestLessonDocumentedActionsHandled: in every lesson state, every action
// the keymap documents is implemented (a fresh screen per probe, because
// lesson actions mutate state).
func TestLessonDocumentedActionsHandled(t *testing.T) {
	for name, build := range lessonStateBuilders() {
		km := build(t).keymap()
		for _, b := range km {
			if b.Action == ActHelp {
				continue
			}
			l := build(t)
			if _, handled := l.handleAction(b.Action); !handled {
				t.Errorf("%s: keymap documents %q (%s) but handleAction ignores it",
					name, b.Label, b.Help)
			}
		}
	}
}

// TestLessonKeymapTracksState: keymap() follows the view's state, so the
// help overlay always describes the keys that are live.
func TestLessonKeymapTracksState(t *testing.T) {
	same := func(a, b KeyMap) bool { return len(a) == len(b) && &a[0] == &b[0] }
	want := map[string]KeyMap{
		"list":         lessonListKeys,
		"section":      lessonSectionKeys,
		"drill":        lessonDrillKeys,
		"scriptIntro":  lessonScriptPauseKeys,
		"scriptActing": lessonScriptActKeys,
		"scriptSizing": lessonScriptSizingKeys,
		"scriptDone":   lessonScriptPauseKeys,
		"splash":       lessonSplashKeys,
	}
	for name, build := range lessonStateBuilders() {
		if !same(build(t).keymap(), want[name]) {
			t.Errorf("%s: keymap does not match its state's map", name)
		}
	}
}

// TestLessonUndocumentedKeysIgnored: keys outside a state's keymap change
// nothing — same view, no command. The probe set is the union of every
// lesson keymap's keys plus strays.
func TestLessonUndocumentedKeysIgnored(t *testing.T) {
	probe := map[string]bool{"z": true, "v": true, "tab": true, "+": true, "-": true, "pgdown": true}
	for _, km := range []KeyMap{lessonListKeys, lessonSectionKeys, lessonDrillKeys,
		lessonScriptPauseKeys, lessonScriptActKeys, lessonScriptSizingKeys, lessonSplashKeys} {
		for _, b := range km {
			for _, k := range b.Keys {
				probe[k] = true
			}
		}
	}
	for name, build := range lessonStateBuilders() {
		l := build(t)
		km := l.keymap()
		for k := range probe {
			if km.Contains(k) {
				continue
			}
			before := l.View()
			_, cmd := l.Update(key(k))
			if cmd != nil {
				t.Errorf("%s: undocumented key %q produced a command", name, k)
			}
			if l.View() != before {
				t.Errorf("%s: undocumented key %q changed the view", name, k)
			}
		}
	}
}

// TestLessonHelpOverlayDocumentsKeymap: in every state, "?" renders every
// binding's keycap and description — the legend and the handlers share the
// keymap vars, and this pins the rendering half.
func TestLessonHelpOverlayDocumentsKeymap(t *testing.T) {
	for name, build := range lessonStateBuilders() {
		l := build(t)
		km := l.keymap()
		l.Update(key("?"))
		view := l.View()
		for _, b := range km {
			if !strings.Contains(view, b.Label) {
				t.Errorf("%s help: missing keycap %q", name, b.Label)
			}
			if !strings.Contains(view, b.Help) {
				t.Errorf("%s help: missing description %q", name, b.Help)
			}
		}
		if !strings.Contains(view, "Press any key to close") {
			t.Errorf("%s help: must say how to close", name)
		}
	}
}

// TestLessonRequestOpen: the Open path App routing uses for a LessonRequest
// payload honours gating exactly like the list's enter key.
func TestLessonRequestOpen(t *testing.T) {
	l, p := newLessonsScreen(t, 80, 24)
	second := tutorial.All()[1]
	if l.Open(second.ID) {
		t.Fatal("Open must refuse a locked lesson")
	}
	for _, pre := range second.Prerequisites {
		p.CompleteLesson(pre)
	}
	if !l.Open(second.ID) {
		t.Fatal("Open should accept an unlocked lesson")
	}
	if l.view == nil || l.view.lesson.ID != second.ID {
		t.Fatal("Open should land inside the requested lesson")
	}
	if l.reg.All()[l.cursor].ID != second.ID {
		t.Error("Open should move the list cursor to the opened lesson")
	}
}

// TestCroppedSectionAlwaysAnnouncesItself is the guard for the worst failure
// this screen can have. Lesson 5's UTG range chart renders 9 of its 13 rows
// at 80x24 and looks complete; a learner would memorize a chart missing its
// bottom four rows and never know. Content that does not fit must say so.
func TestCroppedSectionAlwaysAnnouncesItself(t *testing.T) {
	prof := profile.NewProfile()
	for _, l := range tutorial.All() {
		prof.CompleteLesson(l.ID)
	}

	for _, size := range []struct{ w, h int }{{80, 24}, {104, 30}, {60, 20}} {
		for _, lesson := range tutorial.All() {
			v := newLessonView(lesson, prof)
			for i := range lesson.Sections {
				v.idx, v.scroll = i, 0
				out := v.render(size.w, size.h)
				if v.maxScroll <= 0 {
					continue
				}
				if !strings.Contains(out, "more below") && !strings.Contains(out, "more above") {
					t.Errorf("%dx%d %s section %d crops %d lines with no cue:\n%s",
						size.w, size.h, lesson.ID, i+1, v.maxScroll, out)
				}
				// Scrolled to the bottom, the cue must flip direction rather
				// than keep promising content that is now above.
				v.scroll = v.maxScroll
				if out := v.render(size.w, size.h); !strings.Contains(out, "more above") {
					t.Errorf("%dx%d %s section %d: no cue after scrolling to the end",
						size.w, size.h, lesson.ID, i+1)
				}
			}
		}
	}
}
