package engine

import "errors"

// Sentinel errors returned by the engine. Callers match with errors.Is.
var (
	// ErrNotYourTurn means the action's seat is not the seat to act.
	ErrNotYourTurn = errors.New("engine: not your turn")

	// ErrIllegalAction means the action type is not available in this state
	// (checking when facing a bet, folding when the hand is over, and so on).
	ErrIllegalAction = errors.New("engine: illegal action")

	// ErrBetSizing means the action's type is legal but its amount is not.
	ErrBetSizing = errors.New("engine: illegal bet size")

	// ErrHandInProgress means a table operation was attempted mid-hand that is
	// only valid between hands (seating, rebuying, sitting out).
	ErrHandInProgress = errors.New("engine: hand in progress")

	// ErrHandComplete means the hand is no longer accepting actions.
	ErrHandComplete = errors.New("engine: hand complete")

	// ErrNotEnoughPlayers means a hand cannot start with fewer than two seats.
	ErrNotEnoughPlayers = errors.New("engine: not enough players")

	// ErrDuplicateCard means a scripted setup specified the same card twice.
	ErrDuplicateCard = errors.New("engine: duplicate card")

	// ErrInvalidSeat means a seat index is out of range or not occupied.
	ErrInvalidSeat = errors.New("engine: invalid seat")

	// ErrInvalidBuyIn means a buy-in or rebuy falls outside the table's limits.
	ErrInvalidBuyIn = errors.New("engine: invalid buy-in")

	// ErrDeckExhausted means more cards were drawn than the deck holds. This is
	// always a programmer error.
	ErrDeckExhausted = errors.New("engine: deck exhausted")
)
