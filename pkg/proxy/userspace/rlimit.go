// +build !windows

package userspace

import "syscall"

func setRLimit(limit uint64) error {
	return syscall.Setrlimit(syscall.RLIMIT_NOFILE, &syscall.Rlimit{Max: limit, Cur: limit})
}
