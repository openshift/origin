package origin

import "embed"

//go:embed all:examples/db-templates
//go:embed all:examples/image-streams
//go:embed all:examples/sample-app
//go:embed all:examples/quickstarts
//go:embed all:examples/jenkins
//go:embed all:examples/quickstarts/cakephp-mysql.json
//go:embed all:test/extended/testdata
//go:embed all:e2echart
var f embed.FS

// Asset reads and returns the content of the named file.
func Asset(name string) ([]byte, error) {
	return f.ReadFile(name)
}

// MustAsset reads and returns the content of the named file or panics
// if something went wrong.
func MustAsset(name string) []byte {
	data, err := f.ReadFile(name)
	if err != nil {
		panic(err)
	}

	return data
}

// AssetDir returns the file names in a directory.
func AssetDir(name string) ([]string, error) {
	entries, err := f.ReadDir(name)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, entry := range entries {
		result = append(result, entry.Name())
	}
	return result, nil
}
