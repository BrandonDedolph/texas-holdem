package app

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
)

// Golden renders (docs/design-tui.md §8.3): full frames of the table at
// every breakpoint, reached through the engine via scripted replays at
// SpeedInstant. Views are compared ANSI-stripped and right-trimmed —
// goldens pin geometry and text, not color; color correctness belongs to
// the theme unit tests. A golden diff IS the design review, so the files
// are meant to be read.
//
// Refresh with: go test ./internal/app -run TestGolden -update

var update = flag.Bool("update", false, "rewrite golden files")

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

// stripANSI removes SGR escape sequences so tests compare what a player
// sees, not how it is colored.
func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

// normalizeView strips ANSI and trailing whitespace per line. Trailing
// spaces are real in the render (regions pad to full width) but invisible
// in a file, and an invisible diff would make golden review worthless.
func normalizeView(s string) string {
	lines := strings.Split(stripANSI(s), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n") + "\n"
}

// assertGolden compares got against testdata/<name>.golden, reporting the
// first mismatching row and column — "the whole blob differs" is useless
// output for a 24-line frame.
func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s (run with -update to create): %v", path, err)
	}
	want := string(wantBytes)
	if got == want {
		return
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; ; i++ {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g == w {
			continue
		}
		col := firstRuneDiff(g, w)
		t.Fatalf("golden %s: first mismatch at row %d col %d\n got: %q\nwant: %q",
			name, i+1, col+1, g, w)
	}
}

// firstRuneDiff returns the index of the first differing rune.
func firstRuneDiff(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	for i := 0; i < len(ra) && i < len(rb); i++ {
		if ra[i] != rb[i] {
			return i
		}
	}
	if len(ra) < len(rb) {
		return len(ra)
	}
	return len(rb)
}

// goldenSizes are the three breakpoints every scenario is captured at.
var goldenSizes = []struct{ w, h int }{
	{80, 24}, {104, 30}, {60, 20},
}

func TestGoldenTable(t *testing.T) {
	scenarios := []struct {
		name  string
		build func() tableScenario
	}{
		{"preflop", scenarioPreflop},
		{"flop_sizing", scenarioFlopSizing},
		{"sidepot_allin", scenarioSidePot},
		{"showdown", scenarioShowdown},
		{"between_hands", scenarioBetweenHands},
	}
	for _, sc := range scenarios {
		for _, sz := range goldenSizes {
			name := fmt.Sprintf("table_%s_%dx%d", sc.name, sz.w, sz.h)
			t.Run(name, func(t *testing.T) {
				m := buildTable(t, sc.build(), sz.w, sz.h)
				assertGolden(t, name, normalizeView(m.View()))
			})
		}
	}
}

// TestGoldenTooSmall pins the hard floor screen.
func TestGoldenTooSmall(t *testing.T) {
	assertGolden(t, "too_small_52x18", normalizeView(renderTooSmall(52, 18)))
}

// TestGoldenTableHelp pins the table's help overlay (choosing state), which
// doubles as a readable record of the §4.2 binding table.
func TestGoldenTableHelp(t *testing.T) {
	m := buildTable(t, scenarioPreflop(), 80, 24)
	m.Update(key("?"))
	assertGolden(t, "table_help_80x24", normalizeView(m.View()))
}

// TestGoldenTableASCII forces the ASCII glyph set: every Unicode glyph has
// a same-width substitute, so the frame's geometry must be identical to the
// Unicode golden — only the glyphs differ.
func TestGoldenTableASCII(t *testing.T) {
	old := theme.G
	defer func() { theme.G = old }()
	theme.G = theme.ASCII()

	m := buildTable(t, scenarioFlopSizing(), 80, 24)
	assertGolden(t, "table_flop_sizing_ascii_80x24", normalizeView(m.View()))
}
