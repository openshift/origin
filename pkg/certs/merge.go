package certs

import (
	"context"
	"fmt"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphanalysis"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

/*
MergeRawPKILists takes multiple PKILists and tries to produce a sensible superset using the following logic

1. for each PKIList
	1. for each item
		1. if no location in the NEW overlaps with items RET, then add to the TO-MERGE list
		2. if at least two locations in the NEW overlap with different items in RET, then add to the FAIL list
 		3. else this should mean that at least one location in NEW matches a location in RET and all other locations
		   either match the same or have no match.  In this case, we add to the ADD list and we must map the
		   new serial/commonName/issuer/modulus to the ret serial/commonname/issuer/modulus so we're able to build the graph later
	1. for each FAIL item, add to the list of errors
	1. for each MERGE item, merge into RET and build the mapping for this PKIList
	1. for each NEW item, perform the mapping and then add to the RET
*/

// added a line here so gofmt doesn't lose the formatting
func MergeRawPKILists(ctx context.Context, inLists ...*certgraphapi.PKIList) (*certgraphapi.PKIList, []error) {
	errs := []error{}

	ret := &certgraphapi.PKIList{}
	for _, currPKIList := range inLists {
		var mergeErrors []error
		ret, mergeErrors = mergeOneRawPKILists(ctx, ret, currPKIList)
		errs = append(errs, mergeErrors...)
	}

	if len(errs) > 0 {
		return nil, errs
	}
	// this does deduping for locations until we expose some helpers
	ret = certgraphanalysis.MergePKILists(ctx, ret, &certgraphapi.PKIList{})
	return ret, nil
}

func mergeOneRawPKILists(ctx context.Context, existingPKIList, newPKIList *certgraphapi.PKIList) (*certgraphapi.PKIList, []error) {
	ret := &certgraphapi.PKIList{}
	ret.CertKeyPairs = existingPKIList.CertKeyPairs
	ret.CertificateAuthorityBundles = existingPKIList.CertificateAuthorityBundles
	errs := []error{}

	certKeyPairsToMerge, certKeyPairsToAdd, certIdentifierMappings, categorizationErrs := categorizeCertKeys(ret.CertKeyPairs.Items, newPKIList.CertKeyPairs.Items)
	errs = append(errs, categorizationErrs...)

	for _, toMerge := range certKeyPairsToMerge {
		foundLocations := LocateMatchingCertKeyPairs(ret.CertKeyPairs.Items, toMerge)
		// TODO in place mutation is fraught
		foundLocations[0].Spec.SecretLocations = certgraphanalysis.CombineSecretLocations(foundLocations[0], toMerge.Spec.SecretLocations).Spec.SecretLocations
		foundLocations[0].Spec.OnDiskLocations = certgraphanalysis.CombineCertOnDiskLocations(foundLocations[0], toMerge.Spec.OnDiskLocations).Spec.OnDiskLocations
	}
	for _, sourceToAdd := range certKeyPairsToAdd {
		toAdd := sourceToAdd.DeepCopy()
		newIdentifier, err := certIdentifierMappings.Map(toAdd.Spec.CertMetadata.CertIdentifier)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		toAdd.Spec.CertMetadata.CertIdentifier = *newIdentifier
		ret.CertKeyPairs.Items = append(ret.CertKeyPairs.Items, *toAdd)
	}

	caBundlesToMerge, caBundlesToAdd, categorizationErrs := categorizeCABundles(ret.CertificateAuthorityBundles.Items, newPKIList.CertificateAuthorityBundles.Items)
	errs = append(errs, categorizationErrs...)

	for _, toMerge := range caBundlesToMerge {
		foundLocations := LocateMatchingCertificateAuthorityBundles(ret.CertificateAuthorityBundles.Items, toMerge)
		// TODO in place mutation is fraught
		foundLocations[0].Spec.ConfigMapLocations = certgraphanalysis.CombineConfigMapLocations(foundLocations[0], toMerge.Spec.ConfigMapLocations).Spec.ConfigMapLocations
		foundLocations[0].Spec.OnDiskLocations = certgraphanalysis.CombineCABundleOnDiskLocations(foundLocations[0], toMerge.Spec.OnDiskLocations).Spec.OnDiskLocations

		// every certificate needs to exist in the merged copy and we need it to map everything that's possible and leave everything that isn't
		for i, curr := range toMerge.Spec.CertificateMetadata {
			mappedIdentifier, err := certIdentifierMappings.Map(curr.CertIdentifier)
			if err != nil {
				errs = append(errs, err)
				continue
			}

			found := false
			for _, currRetCert := range foundLocations[0].Spec.CertificateMetadata {
				if currRetCert.CertIdentifier.PubkeyModulus == mappedIdentifier.PubkeyModulus {
					found = true
					break
				}
			}
			if found {
				continue
			}
			foundLocations[0].Spec.CertificateMetadata = append(foundLocations[0].Spec.CertificateMetadata, toMerge.Spec.CertificateMetadata[i])
		}
	}
	for _, sourceToAdd := range caBundlesToAdd {
		toAdd := sourceToAdd.DeepCopy()
		// this needs to exist and we need it to map everything that's possible and leave everything that isn't
		for i, curr := range toAdd.Spec.CertificateMetadata {
			newIdentifier, err := certIdentifierMappings.Map(curr.CertIdentifier)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			toAdd.Spec.CertificateMetadata[i].CertIdentifier = *newIdentifier
		}
		ret.CertificateAuthorityBundles.Items = append(ret.CertificateAuthorityBundles.Items, *toAdd)
	}

	return ret, errs
}

func categorizeCertKeys(existingCertKeyPairs, newCertKeyPairs []certgraphapi.CertKeyPair) ([]certgraphapi.CertKeyPair, []certgraphapi.CertKeyPair, *CertIdentifierMappings, []error) {
	errs := []error{}
	certKeyPairsToMerge := []certgraphapi.CertKeyPair{}
	certIdentifierMappings := &CertIdentifierMappings{}
	certKeyPairsToAdd := []certgraphapi.CertKeyPair{}

	for _, currItem := range newCertKeyPairs {
		foundLocations := LocateMatchingCertKeyPairs(existingCertKeyPairs, currItem)

		switch len(foundLocations) {
		case 0:
			certKeyPairsToAdd = append(certKeyPairsToAdd, currItem)
		case 1:
			certKeyPairsToMerge = append(certKeyPairsToMerge, currItem)
			mapping := CertIdentifierMapping{
				FromValue: currItem.Spec.CertMetadata.CertIdentifier,
				ToValue:   foundLocations[0].Spec.CertMetadata.CertIdentifier,
			}
			certIdentifierMappings.Mappings = append(certIdentifierMappings.Mappings, mapping)
		default:
			errs = append(errs,
				fmt.Errorf("%#v had %d conflicting matches: %v", currItem.Spec.CertMetadata.CertIdentifier, len(foundLocations), foundLocations))
		}
	}

	return certKeyPairsToMerge, certKeyPairsToAdd, certIdentifierMappings, errs
}

func categorizeCABundles(existingCABundles, newCABundles []certgraphapi.CertificateAuthorityBundle) ([]certgraphapi.CertificateAuthorityBundle, []certgraphapi.CertificateAuthorityBundle, []error) {
	errs := []error{}
	caBundlesToMerge := []certgraphapi.CertificateAuthorityBundle{}
	caBundlesToAdd := []certgraphapi.CertificateAuthorityBundle{}

	for _, currItem := range newCABundles {
		foundLocations := LocateMatchingCertificateAuthorityBundles(existingCABundles, currItem)

		switch len(foundLocations) {
		case 0:
			caBundlesToAdd = append(caBundlesToAdd, currItem)
		case 1:
			caBundlesToMerge = append(caBundlesToMerge, currItem)
		default:
			errs = append(errs,
				fmt.Errorf("%#v had %d conflicting matches: %v", currItem.Spec.CertificateMetadata, len(foundLocations), foundLocations))
		}
	}

	return caBundlesToMerge, caBundlesToAdd, errs
}

type CertIdentifierMappings struct {
	// TODO, could create a map key if the iteration became expensive
	Mappings []CertIdentifierMapping
}

func (c *CertIdentifierMappings) Map(in certgraphapi.CertIdentifier) (*certgraphapi.CertIdentifier, error) {
	var ret *certgraphapi.CertIdentifier

	found := false
	for i := range c.Mappings {
		curr := c.Mappings[i]
		switch {
		case curr.AppliesTo(in) && found:
			return nil, fmt.Errorf("conflicting mappings for %v", in)
		case curr.AppliesTo(in):
			ret = &curr.ToValue
			found = true
		}
	}

	if !found {
		// if we're not found, we have a unique cert to add.  Just return what we came in with.
		t := in
		return &t, nil
	}

	return ret, nil
}

type CertIdentifierMapping struct {
	FromValue certgraphapi.CertIdentifier
	ToValue   certgraphapi.CertIdentifier
}

func (c *CertIdentifierMapping) AppliesTo(in certgraphapi.CertIdentifier) bool {
	return IdentifierMatches(in, c.FromValue)
}

// TODO move to library-go
// IdentifierMatches is not Equals because issuer is not checked
func IdentifierMatches(lhs, rhs certgraphapi.CertIdentifier) bool {
	if lhs.PubkeyModulus != rhs.PubkeyModulus {
		return false
	}
	if lhs.CommonName != rhs.CommonName {
		return false
	}
	if lhs.SerialNumber != rhs.SerialNumber {
		return false
	}
	return true
}

func LocateMatchingCertKeyPairs(items []certgraphapi.CertKeyPair, target certgraphapi.CertKeyPair) []*certgraphapi.CertKeyPair {
	foundLocations := []*certgraphapi.CertKeyPair{}
	for _, location := range target.Spec.SecretLocations {
		retLocation, _ := LocateCertKeyPairFromPKIListFromInClusterLocation(items, location)
		if retLocation == nil {
			continue
		}
		existingLocation, _ := LocateCertKeyPairFromPKIListFromIdentifier(foundLocations, retLocation.Spec.CertMetadata.CertIdentifier)
		if existingLocation == nil {
			foundLocations = append(foundLocations, retLocation)
		}
	}

	// matches from cert
	for _, location := range target.Spec.OnDiskLocations {
		retLocation, _ := LocateCertKeyPairFromPKIListFromOnDiskLocation(items, location.Cert.Path)
		if retLocation == nil {
			continue
		}
		existingLocation, _ := LocateCertKeyPairFromPKIListFromIdentifier(foundLocations, retLocation.Spec.CertMetadata.CertIdentifier)
		if existingLocation == nil {
			foundLocations = append(foundLocations, retLocation)
		}
	}
	// matches from key
	for _, location := range target.Spec.OnDiskLocations {
		retLocation, _ := LocateCertKeyPairFromPKIListFromOnDiskLocation(items, location.Key.Path)
		if retLocation == nil {
			continue
		}
		existingLocation, _ := LocateCertKeyPairFromPKIListFromIdentifier(foundLocations, retLocation.Spec.CertMetadata.CertIdentifier)
		if existingLocation == nil {
			foundLocations = append(foundLocations, retLocation)
		}
	}

	return foundLocations
}

func LocateMatchingCertificateAuthorityBundles(items []certgraphapi.CertificateAuthorityBundle, target certgraphapi.CertificateAuthorityBundle) []*certgraphapi.CertificateAuthorityBundle {
	foundLocations := []*certgraphapi.CertificateAuthorityBundle{}
	for _, location := range target.Spec.ConfigMapLocations {
		retLocation, _ := LocateCertificateAuthorityBundleFromPKIListFromInClusterLocation(items, location)
		if retLocation == nil {
			continue
		}
		existingLocation, _ := LocateCertificateAuthorityBundleFromPKIListFromIdentifier(foundLocations, retLocation.Spec.CertificateMetadata)
		if existingLocation == nil {
			foundLocations = append(foundLocations, retLocation)
		}
	}

	for _, location := range target.Spec.OnDiskLocations {
		retLocation, _ := LocateCertificateAuthorityBundleFromPKIListFromOnDiskLocation(items, location.Path)
		if retLocation == nil {
			continue
		}
		existingLocation, _ := LocateCertificateAuthorityBundleFromPKIListFromIdentifier(foundLocations, retLocation.Spec.CertificateMetadata)
		if existingLocation == nil {
			foundLocations = append(foundLocations, retLocation)
		}
	}

	return foundLocations
}

func LocateCertKeyPairFromPKIListFromInClusterLocation(items []certgraphapi.CertKeyPair, targetLocation certgraphapi.InClusterSecretLocation) (*certgraphapi.CertKeyPair, error) {
	for i, currItem := range items {
		for _, currLocation := range currItem.Spec.SecretLocations {
			if targetLocation == currLocation {
				return &items[i], nil
			}
		}
	}
	return nil, fmt.Errorf("not found: %#v", targetLocation)
}

func LocateCertKeyPairFromPKIListFromOnDiskLocation(items []certgraphapi.CertKeyPair, path string) (*certgraphapi.CertKeyPair, error) {
	for _, currItem := range items {
		for _, currLocation := range currItem.Spec.OnDiskLocations {
			if path == currLocation.Key.Path {
				return &currItem, nil
			}
			if path == currLocation.Cert.Path {
				return &currItem, nil
			}
		}
	}
	return nil, fmt.Errorf("not found: %v", path)
}

func LocateCertKeyPairFromPKIListFromIdentifier(items []*certgraphapi.CertKeyPair, target certgraphapi.CertIdentifier) (*certgraphapi.CertKeyPair, error) {
	for _, currItem := range items {
		if IdentifierMatches(currItem.Spec.CertMetadata.CertIdentifier, target) {
			return currItem, nil
		}
	}
	return nil, fmt.Errorf("not found: %#v", target)
}

func LocateCertificateAuthorityBundleFromPKIListFromInClusterLocation(items []certgraphapi.CertificateAuthorityBundle, targetLocation certgraphapi.InClusterConfigMapLocation) (*certgraphapi.CertificateAuthorityBundle, error) {
	for i, currItem := range items {
		for _, currLocation := range currItem.Spec.ConfigMapLocations {
			if targetLocation == currLocation {
				return &items[i], nil
			}
		}
	}
	return nil, fmt.Errorf("not found: %#v", targetLocation)
}

func LocateCertificateAuthorityBundleFromPKIListFromOnDiskLocation(items []certgraphapi.CertificateAuthorityBundle, path string) (*certgraphapi.CertificateAuthorityBundle, error) {
	for _, currItem := range items {
		for _, currLocation := range currItem.Spec.OnDiskLocations {
			if path == currLocation.Path {
				return &currItem, nil
			}
		}
	}
	return nil, fmt.Errorf("not found: %v", path)
}

func LocateCertificateAuthorityBundleFromPKIListFromIdentifier(items []*certgraphapi.CertificateAuthorityBundle, target []certgraphapi.CertKeyMetadata) (*certgraphapi.CertificateAuthorityBundle, error) {
	for _, currItem := range items {
		if certificateAuthorityBundleFromPKIListMatchesIdentifier(currItem, target) {
			return currItem, nil
		}
	}
	return nil, fmt.Errorf("not found: %#v", target)
}

func certificateAuthorityBundleFromPKIListMatchesIdentifier(potentialMatch *certgraphapi.CertificateAuthorityBundle, target []certgraphapi.CertKeyMetadata) bool {
	if len(potentialMatch.Spec.CertificateMetadata) != len(target) {
		return false
	}
	for _, currTarget := range target {
		found := false
		for _, currPotential := range potentialMatch.Spec.CertificateMetadata {
			if IdentifierMatches(currTarget.CertIdentifier, currPotential.CertIdentifier) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
