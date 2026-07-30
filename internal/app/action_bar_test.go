package app

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// Action-bar legality (docs/design-tui.md §8.2): for each LegalActions
// fixture, exactly the right keycaps render with correct amounts; presets
// match presetAmounts and respect the engine clamps; a typed out-of-range
// amount shows the clamp it will receive.

// barFixture is one legality scenario and what the bar must (not) show.
type barFixture struct {
	name    string
	legal   engine.ActionOptions
	show    []string
	hide    []string
	accepts []engine.ActionType
	rejects []engine.ActionType
}

func barFixtures() []barFixture {
	return []barFixture{
		{
			name: "facing a bet",
			legal: engine.ActionOptions{
				{Type: engine.ActionFold},
				{Type: engine.ActionCall, Min: 20, Max: 20},
				{Type: engine.ActionRaise, Min: 40, Max: 990},
			},
			show:    []string{"f FOLD", "c CALL 20", "r RAISE"},
			hide:    []string{"x CHECK", "b BET"},
			accepts: []engine.ActionType{engine.ActionFold, engine.ActionCall, engine.ActionRaise},
			rejects: []engine.ActionType{engine.ActionCheck, engine.ActionBet},
		},
		{
			name: "check or bet",
			legal: engine.ActionOptions{
				{Type: engine.ActionFold},
				{Type: engine.ActionCheck},
				{Type: engine.ActionBet, Min: 10, Max: 900},
			},
			// Fold is legal but hidden when checking is free: rendering a
			// free fold would teach that it is worth weighing (§4.1).
			show:    []string{"x CHECK", "b BET"},
			hide:    []string{"f FOLD", "c CALL", "r RAISE"},
			accepts: []engine.ActionType{engine.ActionCheck, engine.ActionBet},
			rejects: []engine.ActionType{engine.ActionFold, engine.ActionCall, engine.ActionRaise},
		},
		{
			name: "call only (raising rights closed)",
			legal: engine.ActionOptions{
				{Type: engine.ActionFold},
				{Type: engine.ActionCall, Min: 640, Max: 640},
			},
			show:    []string{"f FOLD", "c CALL 640"},
			hide:    []string{"x CHECK", "b BET", "r RAISE"},
			accepts: []engine.ActionType{engine.ActionFold, engine.ActionCall},
			rejects: []engine.ActionType{engine.ActionCheck, engine.ActionBet, engine.ActionRaise},
		},
		{
			name: "raise pinned to all-in",
			legal: engine.ActionOptions{
				{Type: engine.ActionFold},
				{Type: engine.ActionCall, Min: 100, Max: 100},
				{Type: engine.ActionRaise, Min: 285, Max: 285},
			},
			show:    []string{"f FOLD", "c CALL 100", "r ALL-IN 285"},
			hide:    []string{"x CHECK", "b BET", "r RAISE" + theme.G.Ellipsis},
			accepts: []engine.ActionType{engine.ActionFold, engine.ActionCall, engine.ActionRaise},
			rejects: []engine.ActionType{engine.ActionCheck, engine.ActionBet},
		},
	}
}

// armedBar builds a choosing-state bar over a fixture.
func armedBar(f barFixture) *ActionBar {
	b := NewActionBar()
	b.Arm(f.legal, engine.Flop, 185, f.legal.CallAmount(), 0, 10, "")
	return b
}

func TestActionBarRendersExactlyTheLegalKeycaps(t *testing.T) {
	for _, f := range barFixtures() {
		b := armedBar(f)
		view := stripANSI(b.View(80))
		for _, want := range f.show {
			if !strings.Contains(view, want) {
				t.Errorf("%s: bar missing %q:\n%s", f.name, want, view)
			}
		}
		for _, banned := range f.hide {
			if strings.Contains(view, banned) {
				t.Errorf("%s: bar must not render %q:\n%s", f.name, banned, view)
			}
		}
	}
}

func TestActionBarAcceptsExactlyTheRenderedActions(t *testing.T) {
	// The other half of the legend invariant: the dispatch filter is the
	// same visibility set the renderer used.
	for _, f := range barFixtures() {
		b := armedBar(f)
		for _, at := range f.accepts {
			if !b.Accepts(at) {
				t.Errorf("%s: bar renders %v but rejects the key", f.name, at)
			}
		}
		for _, at := range f.rejects {
			if b.Accepts(at) {
				t.Errorf("%s: bar accepts %v without rendering a keycap", f.name, at)
			}
		}
	}
}

