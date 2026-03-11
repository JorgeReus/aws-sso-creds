package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetSSOClientCredsReturnsSavedCredentials(t *testing.T) {
	setCacheDirForTest(t.TempDir())

	region := "us-east-1"
	creds := &SSOClientCredentials{
		ClientId:     "id",
		ClientSecret: "secret",
		ExpiresAt:    time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	if err := creds.Save(&region); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := GetSSOClientCreds(region)
	if err != nil {
		t.Fatalf("GetSSOClientCreds() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetSSOClientCreds() = nil, want credentials")
	}
	if got.ClientId != "id" {
		t.Fatalf("GetSSOClientCreds().ClientId = %q, want %q", got.ClientId, "id")
	}
}

func TestGetSSOClientCredsReturnsNilForExpiredCredentials(t *testing.T) {
	cacheDir := t.TempDir()
	setCacheDirForTest(cacheDir)

	region := "us-east-1"
	creds := &SSOClientCredentials{
		ClientId:     "id",
		ClientSecret: "secret",
		ExpiresAt:    time.Now().Add(-time.Hour).Format(time.RFC3339),
	}
	if err := creds.Save(&region); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := GetSSOClientCreds(region)
	if err != nil {
		t.Fatalf("GetSSOClientCreds() error = %v", err)
	}
	if got != nil {
		t.Fatalf("GetSSOClientCreds() = %#v, want nil", got)
	}

	path := filepath.Join(cacheDir, "botocore-client-id-"+region+".json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("expired cache file size = %d, want 0", info.Size())
	}
}
