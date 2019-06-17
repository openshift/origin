package imagepolicy

import (
	"bytes"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/diff"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/admission"
	corev1listers "k8s.io/client-go/listers/core/v1"
	clientgotesting "k8s.io/client-go/testing"
	kcache "k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/openshift/api/image"
	imagev1 "github.com/openshift/api/image/v1"
	imagev1fakeclient "github.com/openshift/client-go/image/clientset/versioned/fake"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/imagepolicy"
	imagepolicyapi "github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/imagepolicy/apis/imagepolicy/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/imagepolicy/apis/imagepolicy/validation"
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/imagepolicy/rules"
	"github.com/openshift/origin/pkg/image/apiserver/admission/imagepolicy/originimagereferencemutators"
)

const (
	goodSHA = "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"
	badSHA  = "sha256:503c75e8121369581e5e5abe57b5a3f12db859052b217a8ea16eb86f4b5561a1"
)

var (
	buildGroupVersionResource = schema.GroupVersionResource{Group: "build.openshift.io", Version: "v1", Resource: "builds"}
	buildGroupVersionKind     = schema.GroupVersionKind{Group: "build.openshift.io", Version: "v1", Kind: "Build"}

	buildConfigGroupVersionResource = schema.GroupVersionResource{Group: "build.openshift.io", Version: "v1", Resource: "buildconfigs"}
	buildConfigGroupVersionKind     = schema.GroupVersionKind{Group: "build.openshift.io", Version: "v1", Kind: "BuildConfig"}
)

type resolveFunc func(ref *kapi.ObjectReference, defaultNamespace string, forceLocalResolve bool) (*rules.ImagePolicyAttributes, error)

func (fn resolveFunc) ResolveObjectReference(ref *kapi.ObjectReference, defaultNamespace string, forceLocalResolve bool) (*rules.ImagePolicyAttributes, error) {
	return fn(ref, defaultNamespace, forceLocalResolve)
}

func setDefaultCache(p *imagepolicy.ImagePolicyPlugin) kcache.Indexer {
	indexer := kcache.NewIndexer(kcache.MetaNamespaceKeyFunc, kcache.Indexers{})
	p.NsLister = corev1listers.NewNamespaceLister(indexer)
	return indexer
}

