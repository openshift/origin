package ownership

import (
	"encoding/json"
	"fmt"

	"github.com/openshift/origin/pkg/certs"
	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatainterfaces"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"k8s.io/apimachinery/pkg/util/sets"
)

type OwnerRequirement struct {
	name string
}

func NewOwnerRequirement() tlsmetadatainterfaces.Requirement {
	return OwnerRequirement{
		name: "ownership",
	}
}

func (o OwnerRequirement) InspectRequirement(rawData []*certgraphapi.PKIList) (tlsmetadatainterfaces.RequirementResult, error) {
	pkiInfo, err := tlsmetadatainterfaces.ProcessByLocation(rawData)
	if err != nil {
		return nil, fmt.Errorf("transforming raw data %v: %w", o.GetName(), err)
	}

	ownershipJSONBytes, err := json.MarshalIndent(pkiInfo, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failure marshalling %v.json: %w", o.GetName(), err)
	}
	markdown, err := generateOwnershipMarkdown(pkiInfo)
	if err != nil {
		return nil, fmt.Errorf("failure marshalling %v.md: %w", o.GetName(), err)
	}
	violations := generateViolationJSON(pkiInfo)
	violationJSONBytes, err := json.MarshalIndent(violations, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failure marshalling %v-violations.json: %w", o.GetName(), err)
	}

	return tlsmetadatainterfaces.NewRequirementResult(
		o.GetName(),
		ownershipJSONBytes,
		markdown,
		violationJSONBytes)
}

func generateViolationJSON(pkiInfo *certs.PKIRegistryInfo) *certs.PKIRegistryInfo {
	ret := &certs.PKIRegistryInfo{}

	for i := range pkiInfo.CertKeyPairs {
		curr := pkiInfo.CertKeyPairs[i]
		if curr.InClusterLocation == nil {
			continue
		}
		owner := curr.InClusterLocation.CertKeyInfo.OwningJiraComponent
		if len(owner) == 0 || owner == tlsmetadatainterfaces.UnknownOwner {
			ret.CertKeyPairs = append(ret.CertKeyPairs, curr)
		}
	}
	for i := range pkiInfo.CertificateAuthorityBundles {
		curr := pkiInfo.CertificateAuthorityBundles[i]
		if curr.InClusterLocation == nil {
			continue
		}
		owner := curr.InClusterLocation.CABundleInfo.OwningJiraComponent
		if len(owner) == 0 || owner == tlsmetadatainterfaces.UnknownOwner {
			ret.CertificateAuthorityBundles = append(ret.CertificateAuthorityBundles, curr)
		}
	}

	return ret
}

func generateOwnershipMarkdown(pkiInfo *certs.PKIRegistryInfo) ([]byte, error) {
	certsByOwner := map[string][]certgraphapi.PKIRegistryCertKeyPair{}
	certsWithoutOwners := []certgraphapi.PKIRegistryCertKeyPair{}
	caBundlesByOwner := map[string][]certgraphapi.PKIRegistryCABundle{}
	caBundlesWithoutOwners := []certgraphapi.PKIRegistryCABundle{}

	for i := range pkiInfo.CertKeyPairs {
		curr := pkiInfo.CertKeyPairs[i]
		if curr.InClusterLocation == nil {
			continue
		}
		owner := curr.InClusterLocation.CertKeyInfo.OwningJiraComponent
		if len(owner) == 0 || owner == tlsmetadatainterfaces.UnknownOwner {
			certsWithoutOwners = append(certsWithoutOwners, curr)
			continue
		}
		certsByOwner[owner] = append(certsByOwner[owner], curr)
	}
	for i := range pkiInfo.CertificateAuthorityBundles {
		curr := pkiInfo.CertificateAuthorityBundles[i]
		if curr.InClusterLocation == nil {
			continue
		}
		owner := curr.InClusterLocation.CABundleInfo.OwningJiraComponent
		if len(owner) == 0 || owner == tlsmetadatainterfaces.UnknownOwner {
			caBundlesWithoutOwners = append(caBundlesWithoutOwners, curr)
			continue
		}
		caBundlesByOwner[owner] = append(caBundlesByOwner[owner], curr)
	}

	md := tlsmetadatainterfaces.NewMarkdown("Certificate Ownership")

	if len(certsWithoutOwners) > 0 || len(caBundlesWithoutOwners) > 0 {
		md.Title(2, fmt.Sprintf("Missing Owners (%d)", len(certsWithoutOwners)+len(caBundlesWithoutOwners)))
		if len(certsWithoutOwners) > 0 {
			md.Title(3, fmt.Sprintf("Certificates (%d)", len(certsWithoutOwners)))
			md.OrderedListStart()
			for _, curr := range certsWithoutOwners {
				if curr.InClusterLocation == nil {
					continue
				}
				md.NewOrderedListItem()
				md.Textf("ns/%v secret/%v\n", curr.InClusterLocation.SecretLocation.Namespace, curr.InClusterLocation.SecretLocation.Name)
				md.Textf("**Description:** %v", curr.InClusterLocation.CertKeyInfo.Description)
				md.Text("\n")
			}
			md.OrderedListEnd()
			md.Text("\n")
		}
		if len(caBundlesWithoutOwners) > 0 {
			md.Title(3, fmt.Sprintf("Certificate Authority Bundles (%d)", len(caBundlesWithoutOwners)))
			md.OrderedListStart()
			for _, curr := range caBundlesWithoutOwners {
				if curr.InClusterLocation == nil {
					continue
				}
				md.NewOrderedListItem()
				md.Textf("ns/%v configmap/%v\n", curr.InClusterLocation.ConfigMapLocation.Namespace, curr.InClusterLocation.ConfigMapLocation.Name)
				md.Textf("**Description:** %v", curr.InClusterLocation.CABundleInfo.Description)
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
				if curr.InClusterLocation == nil {
					continue
				}
				md.NewOrderedListItem()
				md.Textf("ns/%v secret/%v\n", curr.InClusterLocation.SecretLocation.Namespace, curr.InClusterLocation.SecretLocation.Name)
				md.Textf("**Description:** %v", curr.InClusterLocation.CertKeyInfo.Description)
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
				if curr.InClusterLocation == nil {
					continue
				}
				md.NewOrderedListItem()
				md.Textf("ns/%v configmap/%v\n", curr.InClusterLocation.ConfigMapLocation.Namespace, curr.InClusterLocation.ConfigMapLocation.Name)
				md.Textf("**Description:** %v", curr.InClusterLocation.CABundleInfo.Description)
				md.Text("\n")
			}
			md.OrderedListEnd()
			md.Text("\n")
		}
	}

	return md.Bytes(), nil
}

func (o OwnerRequirement) GetName() string {
	return o.name
}
