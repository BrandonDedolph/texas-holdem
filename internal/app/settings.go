package app

import (
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// BackgroundMode is the theme override: Auto trusts lipgloss's background
// detection, Dark/Light force it for terminals that misreport
// (docs/design-tui.md §7.2).
type BackgroundMode int

// Background modes, in cycle order.
const (
	BackgroundAuto BackgroundMode = iota
	BackgroundDark
	BackgroundLight
)

var backgroundNames = [...]string{"Auto", "Dark", "Light"}

// String is the display label ("Auto").
func (m BackgroundMode) String() string {
	if int(m) < 0 || int(m) >= len(backgroundNames) {
		return "?"
	}
	return backgroundNames[m]
}

// Prefs are the presentation preferences the Settings screen controls. They
// are held here as typed enums and mirrored to profile.Display (which stores
// them as strings) so they survive restarts. CoachMode is deliberately absent:
// it lives directly on the profile.
type Prefs struct {
	Speed      Speed
	Deck       theme.DeckMode
	ASCII      bool
	Background BackgroundMode

	// detectedDark is what lipgloss detected before any override, so
	// switching back to Auto restores reality rather than guessing.
	detectedDark bool
}

// DefaultPrefs captures the terminal's detected state and the learning-tool
// defaults: Learn speed (unlimited reading time) and the four-color deck
// (flush blindness is the classic beginner leak).
func DefaultPrefs() *Prefs {
	return &Prefs{
		Speed:        SpeedLearn,
		Deck:         theme.TwoColor,
		ASCII:        !theme.UnicodeSupported(),
		Background:   BackgroundAuto,
		detectedDark: lipgloss.HasDarkBackground(),
	}
}

// PrefsFrom builds Prefs from a profile's stored display preferences, falling
// back to the detected terminal state for anything unset. Unknown stored values
// fall back to the default rather than erroring: a typo in a hand-edited
// profile.json should never stop the game from starting.
func PrefsFrom(p *profile.Profile) *Prefs {
	prefs := DefaultPrefs()
	if p == nil {
		return prefs
	}
	d := p.Display
	switch d.Speed {
	case "normal":
		prefs.Speed = SpeedNormal
	case "fast":
		prefs.Speed = SpeedFast
	case "instant":
		prefs.Speed = SpeedInstant
	default:
		prefs.Speed = SpeedLearn
	}
	if d.Deck == "four-color" {
		prefs.Deck = theme.FourColor
	} else {
		prefs.Deck = theme.TwoColor
	}
	// ASCII is sticky only when the user turned it on; if the terminal cannot
	// render Unicode we force it regardless of what was stored.
	prefs.ASCII = d.ASCII || !theme.UnicodeSupported()
	switch d.Background {
	case "dark":
		prefs.Background = BackgroundDark
	case "light":
		prefs.Background = BackgroundLight
	default:
		prefs.Background = BackgroundAuto
	}
	return prefs
}

// Display projects the prefs back into the profile's storable form.
func (p *Prefs) Display() profile.Display {
	speed := map[Speed]string{
		SpeedLearn: "learn", SpeedNormal: "normal",
		SpeedFast: "fast", SpeedInstant: "instant",
	}[p.Speed]
	deck := "four-color"
	if p.Deck == theme.TwoColor {
		deck = "two-color"
	}
	background := map[BackgroundMode]string{
		BackgroundAuto: "auto", BackgroundDark: "dark", BackgroundLight: "light",
	}[p.Background]
	return profile.Display{Speed: speed, Deck: deck, ASCII: p.ASCII, Background: background}
}

// Apply pushes the presentation prefs into the global theme state and mirrors
// them onto the profile. Called once at startup and after every Settings
// change; the caller is responsible for saving.
func (p *Prefs) ApplyTo(prof *profile.Profile) {
	if prof != nil {
		prof.Display = p.Display()
	}
	theme.SetDeckMode(p.Deck)
	if p.ASCII {
		theme.G = theme.ASCII()
	} else {
		theme.G = theme.Unicode()
	}
	switch p.Background {
	case BackgroundDark:
		lipgloss.SetHasDarkBackground(true)
	case BackgroundLight:
		lipgloss.SetHasDarkBackground(false)
	default:
		lipgloss.SetHasDarkBackground(p.detectedDark)
	}
}

// Settings edits presentation and coaching preferences. Every change
// applies immediately — the screen behind this one already looks different
// when the player returns to it — and profile-backed values are saved to
// disk on the spot.
type Settings struct {
	prof  *profile.Profile
	prefs *Prefs

	list   *formList
	help   helpOverlay
	width  int
	height int
}

// Settings rows, in list order.
const (
	settingTheme = iota
	settingDeck
	settingGlyphs
	settingCoach
	settingSpeed
	settingLocks
	settingBack
)

// NewSettings builds the settings screen reflecting the current state.
func NewSettings(p *profile.Profile, prefs *Prefs) *Settings {
	rows := []listRow{
		{Label: "Theme", Detail: "Force dark/light if detection guesses wrong",
			Options: []string{BackgroundAuto.String(), BackgroundDark.String(), BackgroundLight.String()},
			Sel:     int(prefs.Background)},
		{Label: "Deck colors", Detail: "Two-color like a real deck; four-color makes flushes pop",
			Options: []string{"Four-color", "Two-color"},
			Sel:     deckSel(prefs.Deck)},
		{Label: "Card glyphs", Detail: "ASCII fallback for terminals without Unicode",
			Options: []string{"Unicode", "ASCII"},
			Sel:     boolSel(prefs.ASCII)},
		{Label: "Coach", Detail: "Full explains, Mistakes corrects, Off stays quiet",
			Options: []string{CoachFull.String(), CoachMistakes.String(), CoachOff.String()},
			Sel:     int(CoachModeFromProfile(p.CoachMode))},
		{Label: "Speed", Detail: "Default pacing; adjustable live with +/-",
			Options: []string{SpeedLearn.String(), SpeedNormal.String(), SpeedFast.String(), SpeedInstant.String()},
			Sel:     int(prefs.Speed)},
		// Lesson locks default to the curriculum order (docs/ui-review.md
		// §5.4): the lockstep is defensible pedagogy, but the app should
		// never fight the one learner it has — "All open" lifts every
		// prerequisite gate while completion tracking carries on unchanged.
		{Label: "Lesson locks", Detail: "Finish prerequisites in order, or lift every lock",
			Options: []string{"In order", "All open"},
			Sel:     boolSel(p.UnlockAllLessons)},
		{Label: "Back", Detail: "Return to the main menu"},
	}
	return &Settings{prof: p, prefs: prefs, list: newFormList(rows)}
}

func deckSel(m theme.DeckMode) int {
	if m == theme.TwoColor {
		return 1
	}
	return 0
}

func boolSel(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Init implements tea.Model.
func (s *Settings) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (s *Settings) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
	case tea.KeyMsg:
		cmd, _ := s.help.dispatch(s.keymap(), msg, s.handleAction)
		return s, cmd
	}
	return s, nil
}

