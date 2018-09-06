package tag

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/api/image"
	imagev1 "github.com/openshift/api/image/v1"
	fakeimagev1client "github.com/openshift/client-go/image/clientset/versioned/fake"
)

type testAction struct {
	verb, resource string
}

func testData() []*imagev1.ImageStream {
	return []*imagev1.ImageStream{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "rails", Namespace: "yourproject", ResourceVersion: "10", CreationTimestamp: metav1.Now()},
			Spec: imagev1.ImageStreamSpec{
				DockerImageRepository: "",
				Tags: []imagev1.TagReference{},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "rails", Namespace: "yourproject", ResourceVersion: "11", CreationTimestamp: metav1.Now()},
			Spec: imagev1.ImageStreamSpec{
				DockerImageRepository: "",
				Tags: []imagev1.TagReference{
					{
						Name: "tip",
						From: &corev1.ObjectReference{
							Name:      "ruby",
							Namespace: "openshift",
							Kind:      "ImageStreamTag",
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "rails", Namespace: "myproject", ResourceVersion: "10", CreationTimestamp: metav1.Now()},
			Spec: imagev1.ImageStreamSpec{
				DockerImageRepository: "",
				Tags: []imagev1.TagReference{
					{
						Name: "latest",
						From: &corev1.ObjectReference{
							Name:      "ruby",
							Namespace: "openshift",
							Kind:      "ImageStreamTag",
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "django", Namespace: "yourproject", ResourceVersion: "11", CreationTimestamp: metav1.Now()},
			Spec: imagev1.ImageStreamSpec{
				DockerImageRepository: "",
				Tags: []imagev1.TagReference{
					{
						Name: "tip",
						From: &corev1.ObjectReference{
							Name:      "python",
							Namespace: "openshift",
							Kind:      "ImageStreamTag",
						},
					},
				},
			},
		},
	}
}

func TestTag(t *testing.T) {
	streams := testData()

	testCases := map[string]struct {
		data            []runtime.Object
		opts            *TagOptions
		expectedActions []testAction
		validateErr     string
		runErr          string
	}{
		"tag across namespaces": {
			data: []runtime.Object{streams[2], streams[0]},
			opts: &TagOptions{
				ref: imagev1.DockerImageReference{
					Namespace: "openshift",
					Name:      "ruby",
					Tag:       "latest",
				},
				referencePolicy: SourceReferencePolicy,
				namespace:       "myproject2",
				sourceKind:      "ImageStreamTag",
				destNamespace:   []string{"yourproject"},
				destNameAndTag:  []string{"rails:tip"},
			},
			expectedActions: []testAction{
				{verb: "update", resource: "imagestreamtags"},
				{verb: "create", resource: "imagestreamtags"},
				{verb: "get", resource: "imagestreams"},
				{verb: "update", resource: "imagestreams"},
			},
		},
		"alias tag across namespaces": {
			data: []runtime.Object{streams[2], streams[0]},
			opts: &TagOptions{
				ref: imagev1.DockerImageReference{
					Namespace: "openshift",
					Name:      "ruby",
					Tag:       "latest",
				},
				aliasTag:        true,
				referencePolicy: SourceReferencePolicy,
				namespace:       "myproject2",
				sourceKind:      "ImageStreamTag",
				destNamespace:   []string{"yourproject"},
				destNameAndTag:  []string{"rails:tip"},
			},
			validateErr: "cannot set alias across different Image Streams",
		},
		"alias tag across image streams": {
			data: []runtime.Object{streams[3], streams[0]},
			opts: &TagOptions{
				ref: imagev1.DockerImageReference{
					Namespace: "yourproject",
					Name:      "rails",
					Tag:       "latest",
				},
				aliasTag:        true,
				referencePolicy: SourceReferencePolicy,
				namespace:       "myproject2",
				sourceKind:      "ImageStreamTag",
				destNamespace:   []string{"yourproject"},
				destNameAndTag:  []string{"python:alias"},
			},
			validateErr: "cannot set alias across different Image Streams",
		},
		"add old": {
			data: []runtime.Object{streams[0]},
			opts: &TagOptions{
				ref: imagev1.DockerImageReference{
					Namespace: "openshift",
					Name:      "ruby",
					Tag:       "2.0",
				},
				referencePolicy: SourceReferencePolicy,
				sourceKind:      "ImageStreamTag",
				destNamespace:   []string{"yourproject"},
				destNameAndTag:  []string{"rails:tip"},
			},
			expectedActions: []testAction{
				{verb: "update", resource: "imagestreamtags"},
				{verb: "create", resource: "imagestreamtags"},
				{verb: "get", resource: "imagestreams"},
				{verb: "update", resource: "imagestreams"},
			},
		},
	}

	for name, test := range testCases {
		client := fakeimagev1client.NewSimpleClientset(test.data...)
		client.PrependReactor("create", "imagestreamtags", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, kapierrors.NewMethodNotSupported(image.Resource("imagestreamtags"), "create")
		})
		client.PrependReactor("update", "imagestreamtags", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, kapierrors.NewMethodNotSupported(image.Resource("imagestreamtags"), "update")
		})

		test.opts.IOStreams = genericclioptions.NewTestIOStreamsDiscard()
		test.opts.client = client.Image()

		err := test.opts.Validate()
		if (err == nil && len(test.validateErr) != 0) || (err != nil && err.Error() != test.validateErr) {
			t.Errorf("%s: validation error mismatch: expected %v, got %v", name, test.validateErr, err)
		}
		if err != nil {
			continue
		}

		err = test.opts.Run()
		if (err == nil && len(test.runErr) != 0) || (err != nil && err.Error() != test.runErr) {
			t.Errorf("%s: run error mismatch: expected %v, got %v", name, test.runErr, err)
		}

		got := client.Actions()
		if len(test.expectedActions) != len(got) {
			t.Fatalf("%s: action length mismatch: expected %d, got %d", name, len(test.expectedActions), len(got))
		}
		for i, action := range test.expectedActions {
			if !got[i].Matches(action.verb, action.resource) {
				t.Errorf("%s: [%o] action mismatch: expected %s %s, got %s %s",
					name, i, action.verb, action.resource, got[i].GetVerb(), got[i].GetResource())
			}
		}
	}
}

func TestRunTag_DeleteOld(t *testing.T) {
	streams := testData()
	client := fakeimagev1client.NewSimpleClientset(streams[1])
	client.PrependReactor("delete", "imagestreamtags", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, kapierrors.NewForbidden(image.Resource("imagestreamtags"), "rails:tip", fmt.Errorf("dne"))
	})
	client.PrependReactor("get", "imagestreams", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, testData()[1], nil
	})
	client.PrependReactor("update", "imagestreams", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, nil
	})

	test := struct {
		opts            *TagOptions
		expectedActions []testAction
		expectedErr     error
	}{
		opts: &TagOptions{
			IOStreams:      genericclioptions.NewTestIOStreamsDiscard(),
			client:         client.Image(),
			deleteTag:      true,
			destNamespace:  []string{"yourproject"},
			destNameAndTag: []string{"rails:tip"},
		},
		expectedActions: []testAction{
			{verb: "delete", resource: "imagestreamtags"},
			{verb: "get", resource: "imagestreams"},
			{verb: "update", resource: "imagestreams"},
		},
		expectedErr: nil,
	}

	if err := test.opts.Run(); err != test.expectedErr {
		t.Fatalf("error mismatch: expected %v, got %v", test.expectedErr, err)
	}

	got := client.Actions()
	if len(test.expectedActions) != len(got) {
		t.Fatalf("action length mismatch: expectedc %d, got %d", len(test.expectedActions), len(got))
	}

	for i, action := range test.expectedActions {
		if !got[i].Matches(action.verb, action.resource) {
			t.Errorf("action mismatch: expected %s %s, got %s %s", action.verb, action.resource, got[i].GetVerb(), got[i].GetResource())
		}
	}
}

