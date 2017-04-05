package buildpod

import (
	"errors"
	"reflect"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	"github.com/openshift/origin/pkg/build/controller/common"
	"github.com/openshift/origin/pkg/client/testclient"
)

type errBuildUpdater struct{}

func (ec *errBuildUpdater) Update(namespace string, build *buildapi.Build) error {
	return errors.New("UpdateBuild error!")
}

type customBuildUpdater struct {
	UpdateFunc func(namespace string, build *buildapi.Build) error
}

func (c *customBuildUpdater) Update(namespace string, build *buildapi.Build) error {
	return c.UpdateFunc(namespace, build)
}

func mockPod(status kapi.PodPhase, exitCode int) *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "data-build-build",
			Namespace: "namespace",
			Annotations: map[string]string{
				buildapi.BuildAnnotation: "data-build",
			},
		},
		Status: kapi.PodStatus{
			Phase: status,
			ContainerStatuses: []kapi.ContainerStatus{
				{
					State: kapi.ContainerState{
						Terminated: &kapi.ContainerStateTerminated{ExitCode: int32(exitCode)},
					},
				},
			},
		},
	}
}

func mockBuild(phase buildapi.BuildPhase, output buildapi.BuildOutput) *buildapi.Build {
	return &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "data-build",
			Namespace: "namespace",
			Annotations: map[string]string{
				buildapi.BuildConfigAnnotation: "test-bc",
			},
			Labels: map[string]string{
				"name": "dataBuild",
				// TODO: Switch this test to use Serial policy
				buildapi.BuildRunPolicyLabel: string(buildapi.BuildRunPolicyParallel),
				buildapi.BuildConfigLabel:    "test-bc",
			},
		},
		Spec: buildapi.BuildSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: buildapi.BuildSource{
					Git: &buildapi.GitBuildSource{
						URI: "http://my.build.com/the/build/Dockerfile",
					},
					ContextDir: "contextimage",
				},
				Strategy: buildapi.BuildStrategy{
					DockerStrategy: &buildapi.DockerBuildStrategy{},
				},
				Output: output,
			},
		},
		Status: buildapi.BuildStatus{
			Phase: phase,
		},
	}
}

type FakeIndexer struct {
	*cache.FakeCustomStore
}

