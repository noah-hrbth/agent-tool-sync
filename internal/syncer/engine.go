package syncer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// FileStatus is the sync state of a single output file.
type FileStatus int

const (
	StatusSynced    FileStatus = iota // disk hash matches snapshot
	StatusDivergent                   // disk hash differs from snapshot (external edit)
	StatusMissing                     // snapshot has entry but file is gone
	StatusNew                         // no snapshot entry; first sync
)

// FileResult is the status of one adapter output file.
type FileResult struct {
	ToolName string
	Path     string     // workspace-relative
	Status   FileStatus
	Content  []byte     // content to write (from adapter Render)
}

// SyncedFile is one written or skipped file with its tool and concept attribution.
type SyncedFile struct {
	ToolName string
	Concept  tools.Concept
	Path     string // workspace-relative
}

// SyncResult summarises a completed sync run.
type SyncResult struct {
	Written  []SyncedFile
	Skipped  []SyncedFile
	Errors   []error
	Warnings []string
}

// SyncOptions controls per-file behaviour during a RunSync call.
type SyncOptions struct {
	// Skip contains workspace-relative paths to not write (deferred files).
	Skip map[string]bool
}

// Status computes the FileResult list for all enabled adapters without writing.
// This is used by the TUI to check for divergences before syncing.
// Adapters whose SupportsScope(scope) is false are skipped.
func Status(workspace string, c *canonical.Canonical, adapters []tools.Adapter, cfg *config.Config, scope tools.Scope) ([]FileResult, error) {
	snap, err := loadSnapshot(workspace)
	if err != nil {
		return nil, err
	}

	var results []FileResult
	for _, a := range adapters {
		if !cfg.IsEnabled(a.Name()) {
			continue
		}
		if !a.SupportsScope(scope).Supported {
			continue
		}
		writes, err := a.Render(c, scope)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", a.Name(), err)
		}
		for _, fw := range writes {
			diskHash, err := hashFile(filepath.Join(workspace, fw.Path))
			if err != nil {
				return nil, err
			}
			snapHash, hasSnap := snap.Files[fw.Path]
			results = append(results, FileResult{
				ToolName: a.Name(),
				Path:     fw.Path,
				Status:   fileStatus(diskHash, snapHash, hasSnap),
				Content:  fw.Content,
			})
		}
	}
	return results, nil
}

// fileStatus determines FileStatus from disk and snapshot hashes.
func fileStatus(diskHash, snapHash string, hasSnap bool) FileStatus {
	if !hasSnap {
		return StatusNew
	}
	if diskHash == "" {
		return StatusMissing
	}
	if diskHash != snapHash {
		return StatusDivergent
	}
	return StatusSynced
}

// RunSync writes all enabled adapter output files, then saves the snapshot.
// Files in opts.Skip are not written (deferred divergences).
// Callers should run Status first, resolve divergences, then call RunSync
// with the deferred paths in opts.Skip.
// Adapters whose SupportsScope(scope) is false are skipped.
func RunSync(workspace string, c *canonical.Canonical, adapters []tools.Adapter, cfg *config.Config, scope tools.Scope, opts SyncOptions) (*SyncResult, error) {
	snap, err := loadSnapshot(workspace)
	if err != nil {
		return nil, err
	}

	// Pre-compute all paths any scope-compatible adapter would render regardless of enabled state.
	// Orphan cleanup uses this so disabling a tool doesn't auto-delete its synced files.
	allRenderedPaths := make(map[string]bool)
	for _, a := range adapters {
		if !a.SupportsScope(scope).Supported {
			continue
		}
		writes, err := a.Render(c, scope)
		if err != nil {
			continue
		}
		for _, fw := range writes {
			allRenderedPaths[fw.Path] = true
		}
	}

	result := &SyncResult{}

	for _, a := range adapters {
		if !cfg.IsEnabled(a.Name()) {
			continue
		}
		if !a.SupportsScope(scope).Supported {
			continue
		}
		writes, err := a.Render(c, scope)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("render %s: %w", a.Name(), err))
			continue
		}
		for _, fw := range writes {
			if opts.Skip[fw.Path] {
				result.Skipped = append(result.Skipped, SyncedFile{ToolName: a.Name(), Concept: fw.Concept, Path: fw.Path})
				continue
			}
			absPath := filepath.Join(workspace, fw.Path)
			if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
				result.Errors = append(result.Errors, err)
				continue
			}
			if err := os.WriteFile(absPath, fw.Content, 0o644); err != nil {
				result.Errors = append(result.Errors, err)
				continue
			}
			snap.Files[fw.Path] = hashBytes(fw.Content)
			result.Written = append(result.Written, SyncedFile{ToolName: a.Name(), Concept: fw.Concept, Path: fw.Path})
		}
	}

	// Orphan cleanup: remove snapshot-tracked paths no longer rendered by any adapter (enabled or not).
	// Uses allRenderedPaths so that disabling a tool does not auto-delete its previously-synced files.
	for snapPath, snapHash := range snap.Files {
		if allRenderedPaths[snapPath] {
			continue
		}
		absPath := filepath.Join(workspace, snapPath)
		diskHash, err := hashFile(absPath)
		if err != nil {
			result.Errors = append(result.Errors, err)
			delete(snap.Files, snapPath)
			continue
		}
		if diskHash == "" {
			// File already gone; prune snapshot entry.
			delete(snap.Files, snapPath)
			continue
		}
		if diskHash == snapHash {
			// Safe to delete: matches last-synced content.
			if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
				result.Errors = append(result.Errors, err)
			}
			delete(snap.Files, snapPath)
		} else {
			// User-modified orphan: preserve file, prune snapshot, warn.
			result.Warnings = append(result.Warnings, fmt.Sprintf("orphan %s has local edits — not deleted", snapPath))
			delete(snap.Files, snapPath)
		}
	}

	if err := saveSnapshot(workspace, snap); err != nil {
		return result, err
	}
	return result, nil
}