func TestRunTag_AddNew(t *testing.T) {
	client := fakeimagev1client.NewSimpleClientset(
		&imagev1.ImageStreamTag{
			ObjectMeta: metav1.ObjectMeta{Name: "rails:tip", Namespace: "yourproject", ResourceVersion: "10", CreationTimestamp: metav1.Now()},
		},
	)

	test := struct {
		opts            *TagOptions
		expectedActions []testAction
		expectedErr     error
	}{
		opts: &TagOptions{
			IOStreams: genericclioptions.NewTestIOStreamsDiscard(),
			client:    client.Image(),
			ref: imagev1.DockerImageReference{
				Namespace: "openshift",
				Name:      "ruby",
				Tag:       "2.0",
			},
			sourceKind:     "ImageStreamTag",
			destNamespace:  []string{"yourproject"},
			destNameAndTag: []string{"rails:tip"},
		},
		expectedActions: []testAction{
			{verb: "update", resource: "imagestreamtags"},
		},
		expectedErr: nil,
	}

	if err := test.opts.Run(); err != test.expectedErr {
		t.Fatalf("error mismatch: expected %v, got %v", test.expectedErr, err)
	}

	got := client.Actions()
	if len(test.expectedActions) != len(got) {
		t.Fatalf("action length mismatch: expectedc %d, got %d", len(test.expectedActions), len(got))
	}

	for i, action := range test.expectedActions {
		if !got[i].Matches(action.verb, action.resource) {
			t.Errorf("action mismatch: expected %s %s, got %s %s", action.verb, action.resource, got[i].GetVerb(), got[i].GetResource())
		}
	}
}

