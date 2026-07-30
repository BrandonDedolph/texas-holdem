package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/trainer"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/components"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// trainerState is the screen's interaction state; each has its own keymap
// so the help overlay and the legend always describe exactly the live keys.
type trainerState int

const (
	trainerMenu     trainerState = iota // picking a quiz
	trainerAsking                       // a question is on screen
	trainerFeedback                     // the answer and explanation are shown
	trainerSummary                      // the session is over
)

// Trainer screen keymaps. Defined here rather than keymap.go to keep the
// screen self-contained (menu routing is wired separately); the shared
// binding fragments still come from keymap.go so labels stay identical
// everywhere.
var (
	trainerMenuKeys = KeyMap{
		bindUp, bindDown, bindSelect,
		backBinding("back to menu"),
		bindHelp,
	}

	// Choice questions bind exactly as many digits as there are choices, so
	// a key without a visible choice is ignored rather than "wrong".
	trainerChoiceKeys = map[int]KeyMap{
		2: trainerChoiceKeyMap(2),
		3: trainerChoiceKeyMap(3),
		4: trainerChoiceKeyMap(4),
	}

	// Numeric answers reuse the sizing-digit action family: each digit is
	// its own action, one legend row for all ten.
	trainerNumericKeys = KeyMap{
		Binding{ActPreset1, []string{"1"}, "0-9", "type your answer"},
		Binding{ActPreset2, []string{"2"}, "0-9", "type your answer"},
		Binding{ActPreset3, []string{"3"}, "0-9", "type your answer"},
		Binding{ActPreset4, []string{"4"}, "0-9", "type your answer"},
		Binding{ActPreset5, []string{"5"}, "0-9", "type your answer"},
		Binding{ActDigit0, []string{"0"}, "0-9", "type your answer"},
		Binding{ActDigit6, []string{"6"}, "0-9", "type your answer"},
		Binding{ActDigit7, []string{"7"}, "0-9", "type your answer"},
		Binding{ActDigit8, []string{"8"}, "0-9", "type your answer"},
		Binding{ActDigit9, []string{"9"}, "0-9", "type your answer"},
		Binding{ActBackspace, []string{"backspace"}, "backspace", "delete a digit"},
		Binding{ActSelect, []string{"enter"}, "enter", "submit answer"},
		backBinding("end the quiz"),
		bindHelp,
	}

	trainerFeedbackKeys = KeyMap{
		Binding{ActContinue, []string{" ", "enter"}, "space", "next question"},
		backBinding("end the quiz"),
		bindHelp,
	}

	trainerSummaryKeys = KeyMap{
		Binding{ActSelect, []string{"enter", " "}, "enter", "practice again"},
		backBinding("back to quiz menu"),
		bindHelp,
	}
)

// trainerChoiceActs maps choice position to its action.
var trainerChoiceActs = []KeyAction{ActTab1, ActTab2, ActTab3, ActTab4}

// trainerChoiceKeyMap binds digits 1..n to the first n choice actions. All
// share one legend row ("1-n · pick an answer") via the help renderer's
// label dedupe.
func trainerChoiceKeyMap(n int) KeyMap {
	label := fmt.Sprintf("1-%d", n)
	km := make(KeyMap, 0, n+2)
	for i := 0; i < n; i++ {
		km = append(km, Binding{trainerChoiceActs[i],
			[]string{string(rune('1' + i))}, label, "pick an answer"})
	}
	return append(km, backBinding("end the quiz"), bindHelp)
}

// Trainer is the standalone drill screen (design-learning.md §9): pick one
// of four quiz kinds, answer ten generated questions, read the explanation
// after every answer, and watch the per-skill level climb. All poker truth
// lives in internal/trainer; this model only routes keys and lays out rows.
type Trainer struct {
	prof  *profile.Profile
	prefs *Prefs

	state   trainerState
	list    *formList
	session *trainer.Session
	outcome *trainer.Outcome
	chosen  int    // choice index answered, -1 for none/numeric
	input   string // typed digits for numeric answers
	elapsed int    // whole seconds since the session began (timed kinds)

	// seed feeds each new session; tests override it for determinism.
	seed func() int64

	help   helpOverlay
	width  int
	height int
}

// resumeKind reads the quiz kind the player last started, so the trainer
// opens on what they are actually practising rather than resetting to the
// top. Persisted on the profile, so it survives a restart and not just a
// model rebuild. Returns -1 when there is nothing to resume.
func resumeKind(prof *profile.Profile) int {
	if prof == nil || prof.LastQuiz == "" {
		return -1
	}
	for k := trainer.QuizKind(0); k < trainer.NumQuizKinds; k++ {
		if k.String() == prof.LastQuiz {
			return int(k)
		}
	}
	return -1
}

