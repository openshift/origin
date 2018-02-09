package tmputil

import (
	"io/ioutil"
	"os"
	"runtime"
)

// TempDir wraps the ioutil.TempDir and add logic to handle private temp directories on OSX.
// This is needed in order to allow mounting configuration/certificates into Docker as the Docker
// for Mac only allows mounting from "/tmp".
func TempDir(prefix string) (string, error) {
	var tmpDirFn func() string
	switch runtime.GOOS {
	case "darwin":
		tmpDirFn = func() string { return "/tmp" }
	default:
		tmpDirFn = os.TempDir
	}
	return ioutil.TempDir(tmpDirFn(), prefix)
}
