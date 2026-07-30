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

// TestCoachStripModesKeepTheirPromises: Mistakes is genuinely SILENT before
// the hero acts — no opinion, and no numbers either, because the strip's
// pot-odds line duplicated the action bar's info cell two rows lower in the
// same frame (ui-review §5 q5). Off shows the neutral line only.
func TestCoachStripModesKeepTheirPromises(t *testing.T) {
	v := CoachView{
		Headline: "Raise to 25",
		Body:     "77 is in the BTN open chart.",
		Chips:    []NumberChip{{"pot odds", "1.5:1 (40%)"}},
	}
	mistLines := stripLines(t, v, CoachMistakes, 80)
	for i, l := range mistLines {
		if l != "" {
			t.Errorf("Mistakes row %d must be silent before the hero errs, got %q", i+1, l)
		}
	}
	off := joined(stripLines(t, v, CoachOff, 80))
	if strings.Contains(off, "Raise") || strings.Contains(off, "1.5:1") {
		t.Errorf("Off mode must show the neutral line only: %s", off)
	}
	if !strings.Contains(off, "neutral") {
		t.Errorf("Off mode lost the neutral line: %s", off)
	}
}

// TestCoachStripMistakesSpeaksOnlyOnError: the silence is the signal — the
// strip stays blank after a good decision and lights up with the grade after
// a bad one. Grades exist in both cases; display is the only difference.
func TestCoachStripMistakesSpeaksOnlyOnError(t *testing.T) {
	good := CoachView{Grade: &GradeView{Symbol: theme.G.Good, Text: "Matched the coach.", Good: true}}
	for i, l := range stripLines(t, good, CoachMistakes, 80) {
		if l != "" {
			t.Errorf("Mistakes row %d must stay silent after a good decision, got %q", i+1, l)
		}
	}
	bad := CoachView{Grade: &GradeView{Symbol: theme.G.Bad, Text: "Fold was the play.", Good: false}}
	all := joined(stripLines(t, bad, CoachMistakes, 80))
	if !strings.Contains(all, "COACH") || !strings.Contains(all, "Fold was the play.") {
		t.Errorf("Mistakes mode must show the grade after an error: %s", all)
	}
}

// TestCoachStripSessionScoreboardBetweenHands (ui-review §5 q3): between
// hands the strip carries the running session pair — decision accuracy AND
// net chips, as two separately labelled figures that are free to disagree.
func TestCoachStripSessionScoreboardBetweenHands(t *testing.T) {
	v := CoachView{
		BetweenHands: true,
		Verdict:      "hand over " + theme.G.Dot + " 1 Mistake (-1.2bb) " + theme.G.Dot + " v review",
		Session: []NumberChip{
			{"hands", "12"},
			{"decisions", "84% good (21/25)"},
			{"net", "-140"},
		},
	}
	for _, mode := range []CoachMode{CoachFull, CoachMistakes, CoachOff} {
		all := joined(stripLines(t, v, mode, 80))
		// Accuracy up, chips down — both figures must survive, separately
		// labelled, in every mode: the scoreboard is the player's, not the
		// coach's.
		if !strings.Contains(all, "decisions 84% good (21/25)") {
			t.Errorf("%v: session accuracy missing between hands: %s", mode, all)
		}
		if !strings.Contains(all, "net -140") {
			t.Errorf("%v: session net chips missing between hands: %s", mode, all)
		}
	}
	// During play the session line stays out of the strip — the rows belong
	// to the live decision.
	live := v
	live.BetweenHands = false
	live.Verdict = ""
	if all := joined(stripLines(t, live, CoachFull, 80)); strings.Contains(all, "net -140") {
		t.Errorf("session line must not render mid-hand: %s", all)
	}
}

// panelText renders the wide side panel and returns it ANSI-stripped.
func panelText(v CoachView, mode CoachMode, height int) string {
	return stripANSI(renderCoachPanel(v, mode, "neutral", height))
}

// TestCoachPanelShowsFullBodyWithoutOverlay (ui-review §5 q1): a body long
// enough to earn the [e more] clip on the 80-col strip must appear in the
// wide panel whole — wide terminals get the full reasoning with no overlay.
func TestCoachPanelShowsFullBodyWithoutOverlay(t *testing.T) {
	body := "You need 33% to call (85 into 170) and have about 17% against his " +
		"likely range. Bad price for a weak draw - fold and keep the 85. " +
		"Position will not save a hand this far behind the range."
	v := CoachView{Headline: "Fold", Body: body}

	// Confirm the premise: this body does overflow the strip at 80 cols.
	if !strings.Contains(joined(stripLines(t, v, CoachFull, 80)), expandMarker) {
		t.Fatal("premise: this body should overflow the 80-col strip")
	}

	panel := panelText(v, CoachFull, 30)
	for _, word := range strings.Fields(body) {
		if !strings.Contains(panel, word) {
			t.Errorf("wide panel lost %q of the reasoning:\n%s", word, panel)
		}
	}
	if strings.Contains(panel, expandMarker) || strings.Contains(panel, theme.G.Ellipsis) {
		t.Errorf("wide panel must never clip the reasoning:\n%s", panel)
	}
}

// TestCoachPanelSessionBlockPinnedInEveryMode: the wide panel carries the
// session scoreboard permanently — even at Mistakes (silent about the
// decision) and Off, because it is the player's ledger, not advice.
func TestCoachPanelSessionBlockPinnedInEveryMode(t *testing.T) {
	v := CoachView{
		Session: []NumberChip{
			{"hands", "12"},
			{"decisions", "84% good (21/25)"},
			{"net", "+140"},
		},
	}
	for _, mode := range []CoachMode{CoachFull, CoachMistakes, CoachOff} {
		panel := panelText(v, mode, 30)
		for _, want := range []string{"SESSION", "hands 12", "84% good", "net +140"} {
			if !strings.Contains(panel, want) {
				t.Errorf("%v: wide panel session block lost %q:\n%s", mode, want, panel)
			}
		}
	}
	// Mistakes stays silent about the decision itself: advice fields must
	// not leak around the session block.
	loud := v
	loud.Headline = "Raise to 25"
	loud.Chips = []NumberChip{{"pot odds", "1.5:1 (40%)"}}
	panel := panelText(loud, CoachMistakes, 30)
	if strings.Contains(panel, "Raise to 25") || strings.Contains(panel, "1.5:1") {
		t.Errorf("Mistakes panel leaked the pre-decision opinion or numbers:\n%s", panel)
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
