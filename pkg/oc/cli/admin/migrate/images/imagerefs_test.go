package images

import (
	"testing"

	kappsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/oc/pkg/helpers/originpolymorphichelpers"
)

func TestImageReferenceMappingsMapReference(t *testing.T) {
	testCases := []struct {
		mappings ImageReferenceMappings
		results  map[string]string
	}{
		{
			mappings: ImageReferenceMappings{{FromRegistry: "docker.io", ToRegistry: "index.docker.io"}},
			results: map[string]string{
				"mysql":                "index.docker.io/mysql",
				"mysql:latest":         "index.docker.io/mysql:latest",
				"default/mysql:latest": "index.docker.io/default/mysql:latest",

				"mysql@sha256:b2f400f4a5e003b0543decf61a0a010939f3fba07bafa226f11ed7b5f1e81237": "index.docker.io/mysql@sha256:b2f400f4a5e003b0543decf61a0a010939f3fba07bafa226f11ed7b5f1e81237",

				"docker.io/mysql":                "index.docker.io/mysql",
				"docker.io/mysql:latest":         "index.docker.io/mysql:latest",
				"docker.io/default/mysql:latest": "index.docker.io/default/mysql:latest",

				"docker.io/mysql@sha256:b2f400f4a5e003b0543decf61a0a010939f3fba07bafa226f11ed7b5f1e81237": "index.docker.io/mysql@sha256:b2f400f4a5e003b0543decf61a0a010939f3fba07bafa226f11ed7b5f1e81237",
			},
		},
		{
			mappings: ImageReferenceMappings{{FromName: "test/other", ToRegistry: "another.registry"}},
			results: map[string]string{
				"test/other":                       "another.registry/test/other",
				"test/other:latest":                "another.registry/test/other:latest",
				"myregistry.com/test/other:latest": "another.registry/test/other:latest",

				"myregistry.com/b/test/other:latest": "myregistry.com/b/test/other:latest",
			},
		},
		{
			mappings: ImageReferenceMappings{{FromName: "test/other", ToName: "other/test"}},
			results: map[string]string{
				"test/other":                       "other/test",
				"test/other:latest":                "other/test:latest",
				"myregistry.com/test/other:latest": "myregistry.com/other/test:latest",

				"test/other/b:latest": "test/other/b:latest",
			},
		},
	}

	for i, test := range testCases {
		for in, out := range test.results {
			result := test.mappings.MapReference(in)
			if result != out {
				t.Errorf("%d: expect %s -> %s, got %q", i, in, out, result)
				continue
			}
		}
	}
}

func TestImageReferenceMappingsMapDockerAuthKey(t *testing.T) {
	testCases := []struct {
		mappings ImageReferenceMappings
		results  map[string]string
	}{
		{
			mappings: ImageReferenceMappings{{FromRegistry: "docker.io", ToRegistry: "index.docker.io"}},
			results: map[string]string{
				"docker.io":                   "index.docker.io",
				"index.docker.io":             "index.docker.io",
				"https://index.docker.io/v1/": "https://index.docker.io/v1/",
				"https://docker.io/v1/":       "index.docker.io",

				"other.docker.io":             "other.docker.io",
				"other.docker.io/names":       "other.docker.io/names",
				"other.docker.io:5000/names":  "other.docker.io:5000/names",
				"https://other.docker.io/v1/": "https://other.docker.io/v1/",
			},
		},
		{
			mappings: ImageReferenceMappings{{FromRegistry: "index.docker.io", ToRegistry: "another.registry"}},
			results: map[string]string{
				"index.docker.io":                  "another.registry",
				"index.docker.io/other":            "another.registry/other",
				"https://index.docker.io/v1/other": "another.registry/other",
				"https://index.docker.io/v1/":      "another.registry",
				"https://index.docker.io/":         "another.registry",
				"https://index.docker.io":          "another.registry",

				"docker.io":                   "docker.io",
				"https://docker.io/v1/":       "https://docker.io/v1/",
				"other.docker.io":             "other.docker.io",
				"other.docker.io/names":       "other.docker.io/names",
				"other.docker.io:5000/names":  "other.docker.io:5000/names",
				"https://other.docker.io/v1/": "https://other.docker.io/v1/",
			},
		},
		{
			mappings: ImageReferenceMappings{{FromRegistry: "index.docker.io", ToRegistry: "another.registry", ToName: "extra"}},
			results: map[string]string{
				"index.docker.io":                  "another.registry/extra",
				"index.docker.io/other":            "another.registry/extra",
				"https://index.docker.io/v1/other": "another.registry/extra",
				"https://index.docker.io/v1/":      "another.registry/extra",
				"https://index.docker.io/":         "another.registry/extra",

				"docker.io":                   "docker.io",
				"https://docker.io/v1/":       "https://docker.io/v1/",
				"other.docker.io":             "other.docker.io",
				"other.docker.io/names":       "other.docker.io/names",
				"other.docker.io:5000/names":  "other.docker.io:5000/names",
				"https://other.docker.io/v1/": "https://other.docker.io/v1/",
			},
		},
	}

	for i, test := range testCases {
		for in, out := range test.results {
			result := test.mappings.MapDockerAuthKey(in)
			if result != out {
				t.Errorf("%d: expect %s -> %s, got %q", i, in, out, result)
				continue
			}
		}
	}
}

