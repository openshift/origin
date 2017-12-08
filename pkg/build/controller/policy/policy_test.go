package policy

import (
	"errors"
	"strings"
	"testing"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildlister "github.com/openshift/origin/pkg/build/generated/listers/build/internalversion"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

type fakeBuildClient struct {
	builds         *buildapi.BuildList
	updateErrCount int
}

func newTestClient(builds []buildapi.Build) *fakeBuildClient {
	return &fakeBuildClient{builds: &buildapi.BuildList{Items: builds}}
}

func (f *fakeBuildClient) List(namespace string, opts metav1.ListOptions) (*buildapi.BuildList, error) {
	return f.builds, nil
}

func (f *fakeBuildClient) Update(namespace string, build *buildapi.Build) error {
	// Make sure every update fails at least once with conflict to ensure build updates are
	// retried.
	if f.updateErrCount == 0 {
		f.updateErrCount = 1
		return kerrors.NewConflict(kapi.Resource("builds"), build.Name, errors.New("confict"))
	} else {
		f.updateErrCount = 0
	}
	for i, item := range f.builds.Items {
		if build.Name == item.Name {
			f.builds.Items[i] = *build
		}
	}
	return nil
}

func (f *fakeBuildClient) Lister() buildlister.BuildLister {
	return &fakeBuildLister{f: f}
}

type fakeBuildLister struct {
	f *fakeBuildClient
}

func (f *fakeBuildLister) List(label labels.Selector) ([]*buildapi.Build, error) {
	var items []*buildapi.Build
	for i := range f.f.builds.Items {
		items = append(items, &f.f.builds.Items[i])
	}
	return items, nil
}

func (f *fakeBuildLister) Get(name string) (*buildapi.Build, error) {
	for i := range f.f.builds.Items {
		if f.f.builds.Items[i].Name == name {
			return &f.f.builds.Items[i], nil
		}
	}
	return nil, kerrors.NewNotFound(schema.GroupResource{Resource: "builds"}, name)
}

func (f *fakeBuildLister) Builds(ns string) buildlister.BuildNamespaceLister {
	return f
}

func addBuild(name, bcName string, phase buildapi.BuildPhase, policy buildapi.BuildRunPolicy) buildapi.Build {
	parts := strings.Split(name, "-")
	return buildapi.Build{
		Spec: buildapi.BuildSpec{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test",
			Labels: map[string]string{
				buildapi.BuildRunPolicyLabel: string(policy),
				buildapi.BuildConfigLabel:    bcName,
			},
			Annotations: map[string]string{
				buildapi.BuildNumberAnnotation: parts[len(parts)-1],
			},
		},
		Status: buildapi.BuildStatus{Phase: phase},
	}
}

func TestForBuild(t *testing.T) {
	builds := []buildapi.Build{
		addBuild("build-1", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicyParallel),
		addBuild("build-2", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicySerial),
		addBuild("build-3", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicySerialLatestOnly),
	}
	client := newTestClient(builds)
	policies := GetAllRunPolicies(client.Lister(), client)

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
	builds := []buildapi.Build{
		addBuild("build-1", "sample-bc", buildapi.BuildPhaseComplete, buildapi.BuildRunPolicySerial),
		addBuild("build-2", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicySerial),
		addBuild("build-3", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicySerial),
	}

	client := newTestClient(builds)

	resultBuilds, isRunning, err := GetNextConfigBuild(client.Lister(), "namespace", "bc")
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
	builds := []buildapi.Build{
		addBuild("build-1", "sample-bc", buildapi.BuildPhaseComplete, buildapi.BuildRunPolicyParallel),
		addBuild("build-2", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicyParallel),
		addBuild("build-3", "sample-bc", buildapi.BuildPhaseNew, buildapi.BuildRunPolicyParallel),
	}

	client := newTestClient(builds)

	resultBuilds, running, err := GetNextConfigBuild(client.Lister(), "namespace", "bc")
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
