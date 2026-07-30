package trainer

import (
	"math/rand/v2"
	"strings"
	"testing"
)

// TestPreflopSpotDescriptionIsPossible: "folded to you" asserts that someone
// folded. UTG acts first, so nobody has — and a spot quiz that tells a
// learner otherwise corrupts the position sense it is meant to build.
func TestPreflopSpotDescriptionIsPossible(t *testing.T) {
	rng := rand.New(rand.NewPCG(9, 9))
	seenFirstToAct := false
	for i := 0; i < 400; i++ {
		item, ok := buildPreflopSpot(rng)
		if !ok {
			continue
		}
		prompt := item.Drill.Prompt
		utg := strings.Contains(prompt, "You are UTG") || strings.Contains(prompt, "UTG.")
		if utg && strings.Contains(prompt, "Folded to you") {
			t.Fatalf("item %d claims a fold happened before the first player acted:\n%s", i, prompt)
		}
		if strings.Contains(prompt, "first to act") {
			seenFirstToAct = true
		}
	}
	if !seenFirstToAct {
		t.Skip("no first-to-act spot generated in this sample")
	}
}
