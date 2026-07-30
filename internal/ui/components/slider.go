package components

import (
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// Slider renders the bet-sizing readout as a single row of exactly width
// cells:
//
//	min 10 |------*--------------| all-in 900
//
// It is a pure function of the legal range and the current amount - the
// slider is a readout of the sizing value, not a widget with focus
// (docs/design-tui.md section 4.3). The value is clamped into [min, max];
// clamping semantics beyond display belong to the engine and action bar.
// When width leaves no room for the labels they are dropped and the track
// alone fills the row, so the readout degrades without ever wrapping.
func Slider(min, max, value engine.Chips, width int) string {
	if width <= 0 {
		return ""
	}
	if max < min {
		max = min
	}
	if value < min {
		value = min
	}
	if value > max {
		value = max
	}

	g := theme.G
	th := theme.Current

	left := "min " + min.String()
	right := "all-in " + max.String()
	trackWidth := width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if trackWidth < minTrackWidth {
		left, right = "", ""
		trackWidth = width
		if trackWidth < minTrackWidth {
			trackWidth = minTrackWidth // renderRow clips to width below
		}
	}

	// Knob position within the track interior, proportional to value.
	inner := trackWidth - 2
	if inner < 1 {
		inner = 1
	}
	pos := 0
	if span := int64(max - min); span > 0 && inner > 1 {
		pos = int((int64(value-min)*int64(inner-1) + span/2) / span)
	}

	segs := make([]seg, 0, 7)
	if left != "" {
		segs = append(segs, seg{left, th.ActionLabel}, seg{text: " "})
	}
	segs = append(segs,
		seg{g.SliderLeft + strings.Repeat(g.SliderTrack, pos), th.SizingSlider},
		seg{g.SliderKnob, th.SizingSlider.Bold(true)},
		seg{strings.Repeat(g.SliderTrack, inner-pos-1) + g.SliderRight, th.SizingSlider},
	)
	if right != "" {
		segs = append(segs, seg{text: " "}, seg{right, th.ActionLabel})
	}
	return renderRow(width, segs...)
}

// minTrackWidth is the smallest useful track: both caps plus the knob.
const minTrackWidth = 3
