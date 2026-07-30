package app

import (
	"strconv"
	"strings"
	"time"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/review"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/components"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// HandReview is the post-hand review screen (docs/design-learning.md §7) —
// where the app's hardest lesson lands: a good decision and a good outcome
// are different things.
//
// The screen renders two strictly separated layers per decision:
//
//	Then: the coach's FROZEN grade and note, verbatim from the annotations —
//	      what the hero knew, judged when the hero knew it.
//	Now:  the hindsight layer — every hole card revealed, the true equity —
//	      dimmed and tagged "hindsight", spatially apart, and never allowed
//	      to rewrite the line above it.
//
// The hero steps decision by decision with left/right; one step past the
// last decision is the result frame, deliberately at the end and deliberately
// scored on its own line — results don't grade decisions.
type HandReview struct {
	model    *review.ReplayModel
	returnTo Screen

	// cursor selects the frame: 0..len(Decisions)-1 are hero decisions,
	// len(Decisions) is the result frame.
	cursor int

	// Render lookups derived once from the record.
	names     map[engine.Seat]string
	positions map[engine.Seat]engine.Position
	winners   engine.CardSet // the winning five, for the result frame's gold borders

	help          helpOverlay
	width, height int
}

// NewHandReview builds the review screen over a replayed hand. A nil model
// renders the honest empty state ("no completed hand yet") — the screen must
// route and resize even when there is nothing to review, so the keybind and
// navigation plumbing is always exercised.
func NewHandReview(model *review.ReplayModel, returnTo Screen) *HandReview {
	r := &HandReview{model: model, returnTo: returnTo}
	if model != nil {
		r.names = make(map[engine.Seat]string, len(model.Record.Seats))
		for _, si := range model.Record.Seats {
			r.names[si.Seat] = si.Name
		}
		r.positions = engine.Positions(model.DealtIn, model.Button)
		r.winners = engine.NewCardSet(model.Outcome.WinningFive...)
	}
	return r
}

// Init implements tea.Model.
func (r *HandReview) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (r *HandReview) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width, r.height = msg.Width, msg.Height
	case tea.KeyMsg:
		cmd, _ := r.help.dispatch(r.keymap(), msg, r.handleAction)
		return r, cmd
	}
	return r, nil
}

func (r *HandReview) keymap() KeyMap { return handReviewKeys }

func (r *HandReview) handleAction(a KeyAction) (tea.Cmd, bool) {
	switch a {
	case ActLeft:
		if r.cursor > 0 {
			r.cursor--
		}
		return nil, true
	case ActRight:
		if r.model != nil && r.cursor < r.resultStep() {
			r.cursor++
		}
		return nil, true
	case ActBack:
		return Navigate(r.returnTo), true
	}
	return nil, false
}

// resultStep is the cursor value of the result frame — one past the last
// decision.
func (r *HandReview) resultStep() int {
	if r.model == nil {
		return 0
	}
	return len(r.model.Decisions)
}

// decision returns the selected decision frame, or nil on the result frame.
func (r *HandReview) decision() *review.DecisionFrame {
	if r.model == nil || r.cursor >= len(r.model.Decisions) {
		return nil
	}
	return &r.model.Decisions[r.cursor]
}

// --- Rendering ----------------------------------------------------------------
//
// Same layout discipline as the table: every region has a fixed row budget
// and renders blank when empty, so stepping between frames never moves an
// anchor. Full layout is exactly 24 rows, compact exactly 20; extra terminal
// height pads above the status/footer pair.

// View implements tea.Model.
func (r *HandReview) View() string {
	w, h := fallbackSize(r.width, r.height)
	if r.help.open {
		return renderHelp("Hand Review", r.keymap(), w, h)
	}
	if r.model == nil {
		return r.viewEmpty(w, h)
	}
	var rows []string
	if layoutFor(w, h) == LayoutCompact {
		rows = r.compactRows(w)
	} else {
		rows = r.fullRows(w)
	}
	rows = padRowsAboveFooter(rows, h, strings.Repeat(" ", w))
	return cropHeight(strings.Join(rows, "\n"), h)
}

