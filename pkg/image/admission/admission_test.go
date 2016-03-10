package admission

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imagetest "github.com/openshift/origin/pkg/quota/image/testutil"

	kadmission "k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/unversioned"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	clientsetfake "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"
)

func TestAdmitImageStreamTags(t *testing.T) {
	istWithoutFrom := getImageStreamTag()
	istWithoutFrom.Tag.From = nil

	istWithoutTag := getImageStreamTag()
	istWithoutTag.Tag = nil

	tests := map[string]struct {
		imageStreamTag *imageapi.ImageStreamTag
		limitRange     *kapi.LimitRange
		shouldAdmit    bool
		operation      kadmission.Operation
	}{
		"new ist, no limit range": {
			imageStreamTag: getImageStreamTag(),
			operation:      kadmission.Create,
			shouldAdmit:    true,
		},
		"new ist, under limit range": {
			imageStreamTag: getImageStreamTag(),
			limitRange:     getLimitRange("1Ki"),
			operation:      kadmission.Create,
			shouldAdmit:    true,
		},
		"new ist, over limit range": {
			imageStreamTag: getImageStreamTag(),
			limitRange:     getLimitRange("0Ki"),
			operation:      kadmission.Create,
			shouldAdmit:    false,
		},
		"ist without From": {
			imageStreamTag: istWithoutFrom,
			limitRange:     getLimitRange("1Ki"),
			operation:      kadmission.Create,
			shouldAdmit:    true,
		},
		"ist without Tag": {
			imageStreamTag: istWithoutTag,
			limitRange:     getLimitRange("1Ki"),
			operation:      kadmission.Create,
			shouldAdmit:    true,
		},
	}

	for k, v := range tests {
		var fakeKubeClient clientset.Interface
		if v.limitRange != nil {
			fakeKubeClient = clientsetfake.NewSimpleClientset(v.limitRange)
		} else {
			fakeKubeClient = clientsetfake.NewSimpleClientset()
		}

		fakeOSClient := &testclient.Fake{}
		fakeOSClient.AddReactor("get", "images", imagetest.GetFakeImageGetHandler(t, v.imageStreamTag.Namespace))

		plugin, err := NewImageLimitRangerPlugin(fakeKubeClient, nil)
		if err != nil {
			t.Errorf("%s failed creating plugin %v", k, err)
			continue
		}
		plugin.SetOpenshiftClient(fakeOSClient)

		attrs := kadmission.NewAttributesRecord(v.imageStreamTag,
			unversioned.GroupKind{},
			v.imageStreamTag.Namespace,
			v.imageStreamTag.Name,
			imageapi.Resource("imagestreamtags"),
			"",
			v.operation,
			nil)

		err = plugin.Admit(attrs)
		if v.shouldAdmit && err != nil {
			t.Errorf("%s expected to be admitted but recieved errors %#v", k, err)
		}
		if !v.shouldAdmit && err == nil {
			t.Errorf("%s expected to be rejected but received no errors", k)
		}
	}
}

func TestAdmitImageStreamMapping(t *testing.T) {
	tests := map[string]struct {
		imageStreamMapping *imageapi.ImageStreamMapping
		limitRange         *kapi.LimitRange
		shouldAdmit        bool
		operation          kadmission.Operation
	}{
		"new ism, no limit range": {
			imageStreamMapping: getImageStreamMapping(),
			operation:          kadmission.Create,
			shouldAdmit:        true,
		},
		"new ism, under limit range": {
			imageStreamMapping: getImageStreamMapping(),
			limitRange:         getLimitRange("1Ki"),
			operation:          kadmission.Create,
			shouldAdmit:        true,
		},
		"new ism, over limit range": {
			imageStreamMapping: getImageStreamMapping(),
			limitRange:         getLimitRange("0Ki"),
			operation:          kadmission.Create,
			shouldAdmit:        false,
		},
	}

	for k, v := range tests {
		var fakeKubeClient clientset.Interface
		if v.limitRange != nil {
			fakeKubeClient = clientsetfake.NewSimpleClientset(v.limitRange)
		} else {
			fakeKubeClient = clientsetfake.NewSimpleClientset()
		}

		fakeOSClient := &testclient.Fake{}
		fakeOSClient.AddReactor("get", "images", imagetest.GetFakeImageGetHandler(t, v.imageStreamMapping.Namespace))

		plugin, err := NewImageLimitRangerPlugin(fakeKubeClient, nil)
		if err != nil {
			t.Errorf("%s failed creating plugin %v", k, err)
			continue
		}
		plugin.SetOpenshiftClient(fakeOSClient)

		attrs := kadmission.NewAttributesRecord(v.imageStreamMapping,
			unversioned.GroupKind{},
			v.imageStreamMapping.Namespace,
			v.imageStreamMapping.Name,
			imageapi.Resource("imagestreammappings"),
			"",
			v.operation,
			nil)

		err = plugin.Admit(attrs)
		if v.shouldAdmit && err != nil {
			t.Errorf("%s expected to be admitted but recieved errors %#v", k, err)
		}
		if !v.shouldAdmit && err == nil {
			t.Errorf("%s expected to be rejected but received no errors", k)
		}
	}
}

