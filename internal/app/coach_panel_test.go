package app

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// Coach strip tests (ui-review F1): at the 80-column target the strip must
// deliver the coach's whole reasoning for realistic explanations, and must
// say so visibly when it genuinely cannot.

// stripLines renders the strip at width and returns its ANSI-stripped,
// right-trimmed lines.
func stripLines(t *testing.T, v CoachView, mode CoachMode, width int) []string {
	t.Helper()
	out := strings.Split(renderCoachStrip(v, mode, "neutral", width), "\n")
	if len(out) != coachStripRows {
		t.Fatalf("strip is %d rows, want the fixed budget of %d", len(out), coachStripRows)
	}
	for i, l := range out {
		if got := lipgloss.Width(l); got != width {
			t.Errorf("strip row %d is %d cells, want exactly %d", i+1, got, width)
		}
		out[i] = strings.TrimRight(stripANSI(l), " ")
	}
	return out
}

// joined flattens the reasoning rows back into one string for whole-text
// containment checks.
func joined(lines []string) string {
	parts := make([]string, 0, len(lines))
	for _, l := range lines {
		parts = append(parts, strings.TrimSpace(l))
	}
	return strings.Join(parts, " ")
}

// TestCoachStripFitsRealisticExplanationAt80 is the F1 regression: the
// review's captured fold rationale — the arithmetic, the range read, the
// lesson — must be readable in full at the target size, with no ellipsis
// and no marker.
func TestCoachStripFitsRealisticExplanationAt80(t *testing.T) {
	body := "You need 33% to call (85 into 170) and have about 17% against his " +
		"likely range. Bad price for a weak draw - fold and keep the 85."
	v := CoachView{Headline: "Fold", Body: body}

	lines := stripLines(t, v, CoachFull, 80)
	all := joined(lines)
	for _, want := range []string{
		"COACH  Fold",
		"You need 33% to call (85 into 170)",
		"fold and keep the 85.",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("strip lost %q:\n%s", want, strings.Join(lines, "\n"))
		}
	}
	if strings.Contains(all, theme.G.Ellipsis) || strings.Contains(all, expandMarker) {
		t.Errorf("nothing should be truncated for this body:\n%s", all)
	}
}

// TestCoachStripMarksRealTruncation: when the reasoning genuinely exceeds
// the two body rows, the last row must end in the visible [e more] marker —
// an invisible truncation hides that there was more to learn.
func TestCoachStripMarksRealTruncation(t *testing.T) {
	v := CoachView{
		Headline: "Call 50",
		Body: strings.TrimSpace(strings.Repeat("every idea gets one sentence and this "+
			"explanation has far too many of them. ", 4)),
	}
	lines := stripLines(t, v, CoachFull, 80)
	if !strings.HasSuffix(lines[coachStripRows-1], expandMarker) {
		t.Errorf("overlong body must end in %q:\n%s", expandMarker, strings.Join(lines, "\n"))
	}
}

// TestCoachStripModesKeepTheirPromises: Mistakes shows numbers but never
// the opinion before acting; Off shows the neutral line only.
func TestCoachStripModesKeepTheirPromises(t *testing.T) {
	v := CoachView{
		Headline: "Raise to 25",
		Body:     "77 is in the BTN open chart.",
		Chips:    []NumberChip{{"pot odds", "1.5:1 (40%)"}},
	}
	mist := joined(stripLines(t, v, CoachMistakes, 80))
	if strings.Contains(mist, "Raise to 25") || strings.Contains(mist, "open chart") {
		t.Errorf("Mistakes mode leaked the opinion: %s", mist)
	}
	if !strings.Contains(mist, "1.5:1 (40%)") {
		t.Errorf("Mistakes mode must keep the numbers: %s", mist)
	}
	off := joined(stripLines(t, v, CoachOff, 80))
	if strings.Contains(off, "Raise") || strings.Contains(off, "1.5:1") {
		t.Errorf("Off mode must show the neutral line only: %s", off)
	}
	if !strings.Contains(off, "neutral") {
		t.Errorf("Off mode lost the neutral line: %s", off)
	}
}

// TestCompactCoachLineCarriesPriceAndGrade: the one-row compact line owns
// both jobs the taller layouts split across regions (ui-review F2, F6).
func TestCompactCoachLineCarriesPriceAndGrade(t *testing.T) {
	price := "to call 10 " + theme.G.Dot + " 1.5:1 (40%)"
	v := CoachView{Headline: "Fold", Price: price}
	line := stripANSI(renderCoachLine(v, CoachFull, "neutral", 60))
	if !strings.Contains(line, "Fold") || !strings.Contains(line, "1.5:1 (40%)") {
		t.Errorf("compact line must carry headline and price: %q", line)
	}

	// CoachOff keeps the price too — it is the scoreboard, not coaching.
	line = stripANSI(renderCoachLine(v, CoachOff, "preflop", 60))
	if !strings.Contains(line, "1.5:1 (40%)") {
		t.Errorf("compact Off line must keep the price: %q", line)
	}

	g := CoachView{Grade: &GradeView{Symbol: theme.G.Bad, Text: "A mistake."}}
	line = stripANSI(renderCoachLine(g, CoachFull, "neutral", 60))
	if !strings.Contains(line, theme.G.Bad) || !strings.Contains(line, "A mistake.") {
		t.Errorf("compact line must show the grade after acting: %q", line)
	}
}
