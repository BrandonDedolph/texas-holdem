package app

import (
	"strconv"
	"strings"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/components"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
)

// The action bar (docs/design-tui.md §4): a two-row region with three
// states, driven strictly by the engine's LegalActions — the bar never
// computes legality itself, it only renders and clamps within the
// ActionOption ranges the engine hands it.

// ActionBarState is the bar's interaction state.
type ActionBarState int

// Action bar states (§4.1).
const (
	ActionBarWaiting  ActionBarState = iota // not the hero's turn
	ActionBarChoosing                       // one keycap per legal action
	ActionBarSizing                         // bet/raise amount being chosen
)

// ActionBar owns the hero's action interaction. Sizing is ONE amount value:
// presets set it, arrows nudge it, typed digits rewrite it — the slider is a
// readout of that value, never a separate widget with focus (§4.3).
type ActionBar struct {
	state ActionBarState
	legal engine.ActionOptions

	// Sizing state. mode is ActionBet or ActionRaise; opt is that action's
	// engine-computed range — the only source of clamping.
	mode   engine.ActionType
	opt    engine.ActionOption
	amount engine.Chips
	typed  string // digits typed so far; "" means amount holds the value

	// Hero-spot numbers for preset arithmetic and the choosing readout.
	street    engine.Street
	pot       engine.Chips // pot total before the hero acts
	toCall    engine.Chips // chips the hero owes to call (0 when check is free)
	committed engine.Chips // hero's chips already in this street
	bigBlind  engine.Chips // nudge unit
	info      string       // right-aligned readout while choosing (pot odds etc.)
}

// NewActionBar builds a bar in the waiting state.
func NewActionBar() *ActionBar { return &ActionBar{} }

// State returns the bar's interaction state.
func (b *ActionBar) State() ActionBarState { return b.state }

// Wait blanks the bar: not the hero's turn (or no hand at all). The two-row
// region still renders, empty — the layout never reflows.
func (b *ActionBar) Wait() {
	*b = ActionBar{bigBlind: b.bigBlind}
}

// Arm puts the bar in the choosing state for the hero's turn. legal comes
// verbatim from Hand.LegalActions; pot/toCall/committed feed the preset and
// odds arithmetic; info is the pre-formatted right-hand readout.
func (b *ActionBar) Arm(legal engine.ActionOptions, street engine.Street, pot, toCall, committed, bigBlind engine.Chips, info string) {
	b.state = ActionBarChoosing
	b.legal = legal
	b.street = street
	b.pot, b.toCall, b.committed, b.bigBlind = pot, toCall, committed, bigBlind
	b.info = info
	b.mode = 0
	b.opt = engine.ActionOption{}
	b.amount, b.typed = 0, ""
}

// visibleOptions is the subset of legal actions that render as keycaps.
// One deliberate cut: fold is hidden when checking is free. The engine
// rightly reports an open fold as legal, but rendering it would teach a
// beginner that folding for free is a live option worth weighing — the
// mockups omit it, so the bar does too, and dispatch uses this same filter
// so keys-that-act and keycaps-on-screen stay identical.
func (b *ActionBar) visibleOptions() []engine.ActionOption {
	_, checkFree := b.legal.Find(engine.ActionCheck)
	out := make([]engine.ActionOption, 0, len(b.legal))
	for _, o := range b.legal {
		if o.Type == engine.ActionFold && checkFree {
			continue
		}
		out = append(out, o)
	}
	return out
}

// Accepts reports whether a choosing-state key for this action type should
// do anything right now.
func (b *ActionBar) Accepts(t engine.ActionType) bool {
	if b.state != ActionBarChoosing {
		return false
	}
	for _, o := range b.visibleOptions() {
		if o.Type == t {
			return true
		}
	}
	return false
}

