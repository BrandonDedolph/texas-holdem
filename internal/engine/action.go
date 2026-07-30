package engine

// ActionType identifies a kind of player decision.
type ActionType uint8

// Action types. Bet opens a street where CurrentBet == 0; Raise increases a
// nonzero CurrentBet (preflop opens are raises over the big blind).
const (
	ActionFold ActionType = iota
	ActionCheck
	ActionCall
	ActionBet
	ActionRaise
)

var actionNames = [...]string{"fold", "check", "call", "bet", "raise"}

// String is the lowercase name of the action type ("raise").
func (t ActionType) String() string {
	if int(t) >= len(actionNames) {
		return "?"
	}
	return actionNames[t]
}

// Action is a player decision applied to a Hand. There are deliberately no
// posting or dealing actions: blinds and cards are compulsory and
// deterministic, so they are engine-applied side effects visible as events,
// never as decisions. An Action is always a *choice* — which is what keeps
// LegalActions meaning exactly "the choices".
type Action interface {
	Type() ActionType
	Seat() Seat
}

// Fold gives up the hand.
type Fold struct{ S Seat }

// Check passes when no chips are owed.
type Check struct{ S Seat }

// Call matches the current bet. The amount is computed by the engine, so
// callers can never submit a stale amount.
type Call struct{ S Seat }

// Bet opens the betting on a street where nothing has been wagered.
// Amount is the street-total commitment.
type Bet struct {
	S      Seat
	Amount Chips
}

// Raise increases the current bet. To is raise-TO — the total street
// commitment becomes To — not raise-by. This matches how players and the TDA
// express raises and avoids the classic ambiguity bug.
type Raise struct {
	S  Seat
	To Chips
}

// Type implements Action.
func (a Fold) Type() ActionType { return ActionFold }

// Seat implements Action.
func (a Fold) Seat() Seat { return a.S }

// Type implements Action.
func (a Check) Type() ActionType { return ActionCheck }

// Seat implements Action.
func (a Check) Seat() Seat { return a.S }

// Type implements Action.
func (a Call) Type() ActionType { return ActionCall }

// Seat implements Action.
func (a Call) Seat() Seat { return a.S }

// Type implements Action.
func (a Bet) Type() ActionType { return ActionBet }

// Seat implements Action.
func (a Bet) Seat() Seat { return a.S }

// Type implements Action.
func (a Raise) Type() ActionType { return ActionRaise }

// Seat implements Action.
func (a Raise) Seat() Seat { return a.S }

// ActionOption describes one legal action type with its sizing bounds.
// No-limit sizing is continuous, so legality is a range, not an enumeration:
//
//   - Fold/Check: Min = Max = 0.
//   - Call:       Min = Max = chips required to call, capped at the acting
//     stack (an all-in call for less).
//   - Bet:        Min = minimum bet (one big blind, or all-in for less),
//     Max = stack. Amount is the street-total commitment.
//   - Raise:      Min = minimum legal raise-to, Max = raise-to all-in.
//     If the seat's raising rights are closed (the incomplete-raise rule),
//     Raise is simply absent from the slice.
type ActionOption struct {
	Type     ActionType
	Min, Max Chips
}

// ActionOptions is the slice LegalActions returns. It is a named slice type
// only so the DESIGN.md §2.3 helper methods (Find, CallAmount) can exist;
// consumers treat it as a plain []ActionOption.
type ActionOptions []ActionOption

// Find returns the option of the given type, if present.
func (opts ActionOptions) Find(t ActionType) (ActionOption, bool) {
	for _, o := range opts {
		if o.Type == t {
			return o, true
		}
	}
	return ActionOption{}, false
}

// CallAmount returns the chips required to call, or 0 when there is nothing
// to call (checking is legal, or no decision is pending).
func (opts ActionOptions) CallAmount() Chips {
	if o, ok := opts.Find(ActionCall); ok {
		return o.Min
	}
	return 0
}
