package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// marketplaceJSON serializes a marketplace index; raw string interpolation
// breaks on Windows paths, whose backslashes are invalid JSON escapes.
func marketplaceJSON(t *testing.T, entries ...map[string]any) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{"plugins": entries})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInstallFromMarketplace_BackslashPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("backslash is a path separator")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, `we\ird`)
	writeTestFile(t, bin, "x")
	sum := sha256.Sum256([]byte("x"))
	idx := filepath.Join(dir, "idx.json")
	writeTestFile(t, idx, marketplaceJSON(t, map[string]any{"name": "foo", "url": bin, "sha256": hex.EncodeToString(sum[:])}))
	s := NewStore(filepath.Join(dir, "root"))
	if err := s.UpsertMarketplace(Marketplace{Name: "m", URL: idx}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InstallFromMarketplace(context.Background(), "foo", "m"); err != nil {
		t.Fatal(err)
	}
}

func TestInstallFromMarketplace_RequiresChecksum(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	writeTestFile(t, bin, "x")
	idx := filepath.Join(dir, "idx.json")
	writeTestFile(t, idx, marketplaceJSON(t, map[string]any{"name": "foo", "url": bin}))
	s := NewStore(filepath.Join(dir, "root"))
	if err := s.UpsertMarketplace(Marketplace{Name: "m", URL: idx}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InstallFromMarketplace(context.Background(), "foo", "m"); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("want checksum error, got %v", err)
	}
	// direct installs stay optional
	if _, err := s.InstallFromURL(context.Background(), "foo", bin, "", "", "", ""); err != nil {
		t.Fatal(err)
	}
}

func TestInstallFromMarketplace_SourcePersisted(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	writeTestFile(t, bin, "x")
	sum := sha256.Sum256([]byte("x"))
	idx := filepath.Join(dir, "idx.json")
	writeTestFile(t, idx, marketplaceJSON(t, map[string]any{"name": "foo", "url": bin, "sha256": hex.EncodeToString(sum[:])}))
	s := NewStore(filepath.Join(dir, "root"))
	if err := s.UpsertMarketplace(Marketplace{Name: "m", URL: idx}); err != nil {
		t.Fatal(err)
	}
	p, err := s.InstallFromMarketplace(context.Background(), "foo", "m")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.FindPlugin("foo")
	if got == nil || got.Source != p.Source || !strings.HasPrefix(got.Source, "m (") {
		t.Fatalf("source not persisted: %+v", got)
	}
}

func TestCopyLimited(t *testing.T) {
	var sink strings.Builder
	if err := copyLimited(&sink, strings.NewReader("12345"), 5); err != nil {
		t.Fatalf("at limit: %v", err)
	}
	if err := copyLimited(&sink, strings.NewReader("123456"), 5); err == nil {
		t.Fatal("expected size error")
	}
}

func TestInstall_TooLarge(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	writeTestFile(t, bin, "123456")
	s := NewStore(filepath.Join(dir, "root"))
	_, err := s.installFromURL(context.Background(), "foo", bin, "", "", "", "", installOptions{limit: 5})
	if err == nil {
		t.Fatal("expected size error")
	}
	entries, _ := os.ReadDir(s.PluginsDir())
	if len(entries) != 0 {
		t.Fatalf("temp files left behind: %v", entries)
	}
}

func TestInstall_TypeChangeRemovesOldBinary(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	writeTestFile(t, bin, "x")
	s := NewStore(filepath.Join(dir, "root"))
	ctx := context.Background()
	if _, err := s.InstallFromURL(ctx, "foo", bin, "", "", "", TypeExecutable); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InstallFromURL(ctx, "foo", bin, "", "", "", TypeWasm); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.PluginPath("foo", TypeExecutable)); !os.IsNotExist(err) {
		t.Fatalf("old binary should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(s.PluginPath("foo", TypeWasm)); err != nil {
		t.Fatal(err)
	}
}

func TestInstall_InvalidType(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	writeTestFile(t, bin, "x")
	s := NewStore(filepath.Join(dir, "root"))
	if _, err := s.InstallFromURL(context.Background(), "foo", bin, "", "", "", "bogus"); err == nil {
		t.Fatal("expected invalid type error")
	}
}

