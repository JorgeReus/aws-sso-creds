package app

import (
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/mikemucc/aws-sso-creds/internal/pkg/files"
)

type SSOFlow struct {
	accessToken     *string
	configFile      *files.AWSFile
	credentialsFile *files.AWSFile
	ssoClient       *sso.Client
	ssoRegion       *string
	ssoStartUrl     *string
	orgName         string
	prefix          string
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
