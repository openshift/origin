package compat_otp

import exutil "github.com/openshift/origin/test/extended/util"

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	o "github.com/onsi/gomega"
	scheme "github.com/openshift/origin/test/extended/util/compat_otp/scheme"

	"k8s.io/apimachinery/pkg/runtime"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func ReadFixture(path string) (runtime.Object, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %v", path, err)
	}

	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(data, nil, nil)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func ReadFixtureOrFail(path string) runtime.Object {
	obj, err := ReadFixture(path)

	o.Expect(err).NotTo(o.HaveOccurred())

	return obj
}

// Get file content in test/extended/testdata/<basedir>/<name>
func GetFileContent(baseDir string, name string) (fileContent string) {
	filePath := filepath.Join(FixturePath("testdata", baseDir), name)
	fileOpen, err := os.Open(filePath)
	if err != nil {
		e2e.Failf("Failed to open file: %s", filePath)
	}
	fileRead, err := io.ReadAll(fileOpen)
	if err != nil {
		e2e.Failf("Failed to read file: %s", filePath)
	}
	return string(fileRead)
}

/*
This function accept key value replacement in multiple formats
manifestFile, err := GenerateManifestFile(oc, "config-map.yaml", "myDir", map[string]string{"<address>": address, "<username>": user})
manifestFile, err := GenerateManifestFile(oc, "namespace.yaml", "myDir", map[string]string{"<namespace>": namespace})
*/
func GenerateManifestFile(oc *exutil.CLI, baseDir string, manifestFile string, replacement ...map[string]string) (string, error) {
	manifest := GetFileContent(baseDir, manifestFile)

	for _, m := range replacement {
		for key, value := range m {
			manifest = strings.ReplaceAll(manifest, key, value)
		}
	}
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	splitFileName := strings.Split(manifestFile, ".")
	manifestFileName := splitFileName[0] + strings.Replace(ts, ":", "", -1) + "." + splitFileName[1] // get rid of offensive colons
	err := os.WriteFile(manifestFileName, []byte(manifest), 0644)
	return manifestFileName, err
}
