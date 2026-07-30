package tutorial

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/components"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
)

// Visual is a lesson's diagram: exactly one of the pointers is set. All
// rendering goes through internal/ui — cards via components, styles and
// glyphs via theme — so lesson visuals look identical to the live table.
type Visual struct {
	Board      *VisualBoard
	Showdown   *VisualShowdown
	RangeGrid  *VisualRangeGrid
	TableSeats *VisualTableSeats
	HandLadder *VisualHandLadder
	PotOdds    *VisualPotOdds

	Caption string // one line under the visual; may be ""
}

// Render draws the visual within width columns. Every visual fits 80
// columns (the app's floor); narrower widths clip captions but never wrap
// the diagram. Card visuals draw the full-size study card; height-starved
// callers use RenderCompact instead.
func (v *Visual) Render(width int) string { return v.render(width, false) }

// RenderCompact is Render with the card visuals in their 3-row mini form —
// for the 60x20 compact breakpoint, where a 5-row card plus its label rows
// would not leave room for the question under it. The non-card visuals are
// already text-height and render identically.
func (v *Visual) RenderCompact(width int) string { return v.render(width, true) }

func (v *Visual) render(width int, compact bool) string {
	var body string
	switch {
	case v.Board != nil:
		body = v.Board.render(width, compact)
	case v.Showdown != nil:
		body = v.Showdown.render(width, compact)
	case v.RangeGrid != nil:
		body = v.RangeGrid.Render(width)
	case v.TableSeats != nil:
		body = v.TableSeats.Render(width)
	case v.HandLadder != nil:
		body = v.HandLadder.Render(width)
	case v.PotOdds != nil:
		body = v.PotOdds.Render(width)
	default:
		return ""
	}
	if v.Caption != "" {
		body += "\n" + ColorizeCards(clipTo(v.Caption, width), theme.Current.Subtitle)
	}
	return body
}

// --- The full-size study card ---------------------------------------------------
//
// Two card sizes exist on purpose. The table screen keeps the 5x3 mini card:
// its seat ring already spends 13 of 24 rows, and 5-row cards would cost
// roughly four more rows between the board and the hero's hand, which that
// layout cannot pay. Lessons and the trainer exist to LOOK at cards, have
// the rows to spare, and so draw this full 7x5 card — rank in the top-left
// index, suit pip centred, rank mirrored bottom-right, like a real card
// (the euchre project's card, which the owner asked for). Glanceable on the
// table, a full card when the card itself is the subject.
//
// TODO(promote): this renderer belongs in internal/ui/components next to
// MiniCard; it lives here for now because this change does not own that
// package. Two theme gaps worth closing at promotion time: the palette has
// no card-face background (euchre paints a white card body; we render suit
// ink on the terminal background), and glyphs.go has no double-border set
// (euchre's emphasis border; we emphasise with the CardWinner gold ink the
// table's showdown already uses).

// Full-card geometry: every full card is exactly FullCardWidth cells wide
// and FullCardHeight rows tall. The interior is five cells, sized for the
// two-character "10" rank plus breathing room.
const (
	FullCardWidth  = 7
	FullCardHeight = 5

	fullInterior = FullCardWidth - 2
)

// FullCard renders the 7x5 study card. highlight swaps the frame to the
// CardWinner style — the emphasis form for "these five play".
func FullCard(c engine.Card, highlight bool) string {
	g := theme.G
	border := theme.Current.CardBorder
	if highlight {
		border = theme.Current.CardWinner
	}
	rank := c.Rank().Symbol()
	suit := theme.SuitGlyph(c.Suit())
	ink := theme.SuitStyle(c.Suit())
	pad := strings.Repeat(" ", fullInterior-lipgloss.Width(rank))
	mid := strings.Repeat(" ", (fullInterior-1)/2)
	rows := []string{
		border.Render(g.CardTL + strings.Repeat(g.CardH, fullInterior) + g.CardTR),
		border.Render(g.CardVL) + ink.Render(rank+pad) + border.Render(g.CardVR),
		border.Render(g.CardVL) + ink.Render(mid+suit+mid) + border.Render(g.CardVR),
		border.Render(g.CardVL) + ink.Render(pad+rank) + border.Render(g.CardVR),
		border.Render(g.CardBL + strings.Repeat(g.CardH, fullInterior) + g.CardBR),
	}
	return strings.Join(rows, "\n")
}

