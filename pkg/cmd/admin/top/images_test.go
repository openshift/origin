package top

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/docker/distribution/digest"

	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestImagesTop(t *testing.T) {
	testCases := map[string]struct {
		images   *imageapi.ImageList
		streams  *imageapi.ImageStreamList
		pods     *kapi.PodList
		expected []Info
	}{
		"no metadata": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{ObjectMeta: kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imageapi.ImageStreamList{
				Items: []imageapi.ImageStream{
					{
						ObjectMeta: kapi.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imageapi.ImageStreamStatus{
							Tags: map[string]imageapi.TagEventList{
								"tag1": {
									Items: []imageapi.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &kapi.PodList{},
			expected: []Info{
				imageInfo{
					Image:           "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					ImageStreamTags: []string{"ns1/stream1 (tag1)"},
					Metadata:        false,
					Parents:         []string{},
					Usage:           []string{},
				},
			},
		},
		"with metadata": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{
						ObjectMeta: kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
						DockerImageLayers: []imageapi.ImageLayer{
							{Name: "layer1", LayerSize: int64(512)},
							{Name: "layer2", LayerSize: int64(512)},
						},
						DockerImageManifest: "non empty metadata",
					},
				},
			},
			streams: &imageapi.ImageStreamList{
				Items: []imageapi.ImageStream{
					{
						ObjectMeta: kapi.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imageapi.ImageStreamStatus{
							Tags: map[string]imageapi.TagEventList{
								"tag1": {
									Items: []imageapi.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &kapi.PodList{},
			expected: []Info{
				imageInfo{
					Image:           "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					ImageStreamTags: []string{"ns1/stream1 (tag1)"},
					Metadata:        true,
					Parents:         []string{},
					Usage:           []string{},
					Storage:         int64(1024),
				},
			},
		},
		"with metadata and image config": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{
						ObjectMeta: kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
						DockerImageLayers: []imageapi.ImageLayer{
							{Name: "layer1", LayerSize: int64(512)},
							{Name: "layer2", LayerSize: int64(512)},
						},
						DockerImageManifest: "non empty metadata",
						DockerImageConfig:   "raw image config",
						DockerImageMetadata: imageapi.DockerImage{
							ID: "manifestConfigID",
						},
					},
				},
			},
			streams: &imageapi.ImageStreamList{},
			pods:    &kapi.PodList{},
			expected: []Info{
				imageInfo{
					Image:    "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					Metadata: true,
					Parents:  []string{},
					Usage:    []string{},
					Storage:  int64(1024 + len("raw image config")),
				},
			},
		},
		"with metadata and image config and some layers duplicated": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{
						ObjectMeta: kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
						DockerImageLayers: []imageapi.ImageLayer{
							{Name: "layer1", LayerSize: int64(512)},
							{Name: "layer2", LayerSize: int64(256)},
							{Name: "layer1", LayerSize: int64(512)},
						},
						DockerImageManifest: "non empty metadata",
						DockerImageConfig:   "raw image config",
						DockerImageMetadata: imageapi.DockerImage{
							ID: "layer2",
						},
					},
				},
			},
			streams: &imageapi.ImageStreamList{},
			pods:    &kapi.PodList{},
			expected: []Info{
				imageInfo{
					Image:    "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					Metadata: true,
					Parents:  []string{},
					Usage:    []string{},
					Storage:  int64(512 + 256),
				},
			},
		},
		"multiple tags": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{
						ObjectMeta:        kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
						DockerImageLayers: []imageapi.ImageLayer{{Name: "layer1"}, {Name: "layer2"}},
					},
				},
			},
			streams: &imageapi.ImageStreamList{
				Items: []imageapi.ImageStream{
					{
						ObjectMeta: kapi.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imageapi.ImageStreamStatus{
							Tags: map[string]imageapi.TagEventList{
								"tag1": {
									Items: []imageapi.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
								"tag2": {
									Items: []imageapi.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &kapi.PodList{},
			expected: []Info{
				imageInfo{
					Image:           "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					ImageStreamTags: []string{"ns1/stream1 (tag1,tag2)"},
					Metadata:        false,
					Parents:         []string{},
					Usage:           []string{},
				},
			},
		},
		"multiple streams": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{
						ObjectMeta:        kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
						DockerImageLayers: []imageapi.ImageLayer{{Name: "layer1"}, {Name: "layer2"}},
					},
				},
			},
			streams: &imageapi.ImageStreamList{
				Items: []imageapi.ImageStream{
					{
						ObjectMeta: kapi.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imageapi.ImageStreamStatus{
							Tags: map[string]imageapi.TagEventList{
								"tag1": {
									Items: []imageapi.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
								"tag2": {
									Items: []imageapi.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
					{
						ObjectMeta: kapi.ObjectMeta{Name: "stream2", Namespace: "ns2"},
						Status: imageapi.ImageStreamStatus{
							Tags: map[string]imageapi.TagEventList{
								"tag1": {
									Items: []imageapi.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &kapi.PodList{},
			expected: []Info{
				imageInfo{
					Image:           "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					ImageStreamTags: []string{"ns1/stream1 (tag1,tag2)", "ns2/stream2 (tag1)"},
					Metadata:        false,
					Parents:         []string{},
					Usage:           []string{},
				},
			},
		},
		"image without a stream": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{ObjectMeta: kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imageapi.ImageStreamList{},
			pods:    &kapi.PodList{},
			expected: []Info{
				imageInfo{
					Image:           "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					ImageStreamTags: []string{},
					Metadata:        false,
					Parents:         []string{},
					Usage:           []string{},
				},
			},
		},
		"image parents": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{
						ObjectMeta:          kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
						DockerImageLayers:   []imageapi.ImageLayer{{Name: "layer1"}},
						DockerImageManifest: "non empty metadata",
					},
					{
						ObjectMeta: kapi.ObjectMeta{Name: "image2"},
						DockerImageLayers: []imageapi.ImageLayer{
							{Name: "layer1"},
							{Name: "layer2"},
						},
						DockerImageManifest: "non empty metadata",
					},
				},
			},
			streams: &imageapi.ImageStreamList{},
			pods:    &kapi.PodList{},
			expected: []Info{
				imageInfo{
					Image:           "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					ImageStreamTags: []string{},
					Metadata:        true,
					Parents:         []string{},
					Usage:           []string{},
				},
				imageInfo{
					Image:           "image2",
					ImageStreamTags: []string{},
					Metadata:        true,
					Parents:         []string{"sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
					Usage:           []string{},
				},
			},
		},
		"image parents with empty layer": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{
						ObjectMeta:          kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
						DockerImageLayers:   []imageapi.ImageLayer{{Name: "layer1"}},
						DockerImageManifest: "non empty metadata",
					},
					{
						ObjectMeta: kapi.ObjectMeta{Name: "image2"},
						DockerImageLayers: []imageapi.ImageLayer{
							{Name: "layer1"},
							{Name: digest.DigestSha256EmptyTar},
							{Name: "layer2"},
						},
						DockerImageManifest: "non empty metadata",
					},
				},
			},
			streams: &imageapi.ImageStreamList{},
			pods:    &kapi.PodList{},
			expected: []Info{
				imageInfo{
					Image:           "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					ImageStreamTags: []string{},
					Metadata:        true,
					Parents:         []string{},
					Usage:           []string{},
				},
				imageInfo{
					Image:           "image2",
					ImageStreamTags: []string{},
					Metadata:        true,
					Parents:         []string{"sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
					Usage:           []string{},
				},
			},
		},
		"image parents with gzipped empty layer": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{
						ObjectMeta:          kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
						DockerImageLayers:   []imageapi.ImageLayer{{Name: "layer1"}},
						DockerImageManifest: "non empty metadata",
					},
					{
						ObjectMeta: kapi.ObjectMeta{Name: "image2"},
						DockerImageLayers: []imageapi.ImageLayer{
							{Name: "layer1"},
							{Name: digestSHA256GzippedEmptyTar},
							{Name: "layer2"},
						},
						DockerImageManifest: "non empty metadata",
					},
				},
			},
			streams: &imageapi.ImageStreamList{},
			pods:    &kapi.PodList{},
			expected: []Info{
				imageInfo{
					Image:           "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					ImageStreamTags: []string{},
					Metadata:        true,
					Parents:         []string{},
					Usage:           []string{},
				},
				imageInfo{
					Image:           "image2",
					ImageStreamTags: []string{},
					Metadata:        true,
					Parents:         []string{"sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
					Usage:           []string{},
				},
			},
		},
		"build pending": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{ObjectMeta: kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imageapi.ImageStreamList{
				Items: []imageapi.ImageStream{
					{
						ObjectMeta: kapi.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imageapi.ImageStreamStatus{
							Tags: map[string]imageapi.TagEventList{
								"tag1": {
									Items: []imageapi.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &kapi.PodList{
				Items: []kapi.Pod{
					{
						ObjectMeta: kapi.ObjectMeta{Namespace: "ns1", Annotations: map[string]string{buildapi.BuildAnnotation: "build1"}},
						Spec:       kapi.PodSpec{Containers: []kapi.Container{{Image: "image@sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}}},
						Status:     kapi.PodStatus{Phase: kapi.PodPending},
					},
				},
			},
			expected: []Info{
				imageInfo{
					Image:           "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					ImageStreamTags: []string{"ns1/stream1 (tag1)"},
					Metadata:        false,
					Parents:         []string{},
					Usage:           []string{"Build: ns1/build1"},
				},
			},
		},
		"build running": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{ObjectMeta: kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imageapi.ImageStreamList{
				Items: []imageapi.ImageStream{
					{
						ObjectMeta: kapi.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imageapi.ImageStreamStatus{
							Tags: map[string]imageapi.TagEventList{
								"tag1": {
									Items: []imageapi.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &kapi.PodList{
				Items: []kapi.Pod{
					{
						ObjectMeta: kapi.ObjectMeta{Namespace: "ns1", Annotations: map[string]string{buildapi.BuildAnnotation: "build1"}},
						Spec:       kapi.PodSpec{Containers: []kapi.Container{{Image: "image@sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}}},
						Status:     kapi.PodStatus{Phase: kapi.PodRunning},
					},
				},
			},
			expected: []Info{
				imageInfo{
					Image:           "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					ImageStreamTags: []string{"ns1/stream1 (tag1)"},
					Metadata:        false,
					Parents:         []string{},
					Usage:           []string{"Build: ns1/build1"},
				},
			},
		},
		"deployer pending": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{ObjectMeta: kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imageapi.ImageStreamList{
				Items: []imageapi.ImageStream{
					{
						ObjectMeta: kapi.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imageapi.ImageStreamStatus{
							Tags: map[string]imageapi.TagEventList{
								"tag1": {
									Items: []imageapi.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &kapi.PodList{
				Items: []kapi.Pod{
					{
						ObjectMeta: kapi.ObjectMeta{Namespace: "ns1", Annotations: map[string]string{deployapi.DeploymentPodAnnotation: "deployer1"}},
						Spec:       kapi.PodSpec{Containers: []kapi.Container{{Image: "image@sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}}},
						Status:     kapi.PodStatus{Phase: kapi.PodPending},
					},
				},
			},
			expected: []Info{
				imageInfo{
					Image:           "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					ImageStreamTags: []string{"ns1/stream1 (tag1)"},
					Metadata:        false,
					Parents:         []string{},
					Usage:           []string{"Deployer: ns1/deployer1"},
				},
			},
		},
		"deployer running": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{ObjectMeta: kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imageapi.ImageStreamList{
				Items: []imageapi.ImageStream{
					{
						ObjectMeta: kapi.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imageapi.ImageStreamStatus{
							Tags: map[string]imageapi.TagEventList{
								"tag1": {
									Items: []imageapi.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &kapi.PodList{
				Items: []kapi.Pod{
					{
						ObjectMeta: kapi.ObjectMeta{Namespace: "ns1", Annotations: map[string]string{deployapi.DeploymentPodAnnotation: "deployer1"}},
						Spec:       kapi.PodSpec{Containers: []kapi.Container{{Image: "image@sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}}},
						Status:     kapi.PodStatus{Phase: kapi.PodRunning},
					},
				},
			},
			expected: []Info{
				imageInfo{
					Image:           "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					ImageStreamTags: []string{"ns1/stream1 (tag1)"},
					Metadata:        false,
					Parents:         []string{},
					Usage:           []string{"Deployer: ns1/deployer1"},
				},
			},
		},
		"deployement pending": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{ObjectMeta: kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imageapi.ImageStreamList{
				Items: []imageapi.ImageStream{
					{
						ObjectMeta: kapi.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imageapi.ImageStreamStatus{
							Tags: map[string]imageapi.TagEventList{
								"tag1": {
									Items: []imageapi.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &kapi.PodList{
				Items: []kapi.Pod{
					{
						ObjectMeta: kapi.ObjectMeta{Namespace: "ns1", Annotations: map[string]string{deployapi.DeploymentAnnotation: "deplyment1"}},
						Spec:       kapi.PodSpec{Containers: []kapi.Container{{Image: "image@sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}}},
						Status:     kapi.PodStatus{Phase: kapi.PodPending},
					},
				},
			},
			expected: []Info{
				imageInfo{
					Image:           "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					ImageStreamTags: []string{"ns1/stream1 (tag1)"},
					Metadata:        false,
					Parents:         []string{},
					Usage:           []string{"Deployment: ns1/deplyment1"},
				},
			},
		},
		"deployment running": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{ObjectMeta: kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imageapi.ImageStreamList{
				Items: []imageapi.ImageStream{
					{
						ObjectMeta: kapi.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imageapi.ImageStreamStatus{
							Tags: map[string]imageapi.TagEventList{
								"tag1": {
									Items: []imageapi.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &kapi.PodList{
				Items: []kapi.Pod{
					{
						ObjectMeta: kapi.ObjectMeta{Namespace: "ns1", Annotations: map[string]string{deployapi.DeploymentAnnotation: "deplyment1"}},
						Spec:       kapi.PodSpec{Containers: []kapi.Container{{Image: "image@sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}}},
						Status:     kapi.PodStatus{Phase: kapi.PodRunning},
					},
				},
			},
			expected: []Info{
				imageInfo{
					Image:           "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					ImageStreamTags: []string{"ns1/stream1 (tag1)"},
					Metadata:        false,
					Parents:         []string{},
					Usage:           []string{"Deployment: ns1/deplyment1"},
				},
			},
		},
		"unknown controller 1": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{ObjectMeta: kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imageapi.ImageStreamList{
				Items: []imageapi.ImageStream{
					{
						ObjectMeta: kapi.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imageapi.ImageStreamStatus{
							Tags: map[string]imageapi.TagEventList{
								"tag1": {
									Items: []imageapi.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &kapi.PodList{
				Items: []kapi.Pod{
					{
						ObjectMeta: kapi.ObjectMeta{Namespace: "ns1"},
						Spec:       kapi.PodSpec{Containers: []kapi.Container{{Image: "image@sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}}},
						Status:     kapi.PodStatus{Phase: kapi.PodRunning},
					},
				},
			},
			expected: []Info{
				imageInfo{
					Image:           "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					ImageStreamTags: []string{"ns1/stream1 (tag1)"},
					Metadata:        false,
					Parents:         []string{},
					Usage:           []string{"<unknown>"},
				},
			},
		},
		"unknown controller 2": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{ObjectMeta: kapi.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imageapi.ImageStreamList{
				Items: []imageapi.ImageStream{
					{
						ObjectMeta: kapi.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imageapi.ImageStreamStatus{
							Tags: map[string]imageapi.TagEventList{
								"tag1": {
									Items: []imageapi.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &kapi.PodList{
				Items: []kapi.Pod{
					{
						ObjectMeta: kapi.ObjectMeta{Namespace: "ns1", Annotations: map[string]string{"unknown controller": "unknown"}},
						Spec:       kapi.PodSpec{Containers: []kapi.Container{{Image: "image@sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}}},
						Status:     kapi.PodStatus{Phase: kapi.PodRunning},
					},
				},
			},
			expected: []Info{
				imageInfo{
					Image:           "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a",
					ImageStreamTags: []string{"ns1/stream1 (tag1)"},
					Metadata:        false,
					Parents:         []string{},
					Usage:           []string{"<unknown>"},
				},
			},
		},
	}

	for name, test := range testCases {
		o := TopImagesOptions{
			Images:  test.images,
			Streams: test.streams,
			Pods:    test.pods,
		}
		infos := o.imagesTop()
		if !imageInfosEqual(infos, test.expected) {
			t.Errorf("%s: unexpected infos, expected %#v, got %#v", name, test.expected, infos)
		}
	}
}

func imageInfosEqual(actual, expected []Info) bool {
	if len(actual) != len(expected) {
		return false
	}

	for _, a := range actual {
		aii, ok := a.(imageInfo)
		if !ok {
			continue
		}
		for _, e := range expected {
			eii, ok := e.(imageInfo)
			if !ok {
				continue
			}
			if aii.Image != eii.Image {
				continue
			}
			if !stringsEqual(aii.ImageStreamTags, eii.ImageStreamTags) ||
				!stringsEqual(aii.Parents, eii.Parents) ||
				!stringsEqual(aii.Usage, eii.Usage) ||
				aii.Metadata != eii.Metadata ||
				aii.Storage != eii.Storage {
				return false
			}
			return true
		}
	}
	return false
}

func stringsEqual(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}

	for _, a := range actual {
		found := false
		for _, e := range expected {
			if a == e {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
