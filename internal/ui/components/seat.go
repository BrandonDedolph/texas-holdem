package components

import (
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// DefaultSeatWidth is the seat-block slot width used when a SeatView does
// not specify one (docs/design-tui.md section 2.2: 20 cols max).
const DefaultSeatWidth = 20

// SeatView is the view model for one seat block. The app layer maps engine
// state onto it; components never read engine.Hand directly.
type SeatView struct {
	Name     string
	Position engine.Position
	Stack    engine.Chips
	Bet      engine.Chips // chips in front this street; 0 renders nothing

	Hole      [2]engine.Card
	ShowCards bool // face up (hero, showdown, review)

	Folded bool // cards went to the muck; the whole block dims
	AllIn  bool // replaces the stack amount with ALL-IN
	Dealer bool // show the dealer disc after the position badge
	ToAct  bool // prefix the turn marker to the name
	Hero   bool // accent the name (the hero's own seat)

	LastAction string // last action this street ("raises to 30"); may be ""

	Width int // slot width in cells; 0 means DefaultSeatWidth
}

// Render renders the three-row seat block, each row exactly the slot width:
//
//	Nia HJ 830          name, position badge, dealer disc, stack
//	## ##  * 30         hole cards and chips in front
//	raises to 30        last action this street (reserved even when blank)
//
// Rows never wrap: the name and action label truncate instead, so the block
// can never exceed its slot.
func (v SeatView) Render() string {
	w := v.width()
	rows := []string{
		v.renderHeader(w),
		renderRow(w, v.cardSegs(true)...),
		renderRow(w, seg{truncateEllipsis(v.LastAction, w), v.actionStyle()}),
	}
	return strings.Join(rows, "\n")
}

// RenderCompact renders the one-row ledger form used below 80 columns:
//
//	-> YOU BB 900  Ah Kh  check
func (v SeatView) RenderCompact() string {
	w := v.width()
	segs := v.headerSegs(w)
	segs = append(segs, seg{text: "  "})
	segs = append(segs, v.cardSegs(false)...)
	if v.LastAction != "" {
		segs = append(segs, seg{text: "  "}, seg{v.LastAction, v.actionStyle()})
	}
	return renderRow(w, segs...)
}

func (v SeatView) width() int {
	if v.Width > 0 {
		return v.Width
	}
	return DefaultSeatWidth
}

// renderHeader renders row one with the name truncated so that the badge and
// stack always survive: reading stacks at all times is a habit worth
// teaching, so the stack is the last thing to lose cells.
func (v SeatView) renderHeader(w int) string {
	segs := v.headerSegs(w)
	return renderRow(w, segs...)
}

// headerSegs builds the marker/name/badge/dealer/stack segments with the
// name pre-truncated to the space the fixed parts leave over.
func (v SeatView) headerSegs(w int) []seg {
	g := theme.G
	th := theme.Current

	marker := strings.Repeat(" ", lipgloss.Width(g.ToAct))
	markerStyle := lipgloss.NewStyle()
	if v.ToAct {
		marker, markerStyle = g.ToAct, th.SeatToAct
	}

	badge := theme.PositionBadge(v.Position)
	badgeStyle := th.PosBadge
	if v.Position == engine.PosBTN {
		badgeStyle = th.DealerBadge // the BTN badge is gold
	}

	dealer := ""
	if v.Dealer {
		dealer = g.Dealer
	}

	stack := v.Stack.String()
	stackStyle := th.SeatStack
	if v.AllIn {
		stack, stackStyle = "ALL-IN", th.SeatAllIn
	}

	nameStyle := th.SeatName
	if v.Hero {
		nameStyle = th.HeroBadge
	}
	if v.Folded {
		nameStyle, badgeStyle, stackStyle = th.SeatFolded, th.SeatFolded, th.SeatFolded
	}

	fixed := lipgloss.Width(marker) + 1 + lipgloss.Width(badge) +
		lipgloss.Width(dealer) + 1 + lipgloss.Width(stack)
	nameBudget := w - fixed
	if nameBudget < 1 {
		nameBudget = 1
	}
	name := truncateEllipsis(v.Name, nameBudget)

	segs := []seg{
		{marker, markerStyle},
		{name, nameStyle},
		{text: " "},
		{badge, badgeStyle},
	}
	if dealer != "" {
		segs = append(segs, seg{dealer, th.DealerBadge})
	}
	segs = append(segs, seg{text: " "}, seg{stack, stackStyle})
	return segs
}

// cardSegs builds the hole-card segments: mucked pips when folded, face-up
// inline cards at showdown, a face-down pair otherwise. spaced selects the
// three-row block form ("## ##"); the compact form glues the pair ("####").
func (v SeatView) cardSegs(spaced bool) []seg {
	g := theme.G
	th := theme.Current

	var segs []seg
	switch {
	case v.Folded:
		segs = []seg{{g.Mucked, th.SeatFolded}}
	case v.ShowCards:
		segs = inlineCardSegs(v.Hole[0])
		segs = append(segs, seg{text: " "})
		segs = append(segs, inlineCardSegs(v.Hole[1])...)
	default:
		gap := ""
		if spaced {
			gap = " "
		}
		segs = []seg{
			{g.FaceDown, th.CardBack},
			{text: gap},
			{g.FaceDown, th.CardBack},
		}
	}

	if v.Bet > 0 {
		segs = append(segs,
			seg{text: "  "},
			seg{g.ChipsInFront + " " + v.Bet.String(), th.SeatBet},
		)
	}
	return segs
}

// inlineCardSegs is InlineCard as segments, so seat rows can truncate and
// pad without splicing styled strings.
func inlineCardSegs(c engine.Card) []seg {
	if !c.Valid() {
		return []seg{{"??", theme.Current.CardBorder}}
	}
	face := string(c.Rank().Letter()) + theme.SuitGlyph(c.Suit())
	return []seg{{face, theme.SuitStyle(c.Suit())}}
}

func (v SeatView) actionStyle() lipgloss.Style {
	if v.Folded {
		return theme.Current.SeatFolded
	}
	return theme.Current.SeatAction
}

// seg is a run of plain text with the style it renders in. Rows are built
// from segments so truncation happens on plain text, never inside an ANSI
// escape sequence.
type seg struct {
	text  string
	style lipgloss.Style
}

// renderRow renders segments into exactly width cells: segments past the
// budget are clipped (display-width aware, never wrapped) and the remainder
// is padded with spaces. This is the layout-stability primitive every
// one-row component shares.
func renderRow(width int, segs ...seg) string {
	if width <= 0 {
		return ""
	}
	var b strings.Builder
	remaining := width
	for _, s := range segs {
		if remaining <= 0 {
			break
		}
		text := truncateWidth(s.text, remaining)
		if text == "" {
			continue
		}
		remaining -= lipgloss.Width(text)
		b.WriteString(s.style.Render(text))
	}
	if remaining > 0 {
		b.WriteString(strings.Repeat(" ", remaining))
	}
	return b.String()
}

// truncateWidth clips plain text to at most w display cells.
func truncateWidth(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if used+rw > w {
			break
		}
		b.WriteRune(r)
		used += rw
	}
	return b.String()
}

// truncateEllipsis clips plain text to at most w cells, marking the cut with
// the active ellipsis glyph.
func truncateEllipsis(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	ell := theme.G.Ellipsis
	keep := w - lipgloss.Width(ell)
	if keep < 0 {
		return truncateWidth(s, w)
	}
	return truncateWidth(s, keep) + ell
}
