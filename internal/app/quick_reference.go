package app

import (
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
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
type QuickReference struct {
	active refTab
	help   helpOverlay
	width  int
	height int
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
		q.active = (q.active + tabCount - 1) % tabCount
		return nil, true
	case ActRight:
		q.active = (q.active + 1) % tabCount
		return nil, true
	case ActTab1:
		q.active = tabRankings
		return nil, true
	case ActTab2:
		q.active = tabPositions
		return nil, true
	case ActTab3:
		q.active = tabPotOdds
		return nil, true
	case ActTab4:
		q.active = tabGlossary
		return nil, true
	case ActBack:
		return Navigate(ScreenMainMenu), true
	}
	return nil, false
}

// View implements tea.Model. Fixed vertical budget: one title row, one tab
// row, the content area, one footer row — the content area is padded and
// cropped to exactly its budget so switching tabs never moves the chrome.
func (q *QuickReference) View() string {
	w, h := fallbackSize(q.width, q.height)
	if q.help.open {
		return renderHelp("Quick Reference", q.keymap(), w, h)
	}
	th := theme.Current

	title := lipgloss.PlaceHorizontal(w, lipgloss.Center, th.Header.Render("Quick Reference"))
	tabs := lipgloss.PlaceHorizontal(w, lipgloss.Center, q.renderTabBar())
	footer := lipgloss.PlaceHorizontal(w, lipgloss.Center, th.Help.Render(
		"left/right tabs "+theme.G.Dot+" 1-4 jump "+theme.G.Dot+" esc back "+theme.G.Dot+" ? help"))

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
	box := th.ContentBox.Padding(0, 1).Render(panel)

	// Top-anchored, not centered: tabs have different content heights, and
	// top-anchoring keeps the box's top edge fixed as the player flips
	// through them (the euchre QuickReference lesson).
	contentHeight := h - 3
	content := padHeight(
		lipgloss.Place(w, contentHeight, lipgloss.Center, lipgloss.Top, box),
		contentHeight)

	return title + "\n" + tabs + "\n" + content + "\n" + footer
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

// inlineCards renders a card list ("As Ks Qs") through components.InlineCard
// so reference examples look exactly like cards on the table. The card
// strings live in content as text per the repo convention — cards in lesson
// content are written "As Kd", never raw values.
func inlineCards(cards string) string {
	parsed, err := engine.ParseCards(cards)
	if err != nil {
		// Reference content is static; a typo here is a programmer error
		// caught the first time any test renders the panel.
		panic("quick_reference: bad card list " + cards + ": " + err.Error())
	}
	out := make([]string, len(parsed))
	for i, c := range parsed {
		out[i] = components.InlineCard(c)
	}
	return strings.Join(out, " ")
}

// renderRankings draws the ten hand categories, strongest first, each with
// a real example hand — reading "full house" next to J♠ J♦ J♥ 4♣ 4♦ builds
// the pattern recognition a text list never will.
func (q *QuickReference) renderRankings() string {
	th := theme.Current
	rows := []struct {
		name  string
		cards string
		note  string
	}{
		{"Royal Flush", "As Ks Qs Js Ts", "the best hand"},
		{"Straight Flush", "9h 8h 7h 6h 5h", "run of one suit"},
		{"Four of a Kind", "Qc Qd Qh Qs 7d", "\"quads\""},
		{"Full House", "Js Jd Jh 4c 4d", "trips + a pair"},
		{"Flush", "Ad Jd 8d 6d 2d", "five of one suit"},
		{"Straight", "8c 7d 6s 5h 4c", "five in a row"},
		{"Three of a Kind", "9s 9h 9c Kd 2s", "\"trips\", \"a set\""},
		{"Two Pair", "Ah Ac 8d 8s Qc", "top pair wins ties"},
		{"One Pair", "Td Tc As 7h 3c", "kickers break ties"},
		{"High Card", "As Qd 9c 6h 2s", "no made hand"},
	}
	var b strings.Builder
	for i, r := range rows {
		num := padRow(itoa2(i+1)+".", 4)
		name := padRow(r.name, 16)
		b.WriteString(th.Body.Render(num) + th.Header.Render(name) +
			inlineCards(r.cards) + "  " + th.Help.Render(r.note))
		if i < len(rows)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
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
