package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/BrandonDedolph/texas-holdem/internal/engine"
)

// guardUserDirs points every environment fallback at throwaway directories
// and returns a check that fails the test if anything was written to them.
// This is the "tests never touch the real user directory" guarantee, made
// executable rather than taken on faith.
func guardUserDirs(t *testing.T) (verify func()) {
	t.Helper()
	canary := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(canary, "xdg"))
	t.Setenv("HOME", filepath.Join(canary, "home"))
	t.Setenv("AppData", filepath.Join(canary, "appdata"))
	return func() {
		t.Helper()
		entries, err := os.ReadDir(canary)
		if err != nil {
			t.Fatalf("reading canary dir: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("test wrote into user-directory fallbacks: %v", entries)
		}
	}
}

// populated builds a profile with every field set, so the round trip
// exercises the whole schema.
func populated() *Profile {
	p := NewProfile()
	p.CreatedAt = time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	p.Bankroll = engine.Chips(12_345)
	p.LessonsDone["pot-odds"] = time.Date(2026, 7, 2, 9, 30, 0, 0, time.UTC)
	p.MomentsSeen["first_flush_draw"] = time.Date(2026, 7, 3, 20, 15, 0, 0, time.UTC)
	p.DrillStats["outs.flushdraw"] = SkillStat{EMA: 0.85, Attempts: 24, Level: 1}
	p.GradeTotals["good"] = 41
	p.GradeTotals["blunder"] = 3
	p.SessionLog = []SessionSummary{{
		Start:    time.Date(2026, 7, 4, 19, 0, 0, 0, time.UTC),
		Hands:    62,
		NetBB:    -14.5,
		Accuracy: 0.87,
		EVLossBB: 6.25,
	}}
	p.CoachMode = CoachMistakes
	p.TableDefaults = TableConfig{
		Lineup:     []string{"station", "maniac"},
		SmallBlind: 10,
		BigBlind:   25,
		Stack:      2_500,
	}
	return p
}

// sameProfile compares two profiles by their canonical JSON form, which
// sidesteps time.Time's location-pointer inequality after a round trip.
func sameProfile(t *testing.T, a, b *Profile) bool {
	t.Helper()
	ja, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	jb, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(ja) == string(jb)
}

func TestSaveLoadRoundTrip(t *testing.T) {
	verify := guardUserDirs(t)
	st := StoreAt(t.TempDir())

	want := populated()
	if err := st.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := st.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !sameProfile(t, got, want) {
		t.Fatalf("round trip changed the profile:\ngot  %+v\nwant %+v", got, want)
	}
	verify()
}

func TestSavedFileIsHumanReadable(t *testing.T) {
	st := StoreAt(t.TempDir())
	if err := st.Save(populated()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(st.Dir(), profileFile))
	if err != nil {
		t.Fatalf("reading saved profile: %v", err)
	}
	// Indented JSON, not a single machine line.
	if !strings.Contains(string(data), "\n  \"") {
		t.Fatalf("profile.json is not indented:\n%s", data)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatal("profile.json does not end with a newline")
	}
}

func TestLoadMissingFileIsFreshDefaultNotError(t *testing.T) {
	verify := guardUserDirs(t)
	st := StoreAt(t.TempDir())

	p, err := st.Load()
	if err != nil {
		t.Fatalf("first-run Load must never error, got: %v", err)
	}
	if p == nil {
		t.Fatal("Load returned nil profile")
	}
	if p.Version != CurrentVersion || p.CoachMode != CoachFull {
		t.Fatalf("fresh profile has wrong defaults: %+v", p)
	}
	// Usable immediately: maps are allocated.
	p.RecordGrade("good")
	p.RecordDrill("outs.flushdraw", true)
	verify()
}

