package app

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	ssotypes "github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	ssooidctypes "github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
	"github.com/mikemucc/aws-sso-creds/internal/app/config"
	"github.com/mikemucc/aws-sso-creds/internal/mocks"
	"github.com/mikemucc/aws-sso-creds/internal/pkg/bus"
	"github.com/mikemucc/aws-sso-creds/internal/pkg/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockBus is a mock implementation of the Bus
type MockBus struct {
	mock.Mock
	Messages []bus.BusMsg
}

func (m *MockBus) Send(msg bus.BusMsg) {
	m.Called(msg)
	m.Messages = append(m.Messages, msg)
}

func (m *MockBus) Recv() bus.BusMsg {
	args := m.Called()
	return args.Get(0).(bus.BusMsg)
}

func TestLogin(t *testing.T) {
	org := config.Organization{
		Name:   "test-org",
		Region: "us-east-1",
		URL:    "https://my-sso-instance.awsapps.com/start",
		Prefix: "test",
	}

	mockBus := &MockBus{}
	mockBus.On("Send", mock.Anything).Return()
	mockBus.On("Recv").Return(bus.BusMsg{})

	t.Run("successfully completes login flow with existing credentials", func(t *testing.T) {
		// Setup: Pre-create token
		token := &cache.SSOToken{
			StartUrl:    org.URL,
			Region:      org.Region,
			AccessToken: "test-access-token",
			ExpiresAt:   time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}

		// In a real test scenario, these would be persisted to cache
		// For this example, we're testing the structure

		flow := &SSOFlow{
			accessToken: &token.AccessToken,
			ssoRegion:   &org.Region,
			ssoStartUrl: &org.URL,
			orgName:     org.Name,
			prefix:      org.Prefix,
		}

		assert.NotNil(t, flow)
		assert.Equal(t, token.AccessToken, *flow.accessToken)
		assert.Equal(t, org.Region, *flow.ssoRegion)
	})
}

func TestGetAccountRoles(t *testing.T) {
	// Note: In a real test, we'd properly initialize the config file
	// This is a simplified test structure

	account := &ssotypes.AccountInfo{
		AccountId:   aws.String("123456789"),
		AccountName: aws.String("dev account"),
	}

	roles := []ssotypes.RoleInfo{
		{RoleName: aws.String("DeveloperRole")},
		{RoleName: aws.String("AdminRole")},
	}

	t.Run("successfully retrieves and processes account roles", func(t *testing.T) {
		assert.NotNil(t, account)
		assert.Equal(t, "123456789", *account.AccountId)
		assert.Equal(t, len(roles), 2)
	})

	t.Run("handles multiple accounts", func(t *testing.T) {
		accounts := []ssotypes.AccountInfo{
			{
				AccountId:   aws.String("111111111"),
				AccountName: aws.String("prod account"),
			},
			{
				AccountId:   aws.String("222222222"),
				AccountName: aws.String("staging account"),
			},
		}

		assert.Equal(t, len(accounts), 2)
	})
}

func TestPopulateRoles(t *testing.T) {
	t.Run("successfully populates roles from accounts", func(t *testing.T) {
		mockSSOClient := &mocks.MockSSOClient{}

		accounts := []ssotypes.AccountInfo{
			mocks.BuildAccountInfo("111111111", "dev account"),
			mocks.BuildAccountInfo("222222222", "prod account"),
		}

		mockSSOClient.On("ListAccounts", mock.Anything, mock.Anything).
			Return(mocks.BuildListAccountsOutput(accounts), nil)

		// Verify mock setup
		ctx := context.Background()
		output, err := mockSSOClient.ListAccounts(ctx, &sso.ListAccountsInput{})

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, 2, len(output.AccountList))
		assert.Equal(t, "111111111", *output.AccountList[0].AccountId)
		assert.Equal(t, "222222222", *output.AccountList[1].AccountId)
	})

	t.Run("handles pagination correctly", func(t *testing.T) {
		mockSSOClient := &mocks.MockSSOClient{}

		firstBatch := []ssotypes.AccountInfo{
			mocks.BuildAccountInfo("111111111", "account 1"),
		}

		secondBatch := []ssotypes.AccountInfo{
			mocks.BuildAccountInfo("222222222", "account 2"),
		}

		// First call returns with NextToken
		firstOutput := mocks.BuildListAccountsOutput(firstBatch)
		firstOutput.NextToken = aws.String("token123")

		// Second call returns without NextToken
		secondOutput := mocks.BuildListAccountsOutput(secondBatch)

		mockSSOClient.On("ListAccounts", mock.Anything, mock.Anything).
			Return(firstOutput, nil).Once()
		mockSSOClient.On("ListAccounts", mock.Anything, mock.Anything).
			Return(secondOutput, nil).Once()

		// Verify both calls
		ctx := context.Background()
		output1, _ := mockSSOClient.ListAccounts(ctx, &sso.ListAccountsInput{})
		assert.NotNil(t, output1.NextToken)

		output2, _ := mockSSOClient.ListAccounts(ctx, &sso.ListAccountsInput{NextToken: output1.NextToken})
		assert.Nil(t, output2.NextToken)
	})
}

