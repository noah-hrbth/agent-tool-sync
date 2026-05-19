package safepath

import (
	"os"
	"path/filepath"
)

// WriteFile validates rel, creates its parent directory, and writes data.
// Both the target and every parent component are checked for symlinks before
// any os.MkdirAll/os.WriteFile call, so a planted ancestor symlink is rejected
// before it can be followed.
func WriteFile(root, rel string, data []byte, perm os.FileMode) error {
	abs, err := Resolve(root, rel)
	if err != nil {
		return err
	}
	if err := MkdirAll(root, filepath.Dir(rel), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, data, perm)
}

// MkdirAll validates rel and creates the directory tree at root/rel.
func MkdirAll(root, rel string, perm os.FileMode) error {
	abs, err := Resolve(root, rel)
	if err != nil {
		return err
	}
	return os.MkdirAll(abs, perm)
}

// Remove validates rel and removes the single file at root/rel.
func Remove(root, rel string) error {
	abs, err := Resolve(root, rel)
	if err != nil {
		return err
	}
	return os.Remove(abs)
}

// RemoveAll validates rel and recursively removes the tree at root/rel.
func RemoveAll(root, rel string) error {
	abs, err := Resolve(root, rel)
	if err != nil {
		return err
	}
	return os.RemoveAll(abs)
}

// Rename validates both paths and renames root/oldRel to root/newRel.
func Rename(root, oldRel, newRel string) error {
	oldAbs, err := Resolve(root, oldRel)
	if err != nil {
		return err
	}
	newAbs, err := Resolve(root, newRel)
	if err != nil {
		return err
	}
	return os.Rename(oldAbs, newAbs)
}

// ReadFile validates rel and reads root/rel. A missing file surfaces as the
// underlying os error, so callers can still use os.IsNotExist / os.ErrNotExist.
func ReadFile(root, rel string) ([]byte, error) {
	abs, err := Resolve(root, rel)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(abs)
}
