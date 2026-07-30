package equity

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// ParseRange parses the range grammar of docs/design-learning.md §3.1:
// comma-separated terms, whitespace ignored, duplicate terms take the max
// weight. Terms:
//
//	JJ          one pocket pair (6 combos)
//	JJ+         JJ, QQ, KK, AA
//	66-99       pair run (either end may be written first)
//	AQs / KQo   suited (4) / offsuit (12) combos
//	AQ          both (16)
//	AQs+        walk the kicker up to the rank below the first card
//	A2s+        all suited aces
//	T9s-54s     run with a constant gap (suited connectors etc.)
//	AQo-A9o     run sharing the first card
//	AhKh        one exact combo
//	[50]AQo     weight prefix, 0–100 percent, applies to one term
func ParseRange(s string) (Range, error) {
	var r Range
	if err := r.Add(s, 1); err != nil {
		return Range{}, err
	}
	return r, nil
}

// Add parses spec with the ParseRange grammar and merges it into the range,
// scaling every parsed weight by weight (so r.Add("AQo", 0.5) adds AQo at
// half weight). Duplicates take the max, matching ParseRange.
func (r *Range) Add(spec string, weight float32) error {
	for _, raw := range strings.Split(spec, ",") {
		term := stripSpace(raw)
		if term == "" {
			continue
		}
		if err := r.addTerm(term, weight); err != nil {
			return fmt.Errorf("equity: range term %q: %w", strings.TrimSpace(raw), err)
		}
	}
	return nil
}

// stripSpace removes all whitespace — the grammar ignores it entirely, which
// is what lets "Ah Kh" be one exact-combo term.
func stripSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

func (r *Range) addTerm(term string, scale float32) error {
	w := float32(1)
	if strings.HasPrefix(term, "[") {
		end := strings.IndexByte(term, ']')
		if end < 0 {
			return fmt.Errorf("unterminated weight prefix")
		}
		n, err := strconv.Atoi(term[1:end])
		if err != nil || n < 0 || n > 100 {
			return fmt.Errorf("weight prefix must be an integer 0-100")
		}
		w = float32(n) / 100
		term = term[end+1:]
	}
	w *= scale
	combos, err := expandTerm(term)
	if err != nil {
		return err
	}
	for _, c := range combos {
		if w > r.W[c] {
			r.W[c] = w
		}
	}
	return nil
}

// handClass is one 169-grid cell class: a pair, or two ranks with a
// suited/offsuit selector ("AQ" selects both).
type handClass struct {
	pair            bool
	hi, lo          engine.Rank
	suited, offsuit bool
}

func (h handClass) combos() []Combo {
	if h.pair {
		return pairCombos(h.hi)
	}
	var out []Combo
	if h.suited {
		out = append(out, suitedCombos(h.hi, h.lo)...)
	}
	if h.offsuit {
		out = append(out, offsuitCombos(h.hi, h.lo)...)
	}
	return out
}

func expandTerm(term string) ([]Combo, error) {
	if term == "" {
		return nil, fmt.Errorf("empty term")
	}
	if i := strings.IndexByte(term, '-'); i >= 0 {
		return expandRun(term[:i], term[i+1:])
	}
	if strings.HasSuffix(term, "+") {
		return expandPlus(term[:len(term)-1])
	}
	if len(term) == 4 && isSuitByte(term[1]) && isSuitByte(term[3]) {
		a, err := engine.ParseCard(term[:2])
		if err != nil {
			return nil, err
		}
		b, err := engine.ParseCard(term[2:])
		if err != nil {
			return nil, err
		}
		if a == b {
			return nil, fmt.Errorf("combo repeats a card")
		}
		return []Combo{MakeCombo(a, b)}, nil
	}
	h, err := parseClass(term)
	if err != nil {
		return nil, err
	}
	return h.combos(), nil
}

// expandPlus walks a class upward: pairs to AA, non-pairs by kicker up to
// the rank below the first card ("A2s+" is A2s..AKs).
func expandPlus(base string) ([]Combo, error) {
	h, err := parseClass(base)
	if err != nil {
		return nil, err
	}
	var out []Combo
	if h.pair {
		for r := h.hi; r <= engine.Ace; r++ {
			out = append(out, pairCombos(r)...)
		}
		return out, nil
	}
	for lo := h.lo; lo < h.hi; lo++ {
		c := h
		c.lo = lo
		out = append(out, c.combos()...)
	}
	return out, nil
}

// expandRun walks between two classes of the same shape. Pairs walk the
// rank ("66-99"). Non-pairs must either share the first card ("AQo-A9o",
// walking the kicker) or share the gap ("T9s-54s", walking both ranks).
func expandRun(a, b string) ([]Combo, error) {
	ca, err := parseClass(a)
	if err != nil {
		return nil, err
	}
	cb, err := parseClass(b)
	if err != nil {
		return nil, err
	}
	if ca.pair != cb.pair || ca.suited != cb.suited || ca.offsuit != cb.offsuit {
		return nil, fmt.Errorf("run endpoints %q and %q are different shapes", a, b)
	}
	var out []Combo
	if ca.pair {
		lo, hi := ca.hi, cb.hi
		if lo > hi {
			lo, hi = hi, lo
		}
		for r := lo; r <= hi; r++ {
			out = append(out, pairCombos(r)...)
		}
		return out, nil
	}
	if ca.hi == cb.hi {
		lo, hi := ca.lo, cb.lo
		if lo > hi {
			lo, hi = hi, lo
		}
		for k := lo; k <= hi; k++ {
			c := ca
			c.lo = k
			out = append(out, c.combos()...)
		}
		return out, nil
	}
	if ca.hi-ca.lo == cb.hi-cb.lo {
		gap := ca.hi - ca.lo
		lo, hi := ca.hi, cb.hi
		if lo > hi {
			lo, hi = hi, lo
		}
		for top := lo; top <= hi; top++ {
			c := ca
			c.hi, c.lo = top, top-gap
			out = append(out, c.combos()...)
		}
		return out, nil
	}
	return nil, fmt.Errorf("run endpoints %q and %q share neither a high card nor a gap", a, b)
}

