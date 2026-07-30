package app

import (
	"strconv"
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/coach"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/components"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// The in-lesson view: section stepping, visuals, checked drills, and the
// scripted hand that runs in the real engine (design-learning.md §8.4).
// lessonView is owned by the Lessons screen rather than being a Screen of
// its own — it reports "close" and Lessons handles the transition, so the
// navigation graph stays where the router can see it.

// lessonOutcome is what a handled key did to the view's lifecycle.
type lessonOutcome int

const (
	lessonStay  lessonOutcome = iota // still inside the lesson
	lessonClose                      // back to the curriculum list
)

// lessonView steps through one lesson's sections. Progress is the tutorial
// package's Progress — completion rules (every section viewed, every drill
// passed) live there, not here, so the screen cannot invent its own idea of
// "done".
type lessonView struct {
	lesson *tutorial.Lesson
	prog   *tutorial.Progress
	prof   *profile.Profile

	idx       int
	scroll    int
	maxScroll int // set by the last render; scrolling is display-bounded

	drill  drillState
	script *scriptRun // non-nil while the current section is scripted

	splash bool // the "lesson complete" panel
	status string
}

// drillState is the transient input state of the current drill section.
// The authoritative pass/fail lives in tutorial.Progress; this only holds
// what is being typed and what the last check said.
type drillState struct {
	choice   int    // cursor for ChoiceAnswer
	typed    string // digits typed for numeric/ordering (and optional choice)
	answer   string // the input that was checked, for the result panel
	answered bool   // a check happened; the explanation is showing
	correct  bool
}

// newLessonView opens a lesson at its first section.
func newLessonView(lesson *tutorial.Lesson, prof *profile.Profile) *lessonView {
	v := &lessonView{lesson: lesson, prog: tutorial.NewProgress(lesson), prof: prof}
	v.enterSection(0)
	return v
}

// section is the current section.
func (v *lessonView) section() *tutorial.Section { return &v.lesson.Sections[v.idx] }

// enterSection moves to section i and resets per-section state. Text,
// visual and drill sections count as viewed on entry; a scripted section is
// only viewed when its hand completes — skimming past the hand must not
// count as having played it.
func (v *lessonView) enterSection(i int) {
	if i < 0 || i >= len(v.lesson.Sections) {
		return
	}
	v.idx = i
	v.scroll, v.maxScroll = 0, 0
	v.drill = drillState{}
	v.status = ""
	sec := v.section()
	if sec.Script != nil {
		v.script = newScriptRun(sec.Script, v.prof)
		return
	}
	v.script = nil
	v.prog.View(i)
}

// nextSection advances, or — past the last section — settles the lesson:
// complete lessons are recorded to the profile and persisted immediately,
// incomplete ones jump back to the first unfinished section and say why.
func (v *lessonView) nextSection() {
	if v.idx < len(v.lesson.Sections)-1 {
		v.enterSection(v.idx + 1)
		return
	}
	if v.prog.Complete() {
		v.prog.Record(v.prof)
		// Best-effort persistence, same rationale as Settings: a broken disk
		// must not brick the lesson. TODO(wire-banner).
		_ = v.prof.Save()
		v.splash = true
		return
	}
	for i := range v.lesson.Sections {
		drill := v.lesson.Sections[i].Drill
		if !v.prog.Viewed(i) || (drill != nil && !v.prog.Passed(i)) {
			v.enterSection(i)
			if drill != nil {
				v.status = "answer this drill to finish the lesson"
			} else {
				v.status = "this section still needs a look"
			}
			return
		}
	}
}

// keymap returns the key vocabulary of the current state.
func (v *lessonView) keymap() KeyMap {
	switch {
	case v.splash:
		return lessonSplashKeys
	case v.script != nil:
		if v.script.phase == scriptActing {
			if v.script.bar.State() == ActionBarSizing {
				return lessonScriptSizingKeys
			}
			return lessonScriptActKeys
		}
		return lessonScriptPauseKeys
	case v.section().Drill != nil:
		return lessonDrillKeys
	default:
		return lessonSectionKeys
	}
}

// handleAction performs one key action and reports the lifecycle outcome
// plus whether the action is implemented in this state (the keybind-legend
// contract: every documented action must be handled).
func (v *lessonView) handleAction(a KeyAction) (lessonOutcome, bool) {
	if v.splash {
		switch a {
		case ActSelect, ActBack:
			return lessonClose, true
		}
		return lessonStay, false
	}
	if v.script != nil {
		return v.scriptAction(a)
	}
	if v.section().Drill != nil {
		return v.drillAction(a)
	}
	return v.sectionAction(a)
}

// sectionAction handles text and visual sections: step, scroll, leave.
func (v *lessonView) sectionAction(a KeyAction) (lessonOutcome, bool) {
	switch a {
	case ActLeft:
		v.enterSection(v.idx - 1)
		return lessonStay, true
	case ActRight, ActSelect:
		v.nextSection()
		return lessonStay, true
	case ActUp:
		if v.scroll > 0 {
			v.scroll--
		}
		return lessonStay, true
	case ActDown:
		if v.scroll < v.maxScroll {
			v.scroll++
		}
		return lessonStay, true
	case ActBack:
		return lessonClose, true
	}
	return lessonStay, false
}

// --- Drills -------------------------------------------------------------------

// drillAction handles a drill section. The interaction is deliberately
// forgiving: digits type an answer (and pick a choice), enter checks it,
// and after a check enter either advances (right) or clears for another try
// (wrong). Drills retry freely — this is a curriculum, not an exam.
func (v *lessonView) drillAction(a KeyAction) (lessonOutcome, bool) {
	d := v.section().Drill
	switch a {
	case ActUp, ActDown:
		if ca, ok := d.Answer.(tutorial.ChoiceAnswer); ok && !v.drill.answered {
			n := len(ca.Choices)
			dir := 1
			if a == ActUp {
				dir = -1
			}
			if n > 0 {
				v.drill.choice = (v.drill.choice + dir + n) % n
			}
		}
		return lessonStay, true

	case ActPreset1, ActPreset2, ActPreset3, ActPreset4, ActPreset5,
		ActDigit0, ActDigit6, ActDigit7, ActDigit8, ActDigit9:
		v.typeDrillDigit(drillDigit(a), d)
		return lessonStay, true

	case ActBackspace:
		if !v.drill.answered && v.drill.typed != "" {
			v.drill.typed = v.drill.typed[:len(v.drill.typed)-1]
		}
		return lessonStay, true

	case ActSelect:
		if v.drill.answered {
			if v.drill.correct {
				v.nextSection()
			} else {
				// Clear for another try; the passed flag in Progress is
				// untouched, so nothing is lost by experimenting.
				v.drill = drillState{choice: v.drill.choice}
			}
			return lessonStay, true
		}
		v.submitDrill(d)
		return lessonStay, true

	case ActLeft:
		v.enterSection(v.idx - 1)
		return lessonStay, true
	case ActRight:
		v.nextSection()
		return lessonStay, true
	case ActBack:
		return lessonClose, true
	}
	return lessonStay, false
}

// drillDigit maps a digit key action back to its character. Each digit is
// its own action because dispatch hands screens the action, not the key
// (the same design note as the table's sizing digits).
func drillDigit(a KeyAction) byte {
	switch {
	case a >= ActPreset1 && a <= ActPreset5:
		return byte('1' + a - ActPreset1)
	case a == ActDigit0:
		return '0'
	default:
		return byte('6' + a - ActDigit6)
	}
}

// typeDrillDigit appends a digit to the answer buffer. Typing after a check
// starts a fresh attempt; on a multiple choice the digit also moves the
// cursor so the two input styles agree on what will be submitted.
func (v *lessonView) typeDrillDigit(d byte, drill *tutorial.Drill) {
	if v.drill.answered {
		v.drill = drillState{}
	}
	if len(v.drill.typed) >= 9 {
		return
	}
	v.drill.typed += string(d)
	if ca, ok := drill.Answer.(tutorial.ChoiceAnswer); ok {
		if n := int(d - '0'); n >= 1 && n <= len(ca.Choices) {
			v.drill.choice = n - 1
		}
	}
}

// submitDrill checks the current input against the drill's answer. The
// explanation shows whether the answer was right or wrong — a wrong answer
// teaches more than a right one (tutorial.Drill's contract).
func (v *lessonView) submitDrill(d *tutorial.Drill) {
	in := v.drill.typed
	switch d.Answer.(type) {
	case tutorial.ChoiceAnswer:
		if in == "" {
			in = strconv.Itoa(v.drill.choice + 1)
		}
	case tutorial.OrderAnswer:
		in = spreadDigits(in)
	}
	if in == "" {
		v.status = "type your answer first"
		return
	}
	v.drill.answer = in
	v.drill.correct = v.prog.Answer(v.idx, in)
	v.drill.answered = true
	v.status = ""
}

// spreadDigits turns a typed digit run ("213") into the space-separated
// positions OrderAnswer.Check parses ("2 1 3") — the learner just types the
// sequence, no separators needed.
func spreadDigits(s string) string {
	if s == "" {
		return ""
	}
	parts := make([]string, len(s))
	for i := 0; i < len(s); i++ {
		parts[i] = s[i : i+1]
	}
	return strings.Join(parts, " ")
}

// --- Rendering ----------------------------------------------------------------

// render draws the lesson at exactly h rows: header, rule, a fixed-budget
// content area, status, footer — the same fixed-region discipline as every
// other screen, so stepping sections never moves the chrome.
func (v *lessonView) render(w, h int) string {
	th := theme.Current
	dot := " " + theme.G.Dot + " "

	right := "section " + strconv.Itoa(v.idx+1) + " of " +
		strconv.Itoa(len(v.lesson.Sections)) + dot + "? help "
	left := " Lesson " + strconv.Itoa(v.lesson.Order) + dot + v.lesson.Title
	header := rowLR(w, th.Header.Render(clip(left, w-lipgloss.Width(right)-1)),
		th.Footer.Render(right))
	rule := th.Rule.Render(strings.Repeat(theme.G.RuleH, w))

	budget := h - 4
	var body []string
	switch {
	case v.splash:
		body = v.renderSplash()
	case v.script != nil:
		body = v.renderScript(w, budget)
	case v.section().Drill != nil:
		body = v.renderDrill(w)
	default:
		body = v.renderProse(w, budget)
	}
	for i := range body {
		body[i] = padStyledTo(body[i], w)
	}
	content := padHeight(strings.Join(body, "\n"), budget)

	status := padStyledTo(" "+th.StatusLine.Render(clip(v.statusText(), w-2)), w)
	footer := rowLR(w, "", th.Footer.Render(v.footerText()+" "))
	return header + "\n" + rule + "\n" + content + "\n" + status + "\n" + footer
}

// statusText picks the live status line: the script's while a hand runs,
// the view's own otherwise.
func (v *lessonView) statusText() string {
	if v.script != nil && v.script.status != "" {
		return v.script.status
	}
	return v.status
}

// footerText names the keys of the current state, from the same vocabulary
// the keymap documents.
func (v *lessonView) footerText() string {
	dot := " " + theme.G.Dot + " "
	switch {
	case v.splash:
		return "enter continues"
	case v.script != nil:
		switch {
		case v.script.phase != scriptActing:
			return "enter continue" + dot + "esc lessons" + dot + "? help"
		case v.script.bar.State() == ActionBarSizing:
			return "enter confirm" + dot + "esc cancel" + dot + "? help"
		default:
			return "esc lessons" + dot + "? help"
		}
	case v.section().Drill != nil:
		return "enter check" + dot + "left/right sections" + dot + "esc lessons" + dot + "? help"
	default:
		return "left/right sections" + dot + "esc lessons" + dot + "? help"
	}
}

// renderProse draws a text or visual section with scrolling. maxScroll is
// derived from what actually rendered, so the handler can never scroll the
// content out of sight.
func (v *lessonView) renderProse(w, budget int) []string {
	th := theme.Current
	cw := w - 2
	sec := v.section()

	var lines []string
	if sec.Title != "" {
		lines = append(lines, " "+th.Header.Render(clip(sec.Title, cw)), "")
	}
	for _, l := range wrapBody(sec.Text, cw) {
		lines = append(lines, " "+th.Body.Render(l))
	}
	if sec.Visual != nil {
		lines = append(lines, "")
		lines = append(lines, indentBlock(sec.Visual.Render(cw))...)
	}

	v.maxScroll = len(lines) - budget
	if v.maxScroll < 0 {
		v.maxScroll = 0
	}
	if v.scroll > v.maxScroll {
		v.scroll = v.maxScroll
	}
	return lines[v.scroll:]
}

// renderDrill draws the drill: before a check, the prompt, its visual and
// the answer input; after one, the verdict and the explanation. The visual
// yields its rows to the explanation once answered so the teaching text is
// never pushed off screen — retrying brings the visual straight back.
func (v *lessonView) renderDrill(w int) []string {
	th := theme.Current
	cw := w - 2
	d := v.section().Drill

	var lines []string
	if title := v.section().Title; title != "" {
		lines = append(lines, " "+th.Header.Render(clip(title, cw)), "")
	}
	for _, l := range wrapBody(d.Prompt, cw) {
		lines = append(lines, " "+th.Body.Render(l))
	}
	lines = append(lines, "")

	if v.drill.answered {
		lines = append(lines, " "+th.Body.Render("your answer: "+v.drill.answer))
		verdict := th.GradeBad.Render(theme.G.Bad + " Not quite")
		hint := "enter clears it for another try"
		if v.drill.correct {
			verdict = th.GradeGood.Render(theme.G.Good + " Correct")
			hint = "enter continues"
		}
		lines = append(lines, " "+verdict, "")
		for _, l := range wrapBody(d.Explain, cw) {
			lines = append(lines, " "+th.Body.Render(l))
		}
		return append(lines, "", " "+th.Help.Render(hint))
	}

	if d.Visual != nil {
		lines = append(lines, indentBlock(d.Visual.Render(cw))...)
		lines = append(lines, "")
	}
	return append(lines, v.renderDrillInput(d, cw)...)
}

// renderDrillInput draws the answer widget for the drill's answer type.
func (v *lessonView) renderDrillInput(d *tutorial.Drill, cw int) []string {
	th := theme.Current
	caret := th.Help.Render("_")

	switch ans := d.Answer.(type) {
	case tutorial.ChoiceAnswer:
		lines := make([]string, 0, len(ans.Choices))
		for i, c := range ans.Choices {
			row := "  " + strconv.Itoa(i+1) + ". " + c
			if i == v.drill.choice {
				lines = append(lines, " "+th.MenuItemSelected.Render(clip("> "+row[2:], cw-1)))
			} else {
				lines = append(lines, " "+th.MenuItem.Render(clip(row, cw-1)))
			}
		}
		return lines
	case tutorial.OrderAnswer:
		lines := make([]string, 0, len(ans.Items)+2)
		for i, item := range ans.Items {
			lines = append(lines, " "+th.Body.Render(clip("  "+strconv.Itoa(i+1)+". "+item, cw-1)))
		}
		return append(lines, "",
			" "+th.Body.Render("your order: "+spreadDigits(v.drill.typed))+caret)
	default: // NumericAnswer
		return []string{" " + th.Body.Render("your answer: "+v.drill.typed) + caret}
	}
}

// renderSplash is the completion panel: the lesson is recorded and saved by
// the time this shows, so it only celebrates and points back to the list.
func (v *lessonView) renderSplash() []string {
	th := theme.Current
	return []string{
		"",
		" " + th.GradeGood.Render(theme.G.Good+" Lesson complete"),
		"",
		" " + th.Header.Render(v.lesson.Title),
		" " + th.Body.Render(v.lesson.Goal),
		"",
		" " + th.Help.Render("enter returns to the lesson list"),
	}
}

// wrapBody wraps prose to width, preserving blank lines and keeping
// already-short lines (indented formulas, list items) verbatim so authored
// layout survives.
func wrapBody(s string, width int) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if lipgloss.Width(line) <= width {
			out = append(out, line)
			continue
		}
		out = append(out, wrapPlain(line, width)...)
	}
	return out
}

