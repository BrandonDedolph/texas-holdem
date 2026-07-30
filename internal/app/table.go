package app

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/ai/rulebased"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/equity"
	"github.com/BrandonDedolph/texas-holdem/internal/eval"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
	tea "github.com/charmbracelet/bubbletea"
)

// TableScreen is the cash-game session — the centerpiece screen
// (docs/design-tui.md §2). It owns an engine.Table across hands, runs the
// deal → act → showdown loop with Speed-scaled pacing, and composes the
// fixed-budget table layout in table_layout.go.
//
// Naming: DESIGN.md §2.5 allows the TUI model to be TableScreen so it never
// shadows engine.Table in this file.

// heroSeat is the human's seat. The hero is ALWAYS rendered bottom-center;
// position badges rotate around the fixed seats each hand — seats don't
// move, the button does, and that is itself a lesson (§2.2).
const heroSeat engine.Seat = 0

// heroName is how the hero is labelled everywhere.
const heroName = "YOU"

// Opponent decides a villain's action from its PlayerView — the view that
// physically cannot contain other seats' hole cards, which is what makes
// "bots provably cannot cheat" an honest claim.
//
// ai.Player satisfies this interface; the table keeps its own declaration so
// tests can drive scripted villains without pulling in the whole strategy
// engine.
type Opponent interface {
	Act(v *engine.PlayerView) engine.Action
	Name() string
}

// newArchetypeOpponent builds a villain from an archetype key. An unknown key
// falls back to the tight-aggressive baseline rather than failing: a stale
// lineup in a saved profile should seat a sensible opponent, not break the
// table.
//
// The seed is derived from the seat so each villain's mixed decisions (bluff
// or not) differ from its neighbours' while staying reproducible across
// replays of the same hand.
func newArchetypeOpponent(key, name string, seed int64) Opponent {
	p, ok := ai.Archetype(key)
	if !ok {
		p = ai.Baseline()
	}
	return rulebased.New(name, p, seed)
}

// checkCallOpponent checks when checking is free and calls when it is not.
// Real villains come from internal/ai; this exists so tests can reach a
// given board state without the strategy engine's decisions (and its seeded
// bluffing) making the path depend on strategy tuning. A golden render should
// break when the layout changes, not when a chart is adjusted.
type checkCallOpponent struct{ name string }

// Name implements Opponent.
func (o checkCallOpponent) Name() string { return o.name }

// Act implements Opponent.
func (o checkCallOpponent) Act(v *engine.PlayerView) engine.Action {
	if _, ok := v.Legal.Find(engine.ActionCheck); ok {
		return engine.Check{S: v.Seat}
	}
	if _, ok := v.Legal.Find(engine.ActionCall); ok {
		return engine.Call{S: v.Seat}
	}
	return engine.Fold{S: v.Seat}
}

// villainNames are the villains' table names by seat (1..5), matching the
// design mockups. Stable names make table reads transferable between hands:
// "Nia is the aggressive one" only works if Nia stays Nia.
var villainNames = [engine.MaxSeats]string{"", "Tara", "Nia", "Cole", "Ivy", "Sam"}

// stepKind is one kind of presentation step. The engine resolves a whole
// action's consequences synchronously; the queue re-serializes them into
// eye-sized pieces (board cards one at a time, reveals in showdown order,
// pots awarded one at a time — §6.1).
type stepKind int

const (
	stepBoard  stepKind = iota // land board cards (boardShown = n)
	stepPause                  // the decision pause after a street (§6.2)
	stepReveal                 // flip one seat's cards at showdown
	stepAward                  // pay one pot layer, gold-light its cards
	stepDone                   // enter the between-hands state
)

// step is one queued presentation step.
type step struct {
	kind    stepKind
	seat    engine.Seat
	n       int // stepBoard: cards visible after this step
	rank    uint32
	text    string
	street  engine.Street
	winning []engine.Card // stepAward: the five cards that play
}

