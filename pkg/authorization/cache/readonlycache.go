package cache

import (
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/client"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	clusterbindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	bindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
)

// ReadOnlyCache exposes administrative methods for the readOnlyAuthorizationCache
type ReadOnlyCache interface {
	Run()
	RunUntil(bindingStopChannel, policyStopChannel <-chan struct{})
}

// readOnlyAuthorizationCache embeds four parallel caches for policies and bindings on both the project and cluster level
type readOnlyAuthorizationCache struct {
	readOnlyPolicyCache               *readOnlyPolicyCache
	readOnlyClusterPolicyCache        *readOnlyClusterPolicyCache
	readOnlyPolicyBindingCache        *readOnlyPolicyBindingCache
	readOnlyClusterPolicyBindingCache *readOnlyClusterPolicyBindingCache
}

// Run begins watching and synchronizing the cache
func (c *readOnlyAuthorizationCache) Run() {
	c.readOnlyPolicyCache.Run()
	c.readOnlyClusterPolicyCache.Run()
	c.readOnlyPolicyBindingCache.Run()
	c.readOnlyClusterPolicyBindingCache.Run()
}

// RunUntil starts a watch and handles watch events. Will restart the watch if it is closed.
// RunUntil starts a goroutine and returns immediately. It will exit when stopCh is closed.
func (c *readOnlyAuthorizationCache) RunUntil(bindingStopChannel, policyStopChannel <-chan struct{}) {
	c.readOnlyPolicyCache.RunUntil(policyStopChannel)
	c.readOnlyClusterPolicyCache.RunUntil(policyStopChannel)
	c.readOnlyPolicyBindingCache.RunUntil(bindingStopChannel)
	c.readOnlyClusterPolicyBindingCache.RunUntil(bindingStopChannel)
}

// NewReadOnlyCache creates a new readOnlyAuthorizationCache.  You cannot use a normal client, because you don't want policy guarding the policy from the authorizer
func NewReadOnlyCacheAndClient(bindingRegistry bindingregistry.WatchingRegistry,
	policyRegistry policyregistry.WatchingRegistry,
	clusterBindingRegistry clusterbindingregistry.WatchingRegistry,
	clusterPolicyRegistry clusterpolicyregistry.WatchingRegistry) (cache ReadOnlyCache, client client.ReadOnlyPolicyClient) {
	r := readOnlyAuthorizationCache{
		readOnlyPolicyCache:               NewReadOnlyPolicyCache(policyRegistry),
		readOnlyClusterPolicyCache:        NewReadOnlyClusterPolicyCache(clusterPolicyRegistry),
		readOnlyPolicyBindingCache:        NewReadOnlyPolicyBindingCache(bindingRegistry),
		readOnlyClusterPolicyBindingCache: NewReadOnlyClusterPolicyBindingCache(clusterBindingRegistry),
	}
	cache = &r
	client = &r
	return
}

// LastSyncResourceVersion exposes the LastSyncResourceVersion of the internal reflectors from each cache - conforms to the standard that
// a SyncResourceVersion is a monotonically increasing integer
func (c readOnlyAuthorizationCache) LastSyncResourceVersion() string {
	return strings.Join([]string{
		c.readOnlyPolicyCache.LastSyncResourceVersion(),
		c.readOnlyClusterPolicyCache.LastSyncResourceVersion(),
		c.readOnlyPolicyBindingCache.LastSyncResourceVersion(),
		c.readOnlyClusterPolicyBindingCache.LastSyncResourceVersion(),
	}, "")
}

// Methods below conform the readOnlyAuthorizationCache to the ReadOnlyPolicyClient interface

func (c readOnlyAuthorizationCache) ReadOnlyPolicies(namespace string) client.ReadOnlyPolicyInterface {
	return newReadOnlyPolicies(c, namespace)
}

func (c readOnlyAuthorizationCache) ReadOnlyPolicyBindings(namespace string) client.ReadOnlyPolicyBindingInterface {
	return newReadOnlyPolicyBindings(c, namespace)
}

