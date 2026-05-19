// Package safepath validates filesystem paths before any write or delete so a
// repo-supplied relative path cannot escape the workspace via "..", an absolute
// path, or a planted symlink.
//
// Contract: root is assumed to be an already realpath-resolved directory (the
// caller resolves it once via filepath.EvalSymlinks at the CLI boundary).
// safepath validates only components strictly at or below root: it rejects
// non-local rel paths and any symlink encountered while descending from root,
// including a symlinked final target. It deliberately does NOT re-resolve root
// or forbid root itself being reached through OS symlinks (e.g. macOS
// /var->/private/var, a repo under a symlinked directory).
//
// A TOCTOU window exists between validation and the os call; this is an
// accepted residual for the single-user local-clone threat model (no
// concurrent attacker process racing the sync).
package safepath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrUnsafe wraps every rejection so callers can errors.Is against it.
var ErrUnsafe = errors.New("safepath: unsafe path")

// Error describes why a path was rejected; Path names the offending component.
type Error struct {
	Path   string
	Reason string
}

func (e *Error) Error() string { return fmt.Sprintf("safepath: %s: %s", e.Path, e.Reason) }

func (e *Error) Unwrap() error { return ErrUnsafe }

// Resolve validates rel against an already-resolved root and returns the
// cleaned absolute path. It errors if rel is empty or not local ("..",
// absolute, drive-relative), or if any existing component at or below root —
// including the final target — is a symlink.
func Resolve(root, rel string) (string, error) {
	if rel == "" || !filepath.IsLocal(rel) {
		return "", &Error{Path: rel, Reason: "not a local path"}
	}
	rel = filepath.Clean(rel)
	abs := filepath.Join(root, rel)
	if rel == "." {
		// rel refers to root itself; root is trusted, nothing to walk
		return abs, nil
	}

	// descend segment-by-segment; reject the first symlink below root
	cur := root
	for _, seg := range strings.Split(rel, string(os.PathSeparator)) {
		cur = filepath.Join(cur, seg)
		fi, err := os.Lstat(cur)
		if err != nil {
			if os.IsNotExist(err) {
				// this and every deeper component does not exist yet
				break
			}
			return "", fmt.Errorf("safepath stat %s: %w", cur, err)
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return "", &Error{Path: cur, Reason: "path component is a symlink"}
		}
	}
	return abs, nil
}
