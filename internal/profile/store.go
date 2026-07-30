package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// On-disk layout under the data directory:
//
//	profile.json           the Profile — small, indented, human-readable
//	profile.json.bad       quarantined copy of a corrupt profile (never deleted)
//	hands/2026-07-29.jsonl hand records, one compact JSON object per line
const (
	appDirName  = "holdem"
	profileFile = "profile.json"
	badSuffix   = ".bad"
	handsSubdir = "hands"
	jsonlExt    = ".jsonl"
	handDateFmt = "2006-01-02"
)

// Store reads and writes the profile and hand histories under one base
// directory. The directory is a plain field so tests construct a Store over
// t.TempDir() — there is no package-level path state to leak between tests
// or into the real user directory.
type Store struct {
	dir string
}

// NewStore resolves the user's data directory. It does not create anything
// on disk; directories appear on first write.
func NewStore() (*Store, error) {
	dir, err := defaultDataDir()
	if err != nil {
		return nil, err
	}
	return StoreAt(dir), nil
}

// StoreAt returns a Store rooted at an explicit directory. Tests use this
// with t.TempDir(); the app itself should use NewStore.
func StoreAt(dir string) *Store {
	return &Store{dir: dir}
}

// Dir is the store's base directory.
func (s *Store) Dir() string { return s.dir }

// defaultDataDir resolves where the profile lives.
//
// This is deliberately not os.UserConfigDir: progress and hand histories are
// *data*, not configuration, so the XDG base is $XDG_DATA_HOME (fallback
// ~/.local/share). Go's standard library has no UserDataDir, hence the
// manual resolution. Windows keeps its one conventional location, %AppData%.
func defaultDataDir() (string, error) {
	if runtime.GOOS == "windows" {
		appData := os.Getenv("AppData")
		if appData == "" {
			return "", errors.New("profile: %AppData% is not set")
		}
		return filepath.Join(appData, appDirName), nil
	}
	// Per the XDG spec, a relative $XDG_DATA_HOME is invalid and ignored.
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" && filepath.IsAbs(xdg) {
		return filepath.Join(xdg, appDirName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("profile: cannot resolve home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", appDirName), nil
}

// Load reads the profile. The returned *Profile is always usable, even when
// err != nil:
//
//   - Missing file: a first run, not a failure — fresh default, nil error.
//   - Unreadable or corrupt file: fresh default plus an error the caller
//     should surface. A corrupt file is first preserved as profile.json.bad
//     so a bug in this code can never silently destroy real progress.
//   - Unknown schema version: treated like corruption — quarantined, not
//     "migrated" by guesswork. Real migrations switch on Version here.
func (s *Store) Load() (*Profile, error) {
	path := filepath.Join(s.dir, profileFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s.fresh(), nil
	}
	if err != nil {
		return s.fresh(), fmt.Errorf("profile: reading %s: %w", path, err)
	}

	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		s.quarantine(path)
		return s.fresh(), fmt.Errorf(
			"profile: %s is corrupt (preserved as %s): %w", path, profileFile+badSuffix, err)
	}

	switch p.Version {
	case CurrentVersion:
		// Current schema; nothing to migrate.
	default:
		s.quarantine(path)
		return s.fresh(), fmt.Errorf(
			"profile: %s has unsupported version %d (this build reads version %d; preserved as %s)",
			path, p.Version, CurrentVersion, profileFile+badSuffix)
	}

	p.ensureMaps()
	p.store = s
	return &p, nil
}

// fresh builds a default profile bound to this store.
func (s *Store) fresh() *Profile {
	p := NewProfile()
	p.store = s
	return p
}

// quarantine sets a bad profile aside as profile.json.bad. Best effort: if
// even the rename fails there is nothing safer left to do, and Load's error
// still points the user at the original file.
func (s *Store) quarantine(path string) {
	bad := path + badSuffix
	_ = os.Remove(bad) // os.Rename cannot replace an existing file on Windows
	_ = os.Rename(path, bad)
}

