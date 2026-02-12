package cache

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSOClientCredentialsSave(t *testing.T) {
	// Setup temporary cache directory
	tmpDir := t.TempDir()
	originalCacheDir := cacheDir
	cacheDir = tmpDir
	defer func() { cacheDir = originalCacheDir }()

	t.Run("successfully saves credentials", func(t *testing.T) {
		region := "us-east-1"
		creds := &SSOClientCredentials{
			ClientId:     "test-client-id",
			ClientSecret: "test-secret",
			ExpiresAt:    time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}

		err := creds.Save(&region)
		assert.NoError(t, err)

		// Verify file was created
		filePath := filepath.Join(tmpDir, "botocore-client-id-us-east-1.json")
		assert.FileExists(t, filePath)
	})

	t.Run("creates parent directories if needed", func(t *testing.T) {
		region := "eu-west-1"
		creds := &SSOClientCredentials{
			ClientId:     "test-client-id",
			ClientSecret: "test-secret",
			ExpiresAt:    time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}

		err := creds.Save(&region)
		assert.NoError(t, err)

		filePath := filepath.Join(tmpDir, "botocore-client-id-eu-west-1.json")
		assert.FileExists(t, filePath)
	})
}

func TestGetSSOClientCreds(t *testing.T) {
	// Setup temporary cache directory
	tmpDir := t.TempDir()
	originalCacheDir := cacheDir
	cacheDir = tmpDir
	defer func() { cacheDir = originalCacheDir }()

	t.Run("returns nil when credentials dont exist", func(t *testing.T) {
		creds, err := GetSSOClientCreds("us-east-1")
		assert.NoError(t, err)
		assert.Nil(t, creds)
	})

	t.Run("returns stored credentials when valid", func(t *testing.T) {
		region := "us-east-1"
		expectedCreds := &SSOClientCredentials{
			ClientId:     "test-client-id",
			ClientSecret: "test-secret",
			ExpiresAt:    time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}

		err := expectedCreds.Save(&region)
		require.NoError(t, err)

		creds, err := GetSSOClientCreds(region)
		assert.NoError(t, err)
		assert.NotNil(t, creds)
		assert.Equal(t, expectedCreds.ClientId, creds.ClientId)
		assert.Equal(t, expectedCreds.ClientSecret, creds.ClientSecret)
	})

	t.Run("returns nil for expired credentials", func(t *testing.T) {
		region := "us-east-1"
		expiredCreds := &SSOClientCredentials{
			ClientId:     "test-client-id",
			ClientSecret: "test-secret",
			ExpiresAt:    time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		}

		err := expiredCreds.Save(&region)
		require.NoError(t, err)

		creds, err := GetSSOClientCreds(region)
		assert.NoError(t, err)
		assert.Nil(t, creds)
	})
}

func TestSSOTokenSave(t *testing.T) {
	tmpDir := t.TempDir()
	originalCacheDir := cacheDir
	cacheDir = tmpDir
	defer func() { cacheDir = originalCacheDir }()

	t.Run("successfully saves token", func(t *testing.T) {
		url := "https://my-sso-instance.awsapps.com/start"
		token := &SSOToken{
			StartUrl:    url,
			Region:      "us-east-1",
			AccessToken: "test-access-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}

		err := token.Save(url)
		assert.NoError(t, err)

		// Verify file was created
		files, err := os.ReadDir(tmpDir)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(files))
	})
}

func TestIsExpired(t *testing.T) {
	t.Run("returns true for past time", func(t *testing.T) {
		pastTime := time.Now().Add(-1 * time.Hour)
		assert.True(t, isExpired(pastTime))
	})

	t.Run("returns false for future time", func(t *testing.T) {
		futureTime := time.Now().Add(1 * time.Hour)
		assert.False(t, isExpired(futureTime))
	})
}

func TestGetSSOToken(t *testing.T) {
	tmpDir := t.TempDir()
	originalCacheDir := cacheDir
	cacheDir = tmpDir
	defer func() { cacheDir = originalCacheDir }()

	t.Run("returns nil when token doesnt exist", func(t *testing.T) {
		ctx := context.Background()
		url := "https://my-sso-instance.awsapps.com/start"
		token, err := GetSSOToken(ctx, url, nil, "us-east-1")
		assert.NoError(t, err)
		assert.Nil(t, token)
	})

	t.Run("returns stored token when valid", func(t *testing.T) {
		ctx := context.Background()
		url := "https://my-sso-instance.awsapps.com/start"

		// Pre-save a token
		savedToken := &SSOToken{
			StartUrl:    url,
			Region:      "us-east-1",
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}
		err := savedToken.Save(url)
		require.NoError(t, err)

		// Retrieve it (with nil client - skips API validation)
		token, err := GetSSOToken(ctx, url, nil, "us-east-1")
		// In test mode with nil client, validation is skipped
		assert.NoError(t, err)
		assert.NotNil(t, token)
		assert.Equal(t, savedToken.AccessToken, token.AccessToken)
	})

	t.Run("returns nil for expired token", func(t *testing.T) {
		ctx := context.Background()
		url := "https://my-sso-instance.awsapps.com/start"

		// Pre-save an expired token
		expiredToken := &SSOToken{
			StartUrl:    url,
			Region:      "us-east-1",
			AccessToken: "test-token",
			ExpiresAt:   time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		}
		err := expiredToken.Save(url)
		require.NoError(t, err)

		// Try to retrieve it
		token, err := GetSSOToken(ctx, url, nil, "us-east-1")
		assert.NoError(t, err)
		assert.Nil(t, token)
	})
}
