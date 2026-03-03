package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"ssh-deploy/internal/config"
)

type Runner struct {
	config config.Config
}

func NewRunner(cfg config.Config) Runner {
	return Runner{config: cfg}
}

func (runner Runner) Execute(ctx context.Context, request Request, stdout io.Writer, stderr io.Writer) error {
	switch request.Action {
	case ActionDeploy:
		if err := runner.runCompose(ctx, stdout, stderr, "pull"); err != nil {
			return fmt.Errorf("docker compose pull failed: %w", err)
		}
		if err := runner.runCompose(ctx, stdout, stderr, "up", "-d"); err != nil {
			return fmt.Errorf("docker compose up failed: %w", err)
		}
		return nil
	case ActionDestroy:
		if err := runner.runCompose(ctx, stdout, stderr, "down"); err != nil {
			return fmt.Errorf("docker compose down failed: %w", err)
		}
		return nil
	case ActionPS:
		if err := runner.runCompose(ctx, stdout, stderr, "ps"); err != nil {
			return fmt.Errorf("docker compose ps failed: %w", err)
		}
		return nil
	case ActionLogs:
		if err := runner.runCompose(ctx, stdout, stderr, "logs", "--tail", strconv.Itoa(runner.config.LogsTail), request.Service); err != nil {
			return fmt.Errorf("docker compose logs failed: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported action: %s", request.Action)
	}
}

func (runner Runner) runCompose(ctx context.Context, stdout io.Writer, stderr io.Writer, composeArgs ...string) error {
	args := []string{"compose"}
	if runner.config.ComposeFile != "" {
		args = append(args, "-f", runner.config.ComposeFile)
	}
	args = append(args, composeArgs...)

	command := exec.CommandContext(ctx, "docker", args...)
	command.Dir = runner.config.ComposeProjectDir
	command.Env = sanitizeComposeEnv(os.Environ())
	command.Stdout = stdout
	command.Stderr = stderr

	return command.Run()
}

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
