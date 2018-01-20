package imagequalify_test

import (
	"bytes"
	"reflect"
	"testing"

	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/image/admission/imagequalify"
	"github.com/openshift/origin/pkg/image/admission/imagequalify/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

type admissionTest struct {
	config     *testConfig
	attributes admission.Attributes
	handler    *imagequalify.Plugin
	pod        *kapi.Pod
}

type testConfig struct {
	InitContainers         []kapi.Container
	Containers             []kapi.Container
	ExpectedInitContainers []kapi.Container
	ExpectedContainers     []kapi.Container
	AdmissionObject        runtime.Object
	Resource               string
	Subresource            string
	Config                 *api.ImageQualifyConfig
}

func container(image string) kapi.Container {
	return kapi.Container{
		Image: image,
	}
}

func parseConfigRules(rules []api.ImageQualifyRule) (*api.ImageQualifyConfig, error) {
	config, err := configapilatest.WriteYAML(&api.ImageQualifyConfig{
		Rules: rules,
	})

	if err != nil {
		return nil, err
	}

	return imagequalify.ReadConfig(bytes.NewReader(config))
}

func mustParseRules(rules []api.ImageQualifyRule) *api.ImageQualifyConfig {
	config, err := parseConfigRules(rules)
	if err != nil {
		panic(err)
	}
	return config
}

func newTest(c *testConfig) admissionTest {
	pod := kapi.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "admissionTest",
			Namespace: "newAdmissionTest",
		},
		Spec: kapi.PodSpec{
			InitContainers: c.InitContainers,
			Containers:     c.Containers,
		},
	}

	if c.AdmissionObject == nil {
		c.AdmissionObject = &pod
	}

	if c.Resource == "" {
		c.Resource = "pods"
	}

	attributes := admission.NewAttributesRecord(
		c.AdmissionObject,
		nil,
		kapi.Kind("Pod").WithVersion("version"),
		"Namespace",
		"Name",
		kapi.Resource(c.Resource).WithVersion("version"),
		c.Subresource,
		admission.Create, // XXX and update?
		nil)

	return admissionTest{
		attributes: attributes,
		config:     c,
		handler:    imagequalify.NewPlugin(c.Config.Rules),
		pod:        &pod,
	}
}

func imageNames(containers []kapi.Container) []string {
	names := make([]string, len(containers))
	for i := range containers {
		names[i] = containers[i].Image
	}
	return names
}

func assertImageNamesEqual(t *testing.T, expected, actual []kapi.Container) {
	a, b := imageNames(expected), imageNames(actual)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("expected %v, got %v", a, b)
	}
}

func TestAdmissionQualifiesUnqualifiedImages(t *testing.T) {
	rules := []api.ImageQualifyRule{{
		Pattern: "somerepo/*",
		Domain:  "somerepo.io",
	}, {
		Pattern: "nginx",
		Domain:  "nginx.com",
	}, {
		Pattern: "*/*",
		Domain:  "docker.io",
	}, {
		Pattern: "*",
		Domain:  "docker.io",
	}}

	test := newTest(&testConfig{
		InitContainers: []kapi.Container{
			container("somerepo/busybox"),
			container("example.com/nginx"),
			container("nginx"),
			container("emacs"),
		},
		ExpectedInitContainers: []kapi.Container{
			container("somerepo.io/somerepo/busybox"),
			container("example.com/nginx"),
			container("nginx.com/nginx"),
			container("docker.io/emacs"),
		},
		Containers: []kapi.Container{
			container("example.com/busybox"),
			container("nginx"),
			container("vim"),
		},
		ExpectedContainers: []kapi.Container{
			container("example.com/busybox"),
			container("nginx.com/nginx"),
			container("docker.io/vim"),
		},
		Config: mustParseRules(rules),
	})

	if err := test.handler.Admit(test.attributes); err != nil {
		t.Errorf("unexpected error returned from admission handler: %s", err)

	}

	assertImageNamesEqual(t, test.config.ExpectedInitContainers, test.config.InitContainers)
	assertImageNamesEqual(t, test.config.ExpectedContainers, test.config.Containers)

	if err := test.handler.Validate(test.attributes); err != nil {
		t.Errorf("unexpected error returned from admission handler: %s", err)
	}

	assertImageNamesEqual(t, test.config.ExpectedInitContainers, test.config.InitContainers)
	assertImageNamesEqual(t, test.config.ExpectedContainers, test.config.Containers)
}

func TestAdmissionValidateErrors(t *testing.T) {
	rules := []api.ImageQualifyRule{{
		Pattern: "somerepo/*",
		Domain:  "somerepo.io",
	}}

	test := newTest(&testConfig{
		InitContainers: []kapi.Container{
			container("somerepo/busybox"),
		},
		ExpectedInitContainers: []kapi.Container{
			container("somerepo.io/somerepo/busybox"),
		},
		Config: mustParseRules(rules),
	})

	if err := test.handler.Admit(test.attributes); err != nil {
		t.Errorf("unexpected error returned from admission handler: %s", err)

	}

	assertImageNamesEqual(t, test.config.ExpectedInitContainers, test.config.InitContainers)
	assertImageNamesEqual(t, test.config.ExpectedContainers, test.config.Containers)

	// unqualify image post Admit() and now Validate should error.
	test.config.InitContainers[0].Image = "somerepo/busybox"

	if err := test.handler.Validate(test.attributes); err == nil {
		t.Errorf("expected an error from validate")
	}

	// Test again, but on non-init containers.

	test = newTest(&testConfig{
		Containers: []kapi.Container{
			container("somerepo/busybox"),
		},
		ExpectedContainers: []kapi.Container{
			container("somerepo.io/somerepo/busybox"),
		},
		Config: mustParseRules(rules),
	})

	if err := test.handler.Admit(test.attributes); err != nil {
		t.Errorf("unexpected error returned from admission handler: %s", err)

	}

	assertImageNamesEqual(t, test.config.ExpectedInitContainers, test.config.InitContainers)
	assertImageNamesEqual(t, test.config.ExpectedContainers, test.config.Containers)

	// unqualify image post Admit() and now Validate should error.
	test.config.Containers[0].Image = "somerepo/busybox"

	if err := test.handler.Validate(test.attributes); err == nil {
		t.Errorf("expected an error from validate")
	}
}

