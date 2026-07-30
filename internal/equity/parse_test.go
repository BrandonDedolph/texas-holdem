package equity

import (
	"testing"
)

func TestParseComboCounts(t *testing.T) {
	cases := []struct {
		spec string
		want float64
	}{
		{"JJ", 6},
		{"AQs", 4},
		{"KQo", 12},
		{"AQ", 16},
		{"JJ+", 24},          // JJ QQ KK AA
		{"22+", 78},          // all pairs
		{"66-99", 24},        // pair run, ascending notation
		{"99-66", 24},        // and descending
		{"A2s+", 48},         // all suited aces
		{"AQs+", 8},          // AQs AKs
		{"ATo+", 48},         // ATo AJo AQo AKo
		{"T9s-54s", 24},      // suited-connector run, six cells
		{"54s-T9s", 24},      // either end first
		{"AQo-A9o", 48},      // shared-high-card run
		{"Ah Kh", 1},         // exact combo with the grammar's space
		{"AhKh", 1},          // and without
		{"[50]AQo", 6},       // weight prefix halves the mass
		{"AQo, [50]AQo", 12}, // duplicates take the max weight
		{"[25]JJ, [75]JJ", 4.5},
		{"22+, A2s+, K9s+, QTs+, ATo+, KQo", 210},
	}
	for _, tc := range cases {
		r := mustRange(t, tc.spec)
		if got := r.CountCombos(); !approx(got, tc.want, 1e-4) {
			t.Errorf("ParseRange(%q).CountCombos() = %v, want %v", tc.spec, got, tc.want)
		}
	}
}

// TestParseRealisticChart checks a chart a lesson would actually use lands
// in a plausible opening-range band, so a grammar bug cannot silently turn
// "a 16% range" into something absurd.
func TestParseRealisticChart(t *testing.T) {
	r := mustRange(t, "22+, A2s+, K9s+, QTs+, ATo+, KQo")
	pct := r.Percent()
	if pct < 13 || pct > 19 {
		t.Fatalf("chart is %.1f%% of hands, want a plausible 13-19%%", pct)
	}
}

// TestStringRoundTrip: ParseRange → String → ParseRange must reproduce the
// exact weights, and String must be a fixed point (canonical form).
func TestStringRoundTrip(t *testing.T) {
	specs := []string{
		"JJ",
		"22+",
		"JJ+, AQs+, KQo",
		"66-99, T9s-54s",
		"22+, A2s+, K9s+, QTs+, ATo+, KQo",
		"[50]AQo, [75]JJ, AhKh",
		"AA, [50]KK, QhQd",
		"A9s-A5s, KQo-KTo",
	}
	for _, spec := range specs {
		r1 := mustRange(t, spec)
		s1 := r1.String()
		r2 := mustRange(t, s1)
		if r1.W != r2.W {
			t.Errorf("round trip of %q via %q changed weights", spec, s1)
		}
		if s2 := r2.String(); s2 != s1 {
			t.Errorf("String not canonical for %q: %q then %q", spec, s1, s2)
		}
	}
}

// TestStringMergesRuns pins a few canonical spellings so serialization
// stays readable, not just round-trippable.
func TestStringMergesRuns(t *testing.T) {
	cases := []struct{ spec, want string }{
		{"QQ, KK, AA", "QQ+"},
		{"99, 88, 77, 66", "99-66"},
		{"AKs, AQs, ATs", "AQs+, A10s"}, // the ten serializes as "10", never "T"
		{"QQ, JJ, TT", "QQ-1010"},
		{"A10s, KTs", "A10s, K10s"}, // both input spellings, one output
		{"A2s, A3s, A4s, A5s", "A5s-A2s"},
		{"AhKh", "AhKh"},
		{"[50]KQo", "[50]KQo"},
	}
	for _, tc := range cases {
		if got := mustRange(t, tc.spec).String(); got != tc.want {
			t.Errorf("String of %q = %q, want %q", tc.spec, got, tc.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	bad := []string{
		"XX",       // not a rank
		"AAs",      // pairs have no suitedness
		"AQx",      // bad selector
		"T9s-AKo",  // shape mismatch
		"T9s-K5s",  // neither shared high card nor shared gap
		"[150]AQo", // weight out of range
		"[50AQo",   // unterminated prefix
		"AhAh",     // combo repeats a card
		"A",        // too short
	}
	for _, spec := range bad {
		if _, err := ParseRange(spec); err == nil {
			t.Errorf("ParseRange(%q) succeeded, want error", spec)
		}
	}
}

func approx(got, want, tol float64) bool {
	d := got - want
	return d >= -tol && d <= tol
}