func TestAdmitImageStream(t *testing.T) {
	tests := map[string]struct {
		imageStream *imageapi.ImageStream
		previousIS  *imageapi.ImageStream
		limitRange  *kapi.LimitRange
		shouldAdmit bool
		operation   kadmission.Operation
	}{
		"new image stream, no limit range": {
			imageStream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith2LayersDigest),
							},
						},
					},
				},
			},
			operation:   kadmission.Create,
			shouldAdmit: true,
		},
		"new image stream, under limit range": {
			imageStream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith2LayersDigest),
							},
						},
					},
				},
			},
			limitRange:  getLimitRange("1Ki"),
			operation:   kadmission.Create,
			shouldAdmit: true,
		},
		"new image stream, over limit range": {
			imageStream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith2LayersDigest),
							},
						},
					},
				},
			},
			limitRange:  getLimitRange("0Ki"),
			operation:   kadmission.Create,
			shouldAdmit: false,
		},
		"update image stream, no changes": {
			imageStream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith2LayersDigest),
							},
						},
					},
				},
			},
			previousIS: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith2LayersDigest),
							},
						},
					},
				},
			},
			limitRange:  getLimitRange("0Ki"),
			operation:   kadmission.Update,
			shouldAdmit: true,
		},
		"update image stream, new tag under limit range": {
			imageStream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith2LayersDigest),
							},
						},
						"existing": {
							Name: "existing",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith1LayerDigest),
							},
						},
					},
				},
			},
			previousIS: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"existing": {
							Name: "existing",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith1LayerDigest),
							},
						},
					},
				},
			},
			limitRange:  getLimitRange("1Ki"),
			operation:   kadmission.Update,
			shouldAdmit: true,
		},
		"update image stream, new tag over limit range": {
			imageStream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith2LayersDigest),
							},
						},
						"existing": {
							Name: "existing",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith1LayerDigest),
							},
						},
					},
				},
			},
			previousIS: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"existing": {
							Name: "existing",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith1LayerDigest),
							},
						},
					},
				},
			},
			limitRange:  getLimitRange("0Ki"),
			operation:   kadmission.Update,
			shouldAdmit: false,
		},
	}

	for k, v := range tests {
		var fakeKubeClient clientset.Interface
		if v.limitRange != nil {
			fakeKubeClient = clientsetfake.NewSimpleClientset(v.limitRange)
		} else {
			fakeKubeClient = clientsetfake.NewSimpleClientset()
		}

		fakeOSClient := &testclient.Fake{}
		plugin, err := NewImageLimitRangerPlugin(fakeKubeClient, nil)
		if err != nil {
			t.Errorf("%s failed creating plugin %v", k, err)
			continue
		}

		// if we're using a previous image stream seed the store and also allow it to be retrieved
		// from the reactors
		if v.previousIS != nil {
			plugin.imageStreamStore.Add(v.previousIS)
			fakeOSClient.AddReactor("get", "imagestreams", imagetest.GetFakeImageStreamGetHandler(t, *v.previousIS))
		}

		fakeOSClient.AddReactor("get", "images", imagetest.GetFakeImageGetHandler(t, v.imageStream.Namespace))

		plugin.SetOpenshiftClient(fakeOSClient)

		attrs := kadmission.NewAttributesRecord(v.imageStream,
			unversioned.GroupKind{},
			v.imageStream.Namespace,
			v.imageStream.Name,
			imageapi.Resource("imagestreams"),
			"",
			v.operation,
			nil)

		err = plugin.Admit(attrs)
		if v.shouldAdmit && err != nil {
			t.Errorf("%s expected to be admitted but recieved errors %#v", k, err)
		}
		if !v.shouldAdmit && err == nil {
			t.Errorf("%s expected to be rejected but received no errors", k)
		}
	}
}

