package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mikemucc/aws-sso-creds/internal/app/config"
	"github.com/mikemucc/aws-sso-creds/internal/pkg/cache"
	"github.com/mikemucc/aws-sso-creds/internal/pkg/files"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	ssotypes "github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
	"github.com/mikemucc/aws-sso-creds/internal/pkg/bus"
	"github.com/pkg/browser"
)

const (
	tempCredsPrefix = "tmp"
)

func Login(
	org config.Organization,
	forceLogin, noBrowser bool,
	msgBus *bus.Bus,
) (*SSOFlow, error) {
	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(org.Region))
	if err != nil {
		return nil, err
	}

	ssooidcClient := ssooidc.NewFromConfig(cfg)
	clientCredentials, err := cache.GetSSOClientCreds(org.Region)
	if err != nil {
		return nil, err
	}

	// If theres no credentials to use, create new ones
	if clientCredentials == nil || forceLogin {
		input := &ssooidc.RegisterClientInput{
			ClientName: aws.String(clientName),
			ClientType: aws.String(clientType),
		}

		resp, err := ssooidcClient.RegisterClient(ctx, input)
		if err != nil {
			return nil, err
		}

		tm := time.Unix(resp.ClientSecretExpiresAt, 0)
		clientCredentials = &cache.SSOClientCredentials{
			ClientId:     *resp.ClientId,
			ClientSecret: *resp.ClientSecret,
			ExpiresAt:    tm.Format(time.RFC3339),
		}
		if clientCredentials.Save(&org.Region) != nil {
			return nil, err
		}
	}

	var ssoToken *cache.SSOToken
	ssoToken, err = cache.GetSSOToken(ctx, org.URL, nil, org.Region)
	if err != nil {
		return nil, err
	}

	if ssoToken == nil || forceLogin {
		startDeviceAuthInput := &ssooidc.StartDeviceAuthorizationInput{
			ClientId:     aws.String(clientCredentials.ClientId),
			ClientSecret: aws.String(clientCredentials.ClientSecret),
			StartUrl:     aws.String(org.URL),
		}
		response, err := ssooidcClient.StartDeviceAuthorization(ctx, startDeviceAuthInput)
		if err != nil {
			return nil, err
		}

		msgBus.Send(bus.BusMsg{
			MsgType:  bus.MSG_TYPE_INFO,
			Contents: fmt.Sprintln(fmt.Sprintf("The code received is %s, please verify accordingly", *response.UserCode)),
		})

		if !noBrowser {
			err = browser.OpenURL(*response.VerificationUriComplete)
		}

		if err != nil || noBrowser {
			s := fmt.Sprintf(
				"Can't open your browser, open this URL mannually: %s",
				*response.VerificationUriComplete,
			)
			msgBus.Send(bus.BusMsg{
				MsgType:  bus.MSG_TYPE_ERR,
				Contents: s,
			})

			msgBus.Recv()
		}

		for {
			time.Sleep(time.Second * time.Duration(response.Interval))
			createTokenInput := &ssooidc.CreateTokenInput{
				ClientId:     aws.String(clientCredentials.ClientId),
				ClientSecret: aws.String(clientCredentials.ClientSecret),
				Code:         response.UserCode,
				DeviceCode:   response.DeviceCode,
				GrantType:    aws.String(grantType),
			}
			createTokenOutput, err := ssooidcClient.CreateToken(ctx, createTokenInput)

			if err != nil {
				var authPendingErr *types.AuthorizationPendingException
				if errors.As(err, &authPendingErr) {
					// Authorization still pending, continue polling
					continue
				}
				return nil, err
			}

			ssoToken = &cache.SSOToken{
				StartUrl:    org.URL,
				Region:      org.Region,
				AccessToken: *createTokenOutput.AccessToken,
				ExpiresAt:   time.Now().Add(time.Second * time.Duration(createTokenOutput.ExpiresIn)).Format(time.RFC3339),
			}

			ssoToken.Save(org.URL)
			break
		}
	}
	// Parse the expires time to a human readable output and print it
	t, err := time.Parse(time.RFC3339Nano, ssoToken.ExpiresAt)
	if err != nil {
		return nil, err
	}

	s := fmt.Sprintf("The SSO session will expire at %s", t)
	msgBus.Send(bus.BusMsg{
		MsgType:  bus.MSG_TYPE_INFO,
		Contents: s,
	})
	ssoServiceClient := sso.NewFromConfig(cfg)
	file, err := files.NewConfigFile(config.GetInstance().Home)
	if err != nil {
		return nil, err
	}

	return &SSOFlow{
		accessToken: &ssoToken.AccessToken,
		ssoClient:   ssoServiceClient,
		configFile:  file,
		ssoRegion:   &org.Region,
		ssoStartUrl: &org.URL,
		orgName:     org.Name,
		prefix:      org.Prefix,
	}, nil
}

