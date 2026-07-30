package app

import (
	"fmt"
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// LayoutMode is the breakpoint the current terminal size selects. Layout is
// chosen per-View() from width/height and never stored, so a live resize in
// either direction just works (docs/design-tui.md §2.7).
type LayoutMode int

// Layout modes, roomiest first.
const (
	LayoutWide     LayoutMode = iota // >=104x28: coach side panel fits
	LayoutFull                       // >=80x24: the target layout
	LayoutCompact                    // >=60x20: one-line-per-seat ledger
	LayoutTooSmall                   // below 60x20: refuse, don't garble
)

// Breakpoints (docs/design-tui.md §2.7). Widths and heights gate together:
// a 200x10 terminal is too small no matter how wide it is.
const (
	wideMinWidth, wideMinHeight       = 104, 28
	fullMinWidth, fullMinHeight       = 80, 24
	compactMinWidth, compactMinHeight = 60, 20
)

// layoutFor selects the layout mode for a terminal size.
func layoutFor(w, h int) LayoutMode {
	switch {
	case w >= wideMinWidth && h >= wideMinHeight:
		return LayoutWide
	case w >= fullMinWidth && h >= fullMinHeight:
		return LayoutFull
	case w >= compactMinWidth && h >= compactMinHeight:
		return LayoutCompact
	default:
		return LayoutTooSmall
	}
}

// renderTooSmall is the hard floor: a centered statement of what the app
// needs and what it got, so the fix is obvious (euchre pattern). "x" rather
// than a multiplication glyph: this screen is exactly where a degraded
// terminal is most likely, so it uses nothing but ASCII.
func renderTooSmall(w, h int) string {
	msg := theme.Current.Header.Render("Terminal too small") + "\n" +
		theme.Current.Body.Render(
			fmt.Sprintf("need at least %dx%d, have %dx%d",
				compactMinWidth, compactMinHeight, w, h))
	// Center only when the terminal has any room to center in; below a few
	// cells lipgloss.Place would just pad garbage around wrapped text.
	if w < 20 || h < 3 {
		return msg
	}
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, msg)
}

// fallbackSize substitutes the 80x24 baseline until the first
// tea.WindowSizeMsg arrives, so View() never divides by zero.
func fallbackSize(w, h int) (int, int) {
	if w <= 0 {
		w = fullMinWidth
	}
	if h <= 0 {
		h = fullMinHeight
	}
	return w, h
}

// --- The chrome system --------------------------------------------------------
//
// THE CHROME RULE (docs/ui-review.md F7 — decided, do not drift):
//
// Every screen is full-bleed and speaks the table's grammar, one shared
// five-part shape rendered by renderShell:
//
//	row 1     header — title on the left, context + "? help" on the right
//	row 2     full-width horizontal rule
//	rows 3..  content, fixed budget of exactly h-4 rows, top-anchored
//	row h-1   status — one line about the current selection or state
//	row h     footer — the key legend, right-aligned, ending "? help"
//
// There are no screen borders. The rounded border (frame, below) is
// reserved for true overlays — the help sheet, future confirms/popups —
// where "floating above the screen" is exactly what the border should say.
// The table and hand review implement the same grammar with richer interior
// rules; a new screen must either call renderShell or be an overlay.
//
// Why: the app used to split into two accidental visual families (bordered
// floating menus vs. full-bleed working screens) that flipped mid-flow —
// opening a lesson, starting a drill. One grammar means chrome never moves
// between screens: the title is always at the top-left, the keys are always
// at the bottom-right, and the rule under the title is the same rule the
// table draws.

// shell is one screen's chrome, consumed by renderShell. All fields are
// plain text unless noted; renderShell owns the styling so screens cannot
// drift on treatment.
type shell struct {
	Title       string // header left, rendered in the Header style
	TitleExtra  string // optional pre-styled decoration appended to the title
	HeaderRight string // header right context; "? help" is appended after it
	Status      string // the status row; blank keeps the row present but empty
	Footer      string // key legend; " · ? help" is appended, then right-aligned
}

