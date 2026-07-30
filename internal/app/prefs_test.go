package app

import (
	"testing"

	"github.com/BrandonDedolph/texas-holdem/internal/profile"
	"github.com/BrandonDedolph/texas-holdem/internal/ui/theme"
)

func TestPrefsRoundTripThroughProfile(t *testing.T) {
	want := &Prefs{
		Speed:      SpeedFast,
		Deck:       theme.TwoColor,
		ASCII:      true,
		Background: BackgroundLight,
	}
	p := profile.NewProfile()
	p.Display = want.Display()

	got := PrefsFrom(p)
	if got.Speed != want.Speed || got.Deck != want.Deck ||
		got.ASCII != want.ASCII || got.Background != want.Background {
		t.Fatalf("round trip lost prefs: got %+v, want %+v", got, want)
	}
}

func TestPrefsFromDefaultsOnGarbage(t *testing.T) {
	// A hand-edited profile with nonsense values must not stop the game from
	// starting; every unknown value falls back to its default.
	p := profile.NewProfile()
	p.Display = profile.Display{Speed: "warp", Deck: "puce", Background: "mauve"}

	got := PrefsFrom(p)
	if got.Speed != SpeedLearn {
		t.Errorf("Speed = %v, want SpeedLearn", got.Speed)
	}
	if got.Deck != theme.FourColor {
		t.Errorf("Deck = %v, want FourColor", got.Deck)
	}
	if got.Background != BackgroundAuto {
		t.Errorf("Background = %v, want BackgroundAuto", got.Background)
	}
}

func TestPrefsFromNilProfile(t *testing.T) {
	if got := PrefsFrom(nil); got == nil {
		t.Fatal("PrefsFrom(nil) returned nil, want defaults")
	}
}

func TestApplyToMirrorsOntoProfile(t *testing.T) {
	p := profile.NewProfile()
	prefs := &Prefs{Speed: SpeedInstant, Deck: theme.TwoColor, Background: BackgroundDark}
	prefs.ApplyTo(p)

	if p.Display.Speed != "instant" || p.Display.Deck != "two-color" || p.Display.Background != "dark" {
		t.Fatalf("ApplyTo did not mirror onto the profile: %+v", p.Display)
	}
	// Restore the globals ApplyTo touched so later tests see a clean theme.
	t.Cleanup(func() { DefaultPrefs().ApplyTo(nil) })
}
