package app

import (
	"strconv"
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/coach"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/components"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// Table layout (docs/design-tui.md §2). Everything in this file is pure
// rendering over the TableScreen's state. The one structural invariant that
// everything else hangs off: EVERY region has a fixed row budget and renders
// blank when empty. Seats keep their three rows when folded, the board keeps
// five slots before the flop, the coach strip is reserved even when the
// coach is off — the table never reflows, so the player's eyes learn fixed
// landmarks.
//
// Layout is chosen per-View() from width/height, never stored, so a live
// resize in either direction just works (§2.7).

// Geometry constants for the full (80x24) layout.
const (
	seatSlotW = components.DefaultSeatWidth // 20-col seat blocks (§2.2)
	boardW    = components.BoardSlots*components.MiniCardWidth +
		(components.BoardSlots - 1) // five mini cards, one-col gaps
	heroCardsW = 2*components.MiniCardWidth + 1 // two mini cards, one gap
)

// View implements tea.Model.
func (m *TableScreen) View() string {
	w, h := fallbackSize(m.width, m.height)
	if m.help.open {
		return renderHelp("Table", m.keymap(), w, h)
	}
	var base string
	switch layoutFor(w, h) {
	case LayoutWide:
		base = m.viewWide(w, h)
	case LayoutCompact:
		base = m.viewCompact(w, h)
	case LayoutTooSmall:
		return renderTooSmall(w, h) // the App root already gates this; belt and braces
	default:
		base = m.viewFull(w, h)
	}
	if m.overlay != nil {
		// A teachable moment or the coach's expanded explanation: a modal
		// box over the frozen table, dismissed by any key (§5.1, §5.4).
		return renderTableOverlay(base, w, h, m.overlay)
	}
	return base
}

// viewFull is the 80x24 target layout: the row budget in §2.1, verbatim.
// Terminals taller than the budget get blank rows above the status/footer
// pair, which stays glued to the bottom of the screen.
func (m *TableScreen) viewFull(w, h int) string {
	coach := renderCoachStrip(m.coachView(), m.coachMode, m.coachNeutral(), w)
	rows := m.fullRows(w, strings.SplitN(coach, "\n", coachStripRows))
	rows = padRowsAboveFooter(rows, h, strings.Repeat(" ", w))
	return cropHeight(strings.Join(rows, "\n"), h)
}

// viewWide (>=104x28) swaps the 2-row coach strip for a bordered side panel
// and reclaims the strip rows for the action ticker (§2.6). The left column
// is the full layout at the remaining width, so every full-layout invariant
// carries over unchanged.
func (m *TableScreen) viewWide(w, h int) string {
	leftW := w - coachPanelWidth - 2
	rows := m.fullRows(leftW, m.tickerRows(leftW))
	rows = padRowsAboveFooter(rows, h, strings.Repeat(" ", leftW))
	panel := strings.Split(renderCoachPanel(m.coachView(), m.coachMode, m.coachNeutral(), h), "\n")

	out := make([]string, len(rows))
	for i := range rows {
		right := ""
		if i < len(panel) {
			right = panel[i]
		}
		out[i] = rows[i] + "  " + right
	}
	return cropHeight(strings.Join(out, "\n"), h)
}

// tickerRows renders the last few actions where the coach strip would be —
// the wide layout shows the coach in its side panel instead. The ticker's
// height matches the strip's so the two layouts share one row budget.
func (m *TableScreen) tickerRows(w int) []string {
	th := theme.Current
	rows := make([]string, coachStripRows)
	n := len(m.recent)
	for i := 0; i < coachStripRows && i < n; i++ {
		rows[coachStripRows-1-i] = " " + th.Help.Render(clip(m.recent[n-1-i], w-1))
	}
	return rows
}

// fullRows builds the 24 rows of the full layout. coach supplies rows 17-19
// (the strip, or the wide layout's ticker); every row is exactly w cells.
// The strip's third row came out of the old blank row between the action
// bar and the status line — two blank rows sat there while the coach's
// reasoning clipped mid-thought (ui-review F1). The budget is still a
// constant 24: the strip grew to a fixed height, not a content-dependent
// one.
func (m *TableScreen) fullRows(w int, coach []string) []string {
	blank := strings.Repeat(" ", w)
	rule := theme.Current.Rule.Render(strings.Repeat(theme.G.RuleH, w))

	rows := make([]string, 0, 24)
	rows = append(rows, m.headerRow(w))      // 1
	rows = append(rows, blank)               // 2
	rows = append(rows, m.topSeatRows(w)...) // 3-5
	rows = append(rows, blank)               // 6
	rows = append(rows, m.midRows(w)...)     // 7-9
	rows = append(rows, m.potRow(w))         // 10
	rows = append(rows, blank)               // 11
	rows = append(rows, m.heroRows(w)...)    // 12-14
	rows = append(rows, m.heroLineRow(w))    // 15
	rows = append(rows, rule)                // 16
	for i := 0; i < coachStripRows; i++ {    // 17-19: reserved even when silent
		line := blank
		if i < len(coach) {
			line = padStyledTo(coach[i], w)
		}
		rows = append(rows, line)
	}
	rows = append(rows, rule)                                      // 20
	rows = append(rows, strings.SplitN(m.bar.View(w), "\n", 2)...) // 21-22
	rows = append(rows, m.statusRow(w))                            // 23
	rows = append(rows, m.footerRow(w))                            // 24
	return rows
}

// headerRow: hand number, game, blinds, street on the left; the hero's
// position and the help hint on the right.
func (m *TableScreen) headerRow(w int) string {
	th := theme.Current
	dot := " " + theme.G.Dot + " "

	street := "waiting"
	pos := ""
	if m.hand != nil {
		street = strings.ToUpper(m.hand.Street().String())
		if m.hand.DealtIn().Has(heroSeat) {
			pos = m.hand.Position(heroSeat).String() + dot
		}
	}
	left := " " + "Hand #" + strconv.Itoa(m.handNo) + dot + "6-max NLHE" + dot +
		"blinds " + m.cfg.SmallBlind.String() + "/" + m.cfg.BigBlind.String() + dot + street
	right := pos + "? help "
	return rowLR(w, th.Header.Render(clip(left, w-lipgloss.Width(right)-1)), th.Footer.Render(right))
}

// topSeatRows renders seats 2, 3, 4 (top-left, top-center, top-right).
// Seat index is clockwise from the hero, matching engine action order, so
// after the hero acts the action sweeps up the left side, across the top,
// and down the right — a consistent visual read (§2.2).
func (m *TableScreen) topSeatRows(w int) []string {
	cols := []int{5, (w - seatSlotW) / 2, w - 5 - seatSlotW}
	blocks := []placedBlock{
		{cols[0], m.seatView(2).Render()},
		{cols[1], m.seatView(3).Render()},
		{cols[2], m.seatView(4).Render()},
	}
	return bandRows(w, 3, blocks)
}

// midRows renders mid-left seat 1, the board, and mid-right seat 5.
func (m *TableScreen) midRows(w int) []string {
	boardCol := (w - boardW) / 2
	blocks := []placedBlock{
		{1, m.seatView(1).Render()},
		{boardCol, m.boardBlock()},
		{w - 1 - seatSlotW, m.seatView(5).Render()},
	}
	return bandRows(w, 3, blocks)
}

// heroRows renders the hero band: info line left, hole cards center (always
// face up, always bottom-center), hand-strength label right of the cards.
func (m *TableScreen) heroRows(w int) []string {
	cardCol := (w - boardW) / 2
	labelCol := cardCol + heroCardsW + 8

	cards := m.heroCardsBlock()
	lines := strings.Split(cards, "\n")
	for len(lines) < 3 {
		lines = append(lines, "")
	}

	label := theme.Current.SeatAction.Render(clip(m.heroStrength(), w-labelCol-1))
	rows := make([]string, 3)
	rows[0] = overlayRow(w, at{cardCol, lines[0]})
	rows[1] = overlayRow(w, at{0, m.heroInfo(cardCol)}, at{cardCol, lines[1]}, at{labelCol, label})
	rows[2] = overlayRow(w, at{cardCol, lines[2]})
	return rows
}

// heroInfo is the hero's one-line seat header: turn marker, name, position
// badge, dealer disc, stack, chips in front.
func (m *TableScreen) heroInfo(width int) string {
	g := theme.G
	th := theme.Current

	marker, markerStyle := strings.Repeat(" ", lipgloss.Width(g.ToAct)), th.SeatName
	var badge, dealer, stack, bet string
	stackStyle := th.SeatStack
	if m.hand != nil && m.hand.DealtIn().Has(heroSeat) {
		h := m.hand
		if h.Phase() == engine.PhaseBetting && h.CurrentSeat() == heroSeat {
			marker, markerStyle = g.ToAct, th.SeatToAct
		}
		badge = theme.PositionBadge(h.Position(heroSeat))
		if h.Button() == heroSeat {
			dealer = g.Dealer
		}
		stack = h.Stack(heroSeat).String()
		if h.AllIn().Has(heroSeat) {
			stack, stackStyle = "ALL-IN", th.SeatAllIn
		}
		if c := h.Committed(heroSeat); c > 0 {
			bet = g.ChipsInFront + " " + c.String()
		}
	} else {
		stack = m.eng.Stack(heroSeat).String()
	}

	parts := []strut{
		{"   ", th.SeatName},
		{marker, markerStyle},
		{heroName, th.HeroBadge},
		{" ", th.SeatName},
		{badge, th.PosBadge},
	}
	if dealer != "" {
		// The dealer disc glyph carries its own padding (" (D) " parity), so
		// no separator space follows it.
		parts = append(parts, strut{dealer, th.DealerBadge}, strut{stack, stackStyle})
	} else {
		parts = append(parts, strut{" ", th.SeatName}, strut{stack, stackStyle})
	}
	if bet != "" {
		// Single space: the wide layout's narrower left column leaves the
		// hero line exactly enough room for stack + bet with no slack.
		parts = append(parts, strut{" ", th.SeatName}, strut{bet, th.SeatBet})
	}
	return strutRow(width, parts...)
}

// heroLineRow is the reserved hero action/grade line (row 15, §5.3): the
// hero's last decision joined by its frozen grade — "calls 85  ✗ Mistake ·
// Fold was the play (-1.2bb)". Both persist until the hero's next decision
// replaces them (or the showdown reveal takes the row): grades are computed
// forever, so they must not be shown for a second (ui-review F2).
//
// The grade suffix is composed at render time from lastGrade and the LIVE
// coach mode, so cycling verbosity with tab applies immediately and
// CoachMistakes/CoachOff withhold exactly what they promise to withhold.
func (m *TableScreen) heroLineRow(w int) string {
	th := theme.Current
	action := clip(m.heroLine, w-4)
	suffix := ""
	if m.heroGraded && m.lastGrade != nil && m.lastGrade.Feedback(m.coachMode.ProfileKey()) != "" {
		suffix = gradeSummary(*m.lastGrade)
	}
	if suffix == "" {
		return padStyledTo("   "+th.SeatAction.Render(action), w)
	}
	style := th.GradeBad
	if m.lastGrade.Grade.GoodOrBetter() {
		style = th.GradeGood
	}
	suffix = clip(suffix, w-4-visibleWidth(action)-2)
	return padStyledTo("   "+th.SeatAction.Render(action)+"  "+style.Render(suffix), w)
}

// potRow renders the pot region: the derived pot layers in the open —
// MAIN 900 . SIDE 710 (Nia . you) — or, during the showdown payout, the
// award being paid.
func (m *TableScreen) potRow(w int) string {
	if m.awardText != "" {
		text := theme.Current.PotLine.Render(clip(m.awardText, w))
		pad := (w - lipgloss.Width(text)) / 2
		if pad < 0 {
			pad = 0
		}
		return padStyledTo(strings.Repeat(" ", pad)+text, w)
	}
	return components.PotLine(m.potViews(), w)
}

// potViews maps the engine's derived pots onto the component view model.
// Side pots name their eligible seats because side pots are exactly the
// kind of rule beginners find opaque — the display does the bookkeeping
// visibly (§2.5).
func (m *TableScreen) potViews() []components.PotView {
	if m.hand == nil || m.hand.Phase() == engine.PhaseComplete {
		return nil
	}
	// Until someone is all-in there is nothing side-pot-like to teach: the
	// engine's derived layers would expose every transient uncalled bet as
	// its own layer (the BB "overpays" the SB preflop, every open is
	// "uncalled" for a moment). One total is the truth players track.
	if m.hand.AllIn().Empty() {
		return []components.PotView{{Amount: m.hand.PotTotal()}}
	}
	pots := m.hand.Pots()
	views := make([]components.PotView, len(pots))
	for i, p := range pots {
		views[i].Amount = p.Amount
		if i > 0 {
			for _, s := range p.Eligible.Seats() {
				name := m.seatName(s)
				if s == heroSeat {
					name = "you"
				}
				views[i].Eligible = append(views[i].Eligible, name)
			}
		}
	}
	return views
}

// boardBlock renders the community board: dealt cards fill left to right,
// undealt slots are dim placeholder pips in the exact box a card would
// occupy. At showdown, cards that play in the winning five get the gold
// winner border — board cards included, which is the "best five of seven"
// lesson made visible (§3.5).
func (m *TableScreen) boardBlock() string {
	var board []engine.Card
	if m.hand != nil {
		full := m.hand.Board()
		if m.boardShown < len(full) {
			full = full[:m.boardShown]
		}
		board = full
	}
	slots := make([]string, components.BoardSlots)
	for i := range slots {
		if i < len(board) {
			slots[i] = m.cardCell(board[i])
		} else {
			slots[i] = placeholderCell()
		}
	}
	return hjoin(slots, " ")
}

// cardCell renders one mini card, winner-bordered when it plays in the
// winning five.
func (m *TableScreen) cardCell(c engine.Card) string {
	if m.winners.Has(c) {
		return winnerMiniCard(c)
	}
	return components.MiniCard(c)
}

// placeholderCell is an undealt board slot: same box, dim pip.
func placeholderCell() string {
	blank := strings.Repeat(" ", components.MiniCardWidth)
	return blank + "\n" + theme.Current.BoardPlaceholder.Render(theme.G.BoardSlot) + "\n" + blank
}

// winnerMiniCard is components.MiniCard with the gold winner border —
// euchre's trick-winner treatment (§2.5). It lives here rather than in
// components because winner-ness is a table concept.
func winnerMiniCard(c engine.Card) string {
	g := theme.G
	border := theme.Current.CardWinner
	face := "??"
	interior := border.Render(face)
	if c.Valid() {
		face = string(c.Rank().Letter()) + theme.SuitGlyph(c.Suit())
		interior = theme.SuitStyle(c.Suit()).Bold(true).Render(face)
	}
	top := border.Render(g.CardTL + g.CardH + g.CardH + g.CardTR)
	mid := border.Render(g.CardVL) + interior + border.Render(g.CardVR)
	bottom := border.Render(g.CardBL + g.CardH + g.CardH + g.CardBR)
	return top + "\n" + mid + "\n" + bottom
}

// heroCardsBlock renders the hero's hole cards as mini cards (or the muck
// pips after a fold — folded hands are gone, not hidden).
func (m *TableScreen) heroCardsBlock() string {
	if m.hand == nil {
		return "\n\n"
	}
	hole, ok := m.hand.HoleCards(heroSeat)
	if !ok {
		return "\n\n"
	}
	if !m.hand.Live().Has(heroSeat) {
		return "\n" + theme.Current.SeatFolded.Render(theme.G.Mucked) + "\n"
	}
	return hjoin([]string{m.cardCell(hole[0]), m.cardCell(hole[1])}, " ")
}

// heroStrength is the hand-strength label right of the hero's cards:
// preflop shorthand ("ace-king suited") before the flop, the evaluator's
// English afterwards. TODO(wire-coach): the coach may refine this wording
// ("top pair, top kicker").
func (m *TableScreen) heroStrength() string {
	if m.hand == nil {
		return ""
	}
	hole, ok := m.hand.HoleCards(heroSeat)
	if !ok {
		return ""
	}
	if !m.hand.Live().Has(heroSeat) {
		return "folded"
	}
	if m.boardShown >= 3 {
		board := m.hand.Board()
		if m.boardShown < len(board) {
			board = board[:m.boardShown]
		}
		return strings.ToLower(eval.EvalHoldem(hole, board).String())
	}
	return preflopLabel(hole)
}

// preflopLabel names hole cards the way players say them: "ace-king
// suited", "pair of nines", "queen-ten offsuit".
func preflopLabel(hole [2]engine.Card) string {
	hi, lo := hole[0], hole[1]
	if lo.Rank() > hi.Rank() {
		hi, lo = lo, hi
	}
	if hi.Rank() == lo.Rank() {
		return "pair of " + strings.ToLower(hi.Rank().Plural())
	}
	label := strings.ToLower(hi.Rank().String()) + "-" + strings.ToLower(lo.Rank().String())
	if hi.Suit() == lo.Suit() {
		return label + " suited"
	}
	return label + " offsuit"
}

// statusRow is row 23: whose turn / pacing prompts / temp messages, plus
// the villain-perspective pot odds while sizing.
func (m *TableScreen) statusRow(w int) string {
	th := theme.Current
	if m.bar.State() == ActionBarSizing {
		verb := "bet"
		if m.bar.SizingType() == engine.ActionRaise {
			verb = "raise to"
		}
		dot := " " + theme.G.Dot + " "
		left := " enter " + verb + " " + m.bar.ConfirmAmount().String() + dot + "esc cancel"
		right := m.villainOddsText() + " "
		return rowLR(w, th.StatusLine.Render(clip(left, w-lipgloss.Width(right)-1)),
			th.ActionLabel.Render(right))
	}
	return padStyledTo(" "+th.StatusLine.Render(clip(m.status, w-2)), w)
}

// footerRow is row 24, right-aligned. The review slot is reserved with
// spaces while a hand is live so "esc menu" never shifts column when the
// hand ends.
func (m *TableScreen) footerRow(w int) string {
	dot := " " + theme.G.Dot + " "
	review := dot + "v review "
	if !m.handDone {
		review = strings.Repeat(" ", lipgloss.Width(review))
	}
	text := "esc menu" + dot + "? help" + review
	return rowLR(w, "", theme.Current.Footer.Render(text))
}

// seatView maps a villain seat's engine state onto the component view model.
func (m *TableScreen) seatView(s engine.Seat) components.SeatView {
	v := components.SeatView{Name: m.seatName(s), Width: seatSlotW}
	if m.eng.Status(s) == engine.SeatEmpty {
		return components.SeatView{Width: seatSlotW}
	}
	if m.hand == nil || !m.hand.DealtIn().Has(s) {
		v.Stack = m.eng.Stack(s)
		v.Folded = true
		v.LastAction = "sitting out"
		return v
	}
	h := m.hand
	v.Position = h.Position(s)
	v.Stack = h.Stack(s)
	v.Bet = h.Committed(s)
	v.Folded = !h.Live().Has(s)
	v.AllIn = h.AllIn().Has(s)
	v.Dealer = h.Button() == s
	v.ToAct = h.Phase() == engine.PhaseBetting && h.CurrentSeat() == s
	v.Hero = s == heroSeat
	v.LastAction = m.lastAction[s]
	if s == heroSeat || m.revealed[s] {
		if hole, ok := h.HoleCards(s); ok {
			v.Hole, v.ShowCards = hole, true
		}
	}
	return v
}

// coachView assembles what the coach region shows: the live recommendation
// while it is the hero's turn, the grade of the hero's last action while the
// villains respond, and always the numbers.
//
// Verbosity is applied by the renderer, not here, with one exception: at
// CoachMistakes the opinion is withheld before acting while the numbers stay
// (pot odds are the scoreboard a HUD-less player must compute anyway; showing
// them is not coaching). Building the advice regardless keeps grading
// available in every mode — silence is a display choice, not an amnesty.
func (m *TableScreen) coachView() CoachView {
	v := CoachView{}
	if m.hand == nil {
		return v
	}

	if m.advice != nil {
		if m.coachMode == CoachFull {
			v.Headline = m.advice.Headline
			v.Body = m.advice.Body
		}
		for _, c := range m.advice.Numbers {
			v.Chips = append(v.Chips, NumberChip{Label: c.Label, Value: c.Value})
		}
	}
	pot := m.hand.PotTotal()
	toCall := m.hand.ToCall(heroSeat)
	if m.bar.State() != ActionBarWaiting && toCall > 0 {
		// The compact layout's one number that must survive (ui-review F6).
		v.Price = "to call " + toCall.String() + " " + theme.G.Dot + " " +
			equity.PotOddsText(toCall, pot)
	}
	if len(v.Chips) == 0 {
		if m.bar.State() != ActionBarWaiting && toCall > 0 {
			v.Chips = append(v.Chips,
				NumberChip{"to call", toCall.String()},
				NumberChip{"pot odds", equity.PotOddsText(toCall, pot)},
			)
		} else if pot > 0 {
			v.Chips = append(v.Chips,
				NumberChip{"pot", pot.String() + " (" + m.hand.Blinds().BB(pot) + ")"},
			)
		}
	}

	// The grade of the hero's most recent action. It persists past the
	// villains' responses (ui-review F2), but the moment fresh advice is up
	// the strip belongs to the NEW decision — the old grade stays on the
	// hero's reserved line instead (heroLineRow) — and between hands the
	// strip belongs to the whole hand's verdict (coachNeutral). Feedback
	// returns "" when the mode withholds it.
	if g := m.lastGrade; g != nil && m.advice == nil && !m.handDone {
		if body := g.Feedback(m.coachMode.ProfileKey()); body != "" {
			v.Grade = &GradeView{
				Symbol: gradeSymbol(g.Grade),
				Text:   body,
				Good:   g.Grade.GoodOrBetter(),
			}
		}
	}
	return v
}

// coachNeutral is the summary the reserved strip shows when the coach has
// no advice up — the slot stays occupied, the table never reflows. Between
// hands it carries the hand's verdict, which is also the reason to press v
// (ui-review F2): the grades were computed and frozen either way.
func (m *TableScreen) coachNeutral() string {
	if m.hand == nil {
		return "between hands"
	}
	if m.hand.Phase() == engine.PhaseComplete {
		if s := m.handVerdict(); s != "" {
			return s
		}
		return "between hands"
	}
	return m.hand.Street().String() + " " + theme.G.Dot + " pot " + m.hand.PotTotal().String()
}

// handVerdict summarizes the finished hand's frozen grades for the
// between-hands strip: "hand over · 1 Best, 1 Mistake (-1.2bb) · v reviews".
// It returns "" when the mode withholds it — CoachOff stays silent, and
// CoachMistakes speaks only when a decision actually leaked (§5.4).
func (m *TableScreen) handVerdict() string {
	if m.coachMode == CoachOff || len(m.grades) == 0 {
		return ""
	}
	var counts [5]int
	var loss float64
	leaked := false
	for _, g := range m.grades {
		if gr := int(g.Grade); gr >= 0 && gr < len(counts) {
			counts[gr]++
		}
		loss += g.EVLossBB
		if !g.Grade.GoodOrBetter() {
			leaked = true
		}
	}
	if m.coachMode == CoachMistakes && !leaked {
		return ""
	}
	var parts []string
	for gr := coach.GradeBest; gr <= coach.GradeBlunder; gr++ {
		if n := counts[gr]; n > 0 {
			parts = append(parts, strconv.Itoa(n)+" "+gr.Label())
		}
	}
	dot := " " + theme.G.Dot + " "
	s := "hand over" + dot + strings.Join(parts, ", ")
	if loss >= 0.05 {
		s += " (-" + strconv.FormatFloat(loss, 'f', 1, 64) + "bb)"
	}
	return s + dot + "v reviews"
}

// --- Compact ledger (<80 cols, §2.7) ----------------------------------------

// viewCompact renders the one-line-per-seat ledger: not a degraded
// afterthought — every number stays on screen, which is the whole point.
func (m *TableScreen) viewCompact(w, h int) string {
	th := theme.Current
	dot := " " + theme.G.Dot + " "
	rule := th.Rule.Render(strings.Repeat(theme.G.RuleH, w))
	blank := strings.Repeat(" ", w)

	street := "waiting"
	pot := ""
	if m.hand != nil {
		street = strings.ToUpper(m.hand.Street().String())
		if m.hand.Phase() != engine.PhaseComplete {
			pot = "POT " + m.hand.PotTotal().String() + " "
		}
	}
	header := rowLR(w,
		th.Header.Render(" #"+strconv.Itoa(m.handNo)+dot+
			m.cfg.SmallBlind.String()+"/"+m.cfg.BigBlind.String()+dot+street),
		th.PotLine.Render(pot))

	rows := []string{header, rule}
	for _, s := range []engine.Seat{1, 2, 3, 4, 5, heroSeat} {
		v := m.seatView(s)
		v.Width = w
		rows = append(rows, v.RenderCompact())
	}
	rows = append(rows, rule)
	rows = append(rows, m.compactBoardRow(w))
	rows = append(rows, rule)
	rows = append(rows, padStyledTo(renderCoachLine(m.coachView(), m.coachMode, m.coachNeutral(), w), w))
	rows = append(rows, rule)
	rows = append(rows, strings.SplitN(m.bar.View(w), "\n", 2)...)
	rows = append(rows, blank)
	rows = append(rows, m.statusRow(w))
	rows = append(rows, m.footerRow(w))
	rows = padRowsAboveFooter(rows, h, blank)
	return cropHeight(strings.Join(rows, "\n"), h)
}

// compactBoardRow: " Board  K. 7. 2. .  .        top pair" — inline cards,
// dot pips for undealt slots, strength label right-aligned.
func (m *TableScreen) compactBoardRow(w int) string {
	th := theme.Current
	var sb strings.Builder
	sb.WriteString(" " + th.Header.Render("Board") + "  ")
	var board []engine.Card
	if m.hand != nil {
		board = m.hand.Board()
		if m.boardShown < len(board) {
			board = board[:m.boardShown]
		}
	}
	for i := 0; i < components.BoardSlots; i++ {
		if i < len(board) {
			sb.WriteString(components.InlineCard(board[i]) + " ")
		} else {
			sb.WriteString(th.BoardPlaceholder.Render(theme.G.Dot) + "  ")
		}
	}
	label := th.SeatAction.Render(clip(m.heroStrength(), w/2) + " ")
	return rowLR(w, sb.String(), label)
}

// --- Row-assembly helpers ----------------------------------------------------
//
// These are the layout-stability primitives for styled text: they measure
// with lipgloss.Width (ANSI-aware) and pad with plain spaces, so a style
// change can never move a column.

// strut is a run of text with its style — the local sibling of the seat
// component's segment type, for one-off rows the components don't cover.
type strut struct {
	text  string
	style lipgloss.Style
}

// strutRow renders struts into exactly width cells, clipping (never
// wrapping) and padding.
func strutRow(width int, parts ...strut) string {
	if width <= 0 {
		return ""
	}
	var b strings.Builder
	remaining := width
	for _, p := range parts {
		if remaining <= 0 {
			break
		}
		text := p.text
		if lipgloss.Width(text) > remaining {
			text = truncateTo(text, remaining)
		}
		remaining -= lipgloss.Width(text)
		b.WriteString(p.style.Render(text))
	}
	if remaining > 0 {
		b.WriteString(strings.Repeat(" ", remaining))
	}
	return b.String()
}

// visibleWidth is lipgloss.Width under a name that says what it measures.
func visibleWidth(s string) int { return lipgloss.Width(s) }

// clip truncates plain text only when it overflows. render.go's truncateTo
// unconditionally appends the ellipsis (its callers pre-measure); this is
// the measure-first form for one-off call sites.
func clip(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	return truncateTo(s, width)
}

// padStyledTo pads a styled line with spaces to exactly width cells.
// Content already wider is returned unchanged — callers budget content.
func padStyledTo(s string, width int) string {
	if gap := width - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// rowLR lays a left and a right styled piece on one row of exactly width
// cells, right piece right-aligned. When both cannot fit, the right piece
// is dropped whole: it is always the auxiliary readout, and clipping a
// styled string mid-escape would be worse than losing it.
func rowLR(width int, left, right string) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return padStyledTo(left, width)
	}
	return left + strings.Repeat(" ", gap) + right
}