// TableScreen implements tea.Model for ScreenTable.
type TableScreen struct {
	cfg   TableConfig
	prefs *Prefs

	eng       *engine.Table
	hand      *engine.Hand
	opponents [engine.MaxSeats]Opponent

	bar       *ActionBar
	coachMode CoachMode
	speed     Speed

	// seed drives deterministic decks (tests, replays); zero-value means a
	// fresh crypto-shuffled deck per hand.
	seed   uint64
	seeded bool

	handNo     int
	eventsSeen int

	// Presentation state. Every field here backs a fixed-budget region in
	// table_layout.go: the region renders blank when its field is empty.
	boardShown  int
	boardQueued int
	lastAction  [engine.MaxSeats]string
	heroLine    string // reserved hero action/grade line (row 15)
	revealed    [engine.MaxSeats]bool
	winners     engine.CardSet // showdown: the cards that play, gold-bordered
	awardText   string         // latest pot award, shown in the pot region
	recent      []string       // action ticker for the wide layout
	status      string

	queue          []step
	seq            int
	paused         bool
	handDone       bool
	completeQueued bool

	help          helpOverlay
	width, height int
}

// NewTableScreen builds the session a GameSetup TableConfig describes:
// hero at seat 0, the archetype lineup in seats 1..5, everyone at the
// configured buy-in.
func NewTableScreen(cfg TableConfig, prefs *Prefs) *TableScreen {
	// Sanitize rather than panic: a hand-edited profile must never crash the
	// table (engine.NewTable panics on nonsense blinds by design).
	if cfg.SmallBlind <= 0 || cfg.BigBlind < cfg.SmallBlind {
		cfg.SmallBlind, cfg.BigBlind = 5, 10
	}
	if cfg.Stack <= 0 {
		cfg.Stack = 100 * cfg.BigBlind
	}
	if len(cfg.Lineup) == 0 {
		cfg.Lineup = ClassroomLineup()
	}

	var stacks [engine.MaxSeats]engine.Chips
	var opps [engine.MaxSeats]Opponent
	stacks[heroSeat] = cfg.Stack
	for i := range cfg.Lineup {
		s := engine.Seat(i + 1)
		if !s.Valid() {
			break
		}
		stacks[s] = cfg.Stack
		opps[s] = newArchetypeOpponent(cfg.Lineup[i], villainNames[s], int64(s))
	}
	return newTableScreen(cfg, prefs, stacks, opps)
}

// newTableScreen is the full-control constructor tests use: explicit
// per-seat stacks (side-pot scenarios) and injectable opponents. Game state
// is still reached only through the engine — this controls who sits down
// with what, which is table configuration, not state poking.
func newTableScreen(cfg TableConfig, prefs *Prefs, stacks [engine.MaxSeats]engine.Chips, opps [engine.MaxSeats]Opponent) *TableScreen {
	eng := engine.NewTable(engine.TableConfig{
		SmallBlind: cfg.SmallBlind,
		BigBlind:   cfg.BigBlind,
		Seats:      engine.MaxSeats,
	})
	eng.Eval = eval.Evaluator

	m := &TableScreen{
		cfg:       cfg,
		prefs:     prefs,
		eng:       eng,
		opponents: opps,
		bar:       NewActionBar(),
		coachMode: cfg.CoachMode,
		speed:     cfg.Speed,
	}
	if stacks[heroSeat] > 0 {
		must(eng.Sit(heroSeat, heroName, stacks[heroSeat]))
	}
	for s := engine.Seat(1); s.Valid(); s++ {
		if stacks[s] > 0 {
			name := villainNames[s]
			if opps[s] != nil {
				name = opps[s].Name()
			}
			must(eng.Sit(s, name, stacks[s]))
		}
	}
	return m
}

// must panics on a constructor-time engine error: seats and buy-ins here
// are program-controlled, so failure is a bug, not a runtime condition.
func must(err error) {
	if err != nil {
		panic(err)
	}
}

