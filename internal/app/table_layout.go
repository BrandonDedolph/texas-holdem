package app

import (
	"sort"
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

// Table layout — Direction E of docs/table-redesign-pitch-2.md on a rounded
// rectangle. The drawn ring gives every player one stretch of rim in TRUE
// clockwise seating order: hero embedded in the bottom edge, Tara and Nia up
// the left rim, Cole across the top, Ivy and Sam down the right — so "who is
// on my left" is readable off the shape. Each plate carries an engraved gold
// digit, its action order THIS street (engine.StreetOrder), and the digits
// re-stamp when the street turns: the preflop→postflop switch (blinds act
// last, then first) becomes a visible renumbering of the same six fixed
// plates, taught by the furniture every hand.
//
// Geometry note: the owner dropped the hexagon's diagonal corners — ╱ ╲ are
// not in the box-drawing joining family, so the chamfers read as loose marks
// rather than edges. The frame here is a rounded rectangle built from the
// ring glyph vocabulary (╭ ╮ ╰ ╯ ─ │ plus the plate fusion tees), drawn
// locally because components.HexRing cannot produce a zero-depth corner (its
// Corner <= 0 means "the default depth", and the clamp floors at 1).
//
// The one structural invariant everything hangs off: EVERY region has a
// fixed row budget and renders blank when empty. The ring keeps its rows
// when seats fold, the board keeps five slots before the flop, the coach
// strip is reserved even when the coach is off — the table never reflows,
// so the player's eyes learn fixed landmarks.
//
// Layout is chosen per-View() from width/height, never stored, so a live
// resize in either direction just works (§2.7).

// Plate slot widths, from the Direction E mockups: the top and hero plates
// inset into the horizontal edges, the four side plates fuse into the
// vertical rims.
const (
	ringMaxWidth = 78 // the drawn table's width cap at 80 cols
	topPlateW    = 24
	heroPlateW   = 26
	sidePlateW   = 22

	heroCardsW   = 2*components.MiniCardWidth + 1                        // two mini cards, one gap
	boardInlineW = components.BoardSlots*2 + (components.BoardSlots-1)*3 // inline cards, 3-col gaps

	// boardW is the width of five mini-card slots with one-col gaps — the
	// hand review's board band still draws the mini-card form.
	boardW = components.BoardSlots*components.MiniCardWidth + (components.BoardSlots - 1)
)

// ringSpec is the drawn table's geometry at one breakpoint: overall size and
// the frame rows its organs live on.
type ringSpec struct {
	w, h         int
	upper, lower int // first frame row of the upper/lower side plates
	statusRow    int // top seat's chips/status row
	potRow       int // pot tray row
	boardRow     int // community cards row
	cardsTop     int // first row of the hero's mini cards
}

// ringSpecFor sizes the table for the column budget. The full layout draws
// 13 rows; the wide layout's left column (deep) spends its extra height on
// one more felt row between the board and the lower plates.
func ringSpecFor(w int, deep bool) ringSpec {
	rw := w - 2
	if rw > ringMaxWidth {
		rw = ringMaxWidth
	}
	if deep {
		return ringSpec{w: rw, h: 14, upper: 3, lower: 8,
			statusRow: 1, potRow: 2, boardRow: 6, cardsTop: 10}
	}
	return ringSpec{w: rw, h: 13, upper: 3, lower: 7,
		statusRow: 1, potRow: 2, boardRow: 6, cardsTop: 9}
}

// plateRow reports whether a frame row belongs to the side plates (which
// bring their own borders, fused into the rim with tees).
func (sp ringSpec) plateRow(r int) bool {
	return (r >= sp.upper && r < sp.upper+3) || (r >= sp.lower && r < sp.lower+3)
}

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

// viewFull is the 80x24 target layout. Terminals taller than the budget get
// blank rows above the status/footer pair, which stays glued to the bottom.
func (m *TableScreen) viewFull(w, h int) string {
	coach := renderCoachStrip(m.coachView(), m.coachMode, m.coachNeutral(), w)
	rows := m.fullRows(w, strings.SplitN(coach, "\n", coachStripRows), false)
	rows = padRowsAboveFooter(rows, h, strings.Repeat(" ", w))
	return cropHeight(strings.Join(rows, "\n"), h)
}

// viewWide (>=104x28) swaps the coach strip for a bordered side panel that
// carries the FULL reasoning (never the [e more] clip) plus the session
// scoreboard, and reclaims — and doubles — the strip rows for the action
// ticker (§2.6, ui-review §5 q1). The left column is the full layout at the
// remaining width with the deepened ring, so every full-layout invariant
// carries over unchanged.
func (m *TableScreen) viewWide(w, h int) string {
	leftW := w - coachPanelWidth - 2
	rows := m.fullRows(leftW, m.tickerRows(leftW), true)
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

// wideTickerRows is the wide layout's action-log height: double the strip's
// budget, paid for by the rows the wide breakpoint guarantees. A fixed
// constant — the log region never grows with content.
const wideTickerRows = 6

// tickerRows renders the recent actions where the coach strip would be —
// the wide layout shows the coach in its side panel instead, and spends the
// reclaimed rows on a longer log, bottom-aligned so the newest action is
// always in the same place.
func (m *TableScreen) tickerRows(w int) []string {
	th := theme.Current
	rows := make([]string, wideTickerRows)
	n := len(m.recent)
	for i := 0; i < wideTickerRows && i < n; i++ {
		rows[wideTickerRows-1-i] = " " + th.Help.Render(clip(m.recent[n-1-i], w-1))
	}
	return rows
}

// fullRows builds the rows of the full layout. coach supplies the reserved
// rows between the rules — the 3-row strip at 80 cols, the wide layout's
// 6-row ticker — and every row is exactly w cells. Budgets per breakpoint:
// 24 rows at 80 cols (header 1, ring 13, order rail 1, rule, coach 3, rule,
// bar 2, status, footer), 28 wide (ring 14, ticker 6). The coach region has
// a fixed height per breakpoint, never a content-dependent one (ui-review
// F1); the strip stays reserved even when silent.
func (m *TableScreen) fullRows(w int, coach []string, deep bool) []string {
	rule := theme.Current.Rule.Render(strings.Repeat(theme.G.RuleH, w))

	rows := make([]string, 0, 28)
	rows = append(rows, m.headerRow(w))          // 1
	rows = append(rows, m.tableRows(w, deep)...) // 2..14 (wide: 2..15)
	rows = append(rows, m.orderRailRow(w))       // 15: the order rail
	rows = append(rows, rule)
	for i := range coach { // reserved even when silent
		rows = append(rows, padStyledTo(coach[i], w))
	}
	rows = append(rows, rule)
	rows = append(rows, m.barRows(w)...)
	rows = append(rows, m.statusRow(w))
	rows = append(rows, m.footerRow(w))
	return rows
}

// tableRows draws the ring and everything on it: the rounded-rectangle
// frame with the six seat plates fused into its rim in clockwise seating
// order, and the felt interior — pot tray, board, the hero's cards with the
// potential line, the chip-pile price, and the hero's last graded decision.
// Exactly sp.h rows, each exactly w cells.
func (m *TableScreen) tableRows(w int, deep bool) []string {
	th := theme.Current
	g := theme.G
	sp := ringSpecFor(w, deep)
	col := (w - sp.w) / 2
	if col < 0 {
		col = 0
	}
	orders := m.streetOrders()

	pieces := make([][]at, sp.h)
	add := func(row, c int, s string) {
		if s != "" && row >= 0 && row < sp.h {
			pieces[row] = append(pieces[row], at{col + c, s})
		}
	}

	topCol := (sp.w - topPlateW) / 2
	heroCol := (sp.w - heroPlateW) / 2
	rightCol := sp.w - sidePlateW

	// The frame. Top and bottom edges run corner to corner with the inline
	// plates fused in; the vertical rims carry every row the side plates
	// don't bring their own borders for.
	frame := th.RingFrame
	add(0, 0, frame.Render(g.RingTL+strings.Repeat(g.RingH, topCol-1)))
	add(0, topCol, m.plateView(3, topPlateW, orders).RenderInline())
	add(0, topCol+topPlateW,
		frame.Render(strings.Repeat(g.RingH, sp.w-topCol-topPlateW-1)+g.RingTR))
	add(sp.h-1, 0, frame.Render(g.RingBL+strings.Repeat(g.RingH, heroCol-1)))
	add(sp.h-1, heroCol, m.plateView(heroSeat, heroPlateW, orders).RenderInline())
	add(sp.h-1, heroCol+heroPlateW,
		frame.Render(strings.Repeat(g.RingH, sp.w-heroCol-heroPlateW-1)+g.RingBR))
	for r := 1; r < sp.h-1; r++ {
		if !sp.plateRow(r) {
			add(r, 0, frame.Render(g.RingV))
			add(r, sp.w-1, frame.Render(g.RingV))
		}
	}

	// The side plates, clockwise from the hero's left hand: Tara and Nia up
	// the left rim, Ivy and Sam down the right. Left-hand plates keep their
	// stack on the rim side and chips toward the pot; right-hand mirrored.
	sidePlates := []struct {
		seat engine.Seat
		row  int
		col  int
		hand components.PlateHand
		open components.PlateEdges
	}{
		{2, sp.upper, 0, components.PlateLeftHand, components.PlateEdges{Left: true}},
		{1, sp.lower, 0, components.PlateLeftHand, components.PlateEdges{Left: true}},
		{4, sp.upper, rightCol, components.PlateRightHand, components.PlateEdges{Right: true}},
		{5, sp.lower, rightCol, components.PlateRightHand, components.PlateEdges{Right: true}},
	}
	for _, p := range sidePlates {
		lines := strings.Split(m.plateView(p.seat, sidePlateW, orders).Render(p.hand, p.open), "\n")
		for i, line := range lines {
			add(p.row+i, p.col, line)
		}
	}

	// The top seat's chips/status on the felt beneath its plate, and the
	// showdown reveals lying on the felt in front of each side plate.
	add(sp.statusRow, topCol, m.topSeatRow(3, topPlateW, orders))
	add(sp.upper+1, sidePlateW+2, m.holeReveal(2))
	add(sp.lower+1, sidePlateW+2, m.holeReveal(1))
	if s := m.holeReveal(4); s != "" {
		add(sp.upper+1, rightCol-2-lipgloss.Width(s), s)
	}
	if s := m.holeReveal(5); s != "" {
		add(sp.lower+1, rightCol-2-lipgloss.Width(s), s)
	}

	// The felt centre: pot tray (chip pile in front of it while a price is
	// live), then the board as the high-contrast middle of the table.
	add(sp.potRow, 2, m.potPiece(sp.w-4))
	add(sp.boardRow, (sp.w-boardInlineW)/2, m.boardPiece())

	// The hero's corner of the felt: hole cards dead centre of the bottom
	// edge, hand strength on their left, the potential line ("what your
	// hand can become") on their right, and beneath those the hero's last
	// graded decision and the chip-pile price of the current one.
	cardCol := (sp.w - heroCardsW) / 2
	for i, line := range strings.Split(m.heroCardsBlock(), "\n") {
		add(sp.cardsTop+i, cardCol, line)
	}
	if s := m.heroStrength(); s != "" {
		piece := th.SeatAction.Render(clip(s, cardCol-8)) + " " + th.Rule.Render(g.RingH)
		add(sp.cardsTop+1, cardCol-3-lipgloss.Width(piece), piece)
	}
	feltRight := cardCol + heroCardsW + 2
	feltRightW := sp.w - 2 - feltRight
	if s := m.heroPotential(); s != "" {
		add(sp.cardsTop+1, feltRight,
			th.Rule.Render(g.RingH)+" "+th.Body.Render(clip(s, feltRightW-2)))
	}
	add(sp.cardsTop+2, 2, m.feltHeroLine(cardCol-5))
	add(sp.cardsTop+2, feltRight, m.feltPrice(feltRightW))

	rows := make([]string, sp.h)
	for r := range rows {
		ps := pieces[r]
		sort.SliceStable(ps, func(i, j int) bool { return ps[i].col < ps[j].col })
		rows[r] = overlayRow(w, ps...)
	}
	return rows
}

// streetOrders maps each dealt-in seat to its action-order digit for THIS
// street, first to act = "1" (engine.StreetOrder). Folded seats keep their
// place on the rim — the plate component masks their digit with the fold
// mark, which is exactly the "a fold vacates a turn, not a position" lesson.
func (m *TableScreen) streetOrders() map[engine.Seat]string {
	if m.hand == nil {
		return nil
	}
	out := make(map[engine.Seat]string)
	for i, s := range m.hand.StreetOrder() {
		out[s] = strconv.Itoa(i + 1)
	}
	return out
}

// plateView maps a seat's engine state onto the seat-plate view model.
func (m *TableScreen) plateView(s engine.Seat, width int, orders map[engine.Seat]string) components.SeatPlateView {
	if m.eng.Status(s) == engine.SeatEmpty {
		return components.SeatPlateView{Width: width}
	}
	v := components.SeatPlateView{
		Name:  m.seatName(s),
		Read:  m.reads[s],
		Hero:  s == heroSeat,
		Width: width,
	}
	if m.hand == nil || !m.hand.DealtIn().Has(s) {
		v.Stack = m.eng.Stack(s)
		v.Folded = true
		return v
	}
	h := m.hand
	v.Order = orders[s]
	v.Position = h.Position(s)
	v.Stack = h.Stack(s)
	v.Bet = h.Committed(s)
	v.Raised = seatAggressed(m.lastAction[s])
	v.Folded = !h.Live().Has(s)
	v.AllIn = h.AllIn().Has(s)
	v.Dealer = h.Button() == s
	v.ToAct = h.Phase() == engine.PhaseBetting && h.CurrentSeat() == s
	return v
}

// seatAggressed reports whether a seat's chips arrived by bet or raise this
// street, from the action label the event loop recorded — the raise marker
// flags aggression, not mere money.
func seatAggressed(label string) bool {
	return strings.HasPrefix(label, "raises") || strings.HasPrefix(label, "bets")
}

// topSeatRow is the felt row beneath the top seat's inline plate: stack and
// chips/status while the hand runs, stack and revealed cards at showdown.
func (m *TableScreen) topSeatRow(s engine.Seat, width int, orders map[engine.Seat]string) string {
	if m.revealed[s] && m.hand != nil {
		if hole, ok := m.hand.HoleCards(s); ok {
			text := theme.Current.SeatStack.Render(m.hand.Stack(s).String()) + "  " +
				components.InlineCard(hole[0]) + " " + components.InlineCard(hole[1])
			return centerStyled(text, width)
		}
	}
	return m.plateView(s, width, orders).RenderStatus(width)
}

// holeReveal is a side seat's showdown reveal, lying on the felt in front
// of its plate (two-row plates cannot carry card glyphs themselves).
func (m *TableScreen) holeReveal(s engine.Seat) string {
	if !m.revealed[s] || m.hand == nil {
		return ""
	}
	hole, ok := m.hand.HoleCards(s)
	if !ok {
		return ""
	}
	return components.InlineCard(hole[0]) + " " + components.InlineCard(hole[1])
}

// potPiece renders the pot tray, centered in exactly width cells: the
// derived layers in the open (MAIN 180 · SIDE 480 (Tara · Nia)), the award
// being paid during the showdown, or the single pot — with its chip pile
// lying in front while the hero faces a price, so the "win 3" side of the
// ratio is drawn where the money actually sits.
func (m *TableScreen) potPiece(width int) string {
	th := theme.Current
	if m.awardText != "" {
		return centerStyled(th.PotLine.Render(clip(m.awardText, width)), width)
	}
	views := m.potViews()
	if len(views) == 0 {
		return strings.Repeat(" ", width)
	}
	if len(views) > 1 {
		return components.PotLine(views, width)
	}
	text := th.PotLine.Render("POT " + views[0].Amount.String())
	if pp := m.pilePrice(); pp != nil && pp.PotPile > 0 {
		text = th.SeatBet.Render(strings.Repeat(theme.G.Chip, pp.PotPile)) + " " + text
	}
	return centerStyled(text, width)
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

// pilePrice is the chip-pile price of the current decision (coach.PilePrice
// — Direction H's organ grafted into E): non-nil exactly while the hero is
// choosing or sizing against a live bet.
func (m *TableScreen) pilePrice() *coach.PricePiles {
	if m.hand == nil || m.bar.State() == ActionBarWaiting {
		return nil
	}
	toCall := m.hand.ToCall(heroSeat)
	if toCall <= 0 {
		return nil
	}
	pp := coach.PilePrice(toCall, m.hand.PotTotal())
	return &pp
}

// feltPrice renders the call side of the price on the felt in front of the
// hero's cards: the call pile beside the plain-language ratio — "●● call 2
// to win 3 — need 40%". When the exact pile plus phrase outgrows the slot
// the pile is dropped before the phrase is clipped: the words carry the
// exact price, the chips only illustrate it.
func (m *TableScreen) feltPrice(maxW int) string {
	pp := m.pilePrice()
	if pp == nil {
		return ""
	}
	th := theme.Current
	pile := strings.Repeat(theme.G.Chip, pp.CallPile)
	if lipgloss.Width(pile)+1+lipgloss.Width(pp.Phrase) <= maxW {
		return th.SeatBet.Render(pile) + " " + th.Body.Render(pp.Phrase)
	}
	return th.Body.Render(clip(pp.Phrase, maxW))
}

// feltHeroLine is the hero's reserved action/grade line, lying on the felt
// beside his cards: the last decision joined by its frozen grade — "calls
// 10  ? Inaccuracy · Raise to 25 was the play". Both persist until the next
// decision replaces them (or the showdown reveal takes the slot): grades
// are computed forever, so they must not be shown for a second (ui-review
// F2). The suffix is composed at render time from lastGrade and the LIVE
// coach mode, so cycling verbosity applies immediately and CoachMistakes/
// CoachOff withhold exactly what they promise to withhold.
func (m *TableScreen) feltHeroLine(maxW int) string {
	th := theme.Current
	if m.heroLine == "" || maxW < 2 {
		return ""
	}
	action := clip(m.heroLine, maxW)
	suffix := ""
	if m.heroGraded && m.lastGrade != nil && m.lastGrade.Feedback(m.coachMode.ProfileKey()) != "" {
		suffix = gradeSummary(*m.lastGrade)
	}
	room := maxW - visibleWidth(action) - 2
	if suffix == "" || room < 4 {
		return th.SeatAction.Render(action)
	}
	style := th.GradeBad
	if m.lastGrade.Grade.GoodOrBetter() {
		style = th.GradeGood
	}
	return th.SeatAction.Render(action) + "  " + style.Render(clip(suffix, room))
}

// boardPiece renders the community cards as the felt's high-contrast
// centre: dealt cards inline, undealt slots as dim pips in the same grid.
// At showdown, cards that play in the winning five get the gold winner
// treatment — board cards included, which is the "best five of seven"
// lesson made visible (§3.5).
func (m *TableScreen) boardPiece() string {
	th := theme.Current
	var board []engine.Card
	if m.hand != nil {
		board = m.hand.Board()
		if m.boardShown < len(board) {
			board = board[:m.boardShown]
		}
	}
	parts := make([]string, components.BoardSlots)
	for i := range parts {
		switch {
		case i < len(board) && m.winners.Has(board[i]):
			c := board[i]
			parts[i] = th.CardWinner.Render(string(c.Rank().Letter()) + theme.SuitGlyph(c.Suit()))
		case i < len(board):
			parts[i] = components.InlineCard(board[i])
		default:
			parts[i] = th.BoardPlaceholder.Render(theme.G.Dot) + " "
		}
	}
	return strings.Join(parts, "   ")
}

// orderRailRow is the rail below the ring: this street's action order in
// one line — "order  ✗ Cole · 2 Ivy ▲ 30 · ✗ Sam · ► 4 YOU · 5 Tara — so
// who still acts behind you is spelled out even when the plates are being
// scanned for something else. Digits match the plates; folded seats keep
// their place under the fold mark; commitments ride along with the raise
// marker when they arrived by aggression.
func (m *TableScreen) orderRailRow(w int) string {
	if m.hand == nil {
		return strings.Repeat(" ", w)
	}
	th := theme.Current
	g := theme.G
	h := m.hand

	parts := []strut{{" order  ", th.Help}}
	for i, s := range h.StreetOrder() {
		if i > 0 {
			parts = append(parts, strut{" " + g.Dot + " ", th.Rule})
		}
		if h.Phase() == engine.PhaseBetting && h.CurrentSeat() == s {
			parts = append(parts, strut{g.ToAct, th.SeatToAct})
		}
		name := m.seatName(s)
		nameStyle := th.SeatName
		if s == heroSeat {
			nameStyle = th.HeroBadge
		}
		if !h.Live().Has(s) {
			parts = append(parts,
				strut{g.FoldMark + " ", th.SeatFolded}, strut{name, th.SeatFolded})
			continue
		}
		parts = append(parts,
			strut{strconv.Itoa(i+1) + " ", th.RingDigit}, strut{name, nameStyle})
		switch {
		case h.AllIn().Has(s):
			parts = append(parts, strut{" ", nameStyle}, strut{"ALL-IN", th.SeatAllIn})
		case h.Committed(s) > 0:
			mark, markStyle := g.Chip, th.SeatBet
			if seatAggressed(m.lastAction[s]) {
				mark, markStyle = g.RaiseMark, th.ActionRaise
			}
			parts = append(parts, strut{" ", nameStyle}, strut{mark, markStyle},
				strut{" " + h.Committed(s).String(), th.SeatBet})
		}
	}
	return strutRow(w, parts...)
}

// barRows is the two-row action region: the hero's action bar in play, and
// between hands the deal/review buttons in the same slot — the bar's grammar
// with the session's two verbs.
func (m *TableScreen) barRows(w int) []string {
	if m.handDone {
		th := theme.Current
		g := theme.G
		row := "  " + th.ButtonCheck.Render(g.ButtonFill+" space DEAL NEXT "+g.ButtonFill) +
			"   " + th.ButtonCall.Render(g.ButtonFill+" v REVIEW "+g.ButtonFill)
		return []string{padStyledTo(row, w), strings.Repeat(" ", w)}
	}
	return strings.SplitN(m.bar.View(w), "\n", 2)
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

// cardCell renders one mini card, winner-bordered when it plays in the
// winning five.
func (m *TableScreen) cardCell(c engine.Card) string {
	if m.winners.Has(c) {
		return winnerMiniCard(c)
	}
	return components.MiniCard(c)
}

// placeholderCell is an undealt board slot in the mini-card form: same box,
// dim pip. The live table's board is inline now; the hand review still
// renders the three-row band.
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

// heroStrength is the hand-strength label left of the hero's cards:
// preflop shorthand ("ace-king suited") before the flop, the evaluator's
// English afterwards.
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

// heroPotential is the "what your hand can become" line right of the hero's
// cards (coach.PotentialLine): what the hand flops and how often before the
// flop, the made hand plus its live draw after. Mid-deal (one or two board
// cards shown) it keeps the preflop line — the potential of a partial flop
// is not a concept worth a transient frame.
func (m *TableScreen) heroPotential() string {
	if m.hand == nil {
		return ""
	}
	hole, ok := m.hand.HoleCards(heroSeat)
	if !ok || !m.hand.Live().Has(heroSeat) {
		return ""
	}
	board := m.hand.Board()
	n := m.boardShown
	if n > len(board) {
		n = len(board)
	}
	if n < 3 {
		n = 0
	}
	return coach.PotentialLine(hole, board[:n])
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

// statusRow: whose turn / pacing prompts / temp messages, plus the
// villain-perspective pot odds while sizing.
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

// footerRow is the last row, right-aligned. The review slot is reserved with
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

// seatView maps a villain seat's engine state onto the compact ledger's
// view model (the full layout draws plates instead — plateView).
func (m *TableScreen) seatView(s engine.Seat) components.SeatView {
	v := components.SeatView{Name: m.seatName(s), Width: components.DefaultSeatWidth}
	if m.eng.Status(s) == engine.SeatEmpty {
		return components.SeatView{Width: components.DefaultSeatWidth}
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
	// The session scoreboard rides along in every mode: decision accuracy
	// and net chips are the player's own ledger, not coaching advice, and
	// letting them visibly disagree is the variance lesson (DESIGN.md §1).
	v.Session = m.sessionChips()
	v.BetweenHands = m.hand == nil || m.handDone || m.hand.Phase() == engine.PhaseComplete
	if m.hand == nil {
		return v
	}
	if v.BetweenHands {
		v.Verdict = m.handVerdict()
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
	// hero's reserved felt line instead (feltHeroLine) — and between hands
	// the strip belongs to the whole hand's verdict (coachNeutral). Feedback
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
// no advice up — the slot stays occupied, the table never reflows. The
// finished hand's verdict travels separately (CoachView.Verdict) so each
// mode can withhold or show it without string-sniffing this placeholder.
func (m *TableScreen) coachNeutral() string {
	if m.hand == nil || m.hand.Phase() == engine.PhaseComplete {
		return "between hands"
	}
	return m.hand.Street().String() + " " + theme.G.Dot + " pot " + m.hand.PotTotal().String()
}

// sessionChips is the running session scoreboard: hands played, decisions
// graded good-or-better as a percentage, and net chips — read straight from
// the counters RecordSession persists, never recomputed. Accuracy and net
// are SEPARATE chips by design: blending them into one score would erase
// the divergence the product exists to teach (DESIGN.md §1, ui-review §5
// q3). Empty until the sitting's first hand finishes.
func (m *TableScreen) sessionChips() []NumberChip {
	if m.sessionHands == 0 {
		return nil
	}
	acc := "none graded"
	if m.sessionGraded > 0 {
		pct := (m.sessionGood*100 + m.sessionGraded/2) / m.sessionGraded
		acc = strconv.Itoa(pct) + "% good (" + strconv.Itoa(m.sessionGood) + "/" +
			strconv.Itoa(m.sessionGraded) + ")"
	}
	return []NumberChip{
		{"hands", strconv.Itoa(m.sessionHands)},
		{"decisions", acc},
		{"net", signedChips(m.sessionNet)},
	}
}

// signedChips renders a net amount with an explicit sign — a scoreboard
// figure must say which way it points even when it points up.
func signedChips(c engine.Chips) string {
	if c >= 0 {
		return "+" + c.String()
	}
	return c.String()
}

// handVerdict summarizes the finished hand's frozen grades for the
// between-hands strip: "hand over · 1 Best, 1 Mistake (-1.2bb) · v review".
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
	return s + dot + "v review"
}

// --- Compact ledger (<80 cols, §2.7) ----------------------------------------

// viewCompact renders the one-line-per-seat ledger: not a degraded
// afterthought — every number stays on screen, which is the whole point.
// The one Hexagon organ that survives this size is the order digit column:
// each seat's action order this street, re-stamped when the street turns,
// with the fold mark holding a folded seat's place.
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

	orders := m.streetOrders()
	rows := []string{header, rule}
	for _, s := range []engine.Seat{1, 2, 3, 4, 5, heroSeat} {
		v := m.seatView(s)
		v.Width = w - 3
		digit, style := " ", th.RingDigit
		if m.hand != nil && m.hand.DealtIn().Has(s) {
			if !m.hand.Live().Has(s) {
				digit, style = theme.G.FoldMark, th.SeatFolded
			} else if d, ok := orders[s]; ok {
				digit = d
			}
		}
		rows = append(rows, " "+style.Render(digit)+" "+v.RenderCompact())
	}
	rows = append(rows, rule)
	rows = append(rows, m.compactBoardRow(w))
	rows = append(rows, rule)
	rows = append(rows, padStyledTo(renderCoachLine(m.coachView(), m.coachMode, m.coachNeutral(), w), w))
	rows = append(rows, rule)
	rows = append(rows, m.barRows(w)...)
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

// centerStyled centers a styled piece in exactly width cells.
func centerStyled(s string, width int) string {
	gap := width - lipgloss.Width(s)
	if gap <= 0 {
		return s
	}
	left := gap / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", gap-left)
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