// fullPlaceholder is an undealt board slot at full-card size: a dim pip in
// the box a future card will fill, so the board's width never changes as
// streets arrive.
func fullPlaceholder() string {
	blank := strings.Repeat(" ", FullCardWidth)
	mid := strings.Repeat(" ", (FullCardWidth-1)/2)
	pip := mid + theme.Current.BoardPlaceholder.Render(theme.G.Dot) + mid
	return strings.Join([]string{blank, blank, pip, blank, blank}, "\n")
}

// fullBoardRow renders the community board as five full-size slots.
// highlight marks cards to draw in the emphasis frame; nil highlights none.
func fullBoardRow(dealt []engine.Card, highlight engine.CardSet) string {
	slots := make([]string, components.BoardSlots)
	for i := range slots {
		if i < len(dealt) {
			slots[i] = FullCard(dealt[i], highlight.Has(dealt[i]))
		} else {
			slots[i] = fullPlaceholder()
		}
	}
	return joinBlocks(" ", slots...)
}

// fullHand renders a two-card holding side by side at full size.
func fullHand(hole [2]engine.Card, highlight engine.CardSet) string {
	return joinBlocks(" ",
		FullCard(hole[0], highlight.Has(hole[0])),
		FullCard(hole[1], highlight.Has(hole[1])))
}

// VisualBoard shows community cards, optionally the hero's hole cards, and
// optionally the best five — computed by eval.Best5, never authored, so the
// highlight can't lie.
type VisualBoard struct {
	Board    []engine.Card
	Hole     [2]engine.Card
	ShowHole bool
	ShowBest bool // annotate Best5(hole, board); requires ShowHole and ≥3 board cards
}

// Render implements the visual at full-card size.
func (b *VisualBoard) Render(width int) string { return b.render(width, false) }

func (b *VisualBoard) render(width int, compact bool) string {
	var best engine.CardSet
	var bestLine string
	if b.ShowBest && b.ShowHole && len(b.Board) >= 3 {
		rank, five := eval.Best5(b.Hole, b.Board)
		best = engine.NewCardSet(five[:]...)
		bestLine = clipTo("Best five: "+inlineCards(five[:])+"  "+
			theme.Current.GradeGood.Render(rank.Describe()), width)
	}
	var out []string
	out = append(out, theme.Current.Body.Render("Board"))
	if compact {
		out = append(out, components.BoardRow(b.Board))
	} else {
		out = append(out, fullBoardRow(b.Board, best))
	}
	if b.ShowHole {
		out = append(out, theme.Current.Body.Render("Your hand"))
		if compact {
			out = append(out, joinBlocks(" ", components.MiniCard(b.Hole[0]), components.MiniCard(b.Hole[1])))
		} else {
			out = append(out, fullHand(b.Hole, best))
		}
	}
	if bestLine != "" {
		out = append(out, bestLine)
	}
	return strings.Join(out, "\n")
}

// ShownHand is one labelled holding in a VisualShowdown. The label names
// whose cards these are ("Player A", "Villain") — a comparison the learner
// cannot attribute at a glance teaches nothing.
type ShownHand struct {
	Label string
	Hole  [2]engine.Card
}

// VisualShowdown is the head-to-head diagram: an optional board plus two or
// more labelled hands drawn as cards. It exists because VisualBoard shows
// one holding, and a "who wins?" drill has to show several, each visibly
// owned.
type VisualShowdown struct {
	Board []engine.Card // empty for a preflop matchup
	Hands []ShownHand
}

// showdownGutter separates hand blocks; wide enough to read as separate
// hands, narrow enough that two full-size hands sit inside the 60-column
// compact floor.
const showdownGutter = "   "

// Render implements the visual at full-card size.
func (s *VisualShowdown) Render(width int) string { return s.render(width, false) }

