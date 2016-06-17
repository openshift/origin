package admission

import (
	"fmt"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"

	imagetest "github.com/openshift/origin/pkg/image/admission/testutil"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestGetImageReferenceForObjectReference(t *testing.T) {
	for _, tc := range []struct {
		name           string
		namespace      string
		objRef         kapi.ObjectReference
		expectedString string
		expectedError  bool
	}{
		{
			name: "isimage without namespace",
			objRef: kapi.ObjectReference{
				Kind: "ImageStreamImage",
				Name: imageapi.MakeImageStreamImageName("is", imagetest.BaseImageWith1LayerDigest),
			},
			expectedString: "is@" + imagetest.BaseImageWith1LayerDigest,
		},

		{
			name:      "isimage with a fallback namespace",
			namespace: "fallback",
			objRef: kapi.ObjectReference{
				Kind: "ImageStreamImage",
				Name: imageapi.MakeImageStreamImageName("is", imagetest.BaseImageWith1LayerDigest),
			},
			expectedString: "fallback/is@" + imagetest.BaseImageWith1LayerDigest,
		},

		{
			name:      "isimage with namespace set",
			namespace: "fallback",
			objRef: kapi.ObjectReference{
				Kind:      "ImageStreamImage",
				Namespace: "ns",
				Name:      imageapi.MakeImageStreamImageName("is", imagetest.BaseImageWith1LayerDigest),
			},
			expectedString: "ns/is@" + imagetest.BaseImageWith1LayerDigest,
		},

		{
			name: "isimage missing id",
			objRef: kapi.ObjectReference{
				Kind: "ImageStreamImage",
				Name: imagetest.InternalRegistryURL + "/is",
			},
			expectedError: true,
		},

		{
			name: "isimage with a tag",
			objRef: kapi.ObjectReference{
				Kind: "ImageStreamImage",
				Name: imagetest.InternalRegistryURL + "/is:latest",
			},
			expectedError: true,
		},

		{
			name: "istag without namespace",
			objRef: kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "is:latest",
			},
			expectedString: "is:latest",
		},

		{
			name:      "istag with fallback namespace",
			namespace: "fallback",
			objRef: kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "is:latest",
			},
			expectedString: "fallback/is:latest",
		},

		{
			name:      "istag with namespace set",
			namespace: "fallback",
			objRef: kapi.ObjectReference{
				Kind:      "ImageStreamTag",
				Namespace: "ns",
				Name:      "is:latest",
			},
			expectedString: "ns/is:latest",
		},

		{
			name: "istag with missing tag",
			objRef: kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "is",
			},
			expectedError: true,
		},

		{
			name: "istag with image ID",
			objRef: kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "is@" + imagetest.BaseImageWith1LayerDigest,
			},
			expectedError: true,
		},

		{
			name: "dockerimage without registry url",
			objRef: kapi.ObjectReference{
				Kind:      "DockerImage",
				Namespace: "ns",
				Name:      "repo@" + imagetest.BaseImageWith1LayerDigest,
			},
			expectedString: "docker.io/repo@" + imagetest.BaseImageWith1LayerDigest,
		},

		{
			name: "dockerimage with a default tag",
			objRef: kapi.ObjectReference{
				Kind:      "DockerImage",
				Namespace: "ns",
				Name:      "library/repo:latest",
			},
			expectedString: "docker.io/repo",
		},

		{
			name: "dockerimage with a non-default tag",
			objRef: kapi.ObjectReference{
				Kind:      "DockerImage",
				Namespace: "ns",
				Name:      "repo:tag",
			},
			expectedString: "docker.io/repo:tag",
		},

		{
			name: "dockerimage referencing docker image",
			objRef: kapi.ObjectReference{
				Kind: "DockerImage",
				Name: "index.docker.io/repo@" + imagetest.BaseImageWith1LayerDigest,
			},
			expectedString: "docker.io/repo@" + imagetest.BaseImageWith1LayerDigest,
		},

		{
			name: "dockerimage without tag or id",
			objRef: kapi.ObjectReference{
				Kind: "DockerImage",
				Name: "index.docker.io/user/repo",
			},
			expectedString: "docker.io/user/repo",
		},

		{
			name: "dockerimage with internal registry",
			objRef: kapi.ObjectReference{
				Kind: "DockerImage",
				Name: imagetest.MakeDockerImageReference("test", "is", imagetest.BaseImageWith1LayerDigest),
			},
			expectedString: imagetest.InternalRegistryURL + "/test/is@" + imagetest.BaseImageWith1LayerDigest,
		},

		{
			name: "bad king",
			objRef: kapi.ObjectReference{
				Kind: "dockerImage",
				Name: imagetest.MakeDockerImageReference("test", "is", imagetest.BaseImageWith1LayerDigest),
			},
			expectedError: true,
		},
	} {

		res, err := GetImageReferenceForObjectReference(tc.namespace, &tc.objRef)
		if tc.expectedError && err == nil {
			t.Errorf("[%s] got unexpected non-error", tc.name)
		}
		if !tc.expectedError {
			if err != nil {
				t.Errorf("[%s] got unexpected error: %v", tc.name, err)
			}
			if res != tc.expectedString {
				t.Errorf("[%s] got unexpected results (%q != %q)", tc.name, res, tc.expectedString)
			}
		}
	}
}

