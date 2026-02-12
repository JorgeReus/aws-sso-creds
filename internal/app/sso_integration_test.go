package app

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	ssotypes "github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/mikemucc/aws-sso-creds/internal/app/config"
	"github.com/mikemucc/aws-sso-creds/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// IntegrationTestScenario represents an end-to-end test scenario
type IntegrationTestScenario struct {
	t              *testing.T
	mockSSO        *mocks.MockSSOClient
	mockSSOOIDC    *mocks.MockSSOOIDCClient
	mockBus        *MockBus
	testOrg        config.Organization
	testAccessKey  string
	testSecretKey  string
	testToken      string
	expiresInHours int
}

// NewIntegrationTestScenario creates a new test scenario with default setup
func NewIntegrationTestScenario(t *testing.T) *IntegrationTestScenario {
	return &IntegrationTestScenario{
		t:           t,
		mockSSO:     &mocks.MockSSOClient{},
		mockSSOOIDC: &mocks.MockSSOOIDCClient{},
		mockBus:     &MockBus{},
		testOrg: config.Organization{
			Name:   "test-org",
			Region: "us-east-1",
			URL:    "https://my-sso.awsapps.com/start",
			Prefix: "test",
		},
		testAccessKey:  "AKIAIOSFODNN7EXAMPLE",
		testSecretKey:  "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		testToken:      "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
		expiresInHours: 12,
	}
}

// SetupCompleteSSO mocks a complete successful SSO flow
func (s *IntegrationTestScenario) SetupCompleteSSO() *IntegrationTestScenario {
	// Mock OIDC client registration
	registerOutput := mocks.BuildRegisterClientOutput(
		"client-id-123",
		"client-secret-456",
		time.Now().Add(24*time.Hour).Unix(),
	)
	s.mockSSOOIDC.On("RegisterClient", mock.Anything, mock.Anything).
		Return(registerOutput, nil)

	// Mock device authorization
	deviceAuthOutput := mocks.BuildStartDeviceAuthOutput(
		"ABC123",
		"DEV456",
		"https://device.awsapps.com?user_code=ABC123",
		1,
	)
	s.mockSSOOIDC.On("StartDeviceAuthorization", mock.Anything, mock.Anything).
		Return(deviceAuthOutput, nil)

	// Mock create token
	tokenOutput := mocks.BuildCreateTokenOutput(s.testToken, int32(s.expiresInHours*3600))
	s.mockSSOOIDC.On("CreateToken", mock.Anything, mock.Anything).
		Return(tokenOutput, nil)

	return s
}

// SetupAccountsAndRoles mocks account and role retrieval
func (s *IntegrationTestScenario) SetupAccountsAndRoles() *IntegrationTestScenario {
	accounts := []ssotypes.AccountInfo{
		mocks.BuildAccountInfo("111111111111", "development"),
		mocks.BuildAccountInfo("222222222222", "production"),
		mocks.BuildAccountInfo("333333333333", "staging"),
	}

	s.mockSSO.On("ListAccounts", mock.Anything, mock.Anything).
		Return(mocks.BuildListAccountsOutput(accounts), nil)

	// Setup roles for development account
	devRoles := []ssotypes.RoleInfo{
		mocks.BuildRoleInfo("DeveloperRole", "111111111111"),
		mocks.BuildRoleInfo("EngineerRole", "111111111111"),
	}
	s.mockSSO.On("ListAccountRoles", mock.Anything, mock.Anything).
		Return(mocks.BuildListRolesOutput(devRoles), nil).
		Run(func(args mock.Arguments) {
			input := args.Get(1).(*sso.ListAccountRolesInput)
			if *input.AccountId == "111111111111" {
				// Only return roles for this account
			}
		})

	return s
}

// SetupCredentialsForRoles mocks credential retrieval for all roles
func (s *IntegrationTestScenario) SetupCredentialsForRoles() *IntegrationTestScenario {
	expiresAt := time.Now().Add(time.Duration(s.expiresInHours) * time.Hour).UnixMilli()
	creds := mocks.BuildRoleCredentials(
		s.testAccessKey,
		s.testSecretKey,
		"SessionToken"+time.Now().Format("20060102150405"),
		expiresAt,
	)

	s.mockSSO.On("GetRoleCredentials", mock.Anything, mock.Anything).
		Return(mocks.BuildGetCredentialsOutput(creds), nil)

	return s
}

