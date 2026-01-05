// Package classifier provides interfaces and types for classifying Starlark files.
package classifier

import (
	"github.com/albertocavalcante/sky/internal/starlark/filekind"
)

// Classification represents the result of classifying a file path.
type Classification struct {
	// Dialect is the name of the dialect (e.g., "bazel", "buck2", "starlark").
	Dialect string

	// FileKind is the kind of file within the dialect.
	FileKind filekind.Kind

	// ConfigPath is an optional path to the config file that determined this classification.
	// For example, the path to a MODULE.bazel that indicates this is a Bazel workspace.
	ConfigPath string
}

// Classifier determines the dialect and file kind for a given path.
type Classifier interface {
	// Classify returns the classification for a file path.
	// The path may be absolute or workspace-relative.
	// Returns an error if the path cannot be classified.
	Classify(path string) (Classification, error)

	// SupportsDialect returns true if this classifier handles the named dialect.
	SupportsDialect(dialect string) bool
}

// ClassifierFunc is a function type that implements Classifier.
type ClassifierFunc func(path string) (Classification, error)

// Classify implements the Classifier interface.
func (f ClassifierFunc) Classify(path string) (Classification, error) {
	return f(path)
}

// SupportsDialect always returns true for ClassifierFunc.
// Override this if dialect filtering is needed.
func (f ClassifierFunc) SupportsDialect(dialect string) bool {
	return true
}

// ChainClassifier chains multiple classifiers, trying each in order.
type ChainClassifier struct {
	classifiers []Classifier
}

// NewChainClassifier creates a classifier that tries each classifier in order.
func NewChainClassifier(classifiers ...Classifier) *ChainClassifier {
	return &ChainClassifier{classifiers: classifiers}
}

// Classify tries each classifier in order until one succeeds.
func (c *ChainClassifier) Classify(path string) (Classification, error) {
	var lastErr error
	for _, cl := range c.classifiers {
		class, err := cl.Classify(path)
		if err == nil {
			return class, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return Classification{}, lastErr
	}
	return Classification{
		Dialect:  "starlark",
		FileKind: filekind.KindUnknown,
	}, nil
}

// SupportsDialect returns true if any classifier supports the dialect.
func (c *ChainClassifier) SupportsDialect(dialect string) bool {
	for _, cl := range c.classifiers {
		if cl.SupportsDialect(dialect) {
			return true
		}
	}
	return false
}
