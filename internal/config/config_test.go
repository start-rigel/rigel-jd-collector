package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	content := []byte(`service_name: test-jd
http_port: "18081"
postgres_dsn: postgres://rigel:rigel@localhost:5432/rigel?sslmode=disable
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ServiceName != "test-jd" {
		t.Fatalf("expected service name test-jd, got %q", cfg.ServiceName)
	}
	if cfg.JDCollectorMode != "union" {
		t.Fatalf("expected union mode, got %q", cfg.JDCollectorMode)
	}
	if cfg.ReadTimeout.String() != "5s" {
		t.Fatalf("expected default read timeout 5s, got %s", cfg.ReadTimeout)
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	content := []byte(`postgres_dsn: postgres://rigel:rigel@localhost:5432/rigel?sslmode=disable
read_timeout: nope
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(configPath); err == nil {
		t.Fatal("expected invalid duration error")
	}
}
