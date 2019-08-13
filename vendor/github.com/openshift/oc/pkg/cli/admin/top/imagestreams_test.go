package top

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"

	dockerv10 "github.com/openshift/api/image/docker10"
	imagev1 "github.com/openshift/api/image/v1"
)

func TestImageStreamsTop(t *testing.T) {
	testCases := map[string]struct {
		images   *imagev1.ImageList
		streams  *imagev1.ImageStreamList
		expected []Info
	}{
		"empty image stream": {
			images: &imagev1.ImageList{},
			streams: &imagev1.ImageStreamList{
				Items: []imagev1.ImageStream{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "stream1", Namespace: "ns1"},
						Status: imagev1.ImageStreamStatus{
							Tags: []imagev1.NamedTagEventList{
								{
									Tag:   "tag1",
									Items: []imagev1.TagEvent{{Image: "image1"}},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{
						ObjectMeta:        metav1.ObjectMeta{Name: "image1"},
						DockerImageLayers: []imagev1.ImageLayer{{Name: "layer1"}},
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
									Items: []imagev1.TagEvent{{Image: "image1"}},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{
						ObjectMeta:        metav1.ObjectMeta{Name: "image1"},
						DockerImageLayers: []imagev1.ImageLayer{{Name: "layer1", LayerSize: int64(1024)}},
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
									Items: []imagev1.TagEvent{{Image: "image1"}},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "image1"},
						DockerImageLayers: []imagev1.ImageLayer{
							{Name: "layer1", LayerSize: 1024},
							{Name: "layer2", LayerSize: 512},
						},
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
									Items: []imagev1.TagEvent{{Image: "image1"}},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{
						ObjectMeta:        metav1.ObjectMeta{Name: "image1"},
						DockerImageLayers: []imagev1.ImageLayer{{Name: "layer1", LayerSize: int64(1024)}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "image2"},
						DockerImageLayers: []imagev1.ImageLayer{
							{Name: "layer1", LayerSize: int64(1024)},
							{Name: "layer2", LayerSize: int64(128)},
						},
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
									Items: []imagev1.TagEvent{{Image: "image1"}},
								},
								{
									Tag:   "tag2",
									Items: []imagev1.TagEvent{{Image: "image2"}},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{
						ObjectMeta:        metav1.ObjectMeta{Name: "image1"},
						DockerImageLayers: []imagev1.ImageLayer{{Name: "layer1", LayerSize: int64(1024)}},
						DockerImageConfig: "raw image config",
						DockerImageMetadata: runtime.RawExtension{
							Object: &dockerv10.DockerImage{
								ID: "manifestConfigID",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "image2"},
						DockerImageLayers: []imagev1.ImageLayer{
							{Name: "layer1", LayerSize: int64(1024)},
							{Name: "layer2", LayerSize: int64(128)},
						},
						DockerImageConfig: "raw image config",
						DockerImageMetadata: runtime.RawExtension{
							Object: &dockerv10.DockerImage{
								ID: "manifestConfigID",
							},
						},
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
									Items: []imagev1.TagEvent{{Image: "image1"}},
								},
								{
									Tag:   "tag2",
									Items: []imagev1.TagEvent{{Image: "image2"}},
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
			images: &imagev1.ImageList{
				Items: []imagev1.Image{
					{
						ObjectMeta:        metav1.ObjectMeta{Name: "image1"},
						DockerImageLayers: []imagev1.ImageLayer{{Name: "layer1", LayerSize: int64(1024)}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "image2"},
						DockerImageLayers: []imagev1.ImageLayer{
							{Name: "layer1", LayerSize: int64(1024)},
							{Name: "layer2", LayerSize: int64(128)},
						},
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
									Items: []imagev1.TagEvent{{Image: "image1"}},
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
			if kapihelper.Semantic.DeepEqual(a, e) {
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
