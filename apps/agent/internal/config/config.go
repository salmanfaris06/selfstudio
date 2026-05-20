package config

import (
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultHost         = "127.0.0.1"
	DefaultPort         = 8080
	DefaultLocalDataDir = "./local-data"
	placeholderAuthPIN  = "change-this-local-pin"
)

type Config struct {
	Host                    string
	Port                    int
	LocalDataDir            string
	AuthPIN                 string
	CameraReadinessRequired bool
}

func Load() (Config, error) {
	port, err := parsePort(envOrDefault("SELFSTUDIO_AGENT_PORT", strconv.Itoa(DefaultPort)))
	if err != nil {
		return Config{}, err
	}

	authPIN, err := requiredEnv("SELFSTUDIO_AUTH_PIN")
	if err != nil {
		return Config{}, err
	}

	return Config{
		Host:                    envOrDefault("SELFSTUDIO_AGENT_HOST", DefaultHost),
		Port:                    port,
		LocalDataDir:            envOrDefault("SELFSTUDIO_LOCAL_DATA_DIR", DefaultLocalDataDir),
		AuthPIN:                 authPIN,
		CameraReadinessRequired: parseBoolEnv("SELFSTUDIO_CAMERA_READINESS_REQUIRED"),
	}, nil
}

func (c Config) Address() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

func parsePort(value string) (int, error) {
	port, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || port <= 0 || port > 65535 {
		return 0, &PortError{}
	}

	return port, nil
}

type PortError struct{}

func (e *PortError) Error() string {
	return "SELFSTUDIO_AGENT_PORT must be a valid TCP port between 1 and 65535"
}

func envOrDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func parseBoolEnv(key string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func requiredEnv(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", &MissingEnvError{Key: key}
	}
	if key == "SELFSTUDIO_AUTH_PIN" && value == placeholderAuthPIN {
		return "", &PlaceholderEnvError{Key: key}
	}

	return value, nil
}

type MissingEnvError struct {
	Key string
}

func (e *MissingEnvError) Error() string {
	return e.Key + " is required. Set it in your local environment or .env file before starting Selfstudio."
}

type PlaceholderEnvError struct {
	Key string
}

func (e *PlaceholderEnvError) Error() string {
	return e.Key + " still uses the example placeholder. Replace it with a local PIN/password before starting Selfstudio."
}