func TestAdmissionErrorsOnNonPodObject(t *testing.T) {
	rules := []api.ImageQualifyRule{{
		Pattern: "somerepo/*",
		Domain:  "somerepo.io",
	}, {
		Pattern: "nginx",
		Domain:  "nginx.com",
	}}

	test := newTest(&testConfig{
		InitContainers: []kapi.Container{
			container("somerepo/busybox"),
		},
		Containers: []kapi.Container{
			container("foo.io/busybox"),
		},
		AdmissionObject: &kapi.ReplicationController{},
		Config:          mustParseRules(rules),
	})

	if err := test.handler.Admit(test.attributes); err == nil {
		t.Errorf("expected an error from admission handler")
	}

	if err := test.handler.Validate(test.attributes); err == nil {
		t.Errorf("expected an error from admission handler")
	}
}

func TestAdmissionIsIgnoredForSubresource(t *testing.T) {
	rules := []api.ImageQualifyRule{{
		Pattern: "somerepo/*",
		Domain:  "somerepo.io",
	}, {
		Pattern: "nginx",
		Domain:  "nginx.com",
	}}

	test := newTest(&testConfig{
		InitContainers: []kapi.Container{
			container("somerepo/busybox"),
			container("foo.io/nginx"),
		},
		ExpectedInitContainers: []kapi.Container{
			container("somerepo/busybox"),
			container("foo.io/nginx"),
		},
		Containers: []kapi.Container{
			container("foo.io/busybox"),
			container("nginx"),
		},
		ExpectedContainers: []kapi.Container{
			container("foo.io/busybox"),
			container("nginx"),
		},
		Subresource: "subresource",
		Config:      mustParseRules(rules),
	})

	// Not expecting an error for Admit() or Validate() because we
	// are operating on a subresource of pod. The handler will
	// ignore calls for these attributes and this means the
	// container names should remain unchanged.

	if err := test.handler.Admit(test.attributes); err != nil {
		t.Errorf("unexpected error from admission handler: %s", err)
	}

	assertImageNamesEqual(t, test.config.ExpectedInitContainers, test.config.InitContainers)
	assertImageNamesEqual(t, test.config.ExpectedContainers, test.config.Containers)

	if err := test.handler.Validate(test.attributes); err != nil {
		t.Errorf("unexpected error from admission handler: %s", err)
	}

	assertImageNamesEqual(t, test.config.ExpectedInitContainers, test.config.InitContainers)
	assertImageNamesEqual(t, test.config.ExpectedContainers, test.config.Containers)
}

func TestAdmissionErrorsOnNonPodsResource(t *testing.T) {
	rules := []api.ImageQualifyRule{{
		Pattern: "somerepo/*",
		Domain:  "somerepo.io",
	}, {
		Pattern: "nginx",
		Domain:  "nginx.com",
	}}

	test := newTest(&testConfig{
		InitContainers: []kapi.Container{
			container("somerepo/busybox"),
			container("foo.io/nginx"),
		},
		ExpectedInitContainers: []kapi.Container{
			container("somerepo/busybox"),
			container("foo.io/nginx"),
		},
		Containers: []kapi.Container{
			container("foo.io/busybox"),
			container("nginx"),
		},
		ExpectedContainers: []kapi.Container{
			container("foo.io/busybox"),
			container("nginx"),
		},
		Resource: "nonpods",
		Config:   mustParseRules(rules),
	})

	if err := test.handler.Admit(test.attributes); err != nil {
		t.Errorf("expected error from admission handler")
	}

	assertImageNamesEqual(t, test.config.ExpectedInitContainers, test.config.InitContainers)
	assertImageNamesEqual(t, test.config.ExpectedContainers, test.config.Containers)

	if err := test.handler.Validate(test.attributes); err != nil {
		t.Errorf("expected error from admission handler")
	}

	assertImageNamesEqual(t, test.config.ExpectedInitContainers, test.config.InitContainers)
	assertImageNamesEqual(t, test.config.ExpectedContainers, test.config.Containers)
}

func TestAdmissionErrorsWhenImageNamesAreInvalid(t *testing.T) {
	rules := []api.ImageQualifyRule{{
		Pattern: "somerepo/*",
		Domain:  "somerepo.io",
	}, {
		Pattern: "nginx",
		Domain:  "nginx.com",
	}}

	test := newTest(&testConfig{
		InitContainers: []kapi.Container{
			container("foo.io/[]!nginx"),
		},
		Config: mustParseRules(rules),
	})

	if err := test.handler.Admit(test.attributes); err == nil {
		t.Errorf("expected error from admission handler")
	}

	if err := test.handler.Validate(test.attributes); err == nil {
		t.Errorf("expected error from admission handler")
	}

	// Same test, but for non init containers.

	test = newTest(&testConfig{
		Containers: []kapi.Container{
			container("foo.io/[]!nginx"),
		},
		Config: mustParseRules(rules),
	})

	if err := test.handler.Admit(test.attributes); err == nil {
		t.Errorf("expected error from admission handler")
	}

	if err := test.handler.Validate(test.attributes); err == nil {
		t.Errorf("expected error from admission handler")
	}
}
