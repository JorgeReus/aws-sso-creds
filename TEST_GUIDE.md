# AWS SSO Credentials - Test Suite Documentation

## Overview

This test suite provides comprehensive testing for the AWS SSO Credentials application with mocked AWS API calls. The tests use `testify/mock` for mocking AWS SDK v2 clients and `testify/assert` for assertions.

## Architecture

### Mock Implementations

**File:** [internal/mocks/mocks.go](../../internal/mocks/mocks.go)

The mocks package provides mock implementations of the AWS SDK clients:

- `MockSSOClient` - Mocks the SSO service client
- `MockSSOOIDCClient` - Mocks the SSO OIDC service client

Additionally, it provides builder functions to easily construct test data:

```go
// Builders for test data
BuildAccountInfo(id, name)                                    // ssotypes.AccountInfo
BuildRoleInfo(name, accountID)                                // ssotypes.RoleInfo
BuildRoleCredentials(accessKey, secretKey, sessionToken, expiresAt)
BuildRegisterClientOutput(clientID, clientSecret, expiresAt)
BuildStartDeviceAuthOutput(userCode, deviceCode, verificationURI, interval)
BuildCreateTokenOutput(accessToken, expiresIn)
BuildListAccountsOutput(accounts)
BuildListRolesOutput(roles)
BuildGetCredentialsOutput(creds)
```

### Test Files

#### Cache Tests
**File:** [internal/pkg/cache/cache_test.go](../../internal/pkg/cache/cache_test.go)

- **Coverage:** 72.5%
- **Test Count:** 8 test groups with subtests

Tests for credential and token caching:

- `TestSSOClientCredentialsSave` - Credential persistence
- `TestGetSSOClientCreds` - Retrieving and managing client credentials
- `TestSSOTokenSave` - Token persistence
- `TestIsExpired` - Expiration time checks
- `TestGetSSOToken` - Token retrieval and validation

#### App Tests
**File:** [internal/app/sso_test.go](../../internal/app/sso_test.go)

Basic unit tests for SSO app functionality:

- `TestLogin` - Login flow validation
- `TestGetAccountRoles` - Account role handling
- `TestPopulateRoles` - Role population with pagination
- `TestGetCredentials` - Credential retrieval
- `TestListAccountRoles` - Role listing
- `TestOIDCRegisterClient` - OIDC client registration
- `TestOIDCStartDeviceAuthorization` - Device authorization flow
- `TestOIDCCreateToken` - Token creation
- `TestMockVerification` - Mock assertion validation

#### Integration Tests
**File:** [internal/app/sso_integration_test.go](../../internal/app/sso_integration_test.go)

End-to-end test scenarios using the `IntegrationTestScenario` builder:

**CompleteSSO Test:**
```
RegisterClient → StartDeviceAuthorization → CreateToken → ListAccounts 
→ ListAccountRoles → GetRoleCredentials
```

**Error Handling Test:** Tests for:
- Invalid access tokens
- Access denied (unauthorized roles)

**Pagination Test:** Tests large result set handling with NextToken

**Concurrent Retrieval Test:** Tests concurrent credential retrieval for multiple roles

## Running Tests

### Run All Tests
```bash
go test ./internal/... -v
```

### Run Specific Package Tests
```bash
# Cache tests only
go test ./internal/pkg/cache/... -v

# App tests only
go test ./internal/app/... -v
```

### Run with Coverage
```bash
# Generate coverage report
go test ./internal/... -cover

# Generate detailed coverage
go test ./internal/pkg/cache ./internal/app -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Run Specific Test
```bash
go test ./internal/app -run TestCompleteSSO -v
```

## Test Patterns

### 1. Using MockBus
```go
mockBus := &MockBus{}
mockBus.On("Send", mock.Anything).Return()
mockBus.On("Recv").Return(bus.BusMsg{})
```

### 2. Setting Up Mock Responses
```go
mockSSO := &mocks.MockSSOClient{}
accounts := []ssotypes.AccountInfo{
    mocks.BuildAccountInfo("111111111111", "dev account"),
}
mockSSO.On("ListAccounts", mock.Anything, mock.Anything).
    Return(mocks.BuildListAccountsOutput(accounts), nil)
```

### 3. Calling Mocks with Context
```go
ctx := context.Background()
output, err := mockSSO.ListAccounts(ctx, &sso.ListAccountsInput{
    AccessToken: aws.String("test-token"),
})
```

### 4. Verifying Mock Calls
```go
mockSSO.AssertCalled(t, "ListAccounts", mock.Anything, mock.Anything)
mockSSO.AssertNumberOfCalls(t, "ListAccounts", 1)
```

### 5. Integration Test Scenario Builder
```go
scenario := NewIntegrationTestScenario(t).
    SetupCompleteSSO().
    SetupAccountsAndRoles().
    SetupCredentialsForRoles()

// Run test using scenario.mockSSO, scenario.mockSSOOIDC, etc.
```

## Key Test Scenarios

### Authentication Flow
1. Client registration (OIDC)
2. Device authorization initiation
3. Token creation after authorization
4. Token storage and retrieval

### Account and Role Management
1. Listing multiple AWS accounts with pagination
2. Retrieving roles for each account
3. Concurrent role processing

### Credentials
1. Retrieving temporary credentials for roles
2. Handling credentials near expiration
3. Storing credentials in cache

### Error Handling
- Invalid access tokens (InvalidRequestException)
- Unauthorized access (UnauthorizedException)
- General API errors

## Dependencies

- `github.com/stretchr/testify/mock` - Mocking framework
- `github.com/stretchr/testify/assert` - Assertion library
- `github.com/stretchr/testify/require` - Assertions with test failure (where used)

## Future Enhancements

1. Add tests for configuration management
2. Expand file operations testing (AddAccountRoles, GetCredentials setup)
3. Add benchmarks for concurrent operations
4. Test error recovery and retry logic
5. Add fuzz testing for input validation
6. Integration tests with real AWS credentials (optional CI setup)

## Running Tests in CI/CD

### GitHub Actions Example
```yaml
- name: Run tests
  run: go test ./internal/... -v -coverprofile=coverage.out

- name: Upload coverage
  uses: codecov/codecov-action@v3
  with:
    files: ./coverage.out
```

## Debugging Tests

### Enable Verbose Logging
```bash
go test ./internal/... -v
```

### Run Single Test with Timeout
```bash
go test ./internal/app -run TestCompleteSSO -timeout 10s -v
```

### Check mock call arguments
```go
mockSSO.On("ListAccounts", mock.Anything, mock.MatchedBy(func(input *sso.ListAccountsInput) bool {
    return input.NextToken == nil
})).Return(...).Once()
```
