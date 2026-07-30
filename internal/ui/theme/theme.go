// Package theme owns every color, style and special glyph the TUI renders
// with. The conventions (enforced by tests): hex color literals live only in
// palette.go, raw non-ASCII glyphs live only in glyphs.go, and components
// take styles from Current rather than constructing colored styles inline.
package theme

import "github.com/charmbracelet/lipgloss"

// Theme is the complete set of lipgloss styles the UI draws with, split by
// concern (docs/design-tui.md section 7.1). Components read the active theme
// through Current at render time.
type Theme struct {
	// Table
	SeatName, SeatStack, SeatBet, SeatAction lipgloss.Style
	SeatFolded, SeatAllIn, SeatToAct         lipgloss.Style
	PotLine, SidePotLine, BoardPlaceholder   lipgloss.Style
	DealerBadge, PosBadge, HeroBadge         lipgloss.Style

	// Cards
	CardBorder, CardBack, CardWinner lipgloss.Style

	// Panels & chrome
	Header, Rule, StatusLine, Footer          lipgloss.Style
	CoachBox, CoachTitle, GradeGood, GradeBad lipgloss.Style
	ActionKeycap, ActionLabel, SizingSlider   lipgloss.Style

	// Menus / shared screens (euchre parity)
	Title, Subtitle, Body, Help, ContentBox, ScreenBorder lipgloss.Style
	MenuItem, MenuItemSelected, MenuItemDisabled          lipgloss.Style
}

// Default returns the default theme, built entirely from the adaptive
// palette so every style works on both light and dark terminals.
func Default() *Theme {
	return &Theme{
		// Table
		SeatName:   lipgloss.NewStyle().Foreground(ColText),
		SeatStack:  lipgloss.NewStyle().Foreground(ColText),
		SeatBet:    lipgloss.NewStyle().Foreground(ColFelt),
		SeatAction: lipgloss.NewStyle().Foreground(ColMuted),
		SeatFolded: lipgloss.NewStyle().Foreground(ColMuted).Faint(true),
		SeatAllIn:  lipgloss.NewStyle().Foreground(ColGold).Bold(true),
		SeatToAct:  lipgloss.NewStyle().Foreground(ColAccent).Bold(true),

		PotLine:          lipgloss.NewStyle().Foreground(ColFelt).Bold(true),
		SidePotLine:      lipgloss.NewStyle().Foreground(ColFelt),
		BoardPlaceholder: lipgloss.NewStyle().Foreground(ColMuted).Faint(true),

		DealerBadge: lipgloss.NewStyle().Foreground(ColGold).Bold(true),
		PosBadge:    lipgloss.NewStyle().Foreground(ColMuted),
		HeroBadge:   lipgloss.NewStyle().Foreground(ColAccent).Bold(true),

		// Cards
		CardBorder: lipgloss.NewStyle().Foreground(ColMuted),
		CardBack:   lipgloss.NewStyle().Foreground(ColPip),
		CardWinner: lipgloss.NewStyle().Foreground(ColGold).Bold(true),

		// Panels & chrome
		Header:     lipgloss.NewStyle().Foreground(ColText).Bold(true),
		Rule:       lipgloss.NewStyle().Foreground(ColMuted),
		StatusLine: lipgloss.NewStyle().Foreground(ColText),
		Footer:     lipgloss.NewStyle().Foreground(ColMuted),

		CoachBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColGold).
			Padding(0, 1),
		CoachTitle: lipgloss.NewStyle().Foreground(ColGold).Bold(true),
		GradeGood:  lipgloss.NewStyle().Foreground(ColFelt).Bold(true),
		GradeBad:   lipgloss.NewStyle().Foreground(ColWarn).Bold(true),

		ActionKeycap: lipgloss.NewStyle().Foreground(ColAccent).Bold(true),
		ActionLabel:  lipgloss.NewStyle().Foreground(ColText),
		SizingSlider: lipgloss.NewStyle().Foreground(ColAccent),

		// Menus / shared screens
		Title:    lipgloss.NewStyle().Foreground(ColAccent).Bold(true).MarginBottom(1),
		Subtitle: lipgloss.NewStyle().Foreground(ColMuted).Italic(true),
		Body:     lipgloss.NewStyle().Foreground(ColText),
		Help:     lipgloss.NewStyle().Foreground(ColMuted).Italic(true),
		ContentBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColMuted).
			Padding(1, 2),
		ScreenBorder: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColAccent),

		MenuItem:         lipgloss.NewStyle().Foreground(ColText).PaddingLeft(2),
		MenuItemSelected: lipgloss.NewStyle().Foreground(ColAccent).Reverse(true).Bold(true).PaddingLeft(2),
		MenuItemDisabled: lipgloss.NewStyle().Foreground(ColMuted).PaddingLeft(2),
	}
}

// Current is the active theme.
var Current = Default()