// viewEmpty is the no-hand state: honest, framed, and still fully navigable.
func (r *HandReview) viewEmpty(w, h int) string {
	th := theme.Current
	body := th.Subtitle.Render("No completed hand to review yet.") + "\n\n" +
		th.Body.Render("Finish a hand at the table, then press v between hands.")
	// The shared chrome, not a bordered box: the rounded border means
	// "overlay" everywhere else in the app, and this is a screen.
	return renderShell(w, h, shell{
		Title:  "Hand Review",
		Status: "nothing to replay yet",
		Footer: "esc back",
	}, body)
}

// fullRows builds the 24 rows of the >=80x24 layout.
func (r *HandReview) fullRows(w int) []string {
	blank := strings.Repeat(" ", w)
	rule := theme.Current.Rule.Render(strings.Repeat(theme.G.RuleH, w))

	rows := make([]string, 0, 24)
	rows = append(rows, r.headerRow(w))     // 1
	rows = append(rows, rule)               // 2
	rows = append(rows, r.seatRows(w)...)   // 3-8
	rows = append(rows, rule)               // 9
	rows = append(rows, r.boardBand(w)...)  // 10-12
	rows = append(rows, r.panelRows(w)...)  // 13-17
	rows = append(rows, rule)               // 18
	rows = append(rows, r.ledgerRows(w)...) // 19-20
	rows = append(rows, blank, blank)       // 21-22
	rows = append(rows, r.statusRow(w))     // 23
	rows = append(rows, r.footerRow(w))     // 24
	return rows
}

// compactRows builds the 20 rows of the >=60x20 ledger layout: inline board
// instead of mini cards, no spare blanks — every number stays on screen.
func (r *HandReview) compactRows(w int) []string {
	rule := theme.Current.Rule.Render(strings.Repeat(theme.G.RuleH, w))

	rows := make([]string, 0, 20)
	rows = append(rows, r.headerRow(w))     // 1
	rows = append(rows, rule)               // 2
	rows = append(rows, r.seatRows(w)...)   // 3-8
	rows = append(rows, rule)               // 9
	rows = append(rows, r.boardLine(w))     // 10
	rows = append(rows, r.panelRows(w)...)  // 11-15
	rows = append(rows, rule)               // 16
	rows = append(rows, r.ledgerRows(w)...) // 17-18
	rows = append(rows, r.statusRow(w))     // 19
	rows = append(rows, r.footerRow(w))     // 20
	return rows
}

// headerRow names the screen, the hand, and the layer disclosure: everything
// below the seats line is hindsight, and the header says so at all times.
func (r *HandReview) headerRow(w int) string {
	th := theme.Current
	dot := " " + theme.G.Dot + " "
	m := r.model
	left := " HAND REVIEW" + dot + "blinds " +
		m.Record.Blinds.Small.String() + "/" + m.Record.Blinds.Big.String() +
		dot + r.frameLabel()
	right := "hindsight: cards revealed "
	return rowLR(w, th.Header.Render(clip(left, w-len(right)-1)), th.Help.Render(right))
}

// frameLabel names the selected frame: "preflop . decision 1/4" or "result".
func (r *HandReview) frameLabel() string {
	if d := r.decision(); d != nil {
		return d.Street.String() + " " + theme.G.Dot + " decision " +
			strconv.Itoa(r.cursor+1) + "/" + strconv.Itoa(len(r.model.Decisions))
	}
	return "result"
}

