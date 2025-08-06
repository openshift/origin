package tlsmetadatainterfaces

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"github.com/openshift/library-go/pkg/markdown"
	"github.com/openshift/origin/pkg/certs"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

const UnknownOwner = "Unknown Owner"

var (
	onDiskCertKeyPairs = certs.CertKeyPairInfoByOnDiskLocation{
		{Path: "/var/lib/ovn-ic/etc/ovnkube-node-certs/ovnkube-client-\u003ctimestamp\u003e.pem"}:                                         {OwningJiraComponent: "Networking / cluster-network-operator"},
		{Path: "/etc/cni/multus/certs/multus-client-\u003ctimestamp\u003e.pem"}:                                                           {OwningJiraComponent: "Networking / cluster-network-operator"},
		{Path: "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/secrets/bound-service-account-signing-key/service-account.key"}: {OwningJiraComponent: "kube-apiserver"},
		{Path: "/var/lib/kubelet/pki/kubelet-client-\u003ctimestamp\u003e.pem"}:                                                           {OwningJiraComponent: "Node / Kubelet"},
		{Path: "/var/lib/kubelet/pki/kubelet-server-\u003ctimestamp\u003e.pem"}:                                                           {OwningJiraComponent: "Node / Kubelet"},
		{Path: "/etc/kubernetes/kubeconfig"}:                                                                                              {OwningJiraComponent: "kube-apiserver"},
	}
	onDiskCABundles = certs.CABundleInfoByOnDiskLocation{
		{Path: "/etc/kubernetes/ca.crt"}: {OwningJiraComponent: "Machine Config Operator"},
		{Path: "/etc/kubernetes/static-pod-resources/kube-apiserver-certs/configmaps/trusted-ca-bundle/ca-bundle.crt"}: {OwningJiraComponent: "kube-apiserver"},
		{Path: "/etc/kubernetes/kubeconfig"}: {OwningJiraComponent: "kube-apiserver"},
		{Path: "/etc/kubernetes/static-pod-resources/kube-controller-manager-certs/configmaps/trusted-ca-bundle/ca-bundle.crt"}: {OwningJiraComponent: "kube-controller-manager"},
		{Path: "/etc/pki/tls/cert.pem"}:            {OwningJiraComponent: "RHCOS"},
		{Path: "/etc/pki/tls/certs/ca-bundle.crt"}: {OwningJiraComponent: "RHCOS"},
		{Path: "/etc/kubernetes/static-pod-resources/kube-controller-manager-certs/secrets/csr-signer/tls.crt"}: {OwningJiraComponent: "kube-controller-manager"},
		{Path: "/etc/kubernetes/cni/net.d/whereabouts.d/whereabouts.kubeconfig"}:                                {OwningJiraComponent: "cluster-network-operator"},
		{Path: "/etc/docker/certs.d/image-registry.openshift-image-registry.svc:5000/ca.crt"}:                   {OwningJiraComponent: "Image Registry"},
		{Path: "/etc/docker/certs.d/image-registry.openshift-image-registry.svc.cluster.local:5000/ca.crt"}:     {OwningJiraComponent: "Image Registry"},
	}
)

func AnnotationValue(whitelistedAnnotations []certgraphapi.AnnotationValue, key string) (string, bool) {
	for _, curr := range whitelistedAnnotations {
		if curr.Key == key {
			return curr.Value, true
		}
	}

	return "", false
}

