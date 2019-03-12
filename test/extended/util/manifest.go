package util

import (
	"fmt"
	"io/ioutil"

	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/origin/test/extended/scheme"
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
