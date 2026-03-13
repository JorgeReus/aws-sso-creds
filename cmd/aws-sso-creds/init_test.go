package awsssocreds

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JorgeReus/aws-sso-creds/internal/app/config"
)

func TestPromptValueReadsInputLine(t *testing.T) {
	in := strings.NewReader("dev\n")
	var out bytes.Buffer

	got, err := promptValue(in, &out, "Organization name", validateNonEmpty)
	if err != nil {
		t.Fatalf("promptValue() error = %v", err)
	}
	if got != "dev" {
		t.Fatalf("promptValue() = %q, want %q", got, "dev")
	}
	if !strings.Contains(out.String(), "Organization name") {
		t.Fatalf("prompt output = %q, want prompt label", out.String())
	}
}

func TestPromptValueRetriesUntilValidInput(t *testing.T) {
	in := strings.NewReader("\nhttps://dev.awsapps.com/start\n")
	var out bytes.Buffer

	got, err := promptValue(in, &out, "Start URL", validateStartURL)
	if err != nil {
		t.Fatalf("promptValue() error = %v", err)
	}
	if got != "https://dev.awsapps.com/start" {
		t.Fatalf("promptValue() = %q, want valid retry value", got)
	}
	if !strings.Contains(out.String(), "value cannot be empty") {
		t.Fatalf("prompt output = %q, want validation feedback", out.String())
	}
}

func TestValidateStartURLRejectsMissingScheme(t *testing.T) {
	err := validateStartURL("d-9067c5703a.awsapps.com/start")
	if err == nil {
		t.Fatal("validateStartURL() error = nil, want invalid URL error")
	}
}

func TestInitCommandCreatesOrganizationConfig(t *testing.T) {
	configPath = filepath.Join(t.TempDir(), "aws-sso-creds.toml")
	var out bytes.Buffer
	cmd := newInitCmd(initDeps{
		in:        strings.NewReader("dev\nhttps://dev.awsapps.com/start\ndev\nus-east-1\n"),
		out:       &out,
		upsertOrg: config.UpsertOrganizationConfig,
		fileExists: func(string) bool {
			return false
		},
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), `Organization "dev" created`) {
		t.Fatalf("output = %q, want created message", out.String())
	}

	ResetConfigAfterInitTest(t)
	if err := config.Init(t.TempDir(), configPath); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if got := config.GetInstance().Orgs["dev"].Prefix; got != "dev" {
		t.Fatalf("prefix = %q, want dev", got)
	}
}

func TestInitCommandUpdatesExistingOrganization(t *testing.T) {
	configPath = filepath.Join(t.TempDir(), "aws-sso-creds.toml")
	if err := os.WriteFile(configPath, []byte(`
[organizations.dev]
url = "https://old.awsapps.com/start"
prefix = "old"
region = "us-west-1"
`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var out bytes.Buffer
	cmd := newInitCmd(initDeps{
		in:        strings.NewReader("dev\nhttps://dev.awsapps.com/start\ndev\nus-east-1\n"),
		out:       &out,
		upsertOrg: config.UpsertOrganizationConfig,
		fileExists: func(string) bool {
			return true
		},
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), `Organization "dev" updated`) {
		t.Fatalf("output = %q, want updated message", out.String())
	}

	ResetConfigAfterInitTest(t)
	if err := config.Init(t.TempDir(), configPath); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if got := config.GetInstance().Orgs["dev"].Region; got != "us-east-1" {
		t.Fatalf("region = %q, want us-east-1", got)
	}
}

func TestInitCommandReturnsDuplicateConflict(t *testing.T) {
	configPath = filepath.Join(t.TempDir(), "aws-sso-creds.toml")
	if err := os.WriteFile(configPath, []byte(`
[organizations.existing]
url = "https://shared.awsapps.com/start"
prefix = "existing"
region = "eu-west-1"
`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var out bytes.Buffer
	cmd := newInitCmd(initDeps{
		in:        strings.NewReader("dev\nhttps://shared.awsapps.com/start\ndev\nus-east-1\n"),
		out:       &out,
		upsertOrg: config.UpsertOrganizationConfig,
		fileExists: func(string) bool {
			return true
		},
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `already uses start URL`) {
		t.Fatalf("Execute() error = %v, want duplicate URL conflict", err)
	}
}

func ResetConfigAfterInitTest(t *testing.T) {
	t.Helper()
	config.ResetForTest()
}
