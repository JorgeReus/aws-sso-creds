package files

import (
	"errors"
	"fmt"
	"os"
	"path"

	"gopkg.in/ini.v1"
)

func NewConfigFile(homedir string) (*AWSFile, error) {
	path := path.Join(homedir, ".aws", "config")
	f, err := os.OpenFile(path, os.O_CREATE, 0644)
	f.Close()
	file, err := ini.Load(path)
	if err != nil {
		return nil, err
	}
	return &AWSFile{
		File: file,
		Path: path,
	}, nil
}

func ConfigFileSSOEmpty(homedir string, organization string) bool {
	path := path.Join(homedir, ".aws", "config")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return true
	}

	file, err := ini.Load(path)
	if err != nil {
		return true
	}
	isFresh := true
	for _, section := range file.Sections() {

		if IsValidEntry(section, organization) {
			isFresh = false
			break
		}
	}
	return isFresh
}

func IsValidEntry(s *ini.Section, organization string) bool {
	org, err := s.GetKey("org")
	if err != nil {
		return false
	}
	if s.HasKey("sso_auto_populated") && org.String() == organization {
		return true
	}

	return false
}

func(f *AWSFile) GetentryByAWSProfile(profile string) (*ini.Section, error) {
  section, err := f.File.GetSection(fmt.Sprintf("profile %s", profile))
  if err != nil {
    return nil, err
  }
  return section, nil
}

func NewCredentialsFile(homedir string) (*AWSFile, error) {
	path := path.Join(homedir, ".aws", "credentials")
	f, err := os.OpenFile(path, os.O_CREATE, 0644)
	f.Close()
	file, err := ini.Load(path)
	if err != nil {
		return nil, err
	}
	return &AWSFile{
		File: file,
		Path: path,
	}, nil
}

func (f *AWSFile) Save() error {
	return f.File.SaveTo(f.Path)
}

func (c *AWSFile) CleanTemporaryRoles(organization string) {
	for _, section := range c.File.Sections() {
		if IsValidEntry(section, organization) {
			c.File.DeleteSection(section.Name())
		}
	}
}
