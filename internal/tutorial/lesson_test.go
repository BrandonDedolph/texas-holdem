package tutorial

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/profile"
)

func twoSectionLesson(id string, order int, prereqs ...string) *Lesson {
	return &Lesson{
		ID:            id,
		Title:         id,
		Goal:          "test lesson",
		Order:         order,
		Prerequisites: prereqs,
		Sections: []Section{
			{Kind: SectionText, Title: "intro", Text: "words"},
			{Kind: SectionDrill, Drill: &Drill{
				Prompt:  "2+2?",
				Answer:  NumericAnswer{Value: 4},
				Explain: "arithmetic",
			}},
		},
	}
}

func TestRegistryOrderAndNext(t *testing.T) {
	r := NewRegistry()
	// Register out of order; All must sort by Order.
	r.Register(twoSectionLesson("b", 2, "a"))
	r.Register(twoSectionLesson("a", 1))
	r.Register(twoSectionLesson("c", 3, "b"))

	all := r.All()
	if len(all) != 3 {
		t.Fatalf("All() returned %d lessons, want 3", len(all))
	}
	for i, want := range []string{"a", "b", "c"} {
		if all[i].ID != want {
			t.Errorf("All()[%d] = %q, want %q", i, all[i].ID, want)
		}
	}

	p := profile.NewProfile()
	if next := r.Next(p); next == nil || next.ID != "a" {
		t.Fatalf("Next on fresh profile = %v, want lesson a", next)
	}
	p.CompleteLesson("a")
	if next := r.Next(p); next == nil || next.ID != "b" {
		t.Fatalf("Next after a = %v, want lesson b", next)
	}
	l, _ := r.Get("c")
	if l.Unlocked(p) {
		t.Error("lesson c unlocked without b completed")
	}
	p.CompleteLesson("b")
	if !l.Unlocked(p) {
		t.Error("lesson c locked with all prerequisites completed")
	}
	p.CompleteLesson("c")
	if next := r.Next(p); next != nil {
		t.Errorf("Next with everything done = %q, want nil", next.ID)
	}
}

func TestRegisterDuplicatePanics(t *testing.T) {
	r := NewRegistry()
	r.Register(twoSectionLesson("dup", 1))
	defer func() {
		if recover() == nil {
			t.Error("registering a duplicate ID did not panic")
		}
	}()
	r.Register(twoSectionLesson("dup", 2))
}

func TestProgressCompletion(t *testing.T) {
	l := twoSectionLesson("p", 1)
	pr := NewProgress(l)

	if pr.Complete() {
		t.Fatal("fresh progress reports complete")
	}
	pr.View(0)
	pr.View(1)
	if pr.Complete() {
		t.Fatal("complete with drill unanswered")
	}

	// Wrong answers don't lock the drill and don't pass it.
	if pr.Answer(1, "5") {
		t.Error("wrong answer accepted")
	}
	if pr.Passed(1) {
		t.Error("drill passed after wrong answer")
	}
	if !pr.Answer(1, "4") {
		t.Error("correct answer rejected after a wrong attempt")
	}
	if !pr.Complete() {
		t.Error("lesson not complete with all sections viewed and drill passed")
	}

	// Answering a non-drill section is a no-op.
	if pr.Answer(0, "4") {
		t.Error("answering a text section reported correct")
	}
}

// TestProgressRoundTripsThroughProfile drives a lesson to completion,
// records it, persists the profile to disk, and reloads it — the
// cross-run path the Learning Journey screen depends on.
func TestProgressRoundTripsThroughProfile(t *testing.T) {
	r := NewRegistry()
	r.Register(twoSectionLesson("first", 1))
	r.Register(twoSectionLesson("second", 2, "first"))

	store := profile.StoreAt(t.TempDir())
	p, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}

	first, _ := r.Get("first")
	pr := NewProgress(first)
	if pr.Record(p) {
		t.Fatal("Record persisted an incomplete lesson")
	}
	pr.View(0)
	pr.View(1)
	pr.Answer(1, "4")
	if !pr.Record(p) {
		t.Fatal("Record refused a complete lesson")
	}
	if err := store.Save(p); err != nil {
		t.Fatal(err)
	}

	p2, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !first.Completed(p2) {
		t.Error("completion did not survive the profile round trip")
	}
	second, _ := r.Get("second")
	if !second.Unlocked(p2) {
		t.Error("second lesson locked after its prerequisite persisted")
	}
	if next := r.Next(p2); next == nil || next.ID != "second" {
		t.Errorf("Next after round trip = %v, want second", next)
	}
}
