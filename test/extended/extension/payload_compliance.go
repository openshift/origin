package extension

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"github.com/openshift/origin/pkg/test/extensions"
	"k8s.io/apimachinery/pkg/util/sets"
)

// knownInfoFailures lists extension binaries that are known to fail the
// "info" command without cluster access. Each entry should have a tracking
// issue for fixing the upstream binary. Remove entries as fixes land.
var knownInfoFailures = sets.New[string](
	"ovn-kubernetes-tests-ext",               // https://github.com/openshift/ovn-kubernetes/pull/3170
	"cloud-controller-manager-aws-tests-ext", // https://github.com/openshift/cluster-cloud-controller-manager-operator/pull/458
)

var _ = g.Describe("[sig-ci] [OTE] Payload extension binaries [Suite:openshift/conformance/parallel]", func() {
	defer g.GinkgoRecover()

	g.It("should all respond to the info command", func(ctx context.Context) {
		extractCtx, extractCancel := context.WithTimeout(ctx, 30*time.Minute)
		defer extractCancel()

		cleanUpFn, allBinaries, _, err := extensions.ExtractAllTestBinaries(extractCtx, 10, extensions.WithPayloadOnly())
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to extract test binaries from payload")
		defer cleanUpFn()

		var failures []string
		for _, binary := range allBinaries {
			binName := binary.Name()
			infoCtx, infoCancel := context.WithTimeout(ctx, 10*time.Minute)
			_, err := binary.Info(infoCtx)
			infoCancel()

			if err != nil {
				if knownInfoFailures.Has(binName) {
					g.GinkgoLogr.Info("Skipping known info failure", "binary", binName, "error", err)
					continue
				}
				failures = append(failures, binName+": "+err.Error())
			} else if knownInfoFailures.Has(binName) {
				failures = append(failures, binName+": listed in knownInfoFailures but info succeeded — remove from exemption list")
			}
		}

		o.Expect(failures).To(o.BeEmpty(), "extension binaries failed the OTE info contract")
	})
})
