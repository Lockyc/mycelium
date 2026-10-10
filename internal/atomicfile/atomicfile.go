// Package atomicfile replaces files and directories so a concurrent reader sees
// either the old content or the new, never a partial write or a gap.
package atomicfile

import (
	"os"
	"path/filepath"
)

// WriteFile writes data to path via a temp file in the same directory plus a
// rename. The temp name is "."+base+".*.tmp" — hidden, and never ending in the
// target's extension — so a temp leaked by a failed rename is ignored by any
// reader that globs the directory for real files (loadManifests reads *.json).
func WriteFile(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// ReplaceDir fills a fresh sibling of dir via fill, then swaps it into place:
// the old dir is renamed aside, the new one renamed in, and the old one removed.
// A reader never sees a half-filled dir; the only gap is between the two
// renames. If fill fails, dir is left untouched.
func ReplaceDir(dir string, fill func(tmp string) error) error {
	parent, base := filepath.Dir(dir), filepath.Base(dir)
	tmp, err := os.MkdirTemp(parent, "."+base+".*.tmp")
	if err != nil {
		return err
	}
	if err := fill(tmp); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	trash := ""
	if _, err := os.Stat(dir); err == nil {
		trash = tmp + ".old"
		if err := os.Rename(dir, trash); err != nil {
			os.RemoveAll(tmp)
			return err
		}
	}
	if err := os.Rename(tmp, dir); err != nil {
		if trash != "" {
			os.Rename(trash, dir)
		}
		os.RemoveAll(tmp)
		return err
	}
	if trash != "" {
		return os.RemoveAll(trash)
	}
	return nil
}
