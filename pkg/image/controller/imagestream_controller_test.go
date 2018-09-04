package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apitesting "k8s.io/apimachinery/pkg/api/testing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	restfake "k8s.io/client-go/rest/fake"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"
	kcontroller "k8s.io/kubernetes/pkg/controller"

	imagev1 "github.com/openshift/api/image/v1"
	fakeimagev1client "github.com/openshift/client-go/image/clientset/versioned/fake"
	imagev1informer "github.com/openshift/client-go/image/informers/externalversions"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

func TestHandleImageStream(t *testing.T) {
	one, two := int64(1), int64(2)
	testCases := []struct {
		stream   *imagev1.ImageStream
		expected *imagev1.ImageStreamImportSpec
	}{
		{
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{imageapi.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
			},
		},
		{
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{imageapi.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: imagev1.ImageStreamSpec{
					DockerImageRepository: "test/other",
				},
			},
		},
		{
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{imageapi.DockerImageRepositoryCheckAnnotation: "a random error"},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: imagev1.ImageStreamSpec{
					DockerImageRepository: "test/other",
				},
			},
		},

		// references are ignored
		{
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{
							Name:      "latest",
							From:      &corev1.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Reference: true,
						},
					},
				},
			},
		},
		{
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{
							Name:      "latest",
							From:      &corev1.ObjectReference{Kind: "AnotherImage", Name: "test/other:latest"},
							Reference: true,
						},
					},
				},
			},
		},

		// spec tag will be imported
		{
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{
							Name: "latest",
							From: &corev1.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
						},
					},
				},
			},
			expected: &imagev1.ImageStreamImportSpec{
				Import: true,
				Images: []imagev1.ImageImportSpec{
					{
						From: corev1.ObjectReference{
							Kind: "DockerImage",
							Name: "test/other:latest",
						},
						To: &corev1.LocalObjectReference{
							Name: "latest",
						},
					},
				},
			},
		},
		// spec tag with generation with no pending status will be imported
		{
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{
							Name:       "latest",
							From:       &corev1.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
			},
			expected: &imagev1.ImageStreamImportSpec{
				Import: true,
				Images: []imagev1.ImageImportSpec{
					{
						From: corev1.ObjectReference{
							Kind: "DockerImage",
							Name: "test/other:latest",
						},
						To: &corev1.LocalObjectReference{
							Name: "latest",
						},
					},
				},
			},
		},
		// spec tag with generation with older status generation will be imported
		{
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{
							Name:       "v1",
							From:       &corev1.ObjectReference{Kind: "DockerImage", Name: "test/other:v1"},
							Generation: &one,
						},
						{
							Name:       "latest",
							From:       &corev1.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
				Status: imagev1.ImageStreamStatus{
					Tags: []imagev1.NamedTagEventList{
						{
							Tag: "v1",
							Items: []imagev1.TagEvent{
								{
									Generation: 1,
								},
							},
						},
						{
							Tag: "latest",
							Items: []imagev1.TagEvent{
								{
									Generation: 1,
								},
							},
						},
					},
				},
			},
			expected: &imagev1.ImageStreamImportSpec{
				Import: true,
				Images: []imagev1.ImageImportSpec{
					{
						From: corev1.ObjectReference{
							Kind: "DockerImage",
							Name: "test/other:latest",
						},
						To: &corev1.LocalObjectReference{
							Name: "latest",
						},
					},
				},
			},
		},
		// spec tag with generation with status condition error and equal generation will not be imported
		{
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{imageapi.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{
							Name:       "latest",
							From:       &corev1.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
				Status: imagev1.ImageStreamStatus{
					Tags: []imagev1.NamedTagEventList{
						{
							Tag: "latest",
							Conditions: []imagev1.TagEventCondition{
								{
									Type:       imagev1.ImportSuccess,
									Status:     corev1.ConditionFalse,
									Generation: 2,
								},
							},
						},
					},
				},
			},
		},
		// spec tag with generation with status condition error and older generation will be imported
		{
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{imageapi.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{
							Name:       "latest",
							From:       &corev1.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
				Status: imagev1.ImageStreamStatus{
					Tags: []imagev1.NamedTagEventList{
						{
							Tag: "latest",
							Conditions: []imagev1.TagEventCondition{
								{
									Type:       imagev1.ImportSuccess,
									Status:     corev1.ConditionFalse,
									Generation: 1,
								},
							},
						},
					},
				},
			},
			expected: &imagev1.ImageStreamImportSpec{
				Import: true,
				Images: []imagev1.ImageImportSpec{
					{
						From: corev1.ObjectReference{
							Kind: "DockerImage",
							Name: "test/other:latest",
						},
						To: &corev1.LocalObjectReference{
							Name: "latest",
						},
					},
				},
			},
		},
		// spec tag with generation with older status generation will be imported
		{
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{imageapi.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{
							Name:       "latest",
							From:       &corev1.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
				Status: imagev1.ImageStreamStatus{
					Tags: []imagev1.NamedTagEventList{
						{
							Tag:   "latest",
							Items: []imagev1.TagEvent{{Generation: 1}},
						},
					},
				},
			},
			expected: &imagev1.ImageStreamImportSpec{
				Import: true,
				Images: []imagev1.ImageImportSpec{
					{
						From: corev1.ObjectReference{
							Kind: "DockerImage",
							Name: "test/other:latest",
						},
						To: &corev1.LocalObjectReference{
							Name: "latest",
						},
					},
				},
			},
		},
		// test external repo
		{
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "other",
				},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{
							Name: "1.1",
							From: &corev1.ObjectReference{
								Kind: "DockerImage",
								Name: "some/repo:mytag",
							},
						},
					},
				},
			},
			expected: &imagev1.ImageStreamImportSpec{
				Import: true,
				Images: []imagev1.ImageImportSpec{
					{
						From: corev1.ObjectReference{
							Kind: "DockerImage",
							Name: "some/repo:mytag",
						},
						To: &corev1.LocalObjectReference{
							Name: "1.1",
						},
					},
				},
			},
		},
		// spec tag with generation with current status generation will be ignored
		{
			stream: &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{imageapi.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: imagev1.ImageStreamSpec{
					Tags: []imagev1.TagReference{
						{
							Name:       "latest",
							From:       &corev1.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
				Status: imagev1.ImageStreamStatus{
					Tags: []imagev1.NamedTagEventList{
						{
							Tag:   "latest",
							Items: []imagev1.TagEvent{{Generation: 2}},
						},
					},
				},
			},
			expected: nil,
		},
	}

	for i, test := range testCases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			_, codecs := apitesting.SchemeForOrDie(imagev1.Install)
			actions := 0
			fakeREST := &restfake.RESTClient{
				NegotiatedSerializer: codecs,
				GroupVersion:         imagev1.SchemeGroupVersion,
				Client: restfake.CreateHTTPClient(func(*http.Request) (*http.Response, error) {
					actions++
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     header(),
						Body:       objBody(&imagev1.ImageStreamImport{}),
					}, nil
				}),
			}
			other := test.stream.DeepCopy()

			if _, err := handleImageStream(test.stream, fakeREST, nil); err != nil {
				t.Errorf("unexpected error: %#v", err)
			}
			if test.expected != nil {
				if actions == 0 {
					t.Fatal("expected remote calls, got: 0")
				}
				if fakeREST.Req.Method != "POST" && !strings.Contains(fakeREST.Req.URL.String(), "imagestreamimports") {
					t.Errorf("expected a create action, got %v %v", fakeREST.Req.Method, fakeREST.Req.URL)
				}
				obj, err := decode(fakeREST.Req.Body, codecs.UniversalDeserializer())
				if err != nil {
					t.Fatalf("unexpected error %#v", err)
				}
				isi, ok := obj.(*imagev1.ImageStreamImport)
				if !ok {
					t.Fatalf("unexpected object %T", obj)
				}
				if !reflect.DeepEqual(*test.expected, isi.Spec) {
					t.Errorf("%d: expected object differs:\n1) %#v\n2) %#v\n\n", i, *test.expected, isi.Spec)
				}
			} else {
				if !kapihelper.Semantic.DeepEqual(test.stream, other) {
					t.Errorf("did not expect change to stream: %s", diff.ObjectGoPrintDiff(test.stream, other))
				}
				if actions != 0 {
					t.Errorf("did not expect remote calls, but got %d", actions)
				}
			}
		})
	}
}

