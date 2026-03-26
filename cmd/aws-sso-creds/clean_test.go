package awsssocreds

import (
	"path/filepath"
	"strings"
	"testing"

	ini "gopkg.in/ini.v1"

	"github.com/JorgeReus/aws-sso-creds/internal/app/config"
	"github.com/JorgeReus/aws-sso-creds/internal/pkg/files"
)

func TestCleanCommandRejectsExpiredWithoutCredentialsScope(t *testing.T) {
	origHome := home
	defer func() { home = origHome }()
	testHome := t.TempDir()
	home = testHome

	cmd := newCleanCmd(cleanDeps{
		initConfig: func(home, path string) error {
			setupCleanConfig(t, home)
			return nil
		},
		nowUnix: func() int64 { return 200 },
	})
	cmd.SetArgs([]string{"--config-only", "--expired"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "requires credentials cleanup") {
		t.Fatalf("Execute() error = %v, want invalid scope error", err)
	}
}

func TestCleanCommandReturnsOrgNotFoundError(t *testing.T) {
	testHome := t.TempDir()
	origHome := home
	defer func() { home = origHome }()
	home = testHome
	origConfigPath := configPath
	defer func() {
		configPath = origConfigPath
	}()
	configPath = filepath.Join(testHome, "aws-sso-creds.toml")

	cmd := newCleanCmd(cleanDeps{
		initConfig: func(home, path string) error {
			config.ResetForTest()
			config.SetInstanceForTest(&config.Config{
				Home: home,
				Orgs: map[string]config.Organization{
					"dev": {Name: "dev"},
				},
			})
			return nil
		},
		nowUnix: func() int64 { return 200 },
	})
	cmd.SetArgs([]string{"missing"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "organization 'missing' not found") {
		t.Fatalf("Execute() error = %v, want org not found", err)
	}
}

func TestCleanCommandDefaultsToCleaningConfigAndCredentials(t *testing.T) {
	testHome := t.TempDir()
	origHome := home
	defer func() { home = origHome }()
	home = testHome
	setupCleanConfig(t, testHome)

	cmd := newCleanCmd(cleanDeps{
		initConfig: func(home, path string) error {
			setupCleanConfig(t, home)
			return nil
		},
		newConfigFile:      files.NewConfigFile,
		newCredentialsFile: files.NewCredentialsFile,
		nowUnix:            func() int64 { return 200 },
	})
	cmd.SetArgs([]string{"dev"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertSectionMissing(t, filepath.Join(testHome, ".aws", "config"), "profile dev:auto")
	assertSectionMissing(t, filepath.Join(testHome, ".aws", "credentials"), "tmp:dev:auto")
}

func TestCleanCommandAllowsExpiredWithConfigAndCredsScopes(t *testing.T) {
	testHome := t.TempDir()
	origHome := home
	defer func() { home = origHome }()
	home = testHome
	setupCleanConfig(t, testHome)
	createCleanFixtureFiles(t, testHome)
	origConfigPath := configPath
	defer func() {
		configPath = origConfigPath
	}()
	configPath = filepath.Join(testHome, "aws-sso-creds.toml")

	cmd := newCleanCmd(cleanDeps{
		initConfig: func(home, path string) error {
			setupCleanConfig(t, home)
			return nil
		},
		newConfigFile:      files.NewConfigFile,
		newCredentialsFile: files.NewCredentialsFile,
		nowUnix:            func() int64 { return 200 },
	})
	cmd.SetArgs([]string{"--config-only", "--creds", "--expired"})

	logCredentialSections(t, filepath.Join(testHome, ".aws", "credentials"), "before clean")
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	logCredentialSections(t, filepath.Join(testHome, ".aws", "credentials"), "after clean")
	assertSectionMissing(t, filepath.Join(testHome, ".aws", "config"), "profile dev:auto")
	assertSectionMissing(t, filepath.Join(testHome, ".aws", "credentials"), "tmp:dev:expired")
	assertSectionPresent(t, filepath.Join(testHome, ".aws", "credentials"), "tmp:dev:active")
}

func TestCleanCommandExpiredOnlyLeavesConfigUntouchedWhenConfigNotRequested(t *testing.T) {
	testHome := t.TempDir()
	origHome := home
	defer func() { home = origHome }()
	home = testHome
	setupCleanConfig(t, testHome)
	createCleanFixtureFiles(t, testHome)

	cmd := newCleanCmd(cleanDeps{
		initConfig: func(home, path string) error {
			setupCleanConfig(t, home)
			return nil
		},
		newConfigFile:      files.NewConfigFile,
		newCredentialsFile: files.NewCredentialsFile,
		nowUnix:            func() int64 { return 200 },
	})
	cmd.SetArgs([]string{"dev", "--expired"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertSectionPresent(t, filepath.Join(testHome, ".aws", "config"), "profile dev:auto")
	assertSectionMissing(t, filepath.Join(testHome, ".aws", "credentials"), "tmp:dev:expired")
	assertSectionPresent(t, filepath.Join(testHome, ".aws", "credentials"), "tmp:dev:active")
}

func TestCleanCommandExpiredOnlyAcrossAllOrganizations(t *testing.T) {
	testHome := t.TempDir()
	origHome := home
	defer func() { home = origHome }()
	home = testHome
	setupCleanConfig(t, testHome)
	createCleanFixtureFiles(t, testHome)
	createCrossOrgCredentialsFixture(t, testHome)

	cmd := newCleanCmd(cleanDeps{
		initConfig: func(home, path string) error {
			setupCleanConfig(t, home)
			return nil
		},
		newConfigFile:      files.NewConfigFile,
		newCredentialsFile: files.NewCredentialsFile,
		nowUnix:            func() int64 { return 200 },
	})
	cmd.SetArgs([]string{"--expired"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	assertSectionPresent(t, filepath.Join(testHome, ".aws", "config"), "profile dev:auto")
	assertSectionMissing(t, filepath.Join(testHome, ".aws", "credentials"), "tmp:dev:expired")
	assertSectionMissing(t, filepath.Join(testHome, ".aws", "credentials"), "tmp:prod:expired")
	assertSectionPresent(t, filepath.Join(testHome, ".aws", "credentials"), "tmp:prod:active")
}

func setupCleanConfig(t *testing.T, home string) {
	t.Helper()
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
}

func createCleanFixtureFiles(t *testing.T, home string) {
	t.Helper()

	cfg, err := files.NewConfigFile(home)
	if err != nil {
		t.Fatalf("NewConfigFile() error = %v", err)
	}
	cfgSection, err := cfg.File.NewSection("profile dev:auto")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = cfgSection.NewKey("org", "dev")
	_, _ = cfgSection.NewKey("sso_auto_populated", "true")
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() config error = %v", err)
	}

	creds, err := files.NewCredentialsFile(home)
	if err != nil {
		t.Fatalf("NewCredentialsFile() error = %v", err)
	}
	addCredentialSection(t, creds, "tmp:dev:expired", "dev", "100")
	addCredentialSection(t, creds, "tmp:dev:active", "dev", "4102444800")
	if err := creds.Save(); err != nil {
		t.Fatalf("Save() credentials error = %v", err)
	}
}

func createCrossOrgCredentialsFixture(t *testing.T, home string) {
	t.Helper()

	creds, err := files.NewCredentialsFile(home)
	if err != nil {
		t.Fatalf("NewCredentialsFile() error = %v", err)
	}
	addCredentialSection(t, creds, "tmp:prod:expired", "prod", "50")
	addCredentialSection(t, creds, "tmp:prod:active", "prod", "4102444800")
	if err := creds.Save(); err != nil {
		t.Fatalf("Save() additional credentials error = %v", err)
	}
}

func addCredentialSection(t *testing.T, file *files.AWSFile, name, org, expires string) {
	t.Helper()

	section, err := file.File.NewSection(name)
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = section.NewKey("org", org)
	_, _ = section.NewKey("sso_auto_populated", "true")
	if expires != "" {
		_, _ = section.NewKey("expires_time", expires)
	}
}

func logCredentialSections(t *testing.T, path, label string) {
	t.Helper()

	file, err := ini.Load(path)
	if err != nil {
		t.Fatalf("ini.Load(%q) error = %v", path, err)
	}
	t.Logf("%s %s sections:", label, path)
	for _, section := range file.SectionStrings() {
		t.Log(section)
	}
}

func assertSectionPresent(t *testing.T, path, section string) {
	t.Helper()

	file, err := ini.Load(path)
	if err != nil {
		t.Fatalf("ini.Load(%q) error = %v", path, err)
	}
	if _, err := file.GetSection(section); err != nil {
		t.Fatalf("expected section %q in %s, got %v", section, path, err)
	}
}

func assertSectionMissing(t *testing.T, path, section string) {
	t.Helper()

	file, err := ini.Load(path)
	if err != nil {
		t.Fatalf("ini.Load(%q) error = %v", path, err)
	}
	if _, err := file.GetSection(section); err == nil {
		t.Fatalf("section %q still present in %s", section, path)
	}
}