// seatRows renders one fixed row per seat, villains first, hero last —
// the table's compact-ledger order, so the eye lands where it always does.
// Every seat's cards are face up: this is the hindsight layer's one global
// reveal, declared in the header. Folded seats keep their cards visible with
// a "folded" action label — "he folded the better hand" is exactly the kind
// of truth the review exists to show.
func (r *HandReview) seatRows(w int) []string {
	order := []engine.Seat{1, 2, 3, 4, 5, 0}
	rows := make([]string, 0, len(order))
	for _, s := range order {
		rows = append(rows, r.seatRow(s, w))
	}
	return rows
}

func (r *HandReview) seatRow(s engine.Seat, w int) string {
	m := r.model
	if !m.DealtIn.Has(s) {
		return strings.Repeat(" ", w)
	}
	stacks, committed, folded := r.frameTable()
	v := components.SeatView{
		Name:      r.names[s],
		Position:  r.positions[s],
		Stack:     stacks[s],
		Bet:       committed[s],
		Hole:      m.Holes[s],
		ShowCards: true,
		Dealer:    s == m.Button,
		Hero:      s == m.HeroSeat,
		ToAct:     r.decision() != nil && s == m.HeroSeat,
		Width:     w,
	}
	switch {
	case folded.Has(s):
		v.LastAction = "folded"
	case r.decision() == nil && r.awardFor(s) > 0:
		v.LastAction = "wins " + r.awardFor(s).String()
	}
	return v.RenderCompact()
}

// frameTable is the table state of the selected frame: at a decision, the
// state the hero saw; at the result, the settled stacks.
func (r *HandReview) frameTable() (stacks, committed [engine.MaxSeats]engine.Chips, folded engine.SeatSet) {
	if d := r.decision(); d != nil {
		return d.Stacks, d.Committed, d.Folded
	}
	o := r.model.Outcome
	return o.Stacks, [engine.MaxSeats]engine.Chips{}, o.Folded
}

// awardFor sums a seat's pot awards, for the result frame's "wins N" labels.
func (r *HandReview) awardFor(s engine.Seat) engine.Chips {
	var sum engine.Chips
	for _, a := range r.model.Outcome.Awards {
		if a.Seat == s {
			sum += a.Amount
		}
	}
	return sum
}

// frameBoard is the community cards visible in the selected frame.
func (r *HandReview) frameBoard() []engine.Card {
	if d := r.decision(); d != nil {
		return d.Board
	}
	return r.model.Outcome.Board
}

// boardBand renders the full layout's board region: five fixed mini-card
// slots plus the pot (or the result's award text) beside them. On the result
// frame the winning five get the gold border — board cards included, because
// the best five of seven is the lesson.
func (r *HandReview) boardBand(w int) []string {
	board := r.frameBoard()
	slots := make([]string, components.BoardSlots)
	for i := range slots {
		switch {
		case i >= len(board):
			slots[i] = placeholderCell()
		case r.decision() == nil && r.winners.Has(board[i]):
			slots[i] = winnerMiniCard(board[i])
		default:
			slots[i] = components.MiniCard(board[i])
		}
	}
	lines := strings.Split(hjoin(slots, " "), "\n")
	out := make([]string, 3)
	out[0] = overlayRow(w, at{1, lines[0]})
	out[1] = overlayRow(w, at{1, lines[1]}, at{boardW + 5, r.potText(w - boardW - 6)})
	out[2] = overlayRow(w, at{1, lines[2]})
	return out
}

// boardLine is the compact layout's one-row board: inline cards, dot pips
// for undealt slots, pot at the right.
func (r *HandReview) boardLine(w int) string {
	th := theme.Current
	board := r.frameBoard()
	var sb strings.Builder
	sb.WriteString(" " + th.Header.Render("Board") + "  ")
	for i := 0; i < components.BoardSlots; i++ {
		if i < len(board) {
			sb.WriteString(components.InlineCardSlot(board[i]) + " ")
		} else {
			sb.WriteString(th.BoardPlaceholder.Render(theme.G.Dot) +
				strings.Repeat(" ", components.InlineCardWidth))
		}
	}
	return rowLR(w, sb.String(), r.potText(w/2)+" ")
}

