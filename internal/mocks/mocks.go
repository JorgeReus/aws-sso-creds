package mocks

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sso"
	ssotypes "github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/stretchr/testify/mock"
)

// MockSSOClient mocks the SSO service client
type MockSSOClient struct {
	mock.Mock
}

func (m *MockSSOClient) ListAccounts(ctx context.Context, params *sso.ListAccountsInput, optFns ...func(*sso.Options)) (*sso.ListAccountsOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sso.ListAccountsOutput), args.Error(1)
}

func (m *MockSSOClient) ListAccountRoles(ctx context.Context, params *sso.ListAccountRolesInput, optFns ...func(*sso.Options)) (*sso.ListAccountRolesOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sso.ListAccountRolesOutput), args.Error(1)
}

func (m *MockSSOClient) GetRoleCredentials(ctx context.Context, params *sso.GetRoleCredentialsInput, optFns ...func(*sso.Options)) (*sso.GetRoleCredentialsOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sso.GetRoleCredentialsOutput), args.Error(1)
}

// MockSSOOIDCClient mocks the SSO OIDC service client
type MockSSOOIDCClient struct {
	mock.Mock
}

func (m *MockSSOOIDCClient) RegisterClient(ctx context.Context, params *ssooidc.RegisterClientInput, optFns ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ssooidc.RegisterClientOutput), args.Error(1)
}

func (m *MockSSOOIDCClient) StartDeviceAuthorization(ctx context.Context, params *ssooidc.StartDeviceAuthorizationInput, optFns ...func(*ssooidc.Options)) (*ssooidc.StartDeviceAuthorizationOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ssooidc.StartDeviceAuthorizationOutput), args.Error(1)
}

func (m *MockSSOOIDCClient) CreateToken(ctx context.Context, params *ssooidc.CreateTokenInput, optFns ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ssooidc.CreateTokenOutput), args.Error(1)
}

// Test data builders
func BuildAccountInfo(id, name string) ssotypes.AccountInfo {
	return ssotypes.AccountInfo{
		AccountId:   &id,
		AccountName: &name,
	}
}

func BuildRoleInfo(name, accountID string) ssotypes.RoleInfo {
	return ssotypes.RoleInfo{
		RoleName: &name,
	}
}

func BuildRoleCredentials(accessKey, secretKey, sessionToken string, expiresAt int64) *ssotypes.RoleCredentials {
	return &ssotypes.RoleCredentials{
		AccessKeyId:     &accessKey,
		SecretAccessKey: &secretKey,
		SessionToken:    &sessionToken,
		Expiration:      expiresAt,
	}
}

func BuildRegisterClientOutput(clientID, clientSecret string, expiresAt int64) *ssooidc.RegisterClientOutput {
	return &ssooidc.RegisterClientOutput{
		ClientId:              &clientID,
		ClientSecret:          &clientSecret,
		ClientSecretExpiresAt: expiresAt,
	}
}

func BuildStartDeviceAuthOutput(userCode, deviceCode, verificationURI string, interval int32) *ssooidc.StartDeviceAuthorizationOutput {
	return &ssooidc.StartDeviceAuthorizationOutput{
		UserCode:                &userCode,
		DeviceCode:              &deviceCode,
		VerificationUriComplete: &verificationURI,
		Interval:                interval,
	}
}

func BuildCreateTokenOutput(accessToken string, expiresIn int32) *ssooidc.CreateTokenOutput {
	return &ssooidc.CreateTokenOutput{
		AccessToken: &accessToken,
		ExpiresIn:   expiresIn,
	}
}

func BuildListAccountsOutput(accounts []ssotypes.AccountInfo) *sso.ListAccountsOutput {
	return &sso.ListAccountsOutput{
		AccountList: accounts,
	}
}

func BuildListRolesOutput(roles []ssotypes.RoleInfo) *sso.ListAccountRolesOutput {
	return &sso.ListAccountRolesOutput{
		RoleList: roles,
	}
}

func BuildGetCredentialsOutput(creds *ssotypes.RoleCredentials) *sso.GetRoleCredentialsOutput {
	return &sso.GetRoleCredentialsOutput{
		RoleCredentials: creds,
	}
}
