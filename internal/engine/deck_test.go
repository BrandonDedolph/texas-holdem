package engine

import "testing"

func drawAll(t *testing.T, src CardSource) []Card {
	t.Helper()
	out := make([]Card, NumCards)
	for i := range out {
		out[i] = src.Draw()
	}
	return out
}

func assertFullDeck(t *testing.T, cards []Card) {
	t.Helper()
	var seen CardSet
	for _, c := range cards {
		if !c.Valid() {
			t.Fatalf("dealt invalid card %d", c)
		}
		if seen.Has(c) {
			t.Fatalf("dealt %s twice", c.Code())
		}
		seen = seen.Add(c)
	}
	if seen.Count() != NumCards {
		t.Fatalf("dealt %d distinct cards, want %d", seen.Count(), NumCards)
	}
}

func TestDeckDealsFullDeck(t *testing.T) {
	assertFullDeck(t, drawAll(t, NewDeck(1)))
}

func TestDeckIsDeterministic(t *testing.T) {
	a := drawAll(t, NewDeck(42))
	b := drawAll(t, NewDeck(42))
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("card %d differs between decks with the same seed: %s vs %s",
				i, a[i].Code(), b[i].Code())
		}
	}
	// Different seeds must (with overwhelming probability) shuffle differently.
	c := drawAll(t, NewDeck(43))
	same := true
	for i := range a {
		if a[i] != c[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("seeds 42 and 43 produced identical shuffles")
	}
}

func TestDeckSeedIsRecorded(t *testing.T) {
	if got := NewDeck(99).Seed(); got != 99 {
		t.Fatalf("Seed() = %d, want 99", got)
	}
}

func TestDeckExhaustionPanics(t *testing.T) {
	d := NewDeck(1)
	drawAll(t, d)
	defer func() {
		if recover() == nil {
			t.Fatal("53rd Draw did not panic")
		}
	}()
	d.Draw()
}

func TestScriptedDeckDealsPrefixFirst(t *testing.T) {
	prefix := MustCards("As Kd 7h 2c")
	d := NewScriptedDeck(prefix, 7)
	all := drawAll(t, d)
	for i, want := range prefix {
		if all[i] != want {
			t.Fatalf("card %d = %s, want scripted %s", i, all[i].Code(), want.Code())
		}
	}
	assertFullDeck(t, all)
}

func TestScriptedDeckFallbackIsDeterministic(t *testing.T) {
	prefix := MustCards("As Kd")
	a := drawAll(t, NewScriptedDeck(prefix, 5))
	b := drawAll(t, NewScriptedDeck(prefix, 5))
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("card %d differs between scripted decks with the same fallback seed", i)
		}
	}
}

func TestScriptedDeckDuplicatePrefixPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate scripted card did not panic")
		}
	}()
	NewScriptedDeck(MustCards("As As"), 1)
}

func TestCloneSourceIsIndependent(t *testing.T) {
	d := NewDeck(3)
	d.Draw()
	cp := d.CloneSource()
	// Both must now deal the identical remaining sequence, independently.
	for i := 0; i < NumCards-1; i++ {
		a, b := d.Draw(), cp.Draw()
		if a != b {
			t.Fatalf("clone diverged at card %d: %s vs %s", i, a.Code(), b.Code())
		}
	}

	s := NewScriptedDeck(MustCards("As Kd"), 9)
	s.Draw()
	sc := s.CloneSource()
	if a, b := s.Draw(), sc.Draw(); a != b || a != MustCard("Kd") {
		t.Fatalf("scripted clone diverged: %s vs %s", a.Code(), b.Code())
	}
}
