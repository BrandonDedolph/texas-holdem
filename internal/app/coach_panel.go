package app

import (
	"strconv"
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/coach"
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

	// Price is the hero's own price while facing a bet ("to call 10 ·
	// 1.5:1 (40%)"). Only the compact layout renders it — the full and wide
	// layouts carry the same numbers in the action bar's info cell, but the
	// 60-col bar has no room for them, and the price is the one number a
	// player must never lose (ui-review F6).
	Price string
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

// coachStripRows is the 80-column strip's fixed height: one headline row
// plus two reasoning rows. The two-row strip cut every explanation at ~70
// characters (ui-review F1); two body rows fit nearly every rationale in
// the corpus, and anything longer gets the [e more] marker instead of an
// invisible truncation.
const coachStripRows = 3

// expandMarker names the key that opens the full-explanation overlay. It is
// appended only when the strip really did clip the reasoning (§5.1).
const expandMarker = "[e more]"

// coachIndent aligns the reasoning rows under the headline ("COACH  " is
// seven cells behind a one-space margin).
const coachIndent = "        "

// renderCoachStrip renders the 80-column coach region: exactly
// coachStripRows rows, each exactly width cells, in every mode. The strip
// is reserved even at CoachOff — it shows the neutral street summary then —
// because toggling the coach must never reflow the table (§5.1).
//
// Verbosity contract (§5.4): Full shows opinion + reasoning; Mistakes shows
// the numbers with no opinion (pot odds are the scoreboard, not coaching)
// plus the grade only when the last decision missed; Off shows the neutral
// summary only.
func renderCoachStrip(v CoachView, mode CoachMode, neutral string, width int) string {
	th := theme.Current
	inner := width - len(coachIndent)

	var line1 string
	rest := make([]string, 0, coachStripRows-1)
	switch mode {
	case CoachOff:
		line1 = " " + th.Help.Render(clip(neutral, width-1))
	case CoachMistakes:
		line1 = " " + th.CoachTitle.Render("COACH") + "  " +
			th.Body.Render(clip(v.chipsText(), inner))
		if v.Grade != nil && !v.Grade.Good {
			rest = gradeLines(*v.Grade, inner, coachStripRows-1)
		}
	default: // CoachFull
		head := v.Headline
		if head == "" {
			head = v.chipsText()
		}
		if head == "" {
			// Nothing of our own to headline (between hands): show the
			// neutral summary — which carries the hand verdict then — and
			// keep any lingering grade on the reasoning rows.
			line1 = " " + th.Help.Render(clip(neutral, width-1))
			if v.Grade != nil {
				rest = gradeLines(*v.Grade, inner, coachStripRows-1)
			}
			break
		}
		line1 = " " + th.CoachTitle.Render("COACH") + "  " +
			th.Body.Render(clip(head, inner))
		switch {
		case v.Grade != nil:
			rest = gradeLines(*v.Grade, inner, coachStripRows-1)
		case v.Body != "":
			rest = bodyLines(v.Body, inner, coachStripRows-1)
		case v.chipsText() != "":
			rest = []string{coachIndent + th.Body.Render(clip(v.chipsText(), inner))}
		}
	}

	lines := append([]string{line1}, rest...)
	for len(lines) < coachStripRows {
		lines = append(lines, "")
	}
	lines = lines[:coachStripRows]
	for i := range lines {
		lines[i] = padStyledTo(lines[i], width)
	}
	return strings.Join(lines, "\n")
}

// bodyLines wraps the coach's reasoning into at most max indented rows.
// When the text genuinely doesn't fit, the last row ends in the visible
// [e more] marker — an invisible truncation is the worst of both worlds,
// because the learner doesn't even know there was more (ui-review F1).
func bodyLines(body string, inner, max int) []string {
	th := theme.Current
	wrapped := wrapPlain(body, inner)
	if len(wrapped) <= max {
		out := make([]string, len(wrapped))
		for i, l := range wrapped {
			out[i] = coachIndent + th.Body.Render(l)
		}
		return out
	}
	out := make([]string, max)
	for i := 0; i < max-1; i++ {
		out[i] = coachIndent + th.Body.Render(wrapped[i])
	}
	last := clip(wrapped[max-1], inner-len(expandMarker)-1)
	out[max-1] = coachIndent + th.Body.Render(last) + " " + th.Help.Render(expandMarker)
	return out
}

// gradeLines wraps a grade's feedback into at most max indented rows in the
// grade's good/bad style.
func gradeLines(g GradeView, inner, max int) []string {
	style := theme.Current.GradeBad
	if g.Good {
		style = theme.Current.GradeGood
	}
	wrapped := wrapPlain(g.Symbol+" "+g.Text, inner)
	if len(wrapped) > max {
		wrapped = wrapped[:max]
		wrapped[max-1] = clip(wrapped[max-1]+" "+theme.G.Ellipsis, inner)
	}
	out := make([]string, len(wrapped))
	for i, l := range wrapped {
		out[i] = coachIndent + style.Render(l)
	}
	return out
}

// renderCoachLine is the compact layout's single coach row (§2.7). It is
// not simply the strip's first line: with only one row, the grade of the
// hero's last action and the price of the current decision both have to
// live here or nowhere (ui-review F2, F6).
func renderCoachLine(v CoachView, mode CoachMode, neutral string, width int) string {
	th := theme.Current
	prefix := " " + th.CoachTitle.Render("COACH") + "  "
	inner := width - len(coachIndent)
	dot := " " + theme.G.Dot + " "

	switch mode {
	case CoachOff:
		text := neutral
		if v.Price != "" {
			// The price is the scoreboard, not coaching (§5.4) — it stays
			// visible even when the coach is silent.
			text += dot + v.Price
		}
		return " " + th.Help.Render(clip(text, width-1))
	case CoachMistakes:
		if v.Grade != nil && !v.Grade.Good {
			return prefix + gradeText(*v.Grade, inner)
		}
		return prefix + th.Body.Render(clip(v.chipsText(), inner))
	default: // CoachFull
		if v.Grade != nil {
			return prefix + gradeText(*v.Grade, inner)
		}
		head := v.Headline
		if head != "" && v.Price != "" {
			head += dot + v.Price
		}
		if head == "" {
			// The chip fallback already carries the to-call numbers, so the
			// price is never appended twice.
			head = v.chipsText()
		}
		if head == "" {
			return renderCoachLine(v, CoachOff, neutral, width)
		}
		return prefix + th.Body.Render(clip(head, inner))
	}
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
		if len(lines) == 0 {
			// Nothing of the coach's own to show (between hands): the
			// neutral line carries the hand verdict then, same as the strip.
			add(neutral, help)
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

// gradeSummary is the terse one-line form of a frozen grade for the hero's
// reserved action line: "✔ Best", or for a leak the verdict with its
// counterfactual — "✗ Mistake · Fold was the play (-1.2bb)". The full
// sentence stays on the coach strip; row 15 gets the durable headline.
func gradeSummary(g coach.GradedDecision) string {
	s := gradeSymbol(g.Grade) + " " + g.Grade.Label()
	if g.Grade.GoodOrBetter() {
		return s
	}
	s += " " + theme.G.Dot + " " + capitalize(actionPhrase(g.Recommended)) + " was the play"
	if g.EVLossBB >= 0.05 {
		s += " (-" + strconv.FormatFloat(g.EVLossBB, 'f', 1, 64) + "bb)"
	}
	return s
}

// capitalize upper-cases the first letter of a display phrase ("raise to
// 25" → "Raise to 25"), matching the coach's own counterfactual casing.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
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
