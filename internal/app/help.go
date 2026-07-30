package app

import (
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// helpOverlay is the "?" help sheet state every shell screen embeds. It owns
// the one piece of key handling that is the same everywhere: "?" opens the
// sheet, the next key (any key) closes it, and while it is open no key
// reaches the screen underneath — the game state is frozen, not consumed
// (euchre pattern).
type helpOverlay struct {
	open bool
}

// dispatch routes one key press through the screen's keymap. It returns the
// command to run and whether the key was consumed.
//
// This is the enforcement point for the keybind-legend invariant: a key not
// in the keymap never reaches do(), so the update loop cannot accept a key
// the legend does not document.
func (o *helpOverlay) dispatch(km KeyMap, msg tea.KeyMsg, do func(KeyAction) (tea.Cmd, bool)) (tea.Cmd, bool) {
	if o.open {
		o.open = false // any key closes
		return nil, true
	}
	a, ok := km.Lookup(msg.String())
	if !ok {
		return nil, false
	}
	if a == ActHelp {
		o.open = true
		return nil, true
	}
	return do(a)
}

// renderHelp draws the help sheet for one screen's keymap, plus the global
// bindings the App root handles. Both the sheet and the screen's update
// loop consume the same KeyMap values — keymap.go is the single source of
// truth and this renderer adds nothing to it.
func renderHelp(title string, km KeyMap, w, h int) string {
	w, h = fallbackSize(w, h)
	th := theme.Current

	// One line, not two, under the title: the table's choosing-state sheet
	// is the longest keymap and must fit inside an 80x24 frame whole.
	var b strings.Builder
	b.WriteString(th.CoachTitle.Render("Controls") + "\n")
	if title != "" {
		b.WriteString(th.Subtitle.Render(title) + "\n")
	}
	writeBindings(&b, km)
	b.WriteString("\n" + th.Subtitle.Render("Everywhere") + "\n")
	writeBindings(&b, globalKeys)
	b.WriteString(th.Help.Render("Press any key to close"))

	// No vertical padding: the table's choosing-state sheet is 18 rows of
	// text, and the frame leaves exactly 20 inside an 80x24 terminal — the
	// border must survive whole.
	sheet := th.ContentBox.Padding(0, 2).Render(b.String())
	return frame(w, h, sheet)
}

// writeBindings renders one keymap as aligned "keycap  description" lines.
// Bindings sharing a label+help render once: key families like the sizing
// digits are five actions but one legend row.
func writeBindings(b *strings.Builder, km KeyMap) {
	labelWidth := 0
	for _, bind := range km {
		if w := lipgloss.Width(bind.Label); w > labelWidth {
			labelWidth = w
		}
	}
	th := theme.Current
	seen := map[string]bool{}
	for _, bind := range km {
		if seen[bind.Label+"\x00"+bind.Help] {
			continue
		}
		seen[bind.Label+"\x00"+bind.Help] = true
		pad := strings.Repeat(" ", labelWidth-lipgloss.Width(bind.Label))
		b.WriteString("  " + th.ActionKeycap.Render(bind.Label) + pad + "  " +
			th.Body.Render(bind.Help) + "\n")
	}
}