func ProcessByLocation(rawData []*certgraphapi.PKIList) (*certs.PKIRegistryInfo, error) {
	errs := []error{}
	inClusterCertKeyPairs := certs.SecretInfoByNamespaceName{}
	inClusterCABundles := certs.ConfigMapInfoByNamespaceName{}
	inMemoryCerts := certs.PodInfoByNamespaceName{}

	for i := range rawData {
		currPKI := rawData[i]
		for i := range currPKI.InClusterResourceData.CertKeyPairs {
			currCert := currPKI.InClusterResourceData.CertKeyPairs[i]
			existing, ok := inClusterCertKeyPairs[currCert.SecretLocation]
			if ok && !reflect.DeepEqual(existing, currCert.CertKeyInfo) {
				errs = append(errs, fmt.Errorf("mismatch of certificate info for --namespace=%v secret/%v:\n%v\n", currCert.SecretLocation.Namespace, currCert.SecretLocation.Name, cmp.Diff(existing, currCert.CertKeyInfo)))
				continue
			}

			inClusterCertKeyPairs[currCert.SecretLocation] = currCert.CertKeyInfo
		}

		for i := range currPKI.InMemoryResourceData.CertKeyPairs {
			currCert := currPKI.InMemoryResourceData.CertKeyPairs[i]
			existing, ok := inMemoryCerts[currCert.PodLocation]
			if ok && !reflect.DeepEqual(existing, currCert.CertKeyInfo) {
				errs = append(errs, fmt.Errorf("mismatch of certificate info for --namespace=%v pod/%v:\n%v\n", currCert.PodLocation.Namespace, currCert.PodLocation.Name, cmp.Diff(existing, currCert.CertKeyInfo)))
				continue
			}

			inMemoryCerts[currCert.PodLocation] = currCert.CertKeyInfo
		}

		for i := range currPKI.InClusterResourceData.CertificateAuthorityBundles {
			currCert := currPKI.InClusterResourceData.CertificateAuthorityBundles[i]
			existing, ok := inClusterCABundles[currCert.ConfigMapLocation]
			if ok && !reflect.DeepEqual(existing, currCert.CABundleInfo) {
				errs = append(errs, fmt.Errorf("mismatch of certificate info for --namespace=%v configmap/%v:\n%v\n", currCert.ConfigMapLocation.Namespace, currCert.ConfigMapLocation.Name, cmp.Diff(existing, currCert.CABundleInfo)))
				continue
			}

			inClusterCABundles[currCert.ConfigMapLocation] = currCert.CABundleInfo
		}
	}
	if len(errs) > 0 {
		return nil, utilerrors.NewAggregate(errs)
	}

	return certs.CertsToRegistryInfo(inClusterCertKeyPairs, onDiskCertKeyPairs, inClusterCABundles, onDiskCABundles, inMemoryCerts), nil
}

// FindAllCertificateLocations returns a list of cert key pairs including ondisk files found
// in all raw TLS reports
func FindAllCertificateLocations(certKeyLocation certgraphapi.PKIRegistryCertKeyPair, rawDataList []*certgraphapi.PKIList) []certgraphapi.PKIRegistryCertKeyPair {
	onDiskLocations := sets.New[certgraphapi.OnDiskCertKeyPairLocation]()
	for _, rawData := range rawDataList {
		for _, certKeyPair := range rawData.CertKeyPairs.Items {
			// Find certKeyPair which has this location
			found := false
			for _, curr := range certKeyPair.Spec.SecretLocations {
				if certKeyLocation.InClusterLocation != nil && curr == certKeyLocation.InClusterLocation.SecretLocation {
					found = true
					break
				}
			}
			// If not found lookup in ondisk files
			if certKeyLocation.OnDiskLocation != nil && !found {
				for _, onDiskLocation := range certKeyPair.Spec.OnDiskLocations {
					if onDiskLocation.Cert == certKeyLocation.OnDiskLocation.OnDiskLocation || onDiskLocation.Key == certKeyLocation.OnDiskLocation.OnDiskLocation {
						found = true
						break
					}
				}
			}
			if found {
				// Lookup owner info in onDiskCertKeyPairs
				// This prevents adding certkey pair with matching content to all components
				for _, location := range certKeyPair.Spec.OnDiskLocations {
					expectedInfo, ok := onDiskCertKeyPairs[location.Cert]
					// no hint found
					if !ok {
						onDiskLocations = onDiskLocations.Insert(location)
						continue
					}
					if certKeyLocation.InClusterLocation != nil && expectedInfo.OwningJiraComponent == certKeyLocation.InClusterLocation.CertKeyInfo.OwningJiraComponent {
						onDiskLocations = onDiskLocations.Insert(location)
						continue
					}
					if certKeyLocation.OnDiskLocation != nil && expectedInfo.OwningJiraComponent == certKeyLocation.OnDiskLocation.CertKeyInfo.OwningJiraComponent {
						onDiskLocations = onDiskLocations.Insert(location)
						continue
					}
				}
			}
		}
	}

	// Assemble the result - start with the initial location
	result := []certgraphapi.PKIRegistryCertKeyPair{certKeyLocation}
	for _, onDisk := range onDiskLocations.UnsortedList() {
		certOnDiskLocation := onDisk.Cert
		if len(certOnDiskLocation.Path) == 0 {
			certOnDiskLocation = onDisk.Key
		}
		result = append(result, certgraphapi.PKIRegistryCertKeyPair{
			OnDiskLocation: &certgraphapi.PKIRegistryOnDiskCertKeyPair{
				OnDiskLocation: certOnDiskLocation,
			},
		})
	}
	return result
}

