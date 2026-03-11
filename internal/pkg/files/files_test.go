package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigAndCredentialsHelpersManageAutoPopulatedEntries(t *testing.T) {
	home := t.TempDir()
	err := os.MkdirAll(filepath.Join(home, ".aws"), 0o755)
	if err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	configFile, err := NewConfigFile(home)
	if err != nil {
		t.Fatalf("NewConfigFile() error = %v", err)
	}

	section, err := configFile.File.NewSection("profile dev:account:Admin")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = section.NewKey("org", "dev")
	_, _ = section.NewKey("sso_auto_populated", "true")

	if err := configFile.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if ConfigFileSSOEmpty(home, "dev") {
		t.Fatal("ConfigFileSSOEmpty() = true, want false")
	}

	configFile.CleanTemporaryRoles("dev")
	if err := configFile.Save(); err != nil {
		t.Fatalf("Save() after cleanup error = %v", err)
	}
	if !ConfigFileSSOEmpty(home, "dev") {
		t.Fatal("ConfigFileSSOEmpty() = false, want true after cleanup")
	}
}

func TestGetentryByAWSProfileReturnsExpectedSection(t *testing.T) {
	home := t.TempDir()
	err := os.MkdirAll(filepath.Join(home, ".aws"), 0o755)
	if err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	configFile, err := NewConfigFile(home)
	if err != nil {
		t.Fatalf("NewConfigFile() error = %v", err)
	}
	_, err = configFile.File.NewSection("profile dev:account:Admin")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}

	section, err := configFile.GetentryByAWSProfile("dev:account:Admin")
	if err != nil {
		t.Fatalf("GetentryByAWSProfile() error = %v", err)
	}
	if section.Name() != "profile dev:account:Admin" {
		t.Fatalf("section.Name() = %q, want %q", section.Name(), "profile dev:account:Admin")
	}
}

func TestNewCredentialsFileCreatesCredentialsPath(t *testing.T) {
	home := t.TempDir()
	err := os.MkdirAll(filepath.Join(home, ".aws"), 0o755)
	if err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	credentialsFile, err := NewCredentialsFile(home)
	if err != nil {
		t.Fatalf("NewCredentialsFile() error = %v", err)
	}
	if credentialsFile.Path != filepath.Join(home, ".aws", "credentials") {
		t.Fatalf("Path = %q, want %q", credentialsFile.Path, filepath.Join(home, ".aws", "credentials"))
	}
	if _, err := os.Stat(credentialsFile.Path); err != nil {
		t.Fatalf("credentials file stat error = %v", err)
	}
}
