package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphanalysis"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-arch][Late]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("certificate-checker")

	g.It("collect certificate data", func() {

		ctx := context.Background()
		kubeClient := oc.AdminKubeClient()
		if ok, _ := exutil.IsMicroShiftCluster(kubeClient); ok {
			g.Skip("microshift does not auto-collect TLS.")
		}
		jobType, err := platformidentification.GetJobType(context.TODO(), oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		tlsArtifactFilename := fmt.Sprintf("raw-tls-artifacts-%s-%s-%s-%s.json", jobType.Topology, jobType.Architecture, jobType.Platform, jobType.Network)

		currentPKIContent, err := certgraphanalysis.GatherCertsFromPlatformNamespaces(ctx, kubeClient, certgraphanalysis.SkipRevisioned)
		o.Expect(err).NotTo(o.HaveOccurred())

		// the content here is good, but proxy-ca contains a lot of entries for system-trust that doesn't help
		// us visualize the OCP certs, so if we detect that condition snip it
		pruneSystemTrust(currentPKIContent)

		jsonBytes, err := json.MarshalIndent(currentPKIContent, "", "  ")
		o.Expect(err).NotTo(o.HaveOccurred())

		pkiDir := filepath.Join(exutil.ArtifactDirPath(), "rawTLSInfo")
		err = os.MkdirAll(pkiDir, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = os.WriteFile(filepath.Join(pkiDir, tlsArtifactFilename), jsonBytes, 0644)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

})

// pruneSystemTrust removes certificate metadata for proxy-ca for easier visualization
func pruneSystemTrust(pkiList *certgraphapi.PKIList) {
	for i := range pkiList.CertificateAuthorityBundles.Items {
		curr := pkiList.CertificateAuthorityBundles.Items[i]
		if curr.LogicalName != "proxy-ca" {
			continue
		}

		if len(curr.Spec.CertificateMetadata) > 10 {
			pkiList.CertificateAuthorityBundles.Items[i].Name = "proxy-ca"
			pkiList.CertificateAuthorityBundles.Items[i].Spec.CertificateMetadata = []certgraphapi.CertKeyMetadata{
				{
					CertIdentifier: certgraphapi.CertIdentifier{
						CommonName:   "synthetic-proxy-ca",
						SerialNumber: "0",
						Issuer:       nil,
					},
				},
			}
			return
		}
	}

}
