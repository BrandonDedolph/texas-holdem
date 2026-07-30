package app

import (
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/tutorial"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/components"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// refTab is one section of the quick reference.
type refTab int

// Quick-reference tabs. The order is the learning order: what beats what,
// where you sit, what a call must earn, then the words people use.
const (
	tabRankings refTab = iota
	tabPositions
	tabPotOdds
	tabGlossary
	tabCount // sentinel
)

func (t refTab) String() string {
	switch t {
	case tabRankings:
		return "Rankings"
	case tabPositions:
		return "Positions"
	case tabPotOdds:
		return "Pot Odds"
	case tabGlossary:
		return "Glossary"
	default:
		return "?"
	}
}

// QuickReference is the tabbed cheat sheet. It is a learning screen, not a
// rules dump: hand rankings render with real cards, and the pot-odds table
// is phrased as the decision a player actually faces ("they bet half pot —
// call if you win more than 25%").
//
// A panel taller than the content budget scrolls (the rankings tab's ten
// drawn hands are 30 rows); the status row announces cropped content with
// the same cue the lesson view uses, so nothing is ever silently cut.
type QuickReference struct {
	active refTab
	help   helpOverlay
	width  int
	height int

	scroll    int
	maxScroll int // set by the last render; scrolling is display-bounded
}

// NewQuickReference builds the reference on the rankings tab.
func NewQuickReference() *QuickReference {
	return &QuickReference{}
}

// Init implements tea.Model.
func (q *QuickReference) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (q *QuickReference) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		q.width, q.height = msg.Width, msg.Height
	case tea.KeyMsg:
		cmd, _ := q.help.dispatch(q.keymap(), msg, q.handleAction)
		return q, cmd
	}
	return q, nil
}

func (q *QuickReference) keymap() KeyMap { return quickRefKeys }

func (q *QuickReference) handleAction(a KeyAction) (tea.Cmd, bool) {
	switch a {
	case ActLeft:
		q.setTab((q.active + tabCount - 1) % tabCount)
		return nil, true
	case ActRight:
		q.setTab((q.active + 1) % tabCount)
		return nil, true
	case ActTab1:
		q.setTab(tabRankings)
		return nil, true
	case ActTab2:
		q.setTab(tabPositions)
		return nil, true
	case ActTab3:
		q.setTab(tabPotOdds)
		return nil, true
	case ActTab4:
		q.setTab(tabGlossary)
		return nil, true
	case ActUp:
		if q.scroll > 0 {
			q.scroll--
		}
		return nil, true
	case ActDown:
		if q.scroll < q.maxScroll {
			q.scroll++
		}
		return nil, true
	case ActBack:
		return Navigate(ScreenMainMenu), true
	}
	return nil, false
}

// setTab switches panels and rewinds the scroll — each tab starts at its top.
func (q *QuickReference) setTab(t refTab) {
	q.active = t
	q.scroll, q.maxScroll = 0, 0
}

// View implements tea.Model, in the shared chrome. The tab bar is the first
// content row; the panels underneath have different heights, so the
// shell's top-anchored content region keeps the tab bar's edge fixed as the
// player flips through them (the euchre QuickReference lesson).
func (q *QuickReference) View() string {
	w, h := fallbackSize(q.width, q.height)
	if q.help.open {
		return renderHelp("Quick Reference", q.keymap(), w, h)
	}

	var panel string
	switch q.active {
	case tabRankings:
		panel = q.renderRankings()
	case tabPositions:
		panel = q.renderPositions()
	case tabPotOdds:
		panel = q.renderPotOdds()
	case tabGlossary:
		panel = q.renderGlossary()
	}
	// Scrolling: the content budget is h-4 (the shell), minus the tab bar
	// and its blank line. A panel taller than that scrolls; maxScroll is
	// derived from what actually rendered, so the handler can never scroll
	// the content out of sight.
	lines := strings.Split(panel, "\n")
	avail := h - 6
	q.maxScroll = len(lines) - avail
	if q.maxScroll < 0 {
		q.maxScroll = 0
	}
	if q.scroll > q.maxScroll {
		q.scroll = q.maxScroll
	}
	panel = strings.Join(lines[q.scroll:], "\n")

	// The panel block keeps one width across tabs so the horizontal
	// centering cannot shift the left edge when the player switches.
	panel = padBlock(panel, quickRefPanelWidth)
	content := lipgloss.PlaceHorizontal(quickRefPanelWidth, lipgloss.Center, q.renderTabBar()) +
		"\n\n" + panel

	dot := " " + theme.G.Dot + " "
	footer := "left/right tabs" + dot + "1-4 jump" + dot + "esc back"
	if q.maxScroll > 0 {
		// The scroll keys displace the jump hint: the legend must fit the
		// 60-column compact floor, and the help overlay still documents 1-4.
		footer = "up/down scroll" + dot + "left/right tabs" + dot + "esc back"
	}
	return renderShell(w, h, shell{
		Title:  "Quick Reference",
		Status: q.statusText(),
		Footer: footer,
	}, content)
}

