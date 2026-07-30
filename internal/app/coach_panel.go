package app

import (
	"github.com/BrandonDedolph/texas-holdem/internal/coach"
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
)

// The coach panel (docs/design-tui.md §5). This file renders a view model
// the TUI defines for itself — it does NOT import a coach package.
// internal/coach is being built in parallel; when it lands, its Advice /
// GradedDecision map onto CoachView here and nothing below this comment
// changes. TODO(wire-coach).

// NumberChip is one labelled number the coach panel shows ("Pot odds",
// "2.2:1 (31%)"). Numbers are pre-formatted upstream: the panel renders,
// it never computes.
type NumberChip struct {
	Label, Value string
}

// GradeView is the rendered form of a decision grade. Symbol is a short
// marker ("A", "C"); Text explains ("matched the coach line"); Good selects
// the grade style.
type GradeView struct {
	Symbol string
	Text   string
	Good   bool
}

// gradeSymbol is the one-character marker for a grade band. Display only —
// the coach package names bands ("inaccuracy"), and how they are drawn is
// the UI's business.
func gradeSymbol(g coach.Grade) string {
	switch {
	case g.GoodOrBetter():
		return theme.G.Good
	case g == coach.GradeInaccuracy:
		return theme.G.Query
	default:
		return theme.G.Bad
	}
}

// CoachView is everything the coach region can show for the current
// decision point. Empty fields render blank — the region itself never
// disappears (the layout-stability invariant).
type CoachView struct {
	Headline string       // one-line recommendation ("3-bet to ~90")
	Body     string       // reasoning sentence(s)
	Chips    []NumberChip // the numbers: pot odds, equity, outs
	Grade    *GradeView   // grade of the hero's last action, if any
}

// chipsText joins the number chips into one line: "pot odds 2.2:1 (31%) · to call 20".
func (v CoachView) chipsText() string {
	parts := make([]string, 0, len(v.Chips))
	for _, c := range v.Chips {
		parts = append(parts, c.Label+" "+c.Value)
	}
	return strings.Join(parts, " "+theme.G.Dot+" ")
}

// coachPanelWidth is the wide layout's side panel width (§2.6).
const coachPanelWidth = 28

// renderCoachStrip renders the 80-column coach region: exactly two rows,
// each exactly width cells, in every mode. The strip is reserved even at
// CoachOff — it shows the neutral street summary then — because toggling
// the coach must never reflow the table (§5.1).
//
// Verbosity contract (§5.4): Full shows opinion + reasoning; Mistakes shows
// the numbers with no opinion (pot odds are the scoreboard, not coaching)
// plus the grade only when the last decision missed; Off shows the neutral
// summary only.
func renderCoachStrip(v CoachView, mode CoachMode, neutral string, width int) string {
	th := theme.Current

	var line1, line2 string
	switch mode {
	case CoachOff:
		line1 = " " + th.Help.Render(clip(neutral, width-1))
	case CoachMistakes:
		line1 = " " + th.CoachTitle.Render("COACH") + "  " +
			th.Body.Render(clip(v.chipsText(), width-8))
		if v.Grade != nil && !v.Grade.Good {
			line2 = "        " + gradeText(*v.Grade, width-8)
		}
	default: // CoachFull
		head := v.Headline
		if head == "" {
			head = v.chipsText()
		}
		if head == "" {
			// Nothing to say (between hands): fall back to the neutral
			// summary rather than a bare COACH label.
			return renderCoachStrip(v, CoachOff, neutral, width)
		}
		line1 = " " + th.CoachTitle.Render("COACH") + "  " +
			th.Body.Render(clip(head, width-8))
		switch {
		case v.Grade != nil:
			line2 = "        " + gradeText(*v.Grade, width-8)
		case v.Body != "":
			line2 = "        " + th.Body.Render(clip(v.Body, width-8))
		case v.Headline != "" && v.chipsText() != "":
			line2 = "        " + th.Body.Render(clip(v.chipsText(), width-8))
		}
	}
	return padStyledTo(line1, width) + "\n" + padStyledTo(line2, width)
}

// renderCoachLine is the compact layout's single coach row (§2.7): the
// strip's first line only.
func renderCoachLine(v CoachView, mode CoachMode, neutral string, width int) string {
	strip := renderCoachStrip(v, mode, neutral, width)
	return strings.SplitN(strip, "\n", 2)[0]
}

// renderCoachPanel renders the wide layout's bordered side panel: exactly
// height rows, coachPanelWidth cells wide, with room for the recommendation,
// reasoning, numbers and grade simultaneously (§2.6).
func renderCoachPanel(v CoachView, mode CoachMode, neutral string, height int) string {
	th := theme.Current
	inner := coachPanelWidth - 4 // border + 1 cell padding each side

	var lines []string
	add := func(s string, style func(string) string) {
		for _, l := range wrapPlain(s, inner) {
			lines = append(lines, style(l))
		}
	}
	body := func(s string) string { return th.Body.Render(s) }
	help := func(s string) string { return th.Help.Render(s) }

	switch mode {
	case CoachOff:
		add(neutral, help)
	case CoachMistakes:
		for _, c := range v.Chips {
			add(c.Label+"  "+c.Value, body)
		}
		if v.Grade != nil && !v.Grade.Good {
			lines = append(lines, "")
			add(v.Grade.Symbol+" "+v.Grade.Text, body)
		}
	default: // CoachFull
		if v.Headline != "" {
			add(v.Headline, body)
			lines = append(lines, "")
		}
		if v.Body != "" {
			add(v.Body, body)
			lines = append(lines, "")
		}
		for _, c := range v.Chips {
			add(c.Label+"  "+c.Value, body)
		}
		if v.Grade != nil {
			lines = append(lines, "")
			add(v.Grade.Symbol+" "+v.Grade.Text, body)
		}
	}

	// Fix the interior height so the panel's bottom border sits at the same
	// row no matter how chatty the coach is.
	all := append([]string{th.CoachTitle.Render("COACH"), ""}, lines...)
	interior := height - 2
	if interior < 1 {
		interior = 1
	}
	for len(all) < interior {
		all = append(all, "")
	}
	all = all[:interior]
	for i, l := range all {
		all[i] = padStyledTo(l, inner)
	}
	return th.CoachBox.Render(strings.Join(all, "\n"))
}

// gradeText renders a grade in its good/bad style, truncated to fit.
func gradeText(g GradeView, width int) string {
	style := theme.Current.GradeBad
	if g.Good {
		style = theme.Current.GradeGood
	}
	return style.Render(clip(g.Symbol+" "+g.Text, width))
}

// wrapPlain greedy-wraps plain text into lines of at most width cells.
// Styling is applied per line by the caller, never across a wrap point.
func wrapPlain(s string, width int) []string {
	if s == "" {
		return []string{""}
	}
	var lines []string
	line := ""
	for _, word := range strings.Fields(s) {
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	for i, l := range lines {
		if len(l) > width {
			lines[i] = clip(l, width)
		}
	}
	return lines
}