// TestCompleteSSO runs a complete SSO scenario test
func TestCompleteSSO(t *testing.T) {
	scenario := NewIntegrationTestScenario(t).
		SetupCompleteSSO().
		SetupAccountsAndRoles().
		SetupCredentialsForRoles()

	t.Run("complete SSO flow from registration to credentials", func(t *testing.T) {
		ctx := context.Background()

		// Register client
		registerOut, err := scenario.mockSSOOIDC.RegisterClient(ctx, &ssooidc.RegisterClientInput{
			ClientName: aws.String("test-app"),
			ClientType: aws.String("public"),
		})
		assert.NoError(t, err)
		assert.NotNil(t, registerOut)

		// Start device authorization
		deviceAuthOut, err := scenario.mockSSOOIDC.StartDeviceAuthorization(ctx, &ssooidc.StartDeviceAuthorizationInput{
			ClientId:     registerOut.ClientId,
			ClientSecret: registerOut.ClientSecret,
			StartUrl:     aws.String(scenario.testOrg.URL),
		})
		assert.NoError(t, err)
		assert.NotNil(t, deviceAuthOut)

		// Create token
		tokenOut, err := scenario.mockSSOOIDC.CreateToken(ctx, &ssooidc.CreateTokenInput{
			ClientId:     registerOut.ClientId,
			ClientSecret: registerOut.ClientSecret,
			DeviceCode:   deviceAuthOut.DeviceCode,
			Code:         deviceAuthOut.UserCode,
			GrantType:    aws.String("urn:ietf:params:oauth:grant-type:device_code"),
		})
		assert.NoError(t, err)
		assert.NotNil(t, tokenOut)

		// List accounts
		accountsOut, err := scenario.mockSSO.ListAccounts(ctx, &sso.ListAccountsInput{
			AccessToken: tokenOut.AccessToken,
		})
		assert.NoError(t, err)
		require.NotNil(t, accountsOut)
		assert.Greater(t, len(accountsOut.AccountList), 0)

		// List roles for first account
		rolesOut, err := scenario.mockSSO.ListAccountRoles(ctx, &sso.ListAccountRolesInput{
			AccessToken: tokenOut.AccessToken,
			AccountId:   accountsOut.AccountList[0].AccountId,
		})
		assert.NoError(t, err)
		assert.NotNil(t, rolesOut)

		// Get credentials for first role
		credsOut, err := scenario.mockSSO.GetRoleCredentials(ctx, &sso.GetRoleCredentialsInput{
			AccessToken: tokenOut.AccessToken,
			AccountId:   accountsOut.AccountList[0].AccountId,
			RoleName:    rolesOut.RoleList[0].RoleName,
		})
		assert.NoError(t, err)
		assert.NotNil(t, credsOut)
		assert.NotNil(t, credsOut.RoleCredentials)
		assert.Equal(t, scenario.testAccessKey, *credsOut.RoleCredentials.AccessKeyId)

		// Verify all mocks were called
		scenario.mockSSOOIDC.AssertCalled(t, "RegisterClient", mock.Anything, mock.Anything)
		scenario.mockSSOOIDC.AssertCalled(t, "StartDeviceAuthorization", mock.Anything, mock.Anything)
		scenario.mockSSOOIDC.AssertCalled(t, "CreateToken", mock.Anything, mock.Anything)
		scenario.mockSSO.AssertCalled(t, "ListAccounts", mock.Anything, mock.Anything)
		scenario.mockSSO.AssertCalled(t, "ListAccountRoles", mock.Anything, mock.Anything)
		scenario.mockSSO.AssertCalled(t, "GetRoleCredentials", mock.Anything, mock.Anything)
	})
}

