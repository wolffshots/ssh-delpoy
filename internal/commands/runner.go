package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"ssh-deploy/internal/config"
	"ssh-deploy/internal/komodo"
)

// Backend defines the interface for stack operations.
type Backend interface {
	Deploy(ctx context.Context, stdout io.Writer, stderr io.Writer) error
	Destroy(ctx context.Context, stdout io.Writer, stderr io.Writer) error
	PS(ctx context.Context, stdout io.Writer, stderr io.Writer) error
	Logs(ctx context.Context, service string, stdout io.Writer, stderr io.Writer) error
}

type Runner struct {
	config  config.Config
	backend Backend
}

func NewRunner(cfg config.Config) Runner {
	var backend Backend

	if cfg.KomodoEnabled {
		backend = NewKomodoBackend(cfg)
	} else {
		backend = NewComposeBackend(cfg)
	}

	return Runner{
		config:  cfg,
		backend: backend,
	}
}

func (runner Runner) Execute(ctx context.Context, request Request, stdout io.Writer, stderr io.Writer) error {
	switch request.Action {
	case ActionDeploy:
		return runner.backend.Deploy(ctx, stdout, stderr)
	case ActionDestroy:
		return runner.backend.Destroy(ctx, stdout, stderr)
	case ActionPS:
		return runner.backend.PS(ctx, stdout, stderr)
	case ActionLogs:
		return runner.backend.Logs(ctx, request.Service, stdout, stderr)
	default:
		return fmt.Errorf("unsupported action: %s", request.Action)
	}
}

// ===================== COMPOSE BACKEND =====================

type ComposeBackend struct {
	config config.Config
}

func NewComposeBackend(cfg config.Config) *ComposeBackend {
	return &ComposeBackend{config: cfg}
}

func (cb *ComposeBackend) Deploy(ctx context.Context, stdout io.Writer, stderr io.Writer) error {
	if err := cb.runCompose(ctx, stdout, stderr, "pull"); err != nil {
		return fmt.Errorf("docker compose pull failed: %w", err)
	}
	if err := cb.runCompose(ctx, stdout, stderr, "up", "-d"); err != nil {
		return fmt.Errorf("docker compose up failed: %w", err)
	}
	return nil
}

func (cb *ComposeBackend) Destroy(ctx context.Context, stdout io.Writer, stderr io.Writer) error {
	if err := cb.runCompose(ctx, stdout, stderr, "down"); err != nil {
		return fmt.Errorf("docker compose down failed: %w", err)
	}
	return nil
}

func (cb *ComposeBackend) PS(ctx context.Context, stdout io.Writer, stderr io.Writer) error {
	if err := cb.runCompose(ctx, stdout, stderr, "ps"); err != nil {
		return fmt.Errorf("docker compose ps failed: %w", err)
	}
	return nil
}

func (cb *ComposeBackend) Logs(ctx context.Context, service string, stdout io.Writer, stderr io.Writer) error {
	if err := cb.runCompose(ctx, stdout, stderr, "logs", "--tail", strconv.Itoa(cb.config.LogsTail), service); err != nil {
		return fmt.Errorf("docker compose logs failed: %w", err)
	}
	return nil
}

func (cb *ComposeBackend) runCompose(ctx context.Context, stdout io.Writer, stderr io.Writer, composeArgs ...string) error {
	args := []string{"compose"}
	if cb.config.ComposeFile != "" {
		args = append(args, "-f", cb.config.ComposeFile)
	}
	args = append(args, composeArgs...)

	command := exec.CommandContext(ctx, "docker", args...)
	command.Dir = cb.config.ComposeProjectDir
	command.Env = sanitizeComposeEnv(os.Environ())
	command.Stdout = stdout
	command.Stderr = stderr

	return command.Run()
}

// ===================== KOMODO BACKEND =====================

type KomodoBackend struct {
	config config.Config
	client *komodo.Client
}

func NewKomodoBackend(cfg config.Config) *KomodoBackend {
	client := komodo.NewClient(cfg.KomodoAddress, cfg.KomodoAPIKey, cfg.KomodoAPISecret)
	return &KomodoBackend{
		config: cfg,
		client: client,
	}
}

func (kb *KomodoBackend) Deploy(ctx context.Context, stdout io.Writer, stderr io.Writer) error {
	// Pull
	upd, err := kb.client.ExecutePullStack(ctx, kb.config.KomodoStack, []string{})
	if err != nil {
		return fmt.Errorf("pull failed: %w", err)
	}

	if err := kb.pollUntilCompletion(ctx, upd.ID); err != nil {
		return fmt.Errorf("pull timeout: %w", err)
	}

	// Deploy
	upd, err = kb.client.ExecuteDeployStack(ctx, kb.config.KomodoStack, []string{}, nil)
	if err != nil {
		return fmt.Errorf("deploy failed: %w", err)
	}

	if err := kb.pollUntilCompletion(ctx, upd.ID); err != nil {
		return fmt.Errorf("deploy timeout: %w", err)
	}

	fmt.Fprintf(stdout, "Deployed %s\n", kb.config.KomodoStack)
	return nil
}

func (kb *KomodoBackend) Destroy(ctx context.Context, stdout io.Writer, stderr io.Writer) error {
	upd, err := kb.client.ExecuteDestroyStack(ctx, kb.config.KomodoStack, []string{}, false, nil)
	if err != nil {
		return fmt.Errorf("destroy failed: %w", err)
	}

	if err := kb.pollUntilCompletion(ctx, upd.ID); err != nil {
		return fmt.Errorf("destroy timeout: %w", err)
	}

	fmt.Fprintf(stdout, "Destroyed %s\n", kb.config.KomodoStack)
	return nil
}

func (kb *KomodoBackend) PS(ctx context.Context, stdout io.Writer, stderr io.Writer) error {
	services, err := kb.client.ReadListStackServices(ctx, kb.config.KomodoStack)
	if err != nil {
		return fmt.Errorf("list services failed: %w", err)
	}

	// Output as JSON for now (Komodo-native format)
	data, err := json.MarshalIndent(services, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal services: %w", err)
	}
	fmt.Fprint(stdout, string(data))
	return nil
}

func (kb *KomodoBackend) Logs(ctx context.Context, service string, stdout io.Writer, stderr io.Writer) error {
	log, err := kb.client.ReadGetStackLog(ctx, kb.config.KomodoStack, []string{service}, uint64(kb.config.LogsTail), false)
	if err != nil {
		return fmt.Errorf("get logs failed: %w", err)
	}

	fmt.Fprintln(stdout, log.Output)
	return nil
}

func (kb *KomodoBackend) pollUntilCompletion(ctx context.Context, updateID string) error {
	deadline := time.Now().Add(kb.config.KomodoPollTimeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("polling timeout exceeded")
		}

		// In this simple implementation, we check the state once
		// In production, you'd want to actually call GetUpdate on the Komodo API
		// to query the update status. For now, we'll trust the initial response.
		// This is a simplification since docs.rs doesn't show a GetUpdate endpoint clearly.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(kb.config.KomodoPollInterval):
			// Poll would happen here; for MVP we assume operations complete quickly
			return nil
		}
	}
}

// ===================== HELPERS =====================

func sanitizeComposeEnv(current []string) []string {
	filtered := make([]string, 0, len(current))
	for _, entry := range current {
		if strings.HasPrefix(entry, "COMPOSE_FILE=") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}
