package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	appconfig "github.com/JorgeReus/aws-sso-creds/internal/app/config"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/bus"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/cache"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/files"
	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	awsv2config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	ssotypes "github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	ssooidctypes "github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
	"github.com/pkg/browser"
)

const (
	tempCredsPrefix = "tmp"
)

var loginDepsFactory = defaultLoginDeps

func defaultLoginDeps() loginDeps {
	return loginDeps{
		newOIDCClient: func(ctx context.Context, region string) (oidcClientAPI, error) {
			cfg, err := awsv2config.LoadDefaultConfig(ctx, awsv2config.WithRegion(region))
			if err != nil {
				return nil, err
			}
			return ssooidc.NewFromConfig(cfg), nil
		},
		newSSOClient: func(ctx context.Context, region string) (ssoClientAPI, error) {
			cfg, err := awsv2config.LoadDefaultConfig(ctx, awsv2config.WithRegion(region))
			if err != nil {
				return nil, err
			}
			return sso.NewFromConfig(cfg), nil
		},
		getClientCreds: cache.GetSSOClientCreds,
		saveClientCreds: func(creds *cache.SSOClientCredentials, region *string) error {
			return creds.Save(region)
		},
		getToken:      cache.GetSSOToken,
		saveToken:     func(token *cache.SSOToken, url string) error { return token.Save(url) },
		newConfigFile: files.NewConfigFile,
		openURL:       browser.OpenURL,
		sleep:         time.Sleep,
		now:           time.Now,
	}
}

func Login(
	org appconfig.Organization,
	forceLogin, noBrowser bool,
	msgBus *bus.Bus,
) (*SSOFlow, error) {
	return loginWithDeps(org, forceLogin, noBrowser, msgBus, loginDepsFactory())
}

