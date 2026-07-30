package app

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
)

// The "cards are drawn, not spelled" guard (the owner's rule: if the app is
// talking about specific cards or a specific hand, there are cards on the
// TUI). Two rendered-output smells are banned in every lesson frame and
// every quick-reference frame:
//
//  1. a bare ASCII card code ("As", "Td") — a card the renderer never drew;
//  2. an inline-spelled HAND: three or more colorized cards in a row on one
//     line ("A♠ K♠ Q♠ J♠ 10♠"). One or two inline cards remain legal prose
//     ("your K♦ outkicks the Q♥") because they reference cards drawn nearby;
//     a run is a hand, and hands are drawn as card frames.
//
// The checks run against rendered frames under the Unicode glyph set, where
// drawn and inline cards carry suit glyphs (never a code letter) and frame
// glyphs break up card faces, so neither pattern can false-positive on a
// properly drawn card. A future lesson, drill or reference tab that spells
// a hand inline fails here no matter which section type carries it.

// inlineGlyphCard matches one colorized card in Unicode rendered output.
const inlineGlyphCard = `(?:10|[2-9AKQJ])[♠♥♦♣]`

var inlineHandRunRE = regexp.MustCompile(inlineGlyphCard + `(?: ` + inlineGlyphCard + `){2,}`)

// forEachLessonFrame renders every frame a learner can see in every lesson
// at one size — each section at every scroll position, each drill in its
// question and answered states, each scripted section's intro — and hands
// the stripped frame to check.
func forEachLessonFrame(t *testing.T, w, h int, check func(id string, section int, state, frame string)) {
	t.Helper()
	prof := profile.NewProfile()
	for _, l := range tutorial.All() {
		prof.CompleteLesson(l.ID)
	}
	for _, lesson := range tutorial.All() {
		v := newLessonView(lesson, prof)
		for i := range lesson.Sections {
			v.enterSection(i)
			check(lesson.ID, i+1, "view", stripANSI(v.render(w, h)))
			for v.scroll < v.maxScroll {
				v.scroll++
				check(lesson.ID, i+1, "scrolled", stripANSI(v.render(w, h)))
			}
			if d := lesson.Sections[i].Drill; d != nil {
				v.drill.typed = typedFor(d)
				v.handleAction(ActSelect)
				check(lesson.ID, i+1, "answered", stripANSI(v.render(w, h)))
			}
		}
	}
}

// forEachQuickRefFrame renders every quick-reference tab at every scroll
// position at one size.
func forEachQuickRefFrame(t *testing.T, w, h int, check func(tab, frame string)) {
	t.Helper()
	q := NewQuickReference()
	q.Update(tea.WindowSizeMsg{Width: w, Height: h})
	for tab := refTab(0); tab < tabCount; tab++ {
		q.setTab(tab)
		check(tab.String(), stripANSI(q.View()))
		for q.scroll < q.maxScroll {
			q.handleAction(ActDown)
			check(tab.String(), stripANSI(q.View()))
		}
	}
}

// TestNoBareCodesOrInlineHandsAnywhere sweeps every lesson frame and every
// quick-reference frame at all three breakpoints.
func TestNoBareCodesOrInlineHandsAnywhere(t *testing.T) {
	restoreGlyphs(t)
	theme.G = theme.Unicode()
	assertFrame := func(where, frame string) {
		if m := bareCardCode.FindString(frame); m != "" {
			t.Errorf("%s: bare card code %q survives in rendered output", where, m)
		}
		if m := inlineHandRunRE.FindString(frame); m != "" {
			t.Errorf("%s: hand spelled inline as %q; a named hand must be drawn as cards", where, m)
		}
	}
	for _, size := range []struct{ w, h int }{{80, 24}, {104, 30}, {60, 20}} {
		forEachLessonFrame(t, size.w, size.h, func(id string, section int, state, frame string) {
			assertFrame(sizeLabel(size.w, size.h)+" "+id+" section "+strconv.Itoa(section)+" ("+state+")", frame)
		})
		forEachQuickRefFrame(t, size.w, size.h, func(tab, frame string) {
			assertFrame(sizeLabel(size.w, size.h)+" quick-reference "+tab, frame)
		})
	}
}

// TestHandLadderReachableAtAllBreakpoints: lesson 1's ladder section shows
// all ten tiers as drawn cards at every breakpoint — directly, or through
// announced scrolling. A tier a learner cannot reach is a rung the ladder
// does not have.
func TestHandLadderReachableAtAllBreakpoints(t *testing.T) {
	restoreGlyphs(t)
	theme.G = theme.Unicode()
	lesson, ok := tutorial.Get("hand-rankings")
	if !ok {
		t.Fatal("hand-rankings not registered")
	}
	li := sectionIndex(lesson, func(s *tutorial.Section) bool {
		return s.Visual != nil && s.Visual.HandLadder != nil
	})
	if li < 0 {
		t.Fatal("lesson 1 must keep its hand-ladder section")
	}
	prof := profile.NewProfile()

	for _, size := range []struct{ w, h int }{{80, 24}, {104, 30}, {60, 20}} {
		v := newLessonView(lesson, prof)
		v.enterSection(li)
		var seen strings.Builder
		seen.WriteString(stripANSI(v.render(size.w, size.h)))
		if v.maxScroll > 0 && !strings.Contains(seen.String(), "more below") {
			t.Errorf("%dx%d: cropped ladder must announce itself", size.w, size.h)
		}
		for v.scroll < v.maxScroll {
			v.scroll++
			seen.WriteString("\n" + stripANSI(v.render(size.w, size.h)))
		}
		union := seen.String()
		for i, r := range tutorial.LadderRungs() {
			if !strings.Contains(union, r.Name) {
				t.Errorf("%dx%d: tier %d (%s) never becomes visible", size.w, size.h, i+1, r.Name)
			}
		}
		// Tiers are drawn: the union holds mini-card frames, not inline rows.
		if got := strings.Count(union, theme.G.CardTL); got < 50 {
			t.Errorf("%dx%d: only %d card frames seen across the ladder, want >= 50", size.w, size.h, got)
		}
	}
}

