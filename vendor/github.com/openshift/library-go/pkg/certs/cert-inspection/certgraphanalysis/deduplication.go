package certgraphanalysis

import (
	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
)

func deduplicateCertKeyPairs(in []*certgraphapi.CertKeyPair) []*certgraphapi.CertKeyPair {
	ret := []*certgraphapi.CertKeyPair{}
	for _, currIn := range in {
		if currIn == nil {
			panic("currIn is nil")
		}

		found := false
		for j, currOut := range ret {
			if currOut == nil {
				panic("currOut is nil")
			}
			// currOut has no name set - skip and allow it to be found later
			if len(currOut.Name) == 0 {
				continue
			}
			// Merge if cert identifiers match
			if currOut.Spec.CertMetadata.CertIdentifier.PubkeyModulus == currIn.Spec.CertMetadata.CertIdentifier.PubkeyModulus {
				found = true
				ret[j] = CombineSecretLocations(ret[j], currIn.Spec.SecretLocations)
				ret[j] = CombineCertOnDiskLocations(ret[j], currIn.Spec.OnDiskLocations)
				ret[j] = CombineCertInMemoryLocations(ret[j], currIn.Spec.InMemoryLocations)
				break
			}
		}

		// No match found - add currIn as is
		if !found {
			ret = append(ret, currIn.DeepCopy())
		}
	}

	return ret
}

func deduplicateCertKeyPairList(in *certgraphapi.CertKeyPairList) *certgraphapi.CertKeyPairList {
	ret := &certgraphapi.CertKeyPairList{
		Items: []certgraphapi.CertKeyPair{},
	}
	certs := []*certgraphapi.CertKeyPair{}
	for idx := range in.Items {
		certs = append(certs, &in.Items[idx])
	}
	dedup := deduplicateCertKeyPairs(certs)
	for idx := range dedup {
		ret.Items = append(ret.Items, *dedup[idx])
	}
	return ret
}

// CombineSecretLocations returns a CertKeyPair with all in-cluster locations from in and rhs de-duplicated into a single list
func CombineSecretLocations(in *certgraphapi.CertKeyPair, rhs []certgraphapi.InClusterSecretLocation) *certgraphapi.CertKeyPair {
	out := in.DeepCopy()
	for _, curr := range rhs {
		found := false
		for _, existing := range in.Spec.SecretLocations {
			if curr == existing {
				found = true
			}
		}
		if !found {
			out.Spec.SecretLocations = append(out.Spec.SecretLocations, curr)
		}
	}

	return out
}

// CombineCertOnDiskLocations returns a CertKeyPair with all on-disk locations from in and rhs de-duplicated into a single list
func CombineCertOnDiskLocations(in *certgraphapi.CertKeyPair, rhs []certgraphapi.OnDiskCertKeyPairLocation) *certgraphapi.CertKeyPair {
	keyLocation := certgraphapi.OnDiskLocation{}
	out := in.DeepCopy()
	for _, curr := range rhs {
		found := false
		for _, existing := range in.Spec.OnDiskLocations {
			if curr == existing {
				found = true
			}
		}
		if !found {
			// Store key to be merged into Cert
			if len(curr.Cert.Path) == 0 {
				keyLocation = curr.Key
				continue
			}
			out.Spec.OnDiskLocations = append(out.Spec.OnDiskLocations, curr)
		}
	}

	// Fill in Key property if it was found earlier and unset
	if len(keyLocation.Path) == 0 {
		return out
	}
	for idx, loc := range out.Spec.OnDiskLocations {
		if len(loc.Key.Path) == 0 {
			out.Spec.OnDiskLocations[idx].Key = keyLocation
		}
	}

	return out
}

func deduplicateCABundles(in []*certgraphapi.CertificateAuthorityBundle) []*certgraphapi.CertificateAuthorityBundle {
	ret := []*certgraphapi.CertificateAuthorityBundle{}
	for _, currIn := range in {
		if currIn == nil {
			panic("one")
		}

		found := false
		for j, currOut := range ret {
			if currOut == nil {
				panic("two")
			}
			if currOut.Name == currIn.Name {
				found = true
				ret[j] = CombineConfigMapLocations(ret[j], currIn.Spec.ConfigMapLocations)
				ret[j] = CombineCABundleOnDiskLocations(ret[j], currIn.Spec.OnDiskLocations)
				break
			}
		}

		if !found {
			ret = append(ret, currIn.DeepCopy())
		}
	}

	return ret
}

func deduplicateCABundlesList(in *certgraphapi.CertificateAuthorityBundleList) *certgraphapi.CertificateAuthorityBundleList {
	ret := &certgraphapi.CertificateAuthorityBundleList{
		Items: []certgraphapi.CertificateAuthorityBundle{},
	}
	bundles := []*certgraphapi.CertificateAuthorityBundle{}
	for idx := range in.Items {
		bundles = append(bundles, &in.Items[idx])
	}
	dedup := deduplicateCABundles(bundles)
	for idx := range dedup {
		ret.Items = append(ret.Items, *dedup[idx])
	}
	return ret
}

// CombineConfigMapLocations returns a CertificateAuthorityBundle with all in-cluster locations from in and rhs de-duplicated into a single list
func CombineConfigMapLocations(in *certgraphapi.CertificateAuthorityBundle, rhs []certgraphapi.InClusterConfigMapLocation) *certgraphapi.CertificateAuthorityBundle {
	out := in.DeepCopy()
	for _, curr := range rhs {
		found := false
		for _, existing := range in.Spec.ConfigMapLocations {
			if curr == existing {
				found = true
			}
		}
		if !found {
			out.Spec.ConfigMapLocations = append(out.Spec.ConfigMapLocations, curr)
		}
	}

	return out
}

// CombineCABundleOnDiskLocations returns a CertificateAuthorityBundle with all on-disk locations from in and rhs de-duplicated into a single list
func CombineCABundleOnDiskLocations(in *certgraphapi.CertificateAuthorityBundle, rhs []certgraphapi.OnDiskLocation) *certgraphapi.CertificateAuthorityBundle {
	out := in.DeepCopy()
	for _, curr := range rhs {
		found := false
		for _, existing := range in.Spec.OnDiskLocations {
			if curr == existing {
				found = true
			}
		}
		if !found {
			out.Spec.OnDiskLocations = append(out.Spec.OnDiskLocations, curr)
		}
	}

	return out
}

func deduplicateOnDiskMetadata(in certgraphapi.PerOnDiskResourceData) certgraphapi.PerOnDiskResourceData {
	out := certgraphapi.PerOnDiskResourceData{}
	for _, curr := range in.TLSArtifact {
		found := false
		for _, existing := range out.TLSArtifact {
			if existing.Path == curr.Path {
				found = true
			}
		}
		if !found {
			out.TLSArtifact = append(out.TLSArtifact, curr)
		}
	}
	return out
}

// CombineCertInMemoryLocations returns a CertKeyPair with all in-memory locations from in and rhs de-duplicated into a single list
func CombineCertInMemoryLocations(in *certgraphapi.CertKeyPair, rhs []certgraphapi.InClusterPodLocation) *certgraphapi.CertKeyPair {
	out := in.DeepCopy()
	for _, curr := range rhs {
		found := false
		for _, existing := range in.Spec.InMemoryLocations {
			if curr == existing {
				found = true
			}
		}
		if !found {
			out.Spec.InMemoryLocations = append(out.Spec.InMemoryLocations, curr)
		}
	}
	return out
}