func TestDefaultPolicy(t *testing.T) {
	input, err := os.Open("../../../../cmd/openshift-kube-apiserver/admission/imagepolicy/apis/imagepolicy/v1/default-policy.yaml")
	if err != nil {
		t.Fatal(err)
	}
	config := &imagepolicyapi.ImagePolicyConfig{}
	configContent, err := ioutil.ReadAll(input)
	if err != nil {
		t.Fatal(err)
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(imagepolicyapi.Install(scheme))
	codecs := serializer.NewCodecFactory(scheme)
	err = runtime.DecodeInto(codecs.UniversalDecoder(imagepolicyapi.GroupVersion), configContent, config)
	if err != nil {
		t.Fatal(err)
	}
	imagepolicyapi.SetDefaults_ImagePolicyConfig(config)

	if errs := validation.Validate(config); len(errs) > 0 {
		t.Fatal(errs.ToAggregate())
	}

	plugin, err := imagepolicy.NewImagePolicyPlugin(config)
	if err != nil {
		t.Fatal(err)
	}

	goodImage := &imagev1.Image{
		ObjectMeta:           metav1.ObjectMeta{Name: goodSHA},
		DockerImageReference: "integrated.registry/goodns/goodimage:good",
	}
	badImage := &imagev1.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: badSHA,
			Annotations: map[string]string{
				"images.openshift.io/deny-execution": "true",
			},
		},
		DockerImageReference: "integrated.registry/badns/badimage:bad",
	}

	goodTag := &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{Name: "mysql:goodtag", Namespace: "repo"},
		Image:      *goodImage,
	}
	badTag := &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{Name: "mysql:badtag", Namespace: "repo"},
		Image:      *badImage,
	}

	client := &imagev1fakeclient.Clientset{}
	client.AddReactor("get", "images", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(clientgotesting.GetAction).GetName()
		switch name {
		case goodImage.Name:
			return true, goodImage, nil
		case badImage.Name:
			return true, badImage, nil
		default:
			return true, nil, kerrors.NewNotFound(image.Resource("images"), name)
		}
	})
	client.AddReactor("get", "imagestreamtags", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		name := action.(clientgotesting.GetAction).GetName()
		switch name {
		case goodTag.Name:
			return true, goodTag, nil
		case badTag.Name:
			return true, badTag, nil
		default:
			return true, nil, kerrors.NewNotFound(image.Resource("imagestreamtags"), name)
		}
	})

	setDefaultCache(plugin)
	plugin.Client = client
	plugin.SetDefaultRegistryFunc(func() (string, bool) {
		return "integrated.registry", true
	})
	plugin.SetImageMutators(originimagereferencemutators.OriginImageMutators{})
	if err := plugin.ValidateInitialization(); err != nil {
		t.Fatal(err)
	}

	// should reject the non-integrated image due to the annotation for a build
	attrs := admission.NewAttributesRecord(
		&buildapi.Build{Spec: buildapi.BuildSpec{CommonSpec: buildapi.CommonSpec{Source: buildapi.BuildSource{Images: []buildapi.ImageSource{
			{From: kapi.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/mysql@" + badSHA}},
		}}}}},
		nil, buildGroupVersionKind,
		"default", "build1", buildGroupVersionResource,
		"", admission.Create, false, nil,
	)
	if err := plugin.Admit(attrs, nil); err == nil || !kerrors.IsInvalid(err) {
		t.Fatal(err)
	}
	if err := plugin.Validate(attrs, nil); err == nil || !kerrors.IsInvalid(err) {
		t.Fatal(err)
	}
	attrs = admission.NewAttributesRecord(
		&buildapi.Build{Spec: buildapi.BuildSpec{CommonSpec: buildapi.CommonSpec{Strategy: buildapi.BuildStrategy{DockerStrategy: &buildapi.DockerBuildStrategy{
			From: &kapi.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/mysql@" + badSHA},
		}}}}},
		nil, buildGroupVersionKind,
		"default", "build1", buildGroupVersionResource,
		"", admission.Create, false, nil,
	)
	if err := plugin.Admit(attrs, nil); err == nil || !kerrors.IsInvalid(err) {
		t.Fatal(err)
	}
	if err := plugin.Validate(attrs, nil); err == nil || !kerrors.IsInvalid(err) {
		t.Fatal(err)
	}
	attrs = admission.NewAttributesRecord(
		&buildapi.Build{Spec: buildapi.BuildSpec{CommonSpec: buildapi.CommonSpec{Strategy: buildapi.BuildStrategy{SourceStrategy: &buildapi.SourceBuildStrategy{
			From: kapi.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/mysql@" + badSHA},
		}}}}},
		nil, buildGroupVersionKind,
		"default", "build1", buildGroupVersionResource,
		"", admission.Create, false, nil,
	)
	if err := plugin.Admit(attrs, nil); err == nil || !kerrors.IsInvalid(err) {
		t.Fatal(err)
	}
	if err := plugin.Validate(attrs, nil); err == nil || !kerrors.IsInvalid(err) {
		t.Fatal(err)
	}
	attrs = admission.NewAttributesRecord(
		&buildapi.Build{Spec: buildapi.BuildSpec{CommonSpec: buildapi.CommonSpec{Strategy: buildapi.BuildStrategy{CustomStrategy: &buildapi.CustomBuildStrategy{
			From: kapi.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/mysql@" + badSHA},
		}}}}},
		nil, buildGroupVersionKind,
		"default", "build1", buildGroupVersionResource,
		"", admission.Create, false, nil,
	)
	if err := plugin.Admit(attrs, nil); err == nil || !kerrors.IsInvalid(err) {
		t.Fatal(err)
	}
	if err := plugin.Validate(attrs, nil); err == nil || !kerrors.IsInvalid(err) {
		t.Fatal(err)
	}

	// should allow the non-integrated image due to the annotation for a build config because it's not in the list, even though it has
	// a valid spec
	attrs = admission.NewAttributesRecord(
		&buildapi.BuildConfig{Spec: buildapi.BuildConfigSpec{CommonSpec: buildapi.CommonSpec{Source: buildapi.BuildSource{Images: []buildapi.ImageSource{
			{From: kapi.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/mysql@" + badSHA}},
		}}}}},
		nil, buildConfigGroupVersionKind,
		"default", "build1", buildConfigGroupVersionResource,
		"", admission.Create, false, nil,
	)
	if err := plugin.Admit(attrs, nil); err != nil {
		t.Fatal(err)
	}
	if err := plugin.Validate(attrs, nil); err != nil {
		t.Fatal(err)
	}
}

func TestAdmissionResolveImages(t *testing.T) {
	image1 := &imagev1.Image{
		ObjectMeta:           metav1.ObjectMeta{Name: "sha256:0000000000000000000000000000000000000000000000000000000000000001"},
		DockerImageReference: "integrated.registry/image1/image1@sha256:0000000000000000000000000000000000000000000000000000000000000001",
	}

	defaultPolicyConfig := &imagepolicyapi.ImagePolicyConfig{}
	configContent, err := ioutil.ReadAll(bytes.NewBufferString(`{"kind":"ImagePolicyConfig","apiVersion":"image.openshift.io/v1"}`))
	if err != nil {
		t.Fatal(err)
	}
	scheme := runtime.NewScheme()
	utilruntime.Must(imagepolicyapi.Install(scheme))
	codecs := serializer.NewCodecFactory(scheme)
	err = runtime.DecodeInto(codecs.UniversalDecoder(imagepolicyapi.GroupVersion), configContent, defaultPolicyConfig)
	if err != nil {
		t.Fatal(err)
	}
	imagepolicyapi.SetDefaults_ImagePolicyConfig(defaultPolicyConfig)

	testCases := []struct {
		name   string
		client *imagev1fakeclient.Clientset
		policy imagepolicyapi.ImageResolutionType
		config *imagepolicyapi.ImagePolicyConfig
		attrs  admission.Attributes
		admit  bool
		expect runtime.Object
	}{
		{
			name:   "resolves images in the integrated registry on builds without altering their ref (avoids looking up the tag)",
			policy: imagepolicyapi.RequiredRewrite,
			client: imagev1fakeclient.NewSimpleClientset(
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
				}, nil, buildGroupVersionKind,
				"default", "build1", buildGroupVersionResource,
				"", admission.Create, false, nil,
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
		{
			name:   "resolves builds with image stream tags, uses the image DockerImageReference with SHA set",
			policy: imagepolicyapi.RequiredRewrite,
			client: imagev1fakeclient.NewSimpleClientset(
				&imagev1.ImageStreamTag{
					ObjectMeta: metav1.ObjectMeta{Name: "test:other", Namespace: "default"},
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
				}, nil, buildGroupVersionKind,
				"default", "build1", buildGroupVersionResource,
				"", admission.Create, false, nil,
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
		{
			name:   "does not resolve a build update because the reference didn't change",
			policy: imagepolicyapi.RequiredRewrite,
			client: imagev1fakeclient.NewSimpleClientset(
				&imagev1.ImageStreamTag{
					ObjectMeta: metav1.ObjectMeta{Name: "test:other", Namespace: "default"},
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
				},
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
				},
				buildGroupVersionKind,
				"default", "build1", buildGroupVersionResource,
				"", admission.Create, false, nil,
			),
			admit: true,
			expect: &buildapi.Build{
				Spec: buildapi.BuildSpec{
					CommonSpec: buildapi.CommonSpec{
						Strategy: buildapi.BuildStrategy{
							CustomStrategy: &buildapi.CustomBuildStrategy{
								From: kapi.ObjectReference{Kind: "ImageStreamTag", Name: "test:other"},
							},
						},
					},
				},
			},
		},
		{
			name:   "resolves images in the integrated registry on builds without altering their ref (avoids looking up the tag)",
			policy: imagepolicyapi.RequiredRewrite,
			client: imagev1fakeclient.NewSimpleClientset(
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
				}, nil, buildGroupVersionKind,
				"default", "build1", buildGroupVersionResource,
				"", admission.Create, false, nil,
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
		{
			name: "does not rewrite the config because build has DoNotAttempt by default, which overrides global policy",
			config: &imagepolicyapi.ImagePolicyConfig{
				ResolveImages: imagepolicyapi.RequiredRewrite,
				ResolutionRules: []imagepolicyapi.ImageResolutionPolicyRule{
					{TargetResource: metav1.GroupResource{Group: "build.openshift.io", Resource: "builds"}},
				},
			},
			client: imagev1fakeclient.NewSimpleClientset(
				&imagev1.ImageStreamTag{
					ObjectMeta: metav1.ObjectMeta{Name: "test:other", Namespace: "default"},
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
				}, nil, buildGroupVersionKind,
				"default", "build1", buildGroupVersionResource,
				"", admission.Create, false, nil,
			),
			admit: true,
			expect: &buildapi.Build{
				Spec: buildapi.BuildSpec{
					CommonSpec: buildapi.CommonSpec{
						Strategy: buildapi.BuildStrategy{
							CustomStrategy: &buildapi.CustomBuildStrategy{
								From: kapi.ObjectReference{Kind: "ImageStreamTag", Name: "test:other"},
							},
						},
					},
				},
			},
		},
		{
			name: "does not rewrite the config because the default policy uses attempt by default",
			config: &imagepolicyapi.ImagePolicyConfig{
				ResolveImages: imagepolicyapi.RequiredRewrite,
				ResolutionRules: []imagepolicyapi.ImageResolutionPolicyRule{
					{TargetResource: metav1.GroupResource{Group: "build.openshift.io", Resource: "builds"}, Policy: imagepolicyapi.Attempt},
				},
			},
			client: imagev1fakeclient.NewSimpleClientset(
				&imagev1.ImageStreamTag{
					ObjectMeta: metav1.ObjectMeta{Name: "test:other", Namespace: "default"},
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
				}, nil, buildGroupVersionKind,
				"default", "build1", buildGroupVersionResource,
				"", admission.Create, false, nil,
			),
			admit: true,
			expect: &buildapi.Build{
				Spec: buildapi.BuildSpec{
					CommonSpec: buildapi.CommonSpec{
						Strategy: buildapi.BuildStrategy{
							CustomStrategy: &buildapi.CustomBuildStrategy{
								From: kapi.ObjectReference{Kind: "ImageStreamTag", Name: "test:other"},
							},
						},
					},
				},
			},
		},
		{
			name: "rewrites the config because build has AttemptRewrite which overrides the global policy",
			config: &imagepolicyapi.ImagePolicyConfig{
				ResolveImages: imagepolicyapi.DoNotAttempt,
				ResolutionRules: []imagepolicyapi.ImageResolutionPolicyRule{
					{TargetResource: metav1.GroupResource{Group: "build.openshift.io", Resource: "builds"}, Policy: imagepolicyapi.AttemptRewrite},
				},
			},
			client: imagev1fakeclient.NewSimpleClientset(
				&imagev1.ImageStreamTag{
					ObjectMeta: metav1.ObjectMeta{Name: "test:other", Namespace: "default"},
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
				}, nil, buildGroupVersionKind,
				"default", "build1", buildGroupVersionResource,
				"", admission.Create, false, nil,
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
		{
			name:   "resolves builds.build.openshift.io with image stream tags, uses the image DockerImageReference with SHA set",
			policy: imagepolicyapi.RequiredRewrite,
			client: imagev1fakeclient.NewSimpleClientset(
				&imagev1.ImageStreamTag{
					ObjectMeta: metav1.ObjectMeta{Name: "test:other", Namespace: "default"},
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
				}, nil, buildGroupVersionKind,
				"default", "build1", buildGroupVersionResource,
				"", admission.Create, false, nil,
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
		{
			name:   "resolves builds with image stream images",
			policy: imagepolicyapi.RequiredRewrite,
			client: imagev1fakeclient.NewSimpleClientset(
				&imagev1.ImageStreamImage{
					ObjectMeta: metav1.ObjectMeta{Name: "test@sha256:0000000000000000000000000000000000000000000000000000000000000001", Namespace: "default"},
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
				}, nil, buildGroupVersionKind,
				"default", "build1", buildGroupVersionResource,
				"", admission.Create, false, nil,
			),
			admit: true,
			expect: &buildapi.Build{
				Spec: buildapi.BuildSpec{
					CommonSpec: buildapi.CommonSpec{
						Strategy: buildapi.BuildStrategy{
							DockerStrategy: &buildapi.DockerBuildStrategy{
								From: &kapi.ObjectReference{Kind: "DockerImage", Name: "integrated.registry/image1/image1@sha256:0000000000000000000000000000000000000000000000000000000000000001"},
							},
						},
					},
				},
			},
		},
		{
			name:   "resolves builds that have a local name to their image stream tags, uses the image DockerImageReference with SHA set",
			policy: imagepolicyapi.RequiredRewrite,
			client: imagev1fakeclient.NewSimpleClientset(
				&imagev1.ImageStreamTag{
					ObjectMeta:   metav1.ObjectMeta{Name: "test:other", Namespace: "default"},
					LookupPolicy: imagev1.ImageLookupPolicy{Local: true},
					Image:        *image1,
				},
			),
			attrs: admission.NewAttributesRecord(
				&buildapi.Build{
					Spec: buildapi.BuildSpec{
						CommonSpec: buildapi.CommonSpec{
							Strategy: buildapi.BuildStrategy{
								CustomStrategy: &buildapi.CustomBuildStrategy{
									From: kapi.ObjectReference{Kind: "DockerImage", Name: "test:other"},
								},
							},
						},
					},
				}, nil, buildGroupVersionKind,
				"default", "build1", buildGroupVersionResource,
				"", admission.Create, false, nil,
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
	}
	for i, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			onResources := []metav1.GroupResource{{Group: "build.openshift.io", Resource: "builds"}, {Resource: "pods"}}
			config := test.config
			if config == nil {
				// old style config
				config = &imagepolicyapi.ImagePolicyConfig{
					ResolveImages: test.policy,
					ResolutionRules: []imagepolicyapi.ImageResolutionPolicyRule{
						{LocalNames: true, TargetResource: metav1.GroupResource{Resource: "*"}, Policy: test.policy},
						{LocalNames: true, TargetResource: metav1.GroupResource{Group: "extensions", Resource: "*"}, Policy: test.policy},
					},
					ExecutionRules: []imagepolicyapi.ImageExecutionPolicyRule{
						{ImageCondition: imagepolicyapi.ImageCondition{OnResources: onResources}},
					},
				}
			}
			p, err := imagepolicy.NewImagePolicyPlugin(config)
			if err != nil {
				t.Fatal(err)
			}

			setDefaultCache(p)
			p.Client = test.client
			p.SetDefaultRegistryFunc(func() (string, bool) {
				return "integrated.registry", true
			})
			p.SetImageMutators(originimagereferencemutators.OriginImageMutators{})
			if err := p.ValidateInitialization(); err != nil {
				t.Fatal(err)
			}

			if err := p.Admit(test.attrs, nil); err != nil {
				if test.admit {
					t.Errorf("%d: should admit: %v", i, err)
				}
				return
			}
			if !test.admit {
				t.Errorf("%d: should not admit", i)
				return
			}
			if !reflect.DeepEqual(test.expect, test.attrs.GetObject()) {
				t.Errorf("%d: unequal: %s", i, diff.ObjectReflectDiff(test.expect, test.attrs.GetObject()))
			}

			if err := p.Validate(test.attrs, nil); err != nil {
				t.Errorf("%d: should validate: %v", i, err)
				return
			}
			if !reflect.DeepEqual(test.expect, test.attrs.GetObject()) {
				t.Errorf("%d: unequal: %s", i, diff.ObjectReflectDiff(test.expect, test.attrs.GetObject()))
			}
		})
	}
}
