package generate_owners

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
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
	result, err := o.GetPKIInfoFromRawData()
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

func (o *GenerateOwnersOptions) GetPKIInfoFromRawData() (*certgraphapi.PKIRegistryInfo, error) {
	certs := map[certgraphapi.InClusterSecretLocation]certgraphapi.PKIRegistryCertKeyPairInfo{}
	caBundles := map[certgraphapi.InClusterConfigMapLocation]certgraphapi.PKIRegistryCertificateAuthorityInfo{}

	err := filepath.WalkDir(o.RawTLSInfoDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		filename := filepath.Join(o.RawTLSInfoDir, d.Name())
		currBytes, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		currPKI := &certgraphapi.PKIList{}
		err = json.Unmarshal(currBytes, currPKI)
		if err != nil {
			return err
		}

		for i := range currPKI.InClusterResourceData.CertKeyPairs {
			currCert := currPKI.InClusterResourceData.CertKeyPairs[i]
			existing, ok := certs[currCert.SecretLocation]
			if ok && !reflect.DeepEqual(existing, currCert.CertKeyInfo) {
				return fmt.Errorf("mismatch of certificate info")
			}

			certs[currCert.SecretLocation] = currCert.CertKeyInfo
		}
		for i := range currPKI.InClusterResourceData.CertificateAuthorityBundles {
			currCert := currPKI.InClusterResourceData.CertificateAuthorityBundles[i]
			existing, ok := caBundles[currCert.ConfigMapLocation]
			if ok && !reflect.DeepEqual(existing, currCert.CABundleInfo) {
				return fmt.Errorf("mismatch of certificate info")
			}

			caBundles[currCert.ConfigMapLocation] = currCert.CABundleInfo
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	result := &certgraphapi.PKIRegistryInfo{}

	certKeys := sets.KeySet[certgraphapi.InClusterSecretLocation, certgraphapi.PKIRegistryCertKeyPairInfo](certs).UnsortedList()
	sort.Sort(SecretRefByNamespaceName(certKeys))
	for _, key := range certKeys {
		result.CertKeyPairs = append(result.CertKeyPairs, certgraphapi.PKIRegistryInClusterCertKeyPair{
			SecretLocation: key,
			CertKeyInfo:    certs[key],
		})
	}

	caKeys := sets.KeySet[certgraphapi.InClusterConfigMapLocation, certgraphapi.PKIRegistryCertificateAuthorityInfo](caBundles).UnsortedList()
	sort.Sort(ConfigMapRefByNamespaceName(caKeys))
	for _, key := range caKeys {
		result.CertificateAuthorityBundles = append(result.CertificateAuthorityBundles, certgraphapi.PKIRegistryInClusterCABundle{
			ConfigMapLocation: key,
			CABundleInfo:      caBundles[key],
		})
	}

	return result, nil
}

type SecretRefByNamespaceName []certgraphapi.InClusterSecretLocation

func (n SecretRefByNamespaceName) Len() int {
	return len(n)
}
func (n SecretRefByNamespaceName) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}
func (n SecretRefByNamespaceName) Less(i, j int) bool {
	diff := strings.Compare(n[i].Namespace, n[j].Namespace)
	switch {
	case diff < 0:
		return true
	case diff > 0:
		return false
	}

	return strings.Compare(n[i].Name, n[j].Name) < 0
}

type ConfigMapRefByNamespaceName []certgraphapi.InClusterConfigMapLocation

func (n ConfigMapRefByNamespaceName) Len() int {
	return len(n)
}
func (n ConfigMapRefByNamespaceName) Swap(i, j int) {
	n[i], n[j] = n[j], n[i]
}
func (n ConfigMapRefByNamespaceName) Less(i, j int) bool {
	diff := strings.Compare(n[i].Namespace, n[j].Namespace)
	switch {
	case diff < 0:
		return true
	case diff > 0:
		return false
	}

	return strings.Compare(n[i].Name, n[j].Name) < 0
}
