package cache

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/JorgeReus/aws-sso-creds/internal/pkg/util"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sso"
	"github.com/aws/aws-sdk-go/service/ssooidc"
)

var cacheDir string

func (c *SSOClientCredentials) Save(region *string) error {
	contents, err := json.Marshal(c)
	if err != nil {
		return err
	}
	filePath := path.Join(cacheDir, fmt.Sprintf("botocore-client-id-%s.json", *region))
	err = ioutil.WriteFile(filePath, contents, 0644)
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
	os.MkdirAll(cacheDir, os.ModePerm)
}

func isExpired(expiresAt time.Time) bool {
	if expiresAt.Before(time.Now()) {
		return true
	}
	return false
}

func GetSSOClientCreds(region string) (*SSOClientCredentials, error) {
	var result SSOClientCredentials
	filePath := path.Join(cacheDir, fmt.Sprintf("botocore-client-id-%s.json", region))
	f, err := os.OpenFile(filePath, os.O_CREATE, 0644)
	defer f.Close()
	if err != nil {
		return &result, err
	}
	contents, err := ioutil.ReadAll(f)
	if err != nil {
		return &result, err
	}

	json.Unmarshal([]byte(contents), &result)
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
		os.Truncate(filePath, 0)
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
	err = ioutil.WriteFile(filePath, contents, 0644)
	if err != nil {
		return err
	}
	return nil
}

func GetSSOToken(
	url string,
	sess *session.Session,
	oidcClient *ssooidc.SSOOIDC,
	region string,
) (*SSOToken, error) {
	var result SSOToken
	h := sha1.New()
	h.Write([]byte(url))
	filePath := path.Join(cacheDir, fmt.Sprintf("%s.json", hex.EncodeToString(h.Sum(nil))))
	f, err := os.OpenFile(filePath, os.O_CREATE, 0644)
	defer f.Close()
	if err != nil {
		return &result, err
	}
	contents, err := ioutil.ReadAll(f)
	if err != nil {
		return &result, err
	}

	json.Unmarshal([]byte(contents), &result)
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
		os.Truncate(filePath, 0)
		return nil, nil
	}

	// Do an extra check to see if the token has expired api wise
	aux := sso.New(sess, aws.NewConfig().WithRegion(region))
	_, err = aux.ListAccounts(&sso.ListAccountsInput{
		AccessToken: &result.AccessToken,
	})

	if err != nil {
		return nil, nil
	}

	return &result, nil
}
