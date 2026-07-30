// Package app is the Bubble Tea shell: the root model, screen navigation,
// and the non-gameplay screens (menu, setup, settings, reference, help).
//
// The architecture is euchre's, deliberately (docs/design-tui.md §1.2): a
// root App holding a map of screen models, a NavigateMsg routed in
// App.Update, and window size forwarded to the newly-active screen before
// its Init runs. The one structural difference is session-preserving
// navigation — a screen model is recreated only when it does not exist yet
// or when the navigation carries fresh construction data, so the core loop
// of "play hand, jump to review, come back to the same table with the same
// stacks" holds a live session across navigations.
package app

import (
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/review"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// Screen identifies a screen in the application.
type Screen int

// Screens (docs/design-tui.md §1.1). Lessons and Trainer are routed but
// currently land on placeholders — their models arrive with the packages
// they depend on. TODO(wire-lessons) TODO(wire-trainer).
const (
	ScreenMainMenu Screen = iota
	ScreenGameSetup
	ScreenTable
	ScreenHandReview
	ScreenLessons
	ScreenTrainer
	ScreenQuickReference
	ScreenSettings
)

var screenNames = [...]string{
	"Main Menu", "Game Setup", "Table", "Hand Review",
	"Lessons", "Trainer", "Quick Reference", "Settings",
}

// String is the screen's display name.
func (s Screen) String() string {
	if int(s) < 0 || int(s) >= len(screenNames) {
		return "?"
	}
	return screenNames[s]
}

// App is the root Bubble Tea model. It owns navigation, the profile, and
// the terminal size; everything else lives in the screen models.
type App struct {
	current Screen
	models  map[Screen]tea.Model

	profile *profile.Profile
	prefs   *Prefs

	width, height int
	quitting      bool
}

// New loads the player's profile and builds the app on the main menu.
// profile.Load never returns an unusable profile — on a corrupt or
// unreadable file it hands back working defaults and an error meant for a
// warning banner. TODO(wire-banner): surface that error in the UI.
func New() *App {
	p, _ := profile.Load()
	return NewWithProfile(p)
}

// NewWithProfile builds the app around an explicit profile. Tests use this
// with profile.NewProfile() so nothing touches the real user directory.
func NewWithProfile(p *profile.Profile) *App {
	a := &App{
		current: ScreenMainMenu,
		models:  make(map[Screen]tea.Model),
		profile: p,
		prefs:   PrefsFrom(p),
	}
	a.prefs.ApplyTo(p) // push the restored prefs into the theme globals
	a.models[ScreenMainMenu] = NewMainMenu()
	return a
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	return a.models[a.current].Init()
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// The one global key. Everything else is a screen's business,
		// documented in that screen's keymap.
		if globalKeys.Contains(msg.String()) {
			a.quitting = true
			return a, tea.Quit
		}

	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height

	case NavigateMsg:
		return a.navigate(msg.Screen, msg.Data)

	case EndSessionMsg:
		// The session is over: drop the cached table, and the review screen
		// with it — a review of a dropped session would be stale.
		delete(a.models, ScreenTable)
		delete(a.models, ScreenHandReview)
		return a.navigate(ScreenMainMenu, nil)

	case QuitMsg:
		a.quitting = true
		return a, tea.Quit
	}

	if model, ok := a.models[a.current]; ok {
		updated, cmd := model.Update(msg)
		a.models[a.current] = updated
		return a, cmd
	}
	return a, nil
}

// View implements tea.Model. The too-small floor is enforced here, once,
// so no screen ever has to render into an impossible box.
func (a *App) View() string {
	if a.quitting {
		return "Thanks for playing. Review your hands tomorrow with fresh eyes.\n"
	}
	if a.width > 0 && layoutFor(a.width, a.height) == LayoutTooSmall {
		return renderTooSmall(a.width, a.height)
	}
	if model, ok := a.models[a.current]; ok {
		return model.View()
	}
	return "Loading..."
}

// navigate switches screens, honoring session preservation: an existing
// model is reused unless the message carries fresh construction data
// (docs/design-tui.md §1.2). Window size reaches the target before Init so
// the first View() already lays out correctly.
func (a *App) navigate(screen Screen, data interface{}) (tea.Model, tea.Cmd) {
	if _, exists := a.models[screen]; !exists || data != nil {
		a.models[screen] = a.newScreen(screen, data)
	}
	if a.width > 0 && a.height > 0 {
		updated, _ := a.models[screen].Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
		a.models[screen] = updated
	}
	a.current = screen
	return a, a.models[screen].Init()
}

// newScreen constructs a screen model, threading in whatever shared state
// that screen needs. Navigation payloads are the named structs in
// payloads.go; anything else is ignored and the screen gets its defaults.
func (a *App) newScreen(screen Screen, data interface{}) tea.Model {
	switch screen {
	case ScreenGameSetup:
		return NewGameSetup(a.profile, a.prefs)
	case ScreenSettings:
		return NewSettings(a.profile, a.prefs)
	case ScreenQuickReference:
		return NewQuickReference()
	case ScreenTable:
		if cfg, ok := data.(TableConfig); ok {
			return NewTableScreen(cfg, a.prefs, a.profile)
		}
		// Navigating to the table without a payload and without a cached
		// session (e.g. a future "resume" shortcut): fall back to the
		// profile's last-used setup rather than refusing to play.
		return NewTableScreen(a.defaultTableConfig(), a.prefs, a.profile)
	case ScreenHandReview:
		// The review replays the cached table session's last completed hand.
		// No session (or no finished hand yet) gets the screen's honest empty
		// state rather than a refusal — navigation must always work.
		returnTo := ScreenMainMenu
		if req, ok := data.(ReviewRequest); ok {
			returnTo = req.ReturnTo
		}
		var model *review.ReplayModel
		if t, ok := a.models[ScreenTable].(*TableScreen); ok {
			if rec, ann, ok := t.lastHandArchive(); ok {
				model = review.Replay(rec, ann)
			}
		}
		return NewHandReview(model, returnTo)
	case ScreenLessons:
		l := NewLessons(a.profile, a.prefs)
		if req, ok := data.(LessonRequest); ok {
			// Open declines a locked lesson; the list then explains why
			// rather than silently landing on the curriculum.
			l.Open(req.LessonID)
		}
		return l
	case ScreenTrainer:
		// TODO(wire-trainer): later wave.
		return newComingSoon(screen)
	default:
		return NewMainMenu()
	}
}

