package components

import (
	"strings"
	"testing"
)

func TestPotLineSinglePot(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		got := PotLine([]PotView{{Amount: 185}}, 80)
		assertBlock(t, got, 80, 1)
		if !strings.Contains(got, "POT 185") {
			t.Errorf("PotLine = %q, want it to contain %q", got, "POT 185")
		}
		if strings.Contains(got, "MAIN") {
			t.Errorf("a single pot is POT, never MAIN: %q", got)
		}
	})
}

func TestPotLineTwoPots(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		pots := []PotView{
			{Amount: 900},
			{Amount: 710, Eligible: []string{"Nia", "you"}},
		}
		got := PotLine(pots, 80)
		assertBlock(t, got, 80, 1)
		for _, want := range []string{"MAIN 900", "SIDE 710", "(Nia", "you)"} {
			if !strings.Contains(got, want) {
				t.Errorf("PotLine = %q, want it to contain %q", got, want)
			}
		}
	})
}

func TestPotLineThreePots(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		pots := []PotView{
			{Amount: 900},
			{Amount: 710, Eligible: []string{"Nia", "you"}},
			{Amount: 40, Eligible: []string{"you"}},
		}
		got := PotLine(pots, 80)
		assertBlock(t, got, 80, 1)
		for _, want := range []string{"MAIN 900", "SIDE 710", "SIDE 2 40"} {
			if !strings.Contains(got, want) {
				t.Errorf("PotLine = %q, want it to contain %q", got, want)
			}
		}
	})
}

func TestPotLineEmpty(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		got := PotLine(nil, 40)
		assertBlock(t, got, 40, 1)
		if strings.TrimSpace(got) != "" {
			t.Errorf("no pots should render a blank fixed-width row, got %q", got)
		}
	})
}

func TestPotLineTruncatesLongEligibleList(t *testing.T) {
	glyphSets(t, func(t *testing.T) {
		pots := []PotView{
			{Amount: 900},
			{Amount: 710, Eligible: []string{
				"Bartholomew Kuznetsov III",
				"Maximiliana von Hohenzollern",
				"a third player with a very long name indeed",
			}},
		}
		for _, width := range []int{80, 40, 24, 10} {
			got := PotLine(pots, width)
			assertBlock(t, got, width, 1)
		}
	})
}

func TestPotLineWidthStableAcrossStates(t *testing.T) {
	// The pot region is fixed-size through every state mutation: growing
	// from one pot to three never changes the row's dimensions.
	glyphSets(t, func(t *testing.T) {
		const width = 80
		states := [][]PotView{
			nil,
			{{Amount: 45}},
			{{Amount: 900}, {Amount: 710, Eligible: []string{"Nia", "you"}}},
			{{Amount: 900}, {Amount: 710}, {Amount: 40}},
		}
		for _, pots := range states {
			assertBlock(t, PotLine(pots, width), width, 1)
		}
	})
}