func (s *SSOFlow) getAccountRoles(
	acc *ssotypes.AccountInfo,
	wg *sync.WaitGroup,
	channel chan AccountRolesOutput,
) {
	ctx := context.Background()
	var result AccountRolesOutput
	listRolesInput := &sso.ListAccountRolesInput{
		AccessToken: s.accessToken,
		AccountId:   acc.AccountId,
		NextToken:   nil,
	}

	var roleList []ssotypes.RoleInfo

	for {
		rolesResponse, err := s.ssoClient.ListAccountRoles(ctx, listRolesInput)
		if err != nil {
			result.err = err
			wg.Done()
			channel <- result
			return
		}
		for i := range len(rolesResponse.RoleList) {
			roleList = append(roleList, rolesResponse.RoleList[i])
		}
		if rolesResponse.NextToken == nil {
			break
		}
		listRolesInput.NextToken = rolesResponse.NextToken
	}

	for _, role := range roleList {
		parts := strings.Split(*acc.AccountName, " ")
		var body string
		for i, s := range parts {
			if i > 0 {
				body += "-" + s
			} else {
				body += s
			}
		}
		sectionName := fmt.Sprintf("profile %s:%s:%s", s.prefix, body, *role.RoleName)

		section, err := s.configFile.File.NewSection(sectionName)
		if err != nil {
			result.err = err
			break
		}

		section.NewKey("sso_start_url", *s.ssoStartUrl)
		section.NewKey("sso_region", *s.ssoRegion)
		section.NewKey("sso_account_name", *acc.AccountName)
		section.NewKey("sso_account_id", *acc.AccountId)
		section.NewKey("sso_role_name", *role.RoleName)
		section.NewKey("region", *s.ssoRegion)
		section.NewKey("org", s.orgName)
		section.NewKey("sso_auto_populated", "true")
	}
	channel <- result
	wg.Done()
}