// Init implements tea.Model. Navigation re-runs Init every time the player
// returns to a live session, so an existing hand resumes rather than
// restarts — any pending villain think or animation tick was lost when the
// player left, and resume() reissues it.
func (m *TableScreen) Init() tea.Cmd {
	if m.hand != nil {
		return m.resume()
	}
	return m.startHand()
}

// startHand deals the next hand and kicks the presentation loop.
func (m *TableScreen) startHand() tea.Cmd {
	var src engine.CardSource
	if m.seeded {
		// Seed per hand number so hand N is reproducible independent of how
		// hand N-1 was played.
		src = engine.NewDeck(m.seed + uint64(m.handNo))
	}
	h, err := m.eng.StartHand(src)
	if err != nil {
		// Not enough funded players (the hero or the table went broke).
		// Stay in the between-hands state and say why; esc still leaves.
		m.handDone = true
		m.status = "cannot deal: " + err.Error()
		return nil
	}
	m.hand = h
	m.handNo++
	m.eventsSeen = 0
	m.boardShown, m.boardQueued = 0, 0
	m.lastAction = [engine.MaxSeats]string{}
	m.heroLine = ""
	m.revealed = [engine.MaxSeats]bool{}
	m.winners = 0
	m.awardText = ""
	m.queue = nil
	m.paused, m.handDone, m.completeQueued = false, false, false
	m.bar.Wait()
	m.status = "hand #" + strconv.Itoa(m.handNo) + " dealt"

	m.ingestEvents()
	return m.advanceQueue()
}

// resume re-arms whatever the session was waiting on when the player
// navigated away mid-hand.
func (m *TableScreen) resume() tea.Cmd {
	if m.paused || m.handDone {
		return nil
	}
	if m.bar.State() != ActionBarWaiting {
		return nil // hero's turn: waiting on the keyboard, nothing to schedule
	}
	return m.advanceQueue()
}

// ingestEvents translates engine events appended since the last call into
// presentation: action labels land immediately (the actor just acted);
// board cards, reveals and awards become queued steps so they can be paced.
func (m *TableScreen) ingestEvents() {
	if m.hand == nil {
		return
	}
	evs := m.hand.Events()
	for _, e := range evs[m.eventsSeen:] {
		switch e.Kind {
		case engine.EvAction:
			label := m.actionLabel(e)
			m.lastAction[e.Seat] = label
			m.pushRecent(m.seatName(e.Seat) + " " + label)
			if e.Seat == heroSeat {
				// TODO(wire-coach): append the grade to this line once the
				// coach can produce one ("3-bet to 90  A matched").
				m.heroLine = label
			}
		case engine.EvDealBoard:
			// One step per card: the flop flips one card at a time so board
			// texture is read incrementally (§6.1) — then the street pause.
			for range e.Cards {
				m.boardQueued++
				m.queue = append(m.queue, step{kind: stepBoard, n: m.boardQueued, street: e.Street})
			}
			m.queue = append(m.queue, step{kind: stepPause, street: e.Street})
		case engine.EvShowdown:
			m.queue = append(m.queue, step{
				kind: stepReveal, seat: e.Seat, rank: e.Rank, street: e.Street,
			})
		}
	}
	m.eventsSeen = len(evs)

	if m.hand.Phase() == engine.PhaseComplete && !m.completeQueued {
		m.completeQueued = true
		m.queueAwards()
		m.queue = append(m.queue, step{kind: stepDone})
	}
}

