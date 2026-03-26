package awsssocreds

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	awssso "github.com/aws/aws-sdk-go-v2/service/sso"
	awsssotypes "github.com/aws/aws-sdk-go-v2/service/sso/types"
	ini "gopkg.in/ini.v1"

	"github.com/JorgeReus/aws-sso-creds/internal/app/config"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/files"
)

type fakeCachedFlow struct {
	creds *awssso.GetRoleCredentialsOutput
	err   error
}

func (f fakeCachedFlow) GetCredsByRoleName(
	roleName string,
	accountId string,
) (*awssso.GetRoleCredentialsOutput, error) {
	return f.creds, f.err
}

func TestOpenCommandUsesAWSProfileAndCallsOpenConsole(t *testing.T) {
	called := false
	cmd := newOpenCmd(openDeps{
		initConfig: func(home, configPath string) error { return nil },
		getenv: func(key string) string {
			if key != "AWS_PROFILE" {
				t.Fatalf("getenv key = %q, want AWS_PROFILE", key)
			}
			return "dev:account:Admin"
		},
		openConsole: func(roleName string, sessionDuration uint) error {
			called = true
			if roleName != "dev:account:Admin" {
				t.Fatalf("roleName = %q, want %q", roleName, "dev:account:Admin")
			}
			if sessionDuration != 3600 {
				t.Fatalf("sessionDuration = %d, want 3600", sessionDuration)
			}
			return nil
		},
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !called {
		t.Fatal("openConsole was not called")
	}
}

func TestOpenCommandReturnsErrorWhenProfileMissing(t *testing.T) {
	cmd := newOpenCmd(openDeps{
		initConfig: func(home, configPath string) error { return nil },
		getenv:     func(string) string { return "" },
		openConsole: func(roleName string, sessionDuration uint) error {
			t.Fatal("openConsole should not be called")
			return nil
		},
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
}

func TestOpenCommandReturnsInitConfigError(t *testing.T) {
	wantErr := errors.New("bad config")
	cmd := newOpenCmd(openDeps{
		initConfig: func(home, configPath string) error { return wantErr },
		getenv:     func(string) string { return "ignored" },
	})

	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want %v", err, wantErr)
	}
}

func TestOpenConsoleWithDepsBuildsSigninURL(t *testing.T) {
	setupOpenConfig(t)
	cfgFile := &files.AWSFile{
		File: ini.Empty(),
		Path: filepath.Join(config.GetInstance().Home, ".aws", "config"),
	}
	section, _ := cfgFile.File.NewSection("profile dev:Admin")
	_, _ = section.NewKey("org", "dev")
	_, _ = section.NewKey("sso_role_name", "Admin")
	_, _ = section.NewKey("sso_account_id", "111111111111")
	_, _ = section.NewKey("sso_start_url", "https://dev.awsapps.com/start")
	_, _ = section.NewKey("region", "us-east-1")

	var federatedURL string
	var opened string
	err := openConsoleWithDeps("dev:Admin", 3600, openDeps{
		newConfigFile: func(string) (*files.AWSFile, error) { return cfgFile, nil },
		getCachedFlow: func(config.Organization) (cachedFlow, error) {
			return fakeCachedFlow{
				creds: &awssso.GetRoleCredentialsOutput{
					RoleCredentials: &awsssotypes.RoleCredentials{
						AccessKeyId:     awsv2.String("AKIA"),
						SecretAccessKey: awsv2.String("secret"),
						SessionToken:    awsv2.String("token"),
					},
				},
			}, nil
		},
		httpGet: func(raw string) (*http.Response, error) {
			federatedURL = raw
			return &http.Response{
				Body: io.NopCloser(strings.NewReader(`{"SigninToken":"signin-token"}`)),
			}, nil
		},
		openURL: func(raw string) error {
			opened = raw
			return nil
		},
	})
	if err != nil {
		t.Fatalf("openConsoleWithDeps() error = %v", err)
	}
	if !strings.Contains(federatedURL, "Action=getSigninToken") {
		t.Fatalf("federated URL = %q, want getSigninToken action", federatedURL)
	}
	parsedURL, err := url.Parse(federatedURL)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	sessionValue := parsedURL.Query().Get("Session")
	if sessionValue == "" {
		t.Fatalf("federated URL = %q, want Session parameter", federatedURL)
	}
	decodedSession, err := url.QueryUnescape(sessionValue)
	if err != nil {
		t.Fatalf("QueryUnescape() error = %v", err)
	}
	var sessionPayload map[string]string
	if err := json.Unmarshal([]byte(decodedSession), &sessionPayload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if sessionPayload["sessionId"] != "AKIA" {
		t.Fatalf("sessionId = %q, want %q", sessionPayload["sessionId"], "AKIA")
	}
	if sessionPayload["sessionKey"] != "secret" {
		t.Fatalf("sessionKey = %q, want %q", sessionPayload["sessionKey"], "secret")
	}
	if sessionPayload["sessionToken"] != "token" {
		t.Fatalf("sessionToken = %q, want %q", sessionPayload["sessionToken"], "token")
	}
	if !strings.Contains(opened, "SigninToken=signin-token") {
		t.Fatalf("opened URL = %q, want signin token", opened)
	}
}

func TestOpenConsoleWithDepsReturnsBrowserOpenError(t *testing.T) {
	setupOpenConfig(t)
	cfgFile := &files.AWSFile{
		File: ini.Empty(),
		Path: filepath.Join(config.GetInstance().Home, ".aws", "config"),
	}
	section, _ := cfgFile.File.NewSection("profile dev:Admin")
	_, _ = section.NewKey("org", "dev")
	_, _ = section.NewKey("sso_role_name", "Admin")
	_, _ = section.NewKey("sso_account_id", "111111111111")
	_, _ = section.NewKey("sso_start_url", "https://dev.awsapps.com/start")
	_, _ = section.NewKey("region", "us-east-1")

	err := openConsoleWithDeps("dev:Admin", 3600, openDeps{
		newConfigFile: func(string) (*files.AWSFile, error) { return cfgFile, nil },
		getCachedFlow: func(config.Organization) (cachedFlow, error) {
			return fakeCachedFlow{
				creds: &awssso.GetRoleCredentialsOutput{
					RoleCredentials: &awsssotypes.RoleCredentials{
						AccessKeyId:     awsv2.String("AKIA"),
						SecretAccessKey: awsv2.String("secret"),
						SessionToken:    awsv2.String("token"),
					},
				},
			}, nil
		},
		httpGet: func(string) (*http.Response, error) {
			return &http.Response{
				Body: io.NopCloser(strings.NewReader(`{"SigninToken":"signin-token"}`)),
			}, nil
		},
		openURL: func(string) error { return errors.New("no browser") },
	})
	if err == nil || !strings.Contains(err.Error(), "can't open your browser") {
		t.Fatalf("openConsoleWithDeps() error = %v, want browser error", err)
	}
}

func TestOpenConsoleWithDepsReturnsHTTPError(t *testing.T) {
	setupOpenConfig(t)
	cfgFile := &files.AWSFile{
		File: ini.Empty(),
		Path: filepath.Join(config.GetInstance().Home, ".aws", "config"),
	}
	section, _ := cfgFile.File.NewSection("profile dev:Admin")
	_, _ = section.NewKey("org", "dev")
	_, _ = section.NewKey("sso_role_name", "Admin")
	_, _ = section.NewKey("sso_account_id", "111111111111")
	_, _ = section.NewKey("sso_start_url", "https://dev.awsapps.com/start")
	_, _ = section.NewKey("region", "us-east-1")

	err := openConsoleWithDeps("dev:Admin", 3600, openDeps{
		newConfigFile: func(string) (*files.AWSFile, error) { return cfgFile, nil },
		getCachedFlow: func(config.Organization) (cachedFlow, error) {
			return fakeCachedFlow{
				creds: &awssso.GetRoleCredentialsOutput{
					RoleCredentials: &awsssotypes.RoleCredentials{
						AccessKeyId:     awsv2.String("AKIA"),
						SecretAccessKey: awsv2.String("secret"),
						SessionToken:    awsv2.String("token"),
					},
				},
			}, nil
		},
		httpGet: func(string) (*http.Response, error) { return nil, errors.New("down") },
		openURL: func(string) error { return nil },
	})
	if err == nil || !strings.Contains(err.Error(), "unable to login to AWS") {
		t.Fatalf("openConsoleWithDeps() error = %v, want http error", err)
	}
}

func TestOpenConsoleWithDepsReturnsJSONError(t *testing.T) {
	setupOpenConfig(t)
	cfgFile := &files.AWSFile{
		File: ini.Empty(),
		Path: filepath.Join(config.GetInstance().Home, ".aws", "config"),
	}
	section, _ := cfgFile.File.NewSection("profile dev:Admin")
	_, _ = section.NewKey("org", "dev")
	_, _ = section.NewKey("sso_role_name", "Admin")
	_, _ = section.NewKey("sso_account_id", "111111111111")
	_, _ = section.NewKey("sso_start_url", "https://dev.awsapps.com/start")
	_, _ = section.NewKey("region", "us-east-1")

	err := openConsoleWithDeps("dev:Admin", 3600, openDeps{
		newConfigFile: func(string) (*files.AWSFile, error) { return cfgFile, nil },
		getCachedFlow: func(config.Organization) (cachedFlow, error) {
			return fakeCachedFlow{
				creds: &awssso.GetRoleCredentialsOutput{
					RoleCredentials: &awsssotypes.RoleCredentials{
						AccessKeyId:     awsv2.String("AKIA"),
						SecretAccessKey: awsv2.String("secret"),
						SessionToken:    awsv2.String("token"),
					},
				},
			}, nil
		},
		httpGet: func(string) (*http.Response, error) {
			return &http.Response{Body: io.NopCloser(strings.NewReader("not-json"))}, nil
		},
		openURL: func(string) error { return nil },
	})
	if err == nil || !strings.Contains(err.Error(), "error parsing login response") {
		t.Fatalf("openConsoleWithDeps() error = %v, want json error", err)
	}
}

func TestDefaultOpenDepsProvidesFunctions(t *testing.T) {
	deps := defaultOpenDeps()
	if deps.initConfig == nil || deps.getenv == nil || deps.openConsole == nil ||
		deps.httpGet == nil ||
		deps.openURL == nil {
		t.Fatal("defaultOpenDeps() returned nil dependency")
	}
}

func setupOpenConfig(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	if err := config.ResetAndSetTestConfig(home); err != nil {
		t.Fatalf("ResetAndSetTestConfig() error = %v", err)
	}
	config.SetInstanceForTest(&config.Config{
		Home: home,
		Orgs: map[string]config.Organization{
			"dev": {
				Name:   "dev",
				Prefix: "dev",
				URL:    "https://dev.awsapps.com/start",
				Region: "us-east-1",
			},
		},
		ErrorColor:       "#fa0718",
		InformationColor: "#05fa5f",
		WarningColor:     "#f29830",
		FocusColor:       "#4287f5",
		SpinnerColor:     "#42f551",
	})
}
