package config

import "testing"

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("SELFSTUDIO_AGENT_HOST", "")
	t.Setenv("SELFSTUDIO_AGENT_PORT", "")
	t.Setenv("SELFSTUDIO_LOCAL_DATA_DIR", "")
	t.Setenv("SELFSTUDIO_AUTH_PIN", "123456")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Host != DefaultHost {
		t.Fatalf("Host = %q, want %q", cfg.Host, DefaultHost)
	}
	if cfg.Port != DefaultPort {
		t.Fatalf("Port = %d, want %d", cfg.Port, DefaultPort)
	}
	if cfg.LocalDataDir != DefaultLocalDataDir {
		t.Fatalf("LocalDataDir = %q, want %q", cfg.LocalDataDir, DefaultLocalDataDir)
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("SELFSTUDIO_AGENT_PORT", "not-a-port")
	t.Setenv("SELFSTUDIO_AUTH_PIN", "123456")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error for invalid port")
	}
}

func TestAddressUsesJoinHostPort(t *testing.T) {
	cfg := Config{Host: "::1", Port: 8080}

	if got, want := cfg.Address(), "[::1]:8080"; got != want {
		t.Fatalf("Address() = %q, want %q", got, want)
	}
}

func TestLoadTrimsWhitespace(t *testing.T) {
	t.Setenv("SELFSTUDIO_AGENT_HOST", " 127.0.0.1 ")
	t.Setenv("SELFSTUDIO_AGENT_PORT", " 8081 ")
	t.Setenv("SELFSTUDIO_LOCAL_DATA_DIR", " ./local-data ")
	t.Setenv("SELFSTUDIO_AUTH_PIN", " 123456 ")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Host != "127.0.0.1" {
		t.Fatalf("Host = %q, want 127.0.0.1", cfg.Host)
	}
	if cfg.Port != 8081 {
		t.Fatalf("Port = %d, want 8081", cfg.Port)
	}
	if cfg.LocalDataDir != "./local-data" {
		t.Fatalf("LocalDataDir = %q, want ./local-data", cfg.LocalDataDir)
	}
	if cfg.AuthPIN != "123456" {
		t.Fatalf("AuthPIN = %q, want trimmed configured value", cfg.AuthPIN)
	}
}

func TestLoadRequiresAuthPIN(t *testing.T) {
	t.Setenv("SELFSTUDIO_AUTH_PIN", "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error for missing auth PIN")
	}
}

func TestLoadRejectsWhitespaceAuthPIN(t *testing.T) {
	t.Setenv("SELFSTUDIO_AUTH_PIN", "   ")

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error for whitespace auth PIN")
	}
}

func TestLoadRejectsPlaceholderAuthPIN(t *testing.T) {
	t.Setenv("SELFSTUDIO_AUTH_PIN", placeholderAuthPIN)

	_, err := Load()
	if err == nil {
		t.Fatal("Load returned nil error for placeholder auth PIN")
	}
}
