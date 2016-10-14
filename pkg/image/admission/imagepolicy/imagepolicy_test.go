package imagepolicy

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	apierrs "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kcache "k8s.io/kubernetes/pkg/client/cache"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/diff"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client/testclient"
	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api"
	_ "github.com/openshift/origin/pkg/image/admission/imagepolicy/api/install"
	"github.com/openshift/origin/pkg/image/admission/imagepolicy/api/validation"
	"github.com/openshift/origin/pkg/image/admission/imagepolicy/rules"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/project/cache"
)

const (
	goodSHA = "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"
	badSHA  = "sha256:503c75e8121369581e5e5abe57b5a3f12db859052b217a8ea16eb86f4b5561a1"
)

type resolveFunc func(ref *kapi.ObjectReference, defaultNamespace string) (*rules.ImagePolicyAttributes, error)

func (fn resolveFunc) ResolveObjectReference(ref *kapi.ObjectReference, defaultNamespace string) (*rules.ImagePolicyAttributes, error) {
	return fn(ref, defaultNamespace)
}

func setDefaultCache(p *imagePolicyPlugin) kcache.Indexer {
	kclient := ktestclient.NewSimpleFake()
	store := cache.NewCacheStore(kcache.MetaNamespaceKeyFunc)
	p.SetProjectCache(cache.NewFake(kclient.Namespaces(), store, ""))
	return store
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

	goodImage := &imageapi.Image{
		ObjectMeta:           kapi.ObjectMeta{Name: goodSHA},
		DockerImageReference: "integrated.registry/goodns/goodimage:good",
	}
	badImage := &imageapi.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name: badSHA,
			Annotations: map[string]string{
				"images.openshift.io/deny-execution": "true",
			},
		},
		DockerImageReference: "integrated.registry/badns/badimage:bad",
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

	store := setDefaultCache(plugin)
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

	// should allow a non-integrated image
	attrs := admission.NewAttributesRecord(
		&kapi.Pod{Spec: kapi.PodSpec{Containers: []kapi.Container{{Image: "index.docker.io/mysql:latest"}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err != nil {
		t.Fatal(err)
	}

	// should resolve the non-integrated image and allow it
	attrs = admission.NewAttributesRecord(
		&kapi.Pod{Spec: kapi.PodSpec{Containers: []kapi.Container{{Image: "index.docker.io/mysql@" + goodSHA}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err != nil {
		t.Fatal(err)
	}

	// should resolve the integrated image by digest and allow it
	attrs = admission.NewAttributesRecord(
		&kapi.Pod{Spec: kapi.PodSpec{Containers: []kapi.Container{{Image: "integrated.registry/repo/mysql@" + goodSHA}}}},
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
	t.Logf("%#v", plugin.accepter)
	if err := plugin.Admit(attrs); err == nil || !apierrs.IsInvalid(err) {
		t.Fatal(err)
	}

	// should reject the non-integrated image due to the annotation
	attrs = admission.NewAttributesRecord(
		&kapi.Pod{Spec: kapi.PodSpec{Containers: []kapi.Container{{Image: "index.docker.io/mysql@" + badSHA}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err == nil || !apierrs.IsInvalid(err) {
		t.Fatal(err)
	}

	// should reject the non-integrated image due to the annotation on an init container
	attrs = admission.NewAttributesRecord(
		&kapi.Pod{Spec: kapi.PodSpec{InitContainers: []kapi.Container{{Image: "index.docker.io/mysql@" + badSHA}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err == nil || !apierrs.IsInvalid(err) {
		t.Fatal(err)
	}

	// should reject the non-integrated image due to the annotation for a build
	attrs = admission.NewAttributesRecord(
		&buildapi.Build{Spec: buildapi.BuildSpec{CommonSpec: buildapi.CommonSpec{Source: buildapi.BuildSource{Images: []buildapi.ImageSource{
			{From: kapi.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/mysql@" + badSHA}},
		}}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Build"},
		"default", "build1", unversioned.GroupVersionResource{Version: "v1", Resource: "builds"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err == nil || !apierrs.IsInvalid(err) {
		t.Fatal(err)
	}
	attrs = admission.NewAttributesRecord(
		&buildapi.Build{Spec: buildapi.BuildSpec{CommonSpec: buildapi.CommonSpec{Strategy: buildapi.BuildStrategy{DockerStrategy: &buildapi.DockerBuildStrategy{
			From: &kapi.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/mysql@" + badSHA},
		}}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Build"},
		"default", "build1", unversioned.GroupVersionResource{Version: "v1", Resource: "builds"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err == nil || !apierrs.IsInvalid(err) {
		t.Fatal(err)
	}
	attrs = admission.NewAttributesRecord(
		&buildapi.Build{Spec: buildapi.BuildSpec{CommonSpec: buildapi.CommonSpec{Strategy: buildapi.BuildStrategy{SourceStrategy: &buildapi.SourceBuildStrategy{
			From: kapi.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/mysql@" + badSHA},
		}}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Build"},
		"default", "build1", unversioned.GroupVersionResource{Version: "v1", Resource: "builds"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err == nil || !apierrs.IsInvalid(err) {
		t.Fatal(err)
	}
	attrs = admission.NewAttributesRecord(
		&buildapi.Build{Spec: buildapi.BuildSpec{CommonSpec: buildapi.CommonSpec{Strategy: buildapi.BuildStrategy{CustomStrategy: &buildapi.CustomBuildStrategy{
			From: kapi.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/mysql@" + badSHA},
		}}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Build"},
		"default", "build1", unversioned.GroupVersionResource{Version: "v1", Resource: "builds"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err == nil || !apierrs.IsInvalid(err) {
		t.Fatal(err)
	}

	// should allow the non-integrated image due to the annotation for a build config because it's not in the list, even though it has
	// a valid spec
	attrs = admission.NewAttributesRecord(
		&buildapi.BuildConfig{Spec: buildapi.BuildConfigSpec{CommonSpec: buildapi.CommonSpec{Source: buildapi.BuildSource{Images: []buildapi.ImageSource{
			{From: kapi.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/mysql@" + badSHA}},
		}}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "BuildConfig"},
		"default", "build1", unversioned.GroupVersionResource{Version: "v1", Resource: "buildconfigs"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err != nil {
		t.Fatal(err)
	}

	// should hit the cache on the previously good image and continue to allow it (the copy in cache was previously safe)
	goodImage.Annotations = map[string]string{"images.openshift.io/deny-execution": "true"}
	attrs = admission.NewAttributesRecord(
		&kapi.Pod{Spec: kapi.PodSpec{Containers: []kapi.Container{{Image: "index.docker.io/mysql@" + goodSHA}}}},
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
		&kapi.Pod{Spec: kapi.PodSpec{Containers: []kapi.Container{{Image: "index.docker.io/mysql@" + goodSHA}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err == nil || !apierrs.IsInvalid(err) {
		t.Fatal(err)
	}

	// setting a namespace annotation should allow the rule to be skipped immediately
	store.Add(&kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "",
			Name:      "default",
			Annotations: map[string]string{
				api.IgnorePolicyRulesAnnotation: "execution-denied",
			},
		},
	})
	attrs = admission.NewAttributesRecord(
		&kapi.Pod{Spec: kapi.PodSpec{Containers: []kapi.Container{{Image: "index.docker.io/mysql@" + goodSHA}}}},
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", admission.Create, nil,
	)
	if err := plugin.Admit(attrs); err != nil {
		t.Fatal(err)
	}
}

func TestAdmissionWithoutPodSpec(t *testing.T) {
	onResources := []unversioned.GroupResource{{Resource: "nodes"}}
	p, err := newImagePolicyPlugin(nil, &api.ImagePolicyConfig{
		ExecutionRules: []api.ImageExecutionPolicyRule{
			{ImageCondition: api.ImageCondition{OnResources: onResources}},
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
	if err := p.Admit(attrs); !apierrs.IsForbidden(err) || !strings.Contains(err.Error(), "No list of images available for this object") {
		t.Fatal(err)
	}
}

func TestAdmissionResolution(t *testing.T) {
	onResources := []unversioned.GroupResource{{Resource: "pods"}}
	p, err := newImagePolicyPlugin(nil, &api.ImagePolicyConfig{
		ResolveImages: api.AttemptRewrite,
		ExecutionRules: []api.ImageExecutionPolicyRule{
			{ImageCondition: api.ImageCondition{OnResources: onResources}},
			{Reject: true, ImageCondition: api.ImageCondition{
				OnResources:     onResources,
				MatchRegistries: []string{"index.docker.io"},
			}},
		},
	})
	setDefaultCache(p)

	resolveCalled := 0
	p.resolver = resolveFunc(func(ref *kapi.ObjectReference, defaultNamespace string) (*rules.ImagePolicyAttributes, error) {
		resolveCalled++
		switch ref.Name {
		case "index.docker.io/mysql:latest":
			return &rules.ImagePolicyAttributes{
				Name:  imageapi.DockerImageReference{Registry: "index.docker.io", Name: "mysql", Tag: "latest"},
				Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Name: "1"}},
			}, nil
		case "myregistry.com/mysql/mysql:latest":
			return &rules.ImagePolicyAttributes{
				Name:  imageapi.DockerImageReference{Registry: "myregistry.com", Namespace: "mysql", Name: "mysql", ID: "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"},
				Image: &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Name: "2"}},
			}, nil
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

	pod := &kapi.Pod{
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{Image: "myregistry.com/mysql/mysql:latest"},
				{Image: "myregistry.com/mysql/mysql:latest"},
			},
		},
	}
	attrs := admission.NewAttributesRecord(
		pod,
		nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
		"", admission.Create, nil,
	)
	if err := p.Admit(attrs); err != nil {
		t.Logf("object: %#v", attrs.GetObject())
		t.Fatal(err)
	}
	if pod.Spec.Containers[0].Image != "myregistry.com/mysql/mysql@sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4" ||
		pod.Spec.Containers[1].Image != "myregistry.com/mysql/mysql@sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4" {
		t.Errorf("unexpected image: %#v", pod)
	}
}

func TestAdmissionResolveImages(t *testing.T) {
	image1 := &imageapi.Image{
		ObjectMeta:           kapi.ObjectMeta{Name: "sha256:0000000000000000000000000000000000000000000000000000000000000001"},
		DockerImageReference: "integrated.registry/image1/image1:latest",
	}

	testCases := []struct {
		client *testclient.Fake
		attrs  admission.Attributes
		admit  bool
		expect runtime.Object
	}{
		// fails resolution
		{
			client: testclient.NewSimpleFake(),
			attrs: admission.NewAttributesRecord(
				&kapi.Pod{
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{Image: "integrated.registry/test/mysql@" + goodSHA},
						},
						InitContainers: []kapi.Container{
							{Image: "myregistry.com/mysql/mysql:latest"},
						},
					},
				}, nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
				"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
				"", admission.Create, nil,
			),
		},
		// resolves images in the integrated registry without altering their ref (avoids looking up the tag)
		{
			client: testclient.NewSimpleFake(
				image1,
			),
			attrs: admission.NewAttributesRecord(
				&kapi.Pod{
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{Image: "integrated.registry/test/mysql@sha256:0000000000000000000000000000000000000000000000000000000000000001"},
						},
					},
				}, nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
				"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
				"", admission.Create, nil,
			),
			admit: true,
			expect: &kapi.Pod{
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{Image: "integrated.registry/test/mysql@sha256:0000000000000000000000000000000000000000000000000000000000000001"},
					},
				},
			},
		},
		// resolves images in the integrated registry without altering their ref (avoids looking up the tag)
		{
			client: testclient.NewSimpleFake(
				image1,
			),
			attrs: admission.NewAttributesRecord(
				&kapi.Pod{
					Spec: kapi.PodSpec{
						InitContainers: []kapi.Container{
							{Image: "integrated.registry/test/mysql@sha256:0000000000000000000000000000000000000000000000000000000000000001"},
						},
					},
				}, nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Pod"},
				"default", "pod1", unversioned.GroupVersionResource{Version: "v1", Resource: "pods"},
				"", admission.Create, nil,
			),
			admit: true,
			expect: &kapi.Pod{
				Spec: kapi.PodSpec{
					InitContainers: []kapi.Container{
						{Image: "integrated.registry/test/mysql@sha256:0000000000000000000000000000000000000000000000000000000000000001"},
					},
				},
			},
		},
		// resolves images in the integrated registry on builds without altering their ref (avoids looking up the tag)
		{
			client: testclient.NewSimpleFake(
				image1,
			),
			attrs: admission.NewAttributesRecord(
				&buildapi.Build{
					Spec: buildapi.BuildSpec{
						CommonSpec: buildapi.CommonSpec{
							Strategy: buildapi.BuildStrategy{
								SourceStrategy: &buildapi.SourceBuildStrategy{
									From: kapi.ObjectReference{Kind: "DockerImage", Name: "integrated.registry/test/mysql@sha256:0000000000000000000000000000000000000000000000000000000000000001"},
								},
							},
						},
					},
				}, nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Build"},
				"default", "build1", unversioned.GroupVersionResource{Version: "v1", Resource: "builds"},
				"", admission.Create, nil,
			),
			admit: true,
			expect: &buildapi.Build{
				Spec: buildapi.BuildSpec{
					CommonSpec: buildapi.CommonSpec{
						Strategy: buildapi.BuildStrategy{
							SourceStrategy: &buildapi.SourceBuildStrategy{
								From: kapi.ObjectReference{Kind: "DockerImage", Name: "integrated.registry/test/mysql@sha256:0000000000000000000000000000000000000000000000000000000000000001"},
							},
						},
					},
				},
			},
		},
		// resolves builds with image stream tags, uses the image DockerImageReference with SHA set.
		{
			client: testclient.NewSimpleFake(
				&imageapi.ImageStreamTag{
					ObjectMeta: kapi.ObjectMeta{Name: "test:other", Namespace: "default"},
					Image:      *image1,
				},
			),
			attrs: admission.NewAttributesRecord(
				&buildapi.Build{
					Spec: buildapi.BuildSpec{
						CommonSpec: buildapi.CommonSpec{
							Strategy: buildapi.BuildStrategy{
								CustomStrategy: &buildapi.CustomBuildStrategy{
									From: kapi.ObjectReference{Kind: "ImageStreamTag", Name: "test:other"},
								},
							},
						},
					},
				}, nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Build"},
				"default", "build1", unversioned.GroupVersionResource{Version: "v1", Resource: "builds"},
				"", admission.Create, nil,
			),
			admit: true,
			expect: &buildapi.Build{
				Spec: buildapi.BuildSpec{
					CommonSpec: buildapi.CommonSpec{
						Strategy: buildapi.BuildStrategy{
							CustomStrategy: &buildapi.CustomBuildStrategy{
								From: kapi.ObjectReference{Kind: "DockerImage", Name: "integrated.registry/image1/image1@sha256:0000000000000000000000000000000000000000000000000000000000000001"},
							},
						},
					},
				},
			},
		},
		// resolves builds with image stream images
		{
			client: testclient.NewSimpleFake(
				&imageapi.ImageStreamImage{
					ObjectMeta: kapi.ObjectMeta{Name: "test@sha256:0000000000000000000000000000000000000000000000000000000000000001", Namespace: "default"},
					Image:      *image1,
				},
			),
			attrs: admission.NewAttributesRecord(
				&buildapi.Build{
					Spec: buildapi.BuildSpec{
						CommonSpec: buildapi.CommonSpec{
							Strategy: buildapi.BuildStrategy{
								DockerStrategy: &buildapi.DockerBuildStrategy{
									From: &kapi.ObjectReference{Kind: "ImageStreamImage", Name: "test@sha256:0000000000000000000000000000000000000000000000000000000000000001"},
								},
							},
						},
					},
				}, nil, unversioned.GroupVersionKind{Version: "v1", Kind: "Build"},
				"default", "build1", unversioned.GroupVersionResource{Version: "v1", Resource: "builds"},
				"", admission.Create, nil,
			),
			admit: true,
			expect: &buildapi.Build{
				Spec: buildapi.BuildSpec{
					CommonSpec: buildapi.CommonSpec{
						Strategy: buildapi.BuildStrategy{
							DockerStrategy: &buildapi.DockerBuildStrategy{
								From: &kapi.ObjectReference{Kind: "DockerImage", Name: "integrated.registry/image1/image1:latest"},
							},
						},
					},
				},
			},
		},
	}
	for i, test := range testCases {
		onResources := []unversioned.GroupResource{{Resource: "builds"}, {Resource: "pods"}}
		p, err := newImagePolicyPlugin(nil, &api.ImagePolicyConfig{
			ResolveImages: api.RequiredRewrite,
			ExecutionRules: []api.ImageExecutionPolicyRule{
				{ImageCondition: api.ImageCondition{OnResources: onResources}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		setDefaultCache(p)
		p.SetOpenshiftClient(test.client)
		p.SetDefaultRegistryFunc(func() (string, bool) {
			return "integrated.registry", true
		})
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}

		if err := p.Admit(test.attrs); err != nil {
			if test.admit {
				t.Errorf("%d: should admit: %v", i, err)
			}
			continue
		}
		if !test.admit {
			t.Errorf("%d: should not admit", i)
			continue
		}

		if !reflect.DeepEqual(test.expect, test.attrs.GetObject()) {
			t.Errorf("%d: unequal: %s", i, diff.ObjectReflectDiff(test.expect, test.attrs.GetObject()))
		}
	}
}
