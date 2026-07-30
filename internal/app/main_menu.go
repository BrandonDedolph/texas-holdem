package app

import (
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MainMenu is the entry screen: Play / Lessons / Trainer / Quick Reference /
// Settings / Quit.
type MainMenu struct {
	list   *formList
	help   helpOverlay
	width  int
	height int
}

// Main menu rows, in list order. Indexed by the constants below so
// handleSelect cannot drift from the row slice.
const (
	menuPlay = iota
	menuLessons
	menuTrainer
	menuQuickRef
	menuSettings
	menuQuit
)

// NewMainMenu builds the main menu.
func NewMainMenu() *MainMenu {
	rows := []listRow{
		{Label: "Play", Detail: "6-max cash game against five AI opponents"},
		{Label: "Lessons", Detail: "Guided lessons from rankings to bet sizing"},
		{Label: "Trainer", Detail: "Drills: rank hands, count outs, judge equity"},
		{Label: "Quick Reference", Detail: "Rankings, positions, pot odds and glossary"},
		{Label: "Settings", Detail: "Theme, deck colors, speed, coach verbosity"},
		{Label: "Quit", Detail: "Leave the table"},
	}
	return &MainMenu{list: newFormList(rows)}
}

// Init implements tea.Model.
func (m *MainMenu) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *MainMenu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		cmd, _ := m.help.dispatch(m.keymap(), msg, m.handleAction)
		return m, cmd
	}
	return m, nil
}

func (m *MainMenu) keymap() KeyMap { return mainMenuKeys }

func (m *MainMenu) handleAction(a KeyAction) (tea.Cmd, bool) {
	switch a {
	case ActUp:
		m.list.MoveUp()
		return nil, true
	case ActDown:
		m.list.MoveDown()
		return nil, true
	case ActSelect:
		return m.handleSelect(), true
	case ActBack:
		return Quit(), true
	}
	return nil, false
}

// handleSelect routes the selected row.
func (m *MainMenu) handleSelect() tea.Cmd {
	if r := m.list.Row(); r == nil || r.Disabled {
		return nil
	}
	switch m.list.Cursor() {
	case menuPlay:
		return Navigate(ScreenGameSetup)
	case menuLessons:
		return Navigate(ScreenLessons)
	case menuTrainer:
		return Navigate(ScreenTrainer)
	case menuQuickRef:
		return Navigate(ScreenQuickReference)
	case menuSettings:
		return Navigate(ScreenSettings)
	case menuQuit:
		return Quit()
	}
	return nil
}

// View implements tea.Model.
func (m *MainMenu) View() string {
	w, h := fallbackSize(m.width, m.height)
	if m.help.open {
		return renderHelp("Main Menu", m.keymap(), w, h)
	}
	th := theme.Current

	title := renderAppTitle()
	subtitle := th.Subtitle.Render("Learn no-limit hold'em one decision at a time")
	menuBox := th.ContentBox.Padding(0, 2).Width(52).Render(m.list.Render(46))
	hint := th.Help.Render("up/down move " + theme.G.Dot + " enter select " +
		theme.G.Dot + " ? help " + theme.G.Dot + " esc quit")

	width := lipgloss.Width(menuBox)
	content := lipgloss.PlaceHorizontal(width, lipgloss.Center, title) + "\n" +
		lipgloss.PlaceHorizontal(width, lipgloss.Center, subtitle) + "\n\n" +
		menuBox + "\n" +
		lipgloss.PlaceHorizontal(width, lipgloss.Center, hint)

	return frame(w, h, content)
}

// renderAppTitle draws the one-line banner: the game name flanked by the
// four suits in their deck colors — a tiny, always-visible advertisement of
// the four-color deck the player will see on the table.
func renderAppTitle() string {
	g := theme.G
	left := theme.SuitStyle(engine.Spades).Render(g.SuitSpade) + " " +
		theme.SuitStyle(engine.Hearts).Render(g.SuitHeart)
	right := theme.SuitStyle(engine.Diamonds).Render(g.SuitDiamond) + " " +
		theme.SuitStyle(engine.Clubs).Render(g.SuitClub)
	// Header, not Title: Title carries a bottom margin, which would turn
	// this one-line banner into a two-row block.
	name := theme.Current.Header.Render("TEXAS HOLD'EM")
	return left + " " + name + " " + right
}
