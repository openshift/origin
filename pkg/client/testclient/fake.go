package testclient

import (
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/api"

	_ "github.com/openshift/origin/pkg/api/install"
	"github.com/openshift/origin/pkg/client"
)

// Fake implements Interface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the method you want to test easier.
type Fake struct {
	sync.RWMutex
	actions []clientgotesting.Action // these may be castable to other types, but "Action" is the minimum

	// ReactionChain is the list of reactors that will be attempted for every request in the order they are tried
	ReactionChain []clientgotesting.Reactor
	// WatchReactionChain is the list of watch reactors that will be attempted for every request in the order they are tried
	WatchReactionChain []clientgotesting.WatchReactor
}

// NewSimpleFake returns a client that will respond with the provided objects
func NewSimpleFake(objects ...runtime.Object) *Fake {
	o := clientgotesting.NewObjectTracker(kapi.Scheme, kapi.Codecs.UniversalDecoder())
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	fakeClient := &Fake{}
	fakeClient.AddReactor("*", "*", clientgotesting.ObjectReaction(o))

	fakeClient.AddWatchReactor("*", clientgotesting.DefaultWatchReactor(watch.NewFake(), nil))

	return fakeClient
}

// AddReactor appends a reactor to the end of the chain
func (c *Fake) AddReactor(verb, resource string, reaction clientgotesting.ReactionFunc) {
	c.ReactionChain = append(c.ReactionChain, &clientgotesting.SimpleReactor{Verb: verb, Resource: resource, Reaction: reaction})
}

// PrependReactor adds a reactor to the beginning of the chain
func (c *Fake) PrependReactor(verb, resource string, reaction clientgotesting.ReactionFunc) {
	c.ReactionChain = append([]clientgotesting.Reactor{&clientgotesting.SimpleReactor{Verb: verb, Resource: resource, Reaction: reaction}}, c.ReactionChain...)
}

// AddWatchReactor appends a reactor to the end of the chain
func (c *Fake) AddWatchReactor(resource string, reaction clientgotesting.WatchReactionFunc) {
	c.WatchReactionChain = append(c.WatchReactionChain, &clientgotesting.SimpleWatchReactor{Resource: resource, Reaction: reaction})
}

// PrependWatchReactor adds a reactor to the beginning of the chain.
func (c *Fake) PrependWatchReactor(resource string, reaction clientgotesting.WatchReactionFunc) {
	c.WatchReactionChain = append([]clientgotesting.WatchReactor{&clientgotesting.SimpleWatchReactor{Resource: resource, Reaction: reaction}}, c.WatchReactionChain...)
}

// Invokes records the provided Action and then invokes the ReactFn (if provided).
// defaultReturnObj is expected to be of the same type a normal call would return.
func (c *Fake) Invokes(action clientgotesting.Action, defaultReturnObj runtime.Object) (runtime.Object, error) {
	c.Lock()
	defer c.Unlock()

	c.actions = append(c.actions, action)
	for _, reactor := range c.ReactionChain {
		if !reactor.Handles(action) {
			continue
		}

		handled, ret, err := reactor.React(action)
		if !handled {
			continue
		}

		return ret, err
	}

	return defaultReturnObj, nil
}

// InvokesWatch records the provided Action and then invokes the ReactFn (if provided).
func (c *Fake) InvokesWatch(action clientgotesting.Action) (watch.Interface, error) {
	c.Lock()
	defer c.Unlock()

	c.actions = append(c.actions, action)
	for _, reactor := range c.WatchReactionChain {
		if !reactor.Handles(action) {
			continue
		}

		handled, ret, err := reactor.React(action)
		if !handled {
			continue
		}

		return ret, err
	}

	return nil, fmt.Errorf("unhandled watch: %#v", action)
}

// ClearActions clears the history of actions called on the fake client
func (c *Fake) ClearActions() {
	c.Lock()
	c.Unlock()

	c.actions = make([]clientgotesting.Action, 0)
}

// Actions returns a chronologically ordered slice fake actions called on the fake client
func (c *Fake) Actions() []clientgotesting.Action {
	c.RLock()
	defer c.RUnlock()

	fa := make([]clientgotesting.Action, len(c.actions))
	copy(fa, c.actions)
	return fa
}

var _ client.Interface = &Fake{}

// Builds provides a fake REST client for Builds
func (c *Fake) Builds(namespace string) client.BuildInterface {
	return &FakeBuilds{Fake: c, Namespace: namespace}
}

// BuildConfigs provides a fake REST client for BuildConfigs
func (c *Fake) BuildConfigs(namespace string) client.BuildConfigInterface {
	return &FakeBuildConfigs{Fake: c, Namespace: namespace}
}

