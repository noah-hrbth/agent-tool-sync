package syncer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Snapshot maps workspace-relative paths to their SHA-256 hex hashes at last sync.
type Snapshot struct {
	Files map[string]string `json:"files"`
}

func newSnapshot() *Snapshot {
	return &Snapshot{Files: make(map[string]string)}
}

// snapshotPath returns the path to the snapshot file.
func snapshotPath(workspace string) string {
	return filepath.Join(workspace, ".agentsync", ".state", "snapshot.json")
}

// loadSnapshot reads the snapshot from disk. Returns an empty snapshot if missing.
func loadSnapshot(workspace string) (*Snapshot, error) {
	data, err := os.ReadFile(snapshotPath(workspace))
	if os.IsNotExist(err) {
		return newSnapshot(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read snapshot: %w", err)
	}

	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse snapshot: %w", err)
	}
	if s.Files == nil {
		s.Files = make(map[string]string)
	}
	return &s, nil
}

// saveSnapshot writes the snapshot to disk, creating parent directories as needed.
func saveSnapshot(workspace string, s *Snapshot) error {
	path := snapshotPath(workspace)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return nil
}

// hashFile returns the SHA-256 hex hash of a file on disk.
// Returns ("", nil) if the file does not exist.
func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read file for hash: %w", err)
	}
	return hashBytes(data), nil
}

// hashBytes returns the SHA-256 hex hash of a byte slice.
func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