// renderShell draws one content screen in the shared chrome (see THE CHROME
// RULE above). The result is always exactly h rows and every row exactly w
// cells: header, rule, an h-4 row content region (content is centered
// horizontally, anchored to the top, cropped rather than reflowed), status,
// footer.
func renderShell(w, h int, s shell, content string) string {
	th := theme.Current
	dot := " " + theme.G.Dot + " "

	right := "? help "
	if s.HeaderRight != "" {
		right = s.HeaderRight + dot + "? help "
	}
	titleWidth := w - lipgloss.Width(right) - lipgloss.Width(s.TitleExtra) - 1
	header := rowLR(w,
		th.Header.Render(clip(" "+s.Title, titleWidth))+s.TitleExtra,
		th.Footer.Render(right))
	rule := th.Rule.Render(strings.Repeat(theme.G.RuleH, w))

	budget := h - 4
	body := padHeight(
		lipgloss.Place(w, budget, lipgloss.Center, lipgloss.Top, content),
		budget)

	status := padStyledTo(" "+th.StatusLine.Render(clip(s.Status, w-2)), w)
	footer := rowLR(w, "", th.Footer.Render(s.Footer+dot+"? help "))
	return header + "\n" + rule + "\n" + body + "\n" + status + "\n" + footer
}

// frame wraps content in a rounded border centered in the terminal. Under
// THE CHROME RULE above this shape is reserved for overlays (the help
// sheet); screens themselves render through renderShell. The result is
// always exactly h rows tall.
func frame(w, h int, content string) string {
	content = cropHeight(content, h-4)
	inner := lipgloss.Place(w-4, h-4, lipgloss.Center, lipgloss.Center, content)
	box := theme.Current.ScreenBorder.Width(w - 2).Height(h - 2).Render(inner)
	return cropHeight(lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box), h)
}

// cropHeight hard-limits a block to n rows. lipgloss.Place pads short
// content but lets tall content overflow; overflowing content would grow the
// view and break the height-stability invariant, so anything that doesn't
// fit is cut rather than allowed to reflow the screen.
func cropHeight(s string, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n")
}

