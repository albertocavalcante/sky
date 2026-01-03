package plugins

import (
	"fmt"
	"regexp"
)

var pluginNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// ValidateName ensures a plugin name is safe for filesystem usage.
func ValidateName(name string) error {
	if !pluginNameRe.MatchString(name) {
		return fmt.Errorf("invalid plugin name %q", name)
	}
	return nil
}