// queueAwards turns the hand result into award steps, main pot first —
// side pots resolve on screen one at a time, in the open (§2.5).
func (m *TableScreen) queueAwards() {
	res, ok := m.hand.Result()
	if !ok {
		return
	}
	awards := make([]engine.PotAward, len(res.Awards))
	copy(awards, res.Awards)
	sort.SliceStable(awards, func(i, j int) bool { return awards[i].Pot < awards[j].Pot })

	board := m.hand.Board()
	for _, a := range awards {
		names := make([]string, len(a.Winners))
		for i, s := range a.Winners {
			names[i] = m.seatName(s)
		}
		label := "POT"
		if len(awards) > 1 {
			label = potLabel(a.Pot)
		}
		text := label + " " + a.Amount.String() + " to " + strings.Join(names, " / ")
		var winning []engine.Card
		if a.Rank != 0 {
			text += " (" + strings.ToLower(eval.Rank(a.Rank).String()) + ")"
			if len(board) >= 3 && len(a.Winners) > 0 {
				if hole, ok := m.hand.HoleCards(a.Winners[0]); ok {
					// Best5 identifies WHICH five of seven play — board cards
					// included. Highlighting them teaches "best five of seven".
					_, five := eval.Best5(hole, board)
					winning = five[:]
				}
			}
		}
		m.queue = append(m.queue, step{kind: stepAward, text: text, winning: winning})
	}
}

// potLabel names a pot layer for award text: MAIN, SIDE, SIDE 2, ...
func potLabel(index int) string {
	switch index {
	case 0:
		return "MAIN"
	case 1:
		return "SIDE"
	default:
		return "SIDE " + engine.Chips(index).String()
	}
}

// actionLabel renders an action event as the seat's action-line text.
// Amounts are street totals, matching how the engine (and the TDA) speak.
func (m *TableScreen) actionLabel(e engine.Event) string {
	allIn := m.hand.AllIn().Has(e.Seat)
	switch e.Action {
	case engine.ActionFold:
		return "folds"
	case engine.ActionCheck:
		return "checks"
	case engine.ActionCall:
		if allIn {
			return "all-in " + e.Amount.String()
		}
		return "calls " + e.Amount.String()
	case engine.ActionBet:
		if allIn {
			return "all-in " + e.Amount.String()
		}
		return "bets " + e.Amount.String()
	case engine.ActionRaise:
		if allIn {
			return "all-in " + e.Amount.String()
		}
		return "raises to " + e.Amount.String()
	}
	return ""
}

// pushRecent appends to the wide layout's action ticker.
func (m *TableScreen) pushRecent(s string) {
	m.recent = append(m.recent, s)
	if len(m.recent) > 4 {
		m.recent = m.recent[len(m.recent)-4:]
	}
}

// advanceQueue applies presentation steps: all of them synchronously at
// SpeedInstant, one per tick otherwise. When the queue empties it hands off
// to afterQueue (arm the hero, schedule a villain, or rest between hands).
func (m *TableScreen) advanceQueue() tea.Cmd {
	for {
		if m.paused {
			return nil
		}
		if len(m.queue) == 0 {
			return m.afterQueue()
		}
		st := m.queue[0]
		m.queue = m.queue[1:]
		m.applyStep(st)

		if st.kind == stepPause {
			if m.speed == SpeedLearn {
				// The most important pacing feature is not an animation:
				// unlimited reading time after each street, dismissed by a
				// keypress that is a micro-commitment to having read it (§6.2).
				m.paused = true
				m.status = st.street.String() + " dealt " + theme.G.Dot + " space to continue"
				return nil
			}
			if m.speed != SpeedInstant {
				return m.scheduleStep(scaled(durStreetBeat, m.speed))
			}
			continue
		}
		if m.speed != SpeedInstant {
			return m.scheduleStep(stepDelay(st.kind, m.speed))
		}
	}
}

// scheduleStep arms the next queue tick, invalidating any stale one.
func (m *TableScreen) scheduleStep(d time.Duration) tea.Cmd {
	m.seq++
	return tickMsgAfter(d, stepTickMsg{seq: m.seq})
}

