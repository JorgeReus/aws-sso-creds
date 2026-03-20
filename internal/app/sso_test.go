package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/JorgeReus/aws-sso-creds/internal/app/config"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/bus"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/cache"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/files"
	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	ssotypes "github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	ssooidctypes "github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
	"gopkg.in/ini.v1"
)

type fakeOIDCClient struct {
	registerOut *ssooidc.RegisterClientOutput
	registerErr error
	startOut    *ssooidc.StartDeviceAuthorizationOutput
	startErr    error
	tokenOuts   []*ssooidc.CreateTokenOutput
	tokenErrs   []error
	tokenCalls  int
}

func (f *fakeOIDCClient) RegisterClient(context.Context, *ssooidc.RegisterClientInput, ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error) {
	return f.registerOut, f.registerErr
}

func (f *fakeOIDCClient) StartDeviceAuthorization(context.Context, *ssooidc.StartDeviceAuthorizationInput, ...func(*ssooidc.Options)) (*ssooidc.StartDeviceAuthorizationOutput, error) {
	return f.startOut, f.startErr
}

func (f *fakeOIDCClient) CreateToken(context.Context, *ssooidc.CreateTokenInput, ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error) {
	idx := f.tokenCalls
	f.tokenCalls++
	if idx < len(f.tokenErrs) && f.tokenErrs[idx] != nil {
		return nil, f.tokenErrs[idx]
	}
	if idx < len(f.tokenOuts) {
		return f.tokenOuts[idx], nil
	}
	return nil, errors.New("unexpected CreateToken call")
}

type fakeSSOClient struct {
	listAccountsOutputs []*sso.ListAccountsOutput
	listAccountsErr     error
	listAccountsCalls   int

	listAccountRoles map[string][]*sso.ListAccountRolesOutput
	listRoleErr      error

	roleCredsByRole map[string]*sso.GetRoleCredentialsOutput
	roleCredsErrs   map[string]error
}

func (f *fakeSSOClient) ListAccounts(context.Context, *sso.ListAccountsInput, ...func(*sso.Options)) (*sso.ListAccountsOutput, error) {
	if f.listAccountsErr != nil {
		return nil, f.listAccountsErr
	}
	idx := f.listAccountsCalls
	f.listAccountsCalls++
	if idx < len(f.listAccountsOutputs) {
		return f.listAccountsOutputs[idx], nil
	}
	return &sso.ListAccountsOutput{}, nil
}

func (f *fakeSSOClient) ListAccountRoles(_ context.Context, input *sso.ListAccountRolesInput, _ ...func(*sso.Options)) (*sso.ListAccountRolesOutput, error) {
	if f.listRoleErr != nil {
		return nil, f.listRoleErr
	}
	accountID := awsv2.ToString(input.AccountId)
	responses := f.listAccountRoles[accountID]
	if len(responses) == 0 {
		return &sso.ListAccountRolesOutput{}, nil
	}
	resp := responses[0]
	f.listAccountRoles[accountID] = responses[1:]
	return resp, nil
}

func (f *fakeSSOClient) GetRoleCredentials(_ context.Context, input *sso.GetRoleCredentialsInput, _ ...func(*sso.Options)) (*sso.GetRoleCredentialsOutput, error) {
	roleName := awsv2.ToString(input.RoleName)
	if err, ok := f.roleCredsErrs[roleName]; ok {
		return nil, err
	}
	if creds, ok := f.roleCredsByRole[roleName]; ok {
		return creds, nil
	}
	return nil, errors.New("missing role credentials")
}

func TestLoginUsesCachedCredentialsAndTokenWithoutRegisteringClient(t *testing.T) {
	home := setupAppConfig(t)
	future := time.Now().Add(time.Hour).Format(time.RFC3339)
	deps := defaultFakeLoginDeps(home)
	oidcFactoryCalls := 0
	deps.newOIDCClient = func(context.Context, string) (oidcClientAPI, error) {
		oidcFactoryCalls++
		return nil, errors.New("oidc config failed")
	}
	deps.getClientCreds = func(string) (*cache.SSOClientCredentials, error) {
		return &cache.SSOClientCredentials{ClientId: "id", ClientSecret: "secret", ExpiresAt: future}, nil
	}
	deps.getToken = func(string, string) (*cache.SSOToken, error) {
		return &cache.SSOToken{AccessToken: "token", ExpiresAt: future}, nil
	}

	msgBus := &bus.Bus{Channel: make(chan bus.BusMsg, 10)}
	flow, err := loginWithDeps(validOrg(), false, false, msgBus, deps)
	if err != nil {
		t.Fatalf("loginWithDeps() error = %v", err)
	}
	if flow == nil {
		t.Fatal("loginWithDeps() = nil")
	}
	if flow.ssoClient == nil {
		t.Fatal("flow.ssoClient = nil")
	}
	if oidcFactoryCalls != 0 {
		t.Fatalf("newOIDCClient calls = %d, want 0", oidcFactoryCalls)
	}
}

