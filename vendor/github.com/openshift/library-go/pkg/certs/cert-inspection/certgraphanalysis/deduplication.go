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
			if currOut.Spec.CertMetadata.CertIdentifier.PubkeyModulus == currIn.Spec.CertMetadata.CertIdentifier.PubkeyModulus {
				// Append currIn locations to found certkeypair
				ret[j] = combineSecretLocations(ret[j], currIn.Spec.SecretLocations)
				ret[j] = combineCertOnDiskLocations(ret[j], currIn.Spec.OnDiskLocations)
				found = true
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

func combineSecretLocations(in *certgraphapi.CertKeyPair, rhs []certgraphapi.InClusterSecretLocation) *certgraphapi.CertKeyPair {
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

func combineCertOnDiskLocations(in *certgraphapi.CertKeyPair, rhs []certgraphapi.OnDiskCertKeyPairLocation) *certgraphapi.CertKeyPair {
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

func deduplicateCABundles(in []*certgraphapi.CertificateAuthorityBundle) []*certgraphapi.CertificateAuthorityBundle {
	ret := []*certgraphapi.CertificateAuthorityBundle{}
	for _, currIn := range in {
		found := false
		for j, currOut := range ret {
			if currIn == nil {
				panic("one")
			}
			if currOut == nil {
				panic("two")
			}
			if currOut.Name == currIn.Name {
				ret[j] = combineConfigMapLocations(ret[j], currIn.Spec.ConfigMapLocations)
				ret[j] = combineCABundleOnDiskLocations(ret[j], currIn.Spec.OnDiskLocations)
				found = true
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
	for idx := range deduplicateCABundles(bundles) {
		ret.Items = append(ret.Items, *bundles[idx])
	}
	return ret
}

func combineConfigMapLocations(in *certgraphapi.CertificateAuthorityBundle, rhs []certgraphapi.InClusterConfigMapLocation) *certgraphapi.CertificateAuthorityBundle {
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

func combineCABundleOnDiskLocations(in *certgraphapi.CertificateAuthorityBundle, rhs []certgraphapi.OnDiskLocation) *certgraphapi.CertificateAuthorityBundle {
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