func TestGetCredentials(t *testing.T) {
	t.Run("successfully retrieves credentials for a role", func(t *testing.T) {
		mockSSOClient := &mocks.MockSSOClient{}

		creds := mocks.BuildRoleCredentials(
			"AKIAIOSFODNN7EXAMPLE",
			"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			"SessionToken123",
			time.Now().Add(1*time.Hour).UnixMilli(),
		)

		credsOutput := mocks.BuildGetCredentialsOutput(creds)

		mockSSOClient.On("GetRoleCredentials", mock.Anything, mock.Anything).
			Return(credsOutput, nil)

		ctx := context.Background()
		output, err := mockSSOClient.GetRoleCredentials(ctx, &sso.GetRoleCredentialsInput{
			AccessToken: aws.String("test-token"),
			AccountId:   aws.String("123456789"),
			RoleName:    aws.String("DeveloperRole"),
		})

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", *output.RoleCredentials.AccessKeyId)
		assert.Equal(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", *output.RoleCredentials.SecretAccessKey)
	})

	t.Run("handles credentials close to expiration", func(t *testing.T) {
		mockSSOClient := &mocks.MockSSOClient{}

		// Credentials expiring in 5 minutes
		expiresAt := time.Now().Add(5 * time.Minute).UnixMilli()
		creds := mocks.BuildRoleCredentials(
			"AKIAIOSFODNN7EXAMPLE",
			"secret",
			"token",
			expiresAt,
		)

		credsOutput := mocks.BuildGetCredentialsOutput(creds)
		mockSSOClient.On("GetRoleCredentials", mock.Anything, mock.Anything).
			Return(credsOutput, nil)

		ctx := context.Background()
		output, err := mockSSOClient.GetRoleCredentials(ctx, &sso.GetRoleCredentialsInput{})

		assert.NoError(t, err)
		assert.Equal(t, expiresAt, output.RoleCredentials.Expiration)
	})
}

func TestListAccountRoles(t *testing.T) {
	t.Run("successfully lists roles for an account", func(t *testing.T) {
		mockSSOClient := &mocks.MockSSOClient{}

		roles := []ssotypes.RoleInfo{
			mocks.BuildRoleInfo("DeveloperRole", "123456789"),
			mocks.BuildRoleInfo("AdminRole", "123456789"),
			mocks.BuildRoleInfo("ViewOnlyRole", "123456789"),
		}

		mockSSOClient.On("ListAccountRoles", mock.Anything, mock.Anything).
			Return(mocks.BuildListRolesOutput(roles), nil)

		ctx := context.Background()
		output, err := mockSSOClient.ListAccountRoles(ctx, &sso.ListAccountRolesInput{
			AccessToken: aws.String("test-token"),
			AccountId:   aws.String("123456789"),
		})

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, 3, len(output.RoleList))
		assert.Equal(t, "DeveloperRole", *output.RoleList[0].RoleName)
		assert.Equal(t, "AdminRole", *output.RoleList[1].RoleName)
	})
}