// rememberKind records the quiz the player just started. Best-effort save,
// the same rationale as everywhere else: a broken disk must not stop a drill.
func rememberKind(prof *profile.Profile, k trainer.QuizKind) {
	if prof == nil {
		return
	}
	prof.LastQuiz = k.String()
	_ = prof.Save()
}

// NewTrainer builds the trainer on its quiz menu, with the cursor on the
// last quiz kind practised (this run) rather than always on the first.
func NewTrainer(prof *profile.Profile, prefs *Prefs) *Trainer {
	t := &Trainer{
		prof:   prof,
		prefs:  prefs,
		chosen: -1,
		seed:   func() int64 { return time.Now().UnixNano() },
	}
	t.refreshMenu()
	if k := resumeKind(prof); k >= 0 {
		t.list.cursor = k
	}
	return t
}

// refreshMenu rebuilds the quiz list so the level shown per kind reflects
// the profile after a session.
func (t *Trainer) refreshMenu() {
	blurbs := [trainer.NumQuizKinds]string{
		"Two hands, one board: who wins?",
		"Count your clean outs exactly",
		"Guess your equity within 5 points",
		"Fold, call, or raise: coach-graded",
	}
	rows := make([]listRow, 0, trainer.NumQuizKinds+1)
	for kind := trainer.QuizKind(0); kind < trainer.NumQuizKinds; kind++ {
		rows = append(rows, listRow{
			Label:  kind.Title(),
			Detail: fmt.Sprintf("%s (level %d/%d)", blurbs[kind], trainer.Level(t.prof, kind), trainer.MaxLevel),
		})
	}
	rows = append(rows, listRow{Label: "Back", Detail: "Return to the main menu"})
	t.installMenu(rows)
}

// installMenu swaps in a rebuilt row set, preserving the cursor.
func (t *Trainer) installMenu(rows []listRow) {
	// Rebuilding must not reset the cursor: after a session ends, the menu
	// comes back with the cursor on the quiz just practised, so "practice the
	// next skill" never requires re-navigation.
	cursor := 0
	if t.list != nil {
		cursor = t.list.cursor
	}
	t.list = newFormList(rows)
	if cursor > 0 && cursor < len(rows) {
		t.list.cursor = cursor
	}
}

// Init implements tea.Model.
func (t *Trainer) Init() tea.Cmd { return nil }

// trainerTickMsg advances the session clock once a second while a timed
// session is live.
type trainerTickMsg struct{}

func trainerTick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return trainerTickMsg{} })
}

// ticking reports whether the clock should keep running.
func (t *Trainer) ticking() bool {
	return t.session != nil && t.session.Timed &&
		(t.state == trainerAsking || t.state == trainerFeedback)
}

// Update implements tea.Model.
func (t *Trainer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		t.width, t.height = msg.Width, msg.Height
	case trainerTickMsg:
		if t.ticking() {
			t.elapsed++
			return t, trainerTick()
		}
	case tea.KeyMsg:
		cmd, _ := t.help.dispatch(t.keymap(), msg, t.handleAction)
		return t, cmd
	}
	return t, nil
}

// keymap returns the bindings live in the current state — the same value
// the help overlay renders, which is the keybind/legend invariant.
func (t *Trainer) keymap() KeyMap {
	switch t.state {
	case trainerAsking:
		if it := t.currentItem(); it != nil {
			if a, ok := it.Drill.Answer.(tutorial.ChoiceAnswer); ok {
				n := len(a.Choices)
				if n < 2 {
					n = 2
				}
				if n > 4 {
					n = 4
				}
				return trainerChoiceKeys[n]
			}
			return trainerNumericKeys
		}
		return trainerFeedbackKeys
	case trainerFeedback:
		return trainerFeedbackKeys
	case trainerSummary:
		return trainerSummaryKeys
	default:
		return trainerMenuKeys
	}
}

func (t *Trainer) handleAction(a KeyAction) (tea.Cmd, bool) {
	switch t.state {
	case trainerAsking:
		return t.askingAction(a)
	case trainerFeedback:
		return t.feedbackAction(a)
	case trainerSummary:
		return t.summaryAction(a)
	default:
		return t.menuAction(a)
	}
}

