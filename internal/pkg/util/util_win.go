//go:build windows

package util

import (
	"errors"
	"fmt"
	"os"
	"os/user"
)

func HomeDir() (string, error) {
	if home := os.Getenv("USERPROFILE"); home != "" {
		return home, nil
	}
	return "", errors.New("USERPROFILE env var does not exist")
}

func ValidateSuperuserFile(path string, user *user.User) string {
	_, err := os.Stat(path)
	if err != nil {
		return fmt.Sprintf("The %s file doesn't exist, will try to create it", path)
	}
	return ""
}
