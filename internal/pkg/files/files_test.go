package files

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/ini.v1"
)

func TestConfigAndCredentialsHelpersManageAutoPopulatedEntries(t *testing.T) {
	home := t.TempDir()
	err := os.MkdirAll(filepath.Join(home, ".aws"), 0o755)
	if err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

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

	if ConfigFileSSOEmpty(home, "dev") {
		t.Fatal("ConfigFileSSOEmpty() = true, want false")
	}

	configFile.CleanTemporaryRoles("dev")
	if err := configFile.Save(); err != nil {
		t.Fatalf("Save() after cleanup error = %v", err)
	}
	if !ConfigFileSSOEmpty(home, "dev") {
		t.Fatal("ConfigFileSSOEmpty() = false, want true after cleanup")
	}
}

func TestGetentryByAWSProfileReturnsExpectedSection(t *testing.T) {
	home := t.TempDir()
	err := os.MkdirAll(filepath.Join(home, ".aws"), 0o755)
	if err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	configFile, err := NewConfigFile(home)
	if err != nil {
		t.Fatalf("NewConfigFile() error = %v", err)
	}
	_, err = configFile.File.NewSection("profile dev:account:Admin")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}

	section, err := configFile.GetentryByAWSProfile("dev:account:Admin")
	if err != nil {
		t.Fatalf("GetentryByAWSProfile() error = %v", err)
	}
	if section.Name() != "profile dev:account:Admin" {
		t.Fatalf("section.Name() = %q, want %q", section.Name(), "profile dev:account:Admin")
	}
}

func TestNewCredentialsFileCreatesCredentialsPath(t *testing.T) {
	home := t.TempDir()
	err := os.MkdirAll(filepath.Join(home, ".aws"), 0o755)
	if err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	credentialsFile, err := NewCredentialsFile(home)
	if err != nil {
		t.Fatalf("NewCredentialsFile() error = %v", err)
	}
	if credentialsFile.Path != filepath.Join(home, ".aws", "credentials") {
		t.Fatalf("Path = %q, want %q", credentialsFile.Path, filepath.Join(home, ".aws", "credentials"))
	}
	if _, err := os.Stat(credentialsFile.Path); err != nil {
		t.Fatalf("credentials file stat error = %v", err)
	}
}

func TestNewConfigFileCreatesAWSDirectoryWhenMissing(t *testing.T) {
	home := t.TempDir()

	configFile, err := NewConfigFile(home)
	if err != nil {
		t.Fatalf("NewConfigFile() error = %v", err)
	}
	if configFile.Path != filepath.Join(home, ".aws", "config") {
		t.Fatalf("Path = %q, want %q", configFile.Path, filepath.Join(home, ".aws", "config"))
	}
	if _, err := os.Stat(configFile.Path); err != nil {
		t.Fatalf("config file stat error = %v", err)
	}
}

func TestNewCredentialsFileCreatesAWSDirectoryWhenMissing(t *testing.T) {
	home := t.TempDir()

	credentialsFile, err := NewCredentialsFile(home)
	if err != nil {
		t.Fatalf("NewCredentialsFile() error = %v", err)
	}
	if credentialsFile.Path != filepath.Join(home, ".aws", "credentials") {
		t.Fatalf("Path = %q, want %q", credentialsFile.Path, filepath.Join(home, ".aws", "credentials"))
	}
	if _, err := os.Stat(credentialsFile.Path); err != nil {
		t.Fatalf("credentials file stat error = %v", err)
	}
}