func (t *Trainer) menuAction(a KeyAction) (tea.Cmd, bool) {
	switch a {
	case ActUp:
		t.list.MoveUp()
		return nil, true
	case ActDown:
		t.list.MoveDown()
		return nil, true
	case ActSelect:
		if cursor := t.list.Cursor(); cursor < int(trainer.NumQuizKinds) {
			return t.begin(trainer.QuizKind(cursor)), true
		}
		return Navigate(ScreenMainMenu), true
	case ActBack:
		return Navigate(ScreenMainMenu), true
	}
	return nil, false
}

// begin starts a fresh session of a kind at the profile's current level.
func (t *Trainer) begin(kind trainer.QuizKind) tea.Cmd {
	rememberKind(t.prof, kind)
	t.session = trainer.NewSessionSeeded(kind, t.prof, t.seed())
	t.state = trainerAsking
	t.outcome = nil
	t.chosen = -1
	t.input = ""
	t.elapsed = 0
	if t.session.Timed {
		return trainerTick()
	}
	return nil
}

func (t *Trainer) askingAction(a KeyAction) (tea.Cmd, bool) {
	it := t.currentItem()
	if it == nil {
		return nil, false
	}
	if _, isChoice := it.Drill.Answer.(tutorial.ChoiceAnswer); isChoice {
		for i, act := range trainerChoiceActs {
			if a == act {
				t.submit(i, fmt.Sprintf("%d", i+1))
				return nil, true
			}
		}
	} else {
		if d, ok := trainerDigit(a); ok {
			if len(t.input) < 3 {
				t.input += string(d)
			}
			return nil, true
		}
		switch a {
		case ActBackspace:
			if t.input != "" {
				t.input = t.input[:len(t.input)-1]
			}
			return nil, true
		case ActSelect:
			if t.input != "" {
				t.submit(-1, t.input)
			}
			return nil, true
		}
	}
	if a == ActBack {
		return t.endQuiz(), true
	}
	return nil, false
}

// submit answers the current item and moves to the feedback state.
func (t *Trainer) submit(chosen int, input string) {
	out := t.session.Answer(input)
	t.outcome = &out
	t.chosen = chosen
	t.state = trainerFeedback
}

func (t *Trainer) feedbackAction(a KeyAction) (tea.Cmd, bool) {
	switch a {
	case ActContinue:
		t.outcome = nil
		t.chosen = -1
		t.input = ""
		if t.session.Done() {
			t.state = trainerSummary
			// Progress was recorded per answer; persisting here makes the
			// session's gains survive a crash between sessions. Best-effort
			// for the same reason Settings' save is: a broken disk must not
			// brick the screen.
			_ = t.prof.Save()
		} else {
			t.state = trainerAsking
		}
		return nil, true
	case ActBack:
		return t.endQuiz(), true
	}
	return nil, false
}

func (t *Trainer) summaryAction(a KeyAction) (tea.Cmd, bool) {
	switch a {
	case ActSelect:
		return t.begin(t.session.Kind), true
	case ActBack:
		return t.endQuiz(), true
	}
	return nil, false
}

// endQuiz abandons or closes the session and returns to the quiz menu.
// Answers already given were recorded as they happened, so quitting early
// forfeits nothing but the remaining questions.
func (t *Trainer) endQuiz() tea.Cmd {
	_ = t.prof.Save()
	t.session = nil
	t.outcome = nil
	t.chosen = -1
	t.input = ""
	t.state = trainerMenu
	t.refreshMenu()
	return nil
}

func (t *Trainer) currentItem() *trainer.Item {
	if t.session == nil {
		return nil
	}
	if t.state == trainerFeedback {
		// During feedback the session cursor has advanced; the item being
		// explained is the one just answered.
		if n := t.session.Answered; n > 0 && n <= len(t.session.Items) {
			return &t.session.Items[n-1]
		}
		return nil
	}
	return t.session.Current()
}

// --- rendering ----------------------------------------------------------------

// Row budgets for the question panel. Every region renders at exactly its
// budget (padded or cropped) so answering, feedback, and item changes can
// never move an anchor — the table screen's layout discipline applied here.
const (
	trainerVisualRows   = 8 // full/wide: the tutorial visual (board + hole)
	trainerCompactRows  = 2 // compact: inline card lines instead
	trainerPromptRows   = 3
	trainerAnswerRows   = 4
	trainerFeedbackRows = 5
)