// FindAllCABundleLocations returns a list of CA bundles including ondisk CA bundles found
// in all TLS reports
func FindAllCABundleLocations(caBundleLocation certgraphapi.PKIRegistryCABundle, rawDataList []*certgraphapi.PKIList) []certgraphapi.PKIRegistryCABundle {
	caBundleInfo := GetCABundleInfo(caBundleLocation)

	onDiskLocations := sets.New[certgraphapi.OnDiskLocation]()

	// Lookup cabundle by location
	for _, rawData := range rawDataList {
		for _, caBundle := range rawData.CertificateAuthorityBundles.Items {
			found := false
			for _, curr := range caBundle.Spec.ConfigMapLocations {
				if caBundleLocation.InClusterLocation != nil && curr == caBundleLocation.InClusterLocation.ConfigMapLocation {
					found = true
					break
				}
			}
			// If not found lookup in ondisk files
			if caBundleLocation.OnDiskLocation != nil && !found {
				for _, onDiskLocation := range caBundle.Spec.OnDiskLocations {
					if onDiskLocation == caBundleLocation.OnDiskLocation.OnDiskLocation {
						found = true
						break
					}
				}
			}
			if found {
				// Lookup owner info in onDiskCABundles
				// This prevents adding CA bundle with matching content to all components
				for _, location := range caBundle.Spec.OnDiskLocations {
					expectedInfo, ok := onDiskCABundles[location]
					// no hint found
					if !ok {
						onDiskLocations = onDiskLocations.Insert(location)
						continue
					}
					if caBundleLocation.InClusterLocation != nil && expectedInfo.OwningJiraComponent == caBundleLocation.InClusterLocation.CABundleInfo.OwningJiraComponent {
						onDiskLocations = onDiskLocations.Insert(location)
						continue
					}
					if caBundleLocation.OnDiskLocation != nil && expectedInfo.OwningJiraComponent == caBundleLocation.OnDiskLocation.CABundleInfo.OwningJiraComponent {
						onDiskLocations = onDiskLocations.Insert(location)
						continue
					}
				}
			}
		}
	}
	// Assemble the result - start with the initial location
	result := []certgraphapi.PKIRegistryCABundle{caBundleLocation}
	for _, onDisk := range onDiskLocations.UnsortedList() {
		result = append(result, certgraphapi.PKIRegistryCABundle{
			OnDiskLocation: &certgraphapi.PKIRegistryOnDiskCABundle{
				OnDiskLocation: onDisk,
				CABundleInfo:   *caBundleInfo,
			},
		})
	}
	return result
}

