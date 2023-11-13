package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-cmp/cmp"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphanalysis"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphutils"

	"github.com/openshift/origin/pkg/certs"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	testresult "github.com/openshift/origin/pkg/test/ginkgo/result"
	exutil "github.com/openshift/origin/test/extended/util"
	ownership "github.com/openshift/origin/tls"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

		currentPKIContent, err := certgraphanalysis.GatherCertsFromPlatformNamespaces(ctx, kubeClient, certgraphanalysis.SkipRevisioned, certgraphanalysis.ElideProxyCADetails)
		o.Expect(err).NotTo(o.HaveOccurred())

		jsonBytes, err := json.MarshalIndent(currentPKIContent, "", "  ")
		o.Expect(err).NotTo(o.HaveOccurred())

		pkiDir := filepath.Join(exutil.ArtifactDirPath(), "rawTLSInfo")
		err = os.MkdirAll(pkiDir, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = os.WriteFile(filepath.Join(pkiDir, tlsArtifactFilename), jsonBytes, 0644)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("all tls artifacts must be registered", func() {

		ctx := context.Background()
		kubeClient := oc.AdminKubeClient()
		if ok, _ := exutil.IsMicroShiftCluster(kubeClient); ok {
			g.Skip("microshift does not auto-collect TLS.")
		}

		nodes := map[string]int{}
		controlPlaneLabel := labels.SelectorFromSet(map[string]string{"node-role.kubernetes.io/control-plane": ""})
		nodeList, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: controlPlaneLabel.String()})
		o.Expect(err).NotTo(o.HaveOccurred())
		for i, node := range nodeList.Items {
			nodes[node.Name] = i
		}

		actualPKIContent, err := certgraphanalysis.GatherCertsFromPlatformNamespaces(ctx, kubeClient)
		o.Expect(err).NotTo(o.HaveOccurred())

		expectedPKIContent, err := certs.GetPKIInfoFromEmbeddedOwnership(ownership.PKIOwnership)
		o.Expect(err).NotTo(o.HaveOccurred())

		violationsPKIContent, err := certs.GetPKIInfoFromEmbeddedOwnership(ownership.PKIViolations)
		o.Expect(err).NotTo(o.HaveOccurred())

		newTLSRegistry := &certgraphapi.PKIRegistryInfo{}

		for _, currCertKeyPair := range actualPKIContent.InClusterResourceData.CertKeyPairs {
			currLocation := currCertKeyPair.SecretLocation
			if _, err := certgraphutils.LocateCertKeyPair(currLocation, violationsPKIContent.CertKeyPairs); err == nil {
				continue
			}

			_, err := certgraphutils.LocateCertKeyPair(currLocation, expectedPKIContent.CertKeyPairs)
			if err != nil {
				newTLSRegistry.CertKeyPairs = append(newTLSRegistry.CertKeyPairs, currCertKeyPair)
			}
		}

		for _, currCABundle := range actualPKIContent.InClusterResourceData.CertificateAuthorityBundles {
			currLocation := currCABundle.ConfigMapLocation
			if _, err := certgraphutils.LocateCertificateAuthorityBundle(currLocation, violationsPKIContent.CertificateAuthorityBundles); err == nil {
				continue
			}

			_, err := certgraphutils.LocateCertificateAuthorityBundle(currLocation, expectedPKIContent.CertificateAuthorityBundles)
			if err != nil {
				newTLSRegistry.CertificateAuthorityBundles = append(newTLSRegistry.CertificateAuthorityBundles, currCABundle)
			}
		}

		if len(newTLSRegistry.CertKeyPairs) > 0 || len(newTLSRegistry.CertificateAuthorityBundles) > 0 {
			registryString, err := json.MarshalIndent(newTLSRegistry, "", "  ")
			if err != nil {
				//g.Fail("Failed to marshal registry %#v: %v", newTLSRegistry, err)
				testresult.Flakef("Failed to marshal registry %#v: %v", newTLSRegistry, err)
			}
			// TODO: uncomment when test no longer fails and enhancement is merged
			//g.Fail(fmt.Sprintf("Unregistered TLS certificates:\n%s", registryString))
			testresult.Flakef(fmt.Sprintf("Unregistered TLS certificates found:\n%s\nSee tls/ownership/README.md in origin repo", registryString))
		}
	})

	g.It("all registered tls artifacts must have expected owners", func() {

		ctx := context.Background()
		kubeClient := oc.AdminKubeClient()
		if ok, _ := exutil.IsMicroShiftCluster(kubeClient); ok {
			g.Skip("microshift does not auto-collect TLS.")
		}

		nodes := map[string]int{}
		controlPlaneLabel := labels.SelectorFromSet(map[string]string{"node-role.kubernetes.io/control-plane": ""})
		nodeList, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: controlPlaneLabel.String()})
		o.Expect(err).NotTo(o.HaveOccurred())
		for i, node := range nodeList.Items {
			nodes[node.Name] = i
		}

		actualPKIContent, err := certgraphanalysis.GatherCertsFromPlatformNamespaces(ctx, kubeClient)
		o.Expect(err).NotTo(o.HaveOccurred())

		expectedPKIContent, err := certs.GetPKIInfoFromEmbeddedOwnership(ownership.PKIOwnership)
		o.Expect(err).NotTo(o.HaveOccurred())

		messages := []string{}

		for _, currCertKeyPair := range actualPKIContent.InClusterResourceData.CertKeyPairs {
			currLocation := currCertKeyPair.SecretLocation
			expectedSecret, err := certgraphutils.LocateCertKeyPair(currLocation, expectedPKIContent.CertKeyPairs)
			if err != nil {
				continue
			}
			if diff := cmp.Diff(expectedSecret.CertKeyInfo.OwningJiraComponent, currCertKeyPair.CertKeyInfo.OwningJiraComponent); len(diff) > 0 {
				messages = append(messages, fmt.Sprintf("--namespace=%s, secret/%s:\n%v\n", currLocation.Namespace, currLocation.Name, diff))
			}
		}

		for _, currCABundle := range actualPKIContent.InClusterResourceData.CertificateAuthorityBundles {
			currLocation := currCABundle.ConfigMapLocation
			expectedCABundle, err := certgraphutils.LocateCertificateAuthorityBundle(currLocation, expectedPKIContent.CertificateAuthorityBundles)
			if err != nil {
				continue
			}
			if diff := cmp.Diff(expectedCABundle.CABundleInfo.OwningJiraComponent, currCABundle.CABundleInfo.OwningJiraComponent); len(diff) > 0 {
				messages = append(messages, fmt.Sprintf("--namespace=%s, configmap/%s:\n%v\n", currLocation.Namespace, currLocation.Name, diff))
			}
		}
		if len(messages) > 0 {
			// TODO: uncomment when test no longer fails and enhancement is merged
			//g.Fail(strings.Join(messages, "\n"))
			testresult.Flakef(strings.Join(messages, "\n"))
		}
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