// BuildLogs provides a fake REST client for BuildLogs
func (c *Fake) BuildLogs(namespace string) client.BuildLogsInterface {
	return &FakeBuildLogs{Fake: c, Namespace: namespace}
}

// Images provides a fake REST client for Images
func (c *Fake) Images() client.ImageInterface {
	return &FakeImages{Fake: c}
}

// ImageSignatures provides a fake REST client for ImageSignatures
func (c *Fake) ImageSignatures() client.ImageSignatureInterface {
	return &FakeImageSignatures{Fake: c}
}

// ImageStreams provides a fake REST client for ImageStreams
func (c *Fake) ImageStreamSecrets(namespace string) client.ImageStreamSecretInterface {
	return &FakeImageStreamSecrets{Fake: c, Namespace: namespace}
}

// ImageStreams provides a fake REST client for ImageStreams
func (c *Fake) ImageStreams(namespace string) client.ImageStreamInterface {
	return &FakeImageStreams{Fake: c, Namespace: namespace}
}

// ImageStreamMappings provides a fake REST client for ImageStreamMappings
func (c *Fake) ImageStreamMappings(namespace string) client.ImageStreamMappingInterface {
	return &FakeImageStreamMappings{Fake: c, Namespace: namespace}
}

// ImageStreamTags provides a fake REST client for ImageStreamTags
func (c *Fake) ImageStreamTags(namespace string) client.ImageStreamTagInterface {
	return &FakeImageStreamTags{Fake: c, Namespace: namespace}
}

// ImageStreamImages provides a fake REST client for ImageStreamImages
func (c *Fake) ImageStreamImages(namespace string) client.ImageStreamImageInterface {
	return &FakeImageStreamImages{Fake: c, Namespace: namespace}
}

// DeploymentConfigs provides a fake REST client for DeploymentConfigs
func (c *Fake) DeploymentConfigs(namespace string) client.DeploymentConfigInterface {
	return &FakeDeploymentConfigs{Fake: c, Namespace: namespace}
}

// DeploymentLogs provides a fake REST client for DeploymentLogs
func (c *Fake) DeploymentLogs(namespace string) client.DeploymentLogInterface {
	return &FakeDeploymentLogs{Fake: c, Namespace: namespace}
}

// Routes provides a fake REST client for Routes
func (c *Fake) Routes(namespace string) client.RouteInterface {
	return &FakeRoutes{Fake: c, Namespace: namespace}
}

// HostSubnets provides a fake REST client for HostSubnets
func (c *Fake) HostSubnets() client.HostSubnetInterface {
	return &FakeHostSubnet{Fake: c}
}

// NetNamespaces provides a fake REST client for NetNamespaces
func (c *Fake) NetNamespaces() client.NetNamespaceInterface {
	return &FakeNetNamespace{Fake: c}
}

// ClusterNetwork provides a fake REST client for ClusterNetwork
func (c *Fake) ClusterNetwork() client.ClusterNetworkInterface {
	return &FakeClusterNetwork{Fake: c}
}

// EgressNetworkPolicies provides a fake REST client for EgressNetworkPolicies
func (c *Fake) EgressNetworkPolicies(namespace string) client.EgressNetworkPolicyInterface {
	return &FakeEgressNetworkPolicy{Fake: c, Namespace: namespace}
}

// Templates provides a fake REST client for Templates
func (c *Fake) Templates(namespace string) client.TemplateInterface {
	return &FakeTemplates{Fake: c, Namespace: namespace}
}

// TemplateConfigs provides a fake REST client for TemplateConfigs
func (c *Fake) TemplateConfigs(namespace string) client.TemplateConfigInterface {
	return &FakeTemplateConfigs{Fake: c, Namespace: namespace}
}

// Identities provides a fake REST client for Identities
func (c *Fake) Identities() client.IdentityInterface {
	return &FakeIdentities{Fake: c}
}

// Users provides a fake REST client for Users
func (c *Fake) Users() client.UserInterface {
	return &FakeUsers{Fake: c}
}

// UserIdentityMappings provides a fake REST client for UserIdentityMappings
func (c *Fake) UserIdentityMappings() client.UserIdentityMappingInterface {
	return &FakeUserIdentityMappings{Fake: c}
}

// Groups provides a fake REST client for Groups
func (c *Fake) Groups() client.GroupInterface {
	return &FakeGroups{Fake: c}
}

// Projects provides a fake REST client for Projects
func (c *Fake) Projects() client.ProjectInterface {
	return &FakeProjects{Fake: c}
}

// ProjectRequests provides a fake REST client for ProjectRequests
func (c *Fake) ProjectRequests() client.ProjectRequestInterface {
	return &FakeProjectRequests{Fake: c}
}

// Policies provides a fake REST client for Policies
func (c *Fake) Policies(namespace string) client.PolicyInterface {
	return &FakePolicies{Fake: c, Namespace: namespace}
}