func PrintCertKeyPairDetails(curr certgraphapi.PKIRegistryCertKeyPair, md *markdown.Markdown, rawData []*certgraphapi.PKIList) {
	certKeyInfo := GetCertKeyPairInfo(curr)
	currLocationString := certs.BuildCertKeyPath(curr)
	md.NewOrderedListItem()
	md.Textf("%v\n", currLocationString)
	md.Textf("**Description:** %v", certKeyInfo.Description)

	// Find all locations of this cert key pair and deduplicate paths
	certsIncludingOtherLocations := FindAllCertificateLocations(curr, rawData)
	sort.Sort(certs.CertKeyPairByLocation(certsIncludingOtherLocations))
	foundCerts := slices.CompactFunc(certsIncludingOtherLocations, func(prev, next certgraphapi.PKIRegistryCertKeyPair) bool {
		return strings.Compare(certs.BuildCertKeyPath(prev), certs.BuildCertKeyPath(next)) == 0
	})
	// foundCerts always includes current one, so its needs to be filtered
	if len(foundCerts) > 1 {
		md.Text("\n")
		md.Text("Other locations:\n")
		for _, otherLocation := range foundCerts {
			otherLocationString := certs.BuildCertKeyPath(otherLocation)
			if otherLocationString == currLocationString {
				continue
			}
			md.Textf("* %v", otherLocationString)
		}
	}
	md.Text("\n")
}

func PrintCABundleDetails(curr certgraphapi.PKIRegistryCABundle, md *markdown.Markdown, rawData []*certgraphapi.PKIList) {
	caBundleInfo := GetCABundleInfo(curr)
	currLocationString := certs.BuildCABundlePath(curr)
	md.NewOrderedListItem()
	md.Textf("%v\n", currLocationString)
	md.Textf("**Description:** %v", caBundleInfo.Description)
	// Find all locations of this cabundle and deduplicate paths
	caBundlesIncludingOtherLocations := FindAllCABundleLocations(curr, rawData)
	sort.Sort(certs.CertificateAuthorityBundleByLocation(caBundlesIncludingOtherLocations))
	foundCABundles := slices.CompactFunc(caBundlesIncludingOtherLocations, func(prev, next certgraphapi.PKIRegistryCABundle) bool {
		return strings.Compare(certs.BuildCABundlePath(prev), certs.BuildCABundlePath(next)) == 0
	})
	// foundCABundles always includes current one, so its needs to be filtered
	if len(foundCABundles) > 1 {
		md.Text("\n")
		md.Text("Other locations:\n")

		for _, otherLocation := range foundCABundles {
			otherLocationString := certs.BuildCABundlePath(otherLocation)
			if otherLocationString == currLocationString {
				continue
			}
			md.Textf("* %v", otherLocationString)
		}
	}
	md.Text("\n")
}

// MarshalViolationsToJSON removes certificateAuthorityBundleInfo / certKeyInfo from violations json
// so that label update didn't trigger violations json to be updated as well
func MarshalViolationsToJSON(violations *certs.PKIRegistryInfo) ([]byte, error) {
	strippedViolations := &certs.PKIRegistryInfo{}
	for _, existing := range violations.CertificateAuthorityBundles {
		new := certgraphapi.PKIRegistryCABundle{}
		if incluster := existing.InClusterLocation; incluster != nil {
			new.InClusterLocation = &certgraphapi.PKIRegistryInClusterCABundle{
				ConfigMapLocation: incluster.ConfigMapLocation,
			}
		}
		if ondisk := existing.OnDiskLocation; ondisk != nil {
			new.OnDiskLocation = &certgraphapi.PKIRegistryOnDiskCABundle{
				OnDiskLocation: ondisk.OnDiskLocation,
			}
		}
		strippedViolations.CertificateAuthorityBundles = append(strippedViolations.CertificateAuthorityBundles, new)
	}

	for _, existing := range violations.CertKeyPairs {
		new := certgraphapi.PKIRegistryCertKeyPair{}
		if incluster := existing.InClusterLocation; incluster != nil {
			new.InClusterLocation = &certgraphapi.PKIRegistryInClusterCertKeyPair{
				SecretLocation: incluster.SecretLocation,
			}
		}
		if ondisk := existing.OnDiskLocation; ondisk != nil {
			new.OnDiskLocation = &certgraphapi.PKIRegistryOnDiskCertKeyPair{
				OnDiskLocation: ondisk.OnDiskLocation,
			}
		}
		strippedViolations.CertKeyPairs = append(strippedViolations.CertKeyPairs, new)
	}

	return json.MarshalIndent(strippedViolations, "", "    ")
}