// OpenSizing enters sizing mode for a bet or raise. When the engine has
// pinned Min == Max (the only legal raise is all-in), sizing mode is skipped
// entirely and the amount to submit is returned with skipped == true — there
// is nothing to size (§4.3).
func (b *ActionBar) OpenSizing(t engine.ActionType) (submit engine.Chips, skipped bool) {
	opt, ok := b.legal.Find(t)
	if !ok || b.state != ActionBarChoosing {
		return 0, false
	}
	if opt.Min == opt.Max {
		return opt.Min, true
	}
	b.state = ActionBarSizing
	b.mode = t
	b.opt = opt
	b.typed = ""
	// Start on preset 2: postflop that is half pot, the most common size;
	// opening preflop it is 2.5bb, the standard open the coach teaches. In
	// both cases a teaching anchor — the player sees where they are.
	b.amount = b.presetAmounts()[1]
	return 0, false
}

// OpenSizingAllIn is the `a` shortcut: sizing mode pre-filled to the
// maximum, still requiring enter to confirm — an all-in should never be one
// accidental keypress (§4.2). Reports whether a bet/raise was available.
func (b *ActionBar) OpenSizingAllIn() bool {
	t := engine.ActionRaise
	if _, ok := b.legal.Find(t); !ok {
		t = engine.ActionBet
		if _, ok := b.legal.Find(t); !ok {
			return false
		}
	}
	if amt, skipped := b.OpenSizing(t); skipped {
		// Min == Max: sizing was skipped, but `a` still confirms — re-enter
		// sizing manually with the pinned amount.
		opt, _ := b.legal.Find(t)
		b.state = ActionBarSizing
		b.mode = t
		b.opt = opt
		b.amount, b.typed = amt, ""
		return true
	}
	b.amount, b.typed = b.opt.Max, ""
	return true
}

// Cancel leaves sizing mode back to choosing.
func (b *ActionBar) Cancel() {
	if b.state == ActionBarSizing {
		b.state = ActionBarChoosing
		b.typed = ""
	}
}

// Preset applies sizing preset i (0-based: 1/3, 1/2, 2/3, pot, all-in
// postflop; 2bb, 2.5bb, 3bb, 4bb, all-in opening preflop). While an amount
// is being typed, the digit keys 1-5 belong to the typed number instead —
// see TypeDigit.
func (b *ActionBar) Preset(i int) {
	if b.state != ActionBarSizing || i < 0 || i > 4 {
		return
	}
	if b.typed != "" {
		// Mid-typing, "3" means the digit three, not the 2/3-pot preset.
		b.TypeDigit(byte('0' + i + 1))
		return
	}
	b.amount = b.presetAmounts()[i]
}

// TypeDigit begins or continues a typed amount. With nothing typed yet the
// keys 1-5 are presets, so a typed amount starts with 0 or 6-9; a leading 0
// is the documented way to type any amount ("0120" enters 120).
func (b *ActionBar) TypeDigit(d byte) {
	if b.state != ActionBarSizing || d < '0' || d > '9' {
		return
	}
	if len(b.typed) >= 9 { // beyond any stack this app deals in
		return
	}
	b.typed += string(d)
}

// Backspace deletes the last typed digit; with none left the slider value
// shows again.
func (b *ActionBar) Backspace() {
	if b.state != ActionBarSizing || b.typed == "" {
		return
	}
	b.typed = b.typed[:len(b.typed)-1]
}

// Nudge moves the amount by one big blind (engine-clamped). Nudging adopts
// and then abandons any typed value: the typed string collapses into the
// amount it named, clamped, then moves.
func (b *ActionBar) Nudge(dir int) {
	if b.state != ActionBarSizing {
		return
	}
	b.amount = b.clamp(b.value() + engine.Chips(dir)*b.bigBlind)
	b.typed = ""
}

// value is the current sizing value before clamping: the typed number if
// one is in progress, else the slider amount.
func (b *ActionBar) value() engine.Chips {
	if b.typed == "" {
		return b.amount
	}
	n, err := strconv.ParseInt(b.typed, 10, 64)
	if err != nil {
		return b.amount
	}
	return engine.Chips(n)
}

