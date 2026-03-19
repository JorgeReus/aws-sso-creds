//go:build !windows

package util

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomeDirReturnsPath(t *testing.T) {
	homeDir, err := HomeDir()
	if err != nil {
		t.Fatalf("HomeDir() error = %v", err)
	}
	if homeDir == "" {
		t.Fatal("HomeDir() returned empty path")
	}
}

func TestValidateSuperuserFileReturnsMissingFileWarning(t *testing.T) {
	msg := ValidateSuperuserFile(filepath.Join(t.TempDir(), "missing"), &user.User{Uid: "1000"})
	if !strings.Contains(msg, "doesn't exist") {
		t.Fatalf("ValidateSuperuserFile() = %q, want missing file warning", msg)
	}
}

func TestValidateSuperuserFileReturnsEmptyForRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte("ok"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current() error = %v", err)
	}

	msg := ValidateSuperuserFile(path, currentUser)
	if msg != "" {
		t.Fatalf("ValidateSuperuserFile() = %q, want empty string", msg)
	}
}
