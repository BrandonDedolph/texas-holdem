package theme

import (
	"os"
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// Glyphs is the complete vocabulary of special characters the UI may draw.
// This file is the only place in internal/ui where a raw non-ASCII glyph may
// appear (a convention enforced by TestNoRawGlyphsOutsideGlyphs); components
// always render through the active set G, never a literal.
//
// Every ASCII substitute has the same display width as its Unicode form, so
// swapping sets never moves the layout grid. Where the natural widths differ
// (the dealer disc vs "(D)", the turn pointer vs "->") the Unicode form
// carries built-in padding to keep the pair equal. TestGlyphWidthParity
// asserts the invariant field by field.
type Glyphs struct {
	// Suit symbols, one cell each.
	SuitSpade, SuitHeart, SuitDiamond, SuitClub string

	// Card states. FaceDown is the interior of a face-down mini card (3
	// cells, sized for the "10" rank); Mucked marks folded hole cards (3
	// cells); BoardSlot is the full middle row of an undealt board slot
	// (5 cells, the width of a mini card).
	FaceDown, Mucked, BoardSlot string

	// Table badges. Dealer is the dealer disc (3 cells, padded to match
	// "(D)"); ToAct prefixes the acting seat's name (2 cells, matching
	// "->"); ChipsInFront marks a seat's current-street bet (1 cell).
	Dealer, ToAct, ChipsInFront string

	// Mini-card frame pieces, one cell each. The left and right edges are
	// distinct so the ASCII set can render bracket cards: [As], [##].
	CardTL, CardTR, CardBL, CardBR, CardH, CardVL, CardVR string

	// Double-border card frame, for emphasis: the winning five at showdown,
	// the correct answer in a drill, the card a lesson is pointing at.
	CardDblTL, CardDblTR, CardDblBL, CardDblBR, CardDblH, CardDblV string

	// RuleH draws horizontal rules; Dot separates pot entries and eligible
	// names; Ellipsis marks truncated text. One cell each.
	RuleH, Dot, Ellipsis string

	// Bet-sizing slider pieces, one cell each.
	SliderLeft, SliderRight, SliderKnob, SliderTrack string

	// Position badges (superscript in the Unicode set).
	BadgeUTG, BadgeHJ, BadgeCO, BadgeBTN, BadgeSB, BadgeBB string

	// Grade markers for coach feedback. One cell each: Good marks a decision
	// worth repeating, Bad one that leaked, Query the middle band where the
	// call was defensible but not the best available.
	Good, Bad, Query string

	// Hexagon ring frame pieces (the Direction E table), one cell each.
	// The corners are rounded to match the mini-card frame; RingTeeL/RingTeeR
	// are the junctions that fuse a seat plate's border into a ring line
	// running along it; DiagUp and DiagDown are the slope-1 corner cuts
	// (DiagUp rises left-to-right like /, DiagDown falls like \).
	RingTL, RingTR, RingBL, RingBR, RingH, RingV, RingTeeL, RingTeeR string
	DiagUp, DiagDown                                                 string

	// Felt furniture, one cell each. Chip is a chip lying on the felt (pot
	// and price piles); RaiseMark flags aggression next to a bet; FoldMark
	// replaces a plate's action-order digit when the seat folds; ButtonFill
	// is the fill cap of a colored action-bar button.
	Chip, RaiseMark, FoldMark, ButtonFill string
}

// Unicode returns the full-fidelity glyph set.
func Unicode() Glyphs {
	return Glyphs{
		SuitSpade:   "♠",
		SuitHeart:   "♥",
		SuitDiamond: "♦",
		SuitClub:    "♣",

		FaceDown:  "▓▓▓",
		Mucked:    "· ·",
		BoardSlot: "  ·  ",

		Dealer:       " Ⓓ ",
		ToAct:        "► ",
		ChipsInFront: "●",

		CardDblTL: "╔",
		CardDblTR: "╗",
		CardDblBL: "╚",
		CardDblBR: "╝",
		CardDblH:  "═",
		CardDblV:  "║",

		CardTL: "╭",
		CardTR: "╮",
		CardBL: "╰",
		CardBR: "╯",
		CardH:  "─",
		CardVL: "│",
		CardVR: "│",

		RuleH:    "─",
		Dot:      "·",
		Ellipsis: "…",

		SliderLeft:  "├",
		SliderRight: "┤",
		SliderKnob:  "●",
		SliderTrack: "─",

		BadgeUTG: "ᵁᵀᴳ",
		BadgeHJ:  "ᴴᴶ",
		BadgeCO:  "ᶜᴼ",
		BadgeBTN: "ᴮᵀᴺ",
		BadgeSB:  "ˢᴮ",
		BadgeBB:  "ᴮᴮ",

		Good:  "✔",
		Bad:   "✗",
		Query: "?",

		RingTL:   "╭",
		RingTR:   "╮",
		RingBL:   "╰",
		RingBR:   "╯",
		RingH:    "─",
		RingV:    "│",
		RingTeeL: "├",
		RingTeeR: "┤",
		DiagUp:   "╱",
		DiagDown: "╲",

		Chip:       "●",
		RaiseMark:  "▲",
		FoldMark:   "✗",
		ButtonFill: "▓",
	}
}

// ASCII returns the pure-ASCII fallback set for terminals without Unicode.
// Cards render as bracket pairs ([As ] or [10s] face up, [###] face down,
// [---] for an undealt board slot) and box drawing degrades to -, | and +.
func ASCII() Glyphs {
	return Glyphs{
		SuitSpade:   "s",
		SuitHeart:   "h",
		SuitDiamond: "d",
		SuitClub:    "c",

		FaceDown:  "###",
		Mucked:    ". .",
		BoardSlot: "[---]",

		Dealer:       "(D)",
		ToAct:        "->",
		ChipsInFront: "*",

		CardDblTL: "#",
		CardDblTR: "#",
		CardDblBL: "#",
		CardDblBR: "#",
		CardDblH:  "=",
		CardDblV:  "#",

		CardTL: "+",
		CardTR: "+",
		CardBL: "+",
		CardBR: "+",
		CardH:  "-",
		CardVL: "[",
		CardVR: "]",

		RuleH:    "-",
		Dot:      ".",
		Ellipsis: "~",

		SliderLeft:  "|",
		SliderRight: "|",
		SliderKnob:  "*",
		SliderTrack: "-",

		BadgeUTG: "UTG",
		BadgeHJ:  "HJ",
		BadgeCO:  "CO",
		BadgeBTN: "BTN",
		BadgeSB:  "SB",
		BadgeBB:  "BB",

		Good:  "+",
		Bad:   "x",
		Query: "?",

		RingTL:   "+",
		RingTR:   "+",
		RingBL:   "+",
		RingBR:   "+",
		RingH:    "-",
		RingV:    "|",
		RingTeeL: "+",
		RingTeeR: "+",
		DiagUp:   "/",
		DiagDown: "\\",

		Chip:       "o",
		RaiseMark:  "^",
		FoldMark:   "x",
		ButtonFill: "#",
	}
}

// G is the active glyph set. It defaults to Unicode and is selected once at
// startup by Init; a Settings override may assign it directly afterwards.
// It is not safe to swap mid-render.
var G = Unicode()

// Init selects the active glyph set from the environment: Unicode when the
// locale advertises UTF-8, ASCII otherwise.
func Init() {
	if UnicodeSupported() {
		G = Unicode()
	} else {
		G = ASCII()
	}
}

// UnicodeSupported reports whether the locale environment advertises UTF-8
// (the standard probe: LC_ALL beats LC_CTYPE beats LANG).
func UnicodeSupported() bool {
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if v := os.Getenv(key); v != "" {
			v = strings.ToUpper(strings.ReplaceAll(v, "-", ""))
			return strings.Contains(v, "UTF8")
		}
	}
	return false
}

// SuitGlyph returns the active glyph for a suit ("s" in the ASCII set).
func SuitGlyph(s engine.Suit) string {
	switch s {
	case engine.Spades:
		return G.SuitSpade
	case engine.Hearts:
		return G.SuitHeart
	case engine.Diamonds:
		return G.SuitDiamond
	case engine.Clubs:
		return G.SuitClub
	}
	return "?"
}

// PositionBadge returns the active badge for a table position.
func PositionBadge(p engine.Position) string {
	switch p {
	case engine.PosUTG:
		return G.BadgeUTG
	case engine.PosHJ:
		return G.BadgeHJ
	case engine.PosCO:
		return G.BadgeCO
	case engine.PosBTN:
		return G.BadgeBTN
	case engine.PosSB:
		return G.BadgeSB
	case engine.PosBB:
		return G.BadgeBB
	}
	return "?"
}
