package testclient

import (
	"sync"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	ktestclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/client"
)

// FakeAction is a wrapper around the Kubernetes FakeAction.
// Used for faking various actions on resources.
type FakeAction ktestclient.FakeAction

// Fake implements Interface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the method you want to test easier.
type Fake struct {
	// Fake by default keeps a simple list of the methods that have been called.
	Actions []FakeAction
	Err     error
	// ReactFn is an optional function that will be invoked with the provided action
	// and return a response.
	ReactFn ktestclient.ReactionFunc

	Lock sync.Mutex
}

// NewSimpleFake returns a client that will respond with the provided objects
func NewSimpleFake(objects ...runtime.Object) *Fake {
	o := ktestclient.NewObjects(kapi.Scheme, kapi.Scheme)
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}
	return &Fake{ReactFn: ktestclient.ObjectReaction(o, latest.RESTMapper)}
}

// Invokes registers the passed fake action and reacts on it if a ReactFn
// has been defined
func (c *Fake) Invokes(action FakeAction, obj runtime.Object) (runtime.Object, error) {
	c.Actions = append(c.Actions, action)
	if c.ReactFn != nil {
		return c.ReactFn(ktestclient.FakeAction(action))
	}
	return obj, c.Err
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

// Routes provides a fake REST client for Routes
func (c *Fake) Routes(namespace string) client.RouteInterface {
	return &FakeRoutes{Fake: c, Namespace: namespace}
}

// HostSubnets provides a fake REST client for HostSubnets
func (c *Fake) HostSubnets() client.HostSubnetInterface {
	return &FakeHostSubnet{Fake: c}
}

// ClusterNetwork provides a fake REST client for ClusterNetwork
func (c *Fake) ClusterNetwork() client.ClusterNetworkInterface {
	return &FakeClusterNetwork{Fake: c}
}

// Templates provides a fake REST client for Templates
func (c *Fake) Templates(namespace string) client.TemplateInterface {
	return &FakeTemplates{Fake: c}
}

// TemplateConfigs provides a fake REST client for TemplateConfigs
func (c *Fake) TemplateConfigs(namespace string) client.TemplateConfigInterface {
	return &FakeTemplateConfigs{Fake: c}
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
	return &FakePolicies{Fake: c}
}

// Roles provides a fake REST client for Roles
func (c *Fake) Roles(namespace string) client.RoleInterface {
	return &FakeRoles{Fake: c}
}

// RoleBindings provides a fake REST client for RoleBindings
func (c *Fake) RoleBindings(namespace string) client.RoleBindingInterface {
	return &FakeRoleBindings{Fake: c}
}

// PolicyBindings provides a fake REST client for PolicyBindings
func (c *Fake) PolicyBindings(namespace string) client.PolicyBindingInterface {
	return &FakePolicyBindings{Fake: c}
}

// ResourceAccessReviews provides a fake REST client for ResourceAccessReviews
func (c *Fake) ResourceAccessReviews(namespace string) client.ResourceAccessReviewInterface {
	return &FakeResourceAccessReviews{Fake: c}
}

// ClusterResourceAccessReviews provides a fake REST client for ClusterResourceAccessReviews
func (c *Fake) ClusterResourceAccessReviews() client.ResourceAccessReviewInterface {
	return &FakeClusterResourceAccessReviews{Fake: c}
}

// SubjectAccessReviews provides a fake REST client for SubjectAccessReviews
func (c *Fake) SubjectAccessReviews(namespace string) client.SubjectAccessReviewInterface {
	return &FakeSubjectAccessReviews{Fake: c}
}

// OAuthAccessTokens provides a fake REST client for OAuthAccessTokens
func (c *Fake) OAuthAccessTokens() client.OAuthAccessTokenInterface {
	return &FakeOAuthAccessTokens{Fake: c}
}

// ClusterSubjectAccessReviews provides a fake REST client for ClusterSubjectAccessReviews
func (c *Fake) ClusterSubjectAccessReviews() client.SubjectAccessReviewInterface {
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
