package engine

import "sort"

// Pot is one layer of the (possibly split) pot.
type Pot struct {
	Amount   Chips
	Eligible SeatSet // live seats that can win this layer
}

// BuildPots derives the pot layers from total per-seat contributions.
//
// Pots are DERIVED, never maintained: the engine records only what each seat
// contributed and who folded, and the pot structure is a pure function of
// those two facts. This eliminates the entire class of "pot got out of sync
// with contributions" bugs, at the cost of an O(n log n) recompute over at
// most six seats — free.
//
// The returned refund is the uncalled excess owed back to the deepest live
// contributor (a bet or raise nobody matched). Folded players' contributions
// are never refunded; their dead money falls into the lowest layers it
// reaches, where it belongs.
func BuildPots(committed [MaxSeats]Chips, folded, live SeatSet) (pots []Pot, refund [MaxSeats]Chips) {
	// Step 1: refund the uncalled excess. The benchmark is the highest
	// contribution among ALL other seats, folded included — a player can fold
	// after committing more than every remaining caller (raise, re-raise,
	// fold), and the re-raiser's bet was still "called" up to that amount.
	top := NoSeat
	for _, s := range live.Seats() {
		if top == NoSeat || committed[s] > committed[top] {
			top = s
		}
	}
	if top != NoSeat {
		var second Chips
		for s := Seat(0); s < MaxSeats; s++ {
			if s != top && committed[s] > second {
				second = committed[s]
			}
		}
		if committed[top] > second {
			refund[top] = committed[top] - second
			committed[top] = second // build layers from the effective amount
		}
	}

	// Step 2: the distinct contribution levels of live players, ascending.
	levels := make([]Chips, 0, MaxSeats)
	for _, s := range live.Seats() {
		if c := committed[s]; c > 0 {
			levels = append(levels, c)
		}
	}
	sort.Slice(levels, func(i, j int) bool { return levels[i] < levels[j] })

	// Step 3: one layer per level. Every seat — folded included — contributes
	// the slice of its commitment that falls inside the layer's band. The
	// TOP layer's band is unbounded: folded money above the highest live
	// level (possible when every deep stack open-folds, leaving only short
	// all-ins live) must land somewhere, and it can never go back to a
	// folder — so it falls to the deepest surviving pot, exactly like an
	// uncontested pot going to the last live player. Live money above the
	// top level cannot exist here; step 1 already refunded it.
	prev := Chips(0)
	for _, lvl := range levels {
		if lvl == prev {
			continue // duplicate level
		}
		cap := lvl
		if lvl == levels[len(levels)-1] {
			cap = Chips(1)<<62 - 1 // top layer: sweep everything above prev
		}
		var p Pot
		for s := Seat(0); s < MaxSeats; s++ {
			c := committed[s]
			if c > cap {
				c = cap
			}
			if c > prev {
				p.Amount += c - prev
			}
			if live.Has(s) && committed[s] >= lvl {
				p.Eligible = p.Eligible.Add(s)
			}
		}
		// Step 4: merge adjacent layers with identical eligibility — purely
		// cosmetic, it keeps the TUI from displaying phantom side pots.
		if n := len(pots); n > 0 && pots[n-1].Eligible == p.Eligible {
			pots[n-1].Amount += p.Amount
		} else {
			pots = append(pots, p)
		}
		prev = lvl
	}
	return pots, refund
}

// PotAward records how one pot layer was paid out.
type PotAward struct {
	// Pot is the layer index (0 = main pot).
	Pot int
	// Amount is the layer's full amount, split across Winners.
	Amount Chips
	// Winners are the tied winners in payout order: clockwise starting from
	// the first seat left of the button.
	Winners []Seat
	// Shares is parallel to Winners and sums to Amount; odd chips land on
	// the earliest winners in payout order.
	Shares []Chips
	// Rank is the winning packed eval.HandRank; 0 when the pot was won
	// uncontested (no showdown). TODO(wire-eval).
	Rank uint32
}

// awardPot splits one pot among its winners. The remainder (at most
// winners−1 chips) goes one chip each to the tied winners in clockwise order
// starting from the first seat left of the button — hold'em's odd-chip rule.
func awardPot(index int, p Pot, winners SeatSet, rank uint32, button Seat) PotAward {
	a := PotAward{Pot: index, Amount: p.Amount, Rank: rank}
	// Order the winners clockwise from the seat after the button.
	for s := winners.Next(button); s != NoSeat; s = winners.Next(s) {
		a.Winners = append(a.Winners, s)
		if len(a.Winners) == winners.Count() {
			break
		}
	}
	n := Chips(len(a.Winners))
	base, odd := p.Amount/n, p.Amount%n
	a.Shares = make([]Chips, len(a.Winners))
	for i := range a.Shares {
		a.Shares[i] = base
		if Chips(i) < odd {
			a.Shares[i]++
		}
	}
	return a
}