// View implements tea.Model.
func (t *Trainer) View() string {
	w, h := fallbackSize(t.width, t.height)
	if t.help.open {
		return renderHelp("Trainer", t.keymap(), w, h)
	}
	switch t.state {
	case trainerMenu:
		return t.viewMenu(w, h)
	case trainerSummary:
		return t.viewSummary(w, h)
	default:
		return t.viewQuiz(w, h)
	}
}

// viewMenu draws the quiz list over the skill ladder: the list picks a
// drill, the ladder underneath shows what the selected drill's levels mean
// and how close the next unlock is — the mechanism, not just "level 1/3"
// (docs/ui-review.md §5.6, F12).
func (t *Trainer) viewMenu(w, h int) string {
	content := "\n" + t.list.Render(formListWidth) + "\n\n" +
		t.renderLadder(formListWidth)
	return renderShell(w, h, shell{
		Title:  "Trainer",
		Status: t.list.Detail(),
		Footer: "up/down move " + theme.G.Dot + " enter start " +
			theme.G.Dot + " esc back",
	}, content)
}

// viewQuiz draws a live question in the shared chrome: the drill and level
// in the header, the fixed-budget question panel top-anchored in the
// content region, and the running score on the status row.
func (t *Trainer) viewQuiz(w, h int) string {
	compact := layoutFor(w, h) == LayoutCompact
	qw := 72
	if compact {
		qw = 56
	}
	return renderShell(w, h, shell{
		Title: "Trainer " + theme.G.Dot + " " + t.session.Kind.Title(),
		HeaderRight: fmt.Sprintf("level %d/%d",
			t.session.Level, trainer.MaxLevel),
		Status: t.statusText(),
		Footer: t.footerText(),
	}, t.renderQuestion(qw, compact))
}

// statusText is the fixed-width progress line: question number, streak and
// score in stable columns, plus the clock on timed quizzes.
func (t *Trainer) statusText() string {
	s := t.session
	q := s.Answered
	if t.state == trainerAsking {
		q++
	}
	if q > len(s.Items) {
		q = len(s.Items)
	}
	text := fmt.Sprintf("Question %2d/%d   Streak %2d   Score %4s",
		q, len(s.Items), s.Streak, trainerScore(s.Score))
	if s.Timed {
		text += fmt.Sprintf("   Time %d:%02d", t.elapsed/60, t.elapsed%60)
	}
	return text
}

func (t *Trainer) footerText() string {
	dot := " " + theme.G.Dot + " "
	switch t.state {
	case trainerFeedback:
		return "space next" + dot + "esc end quiz"
	default:
		if it := t.currentItem(); it != nil {
			if a, ok := it.Drill.Answer.(tutorial.ChoiceAnswer); ok {
				return fmt.Sprintf("1-%d answer", len(a.Choices)) + dot + "esc end quiz"
			}
		}
		return "type a number, enter submits" + dot + "esc end quiz"
	}
}

// --- Skill ladder --------------------------------------------------------------

// trainerLadderRows is the ladder's fixed row budget: a heading, one row
// per level, and the unlock-progress line. Always occupied (blank on the
// Back row) so cursor movement never reflows the menu.
const trainerLadderRows = trainer.MaxLevel + 2

// renderLadder draws the selected quiz kind's level ladder: every level
// with what it drills, the current level marked, and how far the gate skill
// is from unlocking the next (docs/ui-review.md §5.6). The thresholds
// quoted come from the same profile constants that enforce them.
func (t *Trainer) renderLadder(width int) string {
	cursor := t.list.Cursor()
	if cursor >= int(trainer.NumQuizKinds) {
		return padHeight("", trainerLadderRows)
	}
	th := theme.Current
	kind := trainer.QuizKind(cursor)
	lvl := trainer.Level(t.prof, kind)

	// The two-space indent matches the MenuItem styles' built-in left
	// padding, so the ladder's left edge aligns with the list rows above.
	indent := "  "
	lines := make([]string, 0, trainerLadderRows)
	lines = append(lines, indent+th.CoachTitle.Render(padRow("SKILL LADDER", width)))
	for l := 1; l <= trainer.MaxLevel; l++ {
		row := fmt.Sprintf("%d  %s", l, trainer.LevelBlurb(kind, l))
		switch {
		case l < lvl:
			lines = append(lines, indent+th.GradeGood.Render(theme.G.Good+" ")+
				th.Help.Render(padRow(row, width-2)))
		case l == lvl:
			lines = append(lines, indent+th.MenuItemSelected.UnsetPaddingLeft().Render(
				padRow("> "+row, width)))
		default:
			lines = append(lines, indent+th.Help.Render(padRow(theme.G.Dot+" "+row, width)))
		}
	}
	lines = append(lines, indent+th.Help.Render(padRow(trainerUnlockLine(t.prof, kind), width)))
	return padHeight(strings.Join(lines, "\n"), trainerLadderRows)
}

