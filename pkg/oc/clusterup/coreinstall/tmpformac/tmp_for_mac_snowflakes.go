package tmpformac

import (
	"io/ioutil"
	"runtime"
)

// TempDir wraps the ioutil.TempDir and add logic to handle private temp directories on OSX.
// This is needed in order to allow mounting configuration/certificates into Docker as the Docker
// for Mac only allows mounting from "/tmp".
func TempDir(basedir, prefix string) (string, error) {
	if runtime.GOOS == "darwin" && len(basedir) == 0 {
		basedir = "/tmp"
	}
	return ioutil.TempDir(basedir, prefix)
}