// clamp pins a value into the engine's [Min, Max]. The range came from
// LegalActions; the bar never recomputes min-raise rules itself.
func (b *ActionBar) clamp(v engine.Chips) engine.Chips {
	if v < b.opt.Min {
		return b.opt.Min
	}
	if v > b.opt.Max {
		return b.opt.Max
	}
	return v
}

// ConfirmAmount is the value that enter will submit: the current value,
// clamped. A typed amount outside the range is resolved by clamping, never
// by an error dialog — the engine can always resolve it (§4.3).
func (b *ActionBar) ConfirmAmount() engine.Chips { return b.clamp(b.value()) }

// SizingType returns which action is being sized (ActionBet or ActionRaise).
func (b *ActionBar) SizingType() engine.ActionType { return b.mode }

// preflopRaise reports whether the raise being sized is a preflop raise of
// any kind — an open, an iso over limpers, or a 3-bet over someone else's
// open. Preflop sizing is spoken in big blinds ("open to 2.5x", "3-bet to
// 9bb"), postflop in pot fractions, and the controls should teach that
// distinction rather than blur it (docs/ui-review.md F5).
func (b *ActionBar) preflopRaise() bool {
	return b.mode == engine.ActionRaise && b.bigBlind > 0 && b.street == engine.Preflop
}

// facingOpen reports whether there is a raise to 3-bet over, as opposed to
// the hero being first in with only the blind to beat. The current bet
// level is committed + toCall; anything above one big blind means somebody
// has already raised.
func (b *ActionBar) facingOpen() bool {
	return b.committed+b.toCall > b.bigBlind
}

// openingPreflop is a preflop raise with nobody having opened yet.
func (b *ActionBar) openingPreflop() bool {
	return b.preflopRaise() && !b.facingOpen()
}

// presetAmounts computes the five preset values, engine-clamped.
//
// Opening preflop, the presets are the standard big-blind ladder — 2bb,
// 2.5bb, 3bb, 4bb — because that is the vocabulary every chart and lesson
// in this app uses for preflop raises, and it puts the coach's standard
// 2.5bb open one keypress away (F5). Larger sizes (3.5bb opens, +1bb per
// limper) are one 1bb nudge from a rung.
//
// Postflop, fractions are of the pot *after the hero's call* (base =
// committed + toCall matches the current bet first), because that is the
// pot the fraction is "of" — translating "half pot" into chips is precisely
// the arithmetic a beginner needs to internalize, so the labels show these
// resolved numbers.
func (b *ActionBar) presetAmounts() [5]engine.Chips {
	if b.openingPreflop() {
		bb := b.bigBlind
		return [5]engine.Chips{
			b.clamp(2 * bb),
			b.clamp(5 * bb / 2),
			b.clamp(3 * bb),
			b.clamp(4 * bb),
			b.opt.Max,
		}
	}
	if b.preflopRaise() {
		// Facing an open, the standard 3-bet is a multiple of the open, not
		// of the blind: roughly 3x in position and 4x out of it. Anchoring
		// the ladder to the open is what makes "3-bet to 3x" a size the
		// player can reach, and teaches the multiple rather than a number.
		open := b.committed + b.toCall
		return [5]engine.Chips{
			b.clamp(5 * open / 2),
			b.clamp(3 * open),
			b.clamp(7 * open / 2),
			b.clamp(4 * open),
			b.opt.Max,
		}
	}
	potAfterCall := b.pot + b.toCall
	base := b.committed + b.toCall
	frac := func(num, den engine.Chips) engine.Chips {
		return b.clamp(base + (potAfterCall*num+den/2)/den)
	}
	return [5]engine.Chips{
		frac(1, 3),
		frac(1, 2),
		frac(2, 3),
		b.clamp(base + potAfterCall),
		b.opt.Max,
	}
}

