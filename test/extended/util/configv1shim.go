package util

import (
	"fmt"

	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	fakeconfigv1client "github.com/openshift/client-go/config/clientset/versioned/fake"
	configv1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configv1alpha1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"
)

// ConfigClientShim makes sure whenever there's a static
// manifest present for a config v1 kind, fake client is used
// instead of the real one.
type ConfigClientShim struct {
	adminConfig *rest.Config
	v1Kinds     map[string]bool
	fakeClient  *fakeconfigv1client.Clientset
}

func (c *ConfigClientShim) Discovery() discovery.DiscoveryInterface {
	return configv1client.NewForConfigOrDie(c.adminConfig).Discovery()
}
func (c *ConfigClientShim) ConfigV1() configv1.ConfigV1Interface {
	return &ConfigV1ClientShim{
		configv1:           func() configv1.ConfigV1Interface { return configv1client.NewForConfigOrDie(c.adminConfig).ConfigV1() },
		v1Kinds:            c.v1Kinds,
		fakeConfigV1Client: c.fakeClient.ConfigV1(),
	}
}
func (c *ConfigClientShim) ConfigV1alpha1() configv1alpha1.ConfigV1alpha1Interface {
	return configv1client.NewForConfigOrDie(c.adminConfig).ConfigV1alpha1()
}

var _ configv1client.Interface = &ConfigClientShim{}

// ConfigClientShim makes sure whenever there's a static
// manifest present for a config v1 kind, fake client is used
// instead of the real one.
type ConfigV1ClientShim struct {
	configv1           func() configv1.ConfigV1Interface
	v1Kinds            map[string]bool
	fakeConfigV1Client configv1.ConfigV1Interface
}

func (c *ConfigV1ClientShim) APIServers() configv1.APIServerInterface {
	if c.v1Kinds["APIServer"] {
		return c.fakeConfigV1Client.APIServers()
	}
	return c.configv1().APIServers()
}

func (c *ConfigV1ClientShim) Authentications() configv1.AuthenticationInterface {
	if c.v1Kinds["Authentication"] {
		return c.fakeConfigV1Client.Authentications()
	}
	return c.configv1().Authentications()
}

func (c *ConfigV1ClientShim) Builds() configv1.BuildInterface {
	if c.v1Kinds["Build"] {
		return c.fakeConfigV1Client.Builds()
	}
	return c.configv1().Builds()
}

func (c *ConfigV1ClientShim) ClusterOperators() configv1.ClusterOperatorInterface {
	if c.v1Kinds["ClusterOperator"] {
		return c.fakeConfigV1Client.ClusterOperators()
	}
	return c.configv1().ClusterOperators()
}

func (c *ConfigV1ClientShim) ClusterVersions() configv1.ClusterVersionInterface {
	if c.v1Kinds["ClusterVersion"] {
		return c.fakeConfigV1Client.ClusterVersions()
	}
	return c.configv1().ClusterVersions()
}

func (c *ConfigV1ClientShim) Consoles() configv1.ConsoleInterface {
	if c.v1Kinds["Console"] {
		return c.fakeConfigV1Client.Consoles()
	}
	return c.configv1().Consoles()
}

func (c *ConfigV1ClientShim) DNSes() configv1.DNSInterface {
	if c.v1Kinds["DNS"] {
		return c.fakeConfigV1Client.DNSes()
	}
	return c.configv1().DNSes()
}

func (c *ConfigV1ClientShim) FeatureGates() configv1.FeatureGateInterface {
	if c.v1Kinds["FeatureGate"] {
		return c.fakeConfigV1Client.FeatureGates()
	}
	return c.configv1().FeatureGates()
}

func (c *ConfigV1ClientShim) Images() configv1.ImageInterface {
	if c.v1Kinds["Image"] {
		return c.fakeConfigV1Client.Images()
	}
	return c.configv1().Images()
}

func (c *ConfigV1ClientShim) ImageContentPolicies() configv1.ImageContentPolicyInterface {
	if c.v1Kinds["ImageContentPolicie"] {
		return c.fakeConfigV1Client.ImageContentPolicies()
	}
	return c.configv1().ImageContentPolicies()
}

func (c *ConfigV1ClientShim) ImageDigestMirrorSets() configv1.ImageDigestMirrorSetInterface {
	if c.v1Kinds["ImageDigestMirrorSet"] {
		return c.fakeConfigV1Client.ImageDigestMirrorSets()
	}
	return c.configv1().ImageDigestMirrorSets()
}

func (c *ConfigV1ClientShim) ImageTagMirrorSets() configv1.ImageTagMirrorSetInterface {
	if c.v1Kinds["ImageTagMirrorSet"] {
		return c.fakeConfigV1Client.ImageTagMirrorSets()
	}
	return c.configv1().ImageTagMirrorSets()
}

