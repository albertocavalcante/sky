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
	tmp, err := os.CreateTemp(s.PluginsDir(), name+"-tmp-")
	if err != nil {
		return Plugin{}, fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpName) }() // no-op after a successful rename
	if err := copyFile(path, tmpName, 0o755); err != nil {
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
	if err := s.commitInstall(tmpName, plugin); err != nil {
		return Plugin{}, err
	}
	return plugin, nil
}

// commitInstall moves the staged binary into place, records the plugin and
// removes the previous binary, all under the store write lock. The previous
// binary is only removed if it lives inside this store's plugins directory and
// is not the freshly installed destination.
func (s *Store) commitInstall(tmpName string, plugin Plugin) error {
	dest := plugin.Path
	return s.withWriteLock(func() error {
		plugins, err := s.loadPluginsNL()
		if err != nil {
			return err
		}
		oldPath := ""
		for _, p := range plugins {
			if p.Name == plugin.Name {
				oldPath = p.Path
				if oldPath == "" {
					oldPath = s.PluginPath(plugin.Name, p.EffectiveType())
				}
				break
			}
		}
		if err := os.Chmod(tmpName, 0o755); err != nil {
			return fmt.Errorf("chmod plugin: %w", err)
		}
		// Park the live binary under a backup name so a failed catalog write
		// can put it back; the catalog and the binary must never disagree.
		backup, err := s.parkBinary(dest, plugin.Name)
		if err != nil {
			return err
		}
		if err := os.Rename(tmpName, dest); err != nil {
			restoreBinary(backup, dest)
			return fmt.Errorf("install plugin: %w", err)
		}
		if err := s.upsertPluginNL(plugin); err != nil {
			restoreBinary(backup, dest)
			return err
		}
		if backup != "" {
			_ = os.Remove(backup)
		}
		if oldPath != "" && s.ownsBinary(oldPath) && !samePath(oldPath, dest) {
			if err := os.Remove(oldPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("remove previous plugin binary: %w", err)
			}
		}
		return nil
	})
}

// parkBinary moves an existing file at dest to a unique backup name in the
// plugins directory and returns that name, or "" when dest does not exist.
func (s *Store) parkBinary(dest, name string) (string, error) {
	if _, err := os.Lstat(dest); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("stat plugin: %w", err)
	}
	f, err := os.CreateTemp(s.PluginsDir(), name+"-bak-")
	if err != nil {
		return "", fmt.Errorf("create backup: %w", err)
	}
	backup := f.Name()
	_ = f.Close()
	if err := os.Rename(dest, backup); err != nil {
		_ = os.Remove(backup)
		return "", fmt.Errorf("back up previous plugin binary: %w", err)
	}
	return backup, nil
}

// restoreBinary undoes parkBinary, replacing whatever is at dest.
func restoreBinary(backup, dest string) {
	if backup == "" {
		_ = os.Remove(dest)
		return
	}
	_ = os.Rename(backup, dest)
}

// canonicalPath returns an absolute path with symlinks resolved as far as the
// path exists (the final components may not exist yet).
func canonicalPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		abs = filepath.Clean(p)
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(abs)); err == nil {
		return filepath.Join(resolved, filepath.Base(abs))
	}
	return abs
}

func samePath(a, b string) bool {
	return canonicalPath(a) == canonicalPath(b)
}

// ownsBinary reports whether path is a file directly inside this store's
// plugins directory (by its own location, not its symlink target).
func (s *Store) ownsBinary(path string) bool {
	// Do not follow the final component: it may be a symlink that lives
	// outside the store but points into it.
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	return canonicalPath(filepath.Dir(abs)) == canonicalPath(s.PluginsDir())
}

// downloadIdleTimeout aborts a download whose body makes no progress for this long.
var downloadIdleTimeout = 30 * time.Second

// downloadTransport is shared by all downloads so keep-alive connections are
// reused instead of leaking one idle pool per install.
var downloadTransport = &http.Transport{
	Proxy:                 http.ProxyFromEnvironment,
	DialContext:           (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
	TLSHandshakeTimeout:   10 * time.Second,
	ResponseHeaderTimeout: 20 * time.Second,
	IdleConnTimeout:       30 * time.Second,
	MaxIdleConnsPerHost:   2,
}

// maxDownloadBytes caps the size of a downloaded plugin.
const maxDownloadBytes = 256 << 20

// newDownloadClient returns an HTTP client with dial/header timeouts and no
// whole-body timeout; body stalls are caught by idleReader.
func newDownloadClient() *http.Client {
	return &http.Client{Transport: downloadTransport}
}

// idleReader cancels the request when no bytes arrive within the timeout.
type idleReader struct {
	r     io.Reader
	timer *time.Timer
	d     time.Duration
}

func newIdleReader(r io.Reader, d time.Duration, cancel context.CancelFunc) *idleReader {
	return &idleReader{r: r, d: d, timer: time.AfterFunc(d, cancel)}
}

func (i *idleReader) Read(p []byte) (int, error) {
	n, err := i.r.Read(p)
	i.timer.Reset(i.d)
	return n, err
}

func (i *idleReader) stop() { i.timer.Stop() }

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
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		// Cover the header phase too; the idle reader takes over for the body.
		hdrTimer := time.AfterFunc(downloadIdleTimeout, cancel)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return Plugin{}, fmt.Errorf("build request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return Plugin{}, fmt.Errorf("download plugin: %w", err)
		}
		hdrTimer.Stop()
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return Plugin{}, fmt.Errorf("download plugin: status %s", resp.Status)
		}
		body := newIdleReader(resp.Body, downloadIdleTimeout, cancel)
		defer body.stop()
		if err := copyLimited(writer, body, limit); err != nil {
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

	cleaned = true // commitInstall owns the staged file from here on
	if err := s.commitInstall(tmp.Name(), plugin); err != nil {
		_ = os.Remove(tmp.Name())
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
