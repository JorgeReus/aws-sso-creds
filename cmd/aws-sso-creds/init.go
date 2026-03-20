package awsssocreds

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/JorgeReus/aws-sso-creds/internal/app/config"
	"github.com/spf13/cobra"
)

type valueValidator func(string) error

type promptSession struct {
	reader *bufio.Reader
	out    io.Writer
}

type initDeps struct {
	in         io.Reader
	out        io.Writer
	upsertOrg  func(string, config.Organization) error
	fileExists func(string) bool
}

func defaultInitDeps() initDeps {
	return initDeps{
		in:        os.Stdin,
		out:       os.Stdout,
		upsertOrg: config.UpsertOrganizationConfig,
		fileExists: func(path string) bool {
			_, err := os.Stat(path)
			return err == nil
		},
	}
}

func validateNonEmpty(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("value cannot be empty")
	}
	return nil
}

func validateStartURL(value string) error {
	if err := validateNonEmpty(value); err != nil {
		return err
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("start URL must be a valid https URL")
	}
	if !strings.Contains(parsed.Host, ".awsapps.com") || !strings.HasSuffix(parsed.Path, "/start") {
		return fmt.Errorf("start URL must look like an AWS start URL")
	}
	return nil
}

func (p promptSession) promptValue(label string, validator valueValidator) (string, error) {
	for {
		if _, err := fmt.Fprintf(p.out, "%s: ", label); err != nil {
			return "", err
		}
		line, err := p.reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		value := strings.TrimSpace(line)
		if validator != nil {
			if err := validator(value); err != nil {
				if _, writeErr := fmt.Fprintln(p.out, err.Error()); writeErr != nil {
					return "", writeErr
				}
				continue
			}
		}
		return value, nil
	}
}

func promptValue(in io.Reader, out io.Writer, label string, validator valueValidator) (string, error) {
	return promptSession{reader: bufio.NewReader(in), out: out}.promptValue(label, validator)
}

func runInitCommand(deps initDeps) error {
	existed := deps.fileExists(configPath)
	prompts := promptSession{reader: bufio.NewReader(deps.in), out: deps.out}

	name, err := prompts.promptValue("Organization name", validateNonEmpty)
	if err != nil {
		return err
	}
	startURL, err := prompts.promptValue("Start URL", validateStartURL)
	if err != nil {
		return err
	}
	prefix, err := prompts.promptValue("Prefix", validateNonEmpty)
	if err != nil {
		return err
	}
	ssoRegion, err := prompts.promptValue("SSO region", validateNonEmpty)
	if err != nil {
		return err
	}
	defaultRegion, err := prompts.promptValue("Default AWS region (optional, press enter to use the SSO region)", nil)
	if err != nil {
		return err
	}

	if err := deps.upsertOrg(configPath, config.Organization{
		Name:          name,
		URL:           startURL,
		Prefix:        prefix,
		SSORegion:     ssoRegion,
		DefaultRegion: defaultRegion,
	}); err != nil {
		return err
	}

	action := "created"
	if existed {
		action = "updated"
	}
	_, err = fmt.Fprintf(deps.out, "Organization %q %s in %s\n", name, action, configPath)
	return err
}

func newInitCmd(deps initDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create or update the aws-sso-creds config interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitCommand(deps)
		},
		SilenceUsage: true,
	}
}

var initCmd = newInitCmd(defaultInitDeps())

func init() {
	rootCmd.AddCommand(initCmd)
}
