package catalog

import (
	"context"
	"fmt"
	"github.com/openshift/origin/test/extended/catalog/pkg/check"
	"github.com/openshift/origin/test/extended/catalog/pkg/extract"
	"strings"
	"testing"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
)

// images contains the list of images to be validated.
var images = []string{
	"registry.redhat.io/redhat/community-operator-index:v4.19",
	"registry.redhat.io/redhat/redhat-marketplace-index:v4.19",
	"registry.redhat.io/redhat/certified-operator-index:v4.19",
	"registry.redhat.io/redhat/redhat-operator-index:v4.19",
}

func TestSuite(t *testing.T) {
	o.RegisterFailHandler(g.Fail)
	g.RunSpecs(t, "Default Catalog Consistency Tests Suite")
}

var _ = g.Describe("[sig-catalog][Skipped:Disconnected] Check Catalog Consistency", func() {
	for _, url := range images {
		name := getImageNameFrom(url)

		g.It(fmt.Sprintf("consistency check for the image: %s", name), func() {
			ctx := context.Background()
			g.By(fmt.Sprintf("Validating image: %s", url))

			res, err := extract.UnpackImage(ctx, url, name)
			o.Expect(err).ToNot(o.HaveOccurred())
			o.Expect(check.Check(ctx, res.Store, res.TmpDir, check.AllChecks())).To(o.Succeed())
			res.Cleanup()
		})
	}
})

// getImageNameFrom extracts the image name from the link/url.
func getImageNameFrom(ref string) string {
	parts := strings.Split(ref, "/")
	last := parts[len(parts)-1]
	if i := strings.Index(last, ":"); i != -1 {
		return last[:i]
	}
	return last
}
