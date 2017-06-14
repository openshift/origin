package origin

import authorizationlister "github.com/openshift/origin/pkg/authorization/generated/listers/authorization/internalversion"

// These adapters are temporary and will be removed when the authorization chains are refactored
// to use Listers.

type LastSyncResourceVersioner interface {
	LastSyncResourceVersion() string
}

type policyLister struct {
	authorizationlister.PolicyLister
	versioner LastSyncResourceVersioner
}

func (l policyLister) LastSyncResourceVersion() string {
	return l.versioner.LastSyncResourceVersion()
}

type clusterPolicyLister struct {
	authorizationlister.ClusterPolicyLister
	versioner LastSyncResourceVersioner
}

func (l clusterPolicyLister) LastSyncResourceVersion() string {
	return l.versioner.LastSyncResourceVersion()
}

type policyBindingLister struct {
	authorizationlister.PolicyBindingLister
	versioner LastSyncResourceVersioner
}

func (l policyBindingLister) LastSyncResourceVersion() string {
	return l.versioner.LastSyncResourceVersion()
}

type clusterPolicyBindingLister struct {
	authorizationlister.ClusterPolicyBindingLister
	versioner LastSyncResourceVersioner
}

func (l clusterPolicyBindingLister) LastSyncResourceVersion() string {
	return l.versioner.LastSyncResourceVersion()
}