// statusText is the scroll cue when content is cropped, the tab summary
// otherwise — content the reader cannot see outranks anything else the
// line has to say (the lesson view's rule, applied here).
func (q *QuickReference) statusText() string {
	dot := " " + theme.G.Dot + " "
	switch {
	case q.maxScroll <= 0:
		return q.tabSummary()
	case q.scroll == 0:
		return "more below" + dot + "up/down to scroll"
	case q.scroll >= q.maxScroll:
		return "more above" + dot + "up/down to scroll"
	default:
		return "more above and below" + dot + "up/down to scroll"
	}
}

// quickRefPanelWidth is the fixed width every tab panel is padded to; the
// widest row across the four tabs stays under it, and it fits the
// 60-column compact floor.
const quickRefPanelWidth = 56

// tabSummary is the status-row caption for the active tab: what the sheet
// is for, in one line.
func (q *QuickReference) tabSummary() string {
	switch q.active {
	case tabRankings:
		return "Strongest hand first; suits never break ties"
	case tabPositions:
		return "Where you sit decides how many act after you"
	case tabPotOdds:
		return "Compare the price of a call with your chance to win"
	case tabGlossary:
		return "The words every table conversation assumes"
	default:
		return ""
	}
}

// renderTabBar draws the numbered tabs with the active one highlighted.
func (q *QuickReference) renderTabBar() string {
	th := theme.Current
	active := th.ActionKeycap.Reverse(true).Padding(0, 1)
	inactive := th.Help.Padding(0, 1)

	parts := make([]string, 0, int(tabCount))
	for t := refTab(0); t < tabCount; t++ {
		label := string(rune('1'+int(t))) + " " + t.String()
		if t == q.active {
			parts = append(parts, active.Render(label))
		} else {
			parts = append(parts, inactive.Render(label))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, parts...)
}

// rankingNotes is the one-phrase identity of each tier, indexed like
// tutorial.LadderRungs — the example hands themselves come from the ladder,
// so the reference and lesson 1 can never disagree on what illustrates what
// (the ladder's examples are verified against the evaluator by test).
var rankingNotes = [...]string{
	"the best hand",
	"run of one suit",
	"\"quads\"",
	"trips + a pair",
	"five of one suit",
	"five in a row",
	"\"trips\", \"a set\"",
	"top pair wins ties",
	"kickers break ties",
	"no made hand",
}

// renderRankings draws the ten hand categories, strongest first, each
// example hand DRAWN as five mini cards — the tier's name and note ride the
// rows beside its cards. Seeing "Full House" next to five real cards builds
// the pattern recognition an inline list of codes never will. Ten 3-row
// tiers make the panel 30 rows; the View scrolls it with a cue.
func (q *QuickReference) renderRankings() string {
	th := theme.Current
	var out []string
	// The tier's name heads its example hand rather than sitting beside it.
	// Five study cards plus a label and the longest tier name is about 61
	// cells, and this panel is a fixed 56 that must also survive the 60-col
	// floor - so the name goes above, which reads as a heading anyway.
	for i, r := range tutorial.LadderRungs() {
		num := padRow(itoa2(i+1)+".", 4)
		margin := strings.Repeat(" ", len(num))
		out = append(out, th.Body.Render(num)+th.Header.Render(r.Name)+
			"  "+th.Help.Render(rankingNotes[i]))
		for _, row := range strings.Split(components.FullCards(engine.MustCards(r.Cards)...), "\n") {
			out = append(out, margin+row)
		}
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

// itoa2 formats a small positive int, right-aligned to two cells so the
// rankings column of numbers lines up.
func itoa2(n int) string {
	if n < 10 {
		return " " + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// renderPositions draws the six 6-max positions in preflop action order,
// with the one-line strategic identity of each seat.
func (q *QuickReference) renderPositions() string {
	th := theme.Current
	rows := []struct {
		pos  engine.Position
		note string
	}{
		{engine.PosUTG, "acts first preflop; stay tight"},
		{engine.PosHJ, "one seat better; still tight"},
		{engine.PosCO, "opens wider; attacks the button"},
		{engine.PosBTN, "best seat; acts last postflop"},
		{engine.PosSB, "half a blind in; acts first later"},
		{engine.PosBB, "closes preflop; defends cheap"},
	}
	var b strings.Builder
	b.WriteString(th.Body.Render("Preflop action order (6-max):") + "\n")
	order := make([]string, len(rows))
	for i, r := range rows {
		order[i] = r.pos.String()
	}
	b.WriteString(th.Header.Render(strings.Join(order, " -> ")) + "\n\n")
	for _, r := range rows {
		b.WriteString(th.Header.Render(padRow(r.pos.String(), 5)) +
			th.Body.Render(padRow(r.pos.Long(), 15)) +
			th.Help.Render(r.note) + "\n")
	}
	b.WriteString("\n" + th.Body.Render("Postflop: blinds act first, button acts last."))
	return b.String()
}

// renderPotOdds draws the call-or-fold table a beginner can actually use at
// the table: for each common bet size, the odds offered and the break-even
// win percentage. Every number is derived from bet/(pot+2*bet) — nothing in
// this table is hand-tuned. See DESIGN.md §4: a wrong pot-odds figure is
// the most damaging possible bug in a teaching tool.
func (q *QuickReference) renderPotOdds() string {
	th := theme.Current
	rows := []struct {
		bet, odds, need string
	}{
		{"1/3 pot", "4.0 : 1", "20%"},
		{"1/2 pot", "3.0 : 1", "25%"},
		{"2/3 pot", "2.5 : 1", "29%"},
		{"3/4 pot", "2.3 : 1", "30%"},
		{"full pot", "2.0 : 1", "33%"},
		{"2x pot", "1.5 : 1", "40%"},
	}
	var b strings.Builder
	b.WriteString(th.Header.Render(padRow("They bet", 12)+padRow("You get", 11)+"Call if you win") + "\n")
	for _, r := range rows {
		b.WriteString(th.Body.Render(padRow(r.bet, 12)) +
			th.PotLine.Render(padRow(r.odds, 11)) +
			th.Body.Render("more than "+r.need) + "\n")
	}
	b.WriteString("\n" + th.Header.Render("Counting your chances") + "\n")
	b.WriteString(th.Body.Render("outs x 2 = your % to improve on the next card") + "\n")
	b.WriteString(th.Help.Render("(x 4 on the flop to see both turn and river)"))
	return b.String()
}

// renderGlossary defines the dozen words every table conversation assumes.
func (q *QuickReference) renderGlossary() string {
	th := theme.Current
	rows := []struct{ term, def string }{
		{"Blinds", "forced bets that start the action"},
		{"Button", "dealer seat; acts last postflop"},
		{"C-bet", "the preflop raiser bets again"},
		{"Check-raise", "check, then raise a bet behind you"},
		{"Equity", "your share of the pot right now"},
		{"Kicker", "side card that breaks ties"},
		{"Nuts", "the best possible hand right now"},
		{"Outs", "cards that improve you to the winner"},
		{"Pot odds", "price of a call vs. the pot"},
		{"Range", "every hand played this same way"},
		{"Value bet", "a bet that worse hands will call"},
	}
	var b strings.Builder
	for i, r := range rows {
		b.WriteString(th.Header.Render(padRow(r.term, 13)) + th.Body.Render(r.def))
		if i < len(rows)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}