// trainerUnlockLine states the next unlock and the live progress toward it,
// from the gate skill's real EMA and attempt count. Nothing here is
// invented: the numbers are profile.SkillStat's and the thresholds are the
// ones UnlockReady enforces.
func trainerUnlockLine(prof *profile.Profile, kind trainer.QuizKind) string {
	lvl := trainer.Level(prof, kind)
	stat := trainer.GateStat(prof, kind)
	dot := " " + theme.G.Dot + " "
	gate := fmt.Sprintf("level %d unlocks at %d%% over %d",
		lvl+1, int(profile.UnlockEMA*100), profile.UnlockAttempts)
	now := fmt.Sprintf("now %d%% over %d", int(stat.EMA*100+0.5), stat.Attempts)
	switch {
	case lvl >= trainer.MaxLevel:
		if stat.Attempts == 0 {
			return "top level"
		}
		return "top level" + dot + now
	case stat.Attempts == 0:
		return gate + " answers"
	default:
		return gate + dot + now
	}
}

// renderQuestion draws the item at its fixed row budgets.
func (t *Trainer) renderQuestion(qw int, compact bool) string {
	it := t.currentItem()
	if it == nil {
		return padHeight("", trainerPromptRows+trainerAnswerRows+trainerFeedbackRows)
	}
	var parts []string
	if compact {
		parts = append(parts, padHeight(t.renderInlineCards(it, qw), trainerCompactRows))
	} else {
		visual := ""
		if it.Drill.Visual != nil {
			visual = it.Drill.Visual.Render(qw)
		}
		parts = append(parts, padHeight(visual, trainerVisualRows))
	}
	parts = append(parts, padHeight(t.renderPrompt(it, qw), trainerPromptRows))
	parts = append(parts, padHeight(t.renderAnswers(it, qw), trainerAnswerRows))
	parts = append(parts, padHeight(t.renderFeedback(qw), trainerFeedbackRows))
	return padBlock(strings.Join(parts, "\n"), qw)
}

// padBlock pads every line of a block to exactly width display cells, so
// the centered panel keeps one silhouette no matter what each row holds —
// otherwise a short verdict line would re-center and move its column.
func padBlock(s string, width int) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if pad := width - lipgloss.Width(l); pad > 0 {
			lines[i] = l + strings.Repeat(" ", pad)
		}
	}
	return strings.Join(lines, "\n")
}

// renderInlineCards is the compact substitute for the tutorial visual: the
// board and hole cards on two inline rows.
func (t *Trainer) renderInlineCards(it *trainer.Item, qw int) string {
	th := theme.Current
	v := it.Drill.Visual
	if v == nil || v.Board == nil {
		return ""
	}
	board := th.BoardPlaceholder.Render("(no board)")
	if len(v.Board.Board) > 0 {
		board = trainerInline(v.Board.Board)
	}
	lines := []string{padRow2(th.Body.Render("Board: "), board, qw)}
	if v.Board.ShowHole {
		lines = append(lines, padRow2(th.Body.Render("You:   "), trainerInline(v.Board.Hole[:]), qw))
	}
	return strings.Join(lines, "\n")
}

// padRow2 joins a label and styled cards, padding the plain remainder so
// the row occupies exactly qw display cells.
func padRow2(label, cards string, qw int) string {
	line := label + cards
	if pad := qw - lipgloss.Width(line); pad > 0 {
		line += strings.Repeat(" ", pad)
	}
	return line
}

// trainerInline renders cards through the shared card component so quiz
// cards look exactly like table cards.
func trainerInline(cards []engine.Card) string {
	parts := make([]string, len(cards))
	for i, c := range cards {
		parts[i] = components.InlineCard(c)
	}
	return strings.Join(parts, " ")
}

// renderPrompt wraps each logical prompt line to the panel width; the
// caller crops the result to the prompt's row budget, so an overlong
// question loses its tail rather than reflowing the panel.
func (t *Trainer) renderPrompt(it *trainer.Item, qw int) string {
	th := theme.Current
	lines := strings.Split(it.Drill.Prompt, "\n")
	for i, l := range lines {
		lines[i] = th.Body.Width(qw).Render(l)
	}
	return strings.Join(lines, "\n")
}