func (s *VisualShowdown) render(width int, compact bool) string {
	cardW, hand := FullCardWidth, fullHand
	boardRow := func(b []engine.Card) string { return fullBoardRow(b, 0) }
	if compact {
		cardW = components.MiniCardWidth
		hand = func(hole [2]engine.Card, _ engine.CardSet) string {
			return joinBlocks(" ", components.MiniCard(hole[0]), components.MiniCard(hole[1]))
		}
		boardRow = components.BoardRow
	}
	blockWidth := 2*cardW + 1

	var out []string
	if len(s.Board) > 0 {
		out = append(out, theme.Current.Body.Render("Board"))
		out = append(out, boardRow(s.Board))
	}
	labels := make([]string, len(s.Hands))
	blocks := make([]string, len(s.Hands))
	for i, h := range s.Hands {
		label := clipTo(h.Label, blockWidth)
		gap := blockWidth - lipgloss.Width(label)
		if gap < 0 {
			gap = 0
		}
		labels[i] = theme.Current.Body.Render(label) + strings.Repeat(" ", gap)
		blocks[i] = hand(h.Hole, 0)
	}
	out = append(out, strings.Join(labels, showdownGutter))
	out = append(out, joinBlocks(showdownGutter, blocks...))
	return strings.Join(out, "\n")
}

