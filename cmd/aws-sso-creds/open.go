package awsssocreds

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	sso "github.com/JorgeReus/aws-sso-creds/internal/app"
	"github.com/JorgeReus/aws-sso-creds/internal/app/config"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/files"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Opens the AWS web console based on your AWS_PROFILE environment variable",
	Long: `Opens the AWS web console based on your AWS_PROFILE environment variable.
The aws profile, must be a valid profile populated by this CLI
export AWS_PROFILE=myProfile
aws-sso-creds open`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := config.Init(home, configPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		profile := os.Getenv("AWS_PROFILE")
		if profile == "" {
			log.Fatal(fmt.Errorf("The AWS_PROFILE env var must must be set to a valid SSO profile"))
		}
		openConsole(profile, 3600)
	},
}

func openConsole(roleName string, sessionDuration uint) {
	file, err := files.NewConfigFile(config.GetInstance().Home)
	if err != nil {
		log.Fatal(err)
	}
	entry, err := file.GetentryByAWSProfile(roleName)
	if err != nil {
		log.Fatal(err)
	}
	ssoFlow, err := sso.GetCachedSSOFlow(config.GetInstance().Orgs[entry.Key("org").String()])
	if err != nil {
		log.Fatal(err)
	}

	ssoRoleName := entry.Key("sso_role_name").String()
	ssoAccountId := entry.Key("sso_account_id").String()
	ssoStartUrl := entry.Key("sso_start_url").String()
	region := entry.Key("region").String()

	creds, err := ssoFlow.GetCredsByRoleName(ssoRoleName, ssoAccountId)
	if err != nil {
		log.Fatal(err)
	}

	session := sso.SessionUrlParams{
		AccessKeyId:     *creds.RoleCredentials.AccessKeyId,
		SecretAccessKey: *creds.RoleCredentials.SecretAccessKey,
		SessionToken:    *creds.RoleCredentials.SessionToken,
	}
	encodedSession, err := session.Encode()

	if err != nil {
		log.Fatal(fmt.Errorf("Unable to encode session %w", err))
	}
	url := fmt.Sprintf("%s?Action=getSigninToken&SessionDuration=%d&Session=%s",
		sso.AWS_FEDERATED_URL, sessionDuration, encodedSession)

	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(fmt.Errorf("Unable to login to AWS: %s with %s", sso.AWS_FEDERATED_URL, roleName))
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	loginResponse := sso.LoginResponse{}
	err = json.Unmarshal(body, &loginResponse)
	if err != nil {
		log.Fatalf("Error parsing Login response: %s", err.Error())
	}
	login := sso.LoginUrlParams{
		Issuer:      ssoStartUrl,
		Destination: fmt.Sprintf("https://console.aws.amazon.com/console/home?region=%s", region),
		SigninToken: loginResponse.SigninToken,
	}

	loginUrl := login.GetUrl()

	err = browser.OpenURL(loginUrl)

	if err != nil {
		s := fmt.Sprintf(
			"Can't open your browser, open this URL mannually: %s",
			loginUrl,
		)
		log.Println(s)
	}
}

func init() {
	rootCmd.AddCommand(openCmd)
}