// NavigateMsg is sent to switch screens. Data is nil or one of the named
// payload structs in payloads.go — never a map or bare primitive.
type NavigateMsg struct {
	Screen Screen
	Data   interface{}
}

// Navigate returns a command that navigates to a screen, reusing its cached
// model if one exists.
func Navigate(screen Screen) tea.Cmd {
	return func() tea.Msg { return NavigateMsg{Screen: screen} }
}

// NavigateWithData returns a command that navigates with fresh construction
// data, replacing any cached model for the target screen.
func NavigateWithData(screen Screen, data interface{}) tea.Cmd {
	return func() tea.Msg { return NavigateMsg{Screen: screen, Data: data} }
}

// QuitMsg signals the app to quit.
type QuitMsg struct{}

// Quit returns a command that quits the app.
func Quit() tea.Cmd {
	return func() tea.Msg { return QuitMsg{} }
}

// EndSessionMsg ends the table session: the cached Table (and HandReview)
// models are dropped and the app returns to the main menu. Sent when the
// player leaves the table for good, as opposed to Navigate(ScreenMainMenu),
// which would keep the session alive in the background.
type EndSessionMsg struct{}

// EndSession returns a command that ends the table session.
func EndSession() tea.Cmd {
	return func() tea.Msg { return EndSessionMsg{} }
}

// --- ComingSoon ---------------------------------------------------------------

// ComingSoon is the honest placeholder for screens whose real models depend
// on packages that have not landed yet. It routes, resizes, and says exactly
// what it is — so navigation, session preservation and the keybind tests
// exercise the real plumbing today.
type ComingSoon struct {
	screen   Screen
	detail   []string // extra context, e.g. the TableConfig summary
	returnTo Screen
	help     helpOverlay
	width    int
	height   int
}

// newComingSoon builds a placeholder for the given screen.
func newComingSoon(screen Screen) *ComingSoon {
	return &ComingSoon{screen: screen, returnTo: ScreenMainMenu}
}

// Init implements tea.Model.
func (c *ComingSoon) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (c *ComingSoon) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.width, c.height = msg.Width, msg.Height
	case tea.KeyMsg:
		cmd, _ := c.help.dispatch(c.keymap(), msg, c.handleAction)
		return c, cmd
	}
	return c, nil
}

func (c *ComingSoon) keymap() KeyMap { return comingSoonKeys }

func (c *ComingSoon) handleAction(a KeyAction) (tea.Cmd, bool) {
	switch a {
	case ActBack:
		return Navigate(c.returnTo), true
	}
	return nil, false
}

// View implements tea.Model.
func (c *ComingSoon) View() string {
	w, h := fallbackSize(c.width, c.height)
	if c.help.open {
		return renderHelp(c.screen.String(), c.keymap(), w, h)
	}
	th := theme.Current

	var b strings.Builder
	b.WriteString(th.Title.Render(c.screen.String()) + "\n")
	b.WriteString(th.Subtitle.Render("Coming soon: this screen arrives with a later build wave.") + "\n")
	for _, line := range c.detail {
		b.WriteString("\n" + th.Body.Render(line))
	}
	b.WriteString("\n\n" + th.Help.Render(
		"esc back "+theme.G.Dot+" ? help"))
	return frame(w, h, b.String())
}

// defaultTableConfig assembles a session config from the profile's
// last-used table defaults and the live prefs — the same values GameSetup
// would start from.
func (a *App) defaultTableConfig() TableConfig {
	d := a.profile.TableDefaults
	return TableConfig{
		SmallBlind: d.SmallBlind,
		BigBlind:   d.BigBlind,
		Stack:      d.Stack,
		Lineup:     append([]string(nil), d.Lineup...),
		CoachMode:  CoachModeFromProfile(a.profile.CoachMode),
		Speed:      a.prefs.Speed,
	}
}

// tableConfigSummary renders the setup choices a placeholder screen was
// launched with, so starting a game shows the player their config made it
// through the pipe intact. (Still exercised by ComingSoon's tests; the
// table itself no longer uses it.)
func tableConfigSummary(cfg TableConfig) []string {
	labels := make([]string, 0, len(cfg.Lineup))
	byKey := map[string]string{}
	for _, a := range Archetypes() {
		byKey[a.Key] = a.Label
	}
	for _, key := range cfg.Lineup {
		if label, ok := byKey[key]; ok {
			labels = append(labels, label)
		} else {
			labels = append(labels, key)
		}
	}
	dot := " " + theme.G.Dot + " "
	return []string{
		"Blinds " + cfg.SmallBlind.String() + "/" + cfg.BigBlind.String() +
			dot + "Stack " + cfg.Stack.String(),
		"Opponents: " + strings.Join(labels, ", "),
		"Coach " + cfg.CoachMode.String() + dot + "Speed " + cfg.Speed.String(),
	}
}