// Save writes the profile atomically: marshal to a temp file in the same
// directory, fsync, then rename over profile.json. The rename is the commit
// point — a crash at any earlier moment leaves the previous profile intact,
// which is why Save never opens profile.json itself for writing.
func (s *Store) Save(p *Profile) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("profile: creating %s: %w", s.dir, err)
	}

	// Indented on purpose: this is a learning tool, and the player owning
	// their own progress includes being able to read the file that holds it.
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("profile: encoding profile: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(s.dir, profileFile+".tmp-*")
	if err != nil {
		return fmt.Errorf("profile: creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func(err error) error {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return cleanup(fmt.Errorf("profile: writing %s: %w", tmpName, err))
	}
	if err := tmp.Sync(); err != nil {
		return cleanup(fmt.Errorf("profile: syncing %s: %w", tmpName, err))
	}
	if err := tmp.Close(); err != nil {
		return cleanup(fmt.Errorf("profile: closing %s: %w", tmpName, err))
	}
	path := filepath.Join(s.dir, profileFile)
	if err := os.Rename(tmpName, path); err != nil {
		return cleanup(fmt.Errorf("profile: committing %s: %w", path, err))
	}
	syncDir(s.dir) // persist the rename itself; best effort
	return nil
}

// syncDir fsyncs a directory so a just-committed rename survives power loss.
// Failure is ignored: not every platform or filesystem supports it (Windows
// notably), and the rename has already succeeded.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}

// Load reads the profile from the default user data directory. Even on
// error the returned profile is usable — a first-run or broken-disk user
// still gets a working app; the error is for a warning banner, not a fatal.
func Load() (*Profile, error) {
	st, err := NewStore()
	if err != nil {
		return NewProfile(), err
	}
	return st.Load()
}

// Save writes the profile back to the store that loaded it, or to the
// default user data directory for a profile built with NewProfile. Cheap
// enough to call on every state change.
func (p *Profile) Save() error {
	if p.store == nil {
		st, err := NewStore()
		if err != nil {
			return err
		}
		p.store = st
	}
	return p.store.Save(p)
}

// --- Hand histories -------------------------------------------------------
//
// Hand records are JSONL, one file per day, append-only: sessions crash, and
// an append-only log never loses a finished hand. One file per day keeps
// every file small enough to scan without an index (design-learning.md §6).

// handFile is the JSONL path for a given day, named YYYY-MM-DD.jsonl so
// lexicographic filename order is chronological order.
func (s *Store) handFile(day time.Time) string {
	return filepath.Join(s.dir, handsSubdir, day.Format(handDateFmt)+jsonlExt)
}

// AppendHandRecord appends one record to the given day's JSONL file. The
// record is marshaled compactly onto a single line — the one-line-per-record
// invariant is the whole format, so the profile's indentation rule does not
// apply here. The write is fsynced: a finished hand should survive whatever
// the session does next.
//
// TODO(wire-types): rec becomes the {engine.HandRecord, coach.HandAnnotations}
// pair once those packages land; the store stays generic until then.
func (s *Store) AppendHandRecord(day time.Time, rec any) error {
	line, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("profile: encoding hand record: %w", err)
	}
	dir := filepath.Join(s.dir, handsSubdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("profile: creating %s: %w", dir, err)
	}
	path := s.handFile(day)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("profile: opening %s: %w", path, err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return fmt.Errorf("profile: appending to %s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("profile: syncing %s: %w", path, err)
	}
	return f.Close()
}

// LastHandRecords returns up to n of the most recent hand records, newest
// first, as raw JSON for the caller to unmarshal into its own types.
//
// It scans day files newest-first and each file's lines bottom-up, stopping
// as soon as n records are in hand — the review screen's "last 50 hands"
// touches at most a day or two of files, so no index is needed at this
// scale. A torn final line (crash mid-append) is skipped, not an error:
// append-only means every *finished* hand is intact.
func (s *Store) LastHandRecords(n int) ([]json.RawMessage, error) {
	if n <= 0 {
		return nil, nil
	}
	dir := filepath.Join(s.dir, handsSubdir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil // no hands played yet
	}
	if err != nil {
		return nil, fmt.Errorf("profile: reading %s: %w", dir, err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), jsonlExt) {
			names = append(names, e.Name())
		}
	}
	// Newest day first: the date-stamped names sort chronologically.
	sort.Sort(sort.Reverse(sort.StringSlice(names)))

	out := make([]json.RawMessage, 0, n)
	for _, name := range names {
		if len(out) >= n {
			break
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("profile: reading %s: %w", name, err)
		}
		lines := strings.Split(string(data), "\n")
		for i := len(lines) - 1; i >= 0 && len(out) < n; i-- {
			line := strings.TrimSpace(lines[i])
			if line == "" || !json.Valid([]byte(line)) {
				continue
			}
			out = append(out, json.RawMessage(line))
		}
	}
	return out, nil
}
