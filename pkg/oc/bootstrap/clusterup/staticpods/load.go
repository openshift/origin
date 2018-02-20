package staticpods

import (
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"

	"github.com/openshift/origin/pkg/oc/bootstrap"
)

func substitute(in string, replacements map[string]string) string {
	curr := in
	for oldVal, newVal := range replacements {
		curr = strings.Replace(curr, oldVal, newVal, -1)
	}

	return curr
}

func UpsertStaticPod(sourceLocation string, replacements map[string]string, kubeletStaticPodDir string) error {
	data, err := bootstrap.Asset(sourceLocation)
	if err != nil {
		return err
	}

	content := substitute(string(data), replacements)
	fullLockubeletStaticPodDir := path.Join(kubeletStaticPodDir, filepath.Base(sourceLocation))
	return ioutil.WriteFile(fullLockubeletStaticPodDir, []byte(content), 0644)
}