func TestLoginForceAuthRegistersClientAndPollsUntilTokenIssued(t *testing.T) {
	home := setupAppConfig(t)
	now := time.Now()
	oidc := &fakeOIDCClient{
		registerOut: &ssooidc.RegisterClientOutput{
			ClientId:              awsv2.String("id"),
			ClientSecret:          awsv2.String("secret"),
			ClientSecretExpiresAt: now.Add(time.Hour).Unix(),
		},
		startOut: &ssooidc.StartDeviceAuthorizationOutput{
			UserCode:                awsv2.String("ABCD"),
			VerificationUriComplete: awsv2.String("https://verify"),
			Interval:                1,
			DeviceCode:              awsv2.String("device"),
		},
		tokenErrs: []error{&ssooidctypes.AuthorizationPendingException{Message: awsv2.String("pending")}},
		tokenOuts: []*ssooidc.CreateTokenOutput{
			nil,
			{AccessToken: awsv2.String("fresh-token"), ExpiresIn: 3600},
		},
	}
	var savedClient, savedToken bool
	sleepCalls := 0
	deps := defaultFakeLoginDeps(home)
	deps.newOIDCClient = func(context.Context, string) (oidcClientAPI, error) { return oidc, nil }
	deps.getClientCreds = func(string) (*cache.SSOClientCredentials, error) { return nil, nil }
	deps.getToken = func(string, string) (*cache.SSOToken, error) { return nil, nil }
	deps.saveClientCreds = func(*cache.SSOClientCredentials, *string) error {
		savedClient = true
		return nil
	}
	deps.saveToken = func(*cache.SSOToken, string) error {
		savedToken = true
		return nil
	}
	deps.openURL = func(string) error { return nil }
	deps.sleep = func(time.Duration) { sleepCalls++ }
	deps.now = func() time.Time { return now }

	msgBus := &bus.Bus{Channel: make(chan bus.BusMsg, 10)}
	flow, err := loginWithDeps(validOrg(), true, false, msgBus, deps)
	if err != nil {
		t.Fatalf("loginWithDeps() error = %v", err)
	}
	if flow == nil {
		t.Fatal("loginWithDeps() = nil")
	}
	if !savedClient || !savedToken {
		t.Fatalf("savedClient=%v savedToken=%v, want both true", savedClient, savedToken)
	}
	if oidc.tokenCalls != 2 {
		t.Fatalf("CreateToken calls = %d, want 2", oidc.tokenCalls)
	}
	if sleepCalls != 2 {
		t.Fatalf("sleepCalls = %d, want 2", sleepCalls)
	}
}

func TestLoginSendsBrowserFallbackMessage(t *testing.T) {
	home := setupAppConfig(t)
	now := time.Now()
	oidc := &fakeOIDCClient{
		registerOut: &ssooidc.RegisterClientOutput{
			ClientId:              awsv2.String("id"),
			ClientSecret:          awsv2.String("secret"),
			ClientSecretExpiresAt: now.Add(time.Hour).Unix(),
		},
		startOut: &ssooidc.StartDeviceAuthorizationOutput{
			UserCode:                awsv2.String("ABCD"),
			VerificationUriComplete: awsv2.String("https://verify"),
			Interval:                1,
			DeviceCode:              awsv2.String("device"),
		},
		tokenOuts: []*ssooidc.CreateTokenOutput{
			{AccessToken: awsv2.String("fresh-token"), ExpiresIn: 3600},
		},
	}
	deps := defaultFakeLoginDeps(home)
	deps.newOIDCClient = func(context.Context, string) (oidcClientAPI, error) { return oidc, nil }
	deps.getClientCreds = func(string) (*cache.SSOClientCredentials, error) { return nil, nil }
	deps.getToken = func(string, string) (*cache.SSOToken, error) { return nil, nil }
	deps.openURL = func(string) error { return errors.New("open failed") }
	deps.sleep = func(time.Duration) {}
	deps.now = func() time.Time { return now }

	msgBus := &bus.Bus{Channel: make(chan bus.BusMsg, 10)}
	done := make(chan string, 1)
	go func() {
		for {
			msg := msgBus.Recv()
			if msg.MsgType == bus.MSG_TYPE_ERR {
				done <- msg.Contents
				msgBus.Send(bus.BusMsg{MsgType: bus.MSG_TYPE_CONT})
				return
			}
		}
	}()

	if _, err := loginWithDeps(validOrg(), true, false, msgBus, deps); err != nil {
		t.Fatalf("loginWithDeps() error = %v", err)
	}
	if got := <-done; !strings.Contains(got, "Can't open your browser") {
		t.Fatalf("fallback message = %q, want browser warning", got)
	}
}

