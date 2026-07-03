package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAppliesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thirdcupd.json")
	data := []byte(`{
  "checks": {
    "disk": [
      {
        "name": "root",
        "path": "/",
        "max_usage_percent": 90
      }
    ]
  }
}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if got := cfg.Daemon.Interval.Duration(); got != 30*time.Second {
		t.Fatalf("expected default interval 30s, got %s", got)
	}
	if cfg.Observability.ListenAddr != "127.0.0.1:8374" {
		t.Fatalf("unexpected listen addr: %s", cfg.Observability.ListenAddr)
	}
}

func TestLoadRejectsDuplicateNamesWithinType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thirdcupd.json")
	data := []byte(`{
  "checks": {
    "tcp": [
      {"name": "db", "address": "127.0.0.1:5432"},
      {"name": "db", "address": "127.0.0.1:5433"}
    ]
  }
}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected duplicate name error")
	}
}
