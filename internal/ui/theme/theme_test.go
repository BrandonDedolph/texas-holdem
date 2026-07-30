package theme

import "testing"

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
}
