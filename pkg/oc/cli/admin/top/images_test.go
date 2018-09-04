package top

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	appsv1 "github.com/openshift/api/apps/v1"
	imagev1 "github.com/openshift/api/image/v1"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/image/dockerlayer"
)

func TestImagesTop(t *testing.T) {
	testCases := map[string]struct {
		images   *imagev1.ImageList
		streams  *imagev1.ImageStreamList
		pods     *corev1.PodList
		expected []Info
	}{
		"no metadata": {
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{ObjectMeta: metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imagev1.ImageStreamList{
				Items: []imagev1.ImageStream{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imagev1.ImageStreamStatus{
							Tags: []imagev1.NamedTagEventList{
								{
									Tag:   "tag1",
									Items: []imagev1.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &corev1.PodList{},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
						DockerImageLayers: []imagev1.ImageLayer{
							{Name: "layer1", LayerSize: int64(512)},
							{Name: "layer2", LayerSize: int64(512)},
						},
						DockerImageManifest: "non empty metadata",
					},
				},
			},
			streams: &imagev1.ImageStreamList{
				Items: []imagev1.ImageStream{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imagev1.ImageStreamStatus{
							Tags: []imagev1.NamedTagEventList{
								{
									Tag:   "tag1",
									Items: []imagev1.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &corev1.PodList{},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
						DockerImageLayers: []imagev1.ImageLayer{
							{Name: "layer1", LayerSize: int64(512)},
							{Name: "layer2", LayerSize: int64(512)},
						},
						DockerImageManifest: `{"schemaVersion": 1, "history": [{"v1Compatibility": "{\"id\":\"2d24f826cb16146e2016ff349a8a33ed5830f3b938d45c0f82943f4ab8c097e7\",\"parent\":\"117ee323aaa9d1b136ea55e4421f4ce413dfc6c0cc6b2186dea6c88d93e1ad7c\",\"created\":\"2015-02-21T02:11:06.735146646Z\",\"container\":\"c9a3eda5951d28aa8dbe5933be94c523790721e4f80886d0a8e7a710132a38ec\",\"container_config\":{\"Hostname\":\"43bd710ec89a\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) CMD [/bin/bash]\"],\"Image\":\"117ee323aaa9d1b136ea55e4421f4ce413dfc6c0cc6b2186dea6c88d93e1ad7c\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[]},\"docker_version\":\"1.4.1\",\"config\":{\"Hostname\":\"43bd710ec89a\",\"Domainname\":\"\",\"User\":\"\",\"Memory\":0,\"MemorySwap\":0,\"CpuShares\":0,\"Cpuset\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"PortSpecs\":null,\"ExposedPorts\":null,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/bash\"],\"Image\":\"117ee323aaa9d1b136ea55e4421f4ce413dfc6c0cc6b2186dea6c88d93e1ad7c\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"NetworkDisabled\":false,\"MacAddress\":\"\",\"OnBuild\":[]},\"architecture\":\"amd64\",\"os\":\"linux\",\"checksum\":\"tarsum.dev+sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855\",\"Size\":0}\n"}]}`,
						DockerImageConfig:   "raw image config",
						DockerImageMetadata: runtime.RawExtension{
							Raw: []byte(`{"Id":"manifestConfigID"}`),
						},
					},
				},
			},
			streams: &imagev1.ImageStreamList{},
			pods:    &corev1.PodList{},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
						DockerImageLayers: []imagev1.ImageLayer{
							{Name: "layer1", LayerSize: int64(512)},
							{Name: "layer2", LayerSize: int64(256)},
							{Name: "layer1", LayerSize: int64(512)},
						},
						DockerImageManifest: "non empty metadata",
						DockerImageConfig:   "raw image config",
					},
				},
			},
			streams: &imagev1.ImageStreamList{},
			pods:    &corev1.PodList{},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{
						ObjectMeta:        metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
						DockerImageLayers: []imagev1.ImageLayer{{Name: "layer1"}, {Name: "layer2"}},
					},
				},
			},
			streams: &imagev1.ImageStreamList{
				Items: []imagev1.ImageStream{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imagev1.ImageStreamStatus{
							Tags: []imagev1.NamedTagEventList{
								{
									Tag:   "tag1",
									Items: []imagev1.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
								{
									Tag:   "tag2",
									Items: []imagev1.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &corev1.PodList{},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{
						ObjectMeta:        metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
						DockerImageLayers: []imagev1.ImageLayer{{Name: "layer1"}, {Name: "layer2"}},
					},
				},
			},
			streams: &imagev1.ImageStreamList{
				Items: []imagev1.ImageStream{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imagev1.ImageStreamStatus{
							Tags: []imagev1.NamedTagEventList{
								{
									Tag:   "tag1",
									Items: []imagev1.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
								{
									Tag:   "tag2",
									Items: []imagev1.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "stream2", Namespace: "ns2"},
						Status: imagev1.ImageStreamStatus{
							Tags: []imagev1.NamedTagEventList{
								{
									Tag:   "tag1",
									Items: []imagev1.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &corev1.PodList{},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{ObjectMeta: metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imagev1.ImageStreamList{},
			pods:    &corev1.PodList{},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{
						ObjectMeta:          metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
						DockerImageLayers:   []imagev1.ImageLayer{{Name: "layer1"}},
						DockerImageManifest: "non empty metadata",
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "image2"},
						DockerImageLayers: []imagev1.ImageLayer{
							{Name: "layer1"},
							{Name: "layer2"},
						},
						DockerImageManifest: "non empty metadata",
					},
				},
			},
			streams: &imagev1.ImageStreamList{},
			pods:    &corev1.PodList{},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{
						ObjectMeta:          metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
						DockerImageLayers:   []imagev1.ImageLayer{{Name: "layer1"}},
						DockerImageManifest: "non empty metadata",
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "image2"},
						DockerImageLayers: []imagev1.ImageLayer{
							{Name: "layer1"},
							{Name: dockerlayer.DigestSha256EmptyTar},
							{Name: "layer2"},
						},
						DockerImageManifest: "non empty metadata",
					},
				},
			},
			streams: &imagev1.ImageStreamList{},
			pods:    &corev1.PodList{},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{
						ObjectMeta:          metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"},
						DockerImageLayers:   []imagev1.ImageLayer{{Name: "layer1"}},
						DockerImageManifest: "non empty metadata",
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "image2"},
						DockerImageLayers: []imagev1.ImageLayer{
							{Name: "layer1"},
							{Name: dockerlayer.GzippedEmptyLayerDigest},
							{Name: "layer2"},
						},
						DockerImageManifest: "non empty metadata",
					},
				},
			},
			streams: &imagev1.ImageStreamList{},
			pods:    &corev1.PodList{},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{ObjectMeta: metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imagev1.ImageStreamList{
				Items: []imagev1.ImageStream{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imagev1.ImageStreamStatus{
							Tags: []imagev1.NamedTagEventList{
								{
									Tag:   "tag1",
									Items: []imagev1.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Annotations: map[string]string{buildapi.BuildAnnotation: "build1"}},
						Spec:       corev1.PodSpec{Containers: []corev1.Container{{Image: "image@sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}}},
						Status:     corev1.PodStatus{Phase: corev1.PodPending},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{ObjectMeta: metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imagev1.ImageStreamList{
				Items: []imagev1.ImageStream{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imagev1.ImageStreamStatus{
							Tags: []imagev1.NamedTagEventList{
								{
									Tag:   "tag1",
									Items: []imagev1.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Annotations: map[string]string{buildapi.BuildAnnotation: "build1"}},
						Spec:       corev1.PodSpec{Containers: []corev1.Container{{Image: "image@sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}}},
						Status:     corev1.PodStatus{Phase: corev1.PodRunning},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{ObjectMeta: metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imagev1.ImageStreamList{
				Items: []imagev1.ImageStream{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imagev1.ImageStreamStatus{
							Tags: []imagev1.NamedTagEventList{
								{
									Tag:   "tag1",
									Items: []imagev1.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Annotations: map[string]string{appsv1.DeploymentPodAnnotation: "deployer1"}},
						Spec:       corev1.PodSpec{Containers: []corev1.Container{{Image: "image@sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}}},
						Status:     corev1.PodStatus{Phase: corev1.PodPending},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{ObjectMeta: metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imagev1.ImageStreamList{
				Items: []imagev1.ImageStream{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imagev1.ImageStreamStatus{
							Tags: []imagev1.NamedTagEventList{
								{
									Tag:   "tag1",
									Items: []imagev1.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Annotations: map[string]string{appsv1.DeploymentPodAnnotation: "deployer1"}},
						Spec:       corev1.PodSpec{Containers: []corev1.Container{{Image: "image@sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}}},
						Status:     corev1.PodStatus{Phase: corev1.PodRunning},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{ObjectMeta: metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imagev1.ImageStreamList{
				Items: []imagev1.ImageStream{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imagev1.ImageStreamStatus{
							Tags: []imagev1.NamedTagEventList{
								{
									Tag:   "tag1",
									Items: []imagev1.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Annotations: map[string]string{appsv1.DeploymentAnnotation: "deplyment1"}},
						Spec:       corev1.PodSpec{Containers: []corev1.Container{{Image: "image@sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}}},
						Status:     corev1.PodStatus{Phase: corev1.PodPending},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{ObjectMeta: metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imagev1.ImageStreamList{
				Items: []imagev1.ImageStream{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imagev1.ImageStreamStatus{
							Tags: []imagev1.NamedTagEventList{
								{
									Tag:   "tag1",
									Items: []imagev1.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Annotations: map[string]string{appsv1.DeploymentAnnotation: "deplyment1"}},
						Spec:       corev1.PodSpec{Containers: []corev1.Container{{Image: "image@sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}}},
						Status:     corev1.PodStatus{Phase: corev1.PodRunning},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{ObjectMeta: metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imagev1.ImageStreamList{
				Items: []imagev1.ImageStream{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imagev1.ImageStreamStatus{
							Tags: []imagev1.NamedTagEventList{
								{
									Tag:   "tag1",
									Items: []imagev1.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1"},
						Spec:       corev1.PodSpec{Containers: []corev1.Container{{Image: "image@sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}}},
						Status:     corev1.PodStatus{Phase: corev1.PodRunning},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{ObjectMeta: metav1.ObjectMeta{Name: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
				},
			},
			streams: &imagev1.ImageStreamList{
				Items: []imagev1.ImageStream{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imagev1.ImageStreamStatus{
							Tags: []imagev1.NamedTagEventList{
								{
									Tag:   "tag1",
									Items: []imagev1.TagEvent{{Image: "sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}},
								},
							},
						},
					},
				},
			},
			pods: &corev1.PodList{
				Items: []corev1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Annotations: map[string]string{"unknown controller": "unknown"}},
						Spec:       corev1.PodSpec{Containers: []corev1.Container{{Image: "image@sha256:08151bf2fc92355f236918bb16905921e6f66e1d03100fb9b18d60125db3df3a"}}},
						Status:     corev1.PodStatus{Phase: corev1.PodRunning},
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
