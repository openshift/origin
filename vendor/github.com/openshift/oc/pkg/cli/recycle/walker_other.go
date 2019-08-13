// +build !linux

package recycle

import "os"

func getuid(info os.FileInfo) (int64, error) {
	// no-op on non-linux platforms
	return 0, nil
}

func setfsuid(uid int) (err error) {
	// no-op on non-linux platforms
	return nil
}
