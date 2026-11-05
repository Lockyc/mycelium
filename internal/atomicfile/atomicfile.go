// Package atomicfile replaces files so a concurrent reader sees
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
