package tutorial

import (
	"fmt"
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
	RangeGrid  *VisualRangeGrid
	TableSeats *VisualTableSeats
	HandLadder *VisualHandLadder
	PotOdds    *VisualPotOdds

	Caption string // one line under the visual; may be ""
}

// Render draws the visual within width columns. Every visual fits 80
// columns (the app's floor); narrower widths clip captions but never wrap
// the diagram.
func (v *Visual) Render(width int) string {
	var body string
	switch {
	case v.Board != nil:
		body = v.Board.Render(width)
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
		body += "\n" + theme.Current.Subtitle.Render(clipTo(v.Caption, width))
	}
	return body
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

// Render implements the visual.
func (b *VisualBoard) Render(width int) string {
	var out []string
	out = append(out, theme.Current.Body.Render("Board"))
	out = append(out, components.BoardRow(b.Board))
	if b.ShowHole {
		out = append(out, theme.Current.Body.Render("Your hand"))
		out = append(out, joinBlocks(" ", components.MiniCard(b.Hole[0]), components.MiniCard(b.Hole[1])))
	}
	if b.ShowBest && b.ShowHole && len(b.Board) >= 3 {
		rank, best := eval.Best5(b.Hole, b.Board)
		line := "Best five: " + inlineCards(best[:]) + "  " +
			theme.Current.GradeGood.Render(rank.Describe())
		out = append(out, clipTo(line, width))
	}
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

// Render implements the visual. Thirteen rows of thirteen 3-character cells
// plus separators: 52 columns, comfortably inside 80.
func (g *VisualRangeGrid) Render(width int) string {
	grid := g.Range.Grid()
	th := theme.Current
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
			b.WriteString(style.Render(fmt.Sprintf("%-3s", label)))
			if col < 12 {
				b.WriteByte(' ')
			}
		}
		out = append(out, b.String())
	}
	return strings.Join(out, "\n")
}

// cellLabel names a grid cell: "AA" on the diagonal, "AKs" above it, "AKo"
// below, matching every published range chart.
func cellLabel(row, col int) string {
	r1 := engine.Rank(12 - row)
	r2 := engine.Rank(12 - col)
	switch {
	case row == col:
		return string(r1.Letter()) + string(r1.Letter())
	case row < col:
		return string(r1.Letter()) + string(r2.Letter()) + "s"
	default:
		return string(r2.Letter()) + string(r1.Letter()) + "o"
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
