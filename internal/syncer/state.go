package syncer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/safepath"
)

// Snapshot maps workspace-relative paths to their SHA-256 hex hashes at last sync.
type Snapshot struct {
	Files map[string]string `json:"files"`
}

func newSnapshot() *Snapshot {
	return &Snapshot{Files: make(map[string]string)}
}

// snapshotRel is the workspace-relative path to the snapshot file.
var snapshotRel = filepath.Join(".agentsync", ".state", "snapshot.json")

// loadSnapshot reads the snapshot from disk. Returns an empty snapshot if missing.
func loadSnapshot(workspace string) (*Snapshot, error) {
	data, err := safepath.ReadFile(workspace, snapshotRel)
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
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	if err := safepath.WriteFile(workspace, snapshotRel, data, 0o644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return nil
}

// hashFileSafe returns the SHA-256 hex hash of a workspace-relative file.
// Returns ("", nil) if the file does not exist; ("", err) if the path is
// unsafe (e.g. escapes the workspace or crosses a symlink).
func hashFileSafe(workspace, rel string) (string, error) {
	data, err := safepath.ReadFile(workspace, rel)
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