// joinBlocks joins equal-height multi-line blocks horizontally with a gap,
// so a pair of mini cards renders side by side.
func joinBlocks(gap string, blocks ...string) string {
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

// inlineCards renders cards in prose form separated by spaces.
func inlineCards(cards []engine.Card) string {
	parts := make([]string, len(cards))
	for i, c := range cards {
		parts[i] = components.InlineCard(c)
	}
	return strings.Join(parts, " ")
}

// VisualRangeGrid is the 13×13 range chart — *the* preflop teaching device.
// Row and column 0 are Aces; the diagonal is pairs, above it suited, below
// it offsuit (equity.Grid's layout). In-range cells render in the good-grade
// style, fractional-weight cells in the subtitle style, out-of-range cells
// dim.
type VisualRangeGrid struct {
	Range equity.Range
	Title string // e.g. "Button opening range (~42%)"
}

// rangeGridFullWidth is the grid with one-column gaps: thirteen 4-character
// cells (the ten's labels — "1010", "A10s", "109o" — are the wide ones,
// per the owner's "10, never T" decision) plus twelve separators.
const rangeGridFullWidth = 13*4 + 12 // 64 columns, inside the 80-col floor

// Render implements the visual. At full width the cells get one-column
// gaps (64 columns); when the caller's budget is narrower — the 60-column
// compact floor — the gaps are dropped and the packed 52-column grid still
// fits without clipping a cell.
func (g *VisualRangeGrid) Render(width int) string {
	grid := g.Range.Grid()
	th := theme.Current
	gap := width <= 0 || width >= rangeGridFullWidth
	var out []string
	if g.Title != "" {
		out = append(out, th.Header.Render(clipTo(g.Title, width)))
	}
	for row := 0; row < 13; row++ {
		var b strings.Builder
		for col := 0; col < 13; col++ {
			label := cellLabel(row, col)
			style := th.SeatFolded
			switch w := grid[row][col]; {
			case w >= 0.999:
				style = th.GradeGood
			case w > 0:
				style = th.Subtitle
			}
			b.WriteString(style.Render(fmt.Sprintf("%-4s", label)))
			if gap && col < 12 {
				b.WriteByte(' ')
			}
		}
		out = append(out, b.String())
	}
	return strings.Join(out, "\n")
}

// cellLabel names a grid cell: "AA" on the diagonal, "AKs" above it, "AKo"
// below. Published charts write the ten as "T"; this app writes "10"
// everywhere by the owner's decision, so the ten's cells read "1010",
// "A10s", "109o" and the grid pays for it with a wider column.
func cellLabel(row, col int) string {
	r1 := engine.Rank(12 - row)
	r2 := engine.Rank(12 - col)
	switch {
	case row == col:
		return r1.Symbol() + r1.Symbol()
	case row < col:
		return r1.Symbol() + r2.Symbol() + "s"
	default:
		return r2.Symbol() + r1.Symbol() + "o"
	}
}

// VisualTableSeats is the 6-max table ring with position names, for the
// hand-flow and position lessons. Notes annotate seats ("acts first"), and
// highlighted positions render in the accent style.
type VisualTableSeats struct {
	Highlight []engine.Position
	Notes     map[engine.Position]string
}

// Render implements the visual: a fixed ring plus one note line per entry,
// in clockwise deal order.
func (t *VisualTableSeats) Render(width int) string {
	style := func(p engine.Position) string {
		name := p.String()
		if p == engine.PosBTN {
			name += " " + strings.TrimSpace(theme.G.Dealer)
		}
		for _, h := range t.Highlight {
			if h == p {
				return theme.Current.SeatToAct.Render(name)
			}
		}
		return theme.Current.Body.Render(name)
	}
	rows := []string{
		"        " + style(engine.PosSB) + "        " + style(engine.PosBB),
		"",
		"  " + style(engine.PosBTN) + "                    " + style(engine.PosUTG),
		"",
		"        " + style(engine.PosCO) + "        " + style(engine.PosHJ),
	}
	// Notes in preflop action order — the order the learner must internalize.
	order := []engine.Position{
		engine.PosUTG, engine.PosHJ, engine.PosCO,
		engine.PosBTN, engine.PosSB, engine.PosBB,
	}
	for _, p := range order {
		if note, ok := t.Notes[p]; ok {
			line := fmt.Sprintf("  %-3s  %s", p.String(), note)
			rows = append(rows, theme.Current.Subtitle.Render(clipTo(line, width)))
		}
	}
	return strings.Join(rows, "\n")
}

// LadderRung is one row of the hand ladder: a named strength tier with an
// example hand. The examples are data so the visual test can verify each one
// actually evaluates to its tier — the ladder must never misfile a hand.
type LadderRung struct {
	Name     string
	Cards    string // five card codes
	Category eval.Category
}

// LadderRungs returns the ten tiers strongest first. A Royal Flush is the
// ace-high straight flush; it gets its own rung because every learner will
// hear the name.
func LadderRungs() []LadderRung {
	return []LadderRung{
		{"Royal Flush", "As Ks Qs Js Ts", eval.StraightFlush},
		{"Straight Flush", "9h 8h 7h 6h 5h", eval.StraightFlush},
		{"Four of a Kind", "Qc Qd Qh Qs 7d", eval.FourOfAKind},
		{"Full House", "Kc Kh Kd 4s 4d", eval.FullHouse},
		{"Flush", "Ad Td 8d 6d 3d", eval.Flush},
		{"Straight", "9c 8d 7h 6s 5c", eval.Straight},
		{"Three of a Kind", "7c 7d 7h Ks 2d", eval.ThreeOfAKind},
		{"Two Pair", "Ac Ah 9c 9d Qs", eval.TwoPair},
		{"One Pair", "Jc Jh Ad 8s 3c", eval.OnePair},
		{"High Card", "Ah Qc 9d 6s 2h", eval.HighCard},
	}
}

// VisualHandLadder is the ten hand tiers ranked, each with an example.
// Highlight (1-based rung, 0 for none) accents one row.
type VisualHandLadder struct {
	Highlight int
}

// Render implements the visual.
func (l *VisualHandLadder) Render(width int) string {
	rungs := LadderRungs()
	out := make([]string, 0, len(rungs))
	for i, r := range rungs {
		nameStyle := theme.Current.Body
		if l.Highlight == i+1 {
			nameStyle = theme.Current.SeatToAct
		}
		line := fmt.Sprintf("%2d. ", i+1) + nameStyle.Render(fmt.Sprintf("%-16s", r.Name)) +
			inlineCards(engine.MustCards(r.Cards))
		out = append(out, clipTo(line, width))
	}
	return strings.Join(out, "\n")
}

// VisualPotOdds is the pot / call / required-equity diagram: a proportional
// bar of what's already in the middle versus what the call costs, with the
// two forms of the price — the ratio players say and the percentage they
// compare equity against.
type VisualPotOdds struct {
	Pot    engine.Chips // the pot before the hero's call, bet included
	ToCall engine.Chips
}

// Render implements the visual.
func (p *VisualPotOdds) Render(width int) string {
	const barWidth = 36
	total := p.Pot + p.ToCall
	potCells := barWidth / 2
	if total > 0 {
		potCells = int(float64(barWidth)*float64(p.Pot)/float64(total) + 0.5)
	}
	if potCells < 1 {
		potCells = 1
	}
	if potCells > barWidth-1 {
		potCells = barWidth - 1
	}
	th := theme.Current
	bar := th.PotLine.Render(strings.Repeat("#", potCells)) +
		th.SeatToAct.Render(strings.Repeat("=", barWidth-potCells))
	rows := []string{
		th.Body.Render(fmt.Sprintf("Pot: %s   To call: %s", p.Pot, p.ToCall)),
		bar,
		th.Subtitle.Render(fmt.Sprintf("%-*s%s", potCells, "pot", "your call")),
		th.Body.Render(equity.PotOddsText(p.ToCall, p.Pot) + "   " +
			equity.RequiredEquityText(p.ToCall, p.Pot)),
	}
	for i, r := range rows {
		rows[i] = clipTo(r, width)
	}
	return strings.Join(rows, "\n")
}

// cardCodeRE matches one or two concatenated card codes standing alone as a
// word: "As", "10d", and range-spec combos like "9c8c". Hand-class notation
// ("A5s", "K9s+", "A10s+") never matches — there the rank-plus-suit pair is
// glued to other word characters, so the \b anchors fail. The legacy "T"
// rank is deliberately absent: rendered text always spells the ten "10".
var cardCodeRE = regexp.MustCompile(`\b((?:10|[2-9AKQJ])[cdhs])((?:10|[2-9AKQJ])[cdhs])?\b`)

// ColorizeCards renders a prose line with every card code drawn through the
// shared inline-card component — coloured, suited, "10" for the ten — and
// everything else in base. Lesson prose, drill explanations and trainer
// prompts route through this so a sentence's "4c" looks exactly like the 4c
// on the table, not like a bare code the learner must decode. Each card
// keeps its plain-text cell width (the suit glyph is one cell, like the
// letter it replaces), so wrapping computed before colorizing stays valid.
func ColorizeCards(s string, base lipgloss.Style) string {
	ms := cardCodeRE.FindAllStringSubmatchIndex(s, -1)
	if len(ms) == 0 {
		return base.Render(s)
	}
	var b strings.Builder
	last := 0
	for _, m := range ms {
		if m[0] > last {
			b.WriteString(base.Render(s[last:m[0]]))
		}
		for _, g := range [][2]int{{m[2], m[3]}, {m[4], m[5]}} {
			if g[0] >= 0 {
				b.WriteString(inlineCode(s[g[0]:g[1]]))
			}
		}
		last = m[1]
	}
	if last < len(s) {
		b.WriteString(base.Render(s[last:]))
	}
	return b.String()
}

// inlineCode renders one card code through InlineCard, falling back to the
// raw text if it does not parse (the regex guarantees the shape, so the
// fallback is belt and braces).
func inlineCode(code string) string {
	cards, err := engine.ParseCards(code)
	if err != nil || len(cards) != 1 {
		return code
	}
	return components.InlineCard(cards[0])
}

// clipTo truncates a (possibly styled) line to width display cells. Styled
// segments are preserved when the line already fits; oversize lines fall
// back to a plain clip with the theme ellipsis.
func clipTo(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	// Clip rune-wise on display width. This drops styling on the clipped
	// line, which is acceptable for the rare too-narrow terminal.
	var b strings.Builder
	used := 0
	ell := theme.G.Ellipsis
	budget := width - lipgloss.Width(ell)
	for _, r := range stripANSI(s) {
		rw := lipgloss.Width(string(r))
		if used+rw > budget {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String() + ell
}

// stripANSI removes CSI escape sequences so clipTo can measure raw text.
func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case inEsc:
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
		case r == 0x1b:
			inEsc = true
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