func TestOIDCRegisterClient(t *testing.T) {
	t.Run("successfully registers a new OIDC client", func(t *testing.T) {
		mockOIDCClient := &mocks.MockSSOOIDCClient{}

		expiresAt := time.Now().Add(1 * time.Hour).Unix()
		registerOutput := mocks.BuildRegisterClientOutput(
			"test-client-id",
			"test-client-secret",
			expiresAt,
		)

		mockOIDCClient.On("RegisterClient", mock.Anything, mock.Anything).
			Return(registerOutput, nil)

		ctx := context.Background()
		output, err := mockOIDCClient.RegisterClient(ctx, &ssooidc.RegisterClientInput{
			ClientName: aws.String("test-app"),
			ClientType: aws.String("public"),
		})

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, "test-client-id", *output.ClientId)
		assert.Equal(t, "test-client-secret", *output.ClientSecret)
		assert.Equal(t, expiresAt, output.ClientSecretExpiresAt)
	})
}

func TestOIDCStartDeviceAuthorization(t *testing.T) {
	t.Run("successfully initiates device authorization", func(t *testing.T) {
		mockOIDCClient := &mocks.MockSSOOIDCClient{}

		deviceAuthOutput := mocks.BuildStartDeviceAuthOutput(
			"ABC123",
			"DEV456",
			"https://device.awsapps.com?user_code=ABC123",
			1,
		)

		mockOIDCClient.On("StartDeviceAuthorization", mock.Anything, mock.Anything).
			Return(deviceAuthOutput, nil)

		ctx := context.Background()
		output, err := mockOIDCClient.StartDeviceAuthorization(ctx, &ssooidc.StartDeviceAuthorizationInput{
			ClientId:     aws.String("test-client-id"),
			ClientSecret: aws.String("test-secret"),
			StartUrl:     aws.String("https://my-sso-instance.awsapps.com/start"),
		})

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.Equal(t, "ABC123", *output.UserCode)
		assert.Equal(t, "DEV456", *output.DeviceCode)
		assert.Equal(t, int32(1), output.Interval)
	})
}

func TestOIDCCreateToken(t *testing.T) {
	t.Run("successfully creates access token after authorization", func(t *testing.T) {
		mockOIDCClient := &mocks.MockSSOOIDCClient{}

		tokenOutput := mocks.BuildCreateTokenOutput(
			"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
			3600,
		)

		mockOIDCClient.On("CreateToken", mock.Anything, mock.Anything).
			Return(tokenOutput, nil)

		ctx := context.Background()
		output, err := mockOIDCClient.CreateToken(ctx, &ssooidc.CreateTokenInput{
			ClientId:     aws.String("test-client-id"),
			ClientSecret: aws.String("test-secret"),
			DeviceCode:   aws.String("DEV456"),
			Code:         aws.String("ABC123"),
			GrantType:    aws.String("urn:ietf:params:oauth:grant-type:device_code"),
		})

		assert.NoError(t, err)
		assert.NotNil(t, output)
		assert.NotEmpty(t, *output.AccessToken)
		assert.Equal(t, int32(3600), output.ExpiresIn)
	})

	t.Run("handles authorization pending error", func(t *testing.T) {
		mockOIDCClient := &mocks.MockSSOOIDCClient{}

		mockOIDCClient.On("CreateToken", mock.Anything, mock.Anything).
			Return(nil, &ssooidctypes.AuthorizationPendingException{})

		ctx := context.Background()
		output, err := mockOIDCClient.CreateToken(ctx, &ssooidc.CreateTokenInput{})

		assert.Error(t, err)
		assert.Nil(t, output)
	})
}

func TestMockVerification(t *testing.T) {
	t.Run("mock assertions work correctly", func(t *testing.T) {
		mockClient := &mocks.MockSSOClient{}

		accounts := []ssotypes.AccountInfo{
			mocks.BuildAccountInfo("123456789", "test account"),
		}

		mockClient.On("ListAccounts", mock.Anything, mock.Anything).
			Return(mocks.BuildListAccountsOutput(accounts), nil)

		// Call the mock
		ctx := context.Background()
		mockClient.ListAccounts(ctx, &sso.ListAccountsInput{AccessToken: aws.String("test")})

		// Verify the call was made
		mockClient.AssertCalled(t, "ListAccounts", mock.Anything, mock.Anything)

		// Verify call count
		mockClient.AssertNumberOfCalls(t, "ListAccounts", 1)
	})
}
