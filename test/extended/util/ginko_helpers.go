package util

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/origin/pkg/client"
)

// MustGetImageName verifies that there is an Docker image reference available
// for the specified ImageStream and ImageStreamTag name.
func MustGetImageName(c client.ImageStreamInterface, name, tag string) string {
	By(fmt.Sprintf("getting the Docker image reference for %s:%s", name, tag))
	name, err := GetDockerImageReference(c, name, tag)
	Expect(err).NotTo(HaveOccurred())
	Expect(name).NotTo(BeEmpty())
	return name
}
