package awsssocreds

import (
	"errors"
	"testing"

	fuzzyfinder "github.com/ktr0731/go-fuzzyfinder"

	"github.com/JorgeReus/aws-sso-creds/internal/app/config"
)

type fakePreviewer struct {
	selected *string
	err      error
}

func (f fakePreviewer) Preview() (*string, error) {
	return f.selected, f.err
}

func TestSelectCommandInvokesPreviewerFlow(t *testing.T) {
	var printed string
	cmd := newSelectCmd(selectDeps{
		initConfig: func(home, configPath string) error {
			config.ResetForTest()
			config.SetInstanceForTest(&config.Config{Home: "/tmp"})
			return nil
		},
		newFuzzyPreviewer: func(credentialsPath string, configFilePath string) (previewer, error) {
			choice := "dev:account:Admin"
			return fakePreviewer{selected: &choice}, nil
		},
		println: func(args ...interface{}) (int, error) {
			printed = args[0].(string)
			return 0, nil
		},
		getenv: func(string) string { return "" },
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if printed != "dev:account:Admin" {
		t.Fatalf("printed = %q, want %q", printed, "dev:account:Admin")
	}
}

func TestSelectCommandReturnsPreviewerCreationError(t *testing.T) {
	wantErr := errors.New("broken previewer")
	cmd := newSelectCmd(selectDeps{
		initConfig: func(home, configPath string) error {
			config.ResetForTest()
			config.SetInstanceForTest(&config.Config{Home: "/tmp"})
			return nil
		},
		newFuzzyPreviewer: func(credentialsPath string, configFilePath string) (previewer, error) {
			return nil, wantErr
		},
		println: func(args ...interface{}) (int, error) { return 0, nil },
		getenv:  func(string) string { return "" },
	})

	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want %v", err, wantErr)
	}
}

func TestSelectCommandReturnsPreviewError(t *testing.T) {
	wantErr := errors.New("preview failed")
	cmd := newSelectCmd(selectDeps{
		initConfig: func(home, configPath string) error {
			config.ResetForTest()
			config.SetInstanceForTest(&config.Config{Home: "/tmp"})
			return nil
		},
		newFuzzyPreviewer: func(credentialsPath string, configFilePath string) (previewer, error) {
			return fakePreviewer{err: wantErr}, nil
		},
		println: func(args ...interface{}) (int, error) { return 0, nil },
		getenv:  func(string) string { return "" },
	})

	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("Execute() error = %v, want %v", err, wantErr)
	}
}

func TestSelectCommandReturnsEnvValueOnAbort(t *testing.T) {
	const profile = "dev:account:Admin"
	var printed string
	cmd := newSelectCmd(selectDeps{
		initConfig: func(home, configPath string) error {
			config.ResetForTest()
			config.SetInstanceForTest(&config.Config{Home: "/tmp"})
			return nil
		},
		newFuzzyPreviewer: func(credentialsPath string, configFilePath string) (previewer, error) {
			return fakePreviewer{err: fuzzyfinder.ErrAbort}, nil
		},
		println: func(args ...interface{}) (int, error) {
			printed = args[0].(string)
			return 0, nil
		},
		getenv: func(key string) string {
			if key != "AWS_PROFILE" {
				t.Fatalf("unexpected getenv key %q", key)
			}
			return profile
		},
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if printed != profile {
		t.Fatalf("printed = %q, want %q", printed, profile)
	}
}

func TestDefaultSelectDepsProvidesFunctions(t *testing.T) {
	deps := defaultSelectDeps()
	if deps.initConfig == nil || deps.newFuzzyPreviewer == nil || deps.println == nil ||
		deps.getenv == nil {
		t.Fatal("defaultSelectDeps() returned nil dependency")
	}
}
