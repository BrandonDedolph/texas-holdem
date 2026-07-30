// Package tutorial is the guided lesson curriculum — the repo owner's path
// from "what beats what" to "why I'm folding this" (docs/design-learning.md
// §8).
//
// Lessons are static content registered at init time (the euchre registry
// pattern): each file in content/ registers one lesson, ordered and gated by
// prerequisites. A lesson is a sequence of sections — prose, a visual, a
// checked drill, or a scripted hand that runs in the *real* engine — and is
// complete when every section has been viewed and every drill answered
// correctly. Drills retry freely: this is a curriculum, not an exam.
package tutorial

import (
	"fmt"
	"sort"

	"github.com/BrandonDedolph/texas-holdem/internal/profile"
)

// SectionKind identifies what a lesson section contains.
type SectionKind uint8

// Section kinds. Exactly the field matching the kind is set on a Section.
const (
	SectionText     SectionKind = iota // prose only
	SectionVisual                      // prose plus a rendered Visual
	SectionDrill                       // a checked question
	SectionScripted                    // a hand replayed through the engine
)

var sectionKindNames = [...]string{"text", "visual", "drill", "scripted"}

// String is the lowercase kind name.
func (k SectionKind) String() string {
	if int(k) >= len(sectionKindNames) {
		return "?"
	}
	return sectionKindNames[k]
}

// Section is one step of a lesson. Text is markdown-ish prose (card codes
// like "As" are colorized by the renderer); the pointer matching Kind is
// non-nil and the others are nil.
type Section struct {
	Kind   SectionKind
	Title  string
	Text   string
	Visual *Visual
	Drill  *Drill
	Script *ScriptedHand
}

// Lesson is one unit of the curriculum.
type Lesson struct {
	ID            string
	Title         string
	Goal          string // one line, shown on the curriculum screen
	Order         int    // 1-based position in the curriculum
	Prerequisites []string
	Sections      []Section
}

// Drills returns the lesson's drill sections' drills, in order.
func (l *Lesson) Drills() []*Drill {
	var out []*Drill
	for i := range l.Sections {
		if l.Sections[i].Drill != nil {
			out = append(out, l.Sections[i].Drill)
		}
	}
	return out
}

// Scripts returns the lesson's scripted hands, in order.
func (l *Lesson) Scripts() []*ScriptedHand {
	var out []*ScriptedHand
	for i := range l.Sections {
		if l.Sections[i].Script != nil {
			out = append(out, l.Sections[i].Script)
		}
	}
	return out
}

// Visuals returns every visual in the lesson: visual sections' own plus any
// attached to drills.
func (l *Lesson) Visuals() []*Visual {
	var out []*Visual
	for i := range l.Sections {
		if v := l.Sections[i].Visual; v != nil {
			out = append(out, v)
		}
		if d := l.Sections[i].Drill; d != nil && d.Visual != nil {
			out = append(out, d.Visual)
		}
	}
	return out
}

// Completed reports whether the profile records this lesson as done.
func (l *Lesson) Completed(p *profile.Profile) bool {
	_, ok := p.LessonsDone[l.ID]
	return ok
}

// Unlocked reports whether every prerequisite lesson is completed.
func (l *Lesson) Unlocked(p *profile.Profile) bool {
	for _, id := range l.Prerequisites {
		if _, ok := p.LessonsDone[id]; !ok {
			return false
		}
	}
	return true
}

// --- Registry ----------------------------------------------------------------

// Registry holds lessons keyed by ID. Content files register into the
// package-level default at init time; a separate Registry exists only so
// tests can build small curricula.
type Registry struct {
	lessons map[string]*Lesson
	order   []string // registration order
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{lessons: make(map[string]*Lesson)}
}

