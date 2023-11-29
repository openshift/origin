package description

import (
	"fmt"

	"github.com/openshift/origin/pkg/cmd/update-tls-artifacts/generate-owners/tlsmetadatainterfaces"

	"github.com/google/go-cmp/cmp"
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

type DescriptionRequirement struct {
	name string
}

func NewDescriptionRequirement() tlsmetadatainterfaces.Requirement {
	return DescriptionRequirement{
		name: "description",
	}
}

func (d DescriptionRequirement) GetName() string {
	return d.name
}

func (d DescriptionRequirement) GetViolation(name string, pkiInfo *certgraphapi.PKIRegistryInfo) (tlsmetadatainterfaces.Violation, error) {
	registry := &certgraphapi.PKIRegistryInfo{}

	for i := range pkiInfo.CertKeyPairs {
		curr := pkiInfo.CertKeyPairs[i]
		description := curr.CertKeyInfo.Description
		if len(description) == 0 {
			registry.CertKeyPairs = append(registry.CertKeyPairs, curr)
		}
	}
	for i := range pkiInfo.CertificateAuthorityBundles {
		curr := pkiInfo.CertificateAuthorityBundles[i]
		description := curr.CABundleInfo.Description
		if len(description) == 0 {
			registry.CertificateAuthorityBundles = append(registry.CertificateAuthorityBundles, curr)
		}
	}

	v := tlsmetadatainterfaces.Violation{
		Name:     name,
		Registry: registry,
	}

	markdown, err := d.GenerateMarkdown(registry)
	if err != nil {
		return v, err
	}
	v.Markdown = markdown

	return v, nil
}

func (d DescriptionRequirement) GenerateMarkdown(pkiInfo *certgraphapi.PKIRegistryInfo) ([]byte, error) {
	certsWithoutDescription := map[string]certgraphapi.PKIRegistryInClusterCertKeyPair{}
	caBundlesWithoutDescription := map[string]certgraphapi.PKIRegistryInClusterCABundle{}

	for i := range pkiInfo.CertKeyPairs {
		curr := pkiInfo.CertKeyPairs[i]
		owner := curr.CertKeyInfo.OwningJiraComponent
		description := curr.CertKeyInfo.Description
		if len(description) == 0 && len(owner) != 0 {
			certsWithoutDescription[owner] = curr
			continue
		}
	}
	for i := range pkiInfo.CertificateAuthorityBundles {
		curr := pkiInfo.CertificateAuthorityBundles[i]
		owner := curr.CABundleInfo.OwningJiraComponent
		description := curr.CABundleInfo.Description
		if len(description) == 0 && len(owner) != 0 {
			caBundlesWithoutDescription[owner] = curr
			continue
		}
	}

	md := tlsmetadatainterfaces.NewMarkdown("Certificate Description")
	if len(certsWithoutDescription) > 0 || len(caBundlesWithoutDescription) > 0 {
		md.Title(2, fmt.Sprintf("Missing Description (%d)", len(certsWithoutDescription)+len(caBundlesWithoutDescription)))
		if len(certsWithoutDescription) > 0 {
			md.Title(3, fmt.Sprintf("Certificates (%d)", len(certsWithoutDescription)))
			md.OrderedListStart()
			for owner, curr := range certsWithoutDescription {
				md.NewOrderedListItem()
				md.Textf("ns/%v secret/%v\n", curr.SecretLocation.Namespace, curr.SecretLocation.Name)
				md.Textf("**JIRA component:** %v", owner)
				md.Text("\n")
			}
			md.OrderedListEnd()
			md.Text("\n")
		}
		if len(caBundlesWithoutDescription) > 0 {
			md.Title(3, fmt.Sprintf("Certificate Authority Bundles (%d)", len(caBundlesWithoutDescription)))
			md.OrderedListStart()
			for owner, curr := range caBundlesWithoutDescription {
				md.NewOrderedListItem()
				md.Textf("ns/%v configmap/%v\n", curr.ConfigMapLocation.Namespace, curr.ConfigMapLocation.Name)
				md.Textf("**JIRA component:** %v", owner)
				md.Text("\n")
			}
			md.OrderedListEnd()
			md.Text("\n")
		}
	}
	return md.Bytes(), nil
}

func (d DescriptionRequirement) DiffCertKeyPair(actual, expected certgraphapi.PKIRegistryCertKeyPairInfo) string {
	if diff := cmp.Diff(expected.Description, actual.Description); len(diff) > 0 {
		return diff
	}
	return ""
}

func (d DescriptionRequirement) DiffCABundle(actual, expected certgraphapi.PKIRegistryCertificateAuthorityInfo) string {
	if diff := cmp.Diff(expected.Description, actual.Description); len(diff) > 0 {
		return diff
	}
	return ""
}
