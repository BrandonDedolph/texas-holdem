package coach

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/ai"
	"github.com/BrandonDedolph/texas-holdem/internal/engine"
	"github.com/BrandonDedolph/texas-holdem/internal/profile"
)

// TestMomentRegistryComplete pins the §5.4 registry: all fourteen IDs
// exist exactly once, decision-time moments have triggers, review-scoped
// moments (which need showdown information) do not, and every body
// renders without a live view.
func TestMomentRegistryComplete(t *testing.T) {
	reviewScoped := map[string]bool{
		"first_kicker_showdown": true,
		"first_dominated":       true,
		"first_split_pot":       true,
	}
	want := []string{
		"first_flush_draw", "first_oesd", "first_gutshot",
		"first_button_raise_hand", "first_great_price", "first_cbet_spot",
		"first_facing_3bet", "first_kicker_showdown", "first_dominated",
		"first_station_bluff_warning", "first_pot_committed",
		"first_counterfeit", "first_split_pot", "first_semibluff",
	}
	byID := map[string]*Moment{}
	for _, m := range Moments() {
		if byID[m.ID] != nil {
			t.Errorf("duplicate moment ID %q", m.ID)
		}
		byID[m.ID] = m
	}
	if len(byID) != len(want) {
		t.Errorf("registry has %d moments, want %d", len(byID), len(want))
	}
	for _, id := range want {
		m := byID[id]
		if m == nil {
			t.Errorf("moment %q missing", id)
			continue
		}
		if reviewScoped[id] != (m.Trigger == nil) {
			t.Errorf("moment %q trigger presence wrong (review-scoped=%v)", id, reviewScoped[id])
		}
		if m.Title == "" || m.Body == nil || m.Body(nil, Advice{}) == "" {
			t.Errorf("moment %q has no renderable body", id)
		}
	}
}

// TestMomentPriorityFireOncePersist covers the §5.4 semantics in one flow.
// The spot triggers two moments at once (a 6:1 price AND a first flush
// draw); exactly one — the higher priority — fires per decision, each
// fires exactly once, and firing persists to the profile forever.
func TestMomentPriorityFireOncePersist(t *testing.T) {
	prof := profile.NewProfile()
	c := New(prof, 3)

	// 10 into 60: required ~14% (better than 4:1), hero on a flush draw.
	h := flushDrawFacingBet(t, "Ks 7s 2h", "Kd Qd", 10, 21)
	v := heroView(t, h, 0)
	adv := c.Advise(v)

	m1 := c.PendingMoment(v, adv)
	if m1 == nil || m1.ID != "first_great_price" {
		t.Fatalf("first firing = %v, want first_great_price (the higher priority)", id(m1))
	}
	if body := m1.Body(v, adv); body == "" {
		t.Error("great-price body empty")
	}

	// Same decision presented again: the price moment is spent, the flush
	// draw — also triggered — gets its turn. One moment per decision.
	m2 := c.PendingMoment(v, adv)
	if m2 == nil || m2.ID != "first_flush_draw" {
		t.Fatalf("second firing = %v, want first_flush_draw", id(m2))
	}
	if m3 := c.PendingMoment(v, adv); m3 != nil {
		t.Fatalf("third firing = %v, want none left", m3.ID)
	}

	// Persisted, not per-session: a brand-new Coach on the same profile
	// still refuses to re-fire either.
	if _, seen := prof.MomentsSeen["first_great_price"]; !seen {
		t.Error("first_great_price not persisted")
	}
	if _, seen := prof.MomentsSeen["first_flush_draw"]; !seen {
		t.Error("first_flush_draw not persisted")
	}
	c2 := New(prof, 99)
	if m := c2.PendingMoment(v, adv); m != nil {
		t.Errorf("fresh coach on same profile re-fired %q", m.ID)
	}

	// A fresh player (fresh profile) meets the moment for the first time.
	c3 := New(profile.NewProfile(), 3)
	if m := c3.PendingMoment(v, adv); m == nil || m.ID != "first_great_price" {
		t.Errorf("fresh profile got %v, want first_great_price", id(m))
	}
}