// applyStep mutates presentation state for one step.
func (m *TableScreen) applyStep(st step) {
	switch st.kind {
	case stepBoard:
		firstOfStreet := st.n == 1 || st.n == 4 || st.n == 5
		if firstOfStreet {
			// A new street: previous street's action labels are stale.
			// Folded seats keep "folds" — knowing who is gone matters.
			for s := engine.Seat(0); s.Valid(); s++ {
				if !m.hand.Live().Has(s) {
					continue
				}
				m.lastAction[s] = ""
			}
			if m.hand.Live().Has(heroSeat) {
				m.heroLine = ""
			}
		}
		m.boardShown = st.n
		m.status = "dealing the " + st.street.String()
	case stepReveal:
		m.revealed[st.seat] = true
		desc := strings.ToLower(eval.Rank(st.rank).String())
		m.lastAction[st.seat] = "shows " + desc
		if st.seat == heroSeat {
			m.heroLine = "shows " + desc
		}
		m.status = m.seatName(st.seat) + " shows " + desc
	case stepAward:
		m.awardText = st.text
		m.winners = engine.NewCardSet(st.winning...)
		m.status = st.text
	case stepDone:
		m.finishHand()
	}
}

// finishHand settles stacks onto the table and enters the between-hands
// state. The completed Hand object stays around for rendering and review.
func (m *TableScreen) finishHand() {
	if m.hand != nil && m.hand.Phase() == engine.PhaseComplete {
		_ = m.eng.FinishHand(m.hand)
	}
	m.handDone = true
	m.bar.Wait()
	dot := " " + theme.G.Dot + " "
	m.status = "hand complete" + dot + "space deals the next" + dot + "v reviews"
}

// afterQueue runs when nothing is animating: arm the hero's action bar,
// schedule (or immediately run) a villain's decision, or rest.
func (m *TableScreen) afterQueue() tea.Cmd {
	if m.hand == nil || m.handDone || m.hand.Phase() != engine.PhaseBetting {
		return nil
	}
	cur := m.hand.CurrentSeat()
	if cur == engine.NoSeat {
		return nil
	}
	if cur == heroSeat {
		m.armBar()
		return nil
	}
	if m.speed == SpeedInstant {
		m.applyVillain(cur)
		return m.advanceQueue()
	}
	m.status = m.seatName(cur) + " is thinking" + theme.G.Ellipsis
	m.seq++
	return tickMsgAfter(thinkTime(m.speed, m.hand.ToCall(cur) > 0), villainTickMsg{seq: m.seq})
}

// applyVillain asks a villain for its action and applies it. A misbehaving
// opponent (illegal action, nil) must never wedge the session, so the
// fallback is the engine's own idea of the safest legal move.
func (m *TableScreen) applyVillain(seat engine.Seat) {
	var act engine.Action
	if opp := m.opponents[seat]; opp != nil {
		act = opp.Act(m.hand.View(seat))
	}
	if act == nil || act.Seat() != seat || m.hand.Apply(act) != nil {
		if _, ok := m.hand.LegalActions().Find(engine.ActionCheck); ok {
			_ = m.hand.Apply(engine.Check{S: seat})
		} else {
			_ = m.hand.Apply(engine.Fold{S: seat})
		}
	}
	m.ingestEvents()
}

// armBar puts the action bar in the choosing state for the hero's turn,
// with the live math the design calls table stakes information: facing a
// bet, the hero's own pot odds; otherwise the pot and who is left behind it.
func (m *TableScreen) armBar() {
	legal := m.hand.LegalActions()
	pot := m.hand.PotTotal()
	toCall := m.hand.ToCall(heroSeat)
	committed := m.hand.Committed(heroSeat)
	dot := " " + theme.G.Dot + " "

	var info string
	if toCall > 0 {
		info = "to call " + toCall.String() + dot + "pot odds " + equity.PotOddsText(toCall, pot)
	} else {
		info = "pot " + pot.String()
		if v := m.mainVillain(); v != engine.NoSeat {
			info += dot + m.seatName(v) + " " + m.hand.Stack(v).String() + " behind"
		}
	}
	m.bar.Arm(legal, pot, toCall, committed, m.cfg.BigBlind, info)
	m.status = "Your turn " + theme.G.Dot + " " + m.choicesSummary()
}

// choicesSummary spells the currently visible choices out for the status
// line ("fold, call, or raise").
func (m *TableScreen) choicesSummary() string {
	var names []string
	for _, o := range m.bar.visibleOptions() {
		names = append(names, o.Type.String())
	}
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " or " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + ", or " + names[len(names)-1]
	}
}