// indentBlock splits a rendered block into lines with a one-cell margin.
func indentBlock(block string) []string {
	lines := strings.Split(block, "\n")
	for i := range lines {
		lines[i] = " " + lines[i]
	}
	return lines
}

// --- Scripted hands -------------------------------------------------------------

// scriptPhase is where a scripted section's hand is in its life.
type scriptPhase int

const (
	scriptIntro  scriptPhase = iota // the intro text; nothing dealt yet
	scriptActing                    // hand live in the engine
	scriptDone                      // hand complete; debrief showing
)

// scriptRun drives one ScriptedHand through the REAL engine: villains are
// tutorial.ScriptedPlayers, the hero is the keyboard, and every action goes
// through Hand.Apply. There is no parallel simulation — the lesson hand
// obeys exactly the rules a live table does (DESIGN.md §3, script legality).
//
// Villains act synchronously: the pedagogy of a scripted hand is in its
// stops, and each hero decision is a natural pause, so pacing delays would
// only slow the lesson down.
type scriptRun struct {
	script   *tutorial.ScriptedHand
	hand     *engine.Hand
	villains map[engine.Seat]*tutorial.ScriptedPlayer
	names    [engine.MaxSeats]string

	bar   *ActionBar
	coach *coach.Coach

	// Frozen per hero decision, exactly like the table: advice is captured
	// when the turn begins, the grade when the hero commits (DESIGN.md §1).
	advice    *coach.Advice
	lastGrade *coach.GradedDecision

	decisions  int // hero decisions committed so far
	stop       *tutorial.Stop
	stopsFired []int // AtDecision values, in firing order (asserted by tests)

	phase      scriptPhase
	lastAction [engine.MaxSeats]string
	eventsSeen int
	resultLine string
	status     string
}