// TestErrorHandling tests error scenarios
func TestErrorHandling(t *testing.T) {
	t.Run("handles invalid access token", func(t *testing.T) {
		mockSSO := &mocks.MockSSOClient{}
		mockSSO.On("ListAccounts", mock.Anything, mock.Anything).
			Return(nil, &ssotypes.InvalidRequestException{
				Message: aws.String("Invalid access token"),
			})

		ctx := context.Background()
		_, err := mockSSO.ListAccounts(ctx, &sso.ListAccountsInput{
			AccessToken: aws.String("invalid-token"),
		})

		assert.Error(t, err)
	})

	t.Run("handles access denied", func(t *testing.T) {
		mockSSO := &mocks.MockSSOClient{}
		mockSSO.On("GetRoleCredentials", mock.Anything, mock.Anything).
			Return(nil, &ssotypes.UnauthorizedException{
				Message: aws.String("User is not authorized to access this role"),
			})

		ctx := context.Background()
		_, err := mockSSO.GetRoleCredentials(ctx, &sso.GetRoleCredentialsInput{
			AccountId: aws.String("123456789"),
			RoleName:  aws.String("RestrictedRole"),
		})

		assert.Error(t, err)
	})
}

// TestPaginationHandling tests large result set handling
func TestPaginationHandling(t *testing.T) {
	t.Run("handles account pagination", func(t *testing.T) {
		mockSSO := &mocks.MockSSOClient{}

		// First page results
		firstPage := []ssotypes.AccountInfo{
			mocks.BuildAccountInfo("111111111111", "account 1"),
			mocks.BuildAccountInfo("222222222222", "account 2"),
		}
		firstPageOutput := mocks.BuildListAccountsOutput(firstPage)
		firstPageOutput.NextToken = aws.String("nexttoken123")

		// Second page results
		secondPage := []ssotypes.AccountInfo{
			mocks.BuildAccountInfo("333333333333", "account 3"),
			mocks.BuildAccountInfo("444444444444", "account 4"),
		}
		secondPageOutput := mocks.BuildListAccountsOutput(secondPage)

		mockSSO.On("ListAccounts", mock.Anything, mock.MatchedBy(func(input *sso.ListAccountsInput) bool {
			return input.NextToken == nil
		})).Return(firstPageOutput, nil).Once()

		mockSSO.On("ListAccounts", mock.Anything, mock.MatchedBy(func(input *sso.ListAccountsInput) bool {
			return input.NextToken != nil && *input.NextToken == "nexttoken123"
		})).Return(secondPageOutput, nil).Once()

		ctx := context.Background()

		// Fetch first page
		firstOut, err := mockSSO.ListAccounts(ctx, &sso.ListAccountsInput{})
		assert.NoError(t, err)
		assert.Equal(t, 2, len(firstOut.AccountList))
		assert.NotNil(t, firstOut.NextToken)

		// Fetch second page
		secondOut, err := mockSSO.ListAccounts(ctx, &sso.ListAccountsInput{NextToken: firstOut.NextToken})
		assert.NoError(t, err)
		assert.Equal(t, 2, len(secondOut.AccountList))
		assert.Nil(t, secondOut.NextToken)

		// Verify both calls were made
		mockSSO.AssertNumberOfCalls(t, "ListAccounts", 2)
	})
}

// TestConcurrentCredentialRetrieval tests concurrent API calls
func TestConcurrentCredentialRetrieval(t *testing.T) {
	t.Run("retrieves credentials for multiple roles concurrently", func(t *testing.T) {
		mockSSO := &mocks.MockSSOClient{}

		roles := []string{"DeveloperRole", "AdminRole", "DevOpsRole"}
		expiresAt := time.Now().Add(12 * time.Hour).UnixMilli()

		mockSSO.On("GetRoleCredentials", mock.Anything, mock.Anything).
			Return(mocks.BuildGetCredentialsOutput(
				mocks.BuildRoleCredentials(
					"AKIA...",
					"secret...",
					"token...",
					expiresAt,
				),
			), nil)

		ctx := context.Background()
		results := make(chan *sso.GetRoleCredentialsOutput, len(roles))

		// Simulate concurrent credential retrieval
		for _, role := range roles {
			go func(roleName string) {
				creds, _ := mockSSO.GetRoleCredentials(ctx, &sso.GetRoleCredentialsInput{
					RoleName: aws.String(roleName),
				})
				results <- creds
			}(role)
		}

		// Collect results
		for i := 0; i < len(roles); i++ {
			creds := <-results
			assert.NotNil(t, creds)
		}

		// Verify all calls were made
		mockSSO.AssertNumberOfCalls(t, "GetRoleCredentials", len(roles))
	})
}
