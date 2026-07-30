package engine

// bettingState is one street's betting round. Its fields define the rules:
//
//   - CurrentBet is the highest total committed this street. Preflop it
//     starts at the big blind even if the big blind posted short — the BB
//     amount is the price of entry by rule.
//   - LastFullRaiseSz / LastFullRaiseTo describe the last FULL bet or raise.
//     An all-in for more than CurrentBet but less than a full raise moves
//     CurrentBet without touching these — that is the incomplete-raise rule.
//   - ActedAtBet[seat] is what CurrentBet was when the seat last acted this
//     street; -1 means it has not acted. Posting a blind is not acting,
//     which is exactly what gives the big blind its option.
//   - ToAct is the set of seats that still owe a decision. The round closes
//     when it is empty.
//
// A seat may raise only if ActedAtBet[seat] < LastFullRaiseTo — it has not
// yet acted, or a full raise has occurred since it last acted. This single
// inequality encodes the whole reopening rule.
type bettingState struct {
	CurrentBet      Chips
	LastFullRaiseSz Chips
	LastFullRaiseTo Chips
	Committed       [MaxSeats]Chips
	ActedAtBet      [MaxSeats]Chips
	ToAct           SeatSet
}

// reset prepares the state for a new street. Preflop is initialized by the
// hand after blinds post (CurrentBet = BB, LastFullRaiseSz = BB,
// LastFullRaiseTo = BB); postflop the street opens unbet, with the minimum
// bet — one big blind — seeding LastFullRaiseSz so that min-raise arithmetic
// works uniformly (min raise-to is always CurrentBet + LastFullRaiseSz).
func (b *bettingState) reset(preflop bool, bigBlind Chips) {
	*b = bettingState{LastFullRaiseSz: bigBlind}
	if preflop {
		b.CurrentBet = bigBlind
		b.LastFullRaiseTo = bigBlind
	}
	for s := range b.ActedAtBet {
		b.ActedAtBet[s] = -1
	}
}

// mayRaise reports whether the seat's raising rights are open: it has not
// acted at all this street, or a full raise arrived since it last acted.
// Seats facing only an incomplete all-in they have already acted against get
// fold/call only.
func (b *bettingState) mayRaise(seat Seat) bool {
	return b.ActedAtBet[seat] < b.LastFullRaiseTo
}

// options computes the acting seat's legal choices. stack is the seat's
// chips behind; canRespond reports whether at least one other live seat with
// chips remains — with every opponent all-in (or none left), betting and
// raising are pointless (any excess would be returned uncalled) and are not
// offered.
func (b *bettingState) options(seat Seat, stack, bigBlind Chips, canRespond bool) ActionOptions {
	// Fold is always available while a decision is pending. Folding instead
	// of checking is bad poker but legal poker.
	opts := ActionOptions{{Type: ActionFold}}

	toCall := b.CurrentBet - b.Committed[seat]
	if toCall <= 0 {
		opts = append(opts, ActionOption{Type: ActionCheck})
	} else {
		c := toCall
		if c > stack {
			c = stack // all-in call for less
		}
		opts = append(opts, ActionOption{Type: ActionCall, Min: c, Max: c})
	}

	if !canRespond {
		return opts
	}
	if b.CurrentBet == 0 {
		// Opening bet: one big blind minimum, or all-in for less; no cap —
		// table stakes is the only ceiling. Amounts are street totals, and
		// Committed[seat] is necessarily 0 on an unbet street.
		minBet := bigBlind
		if minBet > stack {
			minBet = stack
		}
		opts = append(opts, ActionOption{Type: ActionBet, Min: minBet, Max: stack})
	} else if allInTo := stack + b.Committed[seat]; allInTo > b.CurrentBet && b.mayRaise(seat) {
		minTo := b.CurrentBet + b.LastFullRaiseSz
		if minTo > allInTo {
			minTo = allInTo // short all-in "raise": Min = Max = all-in total
		}
		opts = append(opts, ActionOption{Type: ActionRaise, Min: minTo, Max: allInTo})
	}
	return opts
}

// recordCheckOrCall marks the seat as having acted at the current price.
func (b *bettingState) recordCheckOrCall(seat Seat) {
	b.ActedAtBet[seat] = b.CurrentBet
	b.ToAct = b.ToAct.Remove(seat)
}

// recordAggression applies a bet or raise to street-total `to`, reopening
// action for the given other seats (every live, non-all-in seat). full says
// whether it met the minimum bet/raise; an incomplete all-in moves the price
// but leaves LastFullRaiseSz/To untouched, so seats that already acted keep
// closed raising rights (mayRaise stays false for them).
func (b *bettingState) recordAggression(seat Seat, to Chips, full bool, reopen SeatSet) {
	if full {
		b.LastFullRaiseSz = to - b.CurrentBet
		b.LastFullRaiseTo = to
	}
	b.CurrentBet = to
	b.ActedAtBet[seat] = to
	b.ToAct = reopen.Remove(seat)
}
