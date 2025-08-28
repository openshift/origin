package tlsmetadatainterfaces

import (
	"encoding/json"
	"fmt"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"github.com/openshift/library-go/pkg/markdown"
	"github.com/openshift/origin/pkg/certs"
	"k8s.io/apimachinery/pkg/util/sets"
)

type annotationRequirement struct {
	// requirementName is a unique name for metadata requirement
	requirementName string
	// annotationName is the annotation looked up in cert metadata
	annotationName string
	// title for the markdown
	title string
	// explanationMD is exactly the markdown to include that explains the purposes of the check
	explanationMD string
}

func NewAnnotationRequirement(requirementName, annotationName, title, explanationMD string) AnnotationRequirement {
	return annotationRequirement{
		requirementName: requirementName,
		annotationName:  annotationName,
		title:           title,
		explanationMD:   explanationMD,
	}
}

func (o annotationRequirement) GetName() string {
	return o.requirementName
}

func (o annotationRequirement) GetAnnotationName() string {
	return o.annotationName
}

func (o annotationRequirement) InspectRequirement(rawData []*certgraphapi.PKIList) (RequirementResult, error) {
	pkiInfo, err := ProcessByLocation(rawData)
	if err != nil {
		return nil, fmt.Errorf("transforming raw data %v: %w", o.GetName(), err)
	}

	ownershipJSONBytes, err := json.MarshalIndent(pkiInfo, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failure marshalling %v.json: %w", o.GetName(), err)
	}
	markdown, err := o.generateInspectionMarkdown(pkiInfo, rawData)
	if err != nil {
		return nil, fmt.Errorf("failure marshalling %v.md: %w", o.GetName(), err)
	}
	violations := generateViolationJSONForAnnotationRequirement(o.GetAnnotationName(), pkiInfo)
	violationJSONBytes, err := MarshalViolationsToJSON(violations)
	if err != nil {
		return nil, fmt.Errorf("failure marshalling %v-violations.json: %w", o.GetName(), err)
	}

	return NewRequirementResult(
		o.GetName(),
		ownershipJSONBytes,
		markdown,
		violationJSONBytes)
}

func (o annotationRequirement) generateInspectionMarkdown(pkiInfo *certs.PKIRegistryInfo, rawData []*certgraphapi.PKIList) ([]byte, error) {
	compliantCertsByOwner := map[string][]certgraphapi.PKIRegistryCertKeyPair{}
	violatingCertsByOwner := map[string][]certgraphapi.PKIRegistryCertKeyPair{}
	compliantCABundlesByOwner := map[string][]certgraphapi.PKIRegistryCABundle{}
	violatingCABundlesByOwner := map[string][]certgraphapi.PKIRegistryCABundle{}

	for i := range pkiInfo.CertKeyPairs {
		curr := pkiInfo.CertKeyPairs[i]
		certKeyInfo := GetCertKeyPairInfo(curr)
		if certKeyInfo == nil {
			continue
		}
		owner := certKeyInfo.OwningJiraComponent
		regenerates, _ := AnnotationValue(certKeyInfo.SelectedCertMetadataAnnotations, o.GetAnnotationName())
		if len(regenerates) == 0 {
			violatingCertsByOwner[owner] = append(violatingCertsByOwner[owner], curr)
			continue
		}

		compliantCertsByOwner[owner] = append(compliantCertsByOwner[owner], curr)
	}
	for i := range pkiInfo.CertificateAuthorityBundles {
		curr := pkiInfo.CertificateAuthorityBundles[i]
		caBundleInfo := GetCABundleInfo(curr)
		if caBundleInfo == nil {
			continue
		}
		owner := caBundleInfo.OwningJiraComponent
		regenerates, _ := AnnotationValue(caBundleInfo.SelectedCertMetadataAnnotations, o.GetAnnotationName())
		if len(regenerates) == 0 {
			violatingCABundlesByOwner[owner] = append(violatingCABundlesByOwner[owner], curr)
			continue
		}
		compliantCABundlesByOwner[owner] = append(compliantCABundlesByOwner[owner], curr)
	}

	md := markdown.NewMarkdown(o.title)
	md.Title(2, "How to meet the requirement")
	md.ExactText(o.explanationMD)

	if len(violatingCertsByOwner) > 0 || len(violatingCABundlesByOwner) > 0 {
		numViolators := 0
		for _, v := range violatingCertsByOwner {
			numViolators += len(v)
		}
		for _, v := range violatingCABundlesByOwner {
			numViolators += len(v)
		}
		md.Title(2, fmt.Sprintf("Items Do NOT Meet the Requirement (%d)", numViolators))
		violatingOwners := sets.StringKeySet(violatingCertsByOwner)
		violatingOwners.Insert(sets.StringKeySet(violatingCABundlesByOwner).UnsortedList()...)
		for _, owner := range violatingOwners.List() {
			// Show custom label if owner is unset
			ownerLabel := owner
			if len(owner) == 0 {
				ownerLabel = UnknownOwner
			}
			md.Title(3, fmt.Sprintf("%s (%d)", ownerLabel, len(violatingCertsByOwner[owner])+len(violatingCABundlesByOwner[owner])))
			violatingCerts := violatingCertsByOwner[owner]
			if len(violatingCerts) > 0 {
				md.Title(4, fmt.Sprintf("Certificates (%d)", len(violatingCerts)))
				md.OrderedListStart()
				for _, curr := range violatingCerts {
					PrintCertKeyPairDetails(curr, md, rawData)
				}
				md.OrderedListEnd()
				md.Text("\n")
			}

			caBundles := violatingCABundlesByOwner[owner]
			if len(caBundles) > 0 {
				md.Title(4, fmt.Sprintf("Certificate Authority Bundles (%d)", len(caBundles)))
				md.OrderedListStart()
				for _, curr := range caBundles {
					PrintCABundleDetails(curr, md, rawData)
				}
				md.OrderedListEnd()
				md.Text("\n")
			}
		}
	}

	numCompliant := 0
	for _, v := range compliantCertsByOwner {
		numCompliant += len(v)
	}
	for _, v := range compliantCABundlesByOwner {
		numCompliant += len(v)
	}
	md.Title(2, fmt.Sprintf("Items That DO Meet the Requirement (%d)", numCompliant))
	complaintSet := sets.StringKeySet(compliantCertsByOwner)
	complaintSet.Insert(sets.StringKeySet(compliantCABundlesByOwner).UnsortedList()...)
	for _, owner := range complaintSet.List() {
		md.Title(3, fmt.Sprintf("%s (%d)", owner, len(compliantCertsByOwner[owner])+len(compliantCABundlesByOwner[owner])))
		complaintCerts := compliantCertsByOwner[owner]
		if len(complaintCerts) > 0 {
			md.Title(4, fmt.Sprintf("Certificates (%d)", len(complaintCerts)))
			md.OrderedListStart()
			for _, curr := range complaintCerts {
				PrintCertKeyPairDetails(curr, md, rawData)
			}

			md.OrderedListEnd()
			md.Text("\n")
		}

		caBundles := compliantCABundlesByOwner[owner]
		if len(caBundles) > 0 {
			md.Title(4, fmt.Sprintf("Certificate Authority Bundles (%d)", len(caBundles)))
			md.OrderedListStart()
			for _, curr := range caBundles {
				PrintCABundleDetails(curr, md, rawData)
			}

			md.OrderedListEnd()
			md.Text("\n")
		}
	}

	return md.Bytes(), nil
}

