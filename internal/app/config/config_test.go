package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitLoadsOrganizationsAndDefaults(t *testing.T) {
	ResetForTest()

	home := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "aws-sso-creds.toml")
	err := os.WriteFile(configPath, []byte(`
[organizations.dev]
url = "https://dev.awsapps.com/start"
prefix = "dev"
region = "us-east-1"
`), 0o644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	err = Init(home, configPath)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	cfg := GetInstance()
	if cfg == nil {
		t.Fatal("GetInstance() = nil")
	}
	if cfg.Home != home {
		t.Fatalf("cfg.Home = %q, want %q", cfg.Home, home)
	}
	if cfg.ErrorColor != "#fa0718" {
		t.Fatalf("cfg.ErrorColor = %q, want %q", cfg.ErrorColor, "#fa0718")
	}
	if got := cfg.Orgs["dev"].Name; got != "dev" {
		t.Fatalf("cfg.Orgs[dev].Name = %q, want %q", got, "dev")
	}
}

func TestInitReturnsErrorForMissingOrganizationFields(t *testing.T) {
	ResetForTest()

	home := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "aws-sso-creds.toml")
	err := os.WriteFile(configPath, []byte(`
[organizations.dev]
url = "https://dev.awsapps.com/start"
`), 0o644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	err = Init(home, configPath)
	if err == nil {
		t.Fatal("Init() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "Missing required attributes") {
		t.Fatalf("Init() error = %q, want validation message", err)
	}
}