func TestGetImageStreamImages(t *testing.T) {
	tests := map[string]struct {
		imageStream    *imageapi.ImageStream
		previousIS     *imageapi.ImageStream
		expectedImages []imageapi.Image
	}{
		"new image stream": {
			imageStream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith2LayersDigest),
							},
						},
					},
				},
			},
			expectedImages: []imageapi.Image{getBaseImageWith2Layers()},
		},
		"new image stream with multiple images": {
			imageStream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "newOneLayer",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith1LayerDigest),
							},
						},
						"new2": {
							Name: "newTwoLayer",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith2LayersDigest),
							},
						},
					},
				},
			},
			expectedImages: []imageapi.Image{getBaseImageWith1Layer(), getBaseImageWith2Layers()},
		},
		"new image stream with status tags": {
			imageStream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith1LayerDigest),
									Image:                imagetest.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
			expectedImages: []imageapi.Image{getBaseImageWith1Layer()},
		},
		"new image stream with spec and status": {
			imageStream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith2LayersDigest),
							},
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith1LayerDigest),
									Image:                imagetest.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
			expectedImages: []imageapi.Image{getBaseImageWith1Layer(), getBaseImageWith2Layers()},
		},
		"existing image stream with no changes spec": {
			imageStream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith2LayersDigest),
							},
						},
					},
				},
			},
			previousIS: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith2LayersDigest),
							},
						},
					},
				},
			},
			expectedImages: []imageapi.Image{},
		},
		"existing image stream with no changes status": {
			imageStream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith1LayerDigest),
									Image:                imagetest.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
			previousIS: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith1LayerDigest),
									Image:                imagetest.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
			expectedImages: []imageapi.Image{},
		},
		"existing image stream with change status": {
			imageStream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith2LayersDigest),
							},
						},
						"existing": {
							Name: "existing",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith1LayerDigest),
							},
						},
					},
				},
			},
			previousIS: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"existing": {
							Name: "existing",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith1LayerDigest),
							},
						},
					},
				},
			},
			expectedImages: []imageapi.Image{getBaseImageWith2Layers()},
		},
		"existing image stream with changed status": {
			imageStream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"new": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith2LayersDigest),
									Image:                imagetest.BaseImageWith2LayersDigest,
								},
							},
						},
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith1LayerDigest),
									Image:                imagetest.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
			previousIS: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith1LayerDigest),
									Image:                imagetest.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
			expectedImages: []imageapi.Image{getBaseImageWith2Layers()},
		},
	}

	for k, v := range tests {
		fakeKubeClient := clientsetfake.NewSimpleClientset()
		plugin, err := NewImageLimitRangerPlugin(fakeKubeClient, nil)
		if err != nil {
			t.Errorf("%s failed creating plugin %v", k, err)
			continue
		}

		fakeOSClient := &testclient.Fake{}
		if v.previousIS != nil {
			fakeOSClient.AddReactor("get", "imagestreams", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
				return true, v.previousIS, nil
			})
		}
		fakeOSClient.AddReactor("get", "images", imagetest.GetFakeImageGetHandler(t, v.imageStream.Namespace))
		plugin.SetOpenshiftClient(fakeOSClient)

		imagesToProcess, err := plugin.getImageStreamImages(v.imageStream, v.previousIS)
		if err != nil {
			t.Errorf("%s received error in getImageStreamImages: %v", k, err)
			continue
		}

		if len(imagesToProcess) != len(v.expectedImages) {
			t.Errorf("%s did not recieve the correct number images - wanted %d images and received %d images", k, len(v.expectedImages), len(imagesToProcess))
		}

		for _, expected := range v.expectedImages {
			if actual, ok := imagesToProcess[expected.Name]; !ok {
				t.Errorf("%s was missing %s in imagesToProcess", k, expected.Name)
			} else {
				if !reflect.DeepEqual(expected, actual) {
					t.Errorf("%s does not match the expected image", expected.Name)
					t.Errorf("expected: %#v", expected)
					t.Errorf("received: %#v", actual)
				}
			}
		}
	}
}

