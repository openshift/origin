package jenkinsfile

import (
	"os"
	"path/filepath"

	"github.com/openshift/origin/pkg/oc/generate"
)

type tester bool

func (t tester) Has(dir string) (string, bool, error) {
	path := filepath.Join(dir, "Jenkinsfile")
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return path, true, nil
}

func NewTester() generate.Tester {
	return tester(true)
}