func TestRunTag_AddRestricted(t *testing.T) {
	client := fakeimagev1client.NewSimpleClientset()
	client.PrependReactor("create", "imagestreamtags", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, action.(clientgotesting.CreateAction).GetObject(), nil
	})
	client.PrependReactor("update", "imagestreamtags", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, kapierrors.NewForbidden(image.Resource("imagestreamtags"), "rails:tip", fmt.Errorf("dne"))
	})

	test := struct {
		opts            *TagOptions
		expectedActions []testAction
		expectedErr     error
	}{
		opts: &TagOptions{
			IOStreams: genericclioptions.NewTestIOStreamsDiscard(),
			client:    client.Image(),
			ref: imagev1.DockerImageReference{
				Namespace: "openshift",
				Name:      "ruby",
				Tag:       "2.0",
			},
			sourceKind:     "ImageStreamTag",
			destNamespace:  []string{"yourproject"},
			destNameAndTag: []string{"rails:tip"},
		},
		expectedActions: []testAction{
			{verb: "update", resource: "imagestreamtags"},
			{verb: "create", resource: "imagestreamtags"},
		},
		expectedErr: nil,
	}

	if err := test.opts.Run(); err != test.expectedErr {
		t.Fatalf("error mismatch: expected %v, got %v", test.expectedErr, err)
	}

	got := client.Actions()
	if len(test.expectedActions) != len(got) {
		t.Fatalf("action length mismatch: expectedc %d, got %d", len(test.expectedActions), len(got))
	}

	for i, action := range test.expectedActions {
		if !got[i].Matches(action.verb, action.resource) {
			t.Errorf("action mismatch: expected %s %s, got %s %s", action.verb, action.resource, got[i].GetVerb(), got[i].GetResource())
		}
	}
}

func TestRunTag_DeleteNew(t *testing.T) {
	is := &imagev1.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{Name: "rails:tip", Namespace: "yourproject", ResourceVersion: "11", CreationTimestamp: metav1.Now()},
	}
	client := fakeimagev1client.NewSimpleClientset(is)

	test := struct {
		opts            *TagOptions
		expectedActions []testAction
		expectedErr     error
	}{
		opts: &TagOptions{
			IOStreams:      genericclioptions.NewTestIOStreamsDiscard(),
			client:         client.Image(),
			deleteTag:      true,
			destNamespace:  []string{"yourproject"},
			destNameAndTag: []string{"rails:tip"},
		},
		expectedActions: []testAction{
			{verb: "delete", resource: "imagestreamtags"},
		},
		expectedErr: nil,
	}

	if err := test.opts.Run(); err != test.expectedErr {
		t.Fatalf("error mismatch: expected %v, got %v", test.expectedErr, err)
	}

	got := client.Actions()
	if len(test.expectedActions) != len(got) {
		t.Fatalf("action length mismatch: expectedc %d, got %d", len(test.expectedActions), len(got))
	}

	for i, action := range test.expectedActions {
		if !got[i].Matches(action.verb, action.resource) {
			t.Errorf("action mismatch: expected %s %s, got %s %s", action.verb, action.resource, got[i].GetVerb(), got[i].GetResource())
		}
	}
}