// padHeight pads a block with blank lines to exactly n rows (and crops if
// over). Regions that must hold a fixed row budget render through this.
func padHeight(s string, n int) string {
	lines := strings.Split(cropHeight(s, n), "\n")
	for len(lines) < n {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// --- formList ----------------------------------------------------------------
//
// formList is the one list widget behind MainMenu, GameSetup and Settings: a
// column of rows, some plain actions, some value fields cycled in place.
// It lives here (rather than internal/ui/components) because it is chrome
// for the shell screens, not part of the table rendering vocabulary — and
// keeping it in-package lets the screens stay thin.

// formListWidth is the fixed inner width every formList screen renders at:
// wide enough for the longest label/value pair, narrow enough to fit the
// 60-column compact floor with margins to spare.
const formListWidth = 50

// listRow is one row of a formList. A row with Options is a value field;
// without, it is an action. Disabled rows render dimmed and the cursor
// skips them.
type listRow struct {
	Label    string
	Detail   string // one-line description shown while the row is selected
	Disabled bool
	Options  []string // value choices; empty means the row is an action
	Sel      int      // index into Options
}

// Value is the row's current option, or "" for action rows.
func (r *listRow) Value() string {
	if len(r.Options) == 0 {
		return ""
	}
	return r.Options[r.Sel]
}

// formList is a cursor over rows with wrap-around movement.
type formList struct {
	rows   []listRow
	cursor int
}

// newFormList builds a list with the cursor on the first enabled row.
func newFormList(rows []listRow) *formList {
	f := &formList{rows: rows}
	if len(rows) > 0 && rows[0].Disabled {
		f.MoveDown()
	}
	return f
}

// Cursor is the index of the selected row.
func (f *formList) Cursor() int { return f.cursor }

// Row is the selected row, or nil for an empty list.
func (f *formList) Row() *listRow {
	if len(f.rows) == 0 {
		return nil
	}
	return &f.rows[f.cursor]
}

// MoveUp moves the cursor up, wrapping at the top and skipping disabled
// rows. A list with no enabled rows leaves the cursor in place.
func (f *formList) MoveUp() { f.step(-1) }

// MoveDown moves the cursor down, wrapping at the bottom and skipping
// disabled rows.
func (f *formList) MoveDown() { f.step(1) }

func (f *formList) step(dir int) {
	n := len(f.rows)
	if n == 0 {
		return
	}
	// At most n hops: if every other row is disabled we come back to where
	// we started and stay there rather than spinning forever.
	i := f.cursor
	for hops := 0; hops < n; hops++ {
		i = (i + dir + n) % n
		if !f.rows[i].Disabled {
			f.cursor = i
			return
		}
	}
}

// CycleLeft moves the selected row's value to the previous option, wrapping.
// Reports whether the row had options to cycle.
func (f *formList) CycleLeft() bool { return f.cycle(-1) }

// CycleRight moves the selected row's value to the next option, wrapping.
func (f *formList) CycleRight() bool { return f.cycle(1) }

func (f *formList) cycle(dir int) bool {
	r := f.Row()
	if r == nil || len(r.Options) == 0 {
		return false
	}
	r.Sel = (r.Sel + dir + len(r.Options)) % len(r.Options)
	return true
}

// Detail is the selected row's one-line description, for the shell's
// status row (the chrome rule: the status line describes the selection).
func (f *formList) Detail() string {
	if r := f.Row(); r != nil {
		return r.Detail
	}
	return ""
}

// Render draws the rows at a fixed inner width: every row is padded to
// exactly width cells, so selection changes and value cycling can never
// move an anchor or change the widget's height. The selected row's Detail
// is not drawn here — screens surface it on the shell's status row.
func (f *formList) Render(width int) string {
	th := theme.Current
	var b strings.Builder
	for i := range f.rows {
		r := &f.rows[i]
		// A literal cursor marker in addition to the selected style, so the
		// selection survives monochrome terminals; both prefixes are two
		// cells wide, so moving the cursor never shifts a column.
		line := "  " + r.Label
		if i == f.cursor && !r.Disabled {
			line = "> " + r.Label
		}
		if len(r.Options) > 0 {
			value := "< " + r.Value() + " >"
			gap := width - lipgloss.Width(line) - lipgloss.Width(value)
			if gap < 1 {
				gap = 1
			}
			line += strings.Repeat(" ", gap) + value
		}
		line = padRow(line, width)
		switch {
		case r.Disabled:
			b.WriteString(th.MenuItemDisabled.Render(line))
		case i == f.cursor:
			b.WriteString(th.MenuItemSelected.Render(line))
		default:
			b.WriteString(th.MenuItem.Render(line))
		}
		if i < len(f.rows)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// padRow pads or truncates a plain-text line to exactly width cells.
func padRow(s string, width int) string {
	w := lipgloss.Width(s)
	if w > width {
		return truncateTo(s, width)
	}
	return s + strings.Repeat(" ", width-w)
}

// truncateTo cuts a plain-text line to width cells, marking the cut with
// the theme's ellipsis glyph. Truncation, never wrapping: a wrapped row
// would grow the list and shift everything below it.
func truncateTo(s string, width int) string {
	if width <= 1 {
		return theme.G.Ellipsis
	}
	runes := []rune(s)
	out := make([]rune, 0, width)
	used := 0
	for _, r := range runes {
		rw := lipgloss.Width(string(r))
		if used+rw > width-1 {
			break
		}
		out = append(out, r)
		used += rw
	}
	return string(out) + strings.Repeat(" ", width-1-used) + theme.G.Ellipsis
}
