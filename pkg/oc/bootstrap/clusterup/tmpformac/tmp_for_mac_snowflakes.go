package tmpformac

import (
	"io/ioutil"
	"runtime"
)

// TempDir wraps the ioutil.TempDir and add logic to handle private temp directories on OSX.
// This is needed in order to allow mounting configuration/certificates into Docker as the Docker
// for Mac only allows mounting from "/tmp".
func TempDir(prefix string) (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return ioutil.TempDir("/tmp", prefix)
	default:
		return ioutil.TempDir("", prefix)
	}
}
