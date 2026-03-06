package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ssh-deploy/internal/config"
)

func TestNewRunnerSelectsBackend(t *testing.T) {
	t.Parallel()

	composeRunner := NewRunner(config.Config{KomodoEnabled: false})
	if _, ok := composeRunner.backend.(*ComposeBackend); !ok {
		t.Fatalf("expected ComposeBackend, got %T", composeRunner.backend)
	}

	komodoRunner := NewRunner(config.Config{
		KomodoEnabled:   true,
		KomodoAddress:   "http://example.test",
		KomodoAPIKey:    "key",
		KomodoAPISecret: "secret",
		KomodoStack:     "stack",
	})
	if _, ok := komodoRunner.backend.(*KomodoBackend); !ok {
		t.Fatalf("expected KomodoBackend, got %T", komodoRunner.backend)
	}
}

func TestSanitizeComposeEnv(t *testing.T) {
	t.Parallel()

	in := []string{"A=1", "COMPOSE_FILE=/tmp/compose.yml", "B=2"}
	out := sanitizeComposeEnv(in)

	if len(out) != 2 {
		t.Fatalf("expected 2 env entries, got %d", len(out))
	}
	if strings.Join(out, ",") != "A=1,B=2" {
		t.Fatalf("unexpected sanitized env: %v", out)
	}
}

func TestKomodoBackendDeployCallsPullAndDeploy(t *testing.T) {
	t.Parallel()

	called := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		if r.URL.Path != "/execute" {
			t.Fatalf("expected /execute, got %s", r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}

		typ, _ := payload["type"].(string)
		called = append(called, typ)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": typ + "-id", "status": "created"})
	}))
	defer server.Close()

	cfg := config.Config{
		KomodoEnabled:      true,
		KomodoAddress:      server.URL,
		KomodoAPIKey:       "key",
		KomodoAPISecret:    "secret",
		KomodoStack:        "polyphony",
		KomodoPollTimeout:  100 * time.Millisecond,
		KomodoPollInterval: 1 * time.Millisecond,
	}
	backend := NewKomodoBackend(cfg)

	var stdout, stderr bytes.Buffer
	if err := backend.Deploy(context.Background(), &stdout, &stderr); err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	if len(called) != 2 || called[0] != "PullStack" || called[1] != "DeployStack" {
		t.Fatalf("unexpected execute sequence: %v", called)
	}
	if !strings.Contains(stdout.String(), "Deployed polyphony") {
		t.Fatalf("unexpected deploy output: %q", stdout.String())
	}
}

func TestKomodoBackendPSFormatsServiceState(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		if r.URL.Path != "/read" {
			t.Fatalf("expected /read, got %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"service": "alpine",
				"container": map[string]any{
					"state":  "running",
					"status": "Up 3 minutes",
				},
			},
		})
	}))
	defer server.Close()

	cfg := config.Config{
		KomodoEnabled:   true,
		KomodoAddress:   server.URL,
		KomodoAPIKey:    "key",
		KomodoAPISecret: "secret",
		KomodoStack:     "polyphony",
	}
	backend := NewKomodoBackend(cfg)

	var stdout, stderr bytes.Buffer
	if err := backend.PS(context.Background(), &stdout, &stderr); err != nil {
		t.Fatalf("ps failed: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "SERVICE\tSTATE\tSTATUS") {
		t.Fatalf("missing table header, got: %q", out)
	}
	if !strings.Contains(out, "alpine\trunning\tUp 3 minutes") {
		t.Fatalf("missing service row, got: %q", out)
	}
}

func TestKomodoBackendLogsWritesStdoutAndStderr(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		if r.URL.Path != "/read" {
			t.Fatalf("expected /read, got %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"stdout":  "out-line\n",
			"stderr":  "err-line\n",
			"success": true,
		})
	}))
	defer server.Close()

	cfg := config.Config{
		KomodoEnabled:   true,
		KomodoAddress:   server.URL,
		KomodoAPIKey:    "key",
		KomodoAPISecret: "secret",
		KomodoStack:     "polyphony",
		LogsTail:        50,
	}
	backend := NewKomodoBackend(cfg)

	var stdout, stderr bytes.Buffer
	if err := backend.Logs(context.Background(), "alpine", &stdout, &stderr); err != nil {
		t.Fatalf("logs failed: %v", err)
	}

	if stdout.String() != "out-line\n" {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if stderr.String() != "err-line\n" {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestKomodoBackendLogsReturnsErrorWhenUnsuccessful(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"stage":   "get stack log",
			"success": false,
		})
	}))
	defer server.Close()

	cfg := config.Config{
		KomodoEnabled:   true,
		KomodoAddress:   server.URL,
		KomodoAPIKey:    "key",
		KomodoAPISecret: "secret",
		KomodoStack:     "polyphony",
		LogsTail:        50,
	}
	backend := NewKomodoBackend(cfg)

	err := backend.Logs(context.Background(), "alpine", &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "komodo log request failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
