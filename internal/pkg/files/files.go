package files

import (
	"errors"
	"os"
	"path"

	"github.com/JorgeReus/aws-sso-creds/internal/pkg/util"

	"gopkg.in/ini.v1"
)

func NewConfigFile() (*AWSFile, error) {
	homedir, err := util.HomeDir()
	if err != nil {
		return nil, err
	}
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

func ConfigFileSSOEmpty() bool {
	homedir, err := util.HomeDir()
	if err != nil {
		return true
	}
	path := path.Join(homedir, ".aws", "config")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return true
	}

	file, err := ini.Load(path)
	isFresh := true
	for _, section := range file.Sections() {
		if section.HasKey("sso_auto_populated") {
			isFresh = false
			break
		}
	}
	return isFresh
}

func NewCredentialsFile() (*AWSFile, error) {
	homedir, err := util.HomeDir()
	if err != nil {
		return nil, err
	}
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

func (c *AWSFile) CleanTemporaryRoles() {
	for _, section := range c.File.Sections() {
		if section.HasKey("sso_auto_populated") {
			c.File.DeleteSection(section.Name())
		}
	}
}
