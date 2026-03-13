package app

import (
	"time"

	"github.com/JorgeReus/aws-sso-creds/internal/pkg/cache"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/files"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sso"
	"github.com/aws/aws-sdk-go/service/ssooidc"
)

type SSOFlow struct {
	accessToken     *string
	configFile      *files.AWSFile
	credentialsFile *files.AWSFile
	ssoClient       ssoClientAPI
	ssoRegion       *string
	ssoStartUrl     *string
	orgName         string
	prefix          string
}

type ssoClientAPI interface {
	ListAccounts(*sso.ListAccountsInput) (*sso.ListAccountsOutput, error)
	ListAccountRoles(*sso.ListAccountRolesInput) (*sso.ListAccountRolesOutput, error)
	GetRoleCredentials(*sso.GetRoleCredentialsInput) (*sso.GetRoleCredentialsOutput, error)
}

type oidcClientAPI interface {
	RegisterClient(*ssooidc.RegisterClientInput) (*ssooidc.RegisterClientOutput, error)
	StartDeviceAuthorization(*ssooidc.StartDeviceAuthorizationInput) (*ssooidc.StartDeviceAuthorizationOutput, error)
	CreateToken(*ssooidc.CreateTokenInput) (*ssooidc.CreateTokenOutput, error)
}

type loginDeps struct {
	newSession         func() *session.Session
	newOIDCClient      func(*session.Session, string) oidcClientAPI
	newSSOClient       func(*session.Session, string) ssoClientAPI
	getClientCreds     func(string) (*cache.SSOClientCredentials, error)
	saveClientCreds    func(*cache.SSOClientCredentials, *string) error
	getToken           func(string, *session.Session, string) (*cache.SSOToken, error)
	saveToken          func(*cache.SSOToken, string) error
	newConfigFile      func(string) (*files.AWSFile, error)
	openURL            func(string) error
	sleep              func(time.Duration)
	now                func() time.Time
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
