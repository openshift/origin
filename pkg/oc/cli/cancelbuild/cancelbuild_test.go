package cancelbuild

import (
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	buildv1 "github.com/openshift/api/build/v1"
	buildfake "github.com/openshift/client-go/build/clientset/versioned/fake"
)

// TestCancelBuildDefaultFlags ensures that flags default values are set.
func TestCancelBuildDefaultFlags(t *testing.T) {
	o := NewCancelBuildOptions(genericclioptions.NewTestIOStreamsDiscard())

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

	cmd := NewCmdCancelBuild("oc", CancelBuildRecommendedCommandName, nil, genericclioptions.NewTestIOStreamsDiscard())

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
		phase           buildv1.BuildPhase
		expectedActions []testAction
		expectedErr     error
	}{
		"cancelled": {
			opts: &CancelBuildOptions{
				PrinterCancel:           &discardingPrinter{},
				PrinterCancelInProgress: &discardingPrinter{},
				PrinterRestart:          &discardingPrinter{},
				IOStreams:               genericclioptions.NewTestIOStreamsDiscard(),
				Namespace:               "test",
				States:                  []string{"new", "pending", "running"},
			},
			phase: buildv1.BuildPhaseCancelled,
			expectedActions: []testAction{
				{verb: "get", resource: "builds"},
			},
			expectedErr: nil,
		},
		"complete": {
			opts: &CancelBuildOptions{
				PrinterCancel:           &discardingPrinter{},
				PrinterCancelInProgress: &discardingPrinter{},
				PrinterRestart:          &discardingPrinter{},
				IOStreams:               genericclioptions.NewTestIOStreamsDiscard(),
				Namespace:               "test",
			},
			phase: buildv1.BuildPhaseComplete,
			expectedActions: []testAction{
				{verb: "get", resource: "builds"},
			},
			expectedErr: nil,
		},
		"new": {
			opts: &CancelBuildOptions{
				PrinterCancel:           &discardingPrinter{},
				PrinterCancelInProgress: &discardingPrinter{},
				PrinterRestart:          &discardingPrinter{},
				IOStreams:               genericclioptions.NewTestIOStreamsDiscard(),
				Namespace:               "test",
			},
			phase: buildv1.BuildPhaseNew,
			expectedActions: []testAction{
				{verb: "get", resource: "builds"},
				{verb: "update", resource: "builds"},
				{verb: "get", resource: "builds"},
			},
			expectedErr: nil,
		},
		"pending": {
			opts: &CancelBuildOptions{
				PrinterCancel:           &discardingPrinter{},
				PrinterCancelInProgress: &discardingPrinter{},
				PrinterRestart:          &discardingPrinter{},
				IOStreams:               genericclioptions.NewTestIOStreamsDiscard(),
				Namespace:               "test",
			},
			phase: buildv1.BuildPhaseNew,
			expectedActions: []testAction{
				{verb: "get", resource: "builds"},
				{verb: "update", resource: "builds"},
				{verb: "get", resource: "builds"},
			},
			expectedErr: nil,
		},
		"running and restart": {
			opts: &CancelBuildOptions{
				PrinterCancel:           &discardingPrinter{},
				PrinterCancelInProgress: &discardingPrinter{},
				PrinterRestart:          &discardingPrinter{},
				IOStreams:               genericclioptions.NewTestIOStreamsDiscard(),
				Namespace:               "test",
				Restart:                 true,
			},
			phase: buildv1.BuildPhaseNew,
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
		stubbedBuildRequest := &buildv1.BuildRequest{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: test.opts.Namespace,
				Name:      build.Name,
			},
		}
		client := buildfake.NewSimpleClientset(build, stubbedBuildRequest)
		client.PrependReactor("get", "builds", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, build, nil
		})
		client.PrependReactor("update", "builds", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			if build.Status.Cancelled == true {
				build.Status.Phase = buildv1.BuildPhaseCancelled
			}
			return false, build, nil
		})
		client.PrependReactor("create", "builds", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			if action.GetSubresource() != "clone" {
				return false, nil, nil
			}
			return true, build, nil
		})

		test.opts.timeout = 1 * time.Second
		test.opts.Client = client.BuildV1()
		test.opts.BuildClient = client.Build().Builds(test.opts.Namespace)
		test.opts.ReportError = func(err error) {
			test.opts.HasError = true
			t.Logf("got error: %v", err)
		}
		test.opts.Mapper = testrestmapper.TestOnlyStaticRESTMapper(legacyscheme.Scheme)
		test.opts.BuildNames = []string{"ruby-ex"}
		test.opts.States = []string{"new", "pending", "running"}

		if err := test.opts.RunCancelBuild(); err != test.expectedErr {
			t.Fatalf("%s: error mismatch: expected %v, got %v", testName, test.expectedErr, err)
		}

		got := client.Actions()
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

type discardingPrinter struct{}

func (*discardingPrinter) PrintObj(runtime.Object, io.Writer) error {
	return nil
}

func genBuild(phase buildv1.BuildPhase) *buildv1.Build {
	build := buildv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ruby-ex",
			Namespace: "test",
		},
		Status: buildv1.BuildStatus{
			Phase: phase,
		},
	}
	return &build
}

type testAction struct {
	verb, resource string
}
