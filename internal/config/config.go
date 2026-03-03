package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenHost         string
	ListenPort         string
	AuthorizedKeysPath string
	HostKeyPath        string
	ComposeProjectDir  string
	ComposeFile        string
	AllowedLogServices map[string]struct{}
	LogsTail           int
	CommandTimeout     time.Duration
	IdleTimeout        time.Duration
	MaxTimeout         time.Duration
	// Komodo backend configuration
	KomodoEnabled      bool
	KomodoAddress      string
	KomodoAPIKey       string
	KomodoAPISecret    string
	KomodoStack        string
	KomodoPollTimeout  time.Duration
	KomodoPollInterval time.Duration
}

func (cfg Config) Address() string {
	return net.JoinHostPort(cfg.ListenHost, cfg.ListenPort)
}

func LoadFromEnv() (Config, error) {
	logsTail, err := parsePositiveInt(getEnv("LOGS_TAIL", "300"), "LOGS_TAIL")
	if err != nil {
		return Config{}, err
	}

	commandTimeout, err := parseDuration(getEnv("COMMAND_TIMEOUT", "10m"), "COMMAND_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	idleTimeout, err := parseDuration(getEnv("SSH_IDLE_TIMEOUT", "2m"), "SSH_IDLE_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	maxTimeout, err := parseDuration(getEnv("SSH_MAX_TIMEOUT", "15m"), "SSH_MAX_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	allowedServices := parseCSVSet(getEnv("ALLOWED_LOG_SERVICES", ""))

	// Komodo configuration (optional)
	komodoAddr := strings.TrimSpace(os.Getenv("KOMODO_ADDRESS"))
	komodoKey := strings.TrimSpace(os.Getenv("KOMODO_API_KEY"))
	komodoSecret := strings.TrimSpace(os.Getenv("KOMODO_API_SECRET"))
	komodoStack := strings.TrimSpace(os.Getenv("KOMODO_STACK"))
	komodoEnabled := komodoAddr != "" && komodoKey != "" && komodoSecret != "" && komodoStack != ""

	komodoPolltimeout := 5 * time.Minute // default
	if pollStr := getEnv("KOMODO_POLL_TIMEOUT", ""); pollStr != "" {
		if pt, err := parseDuration(pollStr, "KOMODO_POLL_TIMEOUT"); err == nil {
			komodoPolltimeout = pt
		}
	}

	komodoPollInterval := 5 * time.Second // default
	if pollStr := getEnv("KOMODO_POLL_INTERVAL", ""); pollStr != "" {
		if pi, err := parseDuration(pollStr, "KOMODO_POLL_INTERVAL"); err == nil {
			komodoPollInterval = pi
		}
	}

	cfg := Config{
		ListenHost:         getEnv("SSH_LISTEN_HOST", "0.0.0.0"),
		ListenPort:         getEnv("SSH_LISTEN_PORT", "2222"),
		AuthorizedKeysPath: getEnv("SSH_AUTHORIZED_KEYS_PATH", "/app/data/authorized_keys"),
		HostKeyPath:        getEnv("SSH_HOST_KEY_PATH", "/app/data/ssh_host_ed25519"),
		ComposeProjectDir:  getEnv("DEPLOY_COMPOSE_PROJECT_DIR", getEnv("COMPOSE_PROJECT_DIR", "/srv/target")),
		ComposeFile:        firstNonEmpty(strings.TrimSpace(os.Getenv("DEPLOY_COMPOSE_FILE")), strings.TrimSpace(os.Getenv("COMPOSE_FILE"))),
		AllowedLogServices: allowedServices,
		LogsTail:           logsTail,
		CommandTimeout:     commandTimeout,
		IdleTimeout:        idleTimeout,
		MaxTimeout:         maxTimeout,
		KomodoEnabled:      komodoEnabled,
		KomodoAddress:      komodoAddr,
		KomodoAPIKey:       komodoKey,
		KomodoAPISecret:    komodoSecret,
		KomodoStack:        komodoStack,
		KomodoPollTimeout:  komodoPolltimeout,
		KomodoPollInterval: komodoPollInterval,
	}

	if cfg.ListenPort == "" {
		return Config{}, fmt.Errorf("SSH_LISTEN_PORT cannot be empty")
	}
	if cfg.AuthorizedKeysPath == "" {
		return Config{}, fmt.Errorf("SSH_AUTHORIZED_KEYS_PATH cannot be empty")
	}
	if cfg.HostKeyPath == "" {
		return Config{}, fmt.Errorf("SSH_HOST_KEY_PATH cannot be empty")
	}
	if cfg.ComposeProjectDir == "" {
		return Config{}, fmt.Errorf("COMPOSE_PROJECT_DIR cannot be empty")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func parsePositiveInt(raw string, envName string) (int, error) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", envName)
	}
	return value, nil
}

func parseDuration(raw string, envName string) (time.Duration, error) {
	parsed, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration (example: 10m)", envName)
	}
	return parsed, nil
}

func parseCSVSet(raw string) map[string]struct{} {
	result := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}
		result[entry] = struct{}{}
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
