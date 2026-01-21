package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
)

// TestStoreBasicCRUDAndReplay verifies basic operations and WAL replay.
func TestStoreBasicCRUDAndReplay(t *testing.T) {
	dataDir := t.TempDir()

	s, err := store.New(dataDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := s.Put("foo", []byte("bar")); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	got, ok := s.Get("foo")
	if !ok {
		t.Fatalf("Get() foo = missing, want present")
	}
	if string(got) != "bar" {
		t.Fatalf("Get() foo = %q, want %q", got, "bar")
	}

	keys := s.Keys()
	if len(keys) != 1 || keys[0] != "foo" {
		t.Fatalf("Keys() = %v, want [foo]", keys)
	}

	if err := s.Delete("foo"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, ok := s.Get("foo"); ok {
		t.Fatalf("Get() after Delete returned value, want missing")
	}
	if len(s.Keys()) != 0 {
		t.Fatalf("Keys() after Delete = %v, want []", s.Keys())
	}

	// Write some data and ensure it's replayed on a new Store.
	if err := s.Put("k1", []byte("v1")); err != nil {
		t.Fatalf("Put(k1) error = %v", err)
	}
	if err := s.Put("k2", []byte("v2")); err != nil {
		t.Fatalf("Put(k2) error = %v", err)
	}
	if err := s.Delete("k1"); err != nil {
		t.Fatalf("Delete(k1) error = %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	s2, err := store.New(dataDir)
	if err != nil {
		t.Fatalf("New() after replay error = %v", err)
	}
	defer s2.Close()

	if _, ok := s2.Get("k1"); ok {
		t.Fatalf("Get(k1) after replay = present, want missing")
	}
	v2, ok := s2.Get("k2")
	if !ok || string(v2) != "v2" {
		t.Fatalf("Get(k2) after replay = %q, %v, want %q, true", v2, ok, "v2")
	}
}

// TestStoreEmptyKeyErrors ensures empty keys are rejected.
func TestStoreEmptyKeyErrors(t *testing.T) {
	dataDir := t.TempDir()
	s, err := store.New(dataDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	if err := s.Put("", []byte("x")); err == nil {
		t.Fatalf("Put(\"\") = nil error, want non-nil")
	}
	if err := s.Delete(""); err == nil {
		t.Fatalf("Delete(\"\") = nil error, want non-nil")
	}
}

// TestStoreWALCorruptJSON covers the decode error path in replayWAL.
func TestStoreWALCorruptJSON(t *testing.T) {
	dataDir := t.TempDir()
	walPath := filepath.Join(dataDir, "store.wal")
	if err := os.WriteFile(walPath, []byte("not-json\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := store.New(dataDir); err == nil {
		t.Fatalf("New() with corrupt WAL error = nil, want non-nil")
	}
}

// TestStoreWALUnknownOp covers the unknown operation path in replayWAL.
func TestStoreWALUnknownOp(t *testing.T) {
	dataDir := t.TempDir()
	walPath := filepath.Join(dataDir, "store.wal")
	line := `{"op":"unknown","key":"k","value":"v"}` + "\n"
	if err := os.WriteFile(walPath, []byte(line), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := store.New(dataDir); err == nil {
		t.Fatalf("New() with unknown op in WAL error = nil, want non-nil")
	}
}

// TestStoreCloseTwice ensures Close can be called multiple times safely and
// exercises both branches of the Close implementation.
func TestStoreCloseTwice(t *testing.T) {
	dataDir := t.TempDir()
	s, err := store.New(dataDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	// Second close should be a no-op.
	if err := s.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
}

// TestStoreConcurrentAccess stresses the store with concurrent readers and
// writers to help surface races, deadlocks, or starvation under -race.
func TestStoreConcurrentAccess(t *testing.T) {
	dataDir := t.TempDir()
	s, err := store.New(dataDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer s.Close()

	const (
		numWriters    = 8
		numReaders    = 8
		numIterations = 200
	)

	var wg sync.WaitGroup

	// Writers repeatedly Put keys.
	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numIterations; i++ {
				key := fmt.Sprintf("writer-%d-%d", id, i)
				if err := s.Put(key, []byte("value")); err != nil {
					t.Errorf("Put() error in writer %d: %v", id, err)
					return
				}
			}
		}(w)
	}

	// Readers continuously Get/Keys for a short time window.
	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			deadline := time.Now().Add(500 * time.Millisecond)
			for time.Now().Before(deadline) {
				keys := s.Keys()
				for _, k := range keys {
					_, _ = s.Get(k)
				}
			}
		}(r)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success: all goroutines completed without deadlock/starvation.
	case <-time.After(5 * time.Second):
		t.Fatalf("concurrent access test timed out; possible deadlock or starvation")
	}
}