// potText is the pot at a decision, or the award headline at the result.
func (r *HandReview) potText(budget int) string {
	th := theme.Current
	if d := r.decision(); d != nil {
		return th.PotLine.Render(clip(
			"POT "+d.PotBefore.String()+" ("+r.model.Record.Blinds.BB(d.PotBefore)+")", budget))
	}
	return th.PotLine.Render(clip(r.awardText(), budget))
}

// awardText summarizes the payouts: "MAIN 140 to YOU", side pots appended.
func (r *HandReview) awardText() string {
	o := r.model.Outcome
	if len(o.Awards) == 0 {
		return "no pot awarded"
	}
	dot := " " + theme.G.Dot + " "
	parts := make([]string, 0, len(o.Awards))
	for _, a := range o.Awards {
		parts = append(parts, potLabel(a.Pot)+" "+a.Amount.String()+" to "+r.names[a.Seat])
	}
	return strings.Join(parts, dot)
}

// panelRows is the five-row decision panel — the screen's heart. For a
// decision frame the rows are, per design-learning.md §7:
//
//	YOU CALLED 50        Coach: call        Grade: Good   <- what happened + frozen verdict
//	Then  <frozen coach note, verbatim>                   <- what you knew
//	Now   <revealed hands>                     hindsight  <- what was true (dimmed, tagged)
//	      <true equity vs. the price>                     <- the payoff, on its own row
//	<decision-vs-outcome quadrant line>
//
// The hindsight layer gets two rows (docs/ui-review.md F8) so its genuinely
// new fact — the true equity against the real hands — can never be pushed
// off the end of a long villain list. The frozen line and the hindsight
// rows are separate rows, separate styles, separate sources — that
// separation is enforced upstream (review package) and preserved here.
func (r *HandReview) panelRows(w int) []string {
	if d := r.decision(); d != nil {
		return r.decisionPanel(d, w)
	}
	return r.resultPanel(w)
}

func (r *HandReview) decisionPanel(d *review.DecisionFrame, w int) []string {
	th := theme.Current
	c2, c3 := 2*w/5, 7*w/10

	// Row 1: the action, the frozen recommendation, the frozen grade.
	coach, gradeText, gradeStyle := "Coach: (not recorded)", "Grade: (not recorded)", th.Help
	if d.Frozen != nil {
		coach = "Coach: " + d.Frozen.Recommended
		gradeText, gradeStyle = "Grade: "+d.Frozen.Band, th.GradeBad
		if review.GoodBand(d.Frozen.Band) {
			gradeStyle = th.GradeGood
		}
	}
	row1 := overlayRow(w,
		at{1, th.Header.Render(clip(heroActionText(d), c2-2))},
		at{c2, th.Body.Render(clip(coach, c3-c2-1))},
		at{c3, gradeStyle.Render(clip(gradeText, w-c3-1))},
	)

	// Row 2: the frozen explanation, verbatim. The review may not reword it.
	then := "(no frozen coach note " + theme.G.Dot + " grades attach at decision time)"
	thenStyle := th.Help
	if d.Frozen != nil {
		then = d.Frozen.Body
		thenStyle = th.Body
	}
	row2 := padStyledTo(" "+th.CoachTitle.Render("Then")+"  "+thenStyle.Render(clip(then, w-8)), w)

	// Rows 3-4: hindsight — dimmed, tagged, and never on the frozen line.
	// The revealed hands take the labeled row; the true equity and its
	// price comparison — the one number this layer exists to deliver —
	// get a short row of their own that no villain list can crowd out.
	tag := "hindsight "
	villains, truth := r.hindsightRows(d)
	row3 := rowLR(w,
		" "+th.CoachTitle.Render("Now")+"   "+
			th.Help.Render(clip(villains, w-9-len(tag))),
		th.Help.Render(tag))
	row4 := padStyledTo("       "+th.Help.Render(clip(truth, w-8)), w)

	// Row 5: the decision-vs-outcome quadrant — the lesson line.
	quad := ""
	if d.Frozen != nil {
		quad = review.DecisionOutcomeNote(*d.Frozen, r.model.Summary.HeroNetBB)
	}
	row5 := padStyledTo(" "+th.Body.Render(clip(quad, w-2)), w)

	return []string{row1, row2, row3, row4, row5}
}

