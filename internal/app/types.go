package app

import (
	"context"
	"time"

	"github.com/JorgeReus/aws-sso-creds/internal/pkg/cache"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/files"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	ssotypes "github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
)

type SSOFlow struct {
	accessToken     *string
	configFile      *files.AWSFile
	credentialsFile *files.AWSFile
	ssoClient       ssoClientAPI
	ssoRegion       *string
	defaultRegion   *string
	ssoStartUrl     *string
	orgName         string
	prefix          string
}

type ssoClientAPI interface {
	ListAccounts(context.Context, *sso.ListAccountsInput, ...func(*sso.Options)) (*sso.ListAccountsOutput, error)
	ListAccountRoles(context.Context, *sso.ListAccountRolesInput, ...func(*sso.Options)) (*sso.ListAccountRolesOutput, error)
	GetRoleCredentials(context.Context, *sso.GetRoleCredentialsInput, ...func(*sso.Options)) (*sso.GetRoleCredentialsOutput, error)
}

type oidcClientAPI interface {
	RegisterClient(context.Context, *ssooidc.RegisterClientInput, ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error)
	StartDeviceAuthorization(context.Context, *ssooidc.StartDeviceAuthorizationInput, ...func(*ssooidc.Options)) (*ssooidc.StartDeviceAuthorizationOutput, error)
	CreateToken(context.Context, *ssooidc.CreateTokenInput, ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error)
}

type loginDeps struct {
	newOIDCClient   func(context.Context, string) (oidcClientAPI, error)
	newSSOClient    func(context.Context, string) (ssoClientAPI, error)
	getClientCreds  func(string) (*cache.SSOClientCredentials, error)
	saveClientCreds func(*cache.SSOClientCredentials, *string) error
	getToken        func(string, string) (*cache.SSOToken, error)
	saveToken       func(*cache.SSOToken, string) error
	newConfigFile   func(string) (*files.AWSFile, error)
	openURL         func(string) error
	sleep           func(time.Duration)
	now             func() time.Time
}

type AccountRolesOutput struct {
	creds *sso.ListAccountRolesOutput
	err   error
}

type CredentialsResult struct {
	ProfileName  string
	ExpiresAt    string
	WasSuccesful bool
}

type RoleCredentialsOutput struct {
	creds    *sso.GetRoleCredentialsOutput
	roleName string
	err      error
}

type ssoAccountInfo = ssotypes.AccountInfo

type SessionUrlParams struct {
	AccessKeyId     string `json:"sessionId"`
	SecretAccessKey string `json:"sessionKey"`
	SessionToken    string `json:"sessionToken"`
}

type LoginResponse struct {
	SigninToken string `json:"SigninToken"`
}

type LoginUrlParams struct {
	Issuer      string
	Destination string
	SigninToken string
}

const AWS_FEDERATED_URL = "https://signin.aws.amazon.com/federation"

var (
	clientName = "sso-oidc"
	clientType = "public"
	grantType  = "urn:ietf:params:oauth:grant-type:device_code"
)