func TestLoadCorruptFileQuarantinesAndReturnsUsableProfile(t *testing.T) {
	st := StoreAt(t.TempDir())
	path := filepath.Join(st.Dir(), profileFile)
	garbage := []byte(`{"version": 1, "bankroll": THIS IS NOT JSON`)
	if err := os.WriteFile(path, garbage, 0o644); err != nil {
		t.Fatalf("writing corrupt file: %v", err)
	}

	p, err := st.Load()
	if err == nil {
		t.Fatal("corrupt profile must be reported")
	}
	if p == nil || p.Version != CurrentVersion {
		t.Fatalf("corrupt file must still yield a usable default profile, got %+v", p)
	}
	p.RecordGrade("good") // usable in practice, not just non-nil

	// Nothing silently destroyed: the bad bytes are preserved verbatim.
	bad, readErr := os.ReadFile(path + badSuffix)
	if readErr != nil {
		t.Fatalf("expected %s sidecar: %v", profileFile+badSuffix, readErr)
	}
	if string(bad) != string(garbage) {
		t.Fatalf("quarantined bytes differ from the original corrupt file")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("corrupt profile.json should have been moved aside, stat err = %v", statErr)
	}

	// The next Save starts clean rather than failing forever.
	if err := p.Save(); err != nil {
		t.Fatalf("Save after quarantine: %v", err)
	}
}

func TestLoadUnknownVersionQuarantines(t *testing.T) {
	st := StoreAt(t.TempDir())
	path := filepath.Join(st.Dir(), profileFile)
	if err := os.WriteFile(path, []byte(`{"version": 99}`), 0o644); err != nil {
		t.Fatalf("writing future-version file: %v", err)
	}
	p, err := st.Load()
	if err == nil {
		t.Fatal("unknown version must be reported")
	}
	if p == nil || p.Version != CurrentVersion {
		t.Fatalf("unknown version must still yield a usable default, got %+v", p)
	}
	if _, statErr := os.Stat(path + badSuffix); statErr != nil {
		t.Fatalf("expected quarantine sidecar: %v", statErr)
	}
}

func TestSaveIsAtomicAndLeavesNoTempFiles(t *testing.T) {
	st := StoreAt(t.TempDir())

	// An existing profile must survive a subsequent save intact — the
	// rename either fully replaces it or (on a crash) leaves it untouched.
	first := populated()
	if err := st.Save(first); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	second := populated()
	second.Bankroll = 999
	second.CoachMode = CoachOff
	if err := st.Save(second); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	got, err := st.Load()
	if err != nil {
		t.Fatalf("Load after overwrite: %v", err)
	}
	if !sameProfile(t, got, second) {
		t.Fatal("overwritten profile is not exactly the second save")
	}

	// The commit-by-rename must clean up after itself: the directory holds
	// profile.json and nothing else — in particular no *.tmp-* remnants.
	entries, err := os.ReadDir(st.Dir())
	if err != nil {
		t.Fatalf("reading store dir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != profileFile {
			t.Errorf("unexpected file left behind by Save: %s", e.Name())
		}
	}
}

func TestProfileSaveWritesToLoadingStore(t *testing.T) {
	st := StoreAt(t.TempDir())
	p, err := st.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p.Bankroll = 777
	if err := p.Save(); err != nil {
		t.Fatalf("Profile.Save: %v", err)
	}
	got, err := st.Load()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Bankroll != 777 {
		t.Fatalf("Profile.Save did not write back to its store: bankroll %d", got.Bankroll)
	}
}

func TestDefaultDataDirResolution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("XDG resolution is not the Windows path")
	}
	home := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("HOME", home)

	t.Run("XDG_DATA_HOME set", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", xdg)
		st, err := NewStore()
		if err != nil {
			t.Fatalf("NewStore: %v", err)
		}
		if want := filepath.Join(xdg, "holdem"); st.Dir() != want {
			t.Fatalf("Dir() = %q, want %q", st.Dir(), want)
		}
	})

	t.Run("XDG_DATA_HOME unset falls back to ~/.local/share", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "")
		st, err := NewStore()
		if err != nil {
			t.Fatalf("NewStore: %v", err)
		}
		if want := filepath.Join(home, ".local", "share", "holdem"); st.Dir() != want {
			t.Fatalf("Dir() = %q, want %q", st.Dir(), want)
		}
	})

	t.Run("relative XDG_DATA_HOME is ignored per spec", func(t *testing.T) {
		t.Setenv("XDG_DATA_HOME", "relative/path")
		st, err := NewStore()
		if err != nil {
			t.Fatalf("NewStore: %v", err)
		}
		if want := filepath.Join(home, ".local", "share", "holdem"); st.Dir() != want {
			t.Fatalf("Dir() = %q, want %q", st.Dir(), want)
		}
	})

	// Resolution alone must not create anything on disk.
	if entries, _ := os.ReadDir(xdg); len(entries) != 0 {
		t.Fatalf("NewStore created files under XDG_DATA_HOME: %v", entries)
	}
}

