package cache

import (
	"context"
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

func TestGetSSOTokenReturnsSavedTokenWhenValidationSucceeds(t *testing.T) {
	origValidate := validateToken
	defer func() { validateToken = origValidate }()
	validateToken = func(context.Context, string, string) error { return nil }

	setCacheDirForTest(t.TempDir())
	token := &SSOToken{
		StartUrl:    "https://dev.awsapps.com/start",
		Region:      "us-east-1",
		AccessToken: "token",
		ExpiresAt:   time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	if err := token.Save(token.StartUrl); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := GetSSOToken(token.StartUrl, token.Region)
	if err != nil {
		t.Fatalf("GetSSOToken() error = %v", err)
	}
	if got == nil || got.AccessToken != "token" {
		t.Fatalf("GetSSOToken() = %#v, want token", got)
	}
}

func TestGetSSOTokenReturnsNilWhenValidationFails(t *testing.T) {
	origValidate := validateToken
	defer func() { validateToken = origValidate }()
	validateToken = func(context.Context, string, string) error { return errors.New("expired") }

	setCacheDirForTest(t.TempDir())
	token := &SSOToken{
		StartUrl:    "https://dev.awsapps.com/start",
		Region:      "us-east-1",
		AccessToken: "token",
		ExpiresAt:   time.Now().Add(time.Hour).Format(time.RFC3339),
	}
	if err := token.Save(token.StartUrl); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := GetSSOToken(token.StartUrl, token.Region)
	if err != nil {
		t.Fatalf("GetSSOToken() error = %v", err)
	}
	if got != nil {
		t.Fatalf("GetSSOToken() = %#v, want nil", got)
	}
}

func TestGetSSOTokenParsesLegacyTimestampFormat(t *testing.T) {
	origValidate := validateToken
	defer func() { validateToken = origValidate }()
	validateToken = func(context.Context, string, string) error { return nil }

	setCacheDirForTest(t.TempDir())
	token := &SSOToken{
		StartUrl:    "https://dev.awsapps.com/start",
		Region:      "us-east-1",
		AccessToken: "token",
		ExpiresAt:   time.Now().UTC().Add(time.Hour).Format("2006-01-02T15:04:05UTC"),
	}
	if err := token.Save(token.StartUrl); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := GetSSOToken(token.StartUrl, token.Region)
	if err != nil {
		t.Fatalf("GetSSOToken() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetSSOToken() = nil, want token")
	}
}

func TestGetSSOTokenReturnsNilForExpiredToken(t *testing.T) {
	origValidate := validateToken
	defer func() { validateToken = origValidate }()
	validateToken = func(context.Context, string, string) error {
		t.Fatal("validateToken should not run for expired token")
		return nil
	}

	setCacheDirForTest(t.TempDir())
	token := &SSOToken{
		StartUrl:    "https://dev.awsapps.com/start",
		Region:      "us-east-1",
		AccessToken: "token",
		ExpiresAt:   time.Now().Add(-time.Hour).Format(time.RFC3339),
	}
	if err := token.Save(token.StartUrl); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := GetSSOToken(token.StartUrl, token.Region)
	if err != nil {
		t.Fatalf("GetSSOToken() error = %v", err)
	}
	if got != nil {
		t.Fatalf("GetSSOToken() = %#v, want nil", got)
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
