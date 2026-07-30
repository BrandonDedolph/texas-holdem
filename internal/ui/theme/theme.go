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

	// Hexagon table (docs/table-redesign-pitch-2.md, Direction E)
	RingFrame lipgloss.Style // the drawn hexagon ring, felt green
	RingDigit lipgloss.Style // engraved action-order digits, the rim's one saturated ink
	SeatRead  lipgloss.Style // one-word archetype reads ("tight"), muted italic

	// Panels & chrome
	Header, Rule, StatusLine, Footer          lipgloss.Style
	CoachBox, CoachTitle, GradeGood, GradeBad lipgloss.Style
	ActionKeycap, ActionLabel, SizingSlider   lipgloss.Style

	// Semantic action verbs (docs/table-redesign-pitch.md section 2.1).
	// The Action* styles are the plain inks for inline mentions; the
	// Button* styles are the filled treatment for action-bar caps
	// (reverse video, so the fill adapts to any terminal background).
	ActionFold, ActionCheck, ActionCall, ActionRaise, ActionAllIn lipgloss.Style
	ButtonFold, ButtonCheck, ButtonCall, ButtonRaise, ButtonAllIn lipgloss.Style

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

		// Hexagon table
		RingFrame: lipgloss.NewStyle().Foreground(ColFelt),
		RingDigit: lipgloss.NewStyle().Foreground(ColGold).Bold(true),
		SeatRead:  lipgloss.NewStyle().Foreground(ColMuted).Italic(true),

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

		// Semantic action verbs
		ActionFold:  lipgloss.NewStyle().Foreground(ColActionFold).Bold(true),
		ActionCheck: lipgloss.NewStyle().Foreground(ColActionCheck).Bold(true),
		ActionCall:  lipgloss.NewStyle().Foreground(ColActionCall).Bold(true),
		ActionRaise: lipgloss.NewStyle().Foreground(ColActionRaise).Bold(true),
		ActionAllIn: lipgloss.NewStyle().Foreground(ColActionAllIn).Bold(true),

		ButtonFold:  lipgloss.NewStyle().Foreground(ColActionFold).Reverse(true).Bold(true),
		ButtonCheck: lipgloss.NewStyle().Foreground(ColActionCheck).Reverse(true).Bold(true),
		ButtonCall:  lipgloss.NewStyle().Foreground(ColActionCall).Reverse(true).Bold(true),
		ButtonRaise: lipgloss.NewStyle().Foreground(ColActionRaise).Reverse(true).Bold(true),
		ButtonAllIn: lipgloss.NewStyle().Foreground(ColActionAllIn).Reverse(true).Bold(true),

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
