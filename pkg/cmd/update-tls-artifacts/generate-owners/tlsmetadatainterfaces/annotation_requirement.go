package tlsmetadatainterfaces

import (
	"encoding/json"
	"fmt"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
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
	markdown, err := o.generateInspectionMarkdown(pkiInfo)
	if err != nil {
		return nil, fmt.Errorf("failure marshalling %v.md: %w", o.GetName(), err)
	}
	violations := generateViolationJSONForAnnotationRequirement(o.GetAnnotationName(), pkiInfo)
	violationJSONBytes, err := json.MarshalIndent(violations, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failure marshalling %v-violations.json: %w", o.GetName(), err)
	}

	return NewRequirementResult(
		o.GetName(),
		ownershipJSONBytes,
		markdown,
		violationJSONBytes)
}

func (o annotationRequirement) generateInspectionMarkdown(pkiInfo *certs.PKIRegistryInfo) ([]byte, error) {
	compliantCertsByOwner := map[string][]certgraphapi.PKIRegistryCertKeyPair{}
	violatingCertsByOwner := map[string][]certgraphapi.PKIRegistryCertKeyPair{}
	compliantCABundlesByOwner := map[string][]certgraphapi.PKIRegistryCABundle{}
	violatingCABundlesByOwner := map[string][]certgraphapi.PKIRegistryCABundle{}

	for i := range pkiInfo.CertKeyPairs {
		curr := pkiInfo.CertKeyPairs[i]
		if curr.InClusterLocation == nil {
			continue
		}
		// TODO[vrutkovs]: fetch ondisk metadata here
		owner := curr.InClusterLocation.CertKeyInfo.OwningJiraComponent
		regenerates, _ := AnnotationValue(curr.InClusterLocation.CertKeyInfo.SelectedCertMetadataAnnotations, o.GetAnnotationName())
		if len(regenerates) == 0 {
			violatingCertsByOwner[owner] = append(violatingCertsByOwner[owner], curr)
			continue
		}

		compliantCertsByOwner[owner] = append(compliantCertsByOwner[owner], curr)
	}
	for i := range pkiInfo.CertificateAuthorityBundles {
		curr := pkiInfo.CertificateAuthorityBundles[i]
		if curr.InClusterLocation == nil {
			continue
		}
		owner := curr.InClusterLocation.CABundleInfo.OwningJiraComponent
		regenerates, _ := AnnotationValue(curr.InClusterLocation.CABundleInfo.SelectedCertMetadataAnnotations, o.GetAnnotationName())
		if len(regenerates) == 0 {
			violatingCABundlesByOwner[owner] = append(violatingCABundlesByOwner[owner], curr)
			continue
		}
		compliantCABundlesByOwner[owner] = append(compliantCABundlesByOwner[owner], curr)
	}

	md := NewMarkdown(o.title)
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
			md.Title(3, fmt.Sprintf("%s (%d)", owner, len(violatingCertsByOwner[owner])+len(violatingCABundlesByOwner[owner])))
			certs := violatingCertsByOwner[owner]
			if len(certs) > 0 {
				md.Title(4, fmt.Sprintf("Certificates (%d)", len(certs)))
				md.OrderedListStart()
				for _, curr := range certs {
					if curr.InClusterLocation == nil {
						continue
					}
					md.NewOrderedListItem()
					md.Textf("ns/%v secret/%v\n", curr.InClusterLocation.SecretLocation.Namespace, curr.InClusterLocation.SecretLocation.Name)
					md.Textf("**Description:** %v", curr.InClusterLocation.CertKeyInfo.Description)
					md.Text("\n")

					//TODO[vrutkovs]: on disk case
				}
				md.OrderedListEnd()
				md.Text("\n")
			}

			caBundles := violatingCABundlesByOwner[owner]
			if len(caBundles) > 0 {
				md.Title(4, fmt.Sprintf("Certificate Authority Bundles (%d)", len(caBundles)))
				md.OrderedListStart()
				for _, curr := range caBundles {
					if curr.InClusterLocation == nil {
						continue
					}
					md.NewOrderedListItem()
					md.Textf("ns/%v configmap/%v\n", curr.InClusterLocation.ConfigMapLocation.Namespace, curr.InClusterLocation.ConfigMapLocation.Name)
					md.Textf("**Description:** %v", curr.InClusterLocation.CABundleInfo.Description)
					md.Text("\n")

					//TODO[vrutkovs]: on disk case
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
	allAutoRegenerateAfterOfflineExpirys := sets.StringKeySet(compliantCertsByOwner)
	allAutoRegenerateAfterOfflineExpirys.Insert(sets.StringKeySet(compliantCABundlesByOwner).UnsortedList()...)
	for _, owner := range allAutoRegenerateAfterOfflineExpirys.List() {
		md.Title(3, fmt.Sprintf("%s (%d)", owner, len(compliantCertsByOwner[owner])+len(compliantCABundlesByOwner[owner])))
		certs := compliantCertsByOwner[owner]
		if len(certs) > 0 {
			md.Title(4, fmt.Sprintf("Certificates (%d)", len(certs)))
			md.OrderedListStart()
			for _, curr := range certs {
				md.NewOrderedListItem()
				md.Textf("ns/%v secret/%v\n", curr.InClusterLocation.SecretLocation.Namespace, curr.InClusterLocation.SecretLocation.Name)
				md.Textf("**Description:** %v", curr.InClusterLocation.CertKeyInfo.Description)
				md.Text("\n")
			}
			//TODO[vrutkovs]: on disk case

			md.OrderedListEnd()
			md.Text("\n")
		}

		caBundles := compliantCABundlesByOwner[owner]
		if len(caBundles) > 0 {
			md.Title(4, fmt.Sprintf("Certificate Authority Bundles (%d)", len(caBundles)))
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
			//TODO[vrutkovs]: on disk case

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
		regenerates, _ := AnnotationValue(curr.InClusterLocation.CertKeyInfo.SelectedCertMetadataAnnotations, annotationName)
		if len(regenerates) == 0 {
			ret.CertKeyPairs = append(ret.CertKeyPairs, curr)
		}
		//TODO[vrutkovs]: on disk case
	}
	for i := range pkiInfo.CertificateAuthorityBundles {
		curr := pkiInfo.CertificateAuthorityBundles[i]
		regenerates, _ := AnnotationValue(curr.InClusterLocation.CABundleInfo.SelectedCertMetadataAnnotations, annotationName)
		if len(regenerates) == 0 {
			ret.CertificateAuthorityBundles = append(ret.CertificateAuthorityBundles, curr)
		}
		//TODO[vrutkovs]: on disk case
	}

	return ret
}