func TestAdmitImage(t *testing.T) {
	tests := map[string]struct {
		size           resource.Quantity
		limitSize      resource.Quantity
		shouldAdmit    bool
		limitRangeItem *kapi.LimitRangeItem
	}{
		"under size": {
			size:        resource.MustParse("50Mi"),
			limitSize:   resource.MustParse("100Mi"),
			shouldAdmit: true,
		},
		"equal size": {
			size:        resource.MustParse("100Mi"),
			limitSize:   resource.MustParse("100Mi"),
			shouldAdmit: true,
		},
		"over size": {
			size:        resource.MustParse("101Mi"),
			limitSize:   resource.MustParse("100Mi"),
			shouldAdmit: false,
		},
		"non-applicable limit range item": {
			size: resource.MustParse("100Mi"),
			limitRangeItem: &kapi.LimitRangeItem{
				Type: kapi.LimitTypeContainer,
			},
			shouldAdmit: true,
		},
	}

	for k, v := range tests {
		limitRangeItem := v.limitRangeItem
		if limitRangeItem == nil {
			limitRangeItem = &kapi.LimitRangeItem{
				Type: imageapi.LimitTypeImageSize,
				Max: kapi.ResourceList{
					kapi.ResourceStorage: v.limitSize,
				},
			}
		}

		err := AdmitImage(v.size.Value(), *limitRangeItem)
		if v.shouldAdmit && err != nil {
			t.Errorf("%s expected to be admitted but received error %v", k, err)
		}

		if !v.shouldAdmit && err == nil {
			t.Errorf("%s expected to be denied but was admitted", k)
		}
	}
}

func TestLimitAppliestoImages(t *testing.T) {
	tests := map[string]struct {
		limitRange  *kapi.LimitRange
		shouldApply bool
	}{
		"good limit range": {
			limitRange: &kapi.LimitRange{
				Spec: kapi.LimitRangeSpec{
					Limits: []kapi.LimitRangeItem{
						{
							Type: imageapi.LimitTypeImageSize,
						},
					},
				},
			},
			shouldApply: true,
		},
		"bad limit range": {
			limitRange: &kapi.LimitRange{
				Spec: kapi.LimitRangeSpec{
					Limits: []kapi.LimitRangeItem{
						{
							Type: kapi.LimitTypeContainer,
						},
					},
				},
			},
			shouldApply: false,
		},
		"malformed range with no type": {
			limitRange: &kapi.LimitRange{
				Spec: kapi.LimitRangeSpec{
					Limits: []kapi.LimitRangeItem{},
				},
			},
			shouldApply: false,
		},
		"malformed range with no limits": {
			limitRange:  &kapi.LimitRange{},
			shouldApply: false,
		},
	}

	plugin, err := NewImageLimitRangerPlugin(clientsetfake.NewSimpleClientset(), nil)
	if err != nil {
		t.Fatalf("error creating plugin: %v", err)
	}

	for k, v := range tests {
		supports := plugin.SupportsLimit(v.limitRange)
		if supports && !v.shouldApply {
			t.Errorf("%s expected limit range to not be applicable", k)
		}
		if !supports && v.shouldApply {
			t.Errorf("%s expected limit range to be applicable", k)
		}
	}
}