// testHand stands in for the future {engine.HandRecord, coach.HandAnnotations}
// pair — the store is generic over the record type on purpose.
type testHand struct {
	ID string `json:"id"`
}

func TestHandHistoryAppendAndLastNAcrossDays(t *testing.T) {
	verify := guardUserDirs(t)
	st := StoreAt(t.TempDir())

	day1 := time.Date(2026, 7, 28, 22, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 7, 29, 1, 0, 0, 0, time.UTC) // the session crossed midnight
	for _, h := range []string{"h1", "h2", "h3"} {
		if err := st.AppendHandRecord(day1, testHand{ID: h}); err != nil {
			t.Fatalf("append %s: %v", h, err)
		}
	}
	for _, h := range []string{"h4", "h5"} {
		if err := st.AppendHandRecord(day2, testHand{ID: h}); err != nil {
			t.Fatalf("append %s: %v", h, err)
		}
	}

	// Two files, named by day.
	for _, name := range []string{"2026-07-28.jsonl", "2026-07-29.jsonl"} {
		if _, err := os.Stat(filepath.Join(st.Dir(), handsSubdir, name)); err != nil {
			t.Fatalf("expected hand file %s: %v", name, err)
		}
	}

	ids := func(raws []json.RawMessage) []string {
		out := make([]string, len(raws))
		for i, r := range raws {
			var h testHand
			if err := json.Unmarshal(r, &h); err != nil {
				t.Fatalf("record %d is not valid JSON: %v", i, err)
			}
			out[i] = h.ID
		}
		return out
	}

	// Last 4 spans the day boundary, newest first.
	recs, err := st.LastHandRecords(4)
	if err != nil {
		t.Fatalf("LastHandRecords(4): %v", err)
	}
	if got, want := strings.Join(ids(recs), ","), "h5,h4,h3,h2"; got != want {
		t.Fatalf("LastHandRecords(4) = %s, want %s", got, want)
	}

	// Asking for more than exist returns everything.
	recs, err = st.LastHandRecords(50)
	if err != nil {
		t.Fatalf("LastHandRecords(50): %v", err)
	}
	if got, want := strings.Join(ids(recs), ","), "h5,h4,h3,h2,h1"; got != want {
		t.Fatalf("LastHandRecords(50) = %s, want %s", got, want)
	}
	verify()
}

func TestLastHandRecordsEmptyAndZero(t *testing.T) {
	st := StoreAt(t.TempDir())
	recs, err := st.LastHandRecords(50)
	if err != nil || recs != nil {
		t.Fatalf("no hands dir: got (%v, %v), want (nil, nil)", recs, err)
	}
	if recs, _ := st.LastHandRecords(0); recs != nil {
		t.Fatalf("LastHandRecords(0) = %v, want nil", recs)
	}
}

func TestLastHandRecordsSkipsTornFinalLine(t *testing.T) {
	st := StoreAt(t.TempDir())
	day := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	if err := st.AppendHandRecord(day, testHand{ID: "intact"}); err != nil {
		t.Fatalf("append: %v", err)
	}

	// Simulate a crash mid-append: a truncated record with no newline.
	path := filepath.Join(st.Dir(), handsSubdir, "2026-07-29.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := f.WriteString(`{"id":"torn`); err != nil {
		t.Fatalf("write torn line: %v", err)
	}
	f.Close()

	recs, err := st.LastHandRecords(10)
	if err != nil {
		t.Fatalf("LastHandRecords: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("got %d records, want the 1 intact one", len(recs))
	}
	var h testHand
	if err := json.Unmarshal(recs[0], &h); err != nil || h.ID != "intact" {
		t.Fatalf("surviving record = %s (err %v), want the intact hand", recs[0], err)
	}
}