func TestTransform(t *testing.T) {
	type variant struct {
		changed       bool
		nilReporter   bool
		err           bool
		obj, expected runtime.Object
	}
	testCases := []struct {
		mappings ImageReferenceMappings
		variants []variant
	}{
		{
			mappings: ImageReferenceMappings{{FromRegistry: "docker.io", ToRegistry: "index.docker.io"}},
			variants: []variant{
				{
					obj: &corev1.Pod{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Image: "docker.io/foo/bar"},
								{Image: "foo/bar"},
							},
						},
					},
					changed: true,
					expected: &corev1.Pod{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Image: "index.docker.io/foo/bar"},
								{Image: "index.docker.io/foo/bar"},
							},
						},
					},
				},
				{
					obj: &corev1.ReplicationController{
						Spec: corev1.ReplicationControllerSpec{
							Template: &corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{Image: "docker.io/foo/bar"},
										{Image: "foo/bar"},
									},
								},
							},
						},
					},
					changed: true,
					expected: &corev1.ReplicationController{
						Spec: corev1.ReplicationControllerSpec{
							Template: &corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{Image: "index.docker.io/foo/bar"},
										{Image: "index.docker.io/foo/bar"},
									},
								},
							},
						},
					},
				},
				{
					obj: &kappsv1.Deployment{
						Spec: kappsv1.DeploymentSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{Image: "docker.io/foo/bar"},
										{Image: "foo/bar"},
									},
								},
							},
						},
					},
					changed: true,
					expected: &kappsv1.Deployment{
						Spec: kappsv1.DeploymentSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{Image: "index.docker.io/foo/bar"},
										{Image: "index.docker.io/foo/bar"},
									},
								},
							},
						},
					},
				},
				{
					obj: &appsv1.DeploymentConfig{
						Spec: appsv1.DeploymentConfigSpec{
							Template: &corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{Image: "docker.io/foo/bar"},
										{Image: "foo/bar"},
									},
								},
							},
						},
					},
					changed: true,
					expected: &appsv1.DeploymentConfig{
						Spec: appsv1.DeploymentConfigSpec{
							Template: &corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{Image: "index.docker.io/foo/bar"},
										{Image: "index.docker.io/foo/bar"},
									},
								},
							},
						},
					},
				},
				{
					obj: &kappsv1.DaemonSet{
						Spec: kappsv1.DaemonSetSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{Image: "docker.io/foo/bar"},
										{Image: "foo/bar"},
									},
								},
							},
						},
					},
					changed: true,
					expected: &kappsv1.DaemonSet{
						Spec: kappsv1.DaemonSetSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{Image: "index.docker.io/foo/bar"},
										{Image: "index.docker.io/foo/bar"},
									},
								},
							},
						},
					},
				},
				{
					obj: &kappsv1.ReplicaSet{
						Spec: kappsv1.ReplicaSetSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{Image: "docker.io/foo/bar"},
										{Image: "foo/bar"},
									},
								},
							},
						},
					},
					changed: true,
					expected: &kappsv1.ReplicaSet{
						Spec: kappsv1.ReplicaSetSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{Image: "index.docker.io/foo/bar"},
										{Image: "index.docker.io/foo/bar"},
									},
								},
							},
						},
					},
				},
				{
					obj: &batchv1.Job{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{Image: "docker.io/foo/bar"},
										{Image: "foo/bar"},
									},
								},
							},
						},
					},
					changed: true,
					expected: &batchv1.Job{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{Image: "index.docker.io/foo/bar"},
										{Image: "index.docker.io/foo/bar"},
									},
								},
							},
						},
					},
				},
				{
					obj:         &corev1.Node{},
					nilReporter: true,
				},
				{
					obj: &buildv1.BuildConfig{
						Spec: buildv1.BuildConfigSpec{
							CommonSpec: buildv1.CommonSpec{
								Output: buildv1.BuildOutput{To: &corev1.ObjectReference{Kind: "DockerImage", Name: "docker.io/foo/bar"}},
								Source: buildv1.BuildSource{
									Images: []buildv1.ImageSource{
										{From: corev1.ObjectReference{Kind: "DockerImage", Name: "docker.io/foo/bar"}},
										{From: corev1.ObjectReference{Kind: "DockerImage", Name: "foo/bar"}},
									},
								},
								Strategy: buildv1.BuildStrategy{
									DockerStrategy: &buildv1.DockerBuildStrategy{From: &corev1.ObjectReference{Kind: "DockerImage", Name: "docker.io/foo/bar"}},
									SourceStrategy: &buildv1.SourceBuildStrategy{From: corev1.ObjectReference{Kind: "DockerImage", Name: "docker.io/foo/bar"}},
									CustomStrategy: &buildv1.CustomBuildStrategy{From: corev1.ObjectReference{Kind: "DockerImage", Name: "docker.io/foo/bar"}},
								},
							},
						},
					},
					changed: true,
					expected: &buildv1.BuildConfig{
						Spec: buildv1.BuildConfigSpec{
							CommonSpec: buildv1.CommonSpec{
								Output: buildv1.BuildOutput{To: &corev1.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/foo/bar"}},
								Source: buildv1.BuildSource{
									Images: []buildv1.ImageSource{
										{From: corev1.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/foo/bar"}},
										{From: corev1.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/foo/bar"}},
									},
								},
								Strategy: buildv1.BuildStrategy{
									DockerStrategy: &buildv1.DockerBuildStrategy{From: &corev1.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/foo/bar"}},
									SourceStrategy: &buildv1.SourceBuildStrategy{From: corev1.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/foo/bar"}},
									CustomStrategy: &buildv1.CustomBuildStrategy{From: corev1.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/foo/bar"}},
								},
							},
						},
					},
				},
				{
					obj: &corev1.Secret{
						Type: corev1.SecretTypeDockercfg,
						Data: map[string][]byte{
							corev1.DockerConfigKey: []byte(`{"docker.io":{"auth":"Og=="},"other.server":{"auth":"Og=="}}`),
							"another":              []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
					changed: true,
					expected: &corev1.Secret{
						Type: corev1.SecretTypeDockercfg,
						Data: map[string][]byte{
							corev1.DockerConfigKey: []byte(`{"index.docker.io":{"auth":"Og=="},"other.server":{"auth":"Og=="}}`),
							"another":              []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
				},
				{
					obj: &corev1.Secret{
						Type: corev1.SecretTypeDockercfg,
						Data: map[string][]byte{
							corev1.DockerConfigKey: []byte(`{"myserver.com":{"auth":"Og=="},"other.server":{"auth":"Og=="}}`),
							"another":              []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
					expected: &corev1.Secret{
						Type: corev1.SecretTypeDockercfg,
						Data: map[string][]byte{
							corev1.DockerConfigKey: []byte(`{"myserver.com":{"auth":"Og=="},"other.server":{"auth":"Og=="}}`),
							"another":              []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
				},
				{
					obj: &corev1.Secret{
						Type: corev1.SecretTypeDockerConfigJson,
						Data: map[string][]byte{
							corev1.DockerConfigJsonKey: []byte(`{"auths":{"docker.io":{"auth":"Og=="},"other.server":{"auth":"Og=="}}}`),
							"another":                  []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
					changed: true,
					expected: &corev1.Secret{
						Type: corev1.SecretTypeDockerConfigJson,
						Data: map[string][]byte{
							corev1.DockerConfigJsonKey: []byte(`{"auths":{"index.docker.io":{"auth":"Og=="},"other.server":{"auth":"Og=="}}}`),
							"another":                  []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
				},
				{
					obj: &corev1.Secret{
						Type: corev1.SecretTypeDockerConfigJson,
						Data: map[string][]byte{
							corev1.DockerConfigJsonKey: []byte(`{"auths":{"myserver.com":{},"other.server":{}}}`),
							"another":                  []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
					expected: &corev1.Secret{
						Type: corev1.SecretTypeDockerConfigJson,
						Data: map[string][]byte{
							corev1.DockerConfigJsonKey: []byte(`{"auths":{"myserver.com":{},"other.server":{}}}`),
							"another":                  []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
				},
				{
					obj: &corev1.Secret{
						Type: corev1.SecretTypeDockercfg,
						Data: map[string][]byte{
							corev1.DockerConfigKey: []byte(`{"auths":{`),
							"another":              []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
					err: true,
					expected: &corev1.Secret{
						Type: corev1.SecretTypeDockercfg,
						Data: map[string][]byte{
							corev1.DockerConfigKey: []byte(`{"auths":{`),
							"another":              []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
				},
				{
					obj: &corev1.Secret{
						Type: corev1.SecretTypeDockerConfigJson,
						Data: map[string][]byte{
							corev1.DockerConfigJsonKey: []byte(`{"auths":{`),
							"another":                  []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
					err: true,
					expected: &corev1.Secret{
						Type: corev1.SecretTypeDockerConfigJson,
						Data: map[string][]byte{
							corev1.DockerConfigJsonKey: []byte(`{"auths":{`),
							"another":                  []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
				},
				{
					obj: &corev1.Secret{
						Type: corev1.SecretTypeOpaque,
						Data: map[string][]byte{
							corev1.DockerConfigJsonKey: []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
					expected: &corev1.Secret{
						Type: corev1.SecretTypeOpaque,
						Data: map[string][]byte{
							corev1.DockerConfigJsonKey: []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
				},
				{
					obj: &imagev1.Image{
						DockerImageReference: "docker.io/foo/bar",
					},
					changed: true,
					expected: &imagev1.Image{
						DockerImageReference: "index.docker.io/foo/bar",
					},
				},
				{
					obj: &imagev1.Image{
						DockerImageReference: "other.docker.io/foo/bar",
					},
					expected: &imagev1.Image{
						DockerImageReference: "other.docker.io/foo/bar",
					},
				},
				{
					obj: &imagev1.ImageStream{
						Spec: imagev1.ImageStreamSpec{
							Tags: []imagev1.TagReference{
								{Name: "foo", From: &corev1.ObjectReference{Kind: "DockerImage", Name: "docker.io/foo/bar"}},
								{Name: "bar", From: &corev1.ObjectReference{Kind: "ImageStream", Name: "docker.io/foo/bar"}},
								{Name: "baz"},
							},
							DockerImageRepository: "docker.io/foo/bar",
						},
						Status: imagev1.ImageStreamStatus{
							DockerImageRepository: "docker.io/foo/bar",
							Tags: []imagev1.NamedTagEventList{
								{Tag: "bar", Items: []imagev1.TagEvent{
									{DockerImageReference: "docker.io/foo/bar"},
									{DockerImageReference: "docker.io/foo/bar"},
								}},
								{Tag: "baz", Items: []imagev1.TagEvent{
									{DockerImageReference: "some.other/reference"},
									{DockerImageReference: "docker.io/foo/bar"},
								}},
							},
						},
					},
					changed: true,
					expected: &imagev1.ImageStream{
						Spec: imagev1.ImageStreamSpec{
							Tags: []imagev1.TagReference{
								{Name: "foo", From: &corev1.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/foo/bar"}},
								{Name: "bar", From: &corev1.ObjectReference{Kind: "ImageStream", Name: "docker.io/foo/bar"}},
								{Name: "baz"},
							},
							DockerImageRepository: "index.docker.io/foo/bar",
						},
						Status: imagev1.ImageStreamStatus{
							DockerImageRepository: "docker.io/foo/bar",
							Tags: []imagev1.NamedTagEventList{
								{Tag: "bar", Items: []imagev1.TagEvent{
									{DockerImageReference: "index.docker.io/foo/bar"},
									{DockerImageReference: "index.docker.io/foo/bar"},
								}},
								{Tag: "baz", Items: []imagev1.TagEvent{
									{DockerImageReference: "some.other/reference"},
									{DockerImageReference: "index.docker.io/foo/bar"},
								}},
							},
						},
					},
				},
			},
		},
		{
			mappings: ImageReferenceMappings{{FromRegistry: "index.docker.io", ToRegistry: "another.registry"}},
		},
		{
			mappings: ImageReferenceMappings{{FromRegistry: "index.docker.io", ToRegistry: "another.registry", ToName: "extra"}},
		},
	}

	for _, test := range testCases {
		for i, v := range test.variants {
			o := MigrateImageReferenceOptions{
				Mappings:        test.mappings,
				UpdatePodSpecFn: originpolymorphichelpers.NewUpdatePodSpecForObjectFn(polymorphichelpers.UpdatePodSpecForObjectFn),
			}
			reporter, err := o.transform(v.obj)
			if (err != nil) != v.err {
				t.Errorf("%d: %v %t", i, err, v.err)
				continue
			}
			if err != nil {
				continue
			}
			if (reporter == nil) != v.nilReporter {
				t.Errorf("%d: reporter %#v %t", i, reporter, v.nilReporter)
				continue
			}
			if reporter == nil {
				continue
			}
			if reporter.Changed() != v.changed {
				t.Errorf("%d: changed %#v %t", i, reporter, v.changed)
				continue
			}

			if !kapihelper.Semantic.DeepEqual(v.expected, v.obj) {
				t.Errorf("%d: object: %s", i, diff.ObjectDiff(v.expected, v.obj))
				continue
			}
		}
	}
}
