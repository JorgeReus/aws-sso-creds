package cache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sso"
)

type fakeValidatorClient struct {
	listAccountsFn func(context.Context, *sso.ListAccountsInput, ...func(*sso.Options)) (*sso.ListAccountsOutput, error)
}

func (f *fakeValidatorClient) ListAccounts(
	ctx context.Context,
	input *sso.ListAccountsInput,
	optFns ...func(*sso.Options),
) (*sso.ListAccountsOutput, error) {
	return f.listAccountsFn(ctx, input, optFns...)
}

func TestGetSSOClientCreds(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(t *testing.T) (string, string)
		wantClientID string
		wantNil      bool
		wantErr      bool
		wantTrunc    bool
	}{
		{
			name: "saved credentials",
			setup: func(t *testing.T) (string, string) {
				t.Helper()
				cacheDir := t.TempDir()
				setCacheDirForTest(cacheDir)
				region := "us-east-1"
				creds := &SSOClientCredentials{
					ClientId:     "id",
					ClientSecret: "secret",
					ExpiresAt:    time.Now().Add(time.Hour).Format(time.RFC3339),
				}
				if err := creds.Save(&region); err != nil {
					t.Fatalf("Save() error = %v", err)
				}
				return region, cacheDir
			},
			wantClientID: "id",
		},
		{
			name: "expired credentials",
			setup: func(t *testing.T) (string, string) {
				t.Helper()
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
				return region, cacheDir
			},
			wantTrunc: true,
			wantNil:   true,
		},
		{
			name: "empty cache file",
			setup: func(t *testing.T) (string, string) {
				t.Helper()
				cacheDir := t.TempDir()
				setCacheDirForTest(cacheDir)
				return "us-east-1", cacheDir
			},
			wantNil: true,
		},
		{
			name: "bad timestamp",
			setup: func(t *testing.T) (string, string) {
				t.Helper()
				cacheDir := t.TempDir()
				setCacheDirForTest(cacheDir)
				region := "us-east-1"
				creds := &SSOClientCredentials{
					ClientId:     "id",
					ClientSecret: "secret",
					ExpiresAt:    "definitely-not-a-time",
				}
				if err := creds.Save(&region); err != nil {
					t.Fatalf("Save() error = %v", err)
				}
				return region, cacheDir
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origCacheDir := cacheDir
			defer func() { cacheDir = origCacheDir }()

			region, cacheDir := tt.setup(t)
			got, err := GetSSOClientCreds(region)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("GetSSOClientCreds() = %#v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetSSOClientCreds() error = %v", err)
			}

			if tt.wantNil {
				if got != nil {
					t.Fatalf("GetSSOClientCreds() = %#v, want nil", got)
				}
			} else {
				if got == nil {
					t.Fatal("GetSSOClientCreds() = nil, want credentials")
				}
				if got.ClientId != tt.wantClientID {
					t.Fatalf("GetSSOClientCreds().ClientId = %q, want %q", got.ClientId, tt.wantClientID)
				}
			}

			if tt.wantTrunc {
				path := filepath.Join(cacheDir, "botocore-client-id-"+region+".json")
				info, err := os.Stat(path)
				if err != nil {
					t.Fatalf("Stat() error = %v", err)
				}
				if info.Size() != 0 {
					t.Fatalf("expired cache file size = %d, want 0", info.Size())
				}
			}
		})
	}
}

