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
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/sets"
)

var _ = g.Describe("[sig-arch][Late]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("certificate-checker")

	g.It("all tls artifacts must be known", func() {

		ctx := context.Background()
		kubeClient := oc.AdminKubeClient()
		if ok, _ := exutil.IsMicroShiftCluster(kubeClient); ok {
			g.Skip("microshift does not auto-collect TLS.")
		}
		jobType, err := platformidentification.GetJobType(context.TODO(), oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())
		tlsArtifactFilename := fmt.Sprintf("tls-artifacts-%s-%s-%s-%s.json", jobType.Topology, jobType.Architecture, jobType.Platform, jobType.Network)

		currentPKIContent, err := certgraphanalysis.GatherCertsFromPlatformNamespaces(ctx, kubeClient)
		o.Expect(err).NotTo(o.HaveOccurred())

		tlsRegistryInfo := allCertsToPKIRegistry(currentPKIContent)
		jsonBytes, err := json.MarshalIndent(tlsRegistryInfo, "", "  ")
		o.Expect(err).NotTo(o.HaveOccurred())

		pkiDir := filepath.Join(exutil.ArtifactDirPath(), "tls_for_cluster")
		err = os.MkdirAll(pkiDir, 0755)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = os.WriteFile(filepath.Join(pkiDir, tlsArtifactFilename), jsonBytes, 0644)
		o.Expect(err).NotTo(o.HaveOccurred())

		// TODO read from vendored openshift/api where approvers approve items
		previousPKIContent := &certgraphapi.PKIRegistryInfo{}

		newSecrets := map[certgraphapi.InClusterSecretLocation]certgraphapi.PKIRegistryInClusterCertKeyPair{}
		secretsWithoutComponents := map[certgraphapi.InClusterSecretLocation]certgraphapi.PKIRegistryInClusterCertKeyPair{}
		for _, currCertKeyPair := range tlsRegistryInfo.CertKeyPairs {
			currLocation := currCertKeyPair.SecretLocation
			// TODO add a field for "responsibleComponent" set based on annotation, still key based on namespace,name tuple
			// TODO possibly a look-aside info at the top level of CertKeyPairs

			_, err := locateCertKeyPair(currLocation, previousPKIContent.CertKeyPairs)
			if err == nil {
				continue
			}
			newSecrets[currLocation] = currCertKeyPair
		}

		newConfigMaps := map[certgraphapi.InClusterConfigMapLocation]certgraphapi.PKIRegistryInClusterCABundle{}
		configMapsWithoutComponents := map[certgraphapi.InClusterConfigMapLocation]certgraphapi.PKIRegistryInClusterCABundle{}
		for _, currCABundle := range tlsRegistryInfo.CertificateAuthorityBundles {
			currLocation := currCABundle.ConfigMapLocation
			// TODO add a field for "responsibleComponent" set based on annotation, still key based on namespace,name tuple
			// TODO possibly a look-aside info at the top level of CertKeyPairs

			_, err := locateCertificateAuthorityBundle(currLocation, previousPKIContent.CertificateAuthorityBundles)
			if err == nil {
				continue
			}
			newConfigMaps[currLocation] = currCABundle
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

func allCertsToPKIRegistry(in *certgraphapi.PKIList) *certgraphapi.PKIRegistryInfo {
	ret := &certgraphapi.PKIRegistryInfo{}

	for _, curr := range in.CertKeyPairs.Items {
		for _, location := range curr.Spec.SecretLocations {
			registryInfo := certgraphapi.PKIRegistryInClusterCertKeyPair{
				SecretLocation: certgraphapi.InClusterSecretLocation{
					Namespace: location.Namespace,
					Name:      location.Name,
				},
				// TODO retrieve this via annotations in the API
				CertKeyInfo: certgraphapi.PKIRegistryCertKeyPairInfo{
					OwningJiraComponent: "",
					HumanName:           "",
					Description:         "",
				},
			}
			ret.CertKeyPairs = append(ret.CertKeyPairs, registryInfo)
		}
	}
	for _, curr := range in.CertificateAuthorityBundles.Items {
		for _, location := range curr.Spec.ConfigMapLocations {
			registryInfo := certgraphapi.PKIRegistryInClusterCABundle{
				ConfigMapLocation: certgraphapi.InClusterConfigMapLocation{
					Namespace: location.Namespace,
					Name:      location.Name,
				},
				// TODO retrieve this via annotations in the API
				CABundleInfo: certgraphapi.PKIRegistryCertificateAuthorityInfo{
					OwningJiraComponent: "",
					HumanName:           "",
					Description:         "",
				},
			}
			ret.CertificateAuthorityBundles = append(ret.CertificateAuthorityBundles, registryInfo)
		}
	}

	sort.Sort(registrySecretByNamespaceName(ret.CertKeyPairs))
	sort.Sort(registryConfigMapByNamespaceName(ret.CertificateAuthorityBundles))

	return ret
}

type registrySecretByNamespaceName []certgraphapi.PKIRegistryInClusterCertKeyPair

func (n registrySecretByNamespaceName) Len() int      { return len(n) }
func (n registrySecretByNamespaceName) Swap(i, j int) { n[i], n[j] = n[j], n[i] }
func (n registrySecretByNamespaceName) Less(i, j int) bool {
	if n[i].SecretLocation.Namespace != n[j].SecretLocation.Namespace {
		return n[i].SecretLocation.Namespace < n[j].SecretLocation.Namespace
	}
	return n[i].SecretLocation.Name < n[j].SecretLocation.Name
}

type registryConfigMapByNamespaceName []certgraphapi.PKIRegistryInClusterCABundle

func (n registryConfigMapByNamespaceName) Len() int      { return len(n) }
func (n registryConfigMapByNamespaceName) Swap(i, j int) { n[i], n[j] = n[j], n[i] }
func (n registryConfigMapByNamespaceName) Less(i, j int) bool {
	if n[i].ConfigMapLocation.Namespace != n[j].ConfigMapLocation.Namespace {
		return n[i].ConfigMapLocation.Namespace < n[j].ConfigMapLocation.Namespace
	}
	return n[i].ConfigMapLocation.Name < n[j].ConfigMapLocation.Name
}

// TODO move to library

type secretLocationByNamespaceName []certgraphapi.InClusterSecretLocation

func (n secretLocationByNamespaceName) Len() int      { return len(n) }
func (n secretLocationByNamespaceName) Swap(i, j int) { n[i], n[j] = n[j], n[i] }
func (n secretLocationByNamespaceName) Less(i, j int) bool {
	if n[i].Namespace != n[j].Namespace {
		return n[i].Namespace < n[j].Namespace
	}
	return n[i].Name < n[j].Name
}

type configMapLocationByNamespaceName []certgraphapi.InClusterConfigMapLocation

func (n configMapLocationByNamespaceName) Len() int      { return len(n) }
func (n configMapLocationByNamespaceName) Swap(i, j int) { n[i], n[j] = n[j], n[i] }
func (n configMapLocationByNamespaceName) Less(i, j int) bool {
	if n[i].Namespace != n[j].Namespace {
		return n[i].Namespace < n[j].Namespace
	}
	return n[i].Name < n[j].Name
}

func locateCertKeyPair(targetLocation certgraphapi.InClusterSecretLocation, certKeyPairs []certgraphapi.PKIRegistryInClusterCertKeyPair) (*certgraphapi.PKIRegistryInClusterCertKeyPair, error) {
	for i, curr := range certKeyPairs {
		if targetLocation == curr.SecretLocation {
			return &certKeyPairs[i], nil
		}
	}

	return nil, fmt.Errorf("not found: %#v", targetLocation)
}

func locateCertificateAuthorityBundle(targetLocation certgraphapi.InClusterConfigMapLocation, caBundles []certgraphapi.PKIRegistryInClusterCABundle) (*certgraphapi.PKIRegistryInClusterCABundle, error) {
	for i, curr := range caBundles {
		if targetLocation == curr.ConfigMapLocation {
			return &caBundles[i], nil
		}
	}

	return nil, fmt.Errorf("not found: %#v", targetLocation)
}
