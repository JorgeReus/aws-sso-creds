package awsssocreds

import (
	"errors"
	"fmt"
	"os"

	fuzzyfinder "github.com/ktr0731/go-fuzzyfinder"
	"github.com/spf13/cobra"

	"github.com/JorgeReus/aws-sso-creds/internal/app/config"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/ui"
)

type previewer interface {
	Preview() (*string, error)
}

type selectDeps struct {
	initConfig        func(home, configPath string) error
	newFuzzyPreviewer func(credentialsPath string, configFilePath string) (previewer, error)
	println           func(...interface{}) (int, error)
	getenv            func(string) string
}

var selectDepsFactory = defaultSelectDeps

func defaultSelectDeps() selectDeps {
	return selectDeps{
		initConfig: config.Init,
		newFuzzyPreviewer: func(credentialsPath string, configFilePath string) (previewer, error) {
			return ui.NewFuzzyPreviewer(credentialsPath, configFilePath)
		},
		println: fmt.Println,
		getenv:  os.Getenv,
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
				return fmt.Errorf("error starting program: %w", err)
			}
			selectedEntry, err := fp.Preview()
			if err != nil {
				if errors.Is(err, fuzzyfinder.ErrAbort) {
					if profile := deps.getenv("AWS_PROFILE"); profile != "" {
						_, err = deps.println(profile)
						return err
					}
					return nil
				}
				return fmt.Errorf("error selecting entry: %w", err)
			}
			if selectedEntry == nil {
				return fmt.Errorf("error selecting entry: %w", errors.New("no profile selected"))
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