// newScriptRun prepares a scripted hand in its intro phase. The coach is
// the same TAG-preset strategy the table uses, under the same fixed seed —
// lesson advice must be as reproducible as table advice.
func newScriptRun(s *tutorial.ScriptedHand, prof *profile.Profile) *scriptRun {
	r := &scriptRun{
		script: s,
		bar:    NewActionBar(),
		coach:  coach.New(prof, coachSeed),
		phase:  scriptIntro,
	}
	for _, ss := range s.Seats {
		if ss.Seat.Valid() {
			r.names[ss.Seat] = ss.Name
		}
	}
	return r
}

// deal builds the hand from the script's HandSetup and runs villains up to
// the hero's first decision.
func (r *scriptRun) deal() {
	h, err := r.script.Deal()
	if err != nil {
		// Content is replay-tested, so this is a programmer error surfaced
		// honestly rather than a crash mid-lesson.
		r.status = "cannot deal this hand: " + err.Error()
		return
	}
	r.hand = h
	r.villains = r.script.Players()
	delete(r.villains, r.script.Hero) // the human plays the hero's seat
	r.phase = scriptActing
	r.ingest()
	r.run()
}

// run applies villain actions until the hero must act or the hand ends.
func (r *scriptRun) run() {
	for r.hand.Phase() == engine.PhaseBetting {
		cur := r.hand.CurrentSeat()
		if cur == engine.NoSeat {
			break
		}
		if cur == r.script.Hero {
			r.armHero()
			return
		}
		r.applyVillain(cur)
	}
	r.finish()
}