func (c *ConfigV1ClientShim) Infrastructures() configv1.InfrastructureInterface {
	if c.v1Kinds["Infrastructure"] {
		return c.fakeConfigV1Client.Infrastructures()
	}
	return c.configv1().Infrastructures()
}

func (c *ConfigV1ClientShim) Ingresses() configv1.IngressInterface {
	if c.v1Kinds["Ingresse"] {
		return c.fakeConfigV1Client.Ingresses()
	}
	return c.configv1().Ingresses()
}

func (c *ConfigV1ClientShim) Networks() configv1.NetworkInterface {
	if c.v1Kinds["Network"] {
		return c.fakeConfigV1Client.Networks()
	}
	return c.configv1().Networks()
}

func (c *ConfigV1ClientShim) Nodes() configv1.NodeInterface {
	if c.v1Kinds["Node"] {
		return c.fakeConfigV1Client.Nodes()
	}
	return c.configv1().Nodes()
}

func (c *ConfigV1ClientShim) OAuths() configv1.OAuthInterface {
	if c.v1Kinds["OAuth"] {
		return c.fakeConfigV1Client.OAuths()
	}
	return c.configv1().OAuths()
}

func (c *ConfigV1ClientShim) OperatorHubs() configv1.OperatorHubInterface {
	if c.v1Kinds["OperatorHub"] {
		return c.fakeConfigV1Client.OperatorHubs()
	}
	return c.configv1().OperatorHubs()
}

func (c *ConfigV1ClientShim) Projects() configv1.ProjectInterface {
	if c.v1Kinds["Project"] {
		return c.fakeConfigV1Client.Projects()
	}
	return c.configv1().Projects()
}

func (c *ConfigV1ClientShim) Proxies() configv1.ProxyInterface {
	if c.v1Kinds["Proxie"] {
		return c.fakeConfigV1Client.Proxies()
	}
	return c.configv1().Proxies()
}

func (c *ConfigV1ClientShim) Schedulers() configv1.SchedulerInterface {
	if c.v1Kinds["Scheduler"] {
		return c.fakeConfigV1Client.Schedulers()
	}
	return c.configv1().Schedulers()
}

func (c *ConfigV1ClientShim) RESTClient() rest.Interface {
	return c.configv1().RESTClient()
}

var _ configv1.ConfigV1Interface = &ConfigV1ClientShim{}

var kind2resourceMapping = map[string]string{
	"APIServer":            "apiservers",
	"Authentication":       "authentications",
	"Build":                "build",
	"ClusterOperator":      "clusteroperators",
	"ClusterVersion":       "clusterversions",
	"Console":              "consoles",
	"DNS":                  "dnses",
	"FeatureGate":          "featuregates",
	"Image":                "images",
	"ImageContentPolicie":  "imagecontentpolicies",
	"ImageDigestMirrorSet": "imagedigestmirrorsetes",
	"ImageTagMirrorSet":    "imagetagmirrorsetes",
	"Infrastructure":       "infrastructures",
	"Ingresse":             "ingresses",
	"Network":              "networks",
	"Node":                 "nodes",
	"OAuth":                "oauths",
	"OperatorHub":          "operatorhub",
	"Project":              "projects",
	"Proxie":               "proxies",
	"Scheduler":            "schedulers",
}

type OperationNotPermitted struct {
	Action string
}

func (e OperationNotPermitted) Error() string {
	return fmt.Sprintf("operation %q not permitted", e.Action)
}

func NewConfigClientShim(
	adminConfig *rest.Config,
	objects []runtime.Object,
) (error, *ConfigClientShim) {
	fakeClient := fakeconfigv1client.NewSimpleClientset(objects...)

	v1Kinds := make(map[string]bool)
	// make sure every mutating operation is not permitted
	for _, object := range objects {
		objectKind := object.GetObjectKind().GroupVersionKind()
		// currently supportig only config.openshift.io/v1 apiversion
		if objectKind.Group != "config.openshift.io" {
			return fmt.Errorf("unknown group: %v", objectKind.Group), nil
		}
		if objectKind.Version != "v1" {
			return fmt.Errorf("unknown version: %v", objectKind.Version), nil
		}
		resource, exists := kind2resourceMapping[objectKind.Kind]
		if !exists {
			return fmt.Errorf("unknown kind mapping for %v", objectKind.Kind), nil
		}
		fakeClient.Fake.PrependReactor("update", resource, func(action clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, &OperationNotPermitted{Action: "update"}
		})
		fakeClient.Fake.PrependReactor("delete", resource, func(action clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, &OperationNotPermitted{Action: "delete"}
		})
		v1Kinds[objectKind.Kind] = true
	}

	return nil, &ConfigClientShim{
		adminConfig: adminConfig,
		v1Kinds:     v1Kinds,
		fakeClient:  fakeClient,
	}
}
