package metadata

import (
	"fmt"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"k8s.io/apimachinery/pkg/util/sets"
)

func newOwnerViolation(name string, pkiInfo *certgraphapi.PKIRegistryInfo) (Violation, error) {
	registry := &certgraphapi.PKIRegistryInfo{}

	for i := range pkiInfo.CertKeyPairs {
		curr := pkiInfo.CertKeyPairs[i]
		owner := curr.CertKeyInfo.OwningJiraComponent
		if len(owner) == 0 || owner == unknownOwner {
			registry.CertKeyPairs = append(registry.CertKeyPairs, curr)
		}
	}
	for i := range pkiInfo.CertificateAuthorityBundles {
		curr := pkiInfo.CertificateAuthorityBundles[i]
		owner := curr.CABundleInfo.OwningJiraComponent
		if len(owner) == 0 || owner == unknownOwner {
			registry.CertificateAuthorityBundles = append(registry.CertificateAuthorityBundles, curr)
		}
	}

	v := Violation{
		Name:     name,
		Registry: registry,
	}

	markdown, err := generateMarkdownNoOwner(pkiInfo)
	if err != nil {
		return v, err
	}
	v.Markdown = markdown

	return v, nil
}

func generateMarkdownNoOwner(pkiInfo *certgraphapi.PKIRegistryInfo) ([]byte, error) {
	const unknownOwner = "Unknown"
	certsByOwner := map[string][]certgraphapi.PKIRegistryInClusterCertKeyPair{}
	certsWithoutOwners := []certgraphapi.PKIRegistryInClusterCertKeyPair{}
	caBundlesByOwner := map[string][]certgraphapi.PKIRegistryInClusterCABundle{}
	caBundlesWithoutOwners := []certgraphapi.PKIRegistryInClusterCABundle{}

	for i := range pkiInfo.CertKeyPairs {
		curr := pkiInfo.CertKeyPairs[i]
		owner := curr.CertKeyInfo.OwningJiraComponent
		if len(owner) == 0 || owner == unknownOwner {
			certsWithoutOwners = append(certsWithoutOwners, curr)
			continue
		}
		certsByOwner[owner] = append(certsByOwner[owner], curr)
	}
	for i := range pkiInfo.CertificateAuthorityBundles {
		curr := pkiInfo.CertificateAuthorityBundles[i]
		owner := curr.CABundleInfo.OwningJiraComponent
		if len(owner) == 0 || owner == unknownOwner {
			caBundlesWithoutOwners = append(caBundlesWithoutOwners, curr)
			continue
		}
		caBundlesByOwner[owner] = append(caBundlesByOwner[owner], curr)
	}

	md := NewMarkdown("Certificate Ownership")

	if len(certsWithoutOwners) > 0 || len(caBundlesWithoutOwners) > 0 {
		md.Title(2, fmt.Sprintf("Missing Owners (%d)", len(certsWithoutOwners)+len(caBundlesWithoutOwners)))
		if len(certsWithoutOwners) > 0 {
			md.Title(3, fmt.Sprintf("Certificates (%d)", len(certsWithoutOwners)))
			md.OrderedListStart()
			for _, curr := range certsWithoutOwners {
				md.NewOrderedListItem()
				md.Textf("ns/%v secret/%v\n", curr.SecretLocation.Namespace, curr.SecretLocation.Name)
				md.Textf("**Description:** %v", curr.CertKeyInfo.Description)
				md.Text("\n")
			}
			md.OrderedListEnd()
			md.Text("\n")
		}
		if len(caBundlesWithoutOwners) > 0 {
			md.Title(3, fmt.Sprintf("Certificate Authority Bundles (%d)", len(caBundlesWithoutOwners)))
			md.OrderedListStart()
			for _, curr := range caBundlesWithoutOwners {
				md.NewOrderedListItem()
				md.Textf("ns/%v configmap/%v\n", curr.ConfigMapLocation.Namespace, curr.ConfigMapLocation.Name)
				md.Textf("**Description:** %v", curr.CABundleInfo.Description)
				md.Text("\n")
			}
			md.OrderedListEnd()
			md.Text("\n")
		}
	}

	allOwners := sets.StringKeySet(certsByOwner)
	allOwners.Insert(sets.StringKeySet(caBundlesByOwner).UnsortedList()...)
	for _, owner := range allOwners.List() {
		md.Title(2, fmt.Sprintf("%s (%d)", owner, len(certsByOwner[owner])+len(caBundlesByOwner[owner])))
		certs := certsByOwner[owner]
		if len(certs) > 0 {
			md.Title(3, fmt.Sprintf("Certificates (%d)", len(certs)))
			md.OrderedListStart()
			for _, curr := range certs {
				md.NewOrderedListItem()
				md.Textf("ns/%v secret/%v\n", curr.SecretLocation.Namespace, curr.SecretLocation.Name)
				md.Textf("**Description:** %v", curr.CertKeyInfo.Description)
				md.Text("\n")
			}
			md.OrderedListEnd()
			md.Text("\n")
		}

		caBundles := caBundlesByOwner[owner]
		if len(caBundles) > 0 {
			md.Title(3, fmt.Sprintf("Certificate Authority Bundles (%d)", len(caBundles)))
			md.OrderedListStart()
			for _, curr := range caBundles {
				md.NewOrderedListItem()
				md.Textf("ns/%v configmap/%v\n", curr.ConfigMapLocation.Namespace, curr.ConfigMapLocation.Name)
				md.Textf("**Description:** %v", curr.CABundleInfo.Description)
				md.Text("\n")
			}
			md.OrderedListEnd()
			md.Text("\n")
		}
	}

	return md.Bytes(), nil
}

func diffCertKeyPairOwners(actual, expected certgraphapi.PKIRegistryCertKeyPairInfo) error {
	if actual.OwningJiraComponent != expected.OwningJiraComponent {
		return fmt.Errorf("expected JIRA component to be %s, but was %s", expected.OwningJiraComponent, actual.OwningJiraComponent)
	}
	return nil
}

func diffCABundleOwners(actual, expected certgraphapi.PKIRegistryCertificateAuthorityInfo) error {
	if actual.OwningJiraComponent != expected.OwningJiraComponent {
		return fmt.Errorf("expected JIRA component to be %s, but was %s", expected.OwningJiraComponent, actual.OwningJiraComponent)
	}
	return nil
}