// applyVillain plays one villain turn from its script. When the hero
// deviates from the scripted line at an ungated decision the villain's
// remaining actions can become illegal (a scripted check into a surprise
// raise) or run out early; the fallback keeps the hand legal and alive —
// check, else call, else fold — instead of wedging the lesson.
func (r *scriptRun) applyVillain(seat engine.Seat) {
	var a engine.Action
	if p := r.villains[seat]; p != nil && p.Remaining() > 0 {
		a = p.Act(r.hand.View(seat))
	}
	if a == nil || a.Seat() != seat || r.hand.Apply(a) != nil {
		legal := r.hand.LegalActions()
		switch {
		case has(legal, engine.ActionCheck):
			_ = r.hand.Apply(engine.Check{S: seat})
		case has(legal, engine.ActionCall):
			_ = r.hand.Apply(engine.Call{S: seat})
		default:
			_ = r.hand.Apply(engine.Fold{S: seat})
		}
	}
	r.ingest()
}

// has reports whether an action type is legal.
func has(opts engine.ActionOptions, t engine.ActionType) bool {
	_, ok := opts.Find(t)
	return ok
}

// armHero opens the hero's turn: find the stop for this decision, freeze
// the coach's advice (grading needs it even when a stop's Teach replaces it
// on screen), and arm the action bar from LegalActions — the bar never
// computes legality itself.
func (r *scriptRun) armHero() {
	r.stop = nil
	for i := range r.script.Stops {
		if r.script.Stops[i].AtDecision == r.decisions {
			r.stop = &r.script.Stops[i]
			r.stopsFired = append(r.stopsFired, r.decisions)
			break
		}
	}

	adv := r.coach.Advise(r.hand.View(r.script.Hero))
	r.advice = &adv

	hero := r.script.Hero
	pot := r.hand.PotTotal()
	toCall := r.hand.ToCall(hero)
	dot := " " + theme.G.Dot + " "
	info := "pot " + pot.String()
	if toCall > 0 {
		info = "to call " + toCall.String() + dot + "pot odds " + equity.PotOddsText(toCall, pot)
	}
	r.bar.Arm(r.hand.LegalActions(), pot, toCall, r.hand.Committed(hero),
		r.script.BigBlind, info)

	r.status = "your turn"
	if r.stop != nil && r.stop.Expect != nil {
		r.status = "your turn" + dot + "the lesson suggests a line " + theme.G.Ellipsis
	}
}

