package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatadefaults"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatainterfaces"

	ensure_no_violation_regression "github.com/openshift/origin/pkg/cmd/update-tls-artifacts/ensure-no-violation-regression"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/api/annotations"
	configv1 "github.com/openshift/api/config/v1"

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func gatherCertsFromPlatformNamespaces(ctx context.Context, kubeClient kubernetes.Interface) (*certgraphapi.PKIList, error) {
	controlPlaneLabel := labels.SelectorFromSet(map[string]string{"node-role.kubernetes.io/control-plane": ""})
	nodeList, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: controlPlaneLabel.String()})
	o.Expect(err).NotTo(o.HaveOccurred())
	masters := []*corev1.Node{}
	for i := range nodeList.Items {
		masters = append(masters, &nodeList.Items[i])
	}

	annotationsToCollect := []string{annotations.OpenShiftComponent}
	for _, currRequirement := range tlsmetadatadefaults.GetDefaultTLSRequirements() {
		annotationRequirement, ok := currRequirement.(tlsmetadatainterfaces.AnnotationRequirement)
		if ok {
			annotationsToCollect = append(annotationsToCollect, annotationRequirement.GetAnnotationName())
		}
	}

	return certgraphanalysis.GatherCertsFromPlatformNamespaces(ctx, kubeClient,
		certgraphanalysis.SkipRevisioned,
		certgraphanalysis.SkipHashed,
		certgraphanalysis.ElideProxyCADetails,
		certgraphanalysis.RewriteNodeIPs(masters),
		certgraphanalysis.CollectAnnotations(annotationsToCollect...),
	)
}

var _ = g.Describe("[sig-arch][Late]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("certificate-checker")

	g.It("collect certificate data", func() {

		ctx := context.Background()
		kubeClient := oc.AdminKubeClient()
		if ok, _ := exutil.IsMicroShiftCluster(kubeClient); ok {
			g.Skip("microshift does not auto-collect TLS.")
		}
		configClient := oc.AdminConfigClient()
		featureGates, err := configClient.ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		jobType, err := platformidentification.GetJobType(context.TODO(), oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		featureSetString := string(featureGates.Spec.FeatureSet)
		if len(featureSetString) == 0 {
			featureSetString = "Default"
		}
		tlsArtifactFilename := fmt.Sprintf(
			"raw-tls-artifacts-%s-%s-%s-%s-%s.json",
			jobType.Topology,
			jobType.Architecture,
			jobType.Platform,
			jobType.Network,
			strings.ToLower(featureSetString),
		)

		currentPKIContent, err := gatherCertsFromPlatformNamespaces(ctx, kubeClient)
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
		configClient := oc.AdminConfigClient()
		infra, err := configClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if infra.Status.ControlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("hypershift creates TLS differently and we're not yet ready.")
		}

		actualPKIContent, err := gatherCertsFromPlatformNamespaces(ctx, kubeClient)
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

	g.It("all registered tls artifacts must have no metadata violation regressions", func() {

		ctx := context.Background()
		kubeClient := oc.AdminKubeClient()
		if ok, _ := exutil.IsMicroShiftCluster(kubeClient); ok {
			g.Skip("microshift does not auto-collect TLS.")
		}
		configClient := oc.AdminConfigClient()
		infra, err := configClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if infra.Status.ControlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("hypershift creates TLS differently and we're not yet ready.")
		}

		actualPKIContent, err := gatherCertsFromPlatformNamespaces(ctx, kubeClient)
		o.Expect(err).NotTo(o.HaveOccurred())

		violationRegressionOptions := ensure_no_violation_regression.NewEnsureNoViolationRegressionOptions(ownership.AllViolations, genericclioptions.NewTestIOStreamsDiscard())
		messages, _, err := violationRegressionOptions.HaveViolationsRegressed([]*certgraphapi.PKIList{actualPKIContent})
		o.Expect(err).NotTo(o.HaveOccurred())

		if len(messages) > 0 {
			// TODO: uncomment when test no longer fails and enhancement is merged
			//g.Fail(strings.Join(messages, "\n"))
			testresult.Flakef(strings.Join(messages, "\n"))
		}
	})

})