func TestActionBarPresetsMatchArithmetic(t *testing.T) {
	// Mockup B's numbers: pot 185, no bet to face, betting 10..900. The
	// design doc calls every hand-written mockup number suspect until a test
	// computes it (DESIGN.md §4) — this is that test.
	b := NewActionBar()
	legal := engine.ActionOptions{
		{Type: engine.ActionFold},
		{Type: engine.ActionCheck},
		{Type: engine.ActionBet, Min: 10, Max: 900},
	}
	b.Arm(legal, engine.Flop, 185, 0, 0, 10, "")
	if _, skipped := b.OpenSizing(engine.ActionBet); skipped {
		t.Fatal("a real range must open sizing, not skip it")
	}
	want := [5]engine.Chips{62, 93, 123, 185, 900}
	if got := b.presetAmounts(); got != want {
		t.Fatalf("presetAmounts = %v, want %v", got, want)
	}
	view := stripANSI(b.View(80))
	for _, s := range []string{"1 1/3 62", "2 1/2 93", "3 2/3 123", "4 pot 185", "5 all-in"} {
		if !strings.Contains(view, s) {
			t.Errorf("sizing row must show resolved preset %q:\n%s", s, view)
		}
	}
}

func TestActionBarPresetsRespectEngineClamps(t *testing.T) {
	b := NewActionBar()
	legal := engine.ActionOptions{
		{Type: engine.ActionFold},
		{Type: engine.ActionCheck},
		{Type: engine.ActionBet, Min: 50, Max: 100}, // shallow stack
	}
	b.Arm(legal, engine.Flop, 185, 0, 0, 10, "")
	b.OpenSizing(engine.ActionBet)
	want := [5]engine.Chips{62, 93, 100, 100, 100}
	if got := b.presetAmounts(); got != want {
		t.Fatalf("clamped presets = %v, want %v", got, want)
	}
}

func TestActionBarRaisePresetsUsePotAfterCall(t *testing.T) {
	// Facing a bet of 100 with 40 already in: pot 300, to call 60. Presets
	// are fractions of the pot AFTER the call (360), on top of matching
	// (hero total 100): 1/2-pot raise-to = 100 + 180 = 280.
	b := NewActionBar()
	legal := engine.ActionOptions{
		{Type: engine.ActionFold},
		{Type: engine.ActionCall, Min: 60, Max: 60},
		{Type: engine.ActionRaise, Min: 200, Max: 1000},
	}
	b.Arm(legal, engine.Flop, 300, 60, 40, 10, "")
	b.OpenSizing(engine.ActionRaise)
	want := [5]engine.Chips{220, 280, 340, 460, 1000}
	if got := b.presetAmounts(); got != want {
		t.Fatalf("raise presets = %v, want %v", got, want)
	}
}

func TestActionBarTypedAmountShowsClamp(t *testing.T) {
	b := NewActionBar()
	legal := engine.ActionOptions{
		{Type: engine.ActionFold},
		{Type: engine.ActionCheck},
		{Type: engine.ActionBet, Min: 10, Max: 900},
	}
	b.Arm(legal, engine.Flop, 185, 0, 0, 10, "")
	b.OpenSizing(engine.ActionBet)

	// Under min: typed 7 (starts with a leading 0) shows the min clamp.
	b.TypeDigit('0')
	b.TypeDigit('7')
	if got := b.ConfirmAmount(); got != 10 {
		t.Errorf("under-min confirm = %v, want clamp to 10", got)
	}
	if view := stripANSI(b.View(80)); !strings.Contains(view, "min 10") {
		t.Errorf("under-min view must show the clamp:\n%s", view)
	}

	// Over max.
	b.typed = ""
	for _, d := range []byte("99999") {
		b.TypeDigit(d)
	}
	if got := b.ConfirmAmount(); got != 900 {
		t.Errorf("over-max confirm = %v, want clamp to 900", got)
	}
	if view := stripANSI(b.View(80)); !strings.Contains(view, "all-in 900") {
		t.Errorf("over-max view must show the clamp:\n%s", view)
	}
}

