package theme

import (
	"reflect"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/charmbracelet/lipgloss"
)

// setGlyphs swaps the active glyph set for one test.
func setGlyphs(t *testing.T, g Glyphs) {
	t.Helper()
	old := G
	G = g
	t.Cleanup(func() { G = old })
}

// TestGlyphWidthParity is the fallback-safety invariant: for every field of
// the glyph table, the ASCII substitute has exactly the same display width
// as the Unicode form, so swapping sets never moves the layout grid.
func TestGlyphWidthParity(t *testing.T) {
	u, a := Unicode(), ASCII()
	uv, av := reflect.ValueOf(u), reflect.ValueOf(a)
	typ := reflect.TypeOf(u)
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		us, as := uv.Field(i).String(), av.Field(i).String()
		if us == "" || as == "" {
			t.Errorf("Glyphs.%s: empty glyph (unicode %q, ascii %q)", name, us, as)
			continue
		}
		if uw, aw := lipgloss.Width(us), lipgloss.Width(as); uw != aw {
			t.Errorf("Glyphs.%s: unicode %q is %d cells, ascii %q is %d cells",
				name, us, uw, as, aw)
		}
		for _, r := range as {
			if r > 127 {
				t.Errorf("Glyphs.%s: ASCII set contains non-ASCII rune %q", name, r)
			}
		}
	}
}

// TestSuitGlyphsSingleCell pins the risk flagged in docs/DESIGN.md section 4:
// the 5x3 mini card assumes the suit symbols render one cell wide under
// lipgloss.Width. If this fails, the ASCII set is the escape hatch.
func TestSuitGlyphsSingleCell(t *testing.T) {
	u := Unicode()
	for name, glyph := range map[string]string{
		"SuitSpade":   u.SuitSpade,
		"SuitHeart":   u.SuitHeart,
		"SuitDiamond": u.SuitDiamond,
		"SuitClub":    u.SuitClub,
	} {
		if w := lipgloss.Width(glyph); w != 1 {
			t.Errorf("%s %q: display width %d, want 1", name, glyph, w)
		}
	}
}

func TestSuitGlyphAllSuits(t *testing.T) {
	suits := []engine.Suit{engine.Clubs, engine.Diamonds, engine.Hearts, engine.Spades}
	for _, set := range []Glyphs{Unicode(), ASCII()} {
		setGlyphs(t, set)
		seen := map[string]bool{}
		for _, s := range suits {
			g := SuitGlyph(s)
			if g == "?" {
				t.Errorf("SuitGlyph(%v) = %q", s, g)
			}
			if w := lipgloss.Width(g); w != 1 {
				t.Errorf("SuitGlyph(%v) = %q: width %d, want 1", s, g, w)
			}
			if seen[g] {
				t.Errorf("SuitGlyph(%v) = %q: duplicate glyph", s, g)
			}
			seen[g] = true
		}
	}
	if g := SuitGlyph(engine.Suit(9)); g != "?" {
		t.Errorf("SuitGlyph(invalid) = %q, want %q", g, "?")
	}
}

func TestPositionBadgeParity(t *testing.T) {
	positions := []engine.Position{
		engine.PosUTG, engine.PosHJ, engine.PosCO,
		engine.PosBTN, engine.PosSB, engine.PosBB,
	}
	for _, p := range positions {
		setGlyphs(t, Unicode())
		uni := PositionBadge(p)
		setGlyphs(t, ASCII())
		asc := PositionBadge(p)
		if asc != p.String() {
			t.Errorf("PositionBadge(%v) ascii = %q, want %q", p, asc, p.String())
		}
		if uw, aw := lipgloss.Width(uni), lipgloss.Width(asc); uw != aw {
			t.Errorf("PositionBadge(%v): unicode %q is %d cells, ascii %q is %d cells",
				p, uni, uw, asc, aw)
		}
	}
	if b := PositionBadge(engine.Position(9)); b != "?" {
		t.Errorf("PositionBadge(invalid) = %q, want %q", b, "?")
	}
}

func TestUnicodeSupported(t *testing.T) {
	cases := []struct {
		lcAll, lcCtype, lang string
		want                 bool
	}{
		{"en_US.UTF-8", "", "", true},
		{"", "", "en_US.utf8", true},
		{"C", "", "en_US.UTF-8", false}, // LC_ALL wins
		{"", "POSIX", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		t.Setenv("LC_ALL", c.lcAll)
		t.Setenv("LC_CTYPE", c.lcCtype)
		t.Setenv("LANG", c.lang)
		if got := UnicodeSupported(); got != c.want {
			t.Errorf("UnicodeSupported() with LC_ALL=%q LC_CTYPE=%q LANG=%q = %v, want %v",
				c.lcAll, c.lcCtype, c.lang, got, c.want)
		}
	}
}

func TestInitSelectsGlyphSet(t *testing.T) {
	setGlyphs(t, Unicode()) // restore G after the test

	t.Setenv("LC_ALL", "C")
	t.Setenv("LC_CTYPE", "")
	t.Setenv("LANG", "")
	Init()
	if G != ASCII() {
		t.Error("Init() with a C locale should select the ASCII set")
	}

	t.Setenv("LC_ALL", "en_US.UTF-8")
	Init()
	if G != Unicode() {
		t.Error("Init() with a UTF-8 locale should select the Unicode set")
	}
}