// mainVillain picks the opponent the sizing math speaks about: the first
// live seat clockwise from the hero that still has chips to decide with.
func (m *TableScreen) mainVillain() engine.Seat {
	if m.hand == nil {
		return engine.NoSeat
	}
	candidates := (m.hand.Live() &^ m.hand.AllIn()).Remove(heroSeat)
	return candidates.Next(heroSeat)
}

// villainOddsText is the live number the sizing state exists for: the pot
// odds the CURRENT sizing offers the villain — "Nia calls 120 -> gets 2.5:1
// (needs 28%)". It makes "size to deny draw odds" a visible, manipulable
// quantity (§4.3).
func (m *TableScreen) villainOddsText() string {
	v := m.mainVillain()
	if v == engine.NoSeat || m.hand == nil {
		return ""
	}
	total := m.bar.ConfirmAmount() // hero's street-total after this bet/raise
	heroAdds := total - m.hand.Committed(heroSeat)
	call := total - m.hand.Committed(v)
	if stack := m.hand.Stack(v); call > stack {
		call = stack
	}
	if call <= 0 || heroAdds < 0 {
		return ""
	}
	potAfter := m.hand.PotTotal() + heroAdds
	pct := int(equity.PotOdds(call, potAfter)*100 + 0.5)
	return m.seatName(v) + " calls " + call.String() + " " + theme.G.Dot + " gets " +
		equity.OddsToOne(call, potAfter) + " (needs " + engine.Chips(pct).String() + "%)"
}

// seatName is the display name for a seat.
func (m *TableScreen) seatName(s engine.Seat) string {
	if s == heroSeat {
		return heroName
	}
	if name := m.eng.Name(s); name != "" {
		return name
	}
	return villainNames[s]
}

// Update implements tea.Model.
func (m *TableScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case stepTickMsg:
		if msg.seq == m.seq {
			return m, m.advanceQueue()
		}

	case villainTickMsg:
		if msg.seq == m.seq && m.hand != nil && m.hand.Phase() == engine.PhaseBetting {
			if cur := m.hand.CurrentSeat(); cur != engine.NoSeat && cur != heroSeat {
				m.applyVillain(cur)
				return m, m.advanceQueue()
			}
		}

	case tea.KeyMsg:
		cmd, _ := m.help.dispatch(m.keymap(), msg, m.handleAction)
		return m, cmd
	}
	return m, nil
}

// keymap returns the bindings live in the current state — the same data the
// help overlay renders and the legend tests probe.
func (m *TableScreen) keymap() KeyMap {
	switch {
	case m.bar.State() == ActionBarSizing:
		return tableSizingKeys
	case m.paused:
		return tablePauseKeys
	case m.handDone:
		return tableHandDoneKeys
	case m.bar.State() == ActionBarChoosing:
		return tableChoosingKeys
	default:
		return tableWaitingKeys
	}
}

