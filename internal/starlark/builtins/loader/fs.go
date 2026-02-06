package loader

import (
	"fmt"
)

// fsReader is an interface for reading files (allows testing and different backends).
type fsReader interface {
	ReadFile(name string) ([]byte, error)
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
