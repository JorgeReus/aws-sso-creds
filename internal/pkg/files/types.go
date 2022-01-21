package files

import (
	"gopkg.in/ini.v1"
)

type AWSFile struct {
	File *ini.File
	Path string
}
