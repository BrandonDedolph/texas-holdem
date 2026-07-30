package app

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
)

// newTestApp builds an App over a throwaway profile with any profile writes
// redirected into a temp dir, sized to the 80x24 baseline.
func newTestApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	a := NewWithProfile(profile.NewProfile())
	drive(t, a, tea.WindowSizeMsg{Width: 80, Height: 24})
	return a
}

// drive sends one message through the App, following any returned commands
// until the message chain settles (Navigate commands emit NavigateMsg, which
// must itself be routed, exactly as the Bubble Tea runtime would).
func drive(t *testing.T, a *App, msg tea.Msg) {
	t.Helper()
	m, cmd := a.Update(msg)
	if m.(*App) != a {
		t.Fatal("App.Update must return the same *App")
	}
	for cmd != nil {
		out := cmd()
		cmd = nil
		switch out := out.(type) {
		case NavigateMsg, EndSessionMsg, QuitMsg:
			_, cmd = a.Update(out)
		case tea.BatchMsg:
			for _, c := range out {
				if c != nil {
					drive(t, a, c())
				}
			}
		}
	}
}

// key builds a KeyMsg whose String() is s: named keys map to their KeyType,
// anything else is sent as runes.
func key(s string) tea.KeyMsg {
	types := map[string]tea.KeyType{
		"up": tea.KeyUp, "down": tea.KeyDown,
		"left": tea.KeyLeft, "right": tea.KeyRight,
		"enter": tea.KeyEnter, " ": tea.KeySpace,
		"esc": tea.KeyEscape, "tab": tea.KeyTab,
		"ctrl+c": tea.KeyCtrlC, "ctrl+r": tea.KeyCtrlR,
		"pgdown": tea.KeyPgDown, "f1": tea.KeyF1,
		"backspace": tea.KeyBackspace,
	}
	if kt, ok := types[s]; ok {
		return tea.KeyMsg{Type: kt}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestNavigationPreservesCachedModel(t *testing.T) {
	a := newTestApp(t)

	drive(t, a, NavigateMsg{Screen: ScreenQuickReference})
	qr := a.models[ScreenQuickReference].(*QuickReference)
	drive(t, a, key("3")) // move off the default tab: state to preserve

	drive(t, a, NavigateMsg{Screen: ScreenMainMenu})
	drive(t, a, NavigateMsg{Screen: ScreenQuickReference})

	got := a.models[ScreenQuickReference].(*QuickReference)
	if got != qr {
		t.Error("navigating away and back should reuse the cached model")
	}
	if got.active != tabPotOdds {
		t.Errorf("cached model lost its state: tab = %v, want %v", got.active, tabPotOdds)
	}
}

func TestNavigationFreshDataReplacesModel(t *testing.T) {
	a := newTestApp(t)
	cfg := TableConfig{SmallBlind: 5, BigBlind: 10, Stack: 1000,
		Lineup: ClassroomLineup(), CoachMode: CoachFull, Speed: SpeedInstant}

	drive(t, a, NavigateMsg{Screen: ScreenTable, Data: cfg})
	first := a.models[ScreenTable]
	if first == nil {
		t.Fatal("table model not created")
	}

	// nil data returns to the live session: same model.
	drive(t, a, NavigateMsg{Screen: ScreenMainMenu})
	drive(t, a, NavigateMsg{Screen: ScreenTable})
	if a.models[ScreenTable] != first {
		t.Error("Navigate with nil data must reuse the live session model")
	}

	// Fresh construction data replaces it.
	drive(t, a, NavigateMsg{Screen: ScreenTable, Data: cfg})
	if a.models[ScreenTable] == first {
		t.Error("Navigate with fresh data must construct a new model")
	}
}

func TestEndSessionDropsTableAndReview(t *testing.T) {
	a := newTestApp(t)
	// SpeedInstant so the real table model settles synchronously — a Learn
	// session's Init returns a think-time tick that drive() would sleep on.
	cfg := TableConfig{SmallBlind: 5, BigBlind: 10, Stack: 1000,
		Lineup: ClassroomLineup(), Speed: SpeedInstant}

	drive(t, a, NavigateMsg{Screen: ScreenTable, Data: cfg})
	drive(t, a, NavigateMsg{Screen: ScreenHandReview,
		Data: ReviewRequest{ReturnTo: ScreenTable}})
	drive(t, a, EndSessionMsg{})

	if _, ok := a.models[ScreenTable]; ok {
		t.Error("EndSessionMsg must drop the cached table model")
	}
	if _, ok := a.models[ScreenHandReview]; ok {
		t.Error("EndSessionMsg must drop the cached hand-review model")
	}
	if a.current != ScreenMainMenu {
		t.Errorf("EndSessionMsg should land on the main menu, got %v", a.current)
	}
}

func TestWindowSizeForwardedBeforeInit(t *testing.T) {
	a := newTestApp(t)
	drive(t, a, tea.WindowSizeMsg{Width: 100, Height: 30})

	drive(t, a, NavigateMsg{Screen: ScreenLessons})
	cs := a.models[ScreenLessons].(*ComingSoon)
	if cs.width != 100 || cs.height != 30 {
		t.Errorf("new screen sized %dx%d before Init, want 100x30", cs.width, cs.height)
	}
}

func TestGameSetupEmitsTableConfig(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	p := profile.NewProfile()
	g := NewGameSetup(p, DefaultPrefs())

	// Cursor starts on Start Game; selecting it must emit the payload.
	cmd, handled := g.handleAction(ActSelect)
	if !handled || cmd == nil {
		t.Fatal("selecting Start Game must produce a navigation command")
	}
	msg, ok := cmd().(NavigateMsg)
	if !ok {
		t.Fatalf("expected NavigateMsg, got %T", cmd())
	}
	if msg.Screen != ScreenTable {
		t.Errorf("start should navigate to the table, got %v", msg.Screen)
	}
	cfg, ok := msg.Data.(TableConfig)
	if !ok {
		t.Fatalf("payload must be a TableConfig, got %T", msg.Data)
	}

	// Defaults for a fresh profile: 5/10, 100bb, classroom mix, coach Full.
	if cfg.SmallBlind != 5 || cfg.BigBlind != 10 {
		t.Errorf("blinds = %v/%v, want 5/10", cfg.SmallBlind, cfg.BigBlind)
	}
	if cfg.Stack != engine.Chips(1000) || cfg.StackBB() != 100 {
		t.Errorf("stack = %v (%dbb), want 1,000 (100bb)", cfg.Stack, cfg.StackBB())
	}
	if want := ClassroomLineup(); !sameLineup(cfg.Lineup, want) {
		t.Errorf("lineup = %v, want classroom mix %v", cfg.Lineup, want)
	}
	if cfg.CoachMode != CoachFull {
		t.Errorf("coach mode = %v, want Full", cfg.CoachMode)
	}

	// Starting persists the choices as next session's defaults.
	if p.TableDefaults.SmallBlind != 5 || p.TableDefaults.BigBlind != 10 {
		t.Errorf("profile defaults not updated: %+v", p.TableDefaults)
	}
}

func TestRenderTooSmall(t *testing.T) {
	cases := []struct{ w, h int }{
		{52, 18}, {59, 20}, {60, 19}, {30, 10},
	}
	for _, tc := range cases {
		view := renderTooSmall(tc.w, tc.h)
		if !strings.Contains(view, "Terminal too small") {
			t.Errorf("%dx%d: missing headline", tc.w, tc.h)
		}
		if !strings.Contains(view, "need at least 60x20") {
			t.Errorf("%dx%d: must state the requirement", tc.w, tc.h)
		}
		want := "have " + itoa(tc.w) + "x" + itoa(tc.h)
		if !strings.Contains(view, want) {
			t.Errorf("%dx%d: must state the actual size %q", tc.w, tc.h, want)
		}
	}
}

// itoa avoids strconv for tiny test numbers.
func itoa(n int) string {
	if n >= 10 {
		return itoa(n/10) + string(rune('0'+n%10))
	}
	return string(rune('0' + n))
}

func TestAppViewEnforcesTooSmallFloor(t *testing.T) {
	a := newTestApp(t)
	drive(t, a, tea.WindowSizeMsg{Width: 52, Height: 18})
	if view := a.View(); !strings.Contains(view, "Terminal too small") {
		t.Error("App.View must render the too-small screen below 60x20")
	}
	// Growing back restores the screen.
	drive(t, a, tea.WindowSizeMsg{Width: 80, Height: 24})
	if view := a.View(); strings.Contains(view, "Terminal too small") {
		t.Error("App.View must recover when the terminal grows")
	}
}

func TestLayoutFor(t *testing.T) {
	cases := []struct {
		w, h int
		want LayoutMode
	}{
		{104, 28, LayoutWide},
		{120, 40, LayoutWide},
		{104, 27, LayoutFull}, // wide needs both dimensions
		{80, 24, LayoutFull},
		{103, 24, LayoutFull},
		{79, 24, LayoutCompact},
		{80, 23, LayoutCompact},
		{60, 20, LayoutCompact},
		{59, 20, LayoutTooSmall},
		{60, 19, LayoutTooSmall},
		{200, 10, LayoutTooSmall}, // width cannot buy back height
	}
	for _, tc := range cases {
		if got := layoutFor(tc.w, tc.h); got != tc.want {
			t.Errorf("layoutFor(%d, %d) = %v, want %v", tc.w, tc.h, got, tc.want)
		}
	}
}

func TestFormListWrapsAndSkipsDisabled(t *testing.T) {
	f := newFormList([]listRow{
		{Label: "a"},
		{Label: "b", Disabled: true},
		{Label: "c"},
	})
	if f.Cursor() != 0 {
		t.Fatalf("cursor starts at %d, want 0", f.Cursor())
	}
	f.MoveDown() // skips b
	if f.Cursor() != 2 {
		t.Errorf("MoveDown should skip disabled row: cursor %d, want 2", f.Cursor())
	}
	f.MoveDown() // wraps past the end back to a
	if f.Cursor() != 0 {
		t.Errorf("MoveDown should wrap: cursor %d, want 0", f.Cursor())
	}
	f.MoveUp() // wraps up, skipping b
	if f.Cursor() != 2 {
		t.Errorf("MoveUp should wrap and skip disabled: cursor %d, want 2", f.Cursor())
	}
}

func TestFormListDisabledFirstRow(t *testing.T) {
	f := newFormList([]listRow{
		{Label: "a", Disabled: true},
		{Label: "b"},
	})
	if f.Cursor() != 1 {
		t.Errorf("cursor must start on the first enabled row, got %d", f.Cursor())
	}
}

func TestFormListAllDisabledDoesNotSpin(t *testing.T) {
	f := newFormList([]listRow{
		{Label: "a", Disabled: true},
		{Label: "b", Disabled: true},
	})
	f.MoveDown()
	f.MoveUp() // must terminate; cursor position is unspecified but valid
	if c := f.Cursor(); c < 0 || c > 1 {
		t.Errorf("cursor out of range: %d", c)
	}
}

func TestMainMenuRoutes(t *testing.T) {
	a := newTestApp(t)

	// Select "Play" (first row): must land on GameSetup.
	drive(t, a, key("enter"))
	if a.current != ScreenGameSetup {
		t.Fatalf("Play should open GameSetup, got %v", a.current)
	}

	// Esc returns to the menu.
	drive(t, a, key("esc"))
	if a.current != ScreenMainMenu {
		t.Fatalf("esc from setup should return to the menu, got %v", a.current)
	}

	// Lessons and Trainer land on honest placeholders.
	drive(t, a, key("down"))
	drive(t, a, key("enter"))
	if a.current != ScreenLessons {
		t.Fatalf("second row should open Lessons, got %v", a.current)
	}
	if view := a.View(); !strings.Contains(view, "Coming soon") {
		t.Error("Lessons placeholder must say it is coming soon")
	}
}

func TestQuitFromMenu(t *testing.T) {
	a := newTestApp(t)
	m, _ := a.Update(key("ctrl+c"))
	if !m.(*App).quitting {
		t.Error("ctrl+c must quit from anywhere")
	}
}
