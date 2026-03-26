package cache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sso"

	"github.com/JorgeReus/aws-sso-creds/internal/pkg/util"
)

type tokenValidatorAPI interface {
	ListAccounts(
		context.Context,
		*sso.ListAccountsInput,
		...func(*sso.Options),
	) (*sso.ListAccountsOutput, error)
}

var cacheDir string

var newValidationClient = func(region string) tokenValidatorAPI {
	return sso.New(sso.Options{Region: region})
}

var validateToken = func(ctx context.Context, region, accessToken string) error {
	client := newValidationClient(region)
	_, err := client.ListAccounts(ctx, &sso.ListAccountsInput{
		AccessToken: aws.String(accessToken),
	})
	return err
}

func (c *SSOClientCredentials) Save(region *string) error {
	contents, err := json.Marshal(c)
	if err != nil {
		return err
	}
	filePath := path.Join(cacheDir, fmt.Sprintf("botocore-client-id-%s.json", *region))
	err = os.WriteFile(filePath, contents, 0o644)
	if err != nil {
		return err
	}
	return nil
}

func init() {
	homeDir, err := util.HomeDir()
	if err != nil {
		panic(err)
	}
	cacheDir = path.Join(homeDir, ".aws", "sso", "cache")
	if err := os.MkdirAll(cacheDir, os.ModePerm); err != nil {
		panic(err)
	}
}

func setCacheDirForTest(dir string) {
	cacheDir = dir
	if err := os.MkdirAll(cacheDir, os.ModePerm); err != nil {
		panic(err)
	}
}

func isExpired(expiresAt time.Time) bool {
	return expiresAt.Before(time.Now())
}

func GetSSOClientCreds(region string) (*SSOClientCredentials, error) {
	var result SSOClientCredentials
	filePath := path.Join(cacheDir, fmt.Sprintf("botocore-client-id-%s.json", region))
	f, err := os.OpenFile(filePath, os.O_CREATE, 0644)
	if err != nil {
		return &result, err
	}
	defer func() {
		_ = f.Close()
	}()
	contents, err := io.ReadAll(f)
	if err != nil {
		return &result, err
	}

	if err := json.Unmarshal(contents, &result); err != nil && len(contents) > 0 {
		return &result, err
	}
	if result.ExpiresAt == "" {
		return nil, nil
	}
	var t time.Time
	t, err = time.Parse(time.RFC3339, result.ExpiresAt)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05UTC", result.ExpiresAt)
		if err != nil {
			return nil, err
		}
	}
	if isExpired(t) {
		if err := os.Truncate(filePath, 0); err != nil {
			return nil, err
		}
		return nil, nil
	}

	return &result, nil
}

func (c *SSOToken) Save(url string) error {
	contents, err := json.Marshal(c)
	if err != nil {
		return err
	}
	h := sha1.New()
	h.Write([]byte(url))
	filePath := path.Join(cacheDir, fmt.Sprintf("%s.json", hex.EncodeToString(h.Sum(nil))))
	err = os.WriteFile(filePath, contents, 0o644)
	if err != nil {
		return err
	}
	return nil
}

func GetSSOToken(
	url string,
	region string,
) (*SSOToken, error) {
	var result SSOToken
	h := sha1.New()
	h.Write([]byte(url))
	filePath := path.Join(cacheDir, fmt.Sprintf("%s.json", hex.EncodeToString(h.Sum(nil))))
	f, err := os.OpenFile(filePath, os.O_CREATE, 0644)
	if err != nil {
		return &result, err
	}
	defer func() {
		_ = f.Close()
	}()
	contents, err := io.ReadAll(f)
	if err != nil {
		return &result, err
	}

	if err := json.Unmarshal(contents, &result); err != nil && len(contents) > 0 {
		return &result, err
	}
	if result.ExpiresAt == "" {
		return nil, nil
	}
	var t time.Time
	t, err = time.Parse(time.RFC3339, result.ExpiresAt)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05UTC", result.ExpiresAt)
		if err != nil {
			return nil, err
		}
	}
	if isExpired(t) {
		if err := os.Truncate(filePath, 0); err != nil {
			return nil, err
		}
		return nil, nil
	}

	if err := validateToken(context.Background(), region, result.AccessToken); err != nil {
		return nil, nil
	}

	return &result, nil
}