func TestMarketplace_InvalidTypeRejected(t *testing.T) {
	dir := t.TempDir()
	idx := filepath.Join(dir, "idx.json")
	writeTestFile(t, idx, marketplaceJSON(t, map[string]any{"name": "foo", "url": "u", "type": "bogus"}))
	s := NewStore(filepath.Join(dir, "root"))
	if err := s.UpsertMarketplace(Marketplace{Name: "m", URL: idx}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.ResolveMarketplacePlugin(context.Background(), "foo", "m"); err == nil {
		t.Fatal("expected invalid type error")
	}
}

func TestMarketplace_ContinuesPastFetchErrors(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	writeTestFile(t, good, marketplaceJSON(t, map[string]any{"name": "foo", "url": "u"}))
	s := NewStore(filepath.Join(dir, "root"))
	for _, m := range []Marketplace{{Name: "bad", URL: filepath.Join(dir, "missing.json")}, {Name: "good", URL: good}} {
		if err := s.UpsertMarketplace(m); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	if res, err := s.SearchMarketplaces(ctx, "foo", ""); err != nil || len(res) != 1 {
		t.Fatalf("search: %v %v", res, err)
	}
	if _, _, err := s.ResolveMarketplacePlugin(ctx, "foo", ""); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	_, _, err := s.ResolveMarketplacePlugin(ctx, "nope", "")
	if err == nil || !strings.Contains(err.Error(), "missing.json") {
		t.Fatalf("want error mentioning fetch failure, got %v", err)
	}
}

func TestInstall_CleanupDoesNotDeleteOtherStore(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	writeTestFile(t, bin, "x")
	a := NewStore(filepath.Join(dir, "a"))
	if _, err := a.InstallFromURL(context.Background(), "foo", bin, "", "", "", TypeExecutable); err != nil {
		t.Fatal(err)
	}
	// Copy A's catalog into B, so it records A's binary path.
	b := NewStore(filepath.Join(dir, "b"))
	if err := b.Ensure(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(a.PluginsFile())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b.PluginsFile(), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := b.InstallFromURL(context.Background(), "foo", bin, "", "", "", TypeWasm); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.PluginPath("foo", TypeExecutable)); err != nil {
		t.Fatalf("store A binary deleted by store B install: %v", err)
	}
}

func TestInstall_PathAliasKeepsDest(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	bin := filepath.Join(dir, "bin")
	writeTestFile(t, bin, "x")
	rel := NewStore("root")
	if _, err := rel.InstallFromURL(context.Background(), "foo", bin, "", "", "", ""); err != nil {
		t.Fatal(err)
	}
	abs := NewStore(filepath.Join(dir, "root"))
	p, err := abs.InstallFromURL(context.Background(), "foo", bin, "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.Path); err != nil {
		t.Fatalf("freshly installed binary removed: %v", err)
	}
}

func TestInstallFromPath_RemovesPreviousBinaryOfOtherType(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	writeTestFile(t, bin, "x")
	s := NewStore(filepath.Join(dir, "root"))
	if _, err := s.InstallFromPath("foo", bin, "", TypeExecutable); err != nil {
		t.Fatal(err)
	}
	if _, err := s.InstallFromPath("foo", bin, "", TypeWasm); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.PluginPath("foo", TypeExecutable)); !os.IsNotExist(err) {
		t.Fatalf("previous binary should be removed, stat err=%v", err)
	}
	entries, _ := os.ReadDir(s.PluginsDir())
	for _, e := range entries {
		if strings.Contains(e.Name(), "-tmp-") {
			t.Fatalf("leftover temp %s", e.Name())
		}
	}
}

func TestInstallFromURL_StalledBodyTimesOut(t *testing.T) {
	old := downloadIdleTimeout
	downloadIdleTimeout = 200 * time.Millisecond
	t.Cleanup(func() { downloadIdleTimeout = old })

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })

	s := NewStore(filepath.Join(t.TempDir(), "root"))
	done := make(chan error, 1)
	go func() {
		_, err := s.InstallFromURL(context.Background(), "foo", srv.URL, "", "", "", "")
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error for stalled download")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("download hung on a stalled body")
	}
}

func TestInstallFromURL_ReusesConnections(t *testing.T) {
	var conns atomic.Int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("x"))
	}))
	srv.Config.ConnState = func(_ net.Conn, st http.ConnState) {
		if st == http.StateNew {
			conns.Add(1)
		}
	}
	srv.Start()
	t.Cleanup(srv.Close)

	s := NewStore(filepath.Join(t.TempDir(), "root"))
	for i := 0; i < 3; i++ {
		if _, err := s.InstallFromURL(context.Background(), "foo", srv.URL, "", "", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	if n := conns.Load(); n != 1 {
		t.Fatalf("want 1 reused connection, got %d", n)
	}
}

func TestUpdatePluginMetadata_SkipsReinstalledPlugin(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	writeTestFile(t, bin, "x")
	s := NewStore(filepath.Join(dir, "root"))
	old, err := s.InstallFromPath("foo", bin, "1", TypeExecutable)
	if err != nil {
		t.Fatal(err)
	}
	// A concurrent reinstall replaces the entry after it was inspected.
	time.Sleep(2 * time.Millisecond)
	if _, err := s.InstallFromPath("foo", bin, "2", TypeWasm); err != nil {
		t.Fatal(err)
	}
	ok, err := s.UpdatePluginMetadata("foo", old.InstalledAt, "9", "stale")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("stale metadata update must be skipped")
	}
	got, _ := s.FindPlugin("foo")
	if got == nil || got.Type != TypeWasm || got.Version != "2" {
		t.Fatalf("entry clobbered: %+v", got)
	}
}
