package store

import (
	"fmt"
	"strconv"
	"sync"
	"testing"

	"config-center/config"
	"config-center/model"
)

// TestURLStore_ConcurrentSnapshotStability reproduces the original bug:
// under heavy concurrent writes/deletes/reads, a snapshot must never observe
// a partially built map, and every snapshot must reflect the same set of live
// codes (no "occasionally returns zero entries"). Run with -race.
func TestURLStore_ConcurrentSnapshotStability(t *testing.T) {
	const writers = 50
	const iterations = 100

	st, err := NewURLStore(config.Default())
	if err != nil {
		t.Fatalf("NewURLStore: %v", err)
	}

	// Seed one entry so the snapshot path has a baseline.
	if err := st.Save(&model.ShortURL{Code: "seed", RawURL: "http://seed"}, false); err != nil {
		t.Fatalf("seed Save: %v", err)
	}

	var wg sync.WaitGroup
	snapErrs := make(chan error, writers)

	// Writers: each owns a unique code prefix and creates/deletes it.
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			prefix := "w" + strconv.Itoa(id) + "-"
			for i := 0; i < iterations; i++ {
				code := prefix + strconv.Itoa(i)
				u := &model.ShortURL{Code: code, RawURL: "http://" + code}
				if err := st.Save(u, true); err != nil {
					snapErrs <- fmt.Errorf("Save %s: %w", code, err)
					return
				}

				// Also bump visits and read concurrently.
				_ = st.IncrementVisits(code)
				if _, err := st.Get(code); err != nil {
					// A Get miss for a code we just wrote is a real bug.
					snapErrs <- fmt.Errorf("Get %s after Save: %w", code, err)
					return
				}
			}
		}(w)
	}

	// Deleter: re-creates entries via overwrite so they keep churning.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations*writers; i++ {
			code := "w0-" + strconv.Itoa(i%iterations)
			_ = st.Delete(code)
			// Re-add it so the set keeps churning but stays non-empty.
			_ = st.Save(&model.ShortURL{Code: code, RawURL: "http://" + code}, true)
		}
	}()

	// Snapshotter: continuously take snapshots and check invariants.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations*5; i++ {
			snap := st.RawSnapshot()
			// Invariant 1: a snapshot is never nil and never the zero map
			// while the store is seeded and being written to.
			if len(snap) == 0 {
				snapErrs <- fmt.Errorf("snapshot %d returned zero entries", i)
				return
			}
			// Invariant 2: every returned value is internally consistent.
			for code, u := range snap {
				if code != u.Code {
					snapErrs <- fmt.Errorf("snapshot key/value mismatch: key=%s code=%s", code, u.Code)
					return
				}
				if u.RawURL == "" {
					snapErrs <- fmt.Errorf("snapshot entry %s has empty RawURL", code)
					return
				}
			}
			// Invariant 3: "seed" must always be present — it is never deleted.
			if _, ok := snap["seed"]; !ok {
				snapErrs <- fmt.Errorf("snapshot %d lost seed entry", i)
				return
			}
		}
	}()

	wg.Wait()
	close(snapErrs)

	for e := range snapErrs {
		t.Error(e)
	}
}

// TestURLStore_SnapshotIsDefensiveCopy ensures the returned map is a copy:
// mutating it must not affect the store, and concurrent mutations to the
// store must not change a previously returned snapshot.
func TestURLStore_SnapshotIsDefensiveCopy(t *testing.T) {
	st, err := NewURLStore(config.Default())
	if err != nil {
		t.Fatalf("NewURLStore: %v", err)
	}
	if err := st.Save(&model.ShortURL{Code: "abc", RawURL: "http://abc"}, false); err != nil {
		t.Fatalf("Save: %v", err)
	}

	snap := st.RawSnapshot()
	if len(snap) != 1 || snap["abc"].RawURL != "http://abc" {
		t.Fatalf("snapshot = %+v, want single abc entry", snap)
	}

	// Mutate the returned map and the struct inside it.
	snap["abc"] = model.ShortURL{Code: "abc", RawURL: "tampered"}
	snap["injected"] = model.ShortURL{Code: "injected", RawURL: "http://x"}

	// Mutate the store underneath.
	if err := st.Save(&model.ShortURL{Code: "def", RawURL: "http://def"}, false); err != nil {
		t.Fatalf("Save def: %v", err)
	}

	// Re-snapshot; the tamper must not have leaked in, and the new write must be visible.
	snap2 := st.RawSnapshot()
	if _, ok := snap2["injected"]; ok {
		t.Fatal("injected entry leaked into store from mutated snapshot map")
	}
	if snap2["abc"].RawURL == "tampered" {
		t.Fatal("tampered value leaked into store from mutated snapshot struct")
	}
	if _, ok := snap2["def"]; !ok {
		t.Fatal("second Save not visible in subsequent snapshot")
	}

	// The original snapshot must be stable despite the store mutation.
	if _, ok := snap["def"]; ok {
		t.Fatal("original snapshot saw a write that happened after it was taken")
	}
}

// TestURLStore_SaveOverwrite preserves overwrite=false exclusivity under
// concurrent identical inserts (only one should win).
func TestURLStore_ConcurrentCreateExclusivity(t *testing.T) {
	st, err := NewURLStore(config.Default())
	if err != nil {
		t.Fatalf("NewURLStore: %v", err)
	}

	const goroutines = 50
	var wg sync.WaitGroup
	success := make(chan bool, goroutines)
	fail := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := st.Save(&model.ShortURL{Code: "dupe", RawURL: "http://dupe"}, false)
			if err == nil {
				success <- true
			} else {
				fail <- err
			}
		}()
	}
	wg.Wait()
	close(success)
	close(fail)

	wins := 0
	for range success {
		wins++
	}
	if wins != 1 {
		t.Fatalf("expected exactly 1 successful insert, got %d", wins)
	}
	for e := range fail {
		if e == nil {
			t.Fatal("unexpected nil error")
		}
	}
}
