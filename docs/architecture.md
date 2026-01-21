# Architecture

## Control-plane data store

The control-plane needs a small, reliable source of truth for cluster state and metadata.
Examples of the kind of data it tracks:

- **Registered devices**: IDs, capabilities, resource limits.
- **Model deployments**: which model/version should run on which devices.
- **Scheduling decisions and assignments**: which device is currently responsible for which workload.

Storing this only in memory would make the control-plane *ephemeral* (all state is lost on restart),
while using a full external database (Postgres, etcd itself, …) would add significant operational
overhead for edge devices. To strike a balance, the control-plane uses a **minimal, embedded,
disk-backed key–value store** that lives in-process.

### Design goals

- **Durability**: survive control-plane restarts and crashes.
- **Simplicity**: keep the implementation small and easy to reason about.
- **Concurrency-safety**: support concurrent readers and writers inside the control-plane process.
- **Testability**: easy to unit test, including error paths and concurrency behavior.

This is conceptually similar to a tiny, single-node etcd: there is an in-memory map for fast reads,
and an **append-only write-ahead log (WAL)** on disk that is the durable record of all changes.

### How the store works

The store is implemented in `internal/control-plane/store` and exposes a minimal API:

- **`New(dataDir string) (*Store, error)`**: creates/opens the store in a given directory.
- **`Put(key string, value []byte) error`**: set a value.
- **`Get(key string) ([]byte, bool)`**: read a value.
- **`Delete(key string) error`**: remove a key.
- **`Keys() []string`**: list all keys.
- **`Close() error`**: flush and close the underlying WAL file.

Under the hood:

- **In-memory state**: a `map[string][]byte` holds the current state for fast access.
- **WAL file**: each mutation is encoded as a JSON record and appended to `store.wal` on disk.
- **Replay on startup**:
  - On `New(dataDir)`, the store:
    - Ensures the data directory exists.
    - Opens (or creates) `store.wal` for reading.
    - Scans the file line-by-line, decoding each JSON record into an operation.
    - Applies each operation to the in-memory map (`Put` / `Delete` semantics).
  - After replay, the WAL is re-opened in **append** mode for new writes.

This means the **authoritative state** is effectively the sequence of WAL records on disk.
The in-memory map is just a cached projection of that sequence, reconstructed at startup.

### Write path

When a client calls `Put` or `Delete`:

1. The store validates the input (e.g. empty keys are rejected).
2. A WAL record is created:
   - For `Put`: `{ "op": "put", "key": "...", "value": <bytes> }`
   - For `Delete`: `{ "op": "delete", "key": "..." }`
3. The record is marshaled to JSON, a newline is appended, and the bytes are written to `store.wal`.
4. The WAL file is `Sync()`ed to ensure the bytes are flushed to disk.
5. The in-memory map is updated to reflect the new state.

If the process crashes at any point *after* the WAL write and before the in-memory update,
the next startup will replay the WAL and recover a consistent state.

### Read path

Reads (`Get`, `Keys`) operate purely on the in-memory map:

- `Get` returns a copy of the stored byte slice (to avoid callers mutating internal memory).
- `Keys` returns a snapshot list of keys at the time the call was made.

No disk I/O happens on the read path, which keeps reads fast and predictable.

### Locking and concurrency model

The store is used concurrently by multiple goroutines inside the control-plane, so it must be
safe under concurrent access. The concurrency model is intentionally simple:

- The store has a single **`sync.RWMutex`**, named `mu`.
- This mutex **protects both**:
  - The in-memory map (`data`).
  - The single open WAL file handle (`walFile`).

The locking rules are:

- **Writes (`Put`, `Delete`)**
  - Take **`mu.Lock()`** for the entire duration of the operation:
    - Build the WAL record.
    - Append it to the WAL file and `Sync()` it.
    - Apply the change to the in-memory map.
  - This guarantees:
    - No two writers can interleave their WAL writes.
    - WAL file operations are serialized with in-memory mutations.
    - The WAL and the in-memory map always represent a valid sequence of operations.

- **Reads (`Get`, `Keys`)**
  - Take **`mu.RLock()`** / `mu.RUnlock()`:
    - Multiple readers can proceed concurrently.
    - Readers are blocked while a writer holds `mu.Lock()`.

- **Close (`Close`)**
  - Takes **`mu.Lock()`**, closes the WAL file if it is still open, and sets the internal
    file pointer to `nil`.
  - Subsequent calls to `Close` are safe no-ops (they see `walFile == nil` and return `nil`).

This “single lock for everything” approach has a few important properties:

- **No deadlocks**: there is only one mutex; it is never acquired recursively.
- **No file races**: file append and `Sync()` are always done while holding the same mutex
  that protects the in-memory map, so WAL writes cannot be interleaved by other operations.
- **Predictable behavior under load**: readers may be briefly blocked by writers, but the
  implementation is straightforward to reason about and test.

More sophisticated designs (e.g. separate locks for WAL and memory, background snapshotting,
or multi-node replication like etcd’s Raft log) can be built on top of this foundation as
the project evolves and requirements grow. For now, the single-process, single-lock model
provides a **simple, correct, and well-tested** control-plane data store.