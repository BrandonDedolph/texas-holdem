package app

import (
	"strconv"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// GameSetup configures a session — blinds, buy-in, opponent lineup, coach
// verbosity, speed — and emits a TableConfig payload on Start. Defaults come
// from the profile's last-used setup, and starting a game persists the
// choices back, so the second session begins where the first left off.
type GameSetup struct {
	prof  *profile.Profile
	prefs *Prefs

	list   *formList
	help   helpOverlay
	width  int
	height int
}

// Setup rows, in list order.
const (
	setupStart = iota
	setupBlinds
	setupStack
	setupLineup
	setupCoach
	setupSpeed
	setupBack
)

// blindPresets are the selectable stakes. Small integers on purpose: the
// pot-odds arithmetic the app teaches should be doable in a beginner's head.
var blindPresets = []engine.Blinds{
	{Small: 1, Big: 2},
	{Small: 2, Big: 5},
	{Small: 5, Big: 10},
	{Small: 10, Big: 20},
	{Small: 25, Big: 50},
}

// stackPresetsBB are the selectable buy-ins in big blinds. 100bb is the
// canonical cash-game depth every strategy chart assumes.
var stackPresetsBB = []int{40, 60, 100, 150, 200}

// NewGameSetup builds the setup screen with defaults from the profile's
// last-used table config and the session prefs.
func NewGameSetup(p *profile.Profile, prefs *Prefs) *GameSetup {
	defs := p.TableDefaults

	blindOpts := make([]string, len(blindPresets))
	blindSel := 0
	for i, b := range blindPresets {
		blindOpts[i] = b.Small.String() + "/" + b.Big.String()
		if b.Small == defs.SmallBlind && b.Big == defs.BigBlind {
			blindSel = i
		}
	}

	stackOpts := make([]string, len(stackPresetsBB))
	stackSel := indexOfInt(stackPresetsBB, defaultStackBB(defs), 2)
	for i, bb := range stackPresetsBB {
		stackOpts[i] = strconv.Itoa(bb) + " bb"
	}

	presets := LineupPresets()
	lineupOpts := make([]string, len(presets))
	lineupSel := 0
	for i, lp := range presets {
		lineupOpts[i] = lp.Label
		if sameLineup(lp.Keys, defs.Lineup) {
			lineupSel = i
		}
	}

	coachOpts := []string{CoachFull.String(), CoachMistakes.String(), CoachOff.String()}
	speedOpts := []string{SpeedLearn.String(), SpeedNormal.String(), SpeedFast.String(), SpeedInstant.String()}

	rows := []listRow{
		{Label: "Start Game", Detail: "Deal in with the settings below"},
		{Label: "Blinds", Detail: "Table stakes (small/big blind)",
			Options: blindOpts, Sel: blindSel},
		{Label: "Starting stack", Detail: "Buy-in depth in big blinds; 100 bb is standard",
			Options: stackOpts, Sel: stackSel},
		{Label: "Opponents", Detail: presets[lineupSel].describe(),
			Options: lineupOpts, Sel: lineupSel},
		{Label: "Coach", Detail: "Full explains, Mistakes corrects, Off stays quiet",
			Options: coachOpts, Sel: int(CoachModeFromProfile(p.CoachMode))},
		{Label: "Speed", Detail: "Learn pauses on each new card for reading time",
			Options: speedOpts, Sel: int(prefs.Speed)},
		{Label: "Back", Detail: "Return to the main menu"},
	}
	return &GameSetup{prof: p, prefs: prefs, list: newFormList(rows)}
}

// describe summarizes a lineup preset for the detail line: "Nit, TAG, ...".
func (lp LineupPreset) describe() string {
	byKey := map[string]string{}
	for _, a := range Archetypes() {
		byKey[a.Key] = a.Label
	}
	out := ""
	for i, key := range lp.Keys {
		if i > 0 {
			out += ", "
		}
		if label, ok := byKey[key]; ok {
			out += label
		} else {
			out += key
		}
	}
	return out
}

// defaultStackBB converts the persisted chip stack into big blinds.
func defaultStackBB(defs profile.TableConfig) int {
	if defs.BigBlind <= 0 {
		return 100
	}
	return int(defs.Stack / defs.BigBlind)
}

// indexOfInt finds v in vals, or returns fallback.
func indexOfInt(vals []int, v, fallback int) int {
	for i, x := range vals {
		if x == v {
			return i
		}
	}
	return fallback
}

// sameLineup reports whether two lineups list the same archetype keys in
// the same seat order. Order matters: seats are positions at the table.
func sameLineup(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Init implements tea.Model.
func (g *GameSetup) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (g *GameSetup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		g.width, g.height = msg.Width, msg.Height
	case tea.KeyMsg:
		cmd, _ := g.help.dispatch(g.keymap(), msg, g.handleAction)
		return g, cmd
	}
	return g, nil
}

func (g *GameSetup) keymap() KeyMap { return gameSetupKeys }

func (g *GameSetup) handleAction(a KeyAction) (tea.Cmd, bool) {
	switch a {
	case ActUp:
		g.list.MoveUp()
		return nil, true
	case ActDown:
		g.list.MoveDown()
		return nil, true
	case ActLeft:
		g.cycled(g.list.CycleLeft())
		return nil, true
	case ActRight:
		g.cycled(g.list.CycleRight())
		return nil, true
	case ActSelect:
		switch g.list.Cursor() {
		case setupStart:
			return g.start(), true
		case setupBack:
			return Navigate(ScreenMainMenu), true
		default:
			// Enter on a value row cycles it forward, euchre-style, so the
			// whole screen is drivable with two keys.
			g.cycled(g.list.CycleRight())
			return nil, true
		}
	case ActBack:
		return Navigate(ScreenMainMenu), true
	}
	return nil, false
}

// cycled reacts to a value change: the opponents row's detail line names
// the archetypes in the newly-selected preset.
func (g *GameSetup) cycled(changed bool) {
	if !changed {
		return
	}
	if g.list.Cursor() == setupLineup {
		presets := LineupPresets()
		row := g.list.Row()
		if row.Sel >= 0 && row.Sel < len(presets) {
			row.Detail = presets[row.Sel].describe()
		}
	}
}

// start builds the TableConfig, persists it as the new defaults, and
// navigates to the table with the payload.
func (g *GameSetup) start() tea.Cmd {
	cfg := g.buildConfig()

	// Persist the choices as next session's defaults. A failed save must
	// never block sitting down to play — the profile package already
	// guarantees the in-memory profile stays usable, and the next
	// successful save catches everything up. TODO(wire-banner): surface
	// save errors in a status line.
	g.prof.TableDefaults = profile.TableConfig{
		Lineup:     cfg.Lineup,
		SmallBlind: cfg.SmallBlind,
		BigBlind:   cfg.BigBlind,
		Stack:      cfg.Stack,
	}
	g.prof.CoachMode = cfg.CoachMode.ProfileKey()
	_ = g.prof.Save()
	g.prefs.Speed = cfg.Speed

	return NavigateWithData(ScreenTable, cfg)
}

// buildConfig assembles the payload from the current row selections.
func (g *GameSetup) buildConfig() TableConfig {
	rows := g.list.rows
	blinds := blindPresets[rows[setupBlinds].Sel]
	stackBB := stackPresetsBB[rows[setupStack].Sel]
	lineup := LineupPresets()[rows[setupLineup].Sel]
	return TableConfig{
		SmallBlind: blinds.Small,
		BigBlind:   blinds.Big,
		Stack:      engine.Chips(stackBB) * blinds.Big,
		Lineup:     append([]string(nil), lineup.Keys...),
		CoachMode:  CoachMode(rows[setupCoach].Sel),
		Speed:      Speed(rows[setupSpeed].Sel),
	}
}

// View implements tea.Model.
func (g *GameSetup) View() string {
	w, h := fallbackSize(g.width, g.height)
	if g.help.open {
		return renderHelp("Game Setup", g.keymap(), w, h)
	}
	// "enter select" is deliberately absent: with the shell's "? help" tail
	// the full legend overflows the 60-column floor, and rowLR would drop
	// the whole footer rather than clip mid-escape. Enter's role is on the
	// help sheet; the two keys that differ from Settings are the ones shown.
	return renderShell(w, h, shell{
		Title:  "Game Setup",
		Status: g.list.Detail(),
		Footer: "up/down move " + theme.G.Dot + " left/right change " +
			theme.G.Dot + " esc back",
	}, "\n"+g.list.Render(formListWidth))
}