func (s *SSOFlow) PopulateRoles() ([]string, error) {
	ctx := context.Background()
	listAccountsInput := &sso.ListAccountsInput{
		AccessToken: s.accessToken,
		NextToken:   nil,
	}

	var accounts []ssotypes.AccountInfo

	for {
		accsResponse, err := s.ssoClient.ListAccounts(ctx, listAccountsInput)
		if err != nil {
			return nil, err
		}

		for i := range len(accsResponse.AccountList) {
			accounts = append(accounts, accsResponse.AccountList[i])
		}

		if accsResponse.NextToken == nil {
			break
		}

		listAccountsInput.NextToken = accsResponse.NextToken
	}

	s.configFile.CleanTemporaryRoles(s.orgName)

	var wg sync.WaitGroup
	wg.Add(len(accounts))
	queue := make(chan AccountRolesOutput, len(accounts))
	for i := range len(accounts) {
		go s.getAccountRoles(&accounts[i], &wg, queue)
	}
	wg.Wait()
	var result []string
	for _, section := range s.configFile.File.Sections() {
		if files.IsValidEntry(section, s.orgName) {
			name := strings.Replace(section.Name(), "profile ", "", 1)
			result = append(result, name)
		}
	}
	err := s.configFile.Save()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *SSOFlow) GetCredentials() ([]CredentialsResult, error) {
	var result []CredentialsResult
	creds, err := files.NewCredentialsFile(config.GetInstance().Home)
	creds.CleanTemporaryRoles(s.orgName)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	queue := make(chan RoleCredentialsOutput, len(s.configFile.File.Sections()))
	for _, section := range s.configFile.File.Sections() {
		if files.IsValidEntry(section, s.orgName) {
			accId := section.Key("sso_account_id").Value()
			roleName := section.Key("sso_role_name").Value()
			credsInput := &sso.GetRoleCredentialsInput{
				AccessToken: s.accessToken,
				AccountId:   aws.String(accId),
				RoleName:    aws.String(roleName),
			}
			wg.Add(1)
			go s.getRoleCreds(credsInput, &wg, queue, section.Name())
		}
	}
	wg.Wait()
	close(queue)
	for item := range queue {
		if item.err != nil {
			result = append(result, CredentialsResult{
				ProfileName:  item.roleName,
				WasSuccesful: false,
			})
			continue
		}
		profName := fmt.Sprintf(
			"%s:%s",
			tempCredsPrefix,
			strings.TrimPrefix(item.roleName, "profile "),
		)
		credsSection, err := creds.File.NewSection(profName)
		if err != nil {
			return nil, item.err
		}
		expiresTime := item.creds.RoleCredentials.Expiration / 1000
		credsSection.NewKey("aws_access_key_id", *item.creds.RoleCredentials.AccessKeyId)
		credsSection.NewKey("aws_secret_access_key", *item.creds.RoleCredentials.SecretAccessKey)
		credsSection.NewKey("aws_session_token", *item.creds.RoleCredentials.SessionToken)
		credsSection.NewKey("issued_time", fmt.Sprint(time.Now().Unix()))
		credsSection.NewKey("expires_time", fmt.Sprint(expiresTime))
		credsSection.NewKey("org", s.orgName)
		credsSection.NewKey("sso_auto_populated", "true")

		result = append(result, CredentialsResult{
			ProfileName:  profName,
			WasSuccesful: true,
			ExpiresAt:    fmt.Sprint(time.Unix(expiresTime, 0).Local()),
		})
	}

	return result, creds.Save()
}

func (s *SSOFlow) getRoleCreds(
	input *sso.GetRoleCredentialsInput,
	wg *sync.WaitGroup,
	channel chan RoleCredentialsOutput,
	roleName string,
) {
	ctx := context.Background()
	var result RoleCredentialsOutput
	result.roleName = roleName
	credsOutput, err := s.ssoClient.GetRoleCredentials(ctx, input)
	if err != nil {
		result.err = err
	}
	result.creds = credsOutput
	channel <- result
	if wg != nil {
		wg.Done()
	}
}

func (s *SSOFlow) GetCredsByRoleName(roleName string, accountId string) (*sso.GetRoleCredentialsOutput, error) {
	ctx := context.Background()
	var result RoleCredentialsOutput
	result.roleName = roleName
	credsOutput, err := s.ssoClient.GetRoleCredentials(ctx, &sso.GetRoleCredentialsInput{
		AccessToken: s.accessToken,
		AccountId:   aws.String(accountId),
		RoleName:    aws.String(roleName),
	})
	if err != nil {
		return nil, err
	}
	return credsOutput, nil
}

func GetCachedSSOFlow(org config.Organization) (*SSOFlow, error) {
	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(org.Region))
	if err != nil {
		return nil, err
	}

	clientCredentials, err := cache.GetSSOClientCreds(org.Region)
	if err != nil {
		return nil, err
	}

	if clientCredentials == nil {
		return nil, fmt.Errorf("Unable to get client credentials, please login with this CLI and then try again")
	}

	var ssoToken *cache.SSOToken
	ssoToken, err = cache.GetSSOToken(ctx, org.URL, nil, org.Region)
	if err != nil {
		return nil, err
	}

	if ssoToken == nil {
		return nil, fmt.Errorf("Unable to get sso token, please login with this CLI and then try again")
	}
	ssoServiceClient := sso.NewFromConfig(cfg)
	file, err := files.NewConfigFile(config.GetInstance().Home)
	if err != nil {
		return nil, err
	}

	return &SSOFlow{
		accessToken: &ssoToken.AccessToken,
		ssoClient:   ssoServiceClient,
		configFile:  file,
		ssoRegion:   &org.Region,
		ssoStartUrl: &org.URL,
		orgName:     org.Name,
		prefix:      org.Prefix,
	}, nil
}