func TestNewImageRecordingHandler(t *testing.T) {
	images := map[string]imageapi.Image{}
	theFunc := newImageRecordingHandler(images)
	theFunc("tag", "dockerImageReference", &imageapi.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo"}})

	if len(images) != 1 {
		t.Errorf("expected image to be added to passed in map by processor func")
		return
	}
	if img, ok := images["foo"]; !ok || img.Name != "foo" {
		t.Errorf("expected the image 'foo' to be added to map by processor func but found %#v", images)
	}
}

func TestHandles(t *testing.T) {
	plugin, err := NewImageLimitRangerPlugin(clientsetfake.NewSimpleClientset(), nil)
	if err != nil {
		t.Fatalf("error creating plugin: %v", err)
	}
	if !plugin.Handles(kadmission.Create) {
		t.Errorf("plugin is expected to handle create")
	}
	if !plugin.Handles(kadmission.Update) {
		t.Errorf("plugin is expected to handle update")
	}
	if plugin.Handles(kadmission.Delete) {
		t.Errorf("plugin is not expected to handle delete")
	}
	if plugin.Handles(kadmission.Connect) {
		t.Errorf("plugin is expected to handle connect")
	}
}

func TestSupports(t *testing.T) {
	resources := []string{"imagestreammappings", "imagestreamtags", "imagestreams"}
	plugin, err := NewImageLimitRangerPlugin(clientsetfake.NewSimpleClientset(), nil)
	if err != nil {
		t.Fatalf("error creating plugin: %v", err)
	}
	for _, r := range resources {
		attr := kadmission.NewAttributesRecord(nil, unversioned.GroupKind{}, "ns", "name", imageapi.Resource(r), "", kadmission.Create, nil)
		if !plugin.SupportsAttributes(attr) {
			t.Errorf("plugin is expected to support %s", r)
		}
	}

	badResources := []string{"pods", "", "adfasdfas"}
	for _, r := range badResources {
		attr := kadmission.NewAttributesRecord(nil, unversioned.GroupKind{}, "ns", "name", imageapi.Resource(r), "", kadmission.Create, nil)
		if plugin.SupportsAttributes(attr) {
			t.Errorf("plugin is not expected to support %s", r)
		}
	}
}

func getBaseImageWith2Layers() imageapi.Image {
	return imageapi.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name:        imagetest.BaseImageWith2LayersDigest,
			Annotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
		},
		DockerImageReference: fmt.Sprintf("registry.example.org/%s/%s", "test", imagetest.BaseImageWith2LayersDigest),
		DockerImageManifest:  imagetest.BaseImageWith2Layers,
	}
}

func getBaseImageWith1Layer() imageapi.Image {
	return imageapi.Image{
		ObjectMeta: kapi.ObjectMeta{
			Name:        imagetest.BaseImageWith1LayerDigest,
			Annotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
		},
		DockerImageReference: fmt.Sprintf("registry.example.org/%s/%s", "test", imagetest.BaseImageWith1LayerDigest),
		DockerImageManifest:  imagetest.BaseImageWith1Layer,
	}
}

func getLimitRange(limit string) *kapi.LimitRange {
	return &kapi.LimitRange{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test-limit",
			Namespace: "test",
		},
		Spec: kapi.LimitRangeSpec{
			Limits: []kapi.LimitRangeItem{
				{
					Type: imageapi.LimitTypeImageSize,
					Max: kapi.ResourceList{
						kapi.ResourceStorage: resource.MustParse(limit),
					},
				},
			},
		},
	}
}

func getImageStreamMapping() *imageapi.ImageStreamMapping {
	return &imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "test-ism",
			Namespace: "test",
		},
		Image: getBaseImageWith1Layer(),
	}
}

func getImageStreamTag() *imageapi.ImageStreamTag {
	return &imageapi.ImageStreamTag{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: "test",
			Name:      "havingtag:latest",
		},
		Tag: &imageapi.TagReference{
			Name: "latest",
			From: &kapi.ObjectReference{
				Kind:      "ImageStreamImage",
				Namespace: "shared",
				Name:      "is@" + imagetest.ChildImageWith2LayersDigest,
			},
		},
	}
}
