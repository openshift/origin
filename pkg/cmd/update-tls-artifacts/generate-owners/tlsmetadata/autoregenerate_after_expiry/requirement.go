package autoregenerate_after_expiry

import (
	"encoding/json"
	"fmt"

	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatainterfaces"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"k8s.io/apimachinery/pkg/util/sets"
)

// TODO move to openshift/api
const AutoRegenerateAfterOfflineExpiryAnnotation = "certificates.openshift.io/auto-regenerate-after-offline-expiry"

type AutoRegenerateAfterOfflineExpiryRequirement struct {
	name string
}

func NewAutoRegenerateAfterOfflineExpiryRequirement() tlsmetadatainterfaces.Requirement {
	return AutoRegenerateAfterOfflineExpiryRequirement{
		name: "autoregenerate-after-expiry",
	}
}

func (o AutoRegenerateAfterOfflineExpiryRequirement) InspectRequirement(rawData []*certgraphapi.PKIList) (tlsmetadatainterfaces.RequirementResult, error) {
	pkiInfo, err := tlsmetadatainterfaces.ProcessByLocation(rawData)
	if err != nil {
		return nil, fmt.Errorf("transforming raw data %v: %w", o.GetName(), err)
	}

	ownershipJSONBytes, err := json.MarshalIndent(pkiInfo, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failure marshalling %v.json: %w", o.GetName(), err)
	}
	markdown, err := generateAutoRegenerateAfterOfflineExpiryshipMarkdown(pkiInfo)
	if err != nil {
		return nil, fmt.Errorf("failure marshalling %v.md: %w", o.GetName(), err)
	}
	violations := generateViolationJSON(pkiInfo)
	violationJSONBytes, err := json.MarshalIndent(violations, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failure marshalling %v-violations.json: %w", o.GetName(), err)
	}

	return newRequirementResult(
		o.GetName(),
		ownershipJSONBytes,
		markdown,
		violationJSONBytes)
}

func annotationValue(whitelistedAnnotations []certgraphapi.AnnotationValue, key string) (string, bool) {
	for _, curr := range whitelistedAnnotations {
		if curr.Key == key {
			return curr.Value, true
		}
	}

	return "", false
}

func generateViolationJSON(pkiInfo *certgraphapi.PKIRegistryInfo) *certgraphapi.PKIRegistryInfo {
	ret := &certgraphapi.PKIRegistryInfo{}

	for i := range pkiInfo.CertKeyPairs {
		curr := pkiInfo.CertKeyPairs[i]
		regenerates, _ := annotationValue(curr.CertKeyInfo.WhitelistedAnnotations, AutoRegenerateAfterOfflineExpiryAnnotation)
		if len(regenerates) == 0 {
			ret.CertKeyPairs = append(ret.CertKeyPairs, curr)
		}
	}
	for i := range pkiInfo.CertificateAuthorityBundles {
		curr := pkiInfo.CertificateAuthorityBundles[i]
		regenerates, _ := annotationValue(curr.CABundleInfo.WhitelistedAnnotations, AutoRegenerateAfterOfflineExpiryAnnotation)
		if len(regenerates) == 0 {
			ret.CertificateAuthorityBundles = append(ret.CertificateAuthorityBundles, curr)
		}
	}

	return ret
}

