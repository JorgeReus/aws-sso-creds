package files

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/ini.v1"
)

type sectionSpec struct {
	name string
	keys map[string]string
}

func addSections(t *testing.T, cfg *AWSFile, specs []sectionSpec) {
	t.Helper()
	for _, spec := range specs {
		section, err := cfg.File.NewSection(spec.name)
		if err != nil {
			t.Fatalf("NewSection(%s) error = %v", spec.name, err)
		}
		for key, value := range spec.keys {
			if _, err := section.NewKey(key, value); err != nil {
				t.Fatalf("NewKey(%s=%s) error = %v", key, value, err)
			}
		}
	}
}

func mustConfigFile(t *testing.T) (*AWSFile, string) {
	t.Helper()
	home := t.TempDir()
	cfg, err := NewConfigFile(home)
	if err != nil {
		t.Fatalf("NewConfigFile() error = %v", err)
	}
	return cfg, home
}

func assertSectionState(t *testing.T, cfg *AWSFile, name string, shouldExist bool) {
	t.Helper()
	_, err := cfg.File.GetSection(name)
	if shouldExist && err != nil {
		t.Fatalf("expected section %q to exist: %v", name, err)
	}
	if !shouldExist && err == nil {
		t.Fatalf("expected section %q to be removed", name)
	}
}

func ensureDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
}