func mockBuildPodController(build *buildapi.Build, buildUpdater buildclient.BuildUpdater) *BuildPodController {
	buildInformer := cache.NewSharedIndexInformer(&cache.ListWatch{}, &buildapi.Build{}, 2*time.Minute, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	podInformer := cache.NewSharedIndexInformer(&cache.ListWatch{}, &kapi.Pod{}, 2*time.Minute, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	fakeSecret := &kapi.Secret{}
	fakeSecret.Name = "fakeSecret"
	fakeSecret.Namespace = "namespace"
	kclient := fake.NewSimpleClientset(fakeSecret)
	osclient := testclient.NewSimpleFake()
	c := NewBuildPodController(buildInformer, podInformer, kclient, osclient)
	if build != nil {
		c.buildStore.Indexer.Add(build)
	}
	if buildUpdater != nil {
		c.buildUpdater = buildUpdater
	}
	return c
}

func TestHandlePod(t *testing.T) {
	type handlePodTest struct {
		matchID             bool
		inStatus            buildapi.BuildPhase
		outStatus           buildapi.BuildPhase
		startTimestamp      *unversioned.Time
		completionTimestamp *unversioned.Time
		podStatus           kapi.PodPhase
		exitCode            int
		buildUpdater        buildclient.BuildUpdater
	}

	dummy := unversioned.Now()
	curtime := &dummy
	tests := []handlePodTest{
		{ // 0
			matchID:             false,
			inStatus:            buildapi.BuildPhasePending,
			outStatus:           buildapi.BuildPhasePending,
			podStatus:           kapi.PodPending,
			exitCode:            0,
			startTimestamp:      nil,
			completionTimestamp: nil,
		},
		{ // 1
			matchID:             true,
			inStatus:            buildapi.BuildPhasePending,
			outStatus:           buildapi.BuildPhasePending,
			podStatus:           kapi.PodPending,
			exitCode:            0,
			startTimestamp:      nil,
			completionTimestamp: nil,
		},
		{ // 2
			matchID:             true,
			inStatus:            buildapi.BuildPhasePending,
			outStatus:           buildapi.BuildPhaseRunning,
			podStatus:           kapi.PodRunning,
			exitCode:            0,
			startTimestamp:      curtime,
			completionTimestamp: nil,
		},
		{ // 3
			matchID:             true,
			inStatus:            buildapi.BuildPhaseRunning,
			outStatus:           buildapi.BuildPhaseComplete,
			podStatus:           kapi.PodSucceeded,
			exitCode:            0,
			startTimestamp:      nil,
			completionTimestamp: curtime,
		},
		{ // 4
			matchID:             true,
			inStatus:            buildapi.BuildPhaseRunning,
			outStatus:           buildapi.BuildPhaseFailed,
			podStatus:           kapi.PodFailed,
			exitCode:            -1,
			startTimestamp:      nil,
			completionTimestamp: curtime,
		},
		{ // 5
			matchID:             true,
			inStatus:            buildapi.BuildPhaseRunning,
			outStatus:           buildapi.BuildPhaseComplete,
			podStatus:           kapi.PodSucceeded,
			exitCode:            0,
			buildUpdater:        &errBuildUpdater{},
			startTimestamp:      nil,
			completionTimestamp: curtime,
		},
		{ // 6
			matchID:             true,
			inStatus:            buildapi.BuildPhaseCancelled,
			outStatus:           buildapi.BuildPhaseCancelled,
			podStatus:           kapi.PodFailed,
			exitCode:            0,
			startTimestamp:      nil,
			completionTimestamp: nil,
		},
	}

	for i, tc := range tests {
		build := mockBuild(tc.inStatus, buildapi.BuildOutput{})
		// Default build updater to retrieve updated build
		if tc.buildUpdater == nil {
			tc.buildUpdater = &customBuildUpdater{
				UpdateFunc: func(namespace string, updatedBuild *buildapi.Build) error {
					build = updatedBuild
					return nil
				},
			}
		}
		ctrl := mockBuildPodController(build, tc.buildUpdater)
		pod := mockPod(tc.podStatus, tc.exitCode)
		if tc.matchID {
			build.Name = "name"
		}

		err := ctrl.HandlePod(pod)

		if tc.buildUpdater != nil && reflect.TypeOf(tc.buildUpdater).Elem().Name() == "errBuildUpdater" {
			if err == nil {
				t.Errorf("(%d) Expected error, got none", i)
			}
			// can't check tc.outStatus because the local build object does get updated
			// in this test (but would not updated in etcd)
			continue
		}
		if build.Status.Phase != tc.outStatus {
			t.Errorf("(%d) Expected %s, got %s!", i, tc.outStatus, build.Status.Phase)
		}
		if tc.inStatus != buildapi.BuildPhaseCancelled && tc.inStatus != buildapi.BuildPhaseComplete && !common.HasBuildPodNameAnnotation(build) {
			t.Errorf("(%d) Build does not have pod name annotation. %#v", i, build)
		}
		if tc.startTimestamp == nil && build.Status.StartTimestamp != nil {
			t.Errorf("(%d) Expected nil start timestamp, got %v!", i, build.Status.StartTimestamp)
		}
		if tc.startTimestamp != nil && build.Status.StartTimestamp == nil {
			t.Errorf("(%d) nil start timestamp!", i)
		}
		if tc.startTimestamp != nil && !tc.startTimestamp.Before(*build.Status.StartTimestamp) && tc.startTimestamp.Time != build.Status.StartTimestamp.Time {
			t.Errorf("(%d) Expected build start timestamp %v to be equal to or later than %v!", i, build.Status.StartTimestamp, tc.startTimestamp)
		}

		if tc.completionTimestamp == nil && build.Status.CompletionTimestamp != nil {
			t.Errorf("(%d) Expected nil completion timestamp, got %v!", i, build.Status.CompletionTimestamp)
		}
		if tc.completionTimestamp != nil && build.Status.CompletionTimestamp == nil {
			t.Errorf("(%d) nil completion timestamp!", i)
		}
		if tc.completionTimestamp != nil && !tc.completionTimestamp.Before(*build.Status.CompletionTimestamp) && tc.completionTimestamp.Time != build.Status.CompletionTimestamp.Time {
			t.Errorf("(%d) Expected build completion timestamp %v to be equal to or later than %v!", i, build.Status.CompletionTimestamp, tc.completionTimestamp)
		}
	}
}

func TestHandleBuildPodDeletionOK(t *testing.T) {
	updateWasCalled := false
	// only not finished build (buildutil.IsBuildComplete) should be handled
	build := mockBuild(buildapi.BuildPhaseRunning, buildapi.BuildOutput{})
	ctrl := mockBuildPodController(build, &customBuildUpdater{
		UpdateFunc: func(namespace string, build *buildapi.Build) error {
			updateWasCalled = true
			return nil
		},
	})
	pod := mockPod(kapi.PodSucceeded, 0)

	err := ctrl.HandleBuildPodDeletion(pod)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if !updateWasCalled {
		t.Error("UpdateBuild was not called when it should have been!")
	}
}

func TestHandlePipelineBuildPodDeletionOK(t *testing.T) {
	updateWasCalled := false
	// only not finished build (buildutil.IsBuildComplete) should be handled
	build := mockBuild(buildapi.BuildPhaseRunning, buildapi.BuildOutput{})
	build.Spec.Strategy.JenkinsPipelineStrategy = &buildapi.JenkinsPipelineBuildStrategy{}
	ctrl := mockBuildPodController(build, &customBuildUpdater{
		UpdateFunc: func(namespace string, build *buildapi.Build) error {
			updateWasCalled = true
			return nil
		},
	})
	pod := mockPod(kapi.PodSucceeded, 0)

	err := ctrl.HandleBuildPodDeletion(pod)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if updateWasCalled {
		t.Error("UpdateBuild called when it should not have been!")
	}
}

func TestHandleBuildPodDeletionOKFinishedBuild(t *testing.T) {
	updateWasCalled := false
	// finished build buildutil.IsBuildComplete should not be handled
	build := mockBuild(buildapi.BuildPhaseComplete, buildapi.BuildOutput{})
	ctrl := mockBuildPodController(build, &customBuildUpdater{
		UpdateFunc: func(namespace string, build *buildapi.Build) error {
			updateWasCalled = true
			return nil
		},
	})
	pod := mockPod(kapi.PodSucceeded, 0)

	err := ctrl.HandleBuildPodDeletion(pod)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if updateWasCalled {
		t.Error("UpdateBuild was called when it should not!")
	}
}

func TestHandleBuildPodDeletionOKErroneousBuild(t *testing.T) {
	updateWasCalled := false
	// erroneous builds should not be handled
	build := mockBuild(buildapi.BuildPhaseError, buildapi.BuildOutput{})
	ctrl := mockBuildPodController(build, &customBuildUpdater{
		UpdateFunc: func(namespace string, build *buildapi.Build) error {
			updateWasCalled = true
			return nil
		},
	})
	pod := mockPod(kapi.PodSucceeded, 0)

	err := ctrl.HandleBuildPodDeletion(pod)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if updateWasCalled {
		t.Error("UpdateBuild was called when it should not!")
	}
}

type fakeIndexer struct {
	*cache.FakeCustomStore
}

func (*fakeIndexer) Index(indexName string, obj interface{}) ([]interface{}, error) { return nil, nil }
func (*fakeIndexer) ListIndexFuncValues(indexName string) []string                  { return nil }
func (*fakeIndexer) ByIndex(indexName, indexKey string) ([]interface{}, error)      { return nil, nil }
func (*fakeIndexer) GetIndexers() cache.Indexers                                    { return nil }
func (*fakeIndexer) AddIndexers(newIndexers cache.Indexers) error                   { return nil }

func newErrIndexer(err error) cache.Indexer {
	return &fakeIndexer{
		&cache.FakeCustomStore{
			GetByKeyFunc: func(key string) (interface{}, bool, error) {
				return nil, true, err
			},
		},
	}
}

func TestHandleBuildPodDeletionBuildGetError(t *testing.T) {
	ctrl := mockBuildPodController(nil, &customBuildUpdater{})
	ctrl.buildStore.Indexer = newErrIndexer(errors.New("random"))
	pod := mockPod(kapi.PodSucceeded, 0)
	err := ctrl.HandleBuildPodDeletion(pod)
	if err == nil {
		t.Error("Expected random error, but got none!")
	}
	if err != nil && err.Error() != "random" {
		t.Errorf("Expected random error, got: %v", err)
	}
}

func TestHandleBuildPodDeletionBuildNotExists(t *testing.T) {
	updateWasCalled := false
	ctrl := mockBuildPodController(nil, &customBuildUpdater{
		UpdateFunc: func(namespace string, build *buildapi.Build) error {
			updateWasCalled = true
			return nil
		},
	})
	pod := mockPod(kapi.PodSucceeded, 0)

	err := ctrl.HandleBuildPodDeletion(pod)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if updateWasCalled {
		t.Error("UpdateBuild was called when it should not!")
	}
}

func TestHandleBuildPodDeletionBuildUpdateError(t *testing.T) {
	build := mockBuild(buildapi.BuildPhaseRunning, buildapi.BuildOutput{})
	ctrl := mockBuildPodController(build, &customBuildUpdater{
		UpdateFunc: func(namespace string, build *buildapi.Build) error {
			return errors.New("random")
		},
	})
	pod := mockPod(kapi.PodSucceeded, 0)

	err := ctrl.HandleBuildPodDeletion(pod)
	if err == nil {
		t.Error("Expected random error, but got none!")
	}
}
