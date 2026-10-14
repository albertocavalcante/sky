package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInstallFromMarketplace_RequiresChecksum(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	writeTestFile(t, bin, "x")
	idx := filepath.Join(dir, "idx.json")
	writeTestFile(t, idx, `{"plugins":[{"name":"foo","url":"`+bin+`"}]}`)
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
	writeTestFile(t, idx, `{"plugins":[{"name":"foo","url":"`+bin+`","sha256":"`+hex.EncodeToString(sum[:])+`"}]}`)
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
	writeTestFile(t, idx, `{"plugins":[{"name":"foo","url":"u","type":"bogus"}]}`)
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
	writeTestFile(t, good, `{"plugins":[{"name":"foo","url":"u"}]}`)
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
