package policy

import (
	"strings"
	"testing"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	buildv1 "github.com/openshift/api/build/v1"
	"github.com/openshift/client-go/build/clientset/versioned/fake"
	v1 "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	buildlister "github.com/openshift/client-go/build/listers/build/v1"

	buildutil "github.com/openshift/origin/pkg/build/util"
)

func newTestClient(builds ...buildv1.Build) v1.BuildsGetter {
	startingObjects := []runtime.Object{}
	for i := range builds {
		startingObjects = append(startingObjects, &builds[i])
	}
	return fake.NewSimpleClientset(startingObjects...).BuildV1()
}

type fakeBuildLister struct {
	f v1.BuildsGetter
}

func (f *fakeBuildLister) List(label labels.Selector) ([]*buildv1.Build, error) {
	var items []*buildv1.Build
	builds, err := f.f.Builds("test").List(metav1.ListOptions{LabelSelector: label.String()})
	if err != nil {
		return nil, err
	}
	for i := range builds.Items {
		items = append(items, &builds.Items[i])
	}
	return items, nil
}

func (f *fakeBuildLister) Get(name string) (*buildv1.Build, error) {
	builds, err := f.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	for i := range builds {
		if builds[i].Name == name {
			return builds[i], nil
		}
	}
	return nil, kerrors.NewNotFound(schema.GroupResource{Resource: "builds"}, name)
}

func (f *fakeBuildLister) Builds(ns string) buildlister.BuildNamespaceLister {
	return f
}

func addBuild(name, bcName string, phase buildv1.BuildPhase, policy buildv1.BuildRunPolicy) buildv1.Build {
	parts := strings.Split(name, "-")
	return buildv1.Build{
		Spec: buildv1.BuildSpec{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test",
			Labels: map[string]string{
				buildutil.BuildRunPolicyLabel: string(policy),
				buildutil.BuildConfigLabel:    bcName,
			},
			Annotations: map[string]string{
				buildutil.BuildNumberAnnotation: parts[len(parts)-1],
			},
		},
		Status: buildv1.BuildStatus{Phase: phase},
	}
}

func TestForBuild(t *testing.T) {
	builds := []buildv1.Build{
		addBuild("build-1", "sample-bc", buildv1.BuildPhaseNew, buildv1.BuildRunPolicyParallel),
		addBuild("build-2", "sample-bc", buildv1.BuildPhaseNew, buildv1.BuildRunPolicySerial),
		addBuild("build-3", "sample-bc", buildv1.BuildPhaseNew, buildv1.BuildRunPolicySerialLatestOnly),
	}
	client := newTestClient(builds...)
	lister := &fakeBuildLister{f: client}

	policies := GetAllRunPolicies(lister, client)

	if policy := ForBuild(&builds[0], policies); policy != nil {
		if _, ok := policy.(*ParallelPolicy); !ok {
			t.Errorf("expected Parallel policy for build-1, got %T", policy)
		}
	} else {
		t.Errorf("expected Parallel policy for build-1, got nil")
	}

	if policy := ForBuild(&builds[1], policies); policy != nil {
		if _, ok := policy.(*SerialPolicy); !ok {
			t.Errorf("expected Serial policy for build-2, got %T", policy)
		}
	} else {
		t.Errorf("expected Serial policy for build-2, got nil")
	}

	if policy := ForBuild(&builds[2], policies); policy != nil {
		if _, ok := policy.(*SerialLatestOnlyPolicy); !ok {
			t.Errorf("expected SerialLatestOnly policy for build-3, got %T", policy)
		}
	} else {
		t.Errorf("expected SerialLatestOnly policy for build-3, got nil")
	}
}

func TestGetNextConfigBuildSerial(t *testing.T) {
	builds := []buildv1.Build{
		addBuild("build-1", "sample-bc", buildv1.BuildPhaseComplete, buildv1.BuildRunPolicySerial),
		addBuild("build-2", "sample-bc", buildv1.BuildPhaseNew, buildv1.BuildRunPolicySerial),
		addBuild("build-3", "sample-bc", buildv1.BuildPhaseNew, buildv1.BuildRunPolicySerial),
	}
	client := newTestClient(builds...)
	lister := &fakeBuildLister{f: client}

	resultBuilds, isRunning, err := GetNextConfigBuild(lister, "test", "sample-bc")
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if isRunning {
		t.Errorf("expected no running builds")
	}

	if len(resultBuilds) != 1 {
		t.Errorf("expecting single result build, got %d", len(resultBuilds))
		return
	}

	if resultBuilds[0].Name != "build-2" {
		t.Errorf("expected result build to be build-2, got %s", resultBuilds[0].Name)
	}
}

func TestGetNextConfigBuildParallel(t *testing.T) {
	builds := []buildv1.Build{
		addBuild("build-1", "sample-bc", buildv1.BuildPhaseComplete, buildv1.BuildRunPolicyParallel),
		addBuild("build-2", "sample-bc", buildv1.BuildPhaseNew, buildv1.BuildRunPolicyParallel),
		addBuild("build-3", "sample-bc", buildv1.BuildPhaseNew, buildv1.BuildRunPolicyParallel),
	}

	client := newTestClient(builds...)
	lister := &fakeBuildLister{f: client}

	resultBuilds, running, err := GetNextConfigBuild(lister, "test", "sample-bc")
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if running {
		t.Errorf("expected no running builds")
	}

	if len(resultBuilds) != 2 {
		t.Errorf("expecting 2 result builds, got %d", len(resultBuilds))
		return
	}

	includesBuild2 := false
	includesBuild3 := false

	for _, build := range resultBuilds {
		if build.Name == "build-2" {
			includesBuild2 = true
		}
		if build.Name == "build-3" {
			includesBuild3 = true
		}
	}

	if !includesBuild2 || !includesBuild3 {
		t.Errorf("build-2 and build-3 should be included in the result, got %#v", resultBuilds)
	}
}
