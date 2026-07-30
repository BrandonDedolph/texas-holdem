package rulebased

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// The archetype differentiation tests: if these fail, the archetypes are
// decoration and the teaching claim ("learn to read player types") is
// false. The spot battery is seeded and identical across archetypes, so
// the orderings are deterministic — once green, always green.

// preflopBattery runs every hand class through every open position and a
// BB-vs-BTN-open spot for one personality, returning the tallies.
func preflopBattery(t *testing.T, key string) tally {
	t.Helper()
	strat := NewStrategy(personality(t, key), 99)
	var c tally
	reps := classReps()
	for _, pos := range openPositions {
		for i, hole := range reps {
			d := strat.Decide(unopenedSpot(t, pos, hole, uint64(3000+i)))
			c.note(d.Action)
		}
	}
	for i, hole := range reps {
		d := strat.Decide(bbFacingBTNOpenSpot(t, hole, uint64(7000+i)))
		c.note(d.Action)
	}
	return c
}

// TestVPIPOrdering: across ~1000 identical seeded preflop spots, the share
// of hands voluntarily played orders nit < TAG < LAG < maniac, and the
// station plays the most of all (widest range plus limps).
func TestVPIPOrdering(t *testing.T) {
	if testing.Short() {
		t.Skip("battery test")
	}
	vpip := map[string]float64{}
	pfr := map[string]float64{}
	var station tally
	for _, key := range []string{"nit", "tag", "lag", "maniac", "station"} {
		c := preflopBattery(t, key)
		vpip[key] = c.vpipShare()
		pfr[key] = c.aggrShare()
		if key == "station" {
			station = c
		}
	}
	t.Logf("VPIP: nit %.3f tag %.3f lag %.3f maniac %.3f station %.3f",
		vpip["nit"], vpip["tag"], vpip["lag"], vpip["maniac"], vpip["station"])
	t.Logf("PFR:  nit %.3f tag %.3f lag %.3f maniac %.3f station %.3f",
		pfr["nit"], pfr["tag"], pfr["lag"], pfr["maniac"], pfr["station"])

	if !(vpip["nit"] < vpip["tag"] && vpip["tag"] < vpip["lag"] && vpip["lag"] < vpip["maniac"]) {
		t.Errorf("VPIP ordering broken: nit %.3f tag %.3f lag %.3f maniac %.3f",
			vpip["nit"], vpip["tag"], vpip["lag"], vpip["maniac"])
	}
	if !(pfr["nit"] < pfr["tag"] && pfr["tag"] < pfr["lag"] && pfr["lag"] < pfr["maniac"]) {
		t.Errorf("PFR ordering broken: nit %.3f tag %.3f lag %.3f maniac %.3f",
			pfr["nit"], pfr["tag"], pfr["lag"], pfr["maniac"])
	}
	if vpip["station"] <= vpip["tag"] {
		t.Errorf("station VPIP %.3f not above TAG %.3f", vpip["station"], vpip["tag"])
	}
	if station.calls <= station.aggr {
		t.Errorf("station raised (%d) as much as it called (%d) preflop", station.aggr, station.calls)
	}
}

// TestAggressionOrderingFullHands plays full seeded hands on homogeneous
// tables (six copies of one archetype, identical decks) and orders the
// archetypes by the share of decisions that were bets or raises. The same
// decks for everyone makes this a controlled experiment, not a poker
// result.
func TestAggressionOrderingFullHands(t *testing.T) {
	if testing.Short() {
		t.Skip("full-hand simulation")
	}
	const hands = 30
	aggr := map[string]float64{}
	callsVsRaises := map[string][2]int{}
	for _, key := range []string{"nit", "tag", "lag", "maniac", "station"} {
		p := personality(t, key)
		bots := make(map[engine.Seat]*AI, 6)
		for s := engine.Seat(0); s < 6; s++ {
			bots[s] = New(key, p, int64(s)+1)
		}
		tallies := playHands(t, bots, hands, 424242)
		var total tally
		for _, c := range tallies {
			total.folds += c.folds
			total.checks += c.checks
			total.calls += c.calls
			total.aggr += c.aggr
			total.decisions += c.decisions
		}
		aggr[key] = total.aggrShare()
		callsVsRaises[key] = [2]int{total.calls, total.aggr}
	}
	t.Logf("aggressive share: nit %.3f tag %.3f lag %.3f maniac %.3f station %.3f",
		aggr["nit"], aggr["tag"], aggr["lag"], aggr["maniac"], aggr["station"])

	if !(aggr["nit"] < aggr["tag"] && aggr["tag"] < aggr["lag"] && aggr["lag"] < aggr["maniac"]) {
		t.Errorf("aggression ordering broken: nit %.3f tag %.3f lag %.3f maniac %.3f",
			aggr["nit"], aggr["tag"], aggr["lag"], aggr["maniac"])
	}
	if cr := callsVsRaises["station"]; cr[0] <= cr[1] {
		t.Errorf("station across full hands raised (%d) as much as it called (%d)", cr[1], cr[0])
	}
}
