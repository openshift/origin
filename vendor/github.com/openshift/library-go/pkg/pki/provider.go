package pki

import (
	"fmt"

	configv1alpha1 "github.com/openshift/api/config/v1alpha1"
	configv1alpha1listers "github.com/openshift/client-go/config/listers/config/v1alpha1"
)

// PKIProfileProvider provides the PKIProfile that determines certificate key
// configuration. A nil profile indicates Unmanaged mode where the caller
// should use its own defaults.
type PKIProfileProvider interface {
	PKIProfile() (*configv1alpha1.PKIProfile, error)
}

// StaticPKIProfileProvider is a PKIProfileProvider backed by a fixed PKIProfile.
type StaticPKIProfileProvider struct {
	profile *configv1alpha1.PKIProfile
}

// NewStaticPKIProfileProvider returns a PKIProfileProvider backed by the given
// profile. A nil profile signals Unmanaged mode.
func NewStaticPKIProfileProvider(profile *configv1alpha1.PKIProfile) *StaticPKIProfileProvider {
	return &StaticPKIProfileProvider{profile: profile}
}

// PKIProfile returns the static PKIProfile.
func (s *StaticPKIProfileProvider) PKIProfile() (*configv1alpha1.PKIProfile, error) {
	return s.profile, nil
}

// ListerPKIProfileProvider is a PKIProfileProvider that reads a named
// cluster-scoped PKI resource via a lister.
type ListerPKIProfileProvider struct {
	lister       configv1alpha1listers.PKILister
	resourceName string
}

// NewClusterPKIProfileProvider creates a PKIProfileProvider that resolves the
// PKIProfile from the OpenShift cluster configuration PKI resource.
func NewClusterPKIProfileProvider(lister configv1alpha1listers.PKILister) *ListerPKIProfileProvider {
	return NewListerPKIProfileProvider(lister, "cluster")
}

// NewListerPKIProfileProvider returns a PKIProfileProvider that reads the
// named cluster-scoped PKI resource via a lister.
func NewListerPKIProfileProvider(lister configv1alpha1listers.PKILister, resourceName string) *ListerPKIProfileProvider {
	return &ListerPKIProfileProvider{
		lister:       lister,
		resourceName: resourceName,
	}
}

// PKIProfile reads the PKI resource and returns the profile based on its
// certificate management mode. Returns nil for Unmanaged mode.
func (l *ListerPKIProfileProvider) PKIProfile() (*configv1alpha1.PKIProfile, error) {
	pki, err := l.lister.Get(l.resourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get PKI resource %q: %w", l.resourceName, err)
	}

	switch pki.Spec.CertificateManagement.Mode {
	case configv1alpha1.PKICertificateManagementModeUnmanaged:
		return nil, nil
	case configv1alpha1.PKICertificateManagementModeDefault:
		profile := DefaultPKIProfile()
		return &profile, nil
	case configv1alpha1.PKICertificateManagementModeCustom:
		profile := pki.Spec.CertificateManagement.Custom.PKIProfile
		return &profile, nil
	default:
		return nil, fmt.Errorf("unknown PKI certificate management mode: %q", pki.Spec.CertificateManagement.Mode)
	}
}
