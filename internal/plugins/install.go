package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// InstallFromPath installs a plugin binary from a local path.
func (s Store) InstallFromPath(name, path, version string, pluginType PluginType) (Plugin, error) {
	if err := ValidateName(name); err != nil {
		return Plugin{}, err
	}
	if pluginType == "" {
		pluginType = TypeExecutable
	}
	if path == "" {
		return Plugin{}, fmt.Errorf("install path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return Plugin{}, fmt.Errorf("stat plugin: %w", err)
	}
	if info.IsDir() {
		return Plugin{}, fmt.Errorf("plugin path %q is a directory", path)
	}

	if err := s.Ensure(); err != nil {
		return Plugin{}, err
	}

	dest := s.PluginPath(name, pluginType)
	if err := copyFile(path, dest, 0o755); err != nil {
		return Plugin{}, fmt.Errorf("install plugin: %w", err)
	}

	plugin := Plugin{
		Name:        name,
		Version:     version,
		Source:      path,
		InstalledAt: time.Now().UTC(),
		Path:        dest,
		Type:        pluginType,
	}
	if err := s.UpsertPlugin(plugin); err != nil {
		return Plugin{}, err
	}
	return plugin, nil
}

// InstallFromURL installs a plugin binary from a URL.
func (s Store) InstallFromURL(ctx context.Context, name, url, expectedSHA, version, description string, pluginType PluginType) (Plugin, error) {
	if err := ValidateName(name); err != nil {
		return Plugin{}, err
	}
	if pluginType == "" {
		pluginType = TypeExecutable
	}
	if url == "" {
		return Plugin{}, fmt.Errorf("install url is required")
	}
	if err := s.Ensure(); err != nil {
		return Plugin{}, err
	}

	tmp, err := os.CreateTemp(s.PluginsDir(), name+"-tmp-")
	if err != nil {
		return Plugin{}, fmt.Errorf("create temp: %w", err)
	}

	cleaned := false
	defer func() {
		if cleaned {
			return
		}
		_ = os.Remove(tmp.Name())
	}()

	hasher := sha256.New()
	writer := io.MultiWriter(tmp, hasher)

	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		client := &http.Client{Timeout: 20 * time.Second}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return Plugin{}, fmt.Errorf("build request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return Plugin{}, fmt.Errorf("download plugin: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return Plugin{}, fmt.Errorf("download plugin: status %s", resp.Status)
		}
		if _, err := io.Copy(writer, resp.Body); err != nil {
			return Plugin{}, fmt.Errorf("download plugin: %w", err)
		}
	} else {
		path := strings.TrimPrefix(url, "file://")
		src, err := os.Open(path)
		if err != nil {
			return Plugin{}, fmt.Errorf("open plugin: %w", err)
		}
		defer func() { _ = src.Close() }()
		if _, err := io.Copy(writer, src); err != nil {
			return Plugin{}, fmt.Errorf("copy plugin: %w", err)
		}
	}

	if err := tmp.Close(); err != nil {
		return Plugin{}, fmt.Errorf("finalize plugin: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return Plugin{}, fmt.Errorf("chmod plugin: %w", err)
	}

	if expectedSHA != "" {
		actual := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(actual, expectedSHA) {
			return Plugin{}, fmt.Errorf("checksum mismatch: expected %s got %s", expectedSHA, actual)
		}
	}

	dest := s.PluginPath(name, pluginType)
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return Plugin{}, fmt.Errorf("install plugin: %w", err)
	}
	cleaned = true

	plugin := Plugin{
		Name:        name,
		Version:     version,
		Description: description,
		Source:      url,
		InstalledAt: time.Now().UTC(),
		Path:        dest,
		Type:        pluginType,
	}
	if err := s.UpsertPlugin(plugin); err != nil {
		return Plugin{}, err
	}
	return plugin, nil
}

// InstallFromMarketplace installs a plugin using configured marketplaces.
func (s Store) InstallFromMarketplace(ctx context.Context, name, marketplaceName string) (Plugin, error) {
	marketplace, entry, err := s.ResolveMarketplacePlugin(ctx, name, marketplaceName)
	if err != nil {
		return Plugin{}, err
	}

	pluginType := entry.Type
	if pluginType == "" {
		pluginType = TypeExecutable
	}

	plugin, err := s.InstallFromURL(ctx, name, entry.URL, entry.SHA256, entry.Version, entry.Description, pluginType)
	if err != nil {
		return Plugin{}, err
	}

	plugin.Source = fmt.Sprintf("%s (%s)", marketplace.Name, entry.URL)
	if err := s.UpsertPlugin(plugin); err != nil {
		return Plugin{}, err
	}
	return plugin, nil
}

func copyFile(srcPath, destPath string, mode os.FileMode) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	dest, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() { _ = dest.Close() }()

	if _, err := io.Copy(dest, src); err != nil {
		return err
	}
	return dest.Sync()
}