// renderAnswers draws the choice list or the numeric input line. In the
// feedback state the chosen row is marked and the verified answer is
// highlighted; prefixes are constant-width so nothing shifts.
func (t *Trainer) renderAnswers(it *trainer.Item, qw int) string {
	th := theme.Current
	switch a := it.Drill.Answer.(type) {
	case tutorial.ChoiceAnswer:
		rows := make([]string, 0, len(a.Choices))
		for i, c := range a.Choices {
			prefix := "  "
			style := th.Body
			if t.state == trainerFeedback && t.outcome != nil {
				switch {
				case i == a.Correct:
					style = th.GradeGood
				case i == t.chosen && !t.outcome.Correct:
					style = th.GradeBad
				case i == t.chosen && t.outcome.Correct:
					style = th.GradeGood
				}
				if i == t.chosen {
					prefix = "> "
				}
			}
			rows = append(rows, style.Render(padRow(fmt.Sprintf("%s%d. %s", prefix, i+1, c), qw)))
		}
		return strings.Join(rows, "\n")
	case tutorial.NumericAnswer:
		cursor := "_"
		if t.state == trainerFeedback {
			cursor = ""
		}
		return th.Body.Render(padRow("Your answer: "+t.input+cursor, qw))
	}
	return ""
}

// renderFeedback draws the verdict and the explanation; blank while a
// question is open so the panel height never changes.
func (t *Trainer) renderFeedback(qw int) string {
	if t.state != trainerFeedback || t.outcome == nil {
		return ""
	}
	th := theme.Current
	out := t.outcome

	verdict := th.GradeBad.Render("Not quite.")
	switch {
	case out.Correct:
		verdict = th.GradeGood.Render("Correct!")
	case out.Points > 0:
		verdict = th.CoachTitle.Render("Close: half credit.")
	}
	if out.LeveledUp {
		verdict += "  " + th.CoachTitle.Render("Level up!")
	}

	body := out.Explain
	if out.Detail != "" {
		body = out.Detail + "\n" + body
	}
	return verdict + "\n" + th.Body.Width(qw).Render(body)
}

// viewSummary closes the session in the shared chrome. The last line is
// the ladder speaking: either the level-up it just earned, or the live
// distance to the next unlock — the summary always says what the session
// moved, not just what it scored (docs/ui-review.md §5.6).
func (t *Trainer) viewSummary(w, h int) string {
	th := theme.Current
	s := t.session

	progress := trainerUnlockLine(t.prof, s.Kind)
	style := th.Help
	if s.LeveledUp {
		progress = fmt.Sprintf("Level up! %s is now level %d.",
			s.Kind.Title(), trainer.Level(t.prof, s.Kind))
		style = th.CoachTitle
	}
	lines := []string{
		"",
		th.Header.Render("Session Complete"),
		"",
		th.Body.Render(fmt.Sprintf("Correct %d/%d   Accuracy %.0f%%   Score %s",
			s.Correct, s.Answered, s.Accuracy()*100, trainerScore(s.Score))),
		th.Body.Render(fmt.Sprintf("Best streak %d", s.BestStreak)),
		"",
		style.Render(progress),
	}
	return renderShell(w, h, shell{
		Title: "Trainer " + theme.G.Dot + " " + s.Kind.Title(),
		HeaderRight: fmt.Sprintf("level %d/%d",
			trainer.Level(t.prof, s.Kind), trainer.MaxLevel),
		Footer: "enter again " + theme.G.Dot + " esc quiz menu",
	}, padBlock(strings.Join(lines, "\n"), formListWidth))
}

// trainerScore renders the score without trailing zeros ("7.5", "9").
func trainerScore(s float64) string {
	if s == float64(int(s)) {
		return fmt.Sprintf("%d", int(s))
	}
	return fmt.Sprintf("%.1f", s)
}

// trainerDigit maps the sizing-digit action family to its digit.
func trainerDigit(a KeyAction) (byte, bool) {
	switch a {
	case ActDigit0:
		return '0', true
	case ActPreset1:
		return '1', true
	case ActPreset2:
		return '2', true
	case ActPreset3:
		return '3', true
	case ActPreset4:
		return '4', true
	case ActPreset5:
		return '5', true
	case ActDigit6:
		return '6', true
	case ActDigit7:
		return '7', true
	case ActDigit8:
		return '8', true
	case ActDigit9:
		return '9', true
	}
	return 0, false
}
