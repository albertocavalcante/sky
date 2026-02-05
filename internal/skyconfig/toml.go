package skyconfig

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// LoadTOMLConfig loads a configuration from a TOML file.
func LoadTOMLConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing TOML config %s: %w", path, err)
	}

	return &cfg, nil
}
