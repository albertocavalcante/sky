// Package atomicfile replaces files atomically without ever exposing content
// at a wider permission than the final one.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Write atomically replaces path with data. The temporary file is created
// exclusively (mode 0600) next to path with the shortest practical name (".t"
// plus the unique suffix), so any destination that fits the path limit also
// fits its temporary file. It is chmodded to its final mode before any content
// is written, then renamed into place. An existing destination keeps its
// permission bits; a new one gets perm subject to the process umask.
func Write(path string, data []byte, perm os.FileMode) error {
	mode, err := finalMode(filepath.Dir(path), path, perm)
	if err != nil {
		return err
	}
	return WriteExact(path, data, mode)
}

// WriteExact atomically replaces path with data and gives the new file
// exactly mode (no umask, no inspection of the existing destination). The
// mode is set on the open temporary file before any content is written and
// before the rename, so no path-based chmod is needed afterwards.
func WriteExact(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".t*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	fail := func(err error) error {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	// The temp file is 0600 here, so nothing is readable yet. A wider final
	// mode is applied before the content is written (a narrower one is
	// already satisfied).
	if err := tmp.Chmod(mode); err != nil {
		return fail(err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}

func finalMode(dir, path string, perm os.FileMode) (os.FileMode, error) {
	if fi, err := os.Stat(path); err == nil {
		return fi.Mode().Perm(), nil
	}
	// New file: learn the umask-applied mode by creating a probe file.
	probe, err := os.CreateTemp(dir, ".u*")
	if err != nil {
		return 0, err
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	// CreateTemp forces 0600, so emulate the umask with a plain create.
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return 0, fmt.Errorf("probing umask: %w", err)
	}
	fi, err := f.Stat()
	_ = f.Close()
	_ = os.Remove(name)
	if err != nil {
		return 0, err
	}
	return fi.Mode().Perm(), nil
}