// heroTry commits the hero's action. A stop with Expect set is drill-like:
// only that action advances, anything else is rejected with the ask spelled
// out. Ungated decisions are graded with the same GradeAction the table
// uses — before Apply, so no future card can inform the grade.
func (r *scriptRun) heroTry(a engine.Action) {
	if r.stop != nil && r.stop.Expect != nil && a != r.stop.Expect {
		r.bar.Cancel()
		r.status = "the lesson asks you to " + lessonActionText(r.stop.Expect) +
			" here " + theme.G.Dot + " see the coach box"
		return
	}
	if r.advice != nil && (r.stop == nil || r.stop.Expect == nil) {
		g := r.coach.GradeAction(*r.advice, a)
		r.lastGrade = &g
	}
	if err := r.hand.Apply(a); err != nil {
		// The bar gates on LegalActions, so this should be unreachable; if
		// the engine still objects, say so instead of desyncing.
		r.status = err.Error()
		return
	}
	r.decisions++
	r.stop = nil
	r.advice = nil
	r.bar.Wait()
	r.ingest()
	r.run()
}

// lessonActionText names an expected action for the rejection message.
func lessonActionText(a engine.Action) string {
	switch act := a.(type) {
	case engine.Bet:
		return "bet " + act.Amount.String()
	case engine.Raise:
		return "raise to " + act.To.String()
	default:
		return a.Type().String()
	}
}

// ingest translates engine events appended since the last call into the
// per-seat action labels. Board cards land immediately — a lesson hand
// paces itself by decisions, not animation.
func (r *scriptRun) ingest() {
	evs := r.hand.Events()
	for _, e := range evs[r.eventsSeen:] {
		switch e.Kind {
		case engine.EvAction:
			r.lastAction[e.Seat] = scriptActionLabel(r.hand, e)
		case engine.EvDealBoard:
			// New street: stale labels clear, folded seats keep "folds".
			for s := engine.Seat(0); s.Valid(); s++ {
				if r.hand.Live().Has(s) {
					r.lastAction[s] = ""
				}
			}
		case engine.EvShowdown:
			r.lastAction[e.Seat] = "shows " + strings.ToLower(eval.Rank(e.Rank).String())
		}
	}
	r.eventsSeen = len(evs)
}

// scriptActionLabel renders an action event as a seat label, in the same
// street-total vocabulary the table uses.
func scriptActionLabel(h *engine.Hand, e engine.Event) string {
	if h.AllIn().Has(e.Seat) && e.Action != engine.ActionFold && e.Action != engine.ActionCheck {
		return "all-in " + e.Amount.String()
	}
	switch e.Action {
	case engine.ActionFold:
		return "folds"
	case engine.ActionCheck:
		return "checks"
	case engine.ActionCall:
		return "calls " + e.Amount.String()
	case engine.ActionBet:
		return "bets " + e.Amount.String()
	case engine.ActionRaise:
		return "raises to " + e.Amount.String()
	}
	return ""
}

// finish enters the debrief phase and composes the result line.
func (r *scriptRun) finish() {
	r.phase = scriptDone
	r.bar.Wait()
	r.stop = nil
	r.advice = nil
	r.resultLine = r.buildResult()
	r.status = "hand over " + theme.G.Dot + " enter continues"
}