// handleAction performs one key action. Poker actions are gated by the same
// visibility filter the bar renders from, so a key whose keycap is absent
// does nothing.
func (m *TableScreen) handleAction(a KeyAction) (tea.Cmd, bool) {
	switch a {
	case ActCoachCycle:
		m.coachMode = (m.coachMode + 1) % 3
		m.status = "coach: " + m.coachMode.String()
		return nil, true

	case ActSpeedUp:
		if m.speed < SpeedInstant {
			m.speed++
		}
		m.setSpeed()
		return nil, true

	case ActSpeedDown:
		if m.speed > SpeedLearn {
			m.speed--
		}
		m.setSpeed()
		return nil, true

	case ActBack:
		if m.bar.State() == ActionBarSizing {
			m.bar.Cancel()
			m.status = "Your turn " + theme.G.Dot + " " + m.choicesSummary()
			return nil, true
		}
		// Leave for the menu; the session stays cached (App keeps this model
		// until EndSessionMsg). TODO(confirm-leave): §4.2 wants a confirm.
		return Navigate(ScreenMainMenu), true

	case ActContinue:
		if m.paused {
			m.paused = false
			return m.advanceQueue(), true
		}
		if m.handDone {
			return m.startHand(), true
		}
		return nil, true

	case ActReview:
		if m.handDone {
			return NavigateWithData(ScreenHandReview,
				ReviewRequest{ReturnTo: ScreenTable}), true
		}
		return nil, true

	case ActFold:
		return m.heroSimple(engine.ActionFold), true
	case ActCheck:
		return m.heroSimple(engine.ActionCheck), true
	case ActCall:
		return m.heroSimple(engine.ActionCall), true

	case ActBet:
		return m.openSizing(engine.ActionBet), true
	case ActRaise:
		return m.openSizing(engine.ActionRaise), true

	case ActAllIn:
		if m.bar.State() == ActionBarChoosing && m.bar.OpenSizingAllIn() {
			m.status = "all-in armed " + theme.G.Dot + " enter confirms, esc cancels"
		}
		return nil, true

	case ActPreset1, ActPreset2, ActPreset3, ActPreset4, ActPreset5:
		m.bar.Preset(int(a - ActPreset1))
		return nil, true

	case ActDigit0:
		m.bar.TypeDigit('0')
		return nil, true
	case ActDigit6, ActDigit7, ActDigit8, ActDigit9:
		m.bar.TypeDigit(byte('6' + a - ActDigit6))
		return nil, true

	case ActBackspace:
		m.bar.Backspace()
		return nil, true

	case ActLeft:
		m.bar.Nudge(-1)
		return nil, true
	case ActRight:
		m.bar.Nudge(+1)
		return nil, true

	case ActSelect:
		if m.bar.State() != ActionBarSizing {
			return nil, true
		}
		amt := m.bar.ConfirmAmount()
		if m.bar.SizingType() == engine.ActionRaise {
			return m.heroAct(engine.Raise{S: heroSeat, To: amt}), true
		}
		return m.heroAct(engine.Bet{S: heroSeat, Amount: amt}), true
	}
	return nil, false
}

// setSpeed reports the new speed and persists it as the session preference.
func (m *TableScreen) setSpeed() {
	m.status = "speed: " + strings.ToLower(m.speed.String())
	if m.prefs != nil {
		m.prefs.Speed = m.speed
	}
}

// heroSimple applies a no-sizing action (fold/check/call) if its keycap is
// currently on screen; otherwise the key does nothing at all.
func (m *TableScreen) heroSimple(t engine.ActionType) tea.Cmd {
	if !m.bar.Accepts(t) {
		return nil
	}
	switch t {
	case engine.ActionFold:
		return m.heroAct(engine.Fold{S: heroSeat})
	case engine.ActionCheck:
		return m.heroAct(engine.Check{S: heroSeat})
	default:
		return m.heroAct(engine.Call{S: heroSeat})
	}
}

// openSizing enters sizing mode for bet/raise — or submits directly when
// the engine has pinned the size to all-in (§4.3).
func (m *TableScreen) openSizing(t engine.ActionType) tea.Cmd {
	if !m.bar.Accepts(t) {
		return nil
	}
	amt, skipped := m.bar.OpenSizing(t)
	if skipped {
		if t == engine.ActionRaise {
			return m.heroAct(engine.Raise{S: heroSeat, To: amt})
		}
		return m.heroAct(engine.Bet{S: heroSeat, Amount: amt})
	}
	return nil
}

// heroAct applies the hero's decision and resumes the session loop.
func (m *TableScreen) heroAct(a engine.Action) tea.Cmd {
	if m.hand == nil {
		return nil
	}
	if err := m.hand.Apply(a); err != nil {
		// LegalActions gating should make this unreachable; if the engine
		// still objects, say so instead of desyncing.
		m.status = err.Error()
		return nil
	}
	m.bar.Wait()
	m.ingestEvents()
	return m.advanceQueue()
}
