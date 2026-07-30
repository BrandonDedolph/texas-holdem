package components

import (
	"strconv"
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// PotView is the view model for one pot. The app layer builds the slice from
// the engine's derived pots; the first entry is the main pot.
type PotView struct {
	Amount engine.Chips
	// Eligible names the seats contesting this pot. It is rendered for
	// side pots only - side pots are exactly the kind of rule beginners
	// find opaque, so the display does the bookkeeping in the open.
	Eligible []string
}

// PotLine renders the pot region as a single row, centered in exactly width
// cells. One pot renders as "POT 185"; with side pots it switches to the
// multi-pot form:
//
//	MAIN 900   .   SIDE 710 (Nia . you)
//
// The row is always exactly width cells - blank when there are no pots, and
// truncated with an ellipsis when the eligible lists outgrow the width - so
// the region never resizes.
func PotLine(pots []PotView, width int) string {
	if width <= 0 {
		return ""
	}
	if len(pots) == 0 {
		return strings.Repeat(" ", width)
	}

	g := theme.G
	th := theme.Current

	var segs []seg
	if len(pots) == 1 {
		segs = []seg{{"POT " + pots[0].Amount.String(), th.PotLine}}
	} else {
		segs = []seg{{"MAIN " + pots[0].Amount.String(), th.PotLine}}
		for i, p := range pots[1:] {
			label := "SIDE"
			if i > 0 {
				label += " " + strconv.Itoa(i+1)
			}
			segs = append(segs,
				seg{"   " + g.Dot + "   ", th.Rule},
				seg{label + " " + p.Amount.String(), th.SidePotLine},
			)
			if len(p.Eligible) > 0 {
				names := strings.Join(p.Eligible, " "+g.Dot+" ")
				segs = append(segs, seg{" (" + names + ")", th.SidePotLine})
			}
		}
	}

	total := 0
	for _, s := range segs {
		total += lipgloss.Width(s.text)
	}
	if total > width {
		// Overflow: clip and mark the cut. renderRow consumes the whole
		// budget before padding, so the ellipsis lands at the far edge.
		row := renderRow(width-lipgloss.Width(g.Ellipsis), segs...)
		return row + th.Rule.Render(g.Ellipsis)
	}

	left := (width - total) / 2
	right := width - total - left
	return strings.Repeat(" ", left) + renderRow(total, segs...) + strings.Repeat(" ", right)
}
