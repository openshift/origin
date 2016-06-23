package analysis

import (
	"sort"
	"strings"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api/unversioned"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	osgraphtest "github.com/openshift/origin/pkg/api/graph/test"
)

func TestRestartingPodWarning(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../api/graph/test/restarting-pod.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { nowFn = unversioned.Now }()

	recent, _ := time.Parse(time.RFC3339, "2015-07-13T19:36:06Z")
	nowFn = func() unversioned.Time { return unversioned.NewTime(recent.UTC()) }
	markers := FindRestartingPods(g, osgraph.DefaultNamer, "oc logs", "oadm policy")
	sort.Sort(osgraph.BySeverity(markers))
	if e, a := 4, len(markers); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}
	if e, a := CrashLoopingPodError, markers[0].Key; e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}
	if e, a := CrashLoopingPodError, markers[1].Key; e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}
	if e, a := RestartingPodWarning, markers[2].Key; e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}
	if e, a := RestartingPodWarning, markers[3].Key; e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}

	sort.Sort(osgraph.ByNodeID(markers))
	if !strings.HasPrefix(markers[0].Message, "container ") {
		t.Fatalf("message %q should state container", markers[0].Message)
	}
	if !strings.HasPrefix(markers[1].Message, "container ") {
		t.Fatalf("message %q should state container", markers[1].Message)
	}
	if !strings.HasPrefix(markers[2].Message, "container ") {
		t.Fatalf("message %q should state container", markers[2].Message)
	}
	if strings.HasPrefix(markers[3].Message, "container ") {
		t.Fatalf("message %q should not state container", markers[3].Message)
	}

	future, _ := time.Parse(time.RFC3339, "2015-07-13T19:46:06Z")
	nowFn = func() unversioned.Time { return unversioned.NewTime(future.UTC()) }
	markers = FindRestartingPods(g, osgraph.DefaultNamer, "oc logs", "oadm policy")
	sort.Sort(osgraph.BySeverity(markers))
	if e, a := 3, len(markers); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}
	if e, a := CrashLoopingPodError, markers[0].Key; e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}
	if e, a := CrashLoopingPodError, markers[1].Key; e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}
	if e, a := RestartingPodWarning, markers[2].Key; e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}

	sort.Sort(osgraph.ByNodeID(markers))
	if !strings.HasPrefix(markers[0].Message, "container ") {
		t.Fatalf("message %q should state container", markers[0].Message)
	}
	if !strings.HasPrefix(markers[1].Message, "container ") {
		t.Fatalf("message %q should state container", markers[1].Message)
	}
	if strings.HasPrefix(markers[2].Message, "container ") {
		t.Fatalf("message %q should not state container", markers[2].Message)
	}
}