// TestMomentTriggers walks each decision-time moment through a spot built
// to be its first sighting, and asserts it is the one that fires.
func TestMomentTriggers(t *testing.T) {
	cases := []struct {
		name string
		want string
		spot func(t *testing.T, c *Coach) (*engine.PlayerView, Advice)
	}{
		{
			name: "oesd facing a bet",
			want: "first_oesd",
			spot: func(t *testing.T, c *Coach) (*engine.PlayerView, Advice) {
				h := huHand(t, "9c 8d", "Ah Qh", "7h 6s 2c", 1000, 31,
					engine.Raise{S: 0, To: 25}, engine.Call{S: 1}, engine.Bet{S: 1, Amount: 25})
				v := heroView(t, h, 0)
				return v, c.Advise(v)
			},
		},
		{
			name: "gutshot facing a bet",
			want: "first_gutshot",
			spot: func(t *testing.T, c *Coach) (*engine.PlayerView, Advice) {
				h := huHand(t, "9c 8d", "Ah Qh", "6h 5s 2c", 1000, 32,
					engine.Raise{S: 0, To: 25}, engine.Call{S: 1}, engine.Bet{S: 1, Amount: 25})
				v := heroView(t, h, 0)
				return v, c.Advise(v)
			},
		},
		{
			name: "checked-to as the preflop aggressor",
			want: "first_cbet_spot",
			spot: func(t *testing.T, c *Coach) (*engine.PlayerView, Advice) {
				h := huHand(t, "Ah Kd", "Qd Jd", "7c 2d 9h", 1000, 33,
					engine.Raise{S: 0, To: 25}, engine.Call{S: 1}, engine.Check{S: 1})
				v := heroView(t, h, 0)
				return v, c.Advise(v)
			},
		},
		{
			name: "button with a chart-raising hand",
			want: "first_button_raise_hand",
			spot: func(t *testing.T, c *Coach) (*engine.PlayerView, Advice) {
				h := sixHand(t, map[engine.Seat][2]engine.Card{0: engine.Holes("Ad Jd")}, 34,
					engine.Fold{S: 3}, engine.Fold{S: 4}, engine.Fold{S: 5})
				v := heroView(t, h, 0)
				return v, c.Advise(v)
			},
		},
		{
			name: "open raise gets 3-bet",
			want: "first_facing_3bet",
			spot: func(t *testing.T, c *Coach) (*engine.PlayerView, Advice) {
				h := sixHand(t, map[engine.Seat][2]engine.Card{3: engine.Holes("Ah Kh")}, 35,
					engine.Raise{S: 3, To: 25}, engine.Fold{S: 4}, engine.Fold{S: 5},
					engine.Raise{S: 0, To: 75}, engine.Fold{S: 1}, engine.Fold{S: 2})
				v := heroView(t, h, 3)
				return v, c.Advise(v)
			},
		},
		{
			name: "calling leaves under a pot behind",
			want: "first_pot_committed",
			spot: func(t *testing.T, c *Coach) (*engine.PlayerView, Advice) {
				h := huHand(t, "Ah Kd", "Qd Jd", "7c 2d 9h", 150, 36,
					engine.Raise{S: 0, To: 50}, engine.Call{S: 1}, engine.Bet{S: 1, Amount: 60})
				v := heroView(t, h, 0)
				return v, c.Advise(v)
			},
		},
		{
			name: "two pair counterfeited on the river",
			want: "first_counterfeit",
			spot: func(t *testing.T, c *Coach) (*engine.PlayerView, Advice) {
				h := huHand(t, "9c 7d", "Ad Qd", "9h 7s 2c Kd Kh", 1000, 37,
					engine.Raise{S: 0, To: 25}, engine.Call{S: 1},
					engine.Check{S: 1}, engine.Check{S: 0},
					engine.Check{S: 1}, engine.Check{S: 0},
					engine.Check{S: 1})
				v := heroView(t, h, 0)
				return v, c.Advise(v)
			},
		},
		{
			name: "about to bluff a calling station",
			want: "first_station_bluff_warning",
			spot: func(t *testing.T, c *Coach) (*engine.PlayerView, Advice) {
				c.SetRead(1, ai.Archetypes["station"])
				h := bustedComboRiverSpot(t, 38)
				v := heroView(t, h, 0)
				return v, c.Advise(v)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := New(nil, 8)
			v, adv := tc.spot(t, c)
			m := c.PendingMoment(v, adv)
			if m == nil || m.ID != tc.want {
				t.Fatalf("fired %v, want %s", id(m), tc.want)
			}
			if m.Body(v, adv) == "" {
				t.Errorf("%s body empty", tc.want)
			}
		})
	}
}

// TestSemibluffMoment: fires when the coach recommends betting a draw. The
// recommendation is frequency-mixed, so hunt for a seed where the baseline
// takes the semi-bluff line — determinism per seed makes this stable.
func TestSemibluffMoment(t *testing.T) {
	for seed := int64(0); seed < 60; seed++ {
		c := New(nil, seed)
		h := huHand(t, "9s 8s", "Ah Qh", "Ks 7s 6h", 1000, 39,
			engine.Raise{S: 0, To: 25}, engine.Call{S: 1}, engine.Check{S: 1})
		v := heroView(t, h, 0)
		adv := c.Advise(v)
		f, ok := adv.Decision.Rationale.Get(ai.FactSizing)
		if !ok || f.(ai.SizingFact).Purpose != "semi-bluff" {
			continue
		}
		m := c.PendingMoment(v, adv)
		if m == nil || m.ID != "first_semibluff" {
			t.Fatalf("semi-bluff advice fired %v, want first_semibluff", id(m))
		}
		return
	}
	t.Fatal("no seed in 0..59 produced a semi-bluff recommendation; mixing may be broken")
}

// TestFireMoment covers the review-scoped path: fired by ID, once, with a
// body that needs no live view.
func TestFireMoment(t *testing.T) {
	prof := profile.NewProfile()
	c := New(prof, 1)

	m := c.FireMoment("first_split_pot")
	if m == nil || m.ID != "first_split_pot" {
		t.Fatalf("FireMoment returned %v", id(m))
	}
	if m.Body(nil, Advice{}) == "" {
		t.Error("review-scoped body must render without a view")
	}
	if c.FireMoment("first_split_pot") != nil {
		t.Error("review-scoped moment fired twice")
	}
	if _, seen := prof.MomentsSeen["first_split_pot"]; !seen {
		t.Error("FireMoment did not persist")
	}
	if c.FireMoment("no_such_moment") != nil {
		t.Error("unknown ID fired")
	}
}

// id renders a moment pointer for failure messages.
func id(m *Moment) string {
	if m == nil {
		return "<none>"
	}
	return m.ID
}