func TestConfigFileSSOEmptyReturnsTrueForInvalidConfig(t *testing.T) {
	home := t.TempDir()
	awsDir := filepath.Join(home, ".aws")
	if err := os.MkdirAll(awsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	configPath := filepath.Join(awsDir, "config")
	if err := os.WriteFile(configPath, []byte("[profile broken\norg=dev\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if !ConfigFileSSOEmpty(home, "dev") {
		t.Fatal("ConfigFileSSOEmpty() = false, want true for invalid config")
	}
}

func TestIsValidEntryRequiresOrganizationAndAutoPopulatedKey(t *testing.T) {
	file := ini.Empty()
	withOrg, err := file.NewSection("profile dev")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = withOrg.NewKey("org", "dev")
	_, _ = withOrg.NewKey("sso_auto_populated", "true")
	if !IsValidEntry(withOrg, "dev") {
		t.Fatal("IsValidEntry() = false, want true")
	}
	if IsValidEntry(withOrg, "prod") {
		t.Fatal("IsValidEntry() = true, want false for mismatched org")
	}

	missingAuto, err := file.NewSection("profile no-auto")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = missingAuto.NewKey("org", "dev")
	if IsValidEntry(missingAuto, "dev") {
		t.Fatal("IsValidEntry() = true, want false without sso_auto_populated")
	}

	missingOrg, err := file.NewSection("profile no-org")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = missingOrg.NewKey("sso_auto_populated", "true")
	if IsValidEntry(missingOrg, "dev") {
		t.Fatal("IsValidEntry() = true, want false without org")
	}
}

func TestGetentryByAWSProfileReturnsErrorForMissingSection(t *testing.T) {
	home := t.TempDir()
	configFile, err := NewConfigFile(home)
	if err != nil {
		t.Fatalf("NewConfigFile() error = %v", err)
	}

	if _, err := configFile.GetentryByAWSProfile("missing"); err == nil {
		t.Fatal("GetentryByAWSProfile() error = nil, want error")
	}
}

func TestCleanTemporaryRolesRemovesOnlyMatchingOrganization(t *testing.T) {
	home := t.TempDir()
	configFile, err := NewConfigFile(home)
	if err != nil {
		t.Fatalf("NewConfigFile() error = %v", err)
	}

	keepOtherOrg, err := configFile.File.NewSection("profile keep-other-org")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = keepOtherOrg.NewKey("org", "prod")
	_, _ = keepOtherOrg.NewKey("sso_auto_populated", "true")

	keepManual, err := configFile.File.NewSection("profile keep-manual")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = keepManual.NewKey("org", "dev")

	removeMatch, err := configFile.File.NewSection("profile remove-match")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = removeMatch.NewKey("org", "dev")
	_, _ = removeMatch.NewKey("sso_auto_populated", "true")

	configFile.CleanTemporaryRoles("dev")

	if _, err := configFile.File.GetSection("profile remove-match"); err == nil {
		t.Fatal("matched auto-populated section was not removed")
	}
	if _, err := configFile.File.GetSection("profile keep-other-org"); err != nil {
		t.Fatal("non-matching org section was removed")
	}
	if _, err := configFile.File.GetSection("profile keep-manual"); err != nil {
		t.Fatal("manual section was removed")
	}
}

func TestCleanTemporaryRolesAllOrganizationsRemovesOnlyAutoPopulatedSections(t *testing.T) {
	home := t.TempDir()
	configFile, err := NewConfigFile(home)
	if err != nil {
		t.Fatalf("NewConfigFile() error = %v", err)
	}

	keepManual, err := configFile.File.NewSection("profile keep-manual")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = keepManual.NewKey("org", "dev")

	removeDev, err := configFile.File.NewSection("profile remove-dev")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = removeDev.NewKey("org", "dev")
	_, _ = removeDev.NewKey("sso_auto_populated", "true")

	removeProd, err := configFile.File.NewSection("profile remove-prod")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = removeProd.NewKey("org", "prod")
	_, _ = removeProd.NewKey("sso_auto_populated", "true")

	configFile.CleanTemporaryRoles("")

	if _, err := configFile.File.GetSection("profile remove-dev"); err == nil {
		t.Fatal("dev auto-populated section was not removed")
	}
	if _, err := configFile.File.GetSection("profile remove-prod"); err == nil {
		t.Fatal("prod auto-populated section was not removed")
	}
	if _, err := configFile.File.GetSection("profile keep-manual"); err != nil {
		t.Fatal("manual section was removed")
	}
}

func TestCleanExpiredCredentialsRemovesOnlyExpiredMatchingSections(t *testing.T) {
	home := t.TempDir()
	configFile, err := NewConfigFile(home)
	if err != nil {
		t.Fatalf("NewConfigFile() error = %v", err)
	}

	expiredDev, err := configFile.File.NewSection("profile expired-dev")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = expiredDev.NewKey("org", "dev")
	_, _ = expiredDev.NewKey("sso_auto_populated", "true")
	_, _ = expiredDev.NewKey("expires_time", "100")

	activeDev, err := configFile.File.NewSection("profile active-dev")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = activeDev.NewKey("org", "dev")
	_, _ = activeDev.NewKey("sso_auto_populated", "true")
	_, _ = activeDev.NewKey("expires_time", "200")

	otherOrgExpired, err := configFile.File.NewSection("profile expired-prod")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = otherOrgExpired.NewKey("org", "prod")
	_, _ = otherOrgExpired.NewKey("sso_auto_populated", "true")
	_, _ = otherOrgExpired.NewKey("expires_time", "50")

	manualExpired, err := configFile.File.NewSection("profile manual-expired")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = manualExpired.NewKey("org", "dev")
	_, _ = manualExpired.NewKey("expires_time", "25")

	invalidExpiration, err := configFile.File.NewSection("profile invalid-expiration")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = invalidExpiration.NewKey("org", "dev")
	_, _ = invalidExpiration.NewKey("sso_auto_populated", "true")
	_, _ = invalidExpiration.NewKey("expires_time", "bad")

	missingExpiration, err := configFile.File.NewSection("profile missing-expiration")
	if err != nil {
		t.Fatalf("NewSection() error = %v", err)
	}
	_, _ = missingExpiration.NewKey("org", "dev")
	_, _ = missingExpiration.NewKey("sso_auto_populated", "true")

	configFile.CleanExpiredCredentials("dev", 150)

	if _, err := configFile.File.GetSection("profile expired-dev"); err == nil {
		t.Fatal("expired matching section was not removed")
	}
	if _, err := configFile.File.GetSection("profile active-dev"); err != nil {
		t.Fatal("active matching section was removed")
	}
	if _, err := configFile.File.GetSection("profile expired-prod"); err != nil {
		t.Fatal("other-org section was removed")
	}
	if _, err := configFile.File.GetSection("profile manual-expired"); err != nil {
		t.Fatal("manual section was removed")
	}
	if _, err := configFile.File.GetSection("profile invalid-expiration"); err != nil {
		t.Fatal("invalid-expiration section was removed")
	}
	if _, err := configFile.File.GetSection("profile missing-expiration"); err != nil {
		t.Fatal("missing-expiration section was removed")
	}
}