func TestActionBarDigitsVersusPresets(t *testing.T) {
	b := NewActionBar()
	legal := engine.ActionOptions{
		{Type: engine.ActionFold},
		{Type: engine.ActionCheck},
		{Type: engine.ActionBet, Min: 10, Max: 900},
	}
	b.Arm(legal, engine.Flop, 185, 0, 0, 10, "")
	b.OpenSizing(engine.ActionBet)

	// With nothing typed, 2 is the half-pot preset.
	b.Preset(1)
	if got := b.ConfirmAmount(); got != 93 {
		t.Fatalf("preset 2 = %v, want 93", got)
	}
	// Once typing has begun (6), further digits — including 1-5 — append.
	b.TypeDigit('6')
	b.Preset(1) // the "2" key mid-typing
	if got := b.ConfirmAmount(); got != 62 {
		t.Errorf("typed 6 then 2 = %v, want 62", got)
	}
	// Backspace restores digit by digit; empty falls back to the slider value.
	b.Backspace()
	b.Backspace()
	if got := b.ConfirmAmount(); got != 93 {
		t.Errorf("after backspaces = %v, want the prior amount 93", got)
	}
}

func TestActionBarNudgeIsClampedAndUnitBB(t *testing.T) {
	b := NewActionBar()
	legal := engine.ActionOptions{
		{Type: engine.ActionFold},
		{Type: engine.ActionCheck},
		{Type: engine.ActionBet, Min: 10, Max: 40},
	}
	b.Arm(legal, engine.Flop, 60, 0, 0, 10, "")
	b.OpenSizing(engine.ActionBet)
	b.amount = 20
	b.Nudge(+1)
	if b.amount != 30 {
		t.Errorf("nudge up = %v, want 30", b.amount)
	}
	b.Nudge(+1)
	b.Nudge(+1) // would be 50: engine max is 40
	if b.amount != 40 {
		t.Errorf("nudge past max = %v, want clamp at 40", b.amount)
	}
	for i := 0; i < 10; i++ {
		b.Nudge(-1)
	}
	if b.amount != 10 {
		t.Errorf("nudge past min = %v, want clamp at 10", b.amount)
	}
}

func TestActionBarSkipsSizingWhenPinnedToAllIn(t *testing.T) {
	b := NewActionBar()
	legal := engine.ActionOptions{
		{Type: engine.ActionFold},
		{Type: engine.ActionCall, Min: 100, Max: 100},
		{Type: engine.ActionRaise, Min: 285, Max: 285},
	}
	b.Arm(legal, engine.Flop, 400, 100, 0, 10, "")
	amt, skipped := b.OpenSizing(engine.ActionRaise)
	if !skipped || amt != 285 {
		t.Errorf("pinned raise: got (%v, %v), want (285, skipped)", amt, skipped)
	}
	if b.State() != ActionBarChoosing {
		t.Errorf("skipped sizing must not change state, got %v", b.State())
	}
}

func TestActionBarAlwaysExactlyTwoRows(t *testing.T) {
	widths := []int{60, 76, 80, 102}
	states := map[string]func(*ActionBar){
		"waiting": func(b *ActionBar) { b.Wait() },
		"choosing": func(b *ActionBar) {
			b.Arm(engine.ActionOptions{
				{Type: engine.ActionFold},
				{Type: engine.ActionCall, Min: 20, Max: 20},
				{Type: engine.ActionRaise, Min: 40, Max: 990},
			}, engine.Flop, 45, 20, 10, 10, "to call 20")
		},
		"sizing": func(b *ActionBar) {
			b.Arm(engine.ActionOptions{
				{Type: engine.ActionFold},
				{Type: engine.ActionCheck},
				{Type: engine.ActionBet, Min: 10, Max: 900},
			}, engine.Flop, 185, 0, 0, 10, "")
			b.OpenSizing(engine.ActionBet)
		},
	}
	for name, setup := range states {
		for _, w := range widths {
			b := NewActionBar()
			setup(b)
			view := b.View(w)
			lines := strings.Split(view, "\n")
			if len(lines) != 2 {
				t.Fatalf("%s at %d: %d rows, the bar is exactly 2", name, w, len(lines))
			}
			for i, l := range lines {
				if got := lipgloss.Width(l); got != w {
					t.Errorf("%s at %d: row %d is %d cells", name, w, i, got)
				}
			}
		}
	}
}
