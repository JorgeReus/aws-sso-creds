//go:build !windows

package util

import (
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"syscall"

	"github.com/mitchellh/go-homedir"
)

func HomeDir() (string, error) {
	return homedir.Dir()
}

func ValidateSuperuserFile(path string, user *user.User) string {
	var info fs.FileInfo
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Sprintf("The %s file doesn't exist, will try to create it", path)
	} else {
		stat := info.Sys().(*syscall.Stat_t)

		// If the file is owned by root and you are not root, print warning
		if (stat.Uid == 0 || stat.Gid == 0) && user.Uid != "0" {
			return fmt.Sprintf("The %s file is owned by root, this can cause problems", path)
		}
	}
	return ""
}
