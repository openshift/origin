package imagepolicy

import (
	"os"
	"strings"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	apierrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/openshift/origin/pkg/client/testclient"
	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
	_ "github.com/openshift/origin/pkg/image/admission/imagepolicy/api/install"
	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api/validation"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type resolveFunc func(ref imageapi.DockerImageReference) (*imageapi.Image, error)

func (fn resolveFunc) Resolve(ref imageapi.DockerImageReference) (*imageapi.Image, error) {
	return fn(ref)
}

func TestDefaultPolicy(t *testing.T) {
	input, err := os.Open("api/v1/default-policy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := configlatest.ReadYAML(input)
	if err != nil {
		t.Fatal(err)
	}
	if obj == nil {
		t.Fatal(obj)
	}
	config, ok := obj.(*api.ImagePolicyConfig)
	if !ok {
		t.Fatal(config)
	}
	if errs := validation.Validate(config); len(errs) > 0 {
		t.Fatal(errs.ToAggregate())
	}

	plugin, err := newImagePolicyPlugin(nil, config)
	if err != nil {
		t.Fatal(err)
	}

	goodImage := &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Name: "sha256:good"}}
	badImage := &imageapi.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: "sha256:bad",
			Annotations: map[string]string{
				"images.openshift.io/deny-execution": "true",
			},
		},
	}

	client := testclient.NewSimpleFake(
		goodImage,
		badImage,

		// respond to image stream tag in this order:
		&unversioned.Status{
			Reason: unversioned.StatusReasonNotFound,
			Code:   404,
			Details: &unversioned.StatusDetails{
				Kind: "ImageStreamTag",
			},
		},
		&imageapi.ImageStreamTag{
			ObjectMeta: kapi.ObjectMeta{Name: "mysql:goodtag", Namespace: "repo"},
			Image:      *goodImage,
		},
		&imageapi.ImageStreamTag{
			ObjectMeta: kapi.ObjectMeta{Name: "mysql:badtag", Namespace: "repo"},
			Image:      *badImage,
		},
	)
	plugin.SetOpenshiftClient(client)
	plugin.SetDefaultRegistryFunc(func() (string, bool) {
		return "integrated.registry", true
	})
	if err := plugin.Validate(); err != nil {
		t.Fatal(err)
	}

	originalNowFn := now
	defer (func() { now = originalNowFn })()
	now = func() time.Time { return time.Unix(1, 0) }

	// should resolve the non-integrated image and allow it
	attrs := admission.NewAttributesRecord(
		&kapi.Pod{Spec: kapi.PodSpec{Containers: []kapi.Container{{Image: "index.docker.io/mysql@sha256:good"}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err != nil {
		t.Fatal(err)
	}

	// should resolve the integrated image by digest and allow it
	attrs = admission.NewAttributesRecord(
		&kapi.Pod{Spec: kapi.PodSpec{Containers: []kapi.Container{{Image: "integrated.registry/repo/mysql@sha256:good"}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err != nil {
		t.Fatal(err)
	}

	// should attempt resolve the integrated image by tag and fail because tag not found
	attrs = admission.NewAttributesRecord(
		&kapi.Pod{Spec: kapi.PodSpec{Containers: []kapi.Container{{Image: "integrated.registry/repo/mysql:missingtag"}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err != nil {
		t.Fatal(err)
	}

	// should attempt resolve the integrated image by tag and allow it
	attrs = admission.NewAttributesRecord(
		&kapi.Pod{Spec: kapi.PodSpec{Containers: []kapi.Container{{Image: "integrated.registry/repo/mysql:goodtag"}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err != nil {
		t.Fatal(err)
	}

	// should attempt resolve the integrated image by tag and forbid it
	attrs = admission.NewAttributesRecord(
		&kapi.Pod{Spec: kapi.PodSpec{Containers: []kapi.Container{{Image: "integrated.registry/repo/mysql:badtag"}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err == nil || !apierrs.IsInvalid(err) {
		t.Fatal(err)
	}

	// should reject the non-integrated image due to the annotation
	attrs = admission.NewAttributesRecord(
		&kapi.Pod{Spec: kapi.PodSpec{Containers: []kapi.Container{{Image: "index.docker.io/mysql@sha256:bad"}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err == nil || !apierrs.IsInvalid(err) {
		t.Fatal(err)
	}

	// should hit the cache on the previously good image and continue to allow it (the copy in cache was previously safe)
	goodImage.Annotations = map[string]string{"images.openshift.io/deny-execution": "true"}
	attrs = admission.NewAttributesRecord(
		&kapi.Pod{Spec: kapi.PodSpec{Containers: []kapi.Container{{Image: "index.docker.io/mysql@sha256:good"}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err != nil {
		t.Fatal(err)
	}

	// moving 2 minutes in the future should bypass the cache and deny the image
	now = func() time.Time { return time.Unix(1, 0).Add(2 * time.Minute) }
	attrs = admission.NewAttributesRecord(
		&kapi.Pod{Spec: kapi.PodSpec{Containers: []kapi.Container{{Image: "index.docker.io/mysql@sha256:good"}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err == nil || !apierrs.IsInvalid(err) {
		t.Fatal(err)
	}
}

func TestAdmissionWithoutPodSpec(t *testing.T) {
	p, err := newImagePolicyPlugin(nil, &api.ImagePolicyConfig{
		ConsumptionRules: []api.ImageConsumptionPolicyRule{
			{ImageCondition: api.ImageCondition{OnResources: []unversioned.GroupResource{{Resource: "nodes"}}}, Add: []api.ConsumeResourceEffect{{Name: "licensed.software", Quantity: "1"}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	attrs := admission.NewAttributesRecord(
		&kapi.Node{},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Node"},
		"", "node1", unversioned.GroupVersionResource{Version: "v1", Resource: "nodes"},
		"", admission.Create, nil,
	)
	if err := p.Admit(attrs); !apierrs.IsForbidden(err) || !strings.Contains(err.Error(), "because they do not have a pod spec") {
		t.Fatal(err)
	}
}

func TestAdmission(t *testing.T) {
	onResources := []unversioned.GroupResource{{Resource: "pods"}}
	p, err := newImagePolicyPlugin(nil, &api.ImagePolicyConfig{
		ExecutionRules: []api.ImageExecutionPolicyRule{
			{ImageCondition: api.ImageCondition{OnResources: onResources}, Resolve: true},
			{ImageCondition: api.ImageCondition{
				Reject:          true,
				OnResources:     onResources,
				MatchRegistries: []string{"index.docker.io"},
			}},
		},
		ConsumptionRules: []api.ImageConsumptionPolicyRule{
			{ImageCondition: api.ImageCondition{OnResources: onResources}, Add: []api.ConsumeResourceEffect{{Name: "licensed.software", Quantity: "1"}}},
		},
		PlacementRules: []api.ImagePlacementPolicyRule{
			{ImageCondition: api.ImageCondition{OnResources: onResources}, Constrain: []api.ConstrainPodNodeSelectorEffect{{Add: map[string]string{"selector1": "value1"}}}},
		},
	})
	resolveCalled := 0
	p.resolver = resolveFunc(func(ref imageapi.DockerImageReference) (*imageapi.Image, error) {
		resolveCalled++
		switch ref.Registry {
		case "myregistry.com":
			return &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Name: "1"}}, nil
		case "index.docker.io":
			return &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Name: "2"}}, nil
		}
		t.Fatalf("unexpected call to resolve image: %v", ref)
		return nil, nil
	})

	if err != nil {
		t.Fatal(err)
	}
	if !p.Handles(admission.Create) {
		t.Fatal("expected to handle create")
	}
	failingAttrs := admission.NewAttributesRecord(
		&kapi.Pod{
			Spec: kapi.PodSpec{
				Containers: []kapi.Container{
					{Image: "index.docker.io/mysql:latest"},
				},
			},
		},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", admission.Create, nil,
	)
	if err := p.Admit(failingAttrs); err == nil {
		t.Fatal(err)
	}

	attrs := admission.NewAttributesRecord(
		&kapi.Pod{
			Spec: kapi.PodSpec{
				Containers: []kapi.Container{
					{Image: "myregistry.com/mysql/mysql:latest"},
				},
			},
		},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", admission.Create, nil,
	)
	if err := p.Admit(attrs); err != nil {
		t.Logf("object: %#v", attrs.GetObject())
		t.Fatal(err)
	}
}