// View renders the bar as exactly two rows, each exactly width cells — in
// every state, including waiting, so the region can never reflow the table.
func (b *ActionBar) View(width int) string {
	var top, bottom string
	switch b.state {
	case ActionBarChoosing:
		top = rowLR(width, b.choosingRow(), theme.Current.ActionLabel.Render(b.info))
		bottom = strings.Repeat(" ", width)
	case ActionBarSizing:
		top = b.sizingRow(width)
		bottom = rowLR(width, b.presetRow(),
			theme.Current.Help.Render("arrows nudge "+theme.G.Dot+" 0-9 type"))
	default:
		top = strings.Repeat(" ", width)
		bottom = strings.Repeat(" ", width)
	}
	return top + "\n" + bottom
}

// choosingRow renders one keycap chip per visible legal action with its
// cost. Only legal actions render — no dimmed-out fold; absence teaches
// legality better than presence-but-disabled (§4.1).
func (b *ActionBar) choosingRow() string {
	th := theme.Current
	g := theme.G
	var sb strings.Builder
	sb.WriteString("  ")
	for _, o := range b.visibleOptions() {
		var key, label string
		switch o.Type {
		case engine.ActionFold:
			key, label = "f", "fold"
		case engine.ActionCheck:
			key, label = "x", "check"
		case engine.ActionCall:
			key, label = "c", "call "+o.Min.String()
		case engine.ActionBet:
			key, label = "b", "bet"+g.Ellipsis
		case engine.ActionRaise:
			if o.Min == o.Max {
				// The only legal raise is all-in: say so on the keycap.
				key, label = "r", "all-in "+o.Max.String()
			} else {
				key, label = "r", "raise"+g.Ellipsis
			}
		}
		sb.WriteString(th.ActionKeycap.Render(key))
		sb.WriteString(" ")
		sb.WriteString(th.ActionLabel.Render(label))
		sb.WriteString("      ")
	}
	return strings.TrimRight(sb.String(), " ")
}

// sizingRow renders the slider readout row:
//
//	BET   min 10 |------*------| all-in 900        bet: 120
//
// A typed amount outside the legal range renders in the warning style with
// the clamp it will receive ("bet: 7 -> min 10").
func (b *ActionBar) sizingRow(width int) string {
	th := theme.Current

	label := "BET"
	verb := "bet"
	if b.mode == engine.ActionRaise {
		label = "RAISE"
		verb = "raise to"
	}

	val := b.value()
	readout := verb + ": " + val.String()
	style := th.ActionLabel
	switch {
	case val < b.opt.Min:
		readout += " " + theme.G.Ellipsis + " min " + b.opt.Min.String()
		style = th.GradeBad
	case val > b.opt.Max:
		readout += " " + theme.G.Ellipsis + " all-in " + b.opt.Max.String()
		style = th.GradeBad
	}

	left := " " + th.CoachTitle.Render(label) + "  "
	right := "  " + style.Render(readout)
	sliderW := width - visibleWidth(left) - visibleWidth(right)
	if sliderW < 8 {
		sliderW = 8
	}
	return rowLR(width, left+components.Slider(b.opt.Min, b.opt.Max, b.ConfirmAmount(), sliderW), right)
}

// presetRow renders the numbered presets with their resolved chip amounts —
// "2 1/2 93", never just "half pot" (§4.3). Translating fractions into
// chips is exactly the arithmetic a beginner needs to see done. Opening
// preflop, the same translation is bb-to-chips ("2 2.5bb 25"): the labels
// themselves are how the player sees which vocabulary this street speaks.
func (b *ActionBar) presetRow() string {
	th := theme.Current
	amts := b.presetAmounts()
	names := [5]string{"1/3", "1/2", "2/3", "pot", "all-in"}
	if b.openingPreflop() {
		names = [5]string{"2bb", "2.5bb", "3bb", "4bb", "all-in"}
	}
	var sb strings.Builder
	sb.WriteString("  ")
	for i, name := range names {
		if i > 0 {
			sb.WriteString("  ")
		}
		sb.WriteString(th.ActionKeycap.Render(strconv.Itoa(i + 1)))
		sb.WriteString(" " + th.ActionLabel.Render(name))
		if i < 4 { // "5 all-in" needs no amount: the slider's max cap names it
			sb.WriteString(" " + th.ActionLabel.Render(amts[i].String()))
		}
	}
	return sb.String()
}
