package top

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestImageStreamsTop(t *testing.T) {
	testCases := map[string]struct {
		images   *imageapi.ImageList
		streams  *imageapi.ImageStreamList
		expected []Info
	}{
		"empty image stream": {
			images: &imageapi.ImageList{},
			streams: &imageapi.ImageStreamList{
				Items: []imageapi.ImageStream{
					{
						ObjectMeta: kapi.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imageapi.ImageStreamStatus{
							Tags: map[string]imageapi.TagEventList{
								"tag1": {
									Items: []imageapi.TagEvent{{Image: "image1"}},
								},
							},
						},
					},
				},
			},
			expected: []Info{
				imageStreamInfo{
					ImageStream: "ns1/stream1",
					Images:      0,
					Layers:      0,
				},
			},
		},
		"no storage": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{
						ObjectMeta:        kapi.ObjectMeta{Name: "image1"},
						DockerImageLayers: []imageapi.ImageLayer{{Name: "layer1"}},
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
									Items: []imageapi.TagEvent{{Image: "image1"}},
								},
							},
						},
					},
				},
			},
			expected: []Info{
				imageStreamInfo{
					ImageStream: "ns1/stream1",
					Images:      1,
					Layers:      1,
				},
			},
		},
		"with storage": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{
						ObjectMeta:        kapi.ObjectMeta{Name: "image1"},
						DockerImageLayers: []imageapi.ImageLayer{{Name: "layer1", LayerSize: int64(1024)}},
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
									Items: []imageapi.TagEvent{{Image: "image1"}},
								},
							},
						},
					},
				},
			},
			expected: []Info{
				imageStreamInfo{
					ImageStream: "ns1/stream1",
					Storage:     int64(1024),
					Images:      1,
					Layers:      1,
				},
			},
		},
		"multiple layers": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{
						ObjectMeta: kapi.ObjectMeta{Name: "image1"},
						DockerImageLayers: []imageapi.ImageLayer{
							{Name: "layer1", LayerSize: 1024},
							{Name: "layer2", LayerSize: 512},
						},
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
									Items: []imageapi.TagEvent{{Image: "image1"}},
								},
							},
						},
					},
				},
			},
			expected: []Info{
				imageStreamInfo{
					ImageStream: "ns1/stream1",
					Storage:     int64(1536),
					Images:      1,
					Layers:      2,
				},
			},
		},
		"multiple images": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{
						ObjectMeta:        kapi.ObjectMeta{Name: "image1"},
						DockerImageLayers: []imageapi.ImageLayer{{Name: "layer1", LayerSize: int64(1024)}},
					},
					{
						ObjectMeta: kapi.ObjectMeta{Name: "image2"},
						DockerImageLayers: []imageapi.ImageLayer{
							{Name: "layer1", LayerSize: int64(1024)},
							{Name: "layer2", LayerSize: int64(128)},
						},
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
									Items: []imageapi.TagEvent{{Image: "image1"}},
								},
								"tag2": {
									Items: []imageapi.TagEvent{{Image: "image2"}},
								},
							},
						},
					},
				},
			},
			expected: []Info{
				imageStreamInfo{
					ImageStream: "ns1/stream1",
					Storage:     int64(1152),
					Images:      2,
					Layers:      3,
				},
			},
		},
		"multiple images with manifest config": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{
						ObjectMeta:        kapi.ObjectMeta{Name: "image1"},
						DockerImageLayers: []imageapi.ImageLayer{{Name: "layer1", LayerSize: int64(1024)}},
						DockerImageConfig: "raw image config",
						DockerImageMetadata: imageapi.DockerImage{
							ID: "manifestConfigID",
						},
					},
					{
						ObjectMeta: kapi.ObjectMeta{Name: "image2"},
						DockerImageLayers: []imageapi.ImageLayer{
							{Name: "layer1", LayerSize: int64(1024)},
							{Name: "layer2", LayerSize: int64(128)},
						},
						DockerImageConfig: "raw image config",
						DockerImageMetadata: imageapi.DockerImage{
							ID: "manifestConfigID",
						},
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
									Items: []imageapi.TagEvent{{Image: "image1"}},
								},
								"tag2": {
									Items: []imageapi.TagEvent{{Image: "image2"}},
								},
							},
						},
					},
				},
			},
			expected: []Info{
				imageStreamInfo{
					ImageStream: "ns1/stream1",
					Storage:     int64(1152 + len("raw image config")),
					Images:      2,
					Layers:      3,
				},
			},
		},
		"multiple unreferenced images": {
			images: &imageapi.ImageList{
				Items: []imageapi.Image{
					{
						ObjectMeta:        kapi.ObjectMeta{Name: "image1"},
						DockerImageLayers: []imageapi.ImageLayer{{Name: "layer1", LayerSize: int64(1024)}},
					},
					{
						ObjectMeta: kapi.ObjectMeta{Name: "image2"},
						DockerImageLayers: []imageapi.ImageLayer{
							{Name: "layer1", LayerSize: int64(1024)},
							{Name: "layer2", LayerSize: int64(128)},
						},
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
									Items: []imageapi.TagEvent{{Image: "image1"}},
								},
							},
						},
					},
				},
			},
			expected: []Info{
				imageStreamInfo{
					ImageStream: "ns1/stream1",
					Storage:     int64(1024),
					Images:      1,
					Layers:      1,
				},
			},
		},
	}

	for name, test := range testCases {
		o := TopImageStreamsOptions{
			Images:  test.images,
			Streams: test.streams,
		}
		infos := o.imageStreamsTop()
		if !infosEqual(infos, test.expected) {
			t.Errorf("%s: unexpected infos, expected %#v, got %#v", name, test.expected, infos)
		}
	}
}

func infosEqual(actual, expected []Info) bool {
	if len(actual) != len(expected) {
		return false
	}

	for _, a := range actual {
		found := false
		for _, e := range expected {
			if kapi.Semantic.DeepEqual(a, e) {
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