// resultPanel is the result frame's five rows: what was paid, what the hero
// netted, which five cards played, and — set apart by a blank row — the
// screen's thesis line.
func (r *HandReview) resultPanel(w int) []string {
	th := theme.Current
	o := r.model.Outcome
	dot := " " + theme.G.Dot + " "

	head := r.awardText()
	if o.Showdown && o.WinnerDesc != "" {
		head += " (" + strings.ToLower(o.WinnerDesc) + ")"
	}
	row1 := overlayRow(w,
		at{1, th.Header.Render("RESULT")},
		at{9, th.PotLine.Render(clip(head, w-10))})

	net := r.model.Summary.HeroNet
	netStyle, verb, amount := th.GradeGood, "won", net
	if net < 0 {
		netStyle, verb, amount = th.GradeBad, "lost", -net
	}
	row2 := padStyledTo(" You "+verb+" "+netStyle.Render(amount.String())+
		th.Body.Render(" ("+r.model.Record.Blinds.BB(amount)+") this hand."), w)

	five := "No showdown: the last bet took it, cards never mattered."
	if len(o.WinningFive) > 0 {
		five = "Winning five: " + cardsText(o.WinningFive) + dot +
			"best five of seven, board cards included"
	}
	row3 := padStyledTo(" "+th.Body.Render(clip(five, w-2)), w)

	row4 := strings.Repeat(" ", w)

	row5 := padStyledTo(" "+th.Help.Render(clip(
		"The result above is variance's column. Your grades are the one you control.", w-2)), w)

	return []string{row1, row2, row3, row4, row5}
}

// hindsightRows builds the "Now" layer's two texts: who actually held what,
// then the hero's true equity with the review layer's price comparison. The
// split keeps the payoff — the true number — on a row short enough to
// render whole at every breakpoint (docs/ui-review.md F8); the equity text
// plus the longest note stays inside the compact layout's budget.
func (r *HandReview) hindsightRows(d *review.DecisionFrame) (villains, truth string) {
	hs := d.Hindsight
	if len(hs.VillainHands) == 0 {
		return "no live opponents left to compare against", ""
	}
	parts := make([]string, 0, len(hs.VillainHands))
	for s := engine.Seat(0); s.Valid(); s++ {
		hole, ok := hs.VillainHands[s]
		if !ok {
			continue
		}
		parts = append(parts, r.names[s]+" held "+cardsText(hole[:]))
	}
	villains = strings.Join(parts, ", ")
	if hs.HasEquity {
		truth = "true equity " + strconv.Itoa(int(hs.TrueEquity*100+0.5)) + "%"
		if hs.Note != "" {
			truth += " " + theme.G.Dot + " " + hs.Note
		}
	}
	return villains, truth
}

