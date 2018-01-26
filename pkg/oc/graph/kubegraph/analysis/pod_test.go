package analysis

import (
	"sort"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	osgraph "github.com/openshift/origin/pkg/oc/graph/genericgraph"
	osgraphtest "github.com/openshift/origin/pkg/oc/graph/genericgraph/test"
)

func TestRestartingPodWarning(t *testing.T) {
	g, _, err := osgraphtest.BuildGraph("../../../graph/genericgraph/test/restarting-pod.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { nowFn = metav1.Now }()

	recent, _ := time.Parse(time.RFC3339, "2015-07-13T19:36:06Z")
	nowFn = func() metav1.Time { return metav1.NewTime(recent.UTC()) }
	markers := FindRestartingPods(g, osgraph.DefaultNamer, "oc logs", "oc adm policy")
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
	nowFn = func() metav1.Time { return metav1.NewTime(future.UTC()) }
	markers = FindRestartingPods(g, osgraph.DefaultNamer, "oc logs", "oc adm policy")
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
