package theme

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

// uiSourceFiles returns every non-test Go file under internal/ui.
func uiSourceFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walking internal/ui: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("found no Go files under internal/ui")
	}
	return files
}

// TestNoHexLiteralsOutsidePalette enforces the theming convention: hex color
// literals appear only in theme/palette.go, so re-theming is a one-file job
// and no component can drift into inline colors.
func TestNoHexLiteralsOutsidePalette(t *testing.T) {
	hex := regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b`)
	for _, path := range uiSourceFiles(t) {
		if filepath.Base(path) == "palette.go" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if m := hex.Find(data); m != nil {
			t.Errorf("%s: hex color literal %q outside palette.go", path, m)
		}
	}
}

// TestNoRawGlyphsOutsideGlyphs enforces the fallback convention: raw
// non-ASCII glyphs appear only in theme/glyphs.go, so every rendered symbol
// is guaranteed to have a same-width ASCII substitute.
func TestNoRawGlyphsOutsideGlyphs(t *testing.T) {
	for _, path := range uiSourceFiles(t) {
		if filepath.Base(path) == "glyphs.go" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			for _, r := range line {
				if r > unicode.MaxASCII {
					t.Errorf("%s:%d: raw non-ASCII glyph %q outside glyphs.go", path, i+1, r)
					break
				}
			}
		}
	}
}
