package awsssocreds

import (
	_ "embed"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/JorgeReus/aws-sso-creds/internal/app/config"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/ui"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/util"
	"github.com/spf13/cobra"
)

var createStatic, populateRoles, foceLogin, noBrowser bool
var configPath, home string
var version = "dirty"
var selectedOrg config.Organization
var rootDepsFactory = defaultRootDeps
var readBuildInfo = debug.ReadBuildInfo

//go:embed release_version.txt
var embeddedReleaseVersion string

type rootDeps struct {
	initConfig func(home, configPath string) error
	startUI    func(ui.UI) error
	homeDir    func() (string, error)
}

func defaultRootDeps() rootDeps {
	return rootDeps{
		initConfig: config.Init,
		startUI: func(uiVars ui.UI) error {
			return uiVars.Start()
		},
		homeDir: util.HomeDir,
	}
}

func newRootCmd(deps rootDeps) *cobra.Command {
	cobra.AddTemplateFunc("buildVersion", func() string {
		return buildVersion()
	})

	cmd := &cobra.Command{
		Use:   "aws-sso-creds [flags] [organization]",
		Short: "aws-sso-creds - Local AWS SSO credentials made easy",
		Long: `Opinionated CLI app for AWS SSO made in Golang!
AWS SSO Creds is an AWS SSO creds manager for the shell.
Use it to easily manage entries in ~/.aws/config & ~/.aws/credentials files, so you can focus on your AWS workflows, without the hazzle of manually managing your credentials.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := deps.initConfig(home, configPath); err != nil {
				return err
			}

			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return err
			}

			var ok bool
			if selectedOrg, ok = config.GetInstance().Orgs[args[0]]; !ok {
				return fmt.Errorf(
					"Organization '%s' not found in config file %s",
					args[0],
					configPath,
				)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return deps.startUI(ui.UI{
				CreateStatic:  createStatic,
				PopulateRoles: populateRoles,
				ForceLogin:    foceLogin,
				NoBrowser:     noBrowser,
				Org:           selectedOrg,
			})
		},
		SilenceUsage: true,
	}
	cmd.SetHelpTemplate(cmd.HelpTemplate() + "{{if eq .CommandPath \"aws-sso-creds\"}}\nVersion: {{buildVersion}}\n{{end}}")

	cmd.Flags().
		BoolVarP(&createStatic, "temp", "t", false, "Create temporary credentials in ~/.aws/credentials")
	cmd.Flags().
		BoolVarP(&populateRoles, "populateRoles", "p", false, "Populate AWS SSO roles in ~/.aws/config")
	cmd.Flags().
		BoolVarP(&foceLogin, "forceAuth", "f", false, "Force Authentication with AWS SSO")
	cmd.Flags().
		BoolVarP(&noBrowser, "noBrowser", "b", false, "Do not open in the browser automatically")

	return cmd
}

var rootCmd = newRootCmd(rootDepsFactory())

func Execute() {
	var err error
	home, err = rootDepsFactory().homeDir()
	if err != nil {
		panic(fmt.Errorf("Error getting user home dir: %s", err))
	}

	rootCmd.PersistentFlags().
		StringVarP(&configPath, "config", "c", fmt.Sprintf("%s/.config/aws-sso-creds.toml", home), "Directory of the .toml config")

	if err := rootCmd.Execute(); err != nil {
		panic(fmt.Errorf("There was an error running aws-sso-creds '%s'", err))
	}
}

func buildVersion() string {
	if version != "dirty" {
		return version
	}

	releaseVersion := normalizedReleaseVersion()
	info, ok := readBuildInfo()
	if !ok {
		if releaseVersion != "" {
			return releaseVersion
		}
		return version
	}

	switch info.Main.Version {
	case "", "(devel)":
		return "devel"
	default:
		mainVersion := strings.TrimPrefix(info.Main.Version, "v")
		if strings.HasPrefix(mainVersion, "0.0.0-") && releaseVersion != "" {
			return releaseVersion
		}
		return mainVersion
	}
}

func normalizedReleaseVersion() string {
	for _, line := range strings.Split(embeddedReleaseVersion, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "<!--") {
			continue
		}
		return strings.TrimPrefix(line, "v")
	}
	return ""
}
