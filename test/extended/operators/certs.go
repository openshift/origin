package operators

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphanalysis"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/sets"
)

var _ = g.Describe("[sig-arch][Late]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("certificate-checker")

	g.It("all tls artifacts must be known", func() {
		ctx := context.Background()
		kubeClient := oc.AdminKubeClient()
		currentPKIContent, err := certgraphanalysis.GatherCertsFromAllNamespaces(ctx, kubeClient)
		o.Expect(err).NotTo(o.HaveOccurred())

		jsonBytes, err := json.MarshalIndent(currentPKIContent, "", "  ")
		o.Expect(err).NotTo(o.HaveOccurred())

		pkiDir := filepath.Join(exutil.ArtifactDirPath(), "tls_for_cluster")
		err = os.MkdirAll(pkiDir, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = os.WriteFile(filepath.Join(pkiDir, "all-tls-artifacts.json"), jsonBytes, 0644)
		o.Expect(err).NotTo(o.HaveOccurred())

		// TODO read from vendored openshift/api where approvers approve items
		previousPKIContent := &certgraphapi.PKIList{}

		newSecrets := map[certgraphapi.InClusterSecretLocation]certgraphapi.CertKeyPair{}
		secretsWithoutComponents := map[certgraphapi.InClusterSecretLocation]certgraphapi.CertKeyPair{}
		for _, currCertKeyPair := range currentPKIContent.CertKeyPairs.Items {
			for _, currLocation := range currCertKeyPair.Spec.SecretLocations {
				// TODO add a field for "responsibleComponent" set based on annotation, still key based on namespace,name tuple
				// TODO possibly a look-aside info at the top level of CertKeyPairs

				_, err := locateCertKeyPair(currLocation, previousPKIContent.CertKeyPairs.Items)
				if err == nil {
					continue
				}
				newSecrets[currLocation] = currCertKeyPair
			}
		}

		newConfigMaps := map[certgraphapi.InClusterConfigMapLocation]certgraphapi.CertificateAuthorityBundle{}
		configMapsWithoutComponents := map[certgraphapi.InClusterConfigMapLocation]certgraphapi.CertificateAuthorityBundle{}
		for _, currCABundle := range currentPKIContent.CertificateAuthorityBundles.Items {
			for _, currLocation := range currCABundle.Spec.ConfigMapLocations {
				// TODO add a field for "responsibleComponent" set based on annotation, still key based on namespace,name tuple
				// TODO possibly a look-aside info at the top level of CertKeyPairs

				_, err := locateCertificateAuthorityBundle(currLocation, previousPKIContent.CertificateAuthorityBundles.Items)
				if err == nil {
					continue
				}
				newConfigMaps[currLocation] = currCABundle
			}
		}

		messages := []string{}
		if len(newSecrets) > 0 || len(newConfigMaps) > 0 {
			secretKeys := sets.KeySet(newSecrets).UnsortedList()
			sort.Sort(secretLocationByNamespaceName(secretKeys))
			configMapKeys := sets.KeySet(newConfigMaps).UnsortedList()
			sort.Sort(configMapLocationByNamespaceName(configMapKeys))

			messages = append(messages, "")
			messages = append(messages, "######")
			messages = append(messages, "new TLS artifacts like certificates and ca bundles must be registered")
			for _, k := range secretKeys {
				messages = append(messages, fmt.Sprintf("--namespace=%v secret/%v", k.Namespace, k.Name))
			}
			for _, k := range configMapKeys {
				messages = append(messages, fmt.Sprintf("--namespace=%v configmap/%v", k.Namespace, k.Name))
			}
		}

		if len(secretsWithoutComponents) > 0 || len(configMapsWithoutComponents) > 0 {
			secretKeys := sets.KeySet(secretsWithoutComponents).UnsortedList()
			sort.Sort(secretLocationByNamespaceName(secretKeys))
			configMapKeys := sets.KeySet(configMapsWithoutComponents).UnsortedList()
			sort.Sort(configMapLocationByNamespaceName(configMapKeys))

			messages = append(messages, "")
			messages = append(messages, "######")
			messages = append(messages, "all TLS artifacts like certificates and ca bundles must have Jira components")
			for _, k := range secretKeys {
				messages = append(messages, fmt.Sprintf("--namespace=%v secret/%v", k.Namespace, k.Name))
			}
			for _, k := range configMapKeys {
				messages = append(messages, fmt.Sprintf("--namespace=%v configmap/%v", k.Namespace, k.Name))
			}
		}

		if len(messages) > 0 {
			g.Fail(strings.Join(messages, "\n"))
		}
	})

})

// TODO move to library

type secretLocationByNamespaceName []certgraphapi.InClusterSecretLocation

func (n secretLocationByNamespaceName) Len() int      { return len(n) }
func (n secretLocationByNamespaceName) Swap(i, j int) { n[i], n[j] = n[j], n[i] }
func (n secretLocationByNamespaceName) Less(i, j int) bool {
	if n[i].Namespace < n[j].Namespace {
		return true
	}
	return n[i].Name < n[j].Name
}

type configMapLocationByNamespaceName []certgraphapi.InClusterConfigMapLocation

func (n configMapLocationByNamespaceName) Len() int      { return len(n) }
func (n configMapLocationByNamespaceName) Swap(i, j int) { n[i], n[j] = n[j], n[i] }
func (n configMapLocationByNamespaceName) Less(i, j int) bool {
	if n[i].Namespace < n[j].Namespace {
		return true
	}
	return n[i].Name < n[j].Name
}

func locateCertKeyPair(targetLocation certgraphapi.InClusterSecretLocation, certKeyPairs []certgraphapi.CertKeyPair) (*certgraphapi.CertKeyPair, error) {
	for i, curr := range certKeyPairs {
		for _, location := range curr.Spec.SecretLocations {
			if location == targetLocation {
				return &certKeyPairs[i], nil
			}
		}
	}

	return nil, fmt.Errorf("not found: %#v", targetLocation)
}

func locateCertificateAuthorityBundle(targetLocation certgraphapi.InClusterConfigMapLocation, certKeyPairs []certgraphapi.CertificateAuthorityBundle) (*certgraphapi.CertificateAuthorityBundle, error) {
	for i, curr := range certKeyPairs {
		for _, location := range curr.Spec.ConfigMapLocations {
			if location == targetLocation {
				return &certKeyPairs[i], nil
			}
		}
	}

	return nil, fmt.Errorf("not found: %#v", targetLocation)
}
