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
schedule_enabled: true
schedule_time: "04:30"
request_interval: 3s
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
	if cfg.ScheduleTime != "04:30" {
		t.Fatalf("expected schedule time 04:30, got %q", cfg.ScheduleTime)
	}
	if cfg.RequestInterval.String() != "3s" {
		t.Fatalf("expected request interval 3s, got %s", cfg.RequestInterval)
	}
}

func TestLoadRejectsInvalidScheduleTime(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	content := []byte(`postgres_dsn: postgres://rigel:rigel@localhost:5432/rigel?sslmode=disable
schedule_time: "25:00"
`)
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(configPath); err == nil {
		t.Fatal("expected invalid schedule_time error")
	}
}
