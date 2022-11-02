package awsssocreds

import (
	"fmt"
	"log"
	"os"

	"github.com/JorgeReus/aws-sso-creds/internal/app/config"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/ui"
	"github.com/spf13/cobra"
)

var selectCmd = &cobra.Command{
	Use:   "select",
	Short: "Select your role/credentials in a fuzzy-finder previewer",
	Run: func(cmd *cobra.Command, args []string) {
		if err := config.Init(home, configPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		home := config.GetInstance().Home
		credentialsPath := fmt.Sprintf("%s/.aws/credentials", home)
		configFilePath := fmt.Sprintf("%s/.aws/config", home)

		// Preview de credentials & profiles
		fp, err := ui.NewFuzzyPreviewer(credentialsPath, configFilePath)
		if err != nil {
			log.Fatal(fmt.Errorf("Error starting program: %w", err))
		}
		selectedEntry, err := fp.Preview()
		if err != nil {
			log.Fatal(fmt.Errorf("Error selecting entry: %w", err))
		}
		fmt.Println(*selectedEntry)
	},
}

func init() {
	rootCmd.AddCommand(selectCmd)
}
