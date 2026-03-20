package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertOrganizationConfigCreatesNewConfigWithDefaults(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "nested", "aws-sso-creds.toml")
	err := UpsertOrganizationConfig(configPath, Organization{
		Name:      "dev",
		URL:       "https://dev.awsapps.com/start",
		Prefix:    "dev",
		SSORegion: "us-east-1",
	})
	if err != nil {
		t.Fatalf("UpsertOrganizationConfig() error = %v", err)
	}

	ResetForTest()
	home := t.TempDir()
	if err := Init(home, configPath); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	cfg := GetInstance()
	if cfg.ErrorColor != "#fa0718" {
		t.Fatalf("ErrorColor = %q, want default", cfg.ErrorColor)
	}
	org := cfg.Orgs["dev"]
	if org.URL != "https://dev.awsapps.com/start" || org.Prefix != "dev" || org.EffectiveSSORegion() != "us-east-1" {
		t.Fatalf("org = %#v, want written org values", org)
	}
}

func TestUpsertOrganizationConfigPreservesExistingColorsAndOrganizations(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "aws-sso-creds.toml")
	err := os.WriteFile(configPath, []byte(`
error_color = "#000000"

[organizations.existing]
url = "https://existing.awsapps.com/start"
prefix = "existing"
region = "eu-west-1"
`), 0o644)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err = UpsertOrganizationConfig(configPath, Organization{
		Name:      "dev",
		URL:       "https://dev.awsapps.com/start",
		Prefix:    "dev",
		SSORegion: "us-east-1",
	})
	if err != nil {
		t.Fatalf("UpsertOrganizationConfig() error = %v", err)
	}

	ResetForTest()
	if err := Init(t.TempDir(), configPath); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	cfg := GetInstance()
	if cfg.ErrorColor != "#000000" {
		t.Fatalf("ErrorColor = %q, want preserved color", cfg.ErrorColor)
	}
	if _, ok := cfg.Orgs["existing"]; !ok {
		t.Fatal("existing org missing after upsert")
	}
	if got := cfg.Orgs["dev"].EffectiveSSORegion(); got != "us-east-1" {
		t.Fatalf("dev sso region = %q, want %q", got, "us-east-1")
	}
}

func TestUpsertOrganizationConfigRejectsDuplicateStartURL(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "aws-sso-creds.toml")
	err := os.WriteFile(configPath, []byte(`
[organizations.existing]
url = "https://shared.awsapps.com/start"
prefix = "existing"
region = "eu-west-1"
`), 0o644)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err = UpsertOrganizationConfig(configPath, Organization{
		Name:      "dev",
		URL:       "https://shared.awsapps.com/start",
		Prefix:    "dev",
		SSORegion: "us-east-1",
	})
	if err == nil {
		t.Fatal("UpsertOrganizationConfig() error = nil, want duplicate URL error")
	}
}

func TestUpsertOrganizationConfigRejectsDuplicatePrefix(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "aws-sso-creds.toml")
	err := os.WriteFile(configPath, []byte(`
[organizations.existing]
url = "https://existing.awsapps.com/start"
prefix = "shared"
region = "eu-west-1"
`), 0o644)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err = UpsertOrganizationConfig(configPath, Organization{
		Name:      "dev",
		URL:       "https://dev.awsapps.com/start",
		Prefix:    "shared",
		SSORegion: "us-east-1",
	})
	if err == nil {
		t.Fatal("UpsertOrganizationConfig() error = nil, want duplicate prefix error")
	}
}

func TestUpsertOrganizationConfigAllowsSameOrgNameUpdate(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "aws-sso-creds.toml")
	err := os.WriteFile(configPath, []byte(`
[organizations.dev]
url = "https://old.awsapps.com/start"
prefix = "old"
region = "us-west-1"
`), 0o644)
	if err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err = UpsertOrganizationConfig(configPath, Organization{
		Name:          "dev",
		URL:           "https://new.awsapps.com/start",
		Prefix:        "new",
		SSORegion:     "us-east-1",
		DefaultRegion: "us-west-2",
	})
	if err != nil {
		t.Fatalf("UpsertOrganizationConfig() error = %v", err)
	}

	ResetForTest()
	if err := Init(t.TempDir(), configPath); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	org := GetInstance().Orgs["dev"]
	if org.URL != "https://new.awsapps.com/start" || org.Prefix != "new" || org.EffectiveSSORegion() != "us-east-1" || org.EffectiveDefaultRegion() != "us-west-2" {
		t.Fatalf("org = %#v, want updated org", org)
	}
}

func TestUpsertOrganizationConfigWritesExplicitRegionKeys(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "aws-sso-creds.toml")

	err := UpsertOrganizationConfig(configPath, Organization{
		Name:          "dev",
		URL:           "https://dev.awsapps.com/start",
		Prefix:        "dev",
		SSORegion:     "us-east-1",
		DefaultRegion: "eu-west-1",
	})
	if err != nil {
		t.Fatalf("UpsertOrganizationConfig() error = %v", err)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	text := string(raw)
	if !strings.Contains(text, `sso_region = "us-east-1"`) {
		t.Fatalf("config = %q, want sso_region", text)
	}
	if !strings.Contains(text, `default_region = "eu-west-1"`) {
		t.Fatalf("config = %q, want default_region", text)
	}
	if strings.Contains(text, "\nregion = ") {
		t.Fatalf("config = %q, did not want legacy region key", text)
	}
}