func TestGetImageStreamUsage(t *testing.T) {
	for _, tc := range []struct {
		name           string
		is             imageapi.ImageStream
		expectedTags   int64
		expectedImages int64
	}{
		{
			name: "empty",
		},

		{
			name: "single tag",
			is: imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"latest": {
							Name: "latest",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "openshift/base:v1",
							},
						},
					},
				},
			},
			expectedTags: 1,
		},

		{
			name: "single image",
			is: imageapi.ImageStream{
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: imagetest.MakeDockerImageReference("test", "is", imagetest.BaseImageWith1LayerDigest),
									Image:                imagetest.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
			expectedImages: 1,
		},

		{
			name: "tag and image",
			is: imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.MiscImageDigest),
							},
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: imagetest.MakeDockerImageReference("test", "is", imagetest.BaseImageWith1LayerDigest),
									Image:                imagetest.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
			expectedTags:   1,
			expectedImages: 1,
		},

		{
			name: "two images under one tag",
			is: imageapi.ImageStream{
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: imagetest.MakeDockerImageReference("test", "sharedlayer", imagetest.BaseImageWith1LayerDigest),
									Image:                imagetest.BaseImageWith1LayerDigest,
								},
								{
									DockerImageReference: imagetest.MakeDockerImageReference("test", "sharedlayer", imagetest.BaseImageWith2LayersDigest),
									Image:                imagetest.BaseImageWith2LayersDigest,
								},
							},
						},
					},
				},
			},
			expectedImages: 2,
		},

		{
			name: "two different tags",
			is: imageapi.ImageStream{
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"foo": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: imagetest.MakeDockerImageReference("test", "sharedlayer", imagetest.BaseImageWith2LayersDigest),
									Image:                imagetest.BaseImageWith2LayersDigest,
								},
							},
						},
						"bar": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: imagetest.MakeDockerImageReference("test", "sharedlayer", imagetest.ChildImageWith3LayersDigest),
									Image:                imagetest.ChildImageWith3LayersDigest,
								},
							},
						},
					},
				},
			},
			expectedImages: 2,
		},

		{
			name: "the same image under different tags",
			is: imageapi.ImageStream{
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: imagetest.MakeDockerImageReference("test", "noshared", imagetest.ChildImageWith2LayersDigest),
									Image:                imagetest.ChildImageWith2LayersDigest,
								},
							},
						},
						"foo": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: imagetest.MakeDockerImageReference("nm", "repository", imagetest.ChildImageWith2LayersDigest),
									Image:                imagetest.ChildImageWith2LayersDigest,
								},
							},
						},
					},
				},
			},
			expectedImages: 1,
		},

		{
			name: "two non-canonical references",
			is: imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "repo:latest",
							},
						},
						"same": {
							Name: "same",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "index.docker.io/repo",
							},
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"new": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: "docker.io/library/repo:latest",
									Image:                imagetest.ChildImageWith3LayersDigest,
								},
							},
						},
					},
				},
			},
			expectedTags:   1,
			expectedImages: 1,
		},

		{
			name: "the same image in both spec and status",
			is: imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "noshared",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: imagetest.MakeDockerImageReference("test", "noshared", imagetest.ChildImageWith2LayersDigest),
							},
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: imagetest.MakeDockerImageReference("test", "noshared", imagetest.ChildImageWith2LayersDigest),
									Image:                imagetest.ChildImageWith2LayersDigest,
								},
							},
						},
					},
				},
			},
			expectedTags:   1,
			expectedImages: 1,
		},

		{
			name: "imagestreamtag and dockerimage references",
			is: imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"ist": {
							Name: "ist",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamTag",
								Namespace: "shared",
								Name:      "is:latest",
							},
						},
						"dockerimage": {
							Name: "dockerimage",
							From: &kapi.ObjectReference{
								Kind:      "DockerImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is:latest"),
							},
						},
					},
				},
			},
			expectedTags: 2,
		},

		{
			name: "dockerimage reference tagged in status",
			is: imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"dockerimage": {
							Name: "dockerimage",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: imagetest.MakeDockerImageReference("test", "is", imagetest.BaseImageWith1LayerDigest),
							},
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: imagetest.MakeDockerImageReference("test", "is", imagetest.BaseImageWith1LayerDigest),
									Image:                imagetest.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
			expectedTags:   1,
			expectedImages: 1,
		},

		{
			name: "wrong spec image references",
			is: imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"badkind": {
							Name: "badkind",
							From: &kapi.ObjectReference{
								Kind: "unknown",
								Name: imagetest.MakeDockerImageReference("test", "is", imagetest.BaseImageWith1LayerDigest),
							},
						},
						"badistag": {
							Name: "badistag",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamTag",
								Namespace: "shared",
								Name:      "is",
							},
						},
						"badisimage": {
							Name: "badistag",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      "is:tag",
							},
						},
						"good": {
							Name: "good",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: imagetest.MakeDockerImageReference("test", "is", imagetest.BaseImageWith1LayerDigest),
							},
						},
					},
				},
			},
			expectedTags: 1,
		},

		{
			name: "identical tags with fallback namespace",
			is: imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "fallback",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"havingnamespace": {
							Name: "havingnamespace",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamTag",
								Namespace: "fallback",
								Name:      "other:tag",
							},
						},
						"lackingnamespace": {
							Name: "lackingnamespace",
							From: &kapi.ObjectReference{
								Kind: "ImageStreamTag",
								Name: "other:tag",
							},
						},
					},
				},
			},
			expectedTags: 1,
		},

		{
			name: "identical tags without fallback namespace",
			is: imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"havingnamespace": {
							Name: "havingnamespace",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamTag",
								Namespace: "ns",
								Name:      "other:tag",
							},
						},
						"lackingnamespace": {
							Name: "lackingnamespace",
							From: &kapi.ObjectReference{
								Kind: "ImageStreamTag",
								Name: "other:tag",
							},
						},
					},
				},
			},
			expectedTags: 2,
		},
	} {
		usage := GetImageStreamUsage(&tc.is)
		expectedUsage := kapi.ResourceList{
			imageapi.ResourceImageStreamTags:   *resource.NewQuantity(tc.expectedTags, resource.DecimalSI),
			imageapi.ResourceImageStreamImages: *resource.NewQuantity(tc.expectedImages, resource.DecimalSI),
		}

		if len(usage) != len(expectedUsage) {
			t.Errorf("[%s] got unexpected number of limits (%d != %d)", tc.name, len(usage), len(expectedUsage))
		}

		for r, expVal := range expectedUsage {
			val, exists := usage[r]
			if !exists {
				t.Errorf("[%s] expected resource %s is missing", tc.name, r)
				continue
			}
			if val.Cmp(expVal) != 0 {
				t.Errorf("[%s] got unexpected value for resource %s (%s != %s)", tc.name, r, val.String(), expVal.String())
			}
		}

		for r := range usage {
			if _, exists := expectedUsage[r]; !exists {
				t.Errorf("[%s] got unexpected resource %s", tc.name, r)
			}
		}
	}
}