// at places a styled piece at a fixed column.
type at struct {
	col int
	s   string
}

// overlayRow composes one row of exactly width cells from pieces at fixed
// columns (ascending order; callers budget against overlap).
func overlayRow(width int, pieces ...at) string {
	var b strings.Builder
	cur := 0
	for _, p := range pieces {
		if p.s == "" {
			continue
		}
		if p.col > cur {
			b.WriteString(strings.Repeat(" ", p.col-cur))
			cur = p.col
		}
		b.WriteString(p.s)
		cur += lipgloss.Width(p.s)
	}
	if cur < width {
		b.WriteString(strings.Repeat(" ", width-cur))
	}
	return b.String()
}

// placedBlock is a multi-row block placed at a column within a band.
type placedBlock struct {
	col   int
	block string
}

// bandRows renders a band of rows high, overlaying each block's lines at
// its column. Blocks shorter than the band contribute blank lines.
func bandRows(width, rows int, blocks []placedBlock) []string {
	split := make([][]string, len(blocks))
	for i, b := range blocks {
		split[i] = strings.Split(b.block, "\n")
	}
	out := make([]string, rows)
	for r := 0; r < rows; r++ {
		pieces := make([]at, 0, len(blocks))
		for i, b := range blocks {
			if r < len(split[i]) {
				pieces = append(pieces, at{b.col, split[i][r]})
			}
		}
		out[r] = overlayRow(width, pieces...)
	}
	return out
}

// padRowsAboveFooter grows a fixed-budget frame to the terminal height by
// inserting blank rows above the status/footer pair, keeping them glued to
// the bottom of the screen where the eye expects them.
func padRowsAboveFooter(rows []string, h int, blank string) []string {
	extra := h - len(rows)
	if extra <= 0 || len(rows) < 2 {
		return rows
	}
	out := make([]string, 0, h)
	out = append(out, rows[:len(rows)-2]...)
	for i := 0; i < extra; i++ {
		out = append(out, blank)
	}
	return append(out, rows[len(rows)-2:]...)
}

// hjoin joins equal-height multi-line blocks horizontally with a gap.
func hjoin(blocks []string, gap string) string {
	split := make([][]string, len(blocks))
	rows := 0
	for i, b := range blocks {
		split[i] = strings.Split(b, "\n")
		if len(split[i]) > rows {
			rows = len(split[i])
		}
	}
	lines := make([]string, rows)
	for r := 0; r < rows; r++ {
		parts := make([]string, len(blocks))
		for i := range blocks {
			if r < len(split[i]) {
				parts[i] = split[i][r]
			}
		}
		lines[r] = strings.Join(parts, gap)
	}
	return strings.Join(lines, "\n")
}
