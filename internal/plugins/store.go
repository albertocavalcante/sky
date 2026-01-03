package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/gofrs/flock"
)

// Store manages the on-disk plugin catalog and marketplace list.
type Store struct {
	Root string
}

// NewStore creates a store rooted at the provided path.
func NewStore(root string) *Store {
	return &Store{Root: root}
}

// DefaultStore creates a store in the user config directory.
func DefaultStore() (*Store, error) {
	if override := os.Getenv("SKY_CONFIG_DIR"); override != "" {
		return &Store{Root: override}, nil
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("config dir: %w", err)
	}
	return &Store{Root: filepath.Join(base, "sky")}, nil
}

// Ensure creates the config directories if needed.
func (s Store) Ensure() error {
	if err := os.MkdirAll(s.Root, 0o755); err != nil {
		return fmt.Errorf("config dir: %w", err)
	}
	if err := os.MkdirAll(s.PluginsDir(), 0o755); err != nil {
		return fmt.Errorf("plugins dir: %w", err)
	}
	return nil
}

// PluginsDir returns the plugin binaries directory.
func (s Store) PluginsDir() string {
	return filepath.Join(s.Root, "plugins")
}

// PluginsFile returns the plugins catalog path.
func (s Store) PluginsFile() string {
	return filepath.Join(s.Root, "plugins.json")
}

// MarketplacesFile returns the marketplace catalog path.
func (s Store) MarketplacesFile() string {
	return filepath.Join(s.Root, "marketplaces.json")
}

// LockFile returns the path to the lock file.
func (s Store) LockFile() string {
	return filepath.Join(s.Root, "lock")
}

func (s Store) withLock(fn func() error) error {
	if err := s.Ensure(); err != nil {
		return err
	}

	fileLock := flock.New(s.LockFile())
	if err := fileLock.Lock(); err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer func() { _ = fileLock.Unlock() }()

	return fn()
}

// PluginPath returns the expected binary path for a plugin type.
func (s Store) PluginPath(name string, pluginType PluginType) string {
	filename := name
	if pluginType == TypeWasm {
		filename = name + ".wasm"
	}
	return filepath.Join(s.PluginsDir(), filename)
}

// LoadPlugins loads the installed plugins list.
func (s Store) LoadPlugins() ([]Plugin, error) {
	var plugins []Plugin
	if err := readJSON(s.PluginsFile(), &plugins); err != nil {
		return nil, fmt.Errorf("load plugins: %w", err)
	}
	if plugins == nil {
		plugins = []Plugin{}
	}
	for i := range plugins {
		if plugins[i].Type == "" {
			plugins[i].Type = TypeExecutable
		}
	}
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Name < plugins[j].Name
	})
	return plugins, nil
}

// SavePlugins persists the plugin list.
func (s Store) SavePlugins(plugins []Plugin) error {
	if err := s.Ensure(); err != nil {
		return err
	}
	return writeJSON(s.PluginsFile(), plugins)
}

// UpsertPlugin inserts or replaces a plugin entry.
func (s Store) UpsertPlugin(plugin Plugin) error {
	return s.withLock(func() error {
		if err := ValidateName(plugin.Name); err != nil {
			return err
		}

		plugins, err := s.LoadPlugins()
		if err != nil {
			return err
		}

		replaced := false
		for i := range plugins {
			if plugins[i].Name == plugin.Name {
				plugins[i] = plugin
				replaced = true
				break
			}
		}
		if !replaced {
			plugins = append(plugins, plugin)
		}
		return s.SavePlugins(plugins)
	})
}

// FindPlugin returns the plugin entry if installed.
func (s Store) FindPlugin(name string) (*Plugin, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}

	plugins, err := s.LoadPlugins()
	if err != nil {
		return nil, err
	}
	for _, plugin := range plugins {
		if plugin.Name == name {
			found := plugin
			if found.Path == "" {
				found.Path = s.PluginPath(found.Name, found.EffectiveType())
			}
			return &found, nil
		}
	}
	return nil, nil
}

// RemovePlugin removes a plugin entry and its binary.
func (s Store) RemovePlugin(name string) (*Plugin, error) {
	var removed *Plugin
	err := s.withLock(func() error {
		if err := ValidateName(name); err != nil {
			return err
		}

		plugins, err := s.LoadPlugins()
		if err != nil {
			return err
		}

		remaining := make([]Plugin, 0, len(plugins))
		for _, plugin := range plugins {
			if plugin.Name == name {
				copy := plugin
				removed = &copy
				continue
			}
			remaining = append(remaining, plugin)
		}

		if removed == nil {
			return fmt.Errorf("plugin %q not installed", name)
		}

		if err := s.SavePlugins(remaining); err != nil {
			return err
		}

		if err := os.Remove(s.PluginPath(name, removed.EffectiveType())); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove plugin binary: %w", err)
		}
		return nil
	})
	return removed, err
}

// LoadMarketplaces loads the configured marketplaces.
func (s Store) LoadMarketplaces() ([]Marketplace, error) {
	var marketplaces []Marketplace
	if err := readJSON(s.MarketplacesFile(), &marketplaces); err != nil {
		return nil, fmt.Errorf("load marketplaces: %w", err)
	}
	if marketplaces == nil {
		marketplaces = []Marketplace{}
	}
	sort.Slice(marketplaces, func(i, j int) bool {
		return marketplaces[i].Name < marketplaces[j].Name
	})
	return marketplaces, nil
}

// SaveMarketplaces persists the marketplace list.
func (s Store) SaveMarketplaces(marketplaces []Marketplace) error {
	if err := s.Ensure(); err != nil {
		return err
	}
	return writeJSON(s.MarketplacesFile(), marketplaces)
}

// UpsertMarketplace inserts or replaces a marketplace entry.
func (s Store) UpsertMarketplace(marketplace Marketplace) error {
	return s.withLock(func() error {
		if err := ValidateName(marketplace.Name); err != nil {
			return err
		}
		if marketplace.URL == "" {
			return fmt.Errorf("marketplace url is required")
		}

		marketplaces, err := s.LoadMarketplaces()
		if err != nil {
			return err
		}

		replaced := false
		for i := range marketplaces {
			if marketplaces[i].Name == marketplace.Name {
				marketplaces[i] = marketplace
				replaced = true
				break
			}
		}
		if !replaced {
			marketplaces = append(marketplaces, marketplace)
		}
		return s.SaveMarketplaces(marketplaces)
	})
}

// RemoveMarketplace removes a marketplace entry.
func (s Store) RemoveMarketplace(name string) (*Marketplace, error) {
	var removed *Marketplace
	err := s.withLock(func() error {
		if err := ValidateName(name); err != nil {
			return err
		}

		marketplaces, err := s.LoadMarketplaces()
		if err != nil {
			return err
		}

		remaining := make([]Marketplace, 0, len(marketplaces))
		for _, marketplace := range marketplaces {
			if marketplace.Name == name {
				copy := marketplace
				removed = &copy
				continue
			}
			remaining = append(remaining, marketplace)
		}

		if removed == nil {
			return fmt.Errorf("marketplace %q not configured", name)
		}

		return s.SaveMarketplaces(remaining)
	})
	return removed, err
}

func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, target)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*.json")
	if err != nil {
		return err
	}

	cleaned := false
	defer func() {
		if cleaned {
			return
		}
		_ = os.Remove(tmp.Name())
	}()

	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	cleaned = true
	return nil
}
