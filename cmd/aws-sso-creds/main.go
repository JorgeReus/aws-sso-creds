package main

import (
	"fmt"
	"os"

	sso "github.com/JorgeReus/aws-sso-creds/internal/app"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/ui"

	tea "github.com/charmbracelet/bubbletea"
	getopt "github.com/pborman/getopt/v2"
)

var (
	p                       *tea.Program
	flow                    *sso.SSOFlow
	createStaticCredentials = getopt.BoolLong("temp", 't', "Create temporary credentials in ~/.aws/credentials")
	populateRoles           = getopt.BoolLong("populateRoles", 'p', "Populate AWS SSO roles in ~/.aws/config")
	ssoURL                  = getopt.StringLong("SSOUrl", 'u', "", "The SSO URL(https://<my-domain>.awsapps.com/start) the AWS_SSO_START_URL env var has precedence over this flag")
	ssoRegion               = getopt.StringLong("SSORegion", 'r', "", "The AWS SSO region, the AWS_SSO_REGION env var has precedence over this flag")
	forceLogin              = getopt.BoolLong("forceAuth", 'f', "Force Authentication with AWS SSO")
	noInteractive           = getopt.BoolLong("noInteractive", 0, "Do not open in the browser automatically")
	optHelp                 = getopt.BoolLong("help", 'h', "Help")
	fuzzyFinderPreviewer    = getopt.BoolLong("selectFuzzy", 's', "Select your role/credentials in a fuzzy-finder previewer")
)

func main() {

	getopt.Parse()
	if *optHelp {
		getopt.Usage()
		os.Exit(0)
	}

	*ssoURL = os.Getenv("AWS_SSO_START_URL")
	*ssoRegion = os.Getenv("AWS_SSO_REGION")

	if *ssoURL == "" {
		fmt.Fprintln(os.Stderr, "The SSO URL parameter is required")
		getopt.Usage()
		os.Exit(1)
	}

	if *ssoRegion == "" {
		fmt.Fprintln(os.Stderr, "The SSO Region parameter is required")
		getopt.Usage()
		os.Exit(1)
	}

	uiVars := ui.UI{
		CreateStatic:  *createStaticCredentials,
		PopulateRoles: *populateRoles,
		SsoURL:        *ssoURL,
		SsoRegion:     *ssoRegion,
		ForceLogin:    *forceLogin,
		NoInteractive: *noInteractive,
		UsePreviewer:  *fuzzyFinderPreviewer,
	}

	err := uiVars.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
