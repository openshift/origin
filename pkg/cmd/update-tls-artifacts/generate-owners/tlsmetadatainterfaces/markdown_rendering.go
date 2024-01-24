package tlsmetadatainterfaces

import "github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"

type InClusterLocationOrOnDiskPath struct {
	LocationType LocationType

	InClusterLocation *InClusterLocation

	OnDiskLocation *OnDiskLocation
}

type InClusterLocation struct {
	ResourceType string
	Namespace    string
	Name         string

	ResourceMetata []certgraphapi.AnnotationValue
}

type OnDiskLocation struct {
	FileMetadata certgraphapi.OnDiskLocationWithMetadata
}

type LocationType string

var (
	InCluster LocationType = "InCluster"
	OnDisk    LocationType = "OnDisk"
)

type AnnotationViolationCalculator interface {
	// InspectRequirementValue returns false if the instance violates the requirement
	MeetsRequirement(annotations []certgraphapi.AnnotationValue) bool
}

type annotationInspector struct {
	annotationName string
}

func InspectAnnotationHasValue(annotationName string) AnnotationViolationCalculator {
	return &annotationInspector{annotationName: annotationName}
}

func (a annotationInspector) MeetsRequirement(annotations []certgraphapi.AnnotationValue) bool {
	val, _ := AnnotationValue(annotations, a.annotationName)
	if len(val) == 0 {
		return false
	}
	return true
}

type AnnotationComplianceIntermediate struct {
	CompliantCertsByOwner     map[string][]InClusterLocationOrOnDiskPath
	ViolatingCertsByOwner     map[string][]InClusterLocationOrOnDiskPath
	CompliantCABundlesByOwner map[string][]InClusterLocationOrOnDiskPath
	ViolatingCABundlesByOwner map[string][]InClusterLocationOrOnDiskPath
}

// BuildAnnotationComplianceIntermediate returns an intermediate structure useful for generating documentation.
// For non-annotation based requirements (permissions, key width), the categories are likely useful, but
// this function won't produce them easily.
func BuildAnnotationComplianceIntermediate(pkiInfo *certgraphapi.PKIRegistryInfo, inspector AnnotationViolationCalculator) AnnotationComplianceIntermediate {
	ret := AnnotationComplianceIntermediate{
		CompliantCertsByOwner:     map[string][]InClusterLocationOrOnDiskPath{},
		ViolatingCertsByOwner:     map[string][]InClusterLocationOrOnDiskPath{},
		CompliantCABundlesByOwner: map[string][]InClusterLocationOrOnDiskPath{},
		ViolatingCABundlesByOwner: map[string][]InClusterLocationOrOnDiskPath{},
	}

	for i := range pkiInfo.CertKeyPairs {
		curr := pkiInfo.CertKeyPairs[i]
		locationDetails := InClusterLocationOrOnDiskPath{
			LocationType: InCluster,
			InClusterLocation: &InClusterLocation{
				ResourceType:   "secret",
				Namespace:      curr.SecretLocation.Namespace,
				Name:           curr.SecretLocation.Name,
				ResourceMetata: curr.CertKeyInfo.SelectedCertMetadataAnnotations,
			},
		}

		owner := OwnerFor(locationDetails.InClusterLocation.ResourceMetata)
		meetsRequirement := inspector.MeetsRequirement(curr.CertKeyInfo.SelectedCertMetadataAnnotations)
		if !meetsRequirement {
			ret.ViolatingCertsByOwner[owner] = append(ret.ViolatingCertsByOwner[owner], locationDetails)
			continue
		}

		ret.CompliantCertsByOwner[owner] = append(ret.CompliantCertsByOwner[owner], locationDetails)
	}
	for i := range pkiInfo.CertificateAuthorityBundles {
		curr := pkiInfo.CertificateAuthorityBundles[i]
		locationDetails := InClusterLocationOrOnDiskPath{
			LocationType: InCluster,
			InClusterLocation: &InClusterLocation{
				ResourceType:   "configmap",
				Namespace:      curr.ConfigMapLocation.Namespace,
				Name:           curr.ConfigMapLocation.Name,
				ResourceMetata: curr.CABundleInfo.SelectedCertMetadataAnnotations,
			},
		}
		owner := OwnerFor(locationDetails.InClusterLocation.ResourceMetata)
		meetsRequirement := inspector.MeetsRequirement(curr.CABundleInfo.SelectedCertMetadataAnnotations)
		if !meetsRequirement {
			ret.ViolatingCABundlesByOwner[owner] = append(ret.ViolatingCABundlesByOwner[owner], locationDetails)
			continue
		}
		ret.CompliantCABundlesByOwner[owner] = append(ret.CompliantCABundlesByOwner[owner], locationDetails)
	}

	return ret
}

func MarkdownFor(md *Markdown, location InClusterLocationOrOnDiskPath) {
	switch location.LocationType {
	case InCluster:
		md.Textf("ns/%v %v/%v\n", location.InClusterLocation.Namespace, location.InClusterLocation.ResourceType, location.InClusterLocation.Name)
		md.Textf("**Description:** %v", DescriptionFor(location.InClusterLocation.ResourceMetata))
		md.Text("\n")
	case OnDisk:
		md.Textf("%v\n", location.OnDiskLocation.FileMetadata.Path)
		// TODO where to store descriptions

		md.Text("| File | Permissions | User | Group | SE Linux |\n")
		md.Text("| ----------- | ----------- | ----------- | ----------- | ----------- |\n")
		md.Textf("| %v | %v | %v | %v | %v |\n",
			location.OnDiskLocation.FileMetadata.Path,
			location.OnDiskLocation.FileMetadata.Permissions,
			location.OnDiskLocation.FileMetadata.User,
			location.OnDiskLocation.FileMetadata.Group,
			location.OnDiskLocation.FileMetadata.SELinuxOptions)

		md.Text("\n")
	}
}