func TestGetCachedSSOFlowReturnsErrorWhenTokenMissing(t *testing.T) {
	home := setupAppConfig(t)
	deps := defaultFakeLoginDeps(home)
	deps.getClientCreds = func(string) (*cache.SSOClientCredentials, error) {
		return &cache.SSOClientCredentials{ClientId: "id", ClientSecret: "secret", ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339)}, nil
	}
	deps.getToken = func(string, string) (*cache.SSOToken, error) { return nil, nil }

	_, err := getCachedSSOFlowWithDeps(validOrg(), deps)
	if err == nil || !strings.Contains(err.Error(), "Unable to get sso token") {
		t.Fatalf("getCachedSSOFlowWithDeps() error = %v, want missing token error", err)
	}
}

func TestLoginReturnsRegisterClientError(t *testing.T) {
	home := setupAppConfig(t)
	deps := defaultFakeLoginDeps(home)
	deps.newOIDCClient = func(context.Context, string) (oidcClientAPI, error) {
		return &fakeOIDCClient{registerErr: errors.New("register failed")}, nil
	}
	deps.getClientCreds = func(string) (*cache.SSOClientCredentials, error) { return nil, nil }

	_, err := loginWithDeps(validOrg(), true, false, &bus.Bus{Channel: make(chan bus.BusMsg, 10)}, deps)
	if err == nil || err.Error() != "register failed" {
		t.Fatalf("loginWithDeps() error = %v, want register failed", err)
	}
}

func TestLoginReturnsCreateTokenError(t *testing.T) {
	home := setupAppConfig(t)
	now := time.Now()
	deps := defaultFakeLoginDeps(home)
	deps.newOIDCClient = func(context.Context, string) (oidcClientAPI, error) {
		return &fakeOIDCClient{
			registerOut: &ssooidc.RegisterClientOutput{
				ClientId:              awsv2.String("id"),
				ClientSecret:          awsv2.String("secret"),
				ClientSecretExpiresAt: now.Add(time.Hour).Unix(),
			},
			startOut: &ssooidc.StartDeviceAuthorizationOutput{
				UserCode:                awsv2.String("ABCD"),
				VerificationUriComplete: awsv2.String("https://verify"),
				Interval:                1,
				DeviceCode:              awsv2.String("device"),
			},
			tokenErrs: []error{errors.New("token failed")},
		}, nil
	}
	deps.getClientCreds = func(string) (*cache.SSOClientCredentials, error) { return nil, nil }
	deps.getToken = func(string, string) (*cache.SSOToken, error) { return nil, nil }
	deps.sleep = func(time.Duration) {}

	_, err := loginWithDeps(validOrg(), true, false, &bus.Bus{Channel: make(chan bus.BusMsg, 10)}, deps)
	if err == nil || err.Error() != "token failed" {
		t.Fatalf("loginWithDeps() error = %v, want token failed", err)
	}
}

func TestLoginReturnsConfigFileErrorAfterAuth(t *testing.T) {
	home := setupAppConfig(t)
	deps := defaultFakeLoginDeps(home)
	deps.getClientCreds = func(string) (*cache.SSOClientCredentials, error) {
		return &cache.SSOClientCredentials{ClientId: "id", ClientSecret: "secret", ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339)}, nil
	}
	deps.getToken = func(string, string) (*cache.SSOToken, error) {
		return &cache.SSOToken{AccessToken: "token", ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339)}, nil
	}
	deps.newConfigFile = func(string) (*files.AWSFile, error) { return nil, errors.New("config file failed") }

	_, err := loginWithDeps(validOrg(), false, false, &bus.Bus{Channel: make(chan bus.BusMsg, 10)}, deps)
	if err == nil || err.Error() != "config file failed" {
		t.Fatalf("loginWithDeps() error = %v, want config file failed", err)
	}
}

