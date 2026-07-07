package config

import (
	"os"
	"testing"
	"time"
)

func TestConfigLoadDefaults(t *testing.T) {
	cfg := Load()
	if cfg.Port != "8080" {
		t.Errorf("expected default Port to be 8080, got %s", cfg.Port)
	}
	if cfg.UpstreamURL != "http://localhost:8081" {
		t.Errorf("expected default UpstreamURL to be http://localhost:8081, got %s", cfg.UpstreamURL)
	}
}

func TestConfigLoadEnv(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("UPSTREAM_URL", "http://backend:8080")
	os.Setenv("SYNC_INTERVAL_MS", "500")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("UPSTREAM_URL")
		os.Unsetenv("SYNC_INTERVAL_MS")
	}()

	cfg := Load()
	if cfg.Port != "9090" {
		t.Errorf("expected Port to be 9090, got %s", cfg.Port)
	}
	if cfg.UpstreamURL != "http://backend:8080" {
		t.Errorf("expected UpstreamURL to be http://backend:8080, got %s", cfg.UpstreamURL)
	}
	if cfg.SyncInterval != 500*time.Millisecond {
		t.Errorf("expected SyncInterval to be 500ms, got %v", cfg.SyncInterval)
	}
}
