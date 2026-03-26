package awsssocreds

import (
	"fmt"
	"time"

	"github.com/JorgeReus/aws-sso-creds/internal/app/config"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/files"
	"github.com/spf13/cobra"
)

type cleanDeps struct {
	initConfig         func(home, configPath string) error
	newConfigFile      func(string) (*files.AWSFile, error)
	newCredentialsFile func(string) (*files.AWSFile, error)
	nowUnix            func() int64
}

var cleanDepsFactory = defaultCleanDeps

func defaultCleanDeps() cleanDeps {
	return cleanDeps{
		initConfig:         config.Init,
		newConfigFile:      files.NewConfigFile,
		newCredentialsFile: files.NewCredentialsFile,
		nowUnix: func() int64 {
			return time.Now().Unix()
		},
	}
}

func newCleanCmd(deps cleanDeps) *cobra.Command {
	var cleanConfig bool
	var cleanCreds bool
	var expiredOnly bool
	determineScopes := func(cmd *cobra.Command) (bool, bool) {
		configChanged := cmd.Flags().Changed("config-only")
		credsChanged := cmd.Flags().Changed("creds")

		configScope := cleanConfig
		credsScope := cleanCreds

		if !configChanged && !credsChanged {
			credsScope = true
			if expiredOnly {
				configScope = false
			} else {
				configScope = true
			}
		}
		if expiredOnly {
			if !credsChanged {
				credsScope = true
			}
			if !configChanged {
				configScope = false
			}
		}

		return configScope, credsScope
	}

	cmd := &cobra.Command{
		Use:   "clean [org]",
		Short: "Remove auto-populated AWS SSO config and credential entries",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := deps.initConfig(home, configPath); err != nil {
				return err
			}
			if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
				return err
			}

			if expiredOnly && cmd.Flags().Changed("config-only") && !cmd.Flags().Changed("creds") {
				return fmt.Errorf("--expired requires credentials cleanup")
			}

			if len(args) == 1 {
				if _, ok := config.GetInstance().Orgs[args[0]]; !ok {
					return fmt.Errorf(
						"Organization '%s' not found in config file %s",
						args[0],
						configPath,
					)
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			org := ""
			if len(args) == 1 {
				org = args[0]
			}

			effectiveCleanConfig, effectiveCleanCreds := determineScopes(cmd)

			if effectiveCleanConfig {
				cfg, err := deps.newConfigFile(config.GetInstance().Home)
				if err != nil {
					return err
				}
				cfg.CleanTemporaryRoles(org)
				if err := cfg.Save(); err != nil {
					return err
				}
			}

			if effectiveCleanCreds {
				creds, err := deps.newCredentialsFile(config.GetInstance().Home)
				if err != nil {
					return err
				}
				if expiredOnly {
					creds.CleanExpiredCredentials(org, deps.nowUnix())
				} else {
					creds.CleanTemporaryRoles(org)
				}
				if err := creds.Save(); err != nil {
					return err
				}
			}

			return nil
		},
		SilenceUsage: true,
	}

	cmd.Flags().BoolVar(&cleanConfig, "config-only", false, "Clean ~/.aws/config")
	cmd.Flags().BoolVar(&cleanCreds, "creds", false, "Clean ~/.aws/credentials")
	cmd.Flags().BoolVarP(&expiredOnly, "expired", "e", false, "Only clean expired credentials")

	return cmd
}

var cleanCmd = newCleanCmd(cleanDepsFactory())

func init() {
	rootCmd.AddCommand(cleanCmd)
}
