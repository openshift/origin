package ownership

import (
	"encoding/json"
	"fmt"

	"github.com/openshift/api/annotations"
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

	for _, curr := range pkiInfo.CertKeyPairs {
		certKeyPairInfo := certgraphapi.PKIRegistryCertKeyPairInfo{}
		switch {
		case curr.InClusterLocation != nil:
			certKeyPairInfo = curr.InClusterLocation.CertKeyInfo
		case curr.OnDiskLocation != nil:
			certKeyPairInfo = curr.OnDiskLocation.CertKeyInfo
		}
		owner := certKeyPairInfo.OwningJiraComponent
		if len(owner) == 0 || owner == tlsmetadatainterfaces.UnknownOwner {
			ret.CertKeyPairs = append(ret.CertKeyPairs, curr)
		}
	}
	for _, curr := range pkiInfo.CertificateAuthorityBundles {
		caBundleInfo := certgraphapi.PKIRegistryCertificateAuthorityInfo{}
		switch {
		case curr.InClusterLocation != nil:
			caBundleInfo = curr.InClusterLocation.CABundleInfo
		case curr.OnDiskLocation != nil:
			caBundleInfo = curr.OnDiskLocation.CABundleInfo
		}
		owner := caBundleInfo.OwningJiraComponent
		if len(owner) == 0 || owner == tlsmetadatainterfaces.UnknownOwner {
			ret.CertificateAuthorityBundles = append(ret.CertificateAuthorityBundles, curr)
		}
	}

	return ret
}

func generateOwnershipMarkdown(pkiInfo *certs.PKIRegistryInfo) ([]byte, error) {
	complianceIntermediate := tlsmetadatainterfaces.BuildAnnotationComplianceIntermediate(
		pkiInfo, tlsmetadatainterfaces.InspectAnnotationHasValue(annotations.OpenShiftComponent))
	compliantCertsByOwner := complianceIntermediate.CompliantCertsByOwner
	violatingCertsByOwner := complianceIntermediate.ViolatingCertsByOwner
	compliantCABundlesByOwner := complianceIntermediate.CompliantCABundlesByOwner
	violatingCABundlesByOwner := complianceIntermediate.ViolatingCABundlesByOwner

	md := tlsmetadatainterfaces.NewMarkdown("Certificate Ownership")
	certsWithoutOwners := violatingCertsByOwner[tlsmetadatainterfaces.UnknownOwner]
	caBundlesWithoutOwners := violatingCABundlesByOwner[tlsmetadatainterfaces.UnknownOwner]

	if len(certsWithoutOwners) > 0 || len(caBundlesWithoutOwners) > 0 {
		md.Title(2, fmt.Sprintf("Missing Owners (%d)", len(certsWithoutOwners)+len(caBundlesWithoutOwners)))
		if len(certsWithoutOwners) > 0 {
			md.Title(3, fmt.Sprintf("Certificates (%d)", len(certsWithoutOwners)))
			md.OrderedListStart()
			for _, curr := range certsWithoutOwners {
				md.NewOrderedListItem()
				tlsmetadatainterfaces.MarkdownFor(md, curr)
			}
			md.OrderedListEnd()
			md.Text("\n")
		}
		if len(caBundlesWithoutOwners) > 0 {
			md.Title(3, fmt.Sprintf("Certificate Authority Bundles (%d)", len(caBundlesWithoutOwners)))
			md.OrderedListStart()
			for _, curr := range caBundlesWithoutOwners {
				md.NewOrderedListItem()
				tlsmetadatainterfaces.MarkdownFor(md, curr)
			}
			md.OrderedListEnd()
			md.Text("\n")
		}
	}

	allOwners := sets.StringKeySet(compliantCertsByOwner)
	allOwners.Insert(sets.StringKeySet(compliantCABundlesByOwner).UnsortedList()...)
	for _, owner := range allOwners.List() {
		md.Title(2, fmt.Sprintf("%s (%d)", owner, len(compliantCertsByOwner[owner])+len(compliantCABundlesByOwner[owner])))
		certs := compliantCertsByOwner[owner]
		if len(certs) > 0 {
			md.Title(3, fmt.Sprintf("Certificates (%d)", len(certs)))
			md.OrderedListStart()
			for _, curr := range certs {
				md.NewOrderedListItem()
				tlsmetadatainterfaces.MarkdownFor(md, curr)
			}
			md.OrderedListEnd()
			md.Text("\n")
		}

		caBundles := compliantCABundlesByOwner[owner]
		if len(caBundles) > 0 {
			md.Title(3, fmt.Sprintf("Certificate Authority Bundles (%d)", len(caBundles)))
			md.OrderedListStart()
			for _, curr := range caBundles {
				md.NewOrderedListItem()
				tlsmetadatainterfaces.MarkdownFor(md, curr)
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
