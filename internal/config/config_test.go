package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadFromEnvDefaults(t *testing.T) {
	t.Setenv("SSH_LISTEN_HOST", "")
	t.Setenv("SSH_LISTEN_PORT", "")
	t.Setenv("SSH_AUTHORIZED_KEYS_PATH", "")
	t.Setenv("SSH_HOST_KEY_PATH", "")
	t.Setenv("DEPLOY_COMPOSE_PROJECT_DIR", "")
	t.Setenv("COMPOSE_PROJECT_DIR", "")
	t.Setenv("DEPLOY_COMPOSE_FILE", "")
	t.Setenv("COMPOSE_FILE", "")
	t.Setenv("ALLOWED_LOG_SERVICES", "")
	t.Setenv("LOGS_TAIL", "")
	t.Setenv("COMMAND_TIMEOUT", "")
	t.Setenv("SSH_IDLE_TIMEOUT", "")
	t.Setenv("SSH_MAX_TIMEOUT", "")
	t.Setenv("KOMODO_ADDRESS", "")
	t.Setenv("KOMODO_API_KEY", "")
	t.Setenv("KOMODO_API_SECRET", "")
	t.Setenv("KOMODO_STACK", "")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}

	if cfg.ListenHost != "0.0.0.0" || cfg.ListenPort != "2222" {
		t.Fatalf("unexpected listen defaults: %s:%s", cfg.ListenHost, cfg.ListenPort)
	}
	if cfg.ComposeProjectDir != "/srv/target" {
		t.Fatalf("unexpected compose dir default: %q", cfg.ComposeProjectDir)
	}
	if cfg.LogsTail != 300 {
		t.Fatalf("unexpected logs tail default: %d", cfg.LogsTail)
	}
	if cfg.KomodoEnabled {
		t.Fatalf("expected komodo disabled by default")
	}
}

func TestLoadFromEnvKomodoEnabledAndOverrides(t *testing.T) {
	t.Setenv("SSH_LISTEN_HOST", "127.0.0.1")
	t.Setenv("SSH_LISTEN_PORT", "2022")
	t.Setenv("SSH_AUTHORIZED_KEYS_PATH", "/tmp/authorized_keys")
	t.Setenv("SSH_HOST_KEY_PATH", "/tmp/host_key")
	t.Setenv("DEPLOY_COMPOSE_PROJECT_DIR", "/srv/app")
	t.Setenv("DEPLOY_COMPOSE_FILE", "compose.yaml")
	t.Setenv("ALLOWED_LOG_SERVICES", "api, web")
	t.Setenv("LOGS_TAIL", "50")
	t.Setenv("COMMAND_TIMEOUT", "5m")
	t.Setenv("SSH_IDLE_TIMEOUT", "1m")
	t.Setenv("SSH_MAX_TIMEOUT", "20m")
	t.Setenv("KOMODO_ADDRESS", "https://komodo.example")
	t.Setenv("KOMODO_API_KEY", "abc")
	t.Setenv("KOMODO_API_SECRET", "def")
	t.Setenv("KOMODO_STACK", "polyphony")
	t.Setenv("KOMODO_POLL_TIMEOUT", "2m")
	t.Setenv("KOMODO_POLL_INTERVAL", "2s")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv returned error: %v", err)
	}

	if !cfg.KomodoEnabled {
		t.Fatalf("expected komodo enabled")
	}
	if cfg.KomodoStack != "polyphony" {
		t.Fatalf("unexpected komodo stack: %q", cfg.KomodoStack)
	}
	if cfg.KomodoPollTimeout != 2*time.Minute {
		t.Fatalf("unexpected poll timeout: %v", cfg.KomodoPollTimeout)
	}
	if cfg.KomodoPollInterval != 2*time.Second {
		t.Fatalf("unexpected poll interval: %v", cfg.KomodoPollInterval)
	}
	if _, ok := cfg.AllowedLogServices["api"]; !ok {
		t.Fatalf("expected api in allowed log services")
	}
	if _, ok := cfg.AllowedLogServices["web"]; !ok {
		t.Fatalf("expected web in allowed log services")
	}
}

func TestLoadFromEnvErrorsOnInvalidNumericAndDurations(t *testing.T) {
	t.Run("invalid logs tail", func(t *testing.T) {
		t.Setenv("LOGS_TAIL", "0")
		_, err := LoadFromEnv()
		if err == nil || !strings.Contains(err.Error(), "LOGS_TAIL") {
			t.Fatalf("expected LOGS_TAIL error, got: %v", err)
		}
	})

	t.Run("invalid timeout", func(t *testing.T) {
		t.Setenv("LOGS_TAIL", "10")
		t.Setenv("COMMAND_TIMEOUT", "nonsense")
		_, err := LoadFromEnv()
		if err == nil || !strings.Contains(err.Error(), "COMMAND_TIMEOUT") {
			t.Fatalf("expected COMMAND_TIMEOUT error, got: %v", err)
		}
	})
}

func TestHelpers(t *testing.T) {
	if got := firstNonEmpty("", "  ", "value", "other"); got != "value" {
		t.Fatalf("firstNonEmpty got %q", got)
	}

	set := parseCSVSet("api, web, ,db")
	if len(set) != 3 {
		t.Fatalf("parseCSVSet expected 3 entries, got %d", len(set))
	}

	if _, err := parsePositiveInt("-1", "TEST_INT"); err == nil {
		t.Fatalf("expected parsePositiveInt error")
	}

	if _, err := parseDuration("bad", "TEST_DUR"); err == nil {
		t.Fatalf("expected parseDuration error")
	}
}
