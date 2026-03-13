package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bigkevmcd/go-configparser"
	"github.com/ktr0731/go-fuzzyfinder"
)

func TestNewFuzzyPreviewerLoadsCredentialsAndRoles(t *testing.T) {
	dir := t.TempDir()
	credentialsPath := filepath.Join(dir, "credentials")
	rolesPath := filepath.Join(dir, "config")

	if err := os.WriteFile(credentialsPath, []byte(`
[tmp:dev:Admin]
aws_access_key_id=AKIA
issued_time=100
expires_time=4102444800
`), 0o644); err != nil {
		t.Fatalf("WriteFile() credentials error = %v", err)
	}
	if err := os.WriteFile(rolesPath, []byte(`
[profile dev:Admin]
org=dev
sso_auto_populated=true
sso_account_id=111111111111
`), 0o644); err != nil {
		t.Fatalf("WriteFile() roles error = %v", err)
	}

	fp, err := NewFuzzyPreviewer(credentialsPath, rolesPath)
	if err != nil {
		t.Fatalf("NewFuzzyPreviewer() error = %v", err)
	}
	if len(fp.outputSections) == 0 {
		t.Fatal("outputSections is empty")
	}
}

func TestGeneratePreviewAttrsReturnsAccountAndExpiryDetails(t *testing.T) {
	fp := &FuzzyPreviewer{
		rolesMapping: &map[string]string{"selected": "profile dev:Admin"},
		entries: mustPreviewEntries(t, map[string]map[string]string{
			"profile dev:Admin": {
				"sso_account_id":        "111111111111",
				"aws_access_key_id":     "AKIA",
				"aws_secret_access_key": "secret",
				"issued_time":           "100",
				"expires_time":          "4102444800",
			},
		}),
	}

	got, err := fp.generatePreviewAttrs("selected")
	if err != nil {
		t.Fatalf("generatePreviewAttrs() error = %v", err)
	}
	if strings.Contains(*got, "aws_secret_access_key") {
		t.Fatalf("preview leaked secret: %q", *got)
	}
	if !strings.Contains(*got, "Sso Account Id") || !strings.Contains(*got, "Status: Valid") {
		t.Fatalf("preview = %q, want account and status details", *got)
	}
}

func TestPreviewReturnsSelectedProfileName(t *testing.T) {
	origFindMulti := findMulti
	defer func() { findMulti = origFindMulti }()

	fp := &FuzzyPreviewer{
		outputSections: []string{"(SSO profile) dev:Admin"},
		rolesMapping:   &map[string]string{"(SSO profile) dev:Admin": "profile dev:Admin"},
		entries: mustPreviewEntries(t, map[string]map[string]string{
			"profile dev:Admin": {"org": "dev"},
		}),
	}

	findMulti = func(slice interface{}, itemFunc func(i int) string, opts ...fuzzyfinder.Option) ([]int, error) {
		if got := itemFunc(0); got != "(SSO profile) dev:Admin" {
			t.Fatalf("itemFunc(0) = %q", got)
		}
		return []int{0}, nil
	}

	got, err := fp.Preview()
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	if *got != "dev:Admin" {
		t.Fatalf("Preview() = %q, want %q", *got, "dev:Admin")
	}
}

func TestPreviewReturnsFinderError(t *testing.T) {
	origFindMulti := findMulti
	defer func() { findMulti = origFindMulti }()

	findMulti = func(slice interface{}, itemFunc func(i int) string, opts ...fuzzyfinder.Option) ([]int, error) {
		return nil, errors.New("cancelled")
	}

	fp := &FuzzyPreviewer{
		outputSections: []string{"(SSO profile) dev:Admin"},
		rolesMapping:   &map[string]string{"(SSO profile) dev:Admin": "profile dev:Admin"},
		entries:        mustPreviewEntries(t, map[string]map[string]string{"profile dev:Admin": {"org": "dev"}}),
	}

	_, err := fp.Preview()
	if err == nil || err.Error() != "cancelled" {
		t.Fatalf("Preview() error = %v, want cancelled", err)
	}
}

func mustPreviewEntries(t *testing.T, sections map[string]map[string]string) *configparser.ConfigParser {
	t.Helper()
	entries := configparser.New()
	for section, items := range sections {
		if err := entries.AddSection(section); err != nil {
			t.Fatalf("AddSection() error = %v", err)
		}
		for k, v := range items {
			entries.Set(section, k, v)
		}
	}
	return entries
}