func TestProcessNextWorkItemOnRemovedStream(t *testing.T) {
	clientset := fakeimagev1client.NewSimpleClientset()
	informer := imagev1informer.NewSharedInformerFactory(fakeimagev1client.NewSimpleClientset(), 0)
	isc := NewImageStreamController(clientset, informer.Image().V1().ImageStreams())
	isc.queue.Add("other/test")
	isc.processNextWorkItem()
	if isc.queue.Len() != 0 {
		t.Errorf("Unexpected queue length, expected 0, got %d", isc.queue.Len())
	}
}

func TestProcessNextWorkItem(t *testing.T) {
	stream := &imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{imageapi.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
			Name:        "test",
			Namespace:   "other",
		},
	}
	clientset := fakeimagev1client.NewSimpleClientset(stream)
	informer := imagev1informer.NewSharedInformerFactory(fakeimagev1client.NewSimpleClientset(stream), 0)
	isc := NewImageStreamController(clientset, informer.Image().V1().ImageStreams())
	key, _ := kcontroller.KeyFunc(stream)
	isc.queue.Add(key)
	isc.processNextWorkItem()
	if isc.queue.Len() != 0 {
		t.Errorf("Unexpected queue length, expected 0, got %d", isc.queue.Len())
	}
}

func objBody(object interface{}) io.ReadCloser {
	output, err := json.MarshalIndent(object, "", "")
	if err != nil {
		panic(err)
	}
	return ioutil.NopCloser(bytes.NewReader([]byte(output)))
}

func header() http.Header {
	header := http.Header{}
	header.Set("Content-Type", runtime.ContentTypeJSON)
	return header
}

func decode(reader io.ReadCloser, decoder runtime.Decoder) (runtime.Object, error) {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	obj, err := runtime.Decode(decoder, data)
	if err != nil {
		return nil, err
	}
	return obj, nil
}