func (c readOnlyAuthorizationCache) ReadOnlyClusterPolicies() client.ReadOnlyClusterPolicyInterface {
	return newReadOnlyClusterPolicies(c)
}

func (c readOnlyAuthorizationCache) ReadOnlyClusterPolicyBindings() client.ReadOnlyClusterPolicyBindingInterface {
	return newReadOnlyClusterPolicyBindings(c)
}

// namespaceRefersToCluster determines whether the namespace field given to GetPolicy or ListPolicyBindings calls refers to cluster
// policies or cluster policy bindings instead of namespaces policies or policy bindings. Calls to GetPolicy or ListPolicyBindings
// pass an empty string as the namespace to refer to cluster policies or bindings. This conflicts with kapi.NamespaceAll, and calls
// that do not route through GetPolicy() (but rather ReadOnlyPolicies().Get()), or through ReadOnlyPolicyBindings().List() instead
// of ListPolicyBindings() assume that namespace == "" is equivalent to namespace == kapi.NamespaceAll. Calls that route through the
// following methods assume namespace == "" refers to cluster to support the rulevalidation.PolicyGetter and rulevalidation.BindingLister
// interfaces.
func namespaceRefersToCluster(namespace string) bool {
	return len(namespace) == 0
}

// GetPolicy retrieves a specific policy.  It conforms to rulevalidation.PolicyGetter.
func (c readOnlyAuthorizationCache) GetPolicy(ctx kapi.Context, name string) (*authorizationapi.Policy, error) {
	namespace, _ := kapi.NamespaceFrom(ctx)

	if namespaceRefersToCluster(namespace) {
		clusterPolicy, err := c.ReadOnlyClusterPolicies().Get(name)
		if err != nil {
			return &authorizationapi.Policy{}, err
		}
		return authorizationapi.ToPolicy(clusterPolicy), nil
	} else {
		policy, err := c.ReadOnlyPolicies(namespace).Get(name)
		if err != nil {
			return &authorizationapi.Policy{}, err
		}
		return policy, nil
	}
}

// ListPolicyBindings obtains list of policyBindings that match a selector.  It conforms to rulevalidation.BindingLister
func (c readOnlyAuthorizationCache) ListPolicyBindings(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.PolicyBindingList, error) {
	namespace, _ := kapi.NamespaceFrom(ctx)

	if namespaceRefersToCluster(namespace) {
		clusterPolicyBindingList, err := c.ReadOnlyClusterPolicyBindings().List(label, field)
		if err != nil {
			return &authorizationapi.PolicyBindingList{}, err
		}
		return authorizationapi.ToPolicyBindingList(clusterPolicyBindingList), nil
	} else {
		policyBindingList, err := c.ReadOnlyPolicyBindings(namespace).List(label, field)
		if err != nil {
			return &authorizationapi.PolicyBindingList{}, err
		}
		return policyBindingList, nil
	}
}

// GetPolicy retrieves a specific policy.  It conforms to rulevalidation.PolicyGetter.
func (c readOnlyAuthorizationCache) GetClusterPolicy(ctx kapi.Context, name string) (*authorizationapi.ClusterPolicy, error) {
	clusterPolicy, err := c.ReadOnlyClusterPolicies().Get(name)
	if err != nil {
		return &authorizationapi.ClusterPolicy{}, err
	}
	return clusterPolicy, nil
}

// ListPolicyBindings obtains list of policyBindings that match a selector.  It conforms to rulevalidation.BindingLister
func (c readOnlyAuthorizationCache) ListClusterPolicyBindings(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.ClusterPolicyBindingList, error) {
	clusterPolicyBindingList, err := c.ReadOnlyClusterPolicyBindings().List(label, field)
	if err != nil {
		return &authorizationapi.ClusterPolicyBindingList{}, err
	}
	return clusterPolicyBindingList, nil
}