// Register adds a lesson. A duplicate ID is a programmer error — lesson
// content is compiled in, so colliding IDs can only be a typo — and panics.
func (r *Registry) Register(l *Lesson) {
	if l.ID == "" {
		panic("tutorial: Register with empty lesson ID")
	}
	if _, dup := r.lessons[l.ID]; dup {
		panic(fmt.Errorf("tutorial: lesson %q registered twice", l.ID))
	}
	r.lessons[l.ID] = l
	r.order = append(r.order, l.ID)
}

// Get retrieves a lesson by ID.
func (r *Registry) Get(id string) (*Lesson, bool) {
	l, ok := r.lessons[id]
	return l, ok
}

// All returns every lesson sorted by Order — the curriculum sequence.
func (r *Registry) All() []*Lesson {
	out := make([]*Lesson, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.lessons[id])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}

// Next returns the first lesson (by Order) not yet completed, or nil when
// the whole curriculum is done. This is where the Learning Journey screen
// resumes.
func (r *Registry) Next(p *profile.Profile) *Lesson {
	for _, l := range r.All() {
		if !l.Completed(p) {
			return l
		}
	}
	return nil
}

// DefaultRegistry is the global registry the content package populates.
var DefaultRegistry = NewRegistry()

// Register adds a lesson to the default registry.
func Register(l *Lesson) { DefaultRegistry.Register(l) }

// Get retrieves a lesson from the default registry.
func Get(id string) (*Lesson, bool) { return DefaultRegistry.Get(id) }

// All returns the default registry's lessons in curriculum order.
func All() []*Lesson { return DefaultRegistry.All() }

// Next returns the first incomplete lesson from the default registry.
func Next(p *profile.Profile) *Lesson { return DefaultRegistry.Next(p) }

// --- Progress ----------------------------------------------------------------

// Progress tracks one sitting of one lesson: which sections have been viewed
// and which drills have been passed. Completion = every section viewed and
// every drill answered correctly, on whatever attempt (design-learning.md
// §8.5). Progress is in-memory; Record persists the completed flag to the
// profile, which is the only cross-run state.
type Progress struct {
	Lesson *Lesson
	viewed []bool
	passed []bool // indexed like Sections; meaningful only for drill sections
}

// NewProgress starts tracking a lesson.
func NewProgress(l *Lesson) *Progress {
	return &Progress{
		Lesson: l,
		viewed: make([]bool, len(l.Sections)),
		passed: make([]bool, len(l.Sections)),
	}
}

// View marks a section as viewed. Out-of-range indices are ignored.
func (pr *Progress) View(i int) {
	if i >= 0 && i < len(pr.viewed) {
		pr.viewed[i] = true
	}
}

// Viewed reports whether section i has been viewed.
func (pr *Progress) Viewed(i int) bool {
	return i >= 0 && i < len(pr.viewed) && pr.viewed[i]
}

// Answer submits a drill answer for section i and reports whether it was
// correct. A correct answer marks the drill passed; a wrong one changes
// nothing — the drill simply remains open for another try. Answering a
// non-drill section reports false.
func (pr *Progress) Answer(i int, input string) bool {
	if i < 0 || i >= len(pr.Lesson.Sections) {
		return false
	}
	d := pr.Lesson.Sections[i].Drill
	if d == nil {
		return false
	}
	if d.Answer.Check(input) {
		pr.passed[i] = true
		return true
	}
	return false
}

// Passed reports whether section i's drill has been answered correctly.
func (pr *Progress) Passed(i int) bool {
	return i >= 0 && i < len(pr.passed) && pr.passed[i]
}

// Complete reports whether the lesson is finished: all sections viewed and
// every drill passed.
func (pr *Progress) Complete() bool {
	for i := range pr.Lesson.Sections {
		if !pr.viewed[i] {
			return false
		}
		if pr.Lesson.Sections[i].Drill != nil && !pr.passed[i] {
			return false
		}
	}
	return true
}

// Record persists completion into the profile and reports whether it did.
// An incomplete lesson records nothing.
func (pr *Progress) Record(p *profile.Profile) bool {
	if !pr.Complete() {
		return false
	}
	p.CompleteLesson(pr.Lesson.ID)
	return true
}
