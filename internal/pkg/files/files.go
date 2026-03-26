package files

import (
	"errors"
	"fmt"
	"os"
	"path"
	pathpkg "path/filepath"
	"strconv"

	ini "gopkg.in/ini.v1"
)

func NewConfigFile(homedir string) (*AWSFile, error) {
	path := path.Join(homedir, ".aws", "config")
	if err := os.MkdirAll(pathpkg.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o644)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
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
	if s.HasKey("sso_auto_populated") && (organization == "" || org.String() == organization) {
		return true
	}

	return false
}

func (f *AWSFile) GetentryByAWSProfile(profile string) (*ini.Section, error) {
	section, err := f.File.GetSection(fmt.Sprintf("profile %s", profile))
	if err != nil {
		return nil, err
	}
	return section, nil
}

func NewCredentialsFile(homedir string) (*AWSFile, error) {
	path := path.Join(homedir, ".aws", "credentials")
	if err := os.MkdirAll(pathpkg.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o644)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
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
	sections := make([]string, 0)
	for _, section := range c.File.Sections() {
		if IsValidEntry(section, organization) {
			sections = append(sections, section.Name())
		}
	}
	for _, section := range sections {
		c.File.DeleteSection(section)
	}
}

func (c *AWSFile) CleanExpiredCredentials(organization string, nowUnix int64) {
	sections := make([]string, 0)
	for _, section := range c.File.Sections() {
		if !IsValidEntry(section, organization) {
			continue
		}
		expiresTime, err := section.GetKey("expires_time")
		if err != nil {
			continue
		}
		expiresUnix, err := strconv.ParseInt(expiresTime.String(), 10, 64)
		if err != nil {
			continue
		}
		if expiresUnix < nowUnix {
			sections = append(sections, section.Name())
		}
	}
	for _, section := range sections {
		c.File.DeleteSection(section)
	}
}