func (s *Settings) keymap() KeyMap { return settingsKeys }

func (s *Settings) handleAction(a KeyAction) (tea.Cmd, bool) {
	switch a {
	case ActUp:
		s.list.MoveUp()
		return nil, true
	case ActDown:
		s.list.MoveDown()
		return nil, true
	case ActLeft:
		if s.list.CycleLeft() {
			s.applyRow(s.list.Cursor())
		}
		return nil, true
	case ActRight:
		if s.list.CycleRight() {
			s.applyRow(s.list.Cursor())
		}
		return nil, true
	case ActSelect:
		if s.list.Cursor() == settingBack {
			return Navigate(ScreenMainMenu), true
		}
		if s.list.CycleRight() {
			s.applyRow(s.list.Cursor())
		}
		return nil, true
	case ActBack:
		return Navigate(ScreenMainMenu), true
	}
	return nil, false
}

// applyRow makes one changed row take effect immediately: presentation
// changes hit the theme globals, coaching changes hit the profile and are
// saved on the spot. Immediate persistence is the contract — there is no
// "apply" button to forget.
func (s *Settings) applyRow(row int) {
	r := &s.list.rows[row]
	switch row {
	case settingTheme:
		s.prefs.Background = BackgroundMode(r.Sel)
	case settingDeck:
		if r.Sel == 1 {
			s.prefs.Deck = theme.TwoColor
		} else {
			s.prefs.Deck = theme.FourColor
		}
	case settingGlyphs:
		s.prefs.ASCII = r.Sel == 1
	case settingCoach:
		s.prof.CoachMode = CoachMode(r.Sel).ProfileKey()
		// Best-effort, same rationale as GameSetup.start: a broken disk
		// must not brick the settings screen. TODO(wire-banner).
		_ = s.prof.Save()
	case settingSpeed:
		s.prefs.Speed = Speed(r.Sel)
	case settingLocks:
		s.prof.UnlockAllLessons = r.Sel == 1
	}
	s.prefs.ApplyTo(s.prof)
	// Presentation prefs live on the profile too, so persist them alongside
	// the coaching change above. Best-effort for the same reason: a broken
	// disk must not brick the settings screen.
	_ = s.prof.Save()
}

// View implements tea.Model.
func (s *Settings) View() string {
	w, h := fallbackSize(s.width, s.height)
	if s.help.open {
		return renderHelp("Settings", s.keymap(), w, h)
	}
	return renderShell(w, h, shell{
		Title:  "Settings",
		Status: s.list.Detail(),
		Footer: "up/down move " + theme.G.Dot + " left/right change " +
			theme.G.Dot + " esc back",
	}, "\n"+s.list.Render(formListWidth))
}
