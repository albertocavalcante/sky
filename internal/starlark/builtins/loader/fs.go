package loader

import (
	"fmt"
	"os"
	"path/filepath"
)

// fsReader is an interface for reading files (allows testing and different backends).
type fsReader interface {
	ReadFile(name string) ([]byte, error)
}

// diskFS reads files from the actual filesystem (for development/testing).
type diskFS struct {
	baseDir string
}

func (d *diskFS) ReadFile(name string) ([]byte, error) {
	fullPath := filepath.Join(d.baseDir, name)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", fullPath, err)
	}
	return data, nil
}

// embedFS is a simple in-memory filesystem for testing.
type embedFS struct {
	files map[string][]byte
}

func (e *embedFS) ReadFile(name string) ([]byte, error) {
	if data, ok := e.files[name]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("file not found: %s", name)
}