func TestGetSSOToken(t *testing.T) {
	tests := []struct {
		name           string
		validateErr    error
		expiresAt      string
		wantToken      bool
		wantErr        bool
		wantTruncated  bool
		setupTokenJSON bool
		wantValidate   bool
	}{
		{
			name:         "saved token when validation succeeds",
			expiresAt:    time.Now().Add(time.Hour).Format(time.RFC3339),
			wantToken:    true,
			wantValidate: true,
		},
		{
			name:         "nil when validation fails",
			validateErr:  errors.New("expired"),
			expiresAt:    time.Now().Add(time.Hour).Format(time.RFC3339),
			wantValidate: true,
		},
		{
			name:         "legacy timestamp parsing",
			expiresAt:    time.Now().UTC().Add(time.Hour).Format("2006-01-02T15:04:05UTC"),
			wantToken:    true,
			wantValidate: true,
		},
		{
			name:          "expired token truncation",
			expiresAt:     time.Now().Add(-time.Hour).Format(time.RFC3339),
			wantTruncated: true,
		},
		{
			name:           "invalid JSON",
			setupTokenJSON: true,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origValidate := validateToken
			origCacheDir := cacheDir
			defer func() {
				validateToken = origValidate
				cacheDir = origCacheDir
			}()

			testCacheDir := t.TempDir()
			setCacheDirForTest(testCacheDir)

			token := &SSOToken{
				StartUrl:    "https://dev.awsapps.com/start",
				Region:      "us-east-1",
				AccessToken: "token",
				ExpiresAt:   tt.expiresAt,
			}
			validateCalled := false
			validateToken = func(context.Context, string, string) error {
				validateCalled = true
				return tt.validateErr
			}
			if tt.setupTokenJSON {
				filePath := filepath.Join(testCacheDir, tokenCacheFileName(token.StartUrl))
				if err := os.WriteFile(filePath, []byte("{invalid"), 0o644); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
			} else if err := token.Save(token.StartUrl); err != nil {
				t.Fatalf("Save() error = %v", err)
			}

			got, err := GetSSOToken(token.StartUrl, token.Region)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("GetSSOToken() = %#v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetSSOToken() error = %v", err)
			}

			if tt.wantToken {
				if got == nil || got.AccessToken != "token" {
					t.Fatalf("GetSSOToken() = %#v, want token", got)
				}
			} else if got != nil {
				t.Fatalf("GetSSOToken() = %#v, want nil", got)
			}

			if validateCalled != tt.wantValidate {
				t.Fatalf("validateToken() called = %t, want %t", validateCalled, tt.wantValidate)
			}

			if tt.wantTruncated {
				path := filepath.Join(testCacheDir, tokenCacheFileName(token.StartUrl))
				info, err := os.Stat(path)
				if err != nil {
					t.Fatalf("Stat() error = %v", err)
				}
				if info.Size() != 0 {
					t.Fatalf("expired cache file size = %d, want 0", info.Size())
				}
			}
		})
	}
}

func tokenCacheFileName(url string) string {
	h := sha1.New()
	h.Write([]byte(url))
	return hex.EncodeToString(h.Sum(nil)) + ".json"
}

func TestGetSSOClientCredsReturnsErrorForInvalidJSON(t *testing.T) {
	origCacheDir := cacheDir
	defer func() { cacheDir = origCacheDir }()

	testCacheDir := t.TempDir()
	setCacheDirForTest(testCacheDir)
	region := "us-east-1"
	filePath := filepath.Join(testCacheDir, "botocore-client-id-"+region+".json")
	if err := os.WriteFile(filePath, []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := GetSSOClientCreds(region)
	if err == nil {
		t.Fatalf("GetSSOClientCreds() = %#v, want error", got)
	}
}

func TestValidateTokenUsesFactoryRegionAndAccessToken(t *testing.T) {
	origFactory := newValidationClient
	defer func() { newValidationClient = origFactory }()

	var gotRegion string
	var gotToken *string
	newValidationClient = func(region string) tokenValidatorAPI {
		gotRegion = region
		return &fakeValidatorClient{
			listAccountsFn: func(_ context.Context, input *sso.ListAccountsInput, _ ...func(*sso.Options)) (*sso.ListAccountsOutput, error) {
				gotToken = input.AccessToken
				return &sso.ListAccountsOutput{}, nil
			},
		}
	}

	if err := validateToken(context.Background(), "us-east-1", "token"); err != nil {
		t.Fatalf("validateToken() error = %v", err)
	}
	if gotRegion != "us-east-1" {
		t.Fatalf("factory region = %q, want %q", gotRegion, "us-east-1")
	}
	if aws.ToString(gotToken) != "token" {
		t.Fatalf("access token = %q, want %q", aws.ToString(gotToken), "token")
	}
}

func TestValidateTokenReturnsListAccountsError(t *testing.T) {
	origFactory := newValidationClient
	defer func() { newValidationClient = origFactory }()

	wantErr := errors.New("list accounts failed")
	newValidationClient = func(string) tokenValidatorAPI {
		return &fakeValidatorClient{
			listAccountsFn: func(_ context.Context, _ *sso.ListAccountsInput, _ ...func(*sso.Options)) (*sso.ListAccountsOutput, error) {
				return nil, wantErr
			},
		}
	}

	if err := validateToken(context.Background(), "us-east-1", "token"); !errors.Is(err, wantErr) {
		t.Fatalf("validateToken() error = %v, want %v", err, wantErr)
	}
}