func loginWithDeps(
	org appconfig.Organization,
	forceLogin, noBrowser bool,
	msgBus *bus.Bus,
	deps loginDeps,
) (*SSOFlow, error) {
	ctx := context.Background()
	var oidcClient oidcClientAPI
	getOIDCClient := func() (oidcClientAPI, error) {
		if oidcClient != nil {
			return oidcClient, nil
		}
		client, err := deps.newOIDCClient(ctx, org.Region)
		if err != nil {
			return nil, err
		}
		oidcClient = client
		return oidcClient, nil
	}
	clientCredentials, err := deps.getClientCreds(org.Region)
	if err != nil {
		return nil, err
	}

	if clientCredentials == nil || forceLogin {
		oidcClient, err := getOIDCClient()
		if err != nil {
			return nil, err
		}
		input := ssooidc.RegisterClientInput{ClientName: &clientName, ClientType: &clientType}

		resp, err := oidcClient.RegisterClient(ctx, &input)
		if err != nil {
			return nil, err
		}

		tm := time.Unix(resp.ClientSecretExpiresAt, 0)
		clientCredentials = &cache.SSOClientCredentials{
			ClientId:     *resp.ClientId,
			ClientSecret: *resp.ClientSecret,
			ExpiresAt:    tm.Format(time.RFC3339),
		}
		if err := deps.saveClientCreds(clientCredentials, &org.Region); err != nil {
			return nil, err
		}
	}

	ssoToken, err := deps.getToken(org.URL, org.Region)
	if err != nil {
		return nil, err
	}

	if ssoToken == nil || forceLogin {
		oidcClient, err := getOIDCClient()
		if err != nil {
			return nil, err
		}
		startDeviceAuthInput := ssooidc.StartDeviceAuthorizationInput{
			ClientId:     &clientCredentials.ClientId,
			ClientSecret: &clientCredentials.ClientSecret,
			StartUrl:     &org.URL,
		}
		response, err := oidcClient.StartDeviceAuthorization(ctx, &startDeviceAuthInput)
		if err != nil {
			return nil, err
		}

		msgBus.Send(bus.BusMsg{
			MsgType:  bus.MSG_TYPE_INFO,
			Contents: fmt.Sprintln(fmt.Sprintf("The code received is %s, please verify accordingly", *response.UserCode)),
		})

		if !noBrowser {
			err = deps.openURL(*response.VerificationUriComplete)
		}

		if err != nil || noBrowser {
			msgBus.Send(bus.BusMsg{
				MsgType:  bus.MSG_TYPE_ERR,
				Contents: fmt.Sprintf("Can't open your browser, open this URL mannually: %s", *response.VerificationUriComplete),
			})
			msgBus.Recv()
		}

		for {
			deps.sleep(time.Second * time.Duration(response.Interval))
			createTokenInput := ssooidc.CreateTokenInput{
				ClientId:     &clientCredentials.ClientId,
				ClientSecret: &clientCredentials.ClientSecret,
				Code:         response.UserCode,
				DeviceCode:   response.DeviceCode,
				GrantType:    &grantType,
			}
			createTokenOutput, err := oidcClient.CreateToken(ctx, &createTokenInput)

			if err != nil {
				var pendingErr *ssooidctypes.AuthorizationPendingException
				if errors.As(err, &pendingErr) {
					continue
				}
				return nil, err
			}

			ssoToken = &cache.SSOToken{
				StartUrl:    org.URL,
				Region:      org.Region,
				AccessToken: *createTokenOutput.AccessToken,
				ExpiresAt:   deps.now().Add(time.Second * time.Duration(createTokenOutput.ExpiresIn)).Format(time.RFC3339),
			}

			if err := deps.saveToken(ssoToken, org.URL); err != nil {
				return nil, err
			}
			break
		}
	}

	t, err := time.Parse(time.RFC3339Nano, ssoToken.ExpiresAt)
	if err != nil {
		return nil, err
	}

	msgBus.Send(bus.BusMsg{
		MsgType:  bus.MSG_TYPE_INFO,
		Contents: fmt.Sprintf("The SSO session will expire at %s", t),
	})
	file, err := deps.newConfigFile(appconfig.GetInstance().Home)
	if err != nil {
		return nil, err
	}

	ssoClient, err := deps.newSSOClient(ctx, org.Region)
	if err != nil {
		return nil, err
	}

	return &SSOFlow{
		accessToken: &ssoToken.AccessToken,
		ssoClient:   ssoClient,
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
	var result AccountRolesOutput
	listRolesInput := sso.ListAccountRolesInput{
		AccessToken: s.accessToken,
		AccountId:   acc.AccountId,
		NextToken:   nil,
	}

	var roleList []ssotypes.RoleInfo

	for {
		rolesResponse, err := s.ssoClient.ListAccountRoles(context.Background(), &listRolesInput)
		if err != nil {
			result.err = err
			wg.Done()
			channel <- result
			return
		}
		roleList = append(roleList, rolesResponse.RoleList...)
		if rolesResponse.NextToken == nil {
			break
		}
		listRolesInput.NextToken = rolesResponse.NextToken
	}

	for _, role := range roleList {
		parts := strings.Split(awsv2.ToString(acc.AccountName), " ")
		var body string
		for i, part := range parts {
			if i > 0 {
				body += "-" + part
			} else {
				body += part
			}
		}
		sectionName := fmt.Sprintf("profile %s:%s:%s", s.prefix, body, awsv2.ToString(role.RoleName))

		section, err := s.configFile.File.NewSection(sectionName)
		if err != nil {
			result.err = err
			break
		}

		section.NewKey("sso_start_url", *s.ssoStartUrl)
		section.NewKey("sso_region", *s.ssoRegion)
		section.NewKey("sso_account_name", awsv2.ToString(acc.AccountName))
		section.NewKey("sso_account_id", awsv2.ToString(acc.AccountId))
		section.NewKey("sso_role_name", awsv2.ToString(role.RoleName))
		section.NewKey("region", *s.ssoRegion)
		section.NewKey("org", s.orgName)
		section.NewKey("sso_auto_populated", "true")
	}
	channel <- result
	wg.Done()
}

func (s *SSOFlow) PopulateRoles() ([]string, error) {
	listAccountsInput := sso.ListAccountsInput{
		AccessToken: s.accessToken,
		NextToken:   nil,
	}

	var accounts []ssotypes.AccountInfo

	for {
		accsResponse, err := s.ssoClient.ListAccounts(context.Background(), &listAccountsInput)
		if err != nil {
			return nil, err
		}

		accounts = append(accounts, accsResponse.AccountList...)

		if accsResponse.NextToken == nil {
			break
		}

		listAccountsInput.NextToken = accsResponse.NextToken
	}

	s.configFile.CleanTemporaryRoles(s.orgName)

	var wg sync.WaitGroup
	wg.Add(len(accounts))
	queue := make(chan AccountRolesOutput, len(accounts))
	for i := range accounts {
		acc := accounts[i]
		go s.getAccountRoles(&acc, &wg, queue)
	}
	wg.Wait()
	var result []string
	for _, section := range s.configFile.File.Sections() {
		if files.IsValidEntry(section, s.orgName) {
			name := strings.Replace(section.Name(), "profile ", "", 1)
			result = append(result, name)
		}
	}
	if err := s.configFile.Save(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *SSOFlow) GetCredentials() ([]CredentialsResult, error) {
	var result []CredentialsResult
	creds, err := files.NewCredentialsFile(appconfig.GetInstance().Home)
	if err != nil {
		return nil, err
	}
	creds.CleanTemporaryRoles(s.orgName)

	var wg sync.WaitGroup
	queue := make(chan RoleCredentialsOutput, len(s.configFile.File.Sections()))
	for _, section := range s.configFile.File.Sections() {
		if files.IsValidEntry(section, s.orgName) {
			accID := section.Key("sso_account_id").Value()
			roleName := section.Key("sso_role_name").Value()
			credsInput := sso.GetRoleCredentialsInput{
				AccessToken: s.accessToken,
				AccountId:   &accID,
				RoleName:    &roleName,
			}
			wg.Add(1)
			go s.getRoleCreds(&credsInput, &wg, queue, section.Name())
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
		profName := fmt.Sprintf("%s:%s", tempCredsPrefix, strings.TrimPrefix(item.roleName, "profile "))
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
	var result RoleCredentialsOutput
	result.roleName = roleName
	credsOutput, err := s.ssoClient.GetRoleCredentials(context.Background(), input)
	if err != nil {
		result.err = err
	}
	result.creds = credsOutput
	channel <- result
	if wg != nil {
		wg.Done()
	}
}

func (s *SSOFlow) GetCredsByRoleName(roleName string, accountID string) (*sso.GetRoleCredentialsOutput, error) {
	return s.ssoClient.GetRoleCredentials(context.Background(), &sso.GetRoleCredentialsInput{
		AccessToken: s.accessToken,
		AccountId:   &accountID,
		RoleName:    &roleName,
	})
}

func GetCachedSSOFlow(org appconfig.Organization) (*SSOFlow, error) {
	return getCachedSSOFlowWithDeps(org, loginDepsFactory())
}

func getCachedSSOFlowWithDeps(org appconfig.Organization, deps loginDeps) (*SSOFlow, error) {
	clientCredentials, err := deps.getClientCreds(org.Region)
	if err != nil {
		return nil, err
	}

	if clientCredentials == nil {
		return nil, fmt.Errorf("Unable to get client credentials, please login with this CLI and then try again")
	}

	ssoToken, err := deps.getToken(org.URL, org.Region)
	if err != nil {
		return nil, err
	}

	if ssoToken == nil {
		return nil, fmt.Errorf("Unable to get sso token, please login with this CLI and then try again")
	}

	file, err := deps.newConfigFile(appconfig.GetInstance().Home)
	if err != nil {
		return nil, err
	}

	ssoClient, err := deps.newSSOClient(context.Background(), org.Region)
	if err != nil {
		return nil, err
	}

	return &SSOFlow{
		accessToken: &ssoToken.AccessToken,
		ssoClient:   ssoClient,
		configFile:  file,
		ssoRegion:   &org.Region,
		ssoStartUrl: &org.URL,
		orgName:     org.Name,
		prefix:      org.Prefix,
	}, nil
}