func parseClass(t string) (handClass, error) {
	var h handClass
	if len(t) != 2 && len(t) != 3 {
		return h, fmt.Errorf("bad hand class %q", t)
	}
	r1, err := engine.ParseRank(t[0])
	if err != nil {
		return h, err
	}
	r2, err := engine.ParseRank(t[1])
	if err != nil {
		return h, err
	}
	if r1 < r2 {
		r1, r2 = r2, r1
	}
	h.hi, h.lo = r1, r2
	if r1 == r2 {
		h.pair = true
		if len(t) == 3 {
			return h, fmt.Errorf("a pair cannot be suited or offsuit")
		}
		return h, nil
	}
	switch {
	case len(t) == 2:
		h.suited, h.offsuit = true, true
	case t[2] == 's' || t[2] == 'S':
		h.suited = true
	case t[2] == 'o' || t[2] == 'O':
		h.offsuit = true
	default:
		return h, fmt.Errorf("hand class %q must end in 's' or 'o'", t)
	}
	return h, nil
}

func isSuitByte(b byte) bool {
	switch b {
	case 'c', 'd', 'h', 's', 'C', 'D', 'H', 'S':
		return true
	}
	return false
}

// String re-serializes the range canonically: pair cells first (merged into
// "QQ+" / "99-66" runs), then suited cells by first rank ("ATs+",
// "A9s-A5s"), then offsuit, then any combo that is not part of a uniformly
// weighted cell, as an exact term. Non-unit weights get a "[NN]" prefix,
// quantized to whole percent — which is lossless for anything ParseRange
// produced, so ParseRange → String → ParseRange round-trips exactly.
func (r Range) String() string {
	var terms []string
	covered := make([]bool, NumCombos)

	// cellWeight reports whether every combo of a cell carries the same
	// nonzero weight; only such cells may be written in class notation.
	cellWeight := func(combos []Combo) (float32, bool) {
		w := r.W[combos[0]]
		for _, c := range combos[1:] {
			if r.W[c] != w {
				return 0, false
			}
		}
		return w, w > 0
	}
	mark := func(combos []Combo) {
		for _, c := range combos {
			covered[c] = true
		}
	}
	emit := func(term string, w float32) {
		if n := int(w*100 + 0.5); n != 100 {
			term = "[" + strconv.Itoa(n) + "]" + term
		}
		terms = append(terms, term)
	}
	letter := func(rk engine.Rank) string { return string(rk.Letter()) }

	// Pairs, Ace down, merged into runs of equal weight.
	for hi := int(engine.Ace); hi >= 0; hi-- {
		w, ok := cellWeight(pairCombos(engine.Rank(hi)))
		if !ok {
			continue
		}
		lo := hi
		for lo > 0 {
			w2, ok2 := cellWeight(pairCombos(engine.Rank(lo - 1)))
			if !ok2 || w2 != w {
				break
			}
			lo--
		}
		for rk := lo; rk <= hi; rk++ {
			mark(pairCombos(engine.Rank(rk)))
		}
		hiL, loL := letter(engine.Rank(hi)), letter(engine.Rank(lo))
		switch {
		case hi == lo:
			emit(hiL+hiL, w)
		case hi == int(engine.Ace):
			emit(loL+loL+"+", w)
		default:
			emit(hiL+hiL+"-"+loL+loL, w)
		}
		hi = lo // skip what the run consumed
	}

	// Suited then offsuit cells, first rank Ace down, kickers merged into
	// runs of equal weight.
	for _, suited := range []bool{true, false} {
		cell := func(hi, lo engine.Rank) []Combo {
			if suited {
				return suitedCombos(hi, lo)
			}
			return offsuitCombos(hi, lo)
		}
		tag := "o"
		if suited {
			tag = "s"
		}
		for hi := int(engine.Ace); hi >= 1; hi-- {
			for k := hi - 1; k >= 0; k-- {
				w, ok := cellWeight(cell(engine.Rank(hi), engine.Rank(k)))
				if !ok {
					continue
				}
				bottom := k
				for bottom > 0 {
					w2, ok2 := cellWeight(cell(engine.Rank(hi), engine.Rank(bottom-1)))
					if !ok2 || w2 != w {
						break
					}
					bottom--
				}
				for rk := bottom; rk <= k; rk++ {
					mark(cell(engine.Rank(hi), engine.Rank(rk)))
				}
				hiL := letter(engine.Rank(hi))
				topL, botL := letter(engine.Rank(k)), letter(engine.Rank(bottom))
				switch {
				case k == bottom:
					emit(hiL+topL+tag, w)
				case k == hi-1:
					emit(hiL+botL+tag+"+", w)
				default:
					emit(hiL+topL+tag+"-"+hiL+botL+tag, w)
				}
				k = bottom
			}
		}
	}

	// Anything left is a partial or unevenly weighted cell: exact combos,
	// descending so higher cards read first.
	for i := NumCombos - 1; i >= 0; i-- {
		if r.W[i] > 0 && !covered[i] {
			emit(Combo(i).String(), r.W[i])
		}
	}
	return strings.Join(terms, ", ")
}
