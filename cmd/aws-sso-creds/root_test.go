package awsssocreds

import (
	"errors"
	"strings"
	"testing"

	"github.com/JorgeReus/aws-sso-creds/internal/app/config"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/ui"
)

func TestRootCommandReturnsOrgNotFoundError(t *testing.T) {
	cmd := newRootCmd(rootDeps{
		initConfig: func(home, path string) error {
			config.ResetForTest()
			config.SetInstanceForTest(&config.Config{
				Home: home,
				Orgs: map[string]config.Organization{},
			})
			return nil
		},
	})
	cmd.SetArgs([]string{"missing"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "Organization 'missing' not found") {
		t.Fatalf("Execute() error = %v, want org not found", err)
	}
}

func TestRootCommandStartsUIForSelectedOrg(t *testing.T) {
	started := false
	cmd := newRootCmd(rootDeps{
		initConfig: func(home, path string) error {
			config.ResetForTest()
			config.SetInstanceForTest(&config.Config{
				Home: home,
				Orgs: map[string]config.Organization{
					"dev": {
						Name:   "dev",
						Prefix: "dev",
						URL:    "https://dev.awsapps.com/start",
						Region: "us-east-1",
					},
				},
			})
			return nil
		},
		startUI: func(vars ui.UI) error {
			started = true
			if vars.Org.Name != "dev" {
				t.Fatalf("Org.Name = %q, want %q", vars.Org.Name, "dev")
			}
			return nil
		},
	})
	cmd.SetArgs([]string{"dev"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !started {
		t.Fatal("UI start was not invoked")
	}
}

func TestRootCommandReturnsConfigInitError(t *testing.T) {
	wantErr := errors.New("bad config")
	cmd := newRootCmd(rootDeps{
		initConfig: func(home, path string) error {
			return wantErr
		},
	})
	cmd.SetArgs([]string{"dev"})

	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want %v", err, wantErr)
	}
}

func TestExecuteUsesFactoryHomeDir(t *testing.T) {
	origFactory := rootDepsFactory
	origRootCmd := rootCmd
	origHome := home
	origConfigPath := configPath
	defer func() {
		rootDepsFactory = origFactory
		rootCmd = origRootCmd
		home = origHome
		configPath = origConfigPath
	}()

	rootDepsFactory = func() rootDeps {
		return rootDeps{
			homeDir: func() (string, error) { return "/tmp/test-home", nil },
		}
	}

	executed := false
	rootCmd = newRootCmd(rootDeps{
		initConfig: func(home, configPath string) error {
			config.ResetForTest()
			config.SetInstanceForTest(&config.Config{
				Home: home,
				Orgs: map[string]config.Organization{
					"dev": {Name: "dev", Prefix: "dev", URL: "https://dev.awsapps.com/start", Region: "us-east-1"},
				},
			})
			return nil
		},
		startUI: func(ui.UI) error {
			executed = true
			return nil
		},
	})
	rootCmd.SetArgs([]string{"dev"})

	Execute()

	if !executed {
		t.Fatal("Execute() did not run the command")
	}
	if home != "/tmp/test-home" {
		t.Fatalf("home = %q, want /tmp/test-home", home)
	}
}
