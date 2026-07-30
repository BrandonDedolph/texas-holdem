package theme

import "github.com/charmbracelet/lipgloss"

// Palette: the adaptive base colors. This file is the only place in
// internal/ui where hex color literals may appear (a convention enforced by
// TestNoHexLiteralsOutsidePalette); everything else takes colors from these
// vars or from styles built on them in theme.go.
//
// Every color resolves per terminal background via lipgloss.AdaptiveColor so
// chrome stays legible on both light and dark terminals. The suit inks are
// adaptive too: spade and club ink must flip with the background (near-white
// on dark, near-black on light) or black-suit cards vanish; see
// docs/design-tui.md section 3.3.
var (
	ColAccent = lipgloss.AdaptiveColor{Light: "#2178C4", Dark: "#3498DB"} // turn marker, borders
	ColFelt   = lipgloss.AdaptiveColor{Light: "#1E8449", Dark: "#2ECC71"} // pot, money-good
	ColWarn   = lipgloss.AdaptiveColor{Light: "#C0392B", Dark: "#E74C3C"} // fold, errors, bad grades
	ColGold   = lipgloss.AdaptiveColor{Light: "#B9770E", Dark: "#F1C40F"} // dealer, coach, winners
	ColMuted  = lipgloss.AdaptiveColor{Light: "#7F8C8D", Dark: "#95A5A6"} // secondary text
	ColText   = lipgloss.AdaptiveColor{Light: "#2C3E50", Dark: "#ECF0F1"} // primary body text
	ColPip    = lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#60A5FA"} // card backs
)

// Semantic action colors (docs/table-redesign-pitch.md section 2.1): each
// action verb carries its meaning in its color, so the learner absorbs
// "red = give up, gold = everything" without being taught. Fold is retreat
// (the warn red), check is neutral/safe (felt green), call is commitment
// (the accent blue), raise/bet is aggression (violet - the one new hue in
// the palette), all-in is maximum (gold). Each pair is tuned per background:
// darker saturated inks on light terminals, brighter ones on dark.
var (
	ColActionFold  = lipgloss.AdaptiveColor{Light: "#C0392B", Dark: "#E74C3C"}
	ColActionCheck = lipgloss.AdaptiveColor{Light: "#1E8449", Dark: "#2ECC71"}
	ColActionCall  = lipgloss.AdaptiveColor{Light: "#2178C4", Dark: "#3498DB"}
	ColActionRaise = lipgloss.AdaptiveColor{Light: "#6C3483", Dark: "#A569BD"}
	ColActionAllIn = lipgloss.AdaptiveColor{Light: "#B9770E", Dark: "#F1C40F"}
)

// Suit inks for the four-color deck (docs/design-tui.md section 3.3).
// Hearts, diamonds and clubs are fixed hue pairs (a slightly darker shade on
// light backgrounds for contrast); spades flip near-white/near-black with the
// background. The two-color deck reuses ColSuitSpade for both black suits and
// ColSuitHeart for both red suits, so the toggle is one switch in SuitStyle.
var (
	ColSuitSpade   = lipgloss.AdaptiveColor{Light: "#2C3E50", Dark: "#ECF0F1"}
	ColSuitHeart   = lipgloss.AdaptiveColor{Light: "#C0392B", Dark: "#E74C3C"}
	ColSuitDiamond = lipgloss.AdaptiveColor{Light: "#2178C4", Dark: "#60A5FA"}
	ColSuitClub    = lipgloss.AdaptiveColor{Light: "#1E8449", Dark: "#2ECC71"}
)
