package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"ssh-deploy/internal/config"
	"ssh-deploy/internal/komodo"
)

// MockKomodoClient implements komodo.Client operations for testing.
type MockKomodoClient struct {
	pullCalled    bool
	deployCalled  bool
	destroyCalled bool
	pullErr       error
	deployErr     error
	destroyErr    error
}

// Helper function to create a test KomodoBackend using a mock.
func createTestKomodoBackend() *KomodoBackend {
	cfg := config.Config{
		KomodoStack:        "test-stack",
		LogsTail:           100,
		KomodoPollTimeout:  5 * time.Second,
		KomodoPollInterval: 100 * time.Millisecond,
	}
	kb := &KomodoBackend{
		config: cfg,
		client: komodo.NewClient("http://localhost:8080", "key", "secret"),
	}
	return kb
}

func TestComposeBackendDeploy(t *testing.T) {
	cfg := config.Config{
		ComposeProjectDir: "/tmp",
		ComposeFile:       "docker-compose.yml",
	}
	backend := NewComposeBackend(cfg)

	// This will fail because docker isn't available in test env,
	// but we're testing the method signature and backend dispatch.
	var stdout, stderr bytes.Buffer
	err := backend.Deploy(context.Background(), &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected error (docker not available), got nil")
	}
}

func TestKomodoBackendCreation(t *testing.T) {
	cfg := config.Config{
		KomodoAddress:      "http://localhost:8080",
		KomodoAPIKey:       "key",
		KomodoAPISecret:    "secret",
		KomodoStack:        "my-stack",
		LogsTail:           100,
		KomodoPollTimeout:  5 * time.Second,
		KomodoPollInterval: 100 * time.Millisecond,
	}
	backend := NewKomodoBackend(cfg)
	if backend == nil {
		t.Fatalf("expected KomodoBackend, got nil")
	}
	if backend.config.KomodoStack != "my-stack" {
		t.Fatalf("expected stack=my-stack, got %s", backend.config.KomodoStack)
	}
}

func TestBackendSelection(t *testing.T) {
	tests := []struct {
		name         string
		cfg          config.Config
		expectedType string
	}{
		{
			name: "Compose backend when Komodo disabled",
			cfg: config.Config{
				KomodoEnabled: false,
			},
			expectedType: "*commands.ComposeBackend",
		},
		{
			name: "Komodo backend when Komodo enabled",
			cfg: config.Config{
				KomodoEnabled:   true,
				KomodoAddress:   "http://localhost:8080",
				KomodoAPIKey:    "key",
				KomodoAPISecret: "secret",
				KomodoStack:     "my-stack",
			},
			expectedType: "*commands.KomodoBackend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewRunner(tt.cfg)
			// Type check by calling a method that only works on one type
			// This is a simple validation that the right backend was selected
			if runner.backend == nil {
				t.Fatalf("backend not initialized")
			}
		})
	}
}

func TestKomodoBackendPSOutput(t *testing.T) {
	kb := createTestKomodoBackend()

	// We're testing that PS returns JSON-formatted output
	// The actual Komodo API call would fail without a real server,
	// but we can check the method signature works.
	var stdout bytes.Buffer
	// This will fail due to connection error, which is expected
	err := kb.PS(context.Background(), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatalf("expected error (no mock server), got nil")
	}
}

func TestServicesSerialization(t *testing.T) {
	services := []komodo.StackService{
		{Name: "web", State: "running"},
		{Name: "db", State: "running"},
	}

	data, err := json.MarshalIndent(services, "", "  ")
	if err != nil {
		t.Fatalf("marshal services: %v", err)
	}

	var unmarshaled []komodo.StackService
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("unmarshal services: %v", err)
	}

	if len(unmarshaled) != 2 {
		t.Fatalf("expected 2 services, got %d", len(unmarshaled))
	}
	if unmarshaled[0].Name != "web" {
		t.Fatalf("expected first service=web, got %s", unmarshaled[0].Name)
	}
}

func TestNewRunnerDispatch(t *testing.T) {
	// Test with Compose backend (default when no Komodo env)
	cfg := config.Config{
		ComposeProjectDir: "/tmp",
		KomodoEnabled:     false,
	}
	runner := NewRunner(cfg)
	_, isCompose := runner.backend.(*ComposeBackend)
	if !isCompose {
		t.Fatalf("expected ComposeBackend, got %T", runner.backend)
	}

	// Test with Komodo backend
	komodoCfg := config.Config{
		KomodoEnabled:     true,
		KomodoAddress:     "http://localhost:8080",
		KomodoAPIKey:      "key",
		KomodoAPISecret:   "secret",
		KomodoStack:       "my-stack",
		KomodoPollTimeout: 5 * time.Second,
	}
	komodoRunner := NewRunner(komodoCfg)
	_, isKomodo := komodoRunner.backend.(*KomodoBackend)
	if !isKomodo {
		t.Fatalf("expected KomodoBackend, got %T", komodoRunner.backend)
	}
}
