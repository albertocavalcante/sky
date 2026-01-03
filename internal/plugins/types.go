package plugins

import "time"

// Plugin describes an installed plugin.
type Plugin struct {
	Name        string     `json:"name"`
	Version     string     `json:"version,omitempty"`
	Description string     `json:"description,omitempty"`
	Source      string     `json:"source,omitempty"`
	InstalledAt time.Time  `json:"installed_at,omitempty"`
	Path        string     `json:"path,omitempty"`
	Type        PluginType `json:"type,omitempty"`
}

// Marketplace describes a plugin marketplace source.
type Marketplace struct {
	Name    string    `json:"name"`
	URL     string    `json:"url"`
	AddedAt time.Time `json:"added_at,omitempty"`
}

// MarketplaceIndex is the index payload fetched from a marketplace.
type MarketplaceIndex struct {
	Name      string              `json:"name"`
	UpdatedAt time.Time           `json:"updated_at,omitempty"`
	Plugins   []MarketplacePlugin `json:"plugins"`
}

// MarketplacePlugin describes a plugin entry in a marketplace index.
type MarketplacePlugin struct {
	Name        string     `json:"name"`
	Version     string     `json:"version,omitempty"`
	Description string     `json:"description,omitempty"`
	URL         string     `json:"url"`
	SHA256      string     `json:"sha256,omitempty"`
	Type        PluginType `json:"type,omitempty"`
}

// SearchResult captures a plugin matched in a marketplace.
type SearchResult struct {
	Marketplace Marketplace
	Plugin      MarketplacePlugin
}
