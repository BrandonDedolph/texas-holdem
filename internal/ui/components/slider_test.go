package components

import (
	"strings"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
)

func TestSliderExactWidth(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		for _, width := range []int{80, 60, 40, 20, 12, 5, 3, 1} {
			got := Slider(10, 900, 120, width)
			assertBlock(t, got, width, 1)
		}
	})
}

func TestSliderContainsLabels(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		got := Slider(10, 900, 120, 60)
		for _, want := range []string{"min 10", "all-in 900"} {
			if !strings.Contains(got, want) {
				t.Errorf("Slider = %q, want it to contain %q", got, want)
			}
		}
	})
}

func TestSliderKnobAtExtremes(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		g := theme.G
		atMin := Slider(10, 900, 10, 60)
		if !strings.Contains(atMin, g.SliderLeft+g.SliderKnob) {
			t.Errorf("value=min should pin the knob to the left cap: %q", atMin)
		}
		atMax := Slider(10, 900, 900, 60)
		if !strings.Contains(atMax, g.SliderKnob+g.SliderRight) {
			t.Errorf("value=max should pin the knob to the right cap: %q", atMax)
		}
	})
}

func TestSliderKnobMovesWithValue(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		knob := theme.G.SliderKnob
		low := strings.Index(Slider(10, 900, 100, 60), knob)
		high := strings.Index(Slider(10, 900, 800, 60), knob)
		if low < 0 || high < 0 {
			t.Fatal("knob glyph missing from slider")
		}
		if low >= high {
			t.Errorf("knob should move right as the value grows: index %d then %d", low, high)
		}
	})
}

func TestSliderClampsValue(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		if Slider(10, 900, 5000, 60) != Slider(10, 900, 900, 60) {
			t.Error("a value above max should render as max")
		}
		if Slider(10, 900, -3, 60) != Slider(10, 900, 10, 60) {
			t.Error("a value below min should render as min")
		}
	})
}

func TestSliderDegenerateRange(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		// All-in for exactly the minimum: a one-point range still renders.
		got := Slider(285, 285, 285, 40)
		assertBlock(t, got, 40, 1)
		if !strings.Contains(got, theme.G.SliderKnob) {
			t.Errorf("degenerate range should still show the knob: %q", got)
		}
		// A max below min is treated as an empty range, not a panic.
		assertBlock(t, Slider(engine.Chips(100), engine.Chips(50), 75, 40), 40, 1)
	})
}

func TestSliderDropsLabelsWhenCramped(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		// Too narrow for "min 10 ... all-in 900": the track alone fills
		// the row rather than wrapping or overflowing.
		got := Slider(10, 900, 120, 12)
		assertBlock(t, got, 12, 1)
		if strings.Contains(got, "min") || strings.Contains(got, "all-in") {
			t.Errorf("cramped slider should drop its labels: %q", got)
		}
	})
}