// buildResult summarizes who won what, main pot first.
func (r *scriptRun) buildResult() string {
	res, ok := r.hand.Result()
	if !ok {
		return ""
	}
	var parts []string
	for _, a := range res.Awards {
		names := make([]string, len(a.Winners))
		for i, s := range a.Winners {
			names[i] = r.seatName(s)
		}
		p := strings.Join(names, " / ") + " wins " + a.Amount.String()
		if a.Rank != 0 {
			p += " (" + strings.ToLower(eval.Rank(a.Rank).String()) + ")"
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, "; ")
}

// seatName is a seat's display name from the script.
func (r *scriptRun) seatName(s engine.Seat) string {
	if s.Valid() && r.names[s] != "" {
		return r.names[s]
	}
	return "seat " + strconv.Itoa(int(s))
}

// scriptAction handles keys while a scripted section is on screen.
func (v *lessonView) scriptAction(a KeyAction) (lessonOutcome, bool) {
	r := v.script
	switch r.phase {
	case scriptIntro:
		switch a {
		case ActSelect:
			r.deal()
			return lessonStay, true
		case ActLeft:
			v.enterSection(v.idx - 1)
			return lessonStay, true
		case ActRight:
			v.nextSection()
			return lessonStay, true
		case ActBack:
			return lessonClose, true
		}

	case scriptDone:
		switch a {
		case ActSelect:
			// Completing the hand is what "viewing" a scripted section means.
			v.prog.View(v.idx)
			v.nextSection()
			return lessonStay, true
		case ActLeft:
			v.enterSection(v.idx - 1)
			return lessonStay, true
		case ActRight:
			v.prog.View(v.idx)
			v.nextSection()
			return lessonStay, true
		case ActBack:
			return lessonClose, true
		}

	case scriptActing:
		if r.bar.State() == ActionBarSizing {
			return v.scriptSizingAction(a)
		}
		hero := r.script.Hero
		switch a {
		case ActFold:
			if r.bar.Accepts(engine.ActionFold) {
				r.heroTry(engine.Fold{S: hero})
			}
			return lessonStay, true
		case ActCheck:
			if r.bar.Accepts(engine.ActionCheck) {
				r.heroTry(engine.Check{S: hero})
			}
			return lessonStay, true
		case ActCall:
			if r.bar.Accepts(engine.ActionCall) {
				r.heroTry(engine.Call{S: hero})
			}
			return lessonStay, true
		case ActBet, ActRaise:
			t := engine.ActionBet
			if a == ActRaise {
				t = engine.ActionRaise
			}
			if r.bar.Accepts(t) {
				if amt, skipped := r.bar.OpenSizing(t); skipped {
					v.scriptSized(t, amt)
				}
			}
			return lessonStay, true
		case ActAllIn:
			r.bar.OpenSizingAllIn()
			return lessonStay, true
		case ActBack:
			// Abandoning a live hand leaves the section unviewed; re-entering
			// the section deals it fresh.
			return lessonClose, true
		}
	}
	return lessonStay, false
}

// scriptSizingAction mirrors the table's sizing vocabulary over the shared
// ActionBar: presets, typed digits, blind-sized nudges, confirm, cancel.
func (v *lessonView) scriptSizingAction(a KeyAction) (lessonOutcome, bool) {
	r := v.script
	switch a {
	case ActPreset1, ActPreset2, ActPreset3, ActPreset4, ActPreset5:
		r.bar.Preset(int(a - ActPreset1))
		return lessonStay, true
	case ActDigit0:
		r.bar.TypeDigit('0')
		return lessonStay, true
	case ActDigit6, ActDigit7, ActDigit8, ActDigit9:
		r.bar.TypeDigit(byte('6' + a - ActDigit6))
		return lessonStay, true
	case ActBackspace:
		r.bar.Backspace()
		return lessonStay, true
	case ActLeft:
		r.bar.Nudge(-1)
		return lessonStay, true
	case ActRight:
		r.bar.Nudge(+1)
		return lessonStay, true
	case ActSelect:
		v.scriptSized(r.bar.SizingType(), r.bar.ConfirmAmount())
		return lessonStay, true
	case ActBack:
		r.bar.Cancel()
		return lessonStay, true
	}
	return lessonStay, false
}

// scriptSized submits a sized action.
func (v *lessonView) scriptSized(t engine.ActionType, amt engine.Chips) {
	hero := v.script.script.Hero
	if t == engine.ActionRaise {
		v.script.heroTry(engine.Raise{S: hero, To: amt})
		return
	}
	v.script.heroTry(engine.Bet{S: hero, Amount: amt})
}

// renderScript draws the scripted section into the content budget: intro
// text before the deal; afterwards a compact table — header, one ledger row
// per seat, the board, the teach/coach box, and the shared action bar. The
// ledger reuses the table's own seat component so a lesson hand looks like
// the game it is teaching.
func (v *lessonView) renderScript(w, budget int) []string {
	th := theme.Current
	cw := w - 2
	r := v.script

	if r.hand == nil {
		lines := []string{" " + th.Header.Render(clip(sectionTitleOr(v.section(), "Scripted hand"), cw)), ""}
		for _, l := range wrapBody(r.script.Intro, cw) {
			lines = append(lines, " "+th.Body.Render(l))
		}
		return append(lines, "", " "+th.Help.Render("enter deals the hand"))
	}

	h := r.hand
	dot := " " + theme.G.Dot + " "

	var seats []engine.Seat
	for s := engine.Seat(0); s.Valid(); s++ {
		if h.DealtIn().Has(s) {
			seats = append(seats, s)
		}
	}

	// Fixed rows: header, seats, board, blank, blank, bar (2). The coach box
	// absorbs the remainder so the layout fills every budget exactly.
	coachRows := budget - (len(seats) + 6)
	if coachRows < 2 {
		coachRows = 2
	}
	if coachRows > 6 {
		coachRows = 6
	}

	header := " " + th.Header.Render(strings.ToUpper(h.Street().String())) +
		th.Body.Render(dot+"pot "+h.PotTotal().String()+dot+
			"blinds "+r.script.SmallBlind.String()+"/"+r.script.BigBlind.String())
	lines := []string{header}
	for _, s := range seats {
		lines = append(lines, " "+r.seatRow(s, cw))
	}
	lines = append(lines, " "+r.boardRow(cw))
	lines = append(lines, "")
	for _, l := range r.coachLines(cw, coachRows) {
		lines = append(lines, " "+l)
	}
	lines = append(lines, "")
	return append(lines, strings.SplitN(r.bar.View(w), "\n", 2)...)
}

// sectionTitleOr falls back when a section has no authored title.
func sectionTitleOr(sec *tutorial.Section, fallback string) string {
	if sec.Title != "" {
		return sec.Title
	}
	return fallback
}

// seatRow maps one seat's engine state onto the shared seat component's
// compact form. Villain holes reveal only at a real showdown — an
// uncontested pot keeps its secrets, exactly like the table.
func (r *scriptRun) seatRow(s engine.Seat, width int) string {
	h := r.hand
	sv := components.SeatView{
		Name:       r.seatName(s),
		Position:   h.Position(s),
		Stack:      h.Stack(s),
		Bet:        h.Committed(s),
		Folded:     !h.Live().Has(s),
		AllIn:      h.AllIn().Has(s),
		Dealer:     h.Button() == s,
		ToAct:      h.Phase() == engine.PhaseBetting && h.CurrentSeat() == s,
		Hero:       s == r.script.Hero,
		LastAction: r.lastAction[s],
		Width:      width,
	}
	show := s == r.script.Hero
	if res, ok := h.Result(); ok && res.Showdown && h.Live().Has(s) {
		show = true
	}
	if show {
		if hole, ok := h.HoleCards(s); ok {
			sv.Hole, sv.ShowCards = hole, true
		}
	}
	return sv.RenderCompact()
}

// boardRow renders the board inline with dot pips for undealt slots, the
// compact table's convention.
func (r *scriptRun) boardRow(width int) string {
	th := theme.Current
	var sb strings.Builder
	sb.WriteString(th.Header.Render("Board") + "  ")
	board := r.hand.Board()
	for i := 0; i < components.BoardSlots; i++ {
		if i < len(board) {
			sb.WriteString(components.InlineCard(board[i]) + " ")
		} else {
			sb.WriteString(th.BoardPlaceholder.Render(theme.G.Dot) + "  ")
		}
	}
	out := sb.String()
	if lipgloss.Width(out) > width {
		return clip(out, width)
	}
	return out
}

// coachLines fills the teach box: the stop's Teach while one is active
// (the lesson's voice replaces normal advice at a stop), the coach's own
// advice at free decisions, the debrief at the end — plus the grade of the
// previous free decision, which lands at the next natural pause.
func (r *scriptRun) coachLines(width, budget int) []string {
	th := theme.Current
	var lines []string
	body := func(s string) {
		for _, l := range wrapPlain(s, width) {
			lines = append(lines, th.Body.Render(l))
		}
	}

	switch {
	case r.phase == scriptDone:
		lines = append(lines, th.CoachTitle.Render("DEBRIEF"))
		if r.resultLine != "" {
			lines = append(lines, th.PotLine.Render(clip(r.resultLine, width)))
		}
		body(r.script.Debrief)
	case r.stop != nil:
		lines = append(lines, th.CoachTitle.Render("LESSON"))
		body(r.stop.Teach)
	case r.advice != nil:
		head := r.advice.Headline
		lines = append(lines, th.CoachTitle.Render("COACH")+"  "+th.Body.Render(clip(head, width-7)))
		body(r.advice.Body)
	}
	if r.lastGrade != nil && r.phase == scriptActing {
		lines = append(lines, gradeText(GradeView{
			Symbol: gradeSymbol(r.lastGrade.Grade),
			Text:   r.lastGrade.Body,
			Good:   r.lastGrade.Grade.GoodOrBetter(),
		}, width))
	}

	if len(lines) > budget {
		lines = lines[:budget]
	}
	for len(lines) < budget {
		lines = append(lines, "")
	}
	return lines
}

// --- Keymaps -------------------------------------------------------------------
//
// The lesson screens' key vocabulary, one map per state, matching keymap.go's
// conventions (vim aliases wherever arrows appear, shared fragments reused).

// Shared lesson bindings.
var (
	bindPrevSection = Binding{ActLeft, []string{"left", "h"}, "left/h", "previous section"}
	bindNextSection = Binding{ActRight, []string{"right", "l"}, "right/l", "next section"}
	bindLessonBack  = Binding{ActBack, []string{"esc", "q"}, "esc/q", "back to lessons"}
)

// drillDigitHelp is the shared legend text for the drill's digit family.
const drillDigitHelp = "type your answer"

// lessonSectionKeys: text and visual sections.
var lessonSectionKeys = KeyMap{
	bindPrevSection, bindNextSection,
	Binding{ActUp, []string{"up", "k"}, "up/k", "scroll up"},
	Binding{ActDown, []string{"down", "j"}, "down/j", "scroll down"},
	Binding{ActSelect, []string{"enter", " "}, "enter/space", "next section"},
	bindLessonBack, bindHelp,
}

// lessonDrillKeys: a drill section, before and after checking. The digits
// share one legend row (the help renderer dedupes identical label+help),
// but each is its own action because dispatch hands screens the action.
var lessonDrillKeys = KeyMap{
	Binding{ActUp, []string{"up", "k"}, "up/k", "previous choice"},
	Binding{ActDown, []string{"down", "j"}, "down/j", "next choice"},
	Binding{ActPreset1, []string{"1"}, "0-9", drillDigitHelp},
	Binding{ActPreset2, []string{"2"}, "0-9", drillDigitHelp},
	Binding{ActPreset3, []string{"3"}, "0-9", drillDigitHelp},
	Binding{ActPreset4, []string{"4"}, "0-9", drillDigitHelp},
	Binding{ActPreset5, []string{"5"}, "0-9", drillDigitHelp},
	Binding{ActDigit0, []string{"0"}, "0-9", drillDigitHelp},
	Binding{ActDigit6, []string{"6"}, "0-9", drillDigitHelp},
	Binding{ActDigit7, []string{"7"}, "0-9", drillDigitHelp},
	Binding{ActDigit8, []string{"8"}, "0-9", drillDigitHelp},
	Binding{ActDigit9, []string{"9"}, "0-9", drillDigitHelp},
	Binding{ActBackspace, []string{"backspace"}, "backspace", "delete a digit"},
	Binding{ActSelect, []string{"enter"}, "enter", "check answer / continue"},
	bindPrevSection, bindNextSection,
	bindLessonBack, bindHelp,
}

// lessonScriptPauseKeys: the scripted hand's intro and debrief.
var lessonScriptPauseKeys = KeyMap{
	Binding{ActSelect, []string{"enter", " "}, "enter/space", "continue"},
	bindPrevSection, bindNextSection,
	bindLessonBack, bindHelp,
}

// lessonScriptActKeys: the hero's turn in a scripted hand. Section stepping
// is deliberately absent — walking away from a live hand is esc, an
// explicit abandonment, not an accidental arrow key.
var lessonScriptActKeys = KeyMap{
	Binding{ActFold, []string{"f"}, "f", "fold"},
	Binding{ActCheck, []string{"x"}, "x", "check (c is call)"},
	Binding{ActCall, []string{"c"}, "c", "call"},
	Binding{ActBet, []string{"b"}, "b", "bet (opens sizing)"},
	Binding{ActRaise, []string{"r"}, "r", "raise (opens sizing)"},
	Binding{ActAllIn, []string{"a"}, "a", "all-in (confirm with enter)"},
	bindLessonBack, bindHelp,
}

// lessonScriptSizingKeys: sizing an in-lesson bet/raise, the table's sizing
// vocabulary verbatim (same ActionBar underneath).
var lessonScriptSizingKeys = KeyMap{
	Binding{ActPreset1, []string{"1"}, "1-5", presetHelp},
	Binding{ActPreset2, []string{"2"}, "1-5", presetHelp},
	Binding{ActPreset3, []string{"3"}, "1-5", presetHelp},
	Binding{ActPreset4, []string{"4"}, "1-5", presetHelp},
	Binding{ActPreset5, []string{"5"}, "1-5", presetHelp},
	Binding{ActDigit0, []string{"0"}, "0/6-9", digitHelp},
	Binding{ActDigit6, []string{"6"}, "0/6-9", digitHelp},
	Binding{ActDigit7, []string{"7"}, "0/6-9", digitHelp},
	Binding{ActDigit8, []string{"8"}, "0/6-9", digitHelp},
	Binding{ActDigit9, []string{"9"}, "0/6-9", digitHelp},
	Binding{ActBackspace, []string{"backspace"}, "backspace", "delete typed digit"},
	Binding{ActLeft, []string{"left", "h"}, "left/h", "nudge down one big blind"},
	Binding{ActRight, []string{"right", "l"}, "right/l", "nudge up one big blind"},
	Binding{ActSelect, []string{"enter"}, "enter", "confirm bet/raise"},
	Binding{ActBack, []string{"esc"}, "esc", "cancel sizing"},
	bindHelp,
}

// lessonSplashKeys: the completion panel.
var lessonSplashKeys = KeyMap{
	Binding{ActSelect, []string{"enter", " "}, "enter/space", "back to lessons"},
	bindLessonBack, bindHelp,
}
