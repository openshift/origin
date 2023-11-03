package generate_owners

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-cmp/cmp"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"github.com/openshift/origin/pkg/certs"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type GenerateOwnersOptions struct {
	RawTLSInfoDir       string
	TLSOwnershipInfoDir string
	ViolationDir        string
	Verify              bool

	genericclioptions.IOStreams
}

func (o *GenerateOwnersOptions) Run() error {
	result, err := certs.GetPKIInfoFromRawData(o.RawTLSInfoDir)
	if err != nil {
		return err
	}
	ownershipJSONBytes, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		return err
	}
	markdown, err := GenerateOwnershipMarkdown(result)
	if err != nil {
		return err
	}
	violations := GenerateViolationJSON(result)
	violationJSONBytes, err := json.MarshalIndent(violations, "", "    ")
	if err != nil {
		return err
	}

	if o.Verify {
		existingOwnershipJSONBytes, err := os.ReadFile(o.jsonFilename())
		switch {
		case os.IsNotExist(err): // do nothing
		case err != nil:
			return err
		}
		if diff := cmp.Diff(existingOwnershipJSONBytes, ownershipJSONBytes); len(diff) > 0 {
			return fmt.Errorf(diff)
		}

		existingViolationsJSONBytes, err := os.ReadFile(o.violationsFilename())
		switch {
		case os.IsNotExist(err): // do nothing
		case err != nil:
			return err
		}
		if diff := cmp.Diff(existingViolationsJSONBytes, violationJSONBytes); len(diff) > 0 {
			return fmt.Errorf(diff)
		}

		existingMarkdown, err := os.ReadFile(o.markdownFilename())
		switch {
		case os.IsNotExist(err): // do nothing
		case err != nil:
			return err
		}
		if diff := cmp.Diff(existingMarkdown, markdown); len(diff) > 0 {
			return fmt.Errorf(diff)
		}

	} else {
		// write the json out
		if err := os.MkdirAll(o.TLSOwnershipInfoDir, 0755); err != nil {
			return err
		}
		if err := os.MkdirAll(o.ViolationDir, 0755); err != nil {
			return err
		}

		err = os.WriteFile(o.jsonFilename(), ownershipJSONBytes, 0644)
		if err != nil {
			return err
		}
		err = os.WriteFile(o.markdownFilename(), markdown, 0644)
		if err != nil {
			return err
		}
		err = os.WriteFile(o.violationsFilename(), violationJSONBytes, 0644)
		if err != nil {
			return err
		}
	}

	return nil
}

func GenerateViolationJSON(pkiInfo *certgraphapi.PKIRegistryInfo) *certgraphapi.PKIRegistryInfo {
	ret := &certgraphapi.PKIRegistryInfo{}

	const unknownOwner = "Unknown"

	for i := range pkiInfo.CertKeyPairs {
		curr := pkiInfo.CertKeyPairs[i]
		owner := curr.CertKeyInfo.OwningJiraComponent
		if len(owner) == 0 || owner == unknownOwner {
			ret.CertKeyPairs = append(ret.CertKeyPairs, curr)
		}
	}
	for i := range pkiInfo.CertificateAuthorityBundles {
		curr := pkiInfo.CertificateAuthorityBundles[i]
		owner := curr.CABundleInfo.OwningJiraComponent
		if len(owner) == 0 || owner == unknownOwner {
			ret.CertificateAuthorityBundles = append(ret.CertificateAuthorityBundles, curr)
		}
	}

	return ret
}

// TODO make this output include which configuration the certificates exist on.
func GenerateOwnershipMarkdown(pkiInfo *certgraphapi.PKIRegistryInfo) ([]byte, error) {
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

	md := &bytes.Buffer{}

	if len(certsWithoutOwners) > 0 || len(caBundlesWithoutOwners) > 0 {
		fmt.Fprintln(md, "## Missing Owners")
		if len(certsWithoutOwners) > 0 {
			fmt.Fprintln(md, "### Certificates")
			for i, curr := range certsWithoutOwners {
				fmt.Fprintf(md, "%d. ns/%v secret/%v\n\n", i+1, curr.SecretLocation.Namespace, curr.SecretLocation.Name)
				fmt.Fprintf(md, "     **Description:** %v\n", curr.CertKeyInfo.Description)
			}
			fmt.Fprintln(md, "")
		}
		if len(caBundlesWithoutOwners) > 0 {
			fmt.Fprintln(md, "### Certificate Authority Bundles")
			for i, curr := range caBundlesWithoutOwners {
				fmt.Fprintf(md, "%d. ns/%v configmap/%v\n\n", i+1, curr.ConfigMapLocation.Namespace, curr.ConfigMapLocation.Name)
				fmt.Fprintf(md, "     **Description:** %v\n", curr.CABundleInfo.Description)
			}
			fmt.Fprintln(md, "")
		}
	}

	allOwners := sets.StringKeySet(certsByOwner)
	allOwners.Insert(sets.StringKeySet(caBundlesByOwner).UnsortedList()...)
	for _, owner := range allOwners.List() {
		fmt.Fprintf(md, "## %v\n", owner)
		certs := certsByOwner[owner]
		if len(certs) > 0 {
			fmt.Fprintln(md, "### Certificates")
			for i, curr := range certs {
				fmt.Fprintf(md, "%d. ns/%v secret/%v\n\n", i+1, curr.SecretLocation.Namespace, curr.SecretLocation.Name)
				fmt.Fprintf(md, "     **Description:** %v\n", curr.CertKeyInfo.Description)
			}
			fmt.Fprintln(md, "")
		}

		caBundles := caBundlesByOwner[owner]
		if len(caBundles) > 0 {
			fmt.Fprintln(md, "### Certificate Authority Bundles")
			for i, curr := range caBundles {
				fmt.Fprintf(md, "%d. ns/%v configmap/%v\n\n", i+1, curr.ConfigMapLocation.Namespace, curr.ConfigMapLocation.Name)
				fmt.Fprintf(md, "     **Description:** %v\n", curr.CABundleInfo.Description)
			}
			fmt.Fprintln(md, "")
		}
	}

	return md.Bytes(), nil
}

func (o *GenerateOwnersOptions) violationsFilename() string {
	return filepath.Join(o.ViolationDir, "ownership-violations.json")
}

func (o *GenerateOwnersOptions) jsonFilename() string {
	return filepath.Join(o.TLSOwnershipInfoDir, "tls-ownership.json")
}

func (o *GenerateOwnersOptions) markdownFilename() string {
	return filepath.Join(o.TLSOwnershipInfoDir, "ownership.md")
}
