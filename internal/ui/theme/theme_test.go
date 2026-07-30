package theme

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestDefaultTheme(t *testing.T) {
	th := Default()
	if th == nil {
		t.Fatal("Default() returned nil")
	}
	if Current == nil {
		t.Fatal("Current is nil")
	}
	if !th.SeatToAct.GetBold() {
		t.Error("SeatToAct should be bold: the turn marker must pop")
	}
	if !th.SeatAllIn.GetBold() {
		t.Error("SeatAllIn should be bold")
	}
	if !th.SeatFolded.GetFaint() {
		t.Error("SeatFolded should be faint: folded hands are gone, not hidden")
	}
	if !th.CoachBox.GetBorderTop() {
		t.Error("CoachBox should carry a border for the side-panel layout")
	}
	if !th.RingDigit.GetBold() {
		t.Error("RingDigit should be bold: it is the rim's one saturated ink")
	}
	if !th.SeatRead.GetItalic() {
		t.Error("SeatRead should be italic: reads are whispered, not stated")
	}
}

// TestSemanticActionStyles pins the action-color contract: every verb has
// a distinct meaning-carrying ink, and the button form is the filled
// (reverse-video) treatment of the same ink, so the pair always agrees.
func TestSemanticActionStyles(t *testing.T) {
	th := Default()
	inks := map[string]lipgloss.Style{
		"fold":   th.ActionFold,
		"check":  th.ActionCheck,
		"call":   th.ActionCall,
		"raise":  th.ActionRaise,
		"all-in": th.ActionAllIn,
	}
	buttons := map[string]lipgloss.Style{
		"fold":   th.ButtonFold,
		"check":  th.ButtonCheck,
		"call":   th.ButtonCall,
		"raise":  th.ButtonRaise,
		"all-in": th.ButtonAllIn,
	}
	seen := map[string]string{}
	for verb, ink := range inks {
		fg, ok := ink.GetForeground().(lipgloss.AdaptiveColor)
		if !ok {
			t.Fatalf("%s ink should be an adaptive color pair", verb)
		}
		if fg.Light == "" || fg.Dark == "" {
			t.Errorf("%s ink must define both light and dark forms", verb)
		}
		if fg.Light == fg.Dark {
			t.Errorf("%s ink should be tuned per background, got one hue %q", verb, fg.Light)
		}
		key := fg.Light + "/" + fg.Dark
		if prev, dup := seen[key]; dup {
			t.Errorf("%s and %s share an ink: the verbs must be distinguishable", verb, prev)
		}
		seen[key] = verb

		btn := buttons[verb]
		if !btn.GetReverse() {
			t.Errorf("%s button should be reverse video: the filled treatment must adapt to any background", verb)
		}
		if btn.GetForeground() != ink.GetForeground() {
			t.Errorf("%s button and ink disagree on color", verb)
		}
	}
}
