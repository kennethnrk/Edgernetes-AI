package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type opType string

const (
	opPut    opType = "put"
	opDelete opType = "delete"
)

type walRecord struct {
	Op    opType `json:"op"`
	Key   string `json:"key"`
	Value []byte `json:"value,omitempty"`
}

// Store is a simple, single-node, disk-backed key–value store.
type Store struct {
	mu   sync.RWMutex
	data map[string][]byte

	walPath string
	walFile *os.File
}

// New creates a new Store and replays any existing WAL.
func New(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	walPath := filepath.Join(dataDir, "store.wal")

	// Open WAL for read/write, create if not exists.
	f, err := os.OpenFile(walPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open wal: %w", err)
	}

	s := &Store{
		data:    make(map[string][]byte),
		walPath: walPath,
		walFile: f,
	}

	if err := s.replayWAL(); err != nil {
		_ = f.Close()
		return nil, err
	}

	// Reopen WAL in append mode after replay.
	if err := s.reopenWALAppend(); err != nil {
		_ = f.Close()
		return nil, err
	}

	return s, nil
}

// replayWAL reads all records from the WAL and rebuilds in-memory state.
func (s *Store) replayWAL() error {
	if _, err := s.walFile.Seek(0, 0); err != nil {
		return fmt.Errorf("seek wal: %w", err)
	}

	scanner := bufio.NewScanner(s.walFile)
	for scanner.Scan() {
		var rec walRecord
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			return fmt.Errorf("decode wal record: %w", err)
		}

		switch rec.Op {
		case opPut:
			s.data[rec.Key] = append([]byte(nil), rec.Value...)
		case opDelete:
			delete(s.data, rec.Key)
		default:
			return fmt.Errorf("unknown wal op: %s", rec.Op)
		}
	}
	return scanner.Err()
}

func (s *Store) reopenWALAppend() error {
	if err := s.walFile.Close(); err != nil {
		return fmt.Errorf("close wal: %w", err)
	}
	f, err := os.OpenFile(s.walPath, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("reopen wal append: %w", err)
	}
	s.walFile = f
	return nil
}

// Put sets a key to a value (and persists it).
func (s *Store) Put(key string, value []byte) error {
	if key == "" {
		return errors.New("empty key")
	}

	// Serialize WAL writes and in-memory updates with the same mutex to avoid races.
	s.mu.Lock()
	defer s.mu.Unlock()

	rec := walRecord{
		Op:    opPut,
		Key:   key,
		Value: value,
	}

	if err := s.appendRecord(rec); err != nil {
		return err
	}

	s.data[key] = append([]byte(nil), value...)
	return nil
}

// Get returns the value for a key.
func (s *Store) Get(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), v...), true
}

// Delete removes a key (and persists it).
func (s *Store) Delete(key string) error {
	if key == "" {
		return errors.New("empty key")
	}

	// Serialize WAL writes and in-memory updates with the same mutex to avoid races.
	s.mu.Lock()
	defer s.mu.Unlock()

	rec := walRecord{
		Op:  opDelete,
		Key: key,
	}

	if err := s.appendRecord(rec); err != nil {
		return err
	}

	delete(s.data, key)
	return nil
}

// Keys returns a snapshot of all keys.
func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// Close closes the underlying WAL file.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.walFile != nil {
		if err := s.walFile.Close(); err != nil {
			return err
		}
		s.walFile = nil
	}
	return nil
}

// appendRecord writes a single WAL record and fsyncs it.
func (s *Store) appendRecord(rec walRecord) error {
	// NOTE: callers must hold s.mu.Lock while calling this to serialize WAL access.
	b, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal wal record: %w", err)
	}
	b = append(b, '\n')

	if _, err := s.walFile.Write(b); err != nil {
		return fmt.Errorf("write wal: %w", err)
	}
	if err := s.walFile.Sync(); err != nil {
		return fmt.Errorf("sync wal: %w", err)
	}
	return nil
}
