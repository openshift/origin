package testdata

import "github.com/openshift/origin"

// Asset reads and returns the content of the named file.
func Asset(name string) ([]byte, error) {
	return origin.Asset(name)
}

// MustAsset reads and returns the content of the named file or panics
// if something went wrong.
func MustAsset(name string) []byte {
	return origin.MustAsset(name)
}

// AssetDir returns the file names in a directory.
func AssetDir(name string) ([]string, error) {
	return origin.AssetDir(name)
}
