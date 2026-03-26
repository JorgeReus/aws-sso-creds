package awsssocreds

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	awssso "github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	appsso "github.com/JorgeReus/aws-sso-creds/internal/app"
	"github.com/JorgeReus/aws-sso-creds/internal/app/config"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/files"
)

type openDeps struct {
	initConfig    func(home, configPath string) error
	getenv        func(string) string
	openConsole   func(roleName string, sessionDuration uint) error
	newConfigFile func(string) (*files.AWSFile, error)
	getCachedFlow func(config.Organization) (cachedFlow, error)
	httpGet       func(string) (*http.Response, error)
	openURL       func(string) error
}

type cachedFlow interface {
	GetCredsByRoleName(roleName string, accountId string) (*awssso.GetRoleCredentialsOutput, error)
}

var openDepsFactory = defaultOpenDeps

func defaultOpenDeps() openDeps {
	return openDeps{
		initConfig: config.Init,
		getenv:     os.Getenv,
		openConsole: func(roleName string, sessionDuration uint) error {
			return openConsoleWithDeps(roleName, sessionDuration, defaultOpenDeps())
		},
		newConfigFile: files.NewConfigFile,
		getCachedFlow: func(org config.Organization) (cachedFlow, error) {
			return appsso.GetCachedSSOFlow(org)
		},
		httpGet: http.Get,
		openURL: browser.OpenURL,
	}
}

func newOpenCmd(deps openDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "open",
		Short: "Opens the AWS web console based on your AWS_PROFILE environment variable",
		Long: `Opens the AWS web console based on your AWS_PROFILE environment variable.
The aws profile, must be a valid profile populated by this CLI
export AWS_PROFILE=myProfile
aws-sso-creds open`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := deps.initConfig(home, configPath); err != nil {
				return err
			}

			profile := deps.getenv("AWS_PROFILE")
			if profile == "" {
				return fmt.Errorf("the AWS_PROFILE env var must must be set to a valid SSO profile")
			}
			return deps.openConsole(profile, 3600)
		},
		SilenceUsage: true,
	}
}

func openConsoleWithDeps(roleName string, sessionDuration uint, deps openDeps) error {
	file, err := deps.newConfigFile(config.GetInstance().Home)
	if err != nil {
		return err
	}
	entry, err := file.GetentryByAWSProfile(roleName)
	if err != nil {
		return err
	}
	ssoFlow, err := deps.getCachedFlow(config.GetInstance().Orgs[entry.Key("org").String()])
	if err != nil {
		return err
	}

	ssoRoleName := entry.Key("sso_role_name").String()
	ssoAccountId := entry.Key("sso_account_id").String()
	ssoStartUrl := entry.Key("sso_start_url").String()
	region := entry.Key("region").String()

	creds, err := ssoFlow.GetCredsByRoleName(ssoRoleName, ssoAccountId)
	if err != nil {
		return err
	}

	session := appsso.SessionUrlParams{
		AccessKeyId:     *creds.RoleCredentials.AccessKeyId,
		SecretAccessKey: *creds.RoleCredentials.SecretAccessKey,
		SessionToken:    *creds.RoleCredentials.SessionToken,
	}
	encodedSession, err := session.Encode()
	if err != nil {
		return fmt.Errorf("unable to encode session: %w", err)
	}

	url := fmt.Sprintf("%s?Action=getSigninToken&SessionDuration=%d&Session=%s",
		appsso.AWS_FEDERATED_URL, sessionDuration, encodedSession)

	resp, err := deps.httpGet(url)
	if err != nil {
		return fmt.Errorf("unable to login to AWS: %s with %s", appsso.AWS_FEDERATED_URL, roleName)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	loginResponse := appsso.LoginResponse{}
	err = json.Unmarshal(body, &loginResponse)
	if err != nil {
		return fmt.Errorf("error parsing login response: %w", err)
	}
	login := appsso.LoginUrlParams{
		Issuer:      ssoStartUrl,
		Destination: fmt.Sprintf("https://console.aws.amazon.com/console/home?region=%s", region),
		SigninToken: loginResponse.SigninToken,
	}

	loginURL := login.GetUrl()

	err = deps.openURL(loginURL)
	if err != nil {
		return fmt.Errorf("can't open your browser, open this URL mannually: %s", loginURL)
	}
	return nil
}

var openCmd = newOpenCmd(openDepsFactory())

func init() {
	rootCmd.AddCommand(openCmd)
}