// ledgerRows is the two-scoreboard summary, present on every frame. The two
// lines are deliberately parallel and deliberately unaligned in meaning:
// DECISIONS reads the frozen EV ledger, OUTCOME reads the chips — and the
// screen never lets one column grade the other.
func (r *HandReview) ledgerRows(w int) []string {
	th := theme.Current
	dot := " " + theme.G.Dot + " "
	s := r.model.Summary

	decisions := "no frozen grades on this hand" + dot + "grades attach at decision time"
	if s.Graded > 0 {
		decisions = "EV lost: " + bbString(s.EVLossBB)
		if s.KeyDecision >= 0 {
			key := r.model.Decisions[s.KeyDecision]
			decisions += dot + "worst: #" + strconv.Itoa(s.KeyDecision+1) +
				" (" + key.Street.String() + ")"
		}
		decisions += dot + gradeCountsText(s.GradeCounts)
	}
	row1 := padStyledTo(" "+th.CoachTitle.Render("DECISIONS")+"  "+
		th.Body.Render(clip(decisions, w-13)), w)

	outcome := "net " + chipsSigned(s.HeroNet) + " (" + netBBText(r.model) + ")" +
		dot + "results don't grade decisions"
	row2 := padStyledTo(" "+th.CoachTitle.Render("OUTCOME")+"    "+
		th.Body.Render(clip(outcome, w-13)), w)

	return []string{row1, row2}
}

// statusRow tells the player where they are and how to move.
func (r *HandReview) statusRow(w int) string {
	dot := " " + theme.G.Dot + " "
	label := "result"
	if r.decision() != nil {
		label = "decision " + strconv.Itoa(r.cursor+1) + " of " +
			strconv.Itoa(len(r.model.Decisions))
	}
	text := " " + label + dot + "left/right step" + dot + "the last step is the result"
	return padStyledTo(theme.Current.StatusLine.Render(clip(text, w-1)), w)
}

// footerRow mirrors the table's footer grammar.
func (r *HandReview) footerRow(w int) string {
	dot := " " + theme.G.Dot + " "
	return rowLR(w, "", theme.Current.Footer.Render("esc back"+dot+"? help "))
}

// --- Small formatting helpers --------------------------------------------------

// heroActionText names the hero's action the way the design mockup does:
// "YOU CALLED 50", "YOU RAISED TO 300".
func heroActionText(d *review.DecisionFrame) string {
	switch d.Action {
	case engine.ActionFold:
		return "YOU FOLDED"
	case engine.ActionCheck:
		return "YOU CHECKED"
	case engine.ActionCall:
		return "YOU CALLED " + d.Paid.String()
	case engine.ActionBet:
		return "YOU BET " + d.Amount.String()
	case engine.ActionRaise:
		return "YOU RAISED TO " + d.Amount.String()
	}
	return "YOU ?"
}

// cardsText renders cards as plain text through the theme's glyph set
// ("K♠ 10♠", or "Ks 10s" in the ASCII set) — safe inside styled lines
// because it carries no styling of its own.
func cardsText(cards []engine.Card) string {
	parts := make([]string, len(cards))
	for i, c := range cards {
		parts[i] = components.CardFace(c)
	}
	return strings.Join(parts, " ")
}

// chipsSigned renders a chip delta with an explicit sign: "+70", "-40".
func chipsSigned(c engine.Chips) string {
	if c > 0 {
		return "+" + c.String()
	}
	return c.String()
}

// netBBText renders the hero's net in big blinds with an explicit sign,
// rounding the magnitude ("-1bb", "+4.5bb"). engine.Blinds.BB is fed the
// magnitude because its rounding is written for non-negative amounts.
func netBBText(m *review.ReplayModel) string {
	net := m.Summary.HeroNet
	if net < 0 {
		return "-" + m.Record.Blinds.BB(-net)
	}
	return "+" + m.Record.Blinds.BB(net)
}

// bbString renders a big-blind quantity to one decimal: "1.2bb".
func bbString(bb float64) string {
	return strconv.FormatFloat(bb, 'f', 1, 64) + "bb"
}

// gradeCountsText flattens the grade tally: "2 Good, 1 Mistake". Band order
// follows the coach's severity scale so the readout is stable.
func gradeCountsText(counts map[string]int) string {
	order := []string{"Best", "Good", "Inaccuracy", "Mistake", "Blunder"}
	var parts []string
	seen := map[string]bool{}
	for _, band := range order {
		if n := counts[band]; n > 0 {
			parts = append(parts, strconv.Itoa(n)+" "+band)
			seen[band] = true
		}
	}
	// Unknown spellings still render — the review displays what it was
	// handed, it does not gatekeep vocabulary.
	var extra []string
	for band, n := range counts {
		if !seen[band] && n > 0 {
			extra = append(extra, strconv.Itoa(n)+" "+band)
		}
	}
	sortStrings(extra)
	parts = append(parts, extra...)
	return strings.Join(parts, ", ")
}

