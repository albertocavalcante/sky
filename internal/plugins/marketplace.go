package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// SearchMarketplaces returns plugins matching the query across marketplaces.
func (s Store) SearchMarketplaces(ctx context.Context, query, marketplaceName string) ([]SearchResult, error) {
	marketplaces, err := s.LoadMarketplaces()
	if err != nil {
		return nil, err
	}
	if len(marketplaces) == 0 {
		return nil, fmt.Errorf("no marketplaces configured")
	}

	query = strings.ToLower(query)
	var results []SearchResult
	matchedMarketplace := false
	for _, marketplace := range marketplaces {
		if marketplaceName != "" && marketplace.Name != marketplaceName {
			continue
		}
		if marketplaceName != "" {
			matchedMarketplace = true
		}

		index, err := fetchMarketplaceIndex(ctx, marketplace)
		if err != nil {
			return nil, err
		}

		for _, plugin := range index.Plugins {
			name := strings.ToLower(plugin.Name)
			desc := strings.ToLower(plugin.Description)
			if query == "" || strings.Contains(name, query) || strings.Contains(desc, query) {
				results = append(results, SearchResult{Marketplace: marketplace, Plugin: plugin})
			}
		}
	}

	if marketplaceName != "" && !matchedMarketplace {
		return nil, fmt.Errorf("marketplace %q not configured", marketplaceName)
	}
	if marketplaceName != "" && len(results) == 0 {
		return nil, fmt.Errorf("no matches in marketplace %q", marketplaceName)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no plugins matched %q", query)
	}
	return results, nil
}

// ResolveMarketplacePlugin finds a plugin entry by name.
func (s Store) ResolveMarketplacePlugin(ctx context.Context, name, marketplaceName string) (Marketplace, MarketplacePlugin, error) {
	if err := ValidateName(name); err != nil {
		return Marketplace{}, MarketplacePlugin{}, err
	}

	marketplaces, err := s.LoadMarketplaces()
	if err != nil {
		return Marketplace{}, MarketplacePlugin{}, err
	}
	if len(marketplaces) == 0 {
		return Marketplace{}, MarketplacePlugin{}, fmt.Errorf("no marketplaces configured")
	}

	matchedMarketplace := false
	for _, marketplace := range marketplaces {
		if marketplaceName != "" && marketplace.Name != marketplaceName {
			continue
		}
		if marketplaceName != "" {
			matchedMarketplace = true
		}

		index, err := fetchMarketplaceIndex(ctx, marketplace)
		if err != nil {
			return Marketplace{}, MarketplacePlugin{}, err
		}

		for _, plugin := range index.Plugins {
			if plugin.Name == name {
				return marketplace, plugin, nil
			}
		}
	}

	if marketplaceName != "" && !matchedMarketplace {
		return Marketplace{}, MarketplacePlugin{}, fmt.Errorf("marketplace %q not configured", marketplaceName)
	}
	if marketplaceName != "" {
		return Marketplace{}, MarketplacePlugin{}, fmt.Errorf("plugin %q not found in marketplace %q", name, marketplaceName)
	}
	return Marketplace{}, MarketplacePlugin{}, fmt.Errorf("plugin %q not found in marketplaces", name)
}

func fetchMarketplaceIndex(ctx context.Context, marketplace Marketplace) (MarketplaceIndex, error) {
	source := marketplace.URL
	var decoder *json.Decoder

	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		client := &http.Client{Timeout: 10 * time.Second}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if err != nil {
			return MarketplaceIndex{}, fmt.Errorf("marketplace %q: %w", marketplace.Name, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return MarketplaceIndex{}, fmt.Errorf("marketplace %q: %w", marketplace.Name, err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return MarketplaceIndex{}, fmt.Errorf("marketplace %q: unexpected status %s", marketplace.Name, resp.Status)
		}
		decoder = json.NewDecoder(resp.Body)
	} else {
		path := strings.TrimPrefix(source, "file://")
		file, err := os.Open(path)
		if err != nil {
			return MarketplaceIndex{}, fmt.Errorf("marketplace %q: %w", marketplace.Name, err)
		}
		defer func() { _ = file.Close() }()
		decoder = json.NewDecoder(file)
	}

	var index MarketplaceIndex
	if err := decoder.Decode(&index); err != nil {
		return MarketplaceIndex{}, fmt.Errorf("marketplace %q: %w", marketplace.Name, err)
	}
	return index, nil
}