// TestOrderDrillsDrawCards: every order-these-hands drill draws each option
// as card frames — the learner ranks cards they can see, not codes.
func TestOrderDrillsDrawCards(t *testing.T) {
	restoreGlyphs(t)
	theme.G = theme.Unicode()
	prof := profile.NewProfile()
	found := 0
	for _, lesson := range tutorial.All() {
		v := newLessonView(lesson, prof)
		for i := range lesson.Sections {
			d := lesson.Sections[i].Drill
			if d == nil {
				continue
			}
			ans, ok := d.Answer.(tutorial.OrderAnswer)
			if !ok {
				continue
			}
			hands, ok := orderItemsAsCards(ans.Items)
			if !ok {
				continue
			}
			found++
			total := 0
			for _, hand := range hands {
				total += len(hand)
			}
			v.enterSection(i)
			frame := stripANSI(v.render(80, 24))
			if got := strings.Count(frame, theme.G.CardTL); got < total {
				t.Errorf("%s section %d: %d card frames for %d option cards",
					lesson.ID, i+1, got, total)
			}
		}
	}
	if found == 0 {
		t.Fatal("the curriculum ships at least one order-these-hands drill")
	}
}

// TestScriptedIntrosDrawTheHeroHand: an intro that precedes the deal shows
// the hero's holding as drawn cards, and the whole intro fits the content
// budget (the intro cannot scroll, so it must never crop).
func TestScriptedIntrosDrawTheHeroHand(t *testing.T) {
	restoreGlyphs(t)
	theme.G = theme.Unicode()
	prof := profile.NewProfile()
	for _, size := range []struct{ w, h int }{{80, 24}, {104, 30}, {60, 20}} {
		budget := size.h - 4
		compact := layoutFor(size.w, size.h) == LayoutCompact
		for _, lesson := range tutorial.All() {
			v := newLessonView(lesson, prof)
			for i := range lesson.Sections {
				s := lesson.Sections[i].Script
				if s == nil {
					continue
				}
				v.enterSection(i)
				body := v.renderScript(size.w, budget, compact)
				if len(body) > budget {
					t.Errorf("%dx%d %s section %d intro: %d rows exceed the %d-row budget",
						size.w, size.h, lesson.ID, i+1, len(body), budget)
				}
				if _, ok := s.Holes[s.Hero]; !ok {
					continue
				}
				frame := stripANSI(strings.Join(body, "\n"))
				if strings.Count(frame, theme.G.CardTL) < 2 {
					t.Errorf("%dx%d %s section %d: intro does not draw the hero's two cards",
						size.w, size.h, lesson.ID, i+1)
				}
			}
		}
	}
}

// TestQuickRefRankingsDrawCards: the rankings tab draws all ten example
// hands as card frames, reachable through announced scrolling.
func TestQuickRefRankingsDrawCards(t *testing.T) {
	restoreGlyphs(t)
	theme.G = theme.Unicode()
	for _, size := range []struct{ w, h int }{{80, 24}, {104, 30}, {60, 20}} {
		q := NewQuickReference()
		q.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})
		q.setTab(tabRankings)
		var union strings.Builder
		frame := stripANSI(q.View())
		union.WriteString(frame)
		if q.maxScroll > 0 && !strings.Contains(frame, "more below") {
			t.Errorf("%dx%d: cropped rankings must announce themselves", size.w, size.h)
		}
		for q.scroll < q.maxScroll {
			q.handleAction(ActDown)
			union.WriteString("\n" + stripANSI(q.View()))
		}
		for _, r := range tutorial.LadderRungs() {
			if !strings.Contains(union.String(), r.Name) {
				t.Errorf("%dx%d: ranking %q never becomes visible", size.w, size.h, r.Name)
			}
		}
		if got := strings.Count(union.String(), theme.G.CardTL); got < 50 {
			t.Errorf("%dx%d: only %d card frames seen across the rankings, want >= 50", size.w, size.h, got)
		}
	}
}

// restoreGlyphs resets the active glyph set when the test finishes.
func restoreGlyphs(t *testing.T) {
	t.Helper()
	old := theme.G
	t.Cleanup(func() { theme.G = old })
}

// sizeLabel formats a breakpoint for failure messages.
func sizeLabel(w, h int) string { return strconv.Itoa(w) + "x" + strconv.Itoa(h) }