// sortStrings is a tiny insertion sort so the extra-band readout is
// deterministic without pulling in sort for three strings.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// --- Table-session bridge -------------------------------------------------------

// lastHandArchive assembles the finished hand's {record, annotations} pair
// from the live table session — what App.newScreen replays when the player
// presses v. Only a completed hand is reviewable; mid-hand there is nothing
// frozen to show.
//
// Grades come from the coach's frozen GradedDecisions, recorded as the hero
// acted. They are mapped onto event indices here and never recomputed — the
// review layer can see every hole card, and re-grading with that knowledge
// would destroy the whole point (DESIGN.md §1).
//
// TODO(ulid): record IDs become ULIDs when persistence wiring lands.
func (m *TableScreen) lastHandArchive() (engine.HandRecord, review.HandAnnotations, bool) {
	if m.hand == nil || m.hand.Phase() != engine.PhaseComplete {
		return engine.HandRecord{}, review.HandAnnotations{}, false
	}
	res, ok := m.hand.Result()
	if !ok {
		return engine.HandRecord{}, review.HandAnnotations{}, false
	}

	seats := make([]engine.SeatInfo, 0, m.hand.DealtIn().Count())
	for _, s := range m.hand.DealtIn().Seats() {
		info := engine.SeatInfo{
			Seat: s,
			Name: m.seatName(s),
			// Net is won-minus-invested, so the pre-deal stack is recoverable
			// from the settled one — the engine does not expose it directly.
			StartingStack: m.hand.Stack(s) - res.Net[s],
		}
		if s != heroSeat {
			if i := int(s) - 1; i >= 0 && i < len(m.cfg.Lineup) {
				info.Personality = m.cfg.Lineup[i]
			}
		}
		seats = append(seats, info)
	}

	id := "hand-" + strconv.Itoa(m.handNo)
	rec := review.NewRecord(m.hand, id, time.Now().UTC(), seats)
	ann := review.HandAnnotations{HandID: id, Grades: m.frozenGrades(rec)}
	return rec, ann, true
}

// frozenGrades keys this hand's grades by event index. The hero's EvAction
// events appear in the log in the same order the grades were taken, so the
// two zip together; any mismatch means a grade was missed and the extra
// events are simply left ungraded rather than misaligned onto the wrong
// decision, which would attribute a verdict to an action the hero never
// took.
func (m *TableScreen) frozenGrades(rec engine.HandRecord) map[int]review.FrozenGrade {
	out := make(map[int]review.FrozenGrade, len(m.grades))
	next := 0
	for i, e := range rec.Events {
		if e.Kind != engine.EvAction || e.Seat != heroSeat {
			continue
		}
		if next >= len(m.grades) {
			break
		}
		g := m.grades[next]
		next++
		out[i] = review.FrozenGrade{
			Band:        g.Grade.Label(),
			EVLossBB:    g.EVLossBB,
			Body:        g.Body,
			Recommended: actionPhrase(g.Recommended),
		}
	}
	return out
}

// actionPhrase names an action the way the review's "Coach:" column reads —
// "call", "raise to 90". Sizing is included because "raise" alone is not a
// recommendation a learner can act on.
func actionPhrase(a engine.Action) string {
	if a == nil {
		return ""
	}
	switch v := a.(type) {
	case engine.Bet:
		return "bet " + v.Amount.String()
	case engine.Raise:
		return "raise to " + v.To.String()
	default:
		return a.Type().String()
	}
}
