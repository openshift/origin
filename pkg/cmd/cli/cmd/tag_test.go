package cmd

import (
	"fmt"
	"os"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	ktc "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type testAction struct {
	verb, resource string
}

func testData() []*imageapi.ImageStream {
	return []*imageapi.ImageStream{
		{
			ObjectMeta: api.ObjectMeta{Name: "rails", Namespace: "yourproject", ResourceVersion: "10", CreationTimestamp: unversioned.Now()},
			Spec: imageapi.ImageStreamSpec{
				DockerImageRepository: "",
				Tags: map[string]imageapi.TagReference{},
			},
		},
		{
			ObjectMeta: api.ObjectMeta{Name: "rails", Namespace: "yourproject", ResourceVersion: "11", CreationTimestamp: unversioned.Now()},
			Spec: imageapi.ImageStreamSpec{
				DockerImageRepository: "",
				Tags: map[string]imageapi.TagReference{
					"tip": {
						From: &api.ObjectReference{
							Name:      "ruby",
							Namespace: "openshift",
							Kind:      "ImageStreamTag",
						},
					},
				},
			},
		},
		{
			ObjectMeta: api.ObjectMeta{Name: "rails", Namespace: "myproject", ResourceVersion: "10", CreationTimestamp: unversioned.Now()},
			Spec: imageapi.ImageStreamSpec{
				DockerImageRepository: "",
				Tags: map[string]imageapi.TagReference{
					"latest": {
						From: &api.ObjectReference{
							Name:      "ruby",
							Namespace: "openshift",
							Kind:      "ImageStreamTag",
						},
					},
				},
			},
		},
	}
}

func TestRunTag_AddAccrossNamespaces(t *testing.T) {
	streams := testData()
	client := testclient.NewSimpleFake(streams[2], streams[0])
	client.PrependReactor("create", "imagestreamtags", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, kapierrors.NewMethodNotSupported(imageapi.Resource("imagestreamtags"), "create")
	})
	client.PrependReactor("update", "imagestreamtags", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, kapierrors.NewMethodNotSupported(imageapi.Resource("imagestreamtags"), "update")
	})

	test := struct {
		opts            *TagOptions
		expectedActions []testAction
		expectedErr     error
	}{
		opts: &TagOptions{
			out:      os.Stdout,
			osClient: client,
			ref: imageapi.DockerImageReference{
				Namespace: "openshift",
				Name:      "ruby",
				Tag:       "latest",
			},
			namespace:      "myproject2",
			sourceKind:     "ImageStreamTag",
			destNamespace:  []string{"yourproject"},
			destNameAndTag: []string{"rails:tip"},
		},
		expectedActions: []testAction{
			{verb: "update", resource: "imagestreamtags"},
			{verb: "create", resource: "imagestreamtags"},
			{verb: "get", resource: "imagestreams"},
			{verb: "update", resource: "imagestreams"},
		},
		expectedErr: nil,
	}

	if err := test.opts.RunTag(); err != test.expectedErr {
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

func TestRunTag_AddOld(t *testing.T) {
	streams := testData()
	client := testclient.NewSimpleFake(streams[0])
	client.PrependReactor("create", "imagestreamtags", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, kapierrors.NewMethodNotSupported(imageapi.Resource("imagestreamtags"), "create")
	})
	client.PrependReactor("update", "imagestreamtags", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, kapierrors.NewMethodNotSupported(imageapi.Resource("imagestreamtags"), "update")
	})

	test := struct {
		opts            *TagOptions
		expectedActions []testAction
		expectedErr     error
	}{
		opts: &TagOptions{
			out:      os.Stdout,
			osClient: client,
			ref: imageapi.DockerImageReference{
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
			{verb: "get", resource: "imagestreams"},
			{verb: "update", resource: "imagestreams"},
		},
		expectedErr: nil,
	}

	if err := test.opts.RunTag(); err != test.expectedErr {
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

func TestRunTag_DeleteOld(t *testing.T) {
	streams := testData()
	client := testclient.NewSimpleFake(streams[1])
	client.PrependReactor("delete", "imagestreamtags", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, kapierrors.NewForbidden(imageapi.Resource("imagestreamtags"), "rails:tip", fmt.Errorf("dne"))
	})
	client.PrependReactor("get", "imagestreams", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
		return true, testData()[1], nil
	})
	client.PrependReactor("update", "imagestreams", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, nil
	})

	test := struct {
		opts            *TagOptions
		expectedActions []testAction
		expectedErr     error
	}{
		opts: &TagOptions{
			out:            os.Stdout,
			osClient:       client,
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

	if err := test.opts.RunTag(); err != test.expectedErr {
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
	client := testclient.NewSimpleFake(
		&imageapi.ImageStreamTag{
			ObjectMeta: api.ObjectMeta{Name: "rails:tip", Namespace: "yourproject", ResourceVersion: "10", CreationTimestamp: unversioned.Now()},
		},
	)

	test := struct {
		opts            *TagOptions
		expectedActions []testAction
		expectedErr     error
	}{
		opts: &TagOptions{
			out:      os.Stdout,
			osClient: client,
			ref: imageapi.DockerImageReference{
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

	if err := test.opts.RunTag(); err != test.expectedErr {
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
	client := testclient.NewSimpleFake()
	client.PrependReactor("create", "imagestreamtags", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
		return true, action.(ktc.CreateAction).GetObject(), nil
	})
	client.PrependReactor("update", "imagestreamtags", func(action ktc.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, kapierrors.NewForbidden(imageapi.Resource("imagestreamtags"), "rails:tip", fmt.Errorf("dne"))
	})

	test := struct {
		opts            *TagOptions
		expectedActions []testAction
		expectedErr     error
	}{
		opts: &TagOptions{
			out:      os.Stdout,
			osClient: client,
			ref: imageapi.DockerImageReference{
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

	if err := test.opts.RunTag(); err != test.expectedErr {
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
	is := &imageapi.ImageStreamTag{
		ObjectMeta: api.ObjectMeta{Name: "rails:tip", Namespace: "yourproject", ResourceVersion: "11", CreationTimestamp: unversioned.Now()},
	}
	client := testclient.NewSimpleFake(is)

	test := struct {
		opts            *TagOptions
		expectedActions []testAction
		expectedErr     error
	}{
		opts: &TagOptions{
			out:            os.Stdout,
			osClient:       client,
			deleteTag:      true,
			destNamespace:  []string{"yourproject"},
			destNameAndTag: []string{"rails:tip"},
		},
		expectedActions: []testAction{
			{verb: "delete", resource: "imagestreamtags"},
		},
		expectedErr: nil,
	}

	if err := test.opts.RunTag(); err != test.expectedErr {
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
