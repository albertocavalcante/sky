package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateName(t *testing.T) {
	cases := []struct {
		name string
		ok   bool
	}{
		{name: "skyfmt", ok: true},
		{name: "sky-fmt", ok: true},
		{name: "Sky", ok: false},
		{name: "sky_fmt", ok: false},
		{name: "-bad", ok: false},
		{name: "", ok: false},
	}

	for _, tc := range cases {
		err := ValidateName(tc.name)
		if tc.ok && err != nil {
			t.Fatalf("expected name %q to be valid: %v", tc.name, err)
		}
		if !tc.ok && err == nil {
			t.Fatalf("expected name %q to be invalid", tc.name)
		}
	}
}

func TestInstallAndRemovePlugin(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	src := filepath.Join(root, "plugin-bin")
	if err := os.WriteFile(src, []byte("demo"), 0o755); err != nil {
		t.Fatalf("write source: %v", err)
	}

	plugin, err := store.InstallFromPath("demo", src, "1.2.3", TypeExecutable)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if plugin.Path == "" {
		t.Fatalf("expected plugin path to be set")
	}
	if _, err := os.Stat(plugin.Path); err != nil {
		t.Fatalf("installed plugin missing: %v", err)
	}

	plugins, err := store.LoadPlugins()
	if err != nil {
		t.Fatalf("load plugins: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}

	removed, err := store.RemovePlugin("demo")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if removed.Name != "demo" {
		t.Fatalf("expected removed plugin demo, got %q", removed.Name)
	}
	if _, err := os.Stat(plugin.Path); !os.IsNotExist(err) {
		t.Fatalf("expected plugin binary removed, got %v", err)
	}
}

func TestMarketplaceUpsert(t *testing.T) {
	store := NewStore(t.TempDir())
	marketplace := Marketplace{
		Name: "local",
		URL:  "/tmp/market.json",
	}
	if err := store.UpsertMarketplace(marketplace); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	list, err := store.LoadMarketplaces()
	if err != nil {
		t.Fatalf("load marketplaces: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 marketplace, got %d", len(list))
	}
	if list[0].Name != "local" {
		t.Fatalf("expected marketplace name local, got %q", list[0].Name)
	}
}

func TestStoreConcurrency(t *testing.T) {
	root := t.TempDir()

	// Create a dummy plugin binary
	bin := filepath.Join(root, "p")
	if err := os.WriteFile(bin, []byte{}, 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	tests := []struct {
		name    string
		workers int
	}{
		{"SingleWorker", 1},
		{"LowConcurrency", 5},
		{"HighConcurrency", 20},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Isolate each test case in a sub-directory
			subRoot := filepath.Join(root, tc.name)
			subStore := NewStore(subRoot)

			errc := make(chan error, tc.workers)

			for i := 0; i < tc.workers; i++ {
				go func(id int) {
					name := fmt.Sprintf("p%d", id)
					_, err := subStore.InstallFromPath(name, bin, "1.0.0", TypeExecutable)
					errc <- err
				}(i)
			}

			for i := 0; i < tc.workers; i++ {
				if err := <-errc; err != nil {
					t.Errorf("worker failed: %v", err)
				}
			}

			plugins, err := subStore.LoadPlugins()
			if err != nil {
				t.Fatalf("load plugins: %v", err)
			}
			if len(plugins) != tc.workers {
				t.Fatalf("expected %d plugins, got %d", tc.workers, len(plugins))
			}
		})
	}
}

func TestStoreConcurrentUpsert(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root)

	workers := 20
	errc := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			err := store.UpsertPlugin(Plugin{
				Name:    "same-plugin",
				Version: fmt.Sprintf("1.0.%d", id),
			})
			errc <- err
		}(i)
	}

	for i := 0; i < workers; i++ {
		if err := <-errc; err != nil {
			t.Errorf("worker failed: %v", err)
		}
	}

	plugins, err := store.LoadPlugins()
	if err != nil {
		t.Fatalf("load plugins: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
}