func TestPopulateRolesCreatesConfigSectionsForPagedAccountsAndRoles(t *testing.T) {
	setupAppConfig(t)
	cfgFile := newAWSFile(t)
	ssoClient := &fakeSSOClient{
		listAccountsOutputs: []*sso.ListAccountsOutput{
			{
				AccountList: []ssotypes.AccountInfo{{AccountId: awsv2.String("111111111111"), AccountName: awsv2.String("Dev Account")}},
				NextToken:   awsv2.String("next"),
			},
			{
				AccountList: []ssotypes.AccountInfo{{AccountId: awsv2.String("222222222222"), AccountName: awsv2.String("Prod Account")}},
			},
		},
		listAccountRoles: map[string][]*sso.ListAccountRolesOutput{
			"111111111111": {
				{
					RoleList:  []ssotypes.RoleInfo{{RoleName: awsv2.String("Admin")}},
					NextToken: awsv2.String("role-next"),
				},
				{
					RoleList: []ssotypes.RoleInfo{{RoleName: awsv2.String("Audit")}},
				},
			},
			"222222222222": {{RoleList: []ssotypes.RoleInfo{{RoleName: awsv2.String("ReadOnly")}}}},
		},
	}
	flow := newTestFlow(cfgFile, ssoClient)

	got, err := flow.PopulateRoles()
	if err != nil {
		t.Fatalf("PopulateRoles() error = %v", err)
	}
	sort.Strings(got)
	want := []string{"dev:Dev-Account:Admin", "dev:Dev-Account:Audit", "dev:Prod-Account:ReadOnly"}
	if len(got) != len(want) {
		t.Fatalf("PopulateRoles() len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("PopulateRoles()[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	section, err := cfgFile.File.GetSection("profile dev:Dev-Account:Admin")
	if err != nil {
		t.Fatalf("GetSection() error = %v", err)
	}
	if got := section.Key("sso_region").String(); got != "us-east-1" {
		t.Fatalf("sso_region = %q, want %q", got, "us-east-1")
	}
	if got := section.Key("region").String(); got != "us-east-1" {
		t.Fatalf("region = %q, want %q", got, "us-east-1")
	}
}

func TestPopulateRolesUsesDefaultRegionWhenProvided(t *testing.T) {
	setupAppConfig(t)
	cfgFile := newAWSFile(t)
	ssoClient := &fakeSSOClient{
		listAccountsOutputs: []*sso.ListAccountsOutput{
			{
				AccountList: []ssotypes.AccountInfo{{AccountId: awsv2.String("111111111111"), AccountName: awsv2.String("Dev Account")}},
			},
		},
		listAccountRoles: map[string][]*sso.ListAccountRolesOutput{
			"111111111111": {{RoleList: []ssotypes.RoleInfo{{RoleName: awsv2.String("Admin")}}}},
		},
	}
	flow := newTestFlow(cfgFile, ssoClient)
	defaultRegion := "eu-west-1"
	flow.defaultRegion = &defaultRegion

	_, err := flow.PopulateRoles()
	if err != nil {
		t.Fatalf("PopulateRoles() error = %v", err)
	}

	section, err := cfgFile.File.GetSection("profile dev:Dev-Account:Admin")
	if err != nil {
		t.Fatalf("GetSection() error = %v", err)
	}
	if got := section.Key("sso_region").String(); got != "us-east-1" {
		t.Fatalf("sso_region = %q, want %q", got, "us-east-1")
	}
	if got := section.Key("region").String(); got != "eu-west-1" {
		t.Fatalf("region = %q, want %q", got, "eu-west-1")
	}
}

func TestPopulateRolesReturnsListAccountsError(t *testing.T) {
	setupAppConfig(t)
	flow := newTestFlow(newAWSFile(t), &fakeSSOClient{listAccountsErr: errors.New("accounts failed")})

	_, err := flow.PopulateRoles()
	if err == nil || err.Error() != "accounts failed" {
		t.Fatalf("PopulateRoles() error = %v, want accounts failed", err)
	}
}

func TestGetCredentialsWritesTemporaryProfiles(t *testing.T) {
	home := setupAppConfig(t)
	cfgFile := newAWSFile(t)
	section, _ := cfgFile.File.NewSection("profile dev:Dev-Account:Admin")
	_, _ = section.NewKey("org", "dev")
	_, _ = section.NewKey("sso_auto_populated", "true")
	_, _ = section.NewKey("sso_account_id", "111111111111")
	_, _ = section.NewKey("sso_role_name", "Admin")
	ssoClient := &fakeSSOClient{
		roleCredsByRole: map[string]*sso.GetRoleCredentialsOutput{
			"Admin": {
				RoleCredentials: &ssotypes.RoleCredentials{
					AccessKeyId:     awsv2.String("AKIA"),
					SecretAccessKey: awsv2.String("secret"),
					SessionToken:    awsv2.String("token"),
					Expiration:      time.Now().Add(time.Hour).UnixMilli(),
				},
			},
		},
	}
	flow := newTestFlow(cfgFile, ssoClient)

	got, err := flow.GetCredentials()
	if err != nil {
		t.Fatalf("GetCredentials() error = %v", err)
	}
	if len(got) != 1 || !got[0].WasSuccesful || !strings.HasPrefix(got[0].ProfileName, "tmp:") {
		t.Fatalf("GetCredentials() = %#v, want successful temp credential entry", got)
	}

	credsFile, err := files.NewCredentialsFile(home)
	if err != nil {
		t.Fatalf("NewCredentialsFile() error = %v", err)
	}
	if _, err := credsFile.File.GetSection("tmp:dev:Dev-Account:Admin"); err != nil {
		t.Fatalf("expected temp credentials section: %v", err)
	}
}

func TestGetCredentialsMarksFailedRoleFetches(t *testing.T) {
	setupAppConfig(t)
	cfgFile := newAWSFile(t)
	section, _ := cfgFile.File.NewSection("profile dev:Dev-Account:Admin")
	_, _ = section.NewKey("org", "dev")
	_, _ = section.NewKey("sso_auto_populated", "true")
	_, _ = section.NewKey("sso_account_id", "111111111111")
	_, _ = section.NewKey("sso_role_name", "Admin")
	flow := newTestFlow(cfgFile, &fakeSSOClient{
		roleCredsErrs: map[string]error{"Admin": errors.New("boom")},
	})

	got, err := flow.GetCredentials()
	if err != nil {
		t.Fatalf("GetCredentials() error = %v", err)
	}
	if len(got) != 1 || got[0].WasSuccesful {
		t.Fatalf("GetCredentials() = %#v, want failed result", got)
	}
}

func TestGetRoleCredsReturnsClientError(t *testing.T) {
	setupAppConfig(t)
	cfgFile := newAWSFile(t)
	flow := newTestFlow(cfgFile, &fakeSSOClient{
		roleCredsErrs: map[string]error{"Admin": errors.New("boom")},
	})

	_, err := flow.GetCredsByRoleName("Admin", "111111111111")
	if err == nil || err.Error() != "boom" {
		t.Fatalf("GetCredsByRoleName() error = %v, want boom", err)
	}
}

func TestGetCachedSSOFlowHappyPath(t *testing.T) {
	home := setupAppConfig(t)
	deps := defaultFakeLoginDeps(home)
	deps.getClientCreds = func(string) (*cache.SSOClientCredentials, error) {
		return &cache.SSOClientCredentials{ClientId: "id", ClientSecret: "secret", ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339)}, nil
	}
	deps.getToken = func(string, string) (*cache.SSOToken, error) {
		return &cache.SSOToken{AccessToken: "token", ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339)}, nil
	}

	flow, err := getCachedSSOFlowWithDeps(validOrg(), deps)
	if err != nil {
		t.Fatalf("getCachedSSOFlowWithDeps() error = %v", err)
	}
	if flow == nil || flow.accessToken == nil || *flow.accessToken != "token" {
		t.Fatalf("getCachedSSOFlowWithDeps() = %#v, want token-backed flow", flow)
	}
}

func TestGetCachedSSOFlowReturnsMissingClientCredentialsError(t *testing.T) {
	home := setupAppConfig(t)
	deps := defaultFakeLoginDeps(home)
	deps.getClientCreds = func(string) (*cache.SSOClientCredentials, error) { return nil, nil }

	_, err := getCachedSSOFlowWithDeps(validOrg(), deps)
	if err == nil || !strings.Contains(err.Error(), "Unable to get client credentials") {
		t.Fatalf("getCachedSSOFlowWithDeps() error = %v, want missing creds error", err)
	}
}

func TestLoginWrapperUsesFactoryDeps(t *testing.T) {
	home := setupAppConfig(t)
	origFactory := loginDepsFactory
	defer func() { loginDepsFactory = origFactory }()
	loginDepsFactory = func() loginDeps {
		deps := defaultFakeLoginDeps(home)
		deps.getClientCreds = func(string) (*cache.SSOClientCredentials, error) {
			return &cache.SSOClientCredentials{ClientId: "id", ClientSecret: "secret", ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339)}, nil
		}
		deps.getToken = func(string, string) (*cache.SSOToken, error) {
			return &cache.SSOToken{AccessToken: "token", ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339)}, nil
		}
		return deps
	}

	flow, err := Login(validOrg(), false, false, &bus.Bus{Channel: make(chan bus.BusMsg, 10)})
	if err != nil || flow == nil {
		t.Fatalf("Login() flow=%#v err=%v, want success", flow, err)
	}
}

