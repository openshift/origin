package cmd

import (
	"io/ioutil"
	"strconv"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/client/testclient"
)

// TestCancelBuildDefaultFlags ensures that flags default values are set.
func TestCancelBuildDefaultFlags(t *testing.T) {
	o := CancelBuildOptions{}

	tests := map[string]struct {
		flagName   string
		defaultVal string
	}{
		"state": {
			flagName:   "state",
			defaultVal: "[" + strings.Join(o.States, ",") + "]",
		},
		"dump-logs": {
			flagName:   "dump-logs",
			defaultVal: strconv.FormatBool(o.DumpLogs),
		},
		"restart": {
			flagName:   "restart",
			defaultVal: strconv.FormatBool(o.Restart),
		},
	}

	cmd := NewCmdCancelBuild("oc", CancelBuildRecommendedCommandName, nil, nil, nil, nil)

	for _, v := range tests {
		f := cmd.Flag(v.flagName)
		if f == nil {
			t.Fatalf("expected flag %s to be registered but found none", v.flagName)
		}

		if f.DefValue != v.defaultVal {
			t.Errorf("expected default value of %s for %s but found %s", v.defaultVal, v.flagName, f.DefValue)
		}
	}
}

// TestCancelBuildRun ensures that RunCancelBuild command calls the right actions.
func TestCancelBuildRun(t *testing.T) {
	tests := map[string]struct {
		opts            *CancelBuildOptions
		phase           buildapi.BuildPhase
		expectedActions []testAction
		expectedErr     error
	}{
		"cancelled": {
			opts: &CancelBuildOptions{
				Out:       ioutil.Discard,
				Namespace: "test",
				States:    []string{"new", "pending", "running"},
			},
			phase: buildapi.BuildPhaseCancelled,
			expectedActions: []testAction{
				{verb: "get", resource: "builds"},
			},
			expectedErr: nil,
		},
		"complete": {
			opts: &CancelBuildOptions{
				Out:       ioutil.Discard,
				Namespace: "test",
			},
			phase: buildapi.BuildPhaseComplete,
			expectedActions: []testAction{
				{verb: "get", resource: "builds"},
			},
			expectedErr: nil,
		},
		"new": {
			opts: &CancelBuildOptions{
				Out:       ioutil.Discard,
				Namespace: "test",
			},
			phase: buildapi.BuildPhaseNew,
			expectedActions: []testAction{
				{verb: "get", resource: "builds"},
				{verb: "update", resource: "builds"},
				{verb: "get", resource: "builds"},
			},
			expectedErr: nil,
		},
		"pending": {
			opts: &CancelBuildOptions{
				Out:       ioutil.Discard,
				Namespace: "test",
			},
			phase: buildapi.BuildPhaseNew,
			expectedActions: []testAction{
				{verb: "get", resource: "builds"},
				{verb: "update", resource: "builds"},
				{verb: "get", resource: "builds"},
			},
			expectedErr: nil,
		},
		"running and restart": {
			opts: &CancelBuildOptions{
				Out:       ioutil.Discard,
				Namespace: "test",
				Restart:   true,
			},
			phase: buildapi.BuildPhaseNew,
			expectedActions: []testAction{
				{verb: "get", resource: "builds"},
				{verb: "update", resource: "builds"},
				{verb: "get", resource: "builds"},
				{verb: "create", resource: "builds"},
			},
			expectedErr: nil,
		},
	}

	for testName, test := range tests {
		build := genBuild(test.phase)
		// FIXME: we have to fake out a BuildRequest so the fake client will let us
		// pass this test. It considers 'create builds/clone' to be an update on the
		// main resource (builds), but uses the resource from the clone function,
		// which is a BuildRequest. It needs to be able to "update"/"get" a
		// BuildRequest, so we stub one out here.
		stubbedBuildRequest := &buildapi.BuildRequest{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: test.opts.Namespace,
				Name:      build.Name,
			},
		}
		client := testclient.NewSimpleFake(build, stubbedBuildRequest)
		buildClient := NewFakeTestBuilds(client, test.opts.Namespace)

		test.opts.Client = client
		test.opts.BuildClient = buildClient
		test.opts.ReportError = func(err error) {
			test.opts.HasError = true
		}
		test.opts.Mapper = kapi.Registry.RESTMapper()
		test.opts.BuildNames = []string{"ruby-ex"}
		test.opts.States = []string{"new", "pending", "running"}

		if err := test.opts.RunCancelBuild(); err != test.expectedErr {
			t.Fatalf("%s: error mismatch: expected %v, got %v", testName, test.expectedErr, err)
		}

		got := test.opts.Client.(*testclient.Fake).Actions()
		if len(test.expectedActions) != len(got) {
			t.Fatalf("%s: action length mismatch: expected %d, got %d", testName, len(test.expectedActions), len(got))
		}

		for i, action := range test.expectedActions {
			if !got[i].Matches(action.verb, action.resource) {
				t.Errorf("%s: action mismatch: expected %s %s, got %s %s", testName, action.verb, action.resource, got[i].GetVerb(), got[i].GetResource())
			}
		}
	}

}

type FakeTestBuilds struct {
	*testclient.FakeBuilds
	Obj *buildapi.Build
}

func NewFakeTestBuilds(c *testclient.Fake, ns string) *FakeTestBuilds {
	f := FakeTestBuilds{
		FakeBuilds: &testclient.FakeBuilds{
			Fake:      c,
			Namespace: ns,
		},
	}

	return &f
}

func (c *FakeTestBuilds) Get(name string, options metav1.GetOptions) (*buildapi.Build, error) {
	obj, err := c.FakeBuilds.Get(name, options)
	if c.Obj == nil {
		c.Obj = obj
	}

	return c.Obj, err
}

func (c *FakeTestBuilds) Update(inObj *buildapi.Build) (*buildapi.Build, error) {
	_, err := c.FakeBuilds.Update(inObj)
	if inObj.Status.Cancelled == true {
		inObj.Status.Phase = buildapi.BuildPhaseCancelled
	}

	c.Obj = inObj
	return c.Obj, err
}

func genBuild(phase buildapi.BuildPhase) *buildapi.Build {
	build := buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ruby-ex",
			Namespace: "test",
		},
		Status: buildapi.BuildStatus{
			Phase: phase,
		},
	}
	return &build
}