func TestConfigAndCredentialsHelpersManageAutoPopulatedEntries(t *testing.T) {
	home := t.TempDir()
	ensureDir(t, filepath.Join(home, ".aws"))

	configFile, err := NewConfigFile(home)
	if err != nil {
		t.Fatalf("NewConfigFile() error = %v", err)
	}

	section, err := configFile.File.NewSection("profile dev:account:Admin")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = section.NewKey("org", "dev")
	_, _ = section.NewKey("sso_auto_populated", "true")

	if err := configFile.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	cases := []struct {
		name string
		run  func(t *testing.T) bool
		want bool
	}{
		{
			name: "before cleanup",
			run: func(t *testing.T) bool {
				return ConfigFileSSOEmpty(home, "dev")
			},
			want: false,
		},
		{
			name: "after cleanup",
			run: func(t *testing.T) bool {
				configFile.CleanTemporaryRoles("dev")
				if err := configFile.Save(); err != nil {
					t.Fatalf("Save() after cleanup error = %v", err)
				}
				return ConfigFileSSOEmpty(home, "dev")
			},
			want: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.run(t); got != tt.want {
				t.Fatalf("ConfigFileSSOEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetentryByAWSProfile(t *testing.T) {
	tests := []struct {
		name     string
		profile  string
		setup    func(*AWSFile) error
		wantName string
		wantErr  bool
	}{
		{
			name:    "returns expected section",
			profile: "dev:account:Admin",
			setup: func(cfg *AWSFile) error {
				if _, err := cfg.File.NewSection("profile dev:account:Admin"); err != nil {
					return err
				}
				return nil
			},
			wantName: "profile dev:account:Admin",
		},
		{
			name:    "errors when section missing",
			profile: "missing",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := mustConfigFile(t)
			if tt.setup != nil {
				if err := tt.setup(cfg); err != nil {
					t.Fatalf("setup error = %v", err)
				}
			}

			section, err := cfg.GetentryByAWSProfile(tt.profile)
			if tt.wantErr {
				if err == nil {
					t.Fatal("GetentryByAWSProfile() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("GetentryByAWSProfile() error = %v", err)
			}
			if section.Name() != tt.wantName {
				t.Fatalf("section.Name() = %q, want %q", section.Name(), tt.wantName)
			}
		})
	}
}

func TestNewAWSFileCreation(t *testing.T) {
	tests := []struct {
		name    string
		create  func(string) (string, error)
		wantRel string
	}{
		{
			name: "config file creates aws directory",
			create: func(home string) (string, error) {
				cfg, err := NewConfigFile(home)
				if err != nil {
					return "", err
				}
				return cfg.Path, nil
			},
			wantRel: filepath.Join(".aws", "config"),
		},
		{
			name: "credentials file returns expected path",
			create: func(home string) (string, error) {
				cfg, err := NewCredentialsFile(home)
				if err != nil {
					return "", err
				}
				return cfg.Path, nil
			},
			wantRel: filepath.Join(".aws", "credentials"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			path, err := tt.create(home)
			if err != nil {
				t.Fatalf("create error = %v", err)
			}
			want := filepath.Join(home, tt.wantRel)
			if path != want {
				t.Fatalf("Path = %q, want %q", path, want)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("file stat error = %v", err)
			}
		})
	}
}

func TestConfigFileSSOEmptyReturnsTrueForInvalidConfigs(t *testing.T) {
	tests := []struct {
		name     string
		contents string
	}{
		{
			name:     "missing section bracket",
			contents: "[profile broken\norg=dev\n",
		},
		{
			name:     "malformed key",
			contents: "[profile broken]\norg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			awsDir := filepath.Join(home, ".aws")
			ensureDir(t, awsDir)
			configPath := filepath.Join(awsDir, "config")
			if err := os.WriteFile(configPath, []byte(tt.contents), 0o644); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			if !ConfigFileSSOEmpty(home, "dev") {
				t.Fatal("ConfigFileSSOEmpty() = false, want true for invalid config")
			}
		})
	}
}

func TestIsValidEntryRequiresOrganizationAndAutoPopulatedKey(t *testing.T) {
	tests := []struct {
		name        string
		sectionName string
		keys        map[string]string
		profile     string
		want        bool
	}{
		{
			name:        "valid entry",
			sectionName: "profile dev",
			keys: map[string]string{
				"org":                "dev",
				"sso_auto_populated": "true",
			},
			profile: "dev",
			want:    true,
		},
		{
			name:        "mismatched organization",
			sectionName: "profile mismatched",
			keys: map[string]string{
				"org":                "dev",
				"sso_auto_populated": "true",
			},
			profile: "prod",
			want:    false,
		},
		{
			name:        "missing auto-populated",
			sectionName: "profile no-auto",
			keys: map[string]string{
				"org": "dev",
			},
			profile: "dev",
			want:    false,
		},
		{
			name:        "missing organization",
			sectionName: "profile no-org",
			keys: map[string]string{
				"sso_auto_populated": "true",
			},
			profile: "dev",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := ini.Empty()
			section, err := file.NewSection(tt.sectionName)
			if err != nil {
				t.Fatalf("NewSection() error = %v", err)
			}
			for key, value := range tt.keys {
				if _, err := section.NewKey(key, value); err != nil {
					t.Fatalf("NewKey(%s=%s) error = %v", key, value, err)
				}
			}
			if got := IsValidEntry(section, tt.profile); got != tt.want {
				t.Fatalf("IsValidEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCleanTemporaryRoles(t *testing.T) {
	tests := []struct {
		name      string
		org       string
		sections  []sectionSpec
		wantState map[string]bool
	}{
		{
			name: "removes matching organization entries",
			org:  "dev",
			sections: []sectionSpec{
				{name: "profile keep-other-org", keys: map[string]string{"org": "prod", "sso_auto_populated": "true"}},
				{name: "profile keep-manual", keys: map[string]string{"org": "dev"}},
				{name: "profile remove-match", keys: map[string]string{"org": "dev", "sso_auto_populated": "true"}},
			},
			wantState: map[string]bool{
				"profile keep-other-org": true,
				"profile keep-manual":    true,
				"profile remove-match":   false,
			},
		},
		{
			name: "removes all auto-populated entries",
			org:  "",
			sections: []sectionSpec{
				{name: "profile keep-manual", keys: map[string]string{"org": "dev"}},
				{name: "profile remove-dev", keys: map[string]string{"org": "dev", "sso_auto_populated": "true"}},
				{name: "profile remove-prod", keys: map[string]string{"org": "prod", "sso_auto_populated": "true"}},
			},
			wantState: map[string]bool{
				"profile keep-manual": true,
				"profile remove-dev":  false,
				"profile remove-prod": false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := mustConfigFile(t)
			addSections(t, cfg, tt.sections)
			cfg.CleanTemporaryRoles(tt.org)
			for sectionName, shouldExist := range tt.wantState {
				assertSectionState(t, cfg, sectionName, shouldExist)
			}
		})
	}
}

func TestCleanExpiredCredentials(t *testing.T) {
	tests := []struct {
		name      string
		org       string
		cutoff    int64
		sections  []sectionSpec
		wantState map[string]bool
	}{
		{
			name:   "removes only expired matching sections",
			org:    "dev",
			cutoff: 150,
			sections: []sectionSpec{
				{name: "profile expired-dev", keys: map[string]string{"org": "dev", "sso_auto_populated": "true", "expires_time": "100"}},
				{name: "profile active-dev", keys: map[string]string{"org": "dev", "sso_auto_populated": "true", "expires_time": "200"}},
				{name: "profile expired-prod", keys: map[string]string{"org": "prod", "sso_auto_populated": "true", "expires_time": "50"}},
				{name: "profile manual-expired", keys: map[string]string{"org": "dev", "expires_time": "25"}},
				{name: "profile invalid-expiration", keys: map[string]string{"org": "dev", "sso_auto_populated": "true", "expires_time": "bad"}},
				{name: "profile missing-expiration", keys: map[string]string{"org": "dev", "sso_auto_populated": "true"}},
			},
			wantState: map[string]bool{
				"profile expired-dev":        false,
				"profile active-dev":         true,
				"profile expired-prod":       true,
				"profile manual-expired":     true,
				"profile invalid-expiration": true,
				"profile missing-expiration": true,
			},
		},
		{
			name:   "keeps entries when cutoff before expiration",
			org:    "dev",
			cutoff: 50,
			sections: []sectionSpec{
				{name: "profile future-dev", keys: map[string]string{"org": "dev", "sso_auto_populated": "true", "expires_time": "500"}},
				{name: "profile future-prod", keys: map[string]string{"org": "prod", "sso_auto_populated": "true", "expires_time": "500"}},
			},
			wantState: map[string]bool{
				"profile future-dev":  true,
				"profile future-prod": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := mustConfigFile(t)
			addSections(t, cfg, tt.sections)
			cfg.CleanExpiredCredentials(tt.org, tt.cutoff)
			for sectionName, shouldExist := range tt.wantState {
				assertSectionState(t, cfg, sectionName, shouldExist)
			}
		})
	}
}