func TestGetCachedSSOFlowWrapperUsesFactoryDeps(t *testing.T) {
	home := setupAppConfig(t)
	origFactory := loginDepsFactory
	defer func() { loginDepsFactory = origFactory }()
	loginDepsFactory = func() loginDeps {
		deps := defaultFakeLoginDeps(home)
		deps.getClientCreds = func(string) (*cache.SSOClientCredentials, error) {
			return &cache.SSOClientCredentials{ClientId: "id", ClientSecret: "secret", ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339)}, nil
		}
		deps.getToken = func(string, string) (*cache.SSOToken, error) {
			return &cache.SSOToken{AccessToken: "token", ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339)}, nil
		}
		return deps
	}

	flow, err := GetCachedSSOFlow(validOrg())
	if err != nil || flow == nil {
		t.Fatalf("GetCachedSSOFlow() flow=%#v err=%v, want success", flow, err)
	}
}

func defaultFakeLoginDeps(home string) loginDeps {
	return loginDeps{
		newOIDCClient: func(context.Context, string) (oidcClientAPI, error) {
			return &fakeOIDCClient{}, nil
		},
		newSSOClient: func(context.Context, string) (ssoClientAPI, error) {
			return &fakeSSOClient{}, nil
		},
		getClientCreds: func(string) (*cache.SSOClientCredentials, error) { return nil, nil },
		saveClientCreds: func(*cache.SSOClientCredentials, *string) error {
			return nil
		},
		getToken:  func(string, string) (*cache.SSOToken, error) { return nil, nil },
		saveToken: func(*cache.SSOToken, string) error { return nil },
		newConfigFile: func(string) (*files.AWSFile, error) {
			return &files.AWSFile{File: ini.Empty(), Path: filepath.Join(home, ".aws", "config")}, nil
		},
		openURL: func(string) error { return nil },
		sleep:   func(time.Duration) {},
		now:     time.Now,
	}
}

func setupAppConfig(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".aws"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := config.ResetAndSetTestConfig(home); err != nil {
		t.Fatalf("ResetAndSetTestConfig() error = %v", err)
	}
	return home
}

func newAWSFile(t *testing.T) *files.AWSFile {
	t.Helper()
	path := filepath.Join(config.GetInstance().Home, ".aws", "config")
	return &files.AWSFile{File: ini.Empty(), Path: path}
}

func newTestFlow(cfgFile *files.AWSFile, client ssoClientAPI) *SSOFlow {
	token := "token"
	region := "us-east-1"
	defaultRegion := "us-east-1"
	startURL := "https://dev.awsapps.com/start"
	return &SSOFlow{
		accessToken:   &token,
		ssoClient:     client,
		configFile:    cfgFile,
		ssoRegion:     &region,
		defaultRegion: &defaultRegion,
		ssoStartUrl:   &startURL,
		orgName:       "dev",
		prefix:        "dev",
	}
}

func validOrg() config.Organization {
	return config.Organization{
		Name:      "dev",
		Prefix:    "dev",
		URL:       "https://dev.awsapps.com/start",
		SSORegion: "us-east-1",
	}
}
