package app

import (
	"strconv"
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/coach"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	"github.com/charmbracelet/lipgloss"
)

// Teachable-moment and coach-expansion popups (docs/design-tui.md §5.1,
// §5.4) — euchre's tutorial_popups.go pattern: a modal card over the frozen
// table, any key closes. The table owns exactly one overlay at a time; both
// producers (coach.PendingMoment at armBar, the e key on the coach strip)
// go through the same tableOverlay so dismissal, rendering and the
// keymap-freeze behave identically.
//
// Firing rules for moments live in the coach package (fire once per player
// ever, at most one per decision, persisted to profile.MomentsSeen); the
// table adds the two UI-side guarantees: a moment is consulted only when
// the hero's turn arms — never during a villain's turn — and never at
// CoachOff, where an unfired moment is not burned and simply recurs.

// tableOverlay is one modal card: a title, body paragraphs, and optional
// pre-formatted number lines. Content is stored plain and wrapped at render
// time, so a live resize re-fits the card like everything else.
type tableOverlay struct {
	title string
	body  string
	extra []string // one line each, already formatted ("Pot odds  1.5:1 (40%)")

	// footer overrides the dismissal hint ("any key continues" when "").
	footer string

	// dossier marks the seat-dossier card, whose seat lets the o key walk
	// to the next opponent instead of dismissing (Update's overlay branch).
	dossier bool
	seat    engine.Seat
}

// momentOverlay builds the card for a teachable moment. The body is
// rendered immediately — it may cite live cards and numbers from the spot,
// and the spot moves on after the decision.
func momentOverlay(mo *coach.Moment, v *engine.PlayerView, adv coach.Advice) *tableOverlay {
	return &tableOverlay{title: mo.Title, body: mo.Body(v, adv)}
}

// adviceOverlay builds the card for the coach's expanded explanation: the
// full reasoning the strip may have had to clip, plus the number chips.
func adviceOverlay(adv coach.Advice) *tableOverlay {
	o := &tableOverlay{title: "COACH " + theme.G.Dot + " " + adv.Headline, body: adv.Body}
	for _, c := range adv.Numbers {
		o.extra = append(o.extra, c.Label+"  "+c.Value)
	}
	return o
}

// dossierOverlay builds the seat dossier (the o key): the archetype's
// label and blurb — the claim — above the seat's observed session numbers —
// the evidence. The stats come from watching the seat's actions in the
// event log (observeAction), never from asking the AI its dials: a dossier
// built from observed play is honest even for a human opponent, which is
// the method lesson 12 teaches.
func (m *TableScreen) dossierOverlay(s engine.Seat) *tableOverlay {
	p, ok := ai.Archetype(m.archKeys[s])
	if !ok {
		// Unknown lineup keys seat the baseline opponent (newArchetypeOpponent);
		// the dossier describes what actually sat down.
		p = ai.Baseline()
	}
	o := &tableOverlay{
		dossier: true,
		seat:    s,
		title:   m.seatName(s) + " " + theme.G.Dot + " " + p.Label,
		body:    p.Blurb,
		footer:  "o next opponent " + theme.G.Dot + " any key continues",
	}
	o.extra = append(o.extra, "read      \""+p.Read+"\"")
	if n, ok := opponentNotes[p.Key]; ok {
		o.extra = append(o.extra, "adjust    "+n.adjust)
	}
	o.extra = append(o.extra, "")
	st := m.stats[s]
	if st.Hands == 0 {
		o.extra = append(o.extra, "observed  no completed hands yet")
	} else {
		hands := strconv.Itoa(st.Hands) + " hands"
		if st.Hands == 1 {
			hands = "1 hand"
		}
		o.extra = append(o.extra,
			"observed  "+hands+" this session",
			"VPIP      "+pctFrac(st.VPIP, st.Hands)+" entered the pot preflop",
			"PFR       "+pctFrac(st.PFR, st.Hands)+" raised preflop",
			"postflop  "+strconv.Itoa(st.Aggro)+" bets+raises, "+
				strconv.Itoa(st.Calls)+" calls (agg "+aggFactor(st.Aggro, st.Calls)+")",
		)
	}
	o.extra = append(o.extra, "",
		"the read is a claim; the numbers are the evidence")
	return o
}

// pctFrac renders an observed frequency with its evidence: "42% (5/12)".
func pctFrac(n, hands int) string {
	return strconv.Itoa((n*100+hands/2)/hands) + "% (" +
		strconv.Itoa(n) + "/" + strconv.Itoa(hands) + ")"
}

// aggFactor is the postflop aggression factor — bets+raises per call.
// "inf" for a seat that has never called, "-" before any postflop action.
func aggFactor(aggro, calls int) string {
	switch {
	case calls > 0:
		return strconv.FormatFloat(float64(aggro)/float64(calls), 'f', 1, 64)
	case aggro > 0:
		return "inf"
	default:
		return "-"
	}
}

// renderTableOverlay draws the card centered over the frozen table frame.
// Rows the card occupies are replaced whole (the card floats on its own
// full-width band) — splicing a styled row mid-escape would corrupt it, and
// the frame's height must not change, so the band never grows the view.
func renderTableOverlay(base string, w, h int, o *tableOverlay) string {
	th := theme.Current

	boxW := 64
	if boxW > w-6 {
		boxW = w - 6
	}
	inner := boxW - 4 // rounded border + one cell padding each side

	var lines []string
	lines = append(lines, th.CoachTitle.Render(clip(o.title, inner)), "")
	for _, l := range wrapPlain(o.body, inner) {
		lines = append(lines, th.Body.Render(l))
	}
	if len(o.extra) > 0 {
		lines = append(lines, "")
		for _, e := range o.extra {
			lines = append(lines, th.Body.Render(clip(e, inner)))
		}
	}
	dismiss := o.footer
	if dismiss == "" {
		dismiss = "any key continues"
	}
	lines = append(lines, "", th.Help.Render(clip(dismiss, inner)))
	// The card must fit the frame with its border whole; a compact terminal
	// crops the body, never the frame.
	if max := h - 4; len(lines) > max && max > 0 {
		lines = lines[:max]
	}
	for i, l := range lines {
		lines[i] = padStyledTo(l, inner)
	}
	box := th.CoachBox.Render(strings.Join(lines, "\n"))

	rows := strings.Split(base, "\n")
	boxRows := strings.Split(box, "\n")
	start := (len(rows) - len(boxRows)) / 2
	if start < 0 {
		start = 0
	}
	for i, br := range boxRows {
		if start+i >= len(rows) {
			break
		}
		pad := (w - lipgloss.Width(br)) / 2
		if pad < 0 {
			pad = 0
		}
		rows[start+i] = padStyledTo(strings.Repeat(" ", pad)+br, w)
	}
	return strings.Join(rows, "\n")
}
