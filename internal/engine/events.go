package engine

import "time"

// EventKind identifies a hand-history event.
type EventKind uint8

// Event kinds. The comments name the fields each kind populates.
const (
	EvPostBlind      EventKind = iota // Seat, Amount, PotAfter
	EvDealHole                        // Seat, Cards (hidden by the UI as needed)
	EvDealBoard                       // Street, Cards
	EvAction                          // Seat, Action, Amount (street-total), PotAfter
	EvRefundUncalled                  // Seat, Amount
	EvShowdown                        // Seat, Cards, Rank
	EvAwardPot                        // Seat, Amount (this winner's share), PotIndex
)

var eventKindNames = [...]string{
	"post-blind", "deal-hole", "deal-board", "action",
	"refund-uncalled", "showdown", "award-pot",
}

// String is the lowercase name of the event kind.
func (k EventKind) String() string {
	if int(k) >= len(eventKindNames) {
		return "?"
	}
	return eventKindNames[k]
}

// Event is one entry in a hand's append-only log. Every state change appends
// one — this is the raw material for the post-hand review and the coach's
// grading, so it is engine-owned, never reconstructed by the UI.
type Event struct {
	Kind     EventKind
	Street   Street
	Seat     Seat
	Action   ActionType
	Amount   Chips
	PotAfter Chips
	Cards    []Card
	// Rank is the packed eval.HandRank of a showdown hand. The engine treats
	// it as an opaque ordered value supplied by its Evaluator; the eval
	// package decodes it back into English.
	// TODO(wire-eval): retype as eval.HandRank at the app layer if desired.
	Rank uint32
	// PotIndex is the pot layer an EvAwardPot event pays from (0 = main pot).
	PotIndex int
}

// SeatInfo describes one participant in a recorded hand.
type SeatInfo struct {
	Seat Seat
	Name string
	// Personality is the AI archetype key; "" means the human player.
	Personality   string
	StartingStack Chips
}

// HandRecord is the persisted wrapper around one hand's event log
// (DESIGN.md §2.6). The engine defines the shape; the review/profile layers
// assign IDs (ULIDs) and persist it.
type HandRecord struct {
	ID     string
	Time   time.Time
	Blinds Blinds
	Seats  []SeatInfo
	Events []Event // the engine's log, verbatim
}
