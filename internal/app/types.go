package app

import (
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/files"
	"github.com/aws/aws-sdk-go/service/sso"
)

type SSOFlow struct {
	accessToken     *string
	configFile      *files.AWSFile
	credentialsFile *files.AWSFile
	ssoClient       *sso.SSO
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

var clientName = "sso-oidc"
var clientType = "public"
var grantType = "urn:ietf:params:oauth:grant-type:device_code"