// Roles provides a fake REST client for Roles
func (c *Fake) Roles(namespace string) client.RoleInterface {
	return &FakeRoles{Fake: c, Namespace: namespace}
}

// RoleBindings provides a fake REST client for RoleBindings
func (c *Fake) RoleBindings(namespace string) client.RoleBindingInterface {
	return &FakeRoleBindings{Fake: c, Namespace: namespace}
}

// PolicyBindings provides a fake REST client for PolicyBindings
func (c *Fake) PolicyBindings(namespace string) client.PolicyBindingInterface {
	return &FakePolicyBindings{Fake: c, Namespace: namespace}
}

func (c *Fake) SelfSubjectRulesReviews(namespace string) client.SelfSubjectRulesReviewInterface {
	return &FakeSelfSubjectRulesReviews{Fake: c, Namespace: namespace}
}

func (c *Fake) SubjectRulesReviews(namespace string) client.SubjectRulesReviewInterface {
	return &FakeSubjectRulesReviews{Fake: c, Namespace: namespace}
}

// LocalResourceAccessReviews provides a fake REST client for ResourceAccessReviews
func (c *Fake) LocalResourceAccessReviews(namespace string) client.LocalResourceAccessReviewInterface {
	return &FakeLocalResourceAccessReviews{Fake: c}
}

// ResourceAccessReviews provides a fake REST client for ClusterResourceAccessReviews
func (c *Fake) ResourceAccessReviews() client.ResourceAccessReviewInterface {
	return &FakeClusterResourceAccessReviews{Fake: c}
}

// ImpersonateSubjectAccessReviews provides a fake REST client for SubjectAccessReviews
func (c *Fake) ImpersonateSubjectAccessReviews(token string) client.SubjectAccessReviewInterface {
	return &FakeClusterSubjectAccessReviews{Fake: c}
}

// ImpersonateSubjectAccessReviews provides a fake REST client for SubjectAccessReviews
func (c *Fake) ImpersonateLocalSubjectAccessReviews(namespace, token string) client.LocalSubjectAccessReviewInterface {
	return &FakeLocalSubjectAccessReviews{Fake: c, Namespace: namespace}
}

func (c *Fake) OAuthClients() client.OAuthClientInterface {
	return &FakeOAuthClient{Fake: c}
}

func (c *Fake) OAuthClientAuthorizations() client.OAuthClientAuthorizationInterface {
	return &FakeOAuthClientAuthorization{Fake: c}
}

func (c *Fake) OAuthAccessTokens() client.OAuthAccessTokenInterface {
	return &FakeOAuthAccessTokens{Fake: c}
}

func (c *Fake) OAuthAuthorizeTokens() client.OAuthAuthorizeTokenInterface {
	return &FakeOAuthAuthorizeTokens{Fake: c}
}

// LocalSubjectAccessReviews provides a fake REST client for SubjectAccessReviews
func (c *Fake) LocalSubjectAccessReviews(namespace string) client.LocalSubjectAccessReviewInterface {
	return &FakeLocalSubjectAccessReviews{Fake: c}
}

// SubjectAccessReviews provides a fake REST client for ClusterSubjectAccessReviews
func (c *Fake) SubjectAccessReviews() client.SubjectAccessReviewInterface {
	return &FakeClusterSubjectAccessReviews{Fake: c}
}

// ClusterPolicies provides a fake REST client for ClusterPolicies
func (c *Fake) ClusterPolicies() client.ClusterPolicyInterface {
	return &FakeClusterPolicies{Fake: c}
}

// ClusterPolicyBindings provides a fake REST client for ClusterPolicyBindings
func (c *Fake) ClusterPolicyBindings() client.ClusterPolicyBindingInterface {
	return &FakeClusterPolicyBindings{Fake: c}
}

// ClusterRoles provides a fake REST client for ClusterRoles
func (c *Fake) ClusterRoles() client.ClusterRoleInterface {
	return &FakeClusterRoles{Fake: c}
}

// ClusterRoleBindings provides a fake REST client for ClusterRoleBindings
func (c *Fake) ClusterRoleBindings() client.ClusterRoleBindingInterface {
	return &FakeClusterRoleBindings{Fake: c}
}

func (c *Fake) ClusterResourceQuotas() client.ClusterResourceQuotaInterface {
	return &FakeClusterResourceQuotas{Fake: c}
}

func (c *Fake) AppliedClusterResourceQuotas(namespace string) client.AppliedClusterResourceQuotaInterface {
	return &FakeAppliedClusterResourceQuotas{Fake: c, Namespace: namespace}
}

func (c *Fake) RoleBindingRestrictions(namespace string) client.RoleBindingRestrictionInterface {
	return &FakeRoleBindingRestrictions{Fake: c, Namespace: namespace}
}