func generateViolationJSONForAnnotationRequirement(annotationName string, pkiInfo *certs.PKIRegistryInfo) *certs.PKIRegistryInfo {
	ret := &certs.PKIRegistryInfo{}

	for i := range pkiInfo.CertKeyPairs {
		curr := pkiInfo.CertKeyPairs[i]
		certKeyInfo := GetCertKeyPairInfo(curr)
		if certKeyInfo == nil {
			continue
		}

		regenerates, _ := AnnotationValue(certKeyInfo.SelectedCertMetadataAnnotations, annotationName)
		if len(regenerates) == 0 {
			ret.CertKeyPairs = append(ret.CertKeyPairs, curr)
		}
	}
	for i := range pkiInfo.CertificateAuthorityBundles {
		curr := pkiInfo.CertificateAuthorityBundles[i]
		caBundleInfo := GetCABundleInfo(curr)
		if caBundleInfo == nil {
			continue
		}
		regenerates, _ := AnnotationValue(caBundleInfo.SelectedCertMetadataAnnotations, annotationName)
		if len(regenerates) == 0 {
			ret.CertificateAuthorityBundles = append(ret.CertificateAuthorityBundles, curr)
		}
	}

	return ret
}

func GetCertKeyPairInfo(certKeyPair certgraphapi.PKIRegistryCertKeyPair) *certgraphapi.PKIRegistryCertKeyPairInfo {
	var certKeyInfo *certgraphapi.PKIRegistryCertKeyPairInfo
	if certKeyPair.InClusterLocation != nil {
		return &certKeyPair.InClusterLocation.CertKeyInfo
	}
	if certKeyPair.OnDiskLocation != nil {
		return &certKeyPair.OnDiskLocation.CertKeyInfo
	}
	if certKeyPair.InMemoryPodLocation != nil {
		return &certKeyPair.InMemoryPodLocation.CertKeyInfo
	}
	return certKeyInfo
}

func GetCABundleInfo(caBundle certgraphapi.PKIRegistryCABundle) *certgraphapi.PKIRegistryCertificateAuthorityInfo {
	var caBundleInfo *certgraphapi.PKIRegistryCertificateAuthorityInfo
	if caBundle.InClusterLocation != nil {
		return &caBundle.InClusterLocation.CABundleInfo
	}
	if caBundle.OnDiskLocation != nil {
		return &caBundle.OnDiskLocation.CABundleInfo
	}
	return caBundleInfo
}