func generateAutoRegenerateAfterOfflineExpiryshipMarkdown(pkiInfo *certgraphapi.PKIRegistryInfo) ([]byte, error) {
	compliantCertsByOwner := map[string][]certgraphapi.PKIRegistryInClusterCertKeyPair{}
	violatingCertsByOwner := map[string][]certgraphapi.PKIRegistryInClusterCertKeyPair{}
	compliantCABundlesByOwner := map[string][]certgraphapi.PKIRegistryInClusterCABundle{}
	violatingCABundlesByOwner := map[string][]certgraphapi.PKIRegistryInClusterCABundle{}

	for i := range pkiInfo.CertKeyPairs {
		curr := pkiInfo.CertKeyPairs[i]
		owner := curr.CertKeyInfo.OwningJiraComponent
		regenerates, _ := annotationValue(curr.CertKeyInfo.WhitelistedAnnotations, AutoRegenerateAfterOfflineExpiryAnnotation)
		if len(regenerates) == 0 {
			violatingCertsByOwner[owner] = append(violatingCertsByOwner[owner], curr)
			continue
		}

		compliantCertsByOwner[owner] = append(compliantCertsByOwner[owner], curr)
	}
	for i := range pkiInfo.CertificateAuthorityBundles {
		curr := pkiInfo.CertificateAuthorityBundles[i]
		owner := curr.CABundleInfo.OwningJiraComponent
		regenerates, _ := annotationValue(curr.CABundleInfo.WhitelistedAnnotations, AutoRegenerateAfterOfflineExpiryAnnotation)
		if len(regenerates) == 0 {
			violatingCABundlesByOwner[owner] = append(violatingCABundlesByOwner[owner], curr)
			continue
		}
		compliantCABundlesByOwner[owner] = append(compliantCABundlesByOwner[owner], curr)
	}

	md := tlsmetadatainterfaces.NewMarkdown("Auto Regenerate After Offline Expiry")
	md.Text("Acknowledging that a cert/key pair or CA bundle can auto-regenerate after it expires offline means")
	md.Text("that if the cluster is shut down until the certificate expires, when the machines are restarted")
	md.Text("the cluster will automatically create new cert/key pairs or update CA bundles as required without human")
	md.Text("intervention.")
	md.Textf("To assert that a particular cert/key pair or CA bundle can do this, add the %q annotation to the secret or configmap and ",
		AutoRegenerateAfterOfflineExpiryAnnotation)
	md.Text("setting the value of the annotation a github link to the PR adding the annotation.")
	md.Text("This assertion also means that you have")
	md.OrderedListStart()
	md.NewOrderedListItem()
	md.Text("Manually tested that this works or seen someone else manually test that this works.  AND")
	md.NewOrderedListItem()
	md.Text("Written an automated e2e job that your team has an alert for and is a blocking GA criteria, and/or")
	md.Text("QE has required test every release that ensures the functionality works every release.")
	md.OrderedListEnd()
	md.Text("Links should be provided in the PR adding the annotation.")

	if len(violatingCertsByOwner) > 0 || len(violatingCABundlesByOwner) > 0 {
		numViolators := 0
		for _, v := range violatingCertsByOwner {
			numViolators += len(v)
		}
		for _, v := range violatingCABundlesByOwner {
			numViolators += len(v)
		}
		md.Title(2, fmt.Sprintf("Items That Cannot Auto Regenerate After Offline Expiry (%d)", numViolators))
		violatingOwners := sets.StringKeySet(violatingCertsByOwner)
		violatingOwners.Insert(sets.StringKeySet(violatingCABundlesByOwner).UnsortedList()...)
		for _, owner := range violatingOwners.List() {
			md.Title(3, fmt.Sprintf("%s (%d)", owner, len(violatingCertsByOwner[owner])+len(violatingCABundlesByOwner[owner])))
			certs := violatingCertsByOwner[owner]
			if len(certs) > 0 {
				md.Title(4, fmt.Sprintf("Certificates (%d)", len(certs)))
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

			caBundles := violatingCABundlesByOwner[owner]
			if len(caBundles) > 0 {
				md.Title(4, fmt.Sprintf("Certificate Authority Bundles (%d)", len(caBundles)))
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
	}

	numCompliant := 0
	for _, v := range compliantCertsByOwner {
		numCompliant += len(v)
	}
	for _, v := range compliantCABundlesByOwner {
		numCompliant += len(v)
	}
	md.Title(2, fmt.Sprintf("Items That Can Auto Regenerate After Offline Expiry (%d)", numCompliant))
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
				md.Textf("ns/%v secret/%v\n", curr.SecretLocation.Namespace, curr.SecretLocation.Name)
				md.Textf("**Description:** %v", curr.CertKeyInfo.Description)
				md.Text("\n")
			}
			md.OrderedListEnd()
			md.Text("\n")
		}

		caBundles := compliantCABundlesByOwner[owner]
		if len(caBundles) > 0 {
			md.Title(4, fmt.Sprintf("Certificate Authority Bundles (%d)", len(caBundles)))
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

func (o AutoRegenerateAfterOfflineExpiryRequirement) GetName() string {
	return o.name
}
