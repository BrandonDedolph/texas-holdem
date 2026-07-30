package components

import (
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// DefaultPlateWidth is the side-plate slot width from the Direction E
// mockups (docs/table-redesign-pitch-2.md): a fused border cell, twenty
// interior cells, a closing corner.
const DefaultPlateWidth = 22

// maxReadWidth caps the one-word archetype read so a hostile read can never
// squeeze the name out of the title row.
const maxReadWidth = 8

// PlateHand selects which rim of the hexagon a side plate hangs from. It
// controls justification only: the stack sits on the ring side, the chips
// and status sit on the felt side (chips lie toward the pot), and the
// title's spare border fill runs toward the felt.
type PlateHand int

// The two rim variants.
const (
	PlateLeftHand PlateHand = iota
	PlateRightHand
)

// PlateEdges marks which of the plate's four borders render open - drawn
// with fusion junctions so the layout can weld the plate into a ring line
// running along that edge - instead of closed with rounded corners.
//
// Left/Right open turns that edge's corners into tees (the mockup's
// "+ ... | ... +" left rim, ASCII form). Top/Bottom open renders that
// border row in its inset-into-a-horizontal-edge form, "+ title +", the
// shape the top and bottom seats use when their plate sits inside the
// ring's edge.
type PlateEdges struct {
	Top, Bottom, Left, Right bool
}

// SeatPlateView is the view model for one hexagon seat plate. The app
// layer maps engine state onto it; components never read engine.Hand
// directly.
type SeatPlateView struct {
	Order    string // this street's action-order digit; "" hides it
	Name     string
	Position engine.Position
	Read     string // one-word archetype read ("tight"); "" hides it
	Stack    engine.Chips

	Bet    engine.Chips // chips committed this street; 0 renders nothing
	Raised bool         // prefix the raise marker to the bet

	ToAct  bool // turn marker follows the order digit
	Folded bool // fold mark replaces the digit; the plate dims
	AllIn  bool // ALL-IN replaces the stack amount
	Hero   bool // accent the name; inline form shows the stack
	Dealer bool // show the dealer disc after the position badge

	Width int // slot width in cells; 0 means DefaultPlateWidth
}

// Render renders the three-row bordered plate, every row exactly the slot
// width. The title is engraved in the top border, the body row carries the
// stack on the ring side and the chips/status on the felt side, and the
// base row closes the frame (ASCII forms; the Unicode set draws rounded
// corners, tees and box lines):
//
//	left hand:  + 6 Nia BB . sticky -+
//	            | 990          o 10  |
//	            +--------------------+
//
//	right hand: +- 2 Ivy HJ . wild --+
//	            | ^ o 30         970 |
//	            +--------------------+
//
// Rows never wrap: the name (then the read) truncates instead, so the
// plate can never exceed its slot.
func (v SeatPlateView) Render(hand PlateHand, open PlateEdges) string {
	w := v.width()
	rows := []string{
		v.renderTitleRow(w, hand, open),
		v.renderBodyRow(w, hand),
		v.renderBaseRow(w, open),
	}
	return strings.Join(rows, "\n")
}

// RenderInline renders the one-row form for a plate inset into the top or
// bottom horizontal edge of the ring, exactly the slot width (ASCII form):
//
//	villain: + 1 Cole UTG . tight +
//	hero:    + 4 -> YOU BTN (D) 1,000 +
//
// Villain plates carry the read; the hero plate carries the stack (its
// interior rows belong to the hand region, so the stack must live on the
// rim). The layout drops this row into a same-width gap in the ring edge.
func (v SeatPlateView) RenderInline() string {
	g := theme.G
	th := theme.Current
	w := v.width()

	interior := w - 2
	if interior < 0 {
		interior = 0
	}
	content, used := renderSegsClipped(interior, v.titleSegs(interior, v.Hero)...)
	left := (interior - used) / 2
	right := interior - used - left
	return renderRow(w,
		seg{g.RingTeeR, th.RingFrame},
		seg{text: strings.Repeat(" ", left)},
		seg{text: content, style: lipgloss.NewStyle()},
		seg{text: strings.Repeat(" ", right)},
		seg{g.RingTeeL, th.RingFrame},
	)
}

// RenderStatus renders the free-floating status row a top or bottom seat
// shows inside the ring beneath its inline plate, centered in exactly
// width cells (ASCII form):
//
//	1,000 x folded        970 ^ o 30
func (v SeatPlateView) RenderStatus(width int) string {
	segs := []seg{v.stackSeg()}
	if status := v.statusSegs(); len(status) > 0 {
		segs = append(segs, seg{text: " "})
		segs = append(segs, status...)
	}
	content, used := renderSegsClipped(width, segs...)
	left := (width - used) / 2
	right := width - used - left
	if left < 0 || right < 0 {
		left, right = 0, 0
	}
	return strings.Repeat(" ", left) + content + strings.Repeat(" ", right)
}

func (v SeatPlateView) width() int {
	if v.Width > 0 {
		return v.Width
	}
	return DefaultPlateWidth
}

// renderTitleRow engraves the title into the top border. Closed form:
// corner, title, horizontal fill, corner; the fill runs on the felt side
// so the title hugs the ring. Open form (fused into the ring's top edge):
// tees facing outward with the title centered between them.
func (v SeatPlateView) renderTitleRow(w int, hand PlateHand, open PlateEdges) string {
	g := theme.G
	th := theme.Current

	if open.Top {
		return v.RenderInline()
	}

	// One border cell each side plus one space padding each side of the
	// title text.
	budget := w - 4
	if budget < 0 {
		budget = 0
	}
	content, used := renderSegsClipped(budget, v.titleSegs(budget, false)...)
	fill := budget - used

	leftEdge := g.RingTL
	if open.Left {
		leftEdge = g.RingTeeL
	}
	rightEdge := g.RingTR
	if open.Right {
		rightEdge = g.RingTeeR
	}

	var b strings.Builder
	b.WriteString(th.RingFrame.Render(leftEdge))
	if hand == PlateLeftHand {
		b.WriteString(" ")
		b.WriteString(content)
		b.WriteString(" ")
		b.WriteString(th.RingFrame.Render(strings.Repeat(g.RingH, fill)))
	} else {
		b.WriteString(th.RingFrame.Render(strings.Repeat(g.RingH, fill)))
		b.WriteString(" ")
		b.WriteString(content)
		b.WriteString(" ")
	}
	b.WriteString(th.RingFrame.Render(rightEdge))
	return b.String()
}

// renderBodyRow renders the interior row: stack on the ring side, chips
// and status on the felt side, so committed chips always lie toward the
// pot.
func (v SeatPlateView) renderBodyRow(w int, hand PlateHand) string {
	g := theme.G
	th := theme.Current

	budget := w - 4
	if budget < 0 {
		budget = 0
	}

	stack, stackUsed := renderSegsClipped(budget, v.stackSeg())
	status, statusUsed := renderSegsClipped(budget-stackUsed-1, v.statusSegs()...)
	gap := budget - stackUsed - statusUsed
	if gap < 0 {
		gap = 0
	}
	spacer := strings.Repeat(" ", gap)

	var b strings.Builder
	b.WriteString(th.RingFrame.Render(g.RingV))
	b.WriteString(" ")
	if hand == PlateLeftHand {
		b.WriteString(stack)
		b.WriteString(spacer)
		b.WriteString(status)
	} else {
		b.WriteString(status)
		b.WriteString(spacer)
		b.WriteString(stack)
	}
	b.WriteString(" ")
	b.WriteString(th.RingFrame.Render(g.RingV))
	return b.String()
}

// renderBaseRow closes the frame. Open Left/Right fuse the corners into
// the ring verticals with tees; open Bottom renders the inset form for a
// ring edge running along the plate's underside.
func (v SeatPlateView) renderBaseRow(w int, open PlateEdges) string {
	g := theme.G
	th := theme.Current

	fill := w - 2
	if fill < 0 {
		fill = 0
	}

	leftEdge, rightEdge := g.RingBL, g.RingBR
	if open.Bottom {
		leftEdge, rightEdge = g.RingTeeR, g.RingTeeL
	} else {
		if open.Left {
			leftEdge = g.RingTeeL
		}
		if open.Right {
			rightEdge = g.RingTeeR
		}
	}
	return th.RingFrame.Render(leftEdge + strings.Repeat(g.RingH, fill) + rightEdge)
}

// titleSegs builds the engraved title: order digit (or fold mark), turn
// marker, name, position badge, dealer disc, then the read - or the stack
// in place of the read when showStack is set (the hero's inline form).
// The name is pre-truncated so the badge survives it; the read is capped
// so it can never squeeze the name out.
func (v SeatPlateView) titleSegs(budget int, showStack bool) []seg {
	g := theme.G
	th := theme.Current

	order := v.Order
	orderStyle := th.RingDigit
	if v.Folded {
		order, orderStyle = g.FoldMark, th.SeatFolded
	}

	marker := ""
	if v.ToAct {
		marker = g.ToAct
	}

	badge := theme.PositionBadge(v.Position)
	badgeStyle := th.PosBadge
	if v.Position == engine.PosBTN {
		badgeStyle = th.DealerBadge
	}

	dealer := ""
	if v.Dealer {
		dealer = g.Dealer
	}

	nameStyle := th.SeatName
	if v.Hero {
		nameStyle = th.HeroBadge
	}

	var tail []seg
	tailWidth := 0
	switch {
	case showStack:
		s := v.stackSeg()
		tail = []seg{{text: " "}, s}
		tailWidth = 1 + lipgloss.Width(s.text)
	case v.Read != "":
		read := truncateEllipsis(v.Read, maxReadWidth)
		tail = []seg{
			{text: " "},
			{g.Dot, th.Rule},
			{text: " "},
			{read, th.SeatRead},
		}
		tailWidth = 3 + lipgloss.Width(read)
	}

	if v.Folded {
		orderStyle, badgeStyle, nameStyle = th.SeatFolded, th.SeatFolded, th.SeatFolded
	}

	fixed := lipgloss.Width(marker) + 1 + lipgloss.Width(badge) +
		lipgloss.Width(dealer) + tailWidth
	if order != "" {
		fixed += lipgloss.Width(order) + 1
	}
	nameBudget := budget - fixed
	if nameBudget < 1 {
		nameBudget = 1
	}
	name := truncateEllipsis(v.Name, nameBudget)

	var segs []seg
	if order != "" {
		segs = append(segs, seg{order, orderStyle}, seg{text: " "})
	}
	if marker != "" {
		segs = append(segs, seg{marker, th.SeatToAct})
	}
	segs = append(segs,
		seg{name, nameStyle},
		seg{text: " "},
		seg{badge, badgeStyle},
	)
	if dealer != "" {
		segs = append(segs, seg{dealer, th.DealerBadge})
	}
	return append(segs, tail...)
}

// stackSeg renders the stack amount, or ALL-IN in its place.
func (v SeatPlateView) stackSeg() seg {
	th := theme.Current
	switch {
	case v.AllIn:
		return seg{"ALL-IN", th.SeatAllIn}
	case v.Folded:
		return seg{v.Stack.String(), th.SeatFolded}
	}
	return seg{v.Stack.String(), th.SeatStack}
}

// statusSegs renders the felt-side state: the fold word, or the chips
// committed this street with the raise marker when they arrived by raise.
func (v SeatPlateView) statusSegs() []seg {
	g := theme.G
	th := theme.Current

	if v.Folded {
		return []seg{{g.FoldMark + " folded", th.SeatFolded}}
	}
	if v.Bet <= 0 {
		return nil
	}
	var segs []seg
	if v.Raised {
		segs = append(segs, seg{g.RaiseMark, th.ActionRaise}, seg{text: " "})
	}
	return append(segs, seg{g.Chip + " " + v.Bet.String(), th.SeatBet})
}

// renderSegsClipped renders segments clipped to at most width display
// cells with no padding, and reports the width actually used - the
// primitive for rows whose spare cells are filled with something other
// than spaces (border fill, centering).
func renderSegsClipped(width int, segs ...seg) (string, int) {
	if width <= 0 {
		return "", 0
	}
	var b strings.Builder
	used := 0
	for _, s := range segs {
		if used >= width {
			break
		}
		text := truncateWidth(s.text, width-used)
		if text == "" {
			continue
		}
		used += lipgloss.Width(text)
		b.WriteString(s.style.Render(text))
	}
	return b.String(), used
}
