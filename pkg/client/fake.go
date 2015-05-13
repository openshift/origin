package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

type FakeAction testclient.FakeAction

// Fake implements Interface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the method you want to test easier.
type Fake struct {
	// Fake by default keeps a simple list of the methods that have been called.
	Actions []FakeAction
	Err     error
	// ReactFn is an optional function that will be invoked with the provided action
	// and return a response.
	ReactFn testclient.ReactionFunc
}

func (c *Fake) Invokes(action FakeAction, obj runtime.Object) (runtime.Object, error) {
	c.Actions = append(c.Actions, action)
	if c.ReactFn != nil {
		return c.ReactFn(testclient.FakeAction(action))
	}
	return obj, c.Err
}

var _ Interface = &Fake{}

func (c *Fake) Builds(namespace string) BuildInterface {
	return &FakeBuilds{Fake: c, Namespace: namespace}
}

func (c *Fake) BuildConfigs(namespace string) BuildConfigInterface {
	return &FakeBuildConfigs{Fake: c, Namespace: namespace}
}

func (c *Fake) BuildLogs(namespace string) BuildLogsInterface {
	return &FakeBuildLogs{Fake: c, Namespace: namespace}
}

func (c *Fake) Images() ImageInterface {
	return &FakeImages{Fake: c}
}

func (c *Fake) ImageStreams(namespace string) ImageStreamInterface {
	return &FakeImageStreams{Fake: c, Namespace: namespace}
}

func (c *Fake) ImageStreamMappings(namespace string) ImageStreamMappingInterface {
	return &FakeImageStreamMappings{Fake: c, Namespace: namespace}
}

func (c *Fake) ImageStreamTags(namespace string) ImageStreamTagInterface {
	return &FakeImageStreamTags{Fake: c, Namespace: namespace}
}

func (c *Fake) ImageStreamImages(namespace string) ImageStreamImageInterface {
	return &FakeImageStreamImages{Fake: c, Namespace: namespace}
}

func (c *Fake) DeploymentConfigs(namespace string) DeploymentConfigInterface {
	return &FakeDeploymentConfigs{Fake: c, Namespace: namespace}
}

func (c *Fake) Routes(namespace string) RouteInterface {
	return &FakeRoutes{Fake: c, Namespace: namespace}
}

func (c *Fake) HostSubnets() HostSubnetInterface {
	return &FakeHostSubnet{Fake: c}
}

func (c *Fake) ClusterNetwork() ClusterNetworkInterface {
	return &FakeClusterNetwork{Fake: c}
}

func (c *Fake) Templates(namespace string) TemplateInterface {
	return &FakeTemplates{Fake: c}
}

func (c *Fake) TemplateConfigs(namespace string) TemplateConfigInterface {
	return &FakeTemplateConfigs{Fake: c}
}

func (c *Fake) Identities() IdentityInterface {
	return &FakeIdentities{Fake: c}
}

func (c *Fake) Users() UserInterface {
	return &FakeUsers{Fake: c}
}

func (c *Fake) UserIdentityMappings() UserIdentityMappingInterface {
	return &FakeUserIdentityMappings{Fake: c}
}

func (c *Fake) Projects() ProjectInterface {
	return &FakeProjects{Fake: c}
}

func (c *Fake) ProjectRequests() ProjectRequestInterface {
	return &FakeProjectRequests{Fake: c}
}

func (c *Fake) Policies(namespace string) PolicyInterface {
	return &FakePolicies{Fake: c}
}

func (c *Fake) Roles(namespace string) RoleInterface {
	return &FakeRoles{Fake: c}
}

func (c *Fake) RoleBindings(namespace string) RoleBindingInterface {
	return &FakeRoleBindings{Fake: c}
}

func (c *Fake) PolicyBindings(namespace string) PolicyBindingInterface {
	return &FakePolicyBindings{Fake: c}
}

func (c *Fake) ResourceAccessReviews(namespace string) ResourceAccessReviewInterface {
	return &FakeResourceAccessReviews{Fake: c}
}

func (c *Fake) ClusterResourceAccessReviews() ResourceAccessReviewInterface {
	return &FakeClusterResourceAccessReviews{Fake: c}
}

func (c *Fake) SubjectAccessReviews(namespace string) SubjectAccessReviewInterface {
	return &FakeSubjectAccessReviews{Fake: c}
}

func (c *Fake) OAuthAccessTokens() OAuthAccessTokenInterface {
	return &FakeOAuthAccessTokens{Fake: c}

}
func (c *Fake) ClusterSubjectAccessReviews() SubjectAccessReviewInterface {
	return &FakeClusterSubjectAccessReviews{Fake: c}

}

func (c *Fake) ClusterPolicies() ClusterPolicyInterface {
	return &FakeClusterPolicies{Fake: c}
}

func (c *Fake) ClusterPolicyBindings() ClusterPolicyBindingInterface {
	return &FakeClusterPolicyBindings{Fake: c}
}

func (c *Fake) ClusterRoles() ClusterRoleInterface {
	return &FakeClusterRoles{Fake: c}
}

func (c *Fake) ClusterRoleBindings() ClusterRoleBindingInterface {
	return &FakeClusterRoleBindings{Fake: c}
}
