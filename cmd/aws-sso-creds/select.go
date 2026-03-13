package awsssocreds

import (
	"fmt"

	"github.com/JorgeReus/aws-sso-creds/internal/app/config"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/ui"
	"github.com/spf13/cobra"
)

type previewer interface {
	Preview() (*string, error)
}

type selectDeps struct {
	initConfig         func(home, configPath string) error
	newFuzzyPreviewer  func(credentialsPath string, configFilePath string) (previewer, error)
	println            func(...interface{}) (int, error)
}

var selectDepsFactory = defaultSelectDeps

func defaultSelectDeps() selectDeps {
	return selectDeps{
		initConfig: config.Init,
		newFuzzyPreviewer: func(credentialsPath string, configFilePath string) (previewer, error) {
			return ui.NewFuzzyPreviewer(credentialsPath, configFilePath)
		},
		println: fmt.Println,
	}
}

func newSelectCmd(deps selectDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "select",
		Short: "Select your role/credentials in a fuzzy-finder previewer",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := deps.initConfig(home, configPath); err != nil {
				return err
			}

			home := config.GetInstance().Home
			credentialsPath := fmt.Sprintf("%s/.aws/credentials", home)
			configFilePath := fmt.Sprintf("%s/.aws/config", home)

			fp, err := deps.newFuzzyPreviewer(credentialsPath, configFilePath)
			if err != nil {
				return fmt.Errorf("Error starting program: %w", err)
			}
			selectedEntry, err := fp.Preview()
			if err != nil {
				return fmt.Errorf("Error selecting entry: %w", err)
			}
			_, err = deps.println(*selectedEntry)
			return err
		},
		SilenceUsage: true,
	}
}

var selectCmd = newSelectCmd(selectDepsFactory())

func init() {
	rootCmd.AddCommand(selectCmd)
}
