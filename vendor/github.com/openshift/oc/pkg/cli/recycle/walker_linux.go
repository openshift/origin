// +build linux

package recycle

import (
	"errors"
	"os"
	"syscall"
)

var StatError = errors.New("fileinfo.Sys() is not *syscall.Stat_t")

func getuid(info os.FileInfo) (int64, error) {
	stat_t, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, StatError
	}
	return int64(stat_t.Uid), nil
}

func setfsuid(uid int) (err error) {
	return syscall.Setfsuid(uid)
}
