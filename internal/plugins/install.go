package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// InstallFromPath installs a plugin binary from a local path.
func (s *Store) InstallFromPath(name, path, version string, pluginType PluginType) (Plugin, error) {
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

// maxDownloadBytes caps the size of a downloaded plugin.
const maxDownloadBytes = 256 << 20

// newDownloadClient returns an HTTP client with dial/header timeouts but no
// whole-body timeout; callers rely on the context for overall cancellation.
func newDownloadClient() *http.Client {
	return &http.Client{Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
	}}
}

// copyLimited copies src to dst, failing if more than limit bytes are read.
func copyLimited(dst io.Writer, src io.Reader, limit int64) error {
	n, err := io.Copy(dst, io.LimitReader(src, limit+1))
	if err != nil {
		return err
	}
	if n > limit {
		return fmt.Errorf("plugin exceeds maximum size of %d bytes", limit)
	}
	return nil
}

// installOptions carries internal install parameters.
type installOptions struct {
	source     string
	requireSHA bool
	limit      int64
}

// InstallFromURL installs a plugin binary from a URL.
func (s *Store) InstallFromURL(ctx context.Context, name, url, expectedSHA, version, description string, pluginType PluginType) (Plugin, error) {
	return s.installFromURL(ctx, name, url, expectedSHA, version, description, pluginType, installOptions{source: url})
}

func (s *Store) installFromURL(ctx context.Context, name, url, expectedSHA, version, description string, pluginType PluginType, opts installOptions) (Plugin, error) {
	if err := ValidateName(name); err != nil {
		return Plugin{}, err
	}
	if pluginType == "" {
		pluginType = TypeExecutable
	}
	if pluginType != TypeExecutable && pluginType != TypeWasm {
		return Plugin{}, fmt.Errorf("invalid plugin type %q", pluginType)
	}
	if url == "" {
		return Plugin{}, fmt.Errorf("install url is required")
	}
	if opts.requireSHA && expectedSHA == "" {
		return Plugin{}, fmt.Errorf("plugin %q has no sha256 checksum in marketplace entry; refusing to install", name)
	}
	limit := opts.limit
	if limit <= 0 {
		limit = maxDownloadBytes
	}
	if err := s.Ensure(); err != nil {
		return Plugin{}, err
	}

	tmp, err := os.CreateTemp(s.PluginsDir(), name+"-tmp-")
	if err != nil {
		return Plugin{}, fmt.Errorf("create temp: %w", err)
	}

	cleaned := false
	closed := false
	defer func() {
		if !closed {
			_ = tmp.Close()
		}
		if cleaned {
			return
		}
		_ = os.Remove(tmp.Name())
	}()

	hasher := sha256.New()
	writer := io.MultiWriter(tmp, hasher)

	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		client := newDownloadClient()
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
		if err := copyLimited(writer, resp.Body, limit); err != nil {
			return Plugin{}, fmt.Errorf("download plugin: %w", err)
		}
	} else {
		path := strings.TrimPrefix(url, "file://")
		src, err := os.Open(path)
		if err != nil {
			return Plugin{}, fmt.Errorf("open plugin: %w", err)
		}
		defer func() { _ = src.Close() }()
		if err := copyLimited(writer, src, limit); err != nil {
			return Plugin{}, fmt.Errorf("copy plugin: %w", err)
		}
	}

	closed = true
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
	source := opts.source
	if source == "" {
		source = url
	}
	plugin := Plugin{
		Name:        name,
		Version:     version,
		Description: description,
		Source:      source,
		InstalledAt: time.Now().UTC(),
		Path:        dest,
		Type:        pluginType,
	}

	err = s.withWriteLock(func() error {
		plugins, err := s.loadPluginsNL()
		if err != nil {
			return err
		}
		oldPath := ""
		for _, p := range plugins {
			if p.Name == name {
				oldPath = p.Path
				if oldPath == "" {
					oldPath = s.PluginPath(name, p.EffectiveType())
				}
				break
			}
		}
		if err := os.Rename(tmp.Name(), dest); err != nil {
			return fmt.Errorf("install plugin: %w", err)
		}
		cleaned = true
		if err := s.upsertPluginNL(plugin); err != nil {
			return err
		}
		if oldPath != "" && filepath.Clean(oldPath) != filepath.Clean(dest) {
			if err := os.Remove(oldPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove previous plugin binary: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return Plugin{}, err
	}
	return plugin, nil
}

// InstallFromMarketplace installs a plugin using configured marketplaces.
// A sha256 checksum is mandatory for marketplace entries.
func (s *Store) InstallFromMarketplace(ctx context.Context, name, marketplaceName string) (Plugin, error) {
	marketplace, entry, err := s.ResolveMarketplacePlugin(ctx, name, marketplaceName)
	if err != nil {
		return Plugin{}, err
	}

	pluginType := entry.Type
	if pluginType == "" {
		pluginType = TypeExecutable
	}

	return s.installFromURL(ctx, name, entry.URL, entry.SHA256, entry.Version, entry.Description, pluginType, installOptions{
		source:     fmt.Sprintf("%s (%s)", marketplace.Name, entry.URL),
		requireSHA: true,
	})
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
