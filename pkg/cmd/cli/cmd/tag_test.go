package cmd

import (
	"os"
	"testing"

	"k8s.io/kubernetes/pkg/api"
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
	}
}

func TestRunTag_Add(t *testing.T) {
	streams := testData()
	client := testclient.NewSimpleFake(streams[0])

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

func TestRunTag_Delete(t *testing.T) {
	streams := testData()
	client := testclient.NewSimpleFake(streams[1])
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
