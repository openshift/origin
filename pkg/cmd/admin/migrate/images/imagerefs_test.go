package images

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kbatch "k8s.io/kubernetes/pkg/apis/batch"
	kextensions "k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/diff"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
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
					obj: &kapi.Pod{
						Spec: kapi.PodSpec{
							Containers: []kapi.Container{
								{Image: "docker.io/foo/bar"},
								{Image: "foo/bar"},
							},
						},
					},
					changed: true,
					expected: &kapi.Pod{
						Spec: kapi.PodSpec{
							Containers: []kapi.Container{
								{Image: "index.docker.io/foo/bar"},
								{Image: "index.docker.io/foo/bar"},
							},
						},
					},
				},
				{
					obj: &kapi.ReplicationController{
						Spec: kapi.ReplicationControllerSpec{
							Template: &kapi.PodTemplateSpec{
								Spec: kapi.PodSpec{
									Containers: []kapi.Container{
										{Image: "docker.io/foo/bar"},
										{Image: "foo/bar"},
									},
								},
							},
						},
					},
					changed: true,
					expected: &kapi.ReplicationController{
						Spec: kapi.ReplicationControllerSpec{
							Template: &kapi.PodTemplateSpec{
								Spec: kapi.PodSpec{
									Containers: []kapi.Container{
										{Image: "index.docker.io/foo/bar"},
										{Image: "index.docker.io/foo/bar"},
									},
								},
							},
						},
					},
				},
				{
					obj: &kextensions.Deployment{
						Spec: kextensions.DeploymentSpec{
							Template: kapi.PodTemplateSpec{
								Spec: kapi.PodSpec{
									Containers: []kapi.Container{
										{Image: "docker.io/foo/bar"},
										{Image: "foo/bar"},
									},
								},
							},
						},
					},
					changed: true,
					expected: &kextensions.Deployment{
						Spec: kextensions.DeploymentSpec{
							Template: kapi.PodTemplateSpec{
								Spec: kapi.PodSpec{
									Containers: []kapi.Container{
										{Image: "index.docker.io/foo/bar"},
										{Image: "index.docker.io/foo/bar"},
									},
								},
							},
						},
					},
				},
				{
					obj: &deployapi.DeploymentConfig{
						Spec: deployapi.DeploymentConfigSpec{
							Template: &kapi.PodTemplateSpec{
								Spec: kapi.PodSpec{
									Containers: []kapi.Container{
										{Image: "docker.io/foo/bar"},
										{Image: "foo/bar"},
									},
								},
							},
						},
					},
					changed: true,
					expected: &deployapi.DeploymentConfig{
						Spec: deployapi.DeploymentConfigSpec{
							Template: &kapi.PodTemplateSpec{
								Spec: kapi.PodSpec{
									Containers: []kapi.Container{
										{Image: "index.docker.io/foo/bar"},
										{Image: "index.docker.io/foo/bar"},
									},
								},
							},
						},
					},
				},
				{
					obj: &kextensions.DaemonSet{
						Spec: kextensions.DaemonSetSpec{
							Template: kapi.PodTemplateSpec{
								Spec: kapi.PodSpec{
									Containers: []kapi.Container{
										{Image: "docker.io/foo/bar"},
										{Image: "foo/bar"},
									},
								},
							},
						},
					},
					changed: true,
					expected: &kextensions.DaemonSet{
						Spec: kextensions.DaemonSetSpec{
							Template: kapi.PodTemplateSpec{
								Spec: kapi.PodSpec{
									Containers: []kapi.Container{
										{Image: "index.docker.io/foo/bar"},
										{Image: "index.docker.io/foo/bar"},
									},
								},
							},
						},
					},
				},
				{
					obj: &kextensions.ReplicaSet{
						Spec: kextensions.ReplicaSetSpec{
							Template: kapi.PodTemplateSpec{
								Spec: kapi.PodSpec{
									Containers: []kapi.Container{
										{Image: "docker.io/foo/bar"},
										{Image: "foo/bar"},
									},
								},
							},
						},
					},
					changed: true,
					expected: &kextensions.ReplicaSet{
						Spec: kextensions.ReplicaSetSpec{
							Template: kapi.PodTemplateSpec{
								Spec: kapi.PodSpec{
									Containers: []kapi.Container{
										{Image: "index.docker.io/foo/bar"},
										{Image: "index.docker.io/foo/bar"},
									},
								},
							},
						},
					},
				},
				{
					obj: &kbatch.Job{
						Spec: kbatch.JobSpec{
							Template: kapi.PodTemplateSpec{
								Spec: kapi.PodSpec{
									Containers: []kapi.Container{
										{Image: "docker.io/foo/bar"},
										{Image: "foo/bar"},
									},
								},
							},
						},
					},
					changed: true,
					expected: &kbatch.Job{
						Spec: kbatch.JobSpec{
							Template: kapi.PodTemplateSpec{
								Spec: kapi.PodSpec{
									Containers: []kapi.Container{
										{Image: "index.docker.io/foo/bar"},
										{Image: "index.docker.io/foo/bar"},
									},
								},
							},
						},
					},
				},
				{
					obj:         &kapi.Node{},
					nilReporter: true,
				},
				{
					obj: &buildapi.BuildConfig{
						Spec: buildapi.BuildConfigSpec{
							CommonSpec: buildapi.CommonSpec{
								Output: buildapi.BuildOutput{To: &kapi.ObjectReference{Kind: "DockerImage", Name: "docker.io/foo/bar"}},
								Source: buildapi.BuildSource{
									Images: []buildapi.ImageSource{
										{From: kapi.ObjectReference{Kind: "DockerImage", Name: "docker.io/foo/bar"}},
										{From: kapi.ObjectReference{Kind: "DockerImage", Name: "foo/bar"}},
									},
								},
								Strategy: buildapi.BuildStrategy{
									DockerStrategy: &buildapi.DockerBuildStrategy{From: &kapi.ObjectReference{Kind: "DockerImage", Name: "docker.io/foo/bar"}},
									SourceStrategy: &buildapi.SourceBuildStrategy{From: kapi.ObjectReference{Kind: "DockerImage", Name: "docker.io/foo/bar"}},
									CustomStrategy: &buildapi.CustomBuildStrategy{From: kapi.ObjectReference{Kind: "DockerImage", Name: "docker.io/foo/bar"}},
								},
							},
						},
					},
					changed: true,
					expected: &buildapi.BuildConfig{
						Spec: buildapi.BuildConfigSpec{
							CommonSpec: buildapi.CommonSpec{
								Output: buildapi.BuildOutput{To: &kapi.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/foo/bar"}},
								Source: buildapi.BuildSource{
									Images: []buildapi.ImageSource{
										{From: kapi.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/foo/bar"}},
										{From: kapi.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/foo/bar"}},
									},
								},
								Strategy: buildapi.BuildStrategy{
									DockerStrategy: &buildapi.DockerBuildStrategy{From: &kapi.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/foo/bar"}},
									SourceStrategy: &buildapi.SourceBuildStrategy{From: kapi.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/foo/bar"}},
									CustomStrategy: &buildapi.CustomBuildStrategy{From: kapi.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/foo/bar"}},
								},
							},
						},
					},
				},
				{
					obj: &kapi.Secret{
						Type: kapi.SecretTypeDockercfg,
						Data: map[string][]byte{
							kapi.DockerConfigKey: []byte(`{"docker.io":{"auth":"Og=="},"other.server":{"auth":"Og=="}}`),
							"another":            []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
					changed: true,
					expected: &kapi.Secret{
						Type: kapi.SecretTypeDockercfg,
						Data: map[string][]byte{
							kapi.DockerConfigKey: []byte(`{"index.docker.io":{"auth":"Og=="},"other.server":{"auth":"Og=="}}`),
							"another":            []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
				},
				{
					obj: &kapi.Secret{
						Type: kapi.SecretTypeDockercfg,
						Data: map[string][]byte{
							kapi.DockerConfigKey: []byte(`{"myserver.com":{"auth":"Og=="},"other.server":{"auth":"Og=="}}`),
							"another":            []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
					expected: &kapi.Secret{
						Type: kapi.SecretTypeDockercfg,
						Data: map[string][]byte{
							kapi.DockerConfigKey: []byte(`{"myserver.com":{"auth":"Og=="},"other.server":{"auth":"Og=="}}`),
							"another":            []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
				},
				{
					obj: &kapi.Secret{
						Type: kapi.SecretTypeDockerConfigJson,
						Data: map[string][]byte{
							kapi.DockerConfigJsonKey: []byte(`{"auths":{"docker.io":{"auth":"Og=="},"other.server":{"auth":"Og=="}}}`),
							"another":                []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
					changed: true,
					expected: &kapi.Secret{
						Type: kapi.SecretTypeDockerConfigJson,
						Data: map[string][]byte{
							kapi.DockerConfigJsonKey: []byte(`{"auths":{"index.docker.io":{"auth":"Og=="},"other.server":{"auth":"Og=="}}}`),
							"another":                []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
				},
				{
					obj: &kapi.Secret{
						Type: kapi.SecretTypeDockerConfigJson,
						Data: map[string][]byte{
							kapi.DockerConfigJsonKey: []byte(`{"auths":{"myserver.com":{},"other.server":{}}}`),
							"another":                []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
					expected: &kapi.Secret{
						Type: kapi.SecretTypeDockerConfigJson,
						Data: map[string][]byte{
							kapi.DockerConfigJsonKey: []byte(`{"auths":{"myserver.com":{},"other.server":{}}}`),
							"another":                []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
				},
				{
					obj: &kapi.Secret{
						Type: kapi.SecretTypeDockercfg,
						Data: map[string][]byte{
							kapi.DockerConfigKey: []byte(`{"auths":{`),
							"another":            []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
					err: true,
					expected: &kapi.Secret{
						Type: kapi.SecretTypeDockercfg,
						Data: map[string][]byte{
							kapi.DockerConfigKey: []byte(`{"auths":{`),
							"another":            []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
				},
				{
					obj: &kapi.Secret{
						Type: kapi.SecretTypeDockerConfigJson,
						Data: map[string][]byte{
							kapi.DockerConfigJsonKey: []byte(`{"auths":{`),
							"another":                []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
					err: true,
					expected: &kapi.Secret{
						Type: kapi.SecretTypeDockerConfigJson,
						Data: map[string][]byte{
							kapi.DockerConfigJsonKey: []byte(`{"auths":{`),
							"another":                []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
				},
				{
					obj: &kapi.Secret{
						Type: kapi.SecretTypeOpaque,
						Data: map[string][]byte{
							kapi.DockerConfigJsonKey: []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
					expected: &kapi.Secret{
						Type: kapi.SecretTypeOpaque,
						Data: map[string][]byte{
							kapi.DockerConfigJsonKey: []byte(`{"auths":{"docker.io":{},"other.server":{}}}`),
						},
					},
				},
				{
					obj: &imageapi.Image{
						DockerImageReference: "docker.io/foo/bar",
					},
					changed: true,
					expected: &imageapi.Image{
						DockerImageReference: "index.docker.io/foo/bar",
					},
				},
				{
					obj: &imageapi.Image{
						DockerImageReference: "other.docker.io/foo/bar",
					},
					expected: &imageapi.Image{
						DockerImageReference: "other.docker.io/foo/bar",
					},
				},
				{
					obj: &imageapi.ImageStream{
						Spec: imageapi.ImageStreamSpec{
							Tags: map[string]imageapi.TagReference{
								"foo": {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "docker.io/foo/bar"}},
								"bar": {From: &kapi.ObjectReference{Kind: "ImageStream", Name: "docker.io/foo/bar"}},
								"baz": {},
							},
							DockerImageRepository: "docker.io/foo/bar",
						},
						Status: imageapi.ImageStreamStatus{
							DockerImageRepository: "docker.io/foo/bar",
							Tags: map[string]imageapi.TagEventList{
								"bar": {Items: []imageapi.TagEvent{
									{DockerImageReference: "docker.io/foo/bar"},
									{DockerImageReference: "docker.io/foo/bar"},
								}},
								"baz": {Items: []imageapi.TagEvent{
									{DockerImageReference: "some.other/reference"},
									{DockerImageReference: "docker.io/foo/bar"},
								}},
							},
						},
					},
					changed: true,
					expected: &imageapi.ImageStream{
						Spec: imageapi.ImageStreamSpec{
							Tags: map[string]imageapi.TagReference{
								"foo": {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "index.docker.io/foo/bar"}},
								"bar": {From: &kapi.ObjectReference{Kind: "ImageStream", Name: "docker.io/foo/bar"}},
								"baz": {},
							},
							DockerImageRepository: "index.docker.io/foo/bar",
						},
						Status: imageapi.ImageStreamStatus{
							DockerImageRepository: "docker.io/foo/bar",
							Tags: map[string]imageapi.TagEventList{
								"bar": {Items: []imageapi.TagEvent{
									{DockerImageReference: "index.docker.io/foo/bar"},
									{DockerImageReference: "index.docker.io/foo/bar"},
								}},
								"baz": {Items: []imageapi.TagEvent{
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
				UpdatePodSpecFn: clientcmd.NewFactory(nil).UpdatePodSpecForObject,
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
			if !kapi.Semantic.DeepEqual(v.expected, v.obj) {
				t.Errorf("%d: object: %s", i, diff.ObjectDiff(v.expected, v.obj))
				continue
			}
		}
	}
}
