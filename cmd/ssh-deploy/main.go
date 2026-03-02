package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"

	"ssh-deploy/internal/commands"
	"ssh-deploy/internal/config"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	parser := commands.NewParser(cfg.AllowedLogServices)
	runner := commands.NewRunner(cfg)

	server, err := wish.NewServer(
		wish.WithAddress(cfg.Address()),
		wish.WithHostKeyPath(cfg.HostKeyPath),
		wish.WithAuthorizedKeys(cfg.AuthorizedKeysPath),
		wish.WithIdleTimeout(cfg.IdleTimeout),
		wish.WithMaxTimeout(cfg.MaxTimeout),
		wish.WithMiddleware(commandMiddleware(cfg, parser, runner)),
	)
	if err != nil {
		log.Fatalf("failed to create SSH server: %v", err)
	}

	log.Printf("ssh-deploy listening on %s", cfg.Address())

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)

	go func() {
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, ssh.ErrServerClosed) {
			log.Printf("server error: %v", serveErr)
			shutdownSignal <- syscall.SIGTERM
		}
	}()

	<-shutdownSignal
	log.Printf("shutdown requested")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Fatalf("failed to shut down server: %v", err)
	}
}

func handleSession(cfg config.Config, parser commands.Parser, runner commands.Runner, session ssh.Session) {
	request, err := parser.Parse(session.Command())
	if err != nil {
		_, _ = fmt.Fprintln(session.Stderr(), err.Error())
		_ = session.Exit(2)
		return
	}

	ctx, cancel := context.WithTimeout(session.Context(), cfg.CommandTimeout)
	defer cancel()

	if err := runner.Execute(ctx, request, session, session.Stderr()); err != nil {
		_, _ = fmt.Fprintln(session.Stderr(), err.Error())
		_ = session.Exit(1)
		return
	}

	_ = session.Exit(0)
}

func commandMiddleware(cfg config.Config, parser commands.Parser, runner commands.Runner) wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		_ = next
		return func(session ssh.Session) {
			handleSession(cfg, parser, runner, session)
		}
	}
}
