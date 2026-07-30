package trainer

// The quizzes exist to drill card reading, so the cards must be pictures,
// not codes: every generated item carries the cards it asks about in its
// Visual, the two-hand comparisons label whose cards are whose, and no
// bare card code survives in a prompt (a range spec, which the level-3
// prompts legitimately quote, is colorized by the renderer instead).

import (
	"regexp"
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
)

// bareCode matches a card code standing alone as a word. Range-spec combos
// ("9c8c") are included on purpose: they are cards too, and belong to the
// renderer's colorizer, not to this exclusion.
var bareCode = regexp.MustCompile(`\b(?:10|[2-9AKQJT])[cdhs]\b`)

// TestItemsCarryTheirCardsAsVisuals: the cards an item asks about are in
// its Visual — a labelled showdown wherever two hands are compared — and
// the visual's cards are exactly the recorded generation inputs, so the
// picture and the verified answer can never disagree.
func TestItemsCarryTheirCardsAsVisuals(t *testing.T) {
	forEachGeneratedItem(t, QuizRankings, func(t *testing.T, it Item, desc string) {
		sd := it.Drill.Visual.Showdown
		if sd == nil {
			t.Fatalf("%s: rankings item without a showdown visual", desc)
		}
		if len(sd.Hands) != 2 || sd.Hands[0].Label != "Left" || sd.Hands[1].Label != "Right" {
			t.Fatalf("%s: showdown hands must be labelled Left and Right, got %+v", desc, sd.Hands)
		}
		if got := engine.CardsString(sd.Hands[0].Hole[:]); got != it.hero {
			t.Errorf("%s: Left shows %q, generated %q", desc, got, it.hero)
		}
		if got := engine.CardsString(sd.Hands[1].Hole[:]); got != it.villain {
			t.Errorf("%s: Right shows %q, generated %q", desc, got, it.villain)
		}
		if got := engine.CardsString(sd.Board); got != it.board {
			t.Errorf("%s: visual board %q, generated %q", desc, got, it.board)
		}
	})

	for _, kind := range []QuizKind{QuizOuts, QuizEquity} {
		forEachGeneratedItem(t, kind, func(t *testing.T, it Item, desc string) {
			v := it.Drill.Visual
			switch {
			case it.villainIsRange:
				// A range has no single hand to draw; the hero's draw is the
				// picture and the spec stays in the prompt.
				if v.Board == nil || !v.Board.ShowHole {
					t.Fatalf("%s: range item must show the hero's draw", desc)
				}
				if got := engine.CardsString(v.Board.Hole[:]); got != it.hero {
					t.Errorf("%s: hero shows %q, generated %q", desc, got, it.hero)
				}
			default:
				sd := v.Showdown
				if sd == nil {
					t.Fatalf("%s: known-villain item without a showdown visual", desc)
				}
				if len(sd.Hands) != 2 || sd.Hands[0].Label != "You" || sd.Hands[1].Label != "Villain" {
					t.Fatalf("%s: hands must be labelled You and Villain, got %+v", desc, sd.Hands)
				}
				if got := engine.CardsString(sd.Hands[0].Hole[:]); got != it.hero {
					t.Errorf("%s: You shows %q, generated %q", desc, got, it.hero)
				}
				if got := engine.CardsString(sd.Hands[1].Hole[:]); got != it.villain {
					t.Errorf("%s: Villain shows %q, generated %q", desc, got, it.villain)
				}
				if got := engine.CardsString(sd.Board); got != it.board {
					t.Errorf("%s: visual board %q, generated %q", desc, got, it.board)
				}
			}
		})
	}
}

// TestPromptsSpellNoBareCardCodes: prompts describe, visuals show. The one
// sanctioned exception is a range spec ("9c8c, Ac4c") in the level-3
// prompts — those pass through the renderer's colorizer, which this
// package-level test approximates by stripping the specs first.
func TestPromptsSpellNoBareCardCodes(t *testing.T) {
	for kind := QuizKind(0); kind < NumQuizKinds; kind++ {
		forEachGeneratedItem(t, kind, func(t *testing.T, it Item, desc string) {
			prompt := it.Drill.Prompt
			if it.villainIsRange {
				// The spec is quoted verbatim by design; everything around
				// it must still be code-free.
				prompt = regexp.MustCompile(`\(.*\)|range: .*\.`).ReplaceAllString(prompt, "")
			}
			if m := bareCode.FindString(prompt); m != "" {
				t.Errorf("%s: prompt spells bare card code %q: %q", desc, m, it.Drill.Prompt)
			}
			// Choice labels never carry codes either.
			if choice, ok := it.Drill.Answer.(tutorial.ChoiceAnswer); ok {
				for _, c := range choice.Choices {
					if m := bareCode.FindString(c); m != "" {
						t.Errorf("%s: choice %q spells bare card code %q", desc, c, m)
					}
				}
			}
		})
	}
}
