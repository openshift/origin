package controller

import (
	"errors"
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildtest "github.com/openshift/origin/pkg/build/controller/test"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type okBuildUpdater struct{}

func (okc *okBuildUpdater) Update(namespace string, build *buildapi.Build) error {
	return nil
}

type errBuildUpdater struct{}

func (ec *errBuildUpdater) Update(namespace string, build *buildapi.Build) error {
	return errors.New("UpdateBuild error!")
}

type okStrategy struct {
	build *buildapi.Build
}

func (os *okStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	os.build = build
	return &kapi.Pod{}, nil
}

type errStrategy struct{}

func (es *errStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	return nil, errors.New("CreateBuildPod error!")
}

type okPodManager struct{}

func (*okPodManager) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return &kapi.Pod{}, nil
}

func (*okPodManager) DeletePod(namespace string, pod *kapi.Pod) error {
	return nil
}

func (*okPodManager) GetPod(namespace, name string) (*kapi.Pod, error) {
	return &kapi.Pod{}, nil
}

type errPodManager struct{}

func (*errPodManager) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return &kapi.Pod{}, errors.New("CreatePod error!")
}

func (*errPodManager) DeletePod(namespace string, pod *kapi.Pod) error {
	return errors.New("DeletePod error!")
}

func (*errPodManager) GetPod(namespace, name string) (*kapi.Pod, error) {
	return nil, errors.New("GetPod error!")
}

type errExistsPodManager struct{}

func (*errExistsPodManager) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return &kapi.Pod{}, kerrors.NewAlreadyExists("kind", "name")
}

func (*errExistsPodManager) DeletePod(namespace string, pod *kapi.Pod) error {
	return kerrors.NewNotFound("kind", "name")
}

func (*errExistsPodManager) GetPod(namespace, name string) (*kapi.Pod, error) {
	return nil, kerrors.NewNotFound("kind", "name")
}

type okImageStreamClient struct{}

func (*okImageStreamClient) GetImageStream(namespace, name string) (*imageapi.ImageStream, error) {
	return &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: name, Namespace: namespace},
		Status: imageapi.ImageStreamStatus{
			DockerImageRepository: "image/repo",
		},
	}, nil
}

type errImageStreamClient struct{}

func (*errImageStreamClient) GetImageStream(namespace, name string) (*imageapi.ImageStream, error) {
	return nil, errors.New("GetImageStream error!")
}

type errNotFoundImageStreamClient struct{}

func (*errNotFoundImageStreamClient) GetImageStream(namespace, name string) (*imageapi.ImageStream, error) {
	return nil, kerrors.NewNotFound("ImageStream", name)
}

func mockBuild(status buildapi.BuildPhase, output buildapi.BuildOutput) *buildapi.Build {
	return &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "data-build",
			Namespace: "namespace",
			Labels: map[string]string{
				"name": "dataBuild",
			},
		},
		Spec: buildapi.BuildSpec{
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://my.build.com/the/build/Dockerfile",
				},
				ContextDir: "contextimage",
			},
			Strategy: buildapi.BuildStrategy{
				Type:           buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{},
			},
			Output: output,
		},
		Status: buildapi.BuildStatus{
			Phase: status,
		},
	}
}

func mockBuildController() *BuildController {
	return &BuildController{
		BuildUpdater:      &okBuildUpdater{},
		PodManager:        &okPodManager{},
		BuildStrategy:     &okStrategy{},
		ImageStreamClient: &okImageStreamClient{},
		Recorder:          &record.FakeRecorder{},
	}
}

func mockBuildPodController(build *buildapi.Build) *BuildPodController {
	return &BuildPodController{
		BuildStore:   buildtest.NewFakeBuildStore(build),
		BuildUpdater: &okBuildUpdater{},
		PodManager:   &okPodManager{},
	}
}

func mockPod(status kapi.PodPhase, exitCode int) *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name: "data-build-build",
			Annotations: map[string]string{
				buildapi.BuildAnnotation: "data-build",
			},
		},
		Status: kapi.PodStatus{
			Phase: status,
			ContainerStatuses: []kapi.ContainerStatus{
				{
					State: kapi.ContainerState{
						Terminated: &kapi.ContainerStateTerminated{ExitCode: exitCode},
					},
				},
			},
		},
	}
}

func TestHandleBuild(t *testing.T) {
	type handleBuildTest struct {
		inStatus      buildapi.BuildPhase
		outStatus     buildapi.BuildPhase
		buildOutput   buildapi.BuildOutput
		buildStrategy BuildStrategy
		buildUpdater  buildclient.BuildUpdater
		imageClient   imageStreamClient
		podManager    podManager
		outputSpec    string
	}

	tests := []handleBuildTest{
		{ // 0
			inStatus:  buildapi.BuildPhaseNew,
			outStatus: buildapi.BuildPhasePending,
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: "repository/dataBuild",
				},
			},
		},
		{ // 1
			inStatus:  buildapi.BuildPhasePending,
			outStatus: buildapi.BuildPhasePending,
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: "repository/dataBuild",
				},
			},
		},
		{ // 2
			inStatus:  buildapi.BuildPhaseRunning,
			outStatus: buildapi.BuildPhaseRunning,
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: "repository/dataBuild",
				},
			},
		},
		{ // 3
			inStatus:  buildapi.BuildPhaseComplete,
			outStatus: buildapi.BuildPhaseComplete,
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: "repository/dataBuild",
				},
			},
		},
		{ // 4
			inStatus:  buildapi.BuildPhaseFailed,
			outStatus: buildapi.BuildPhaseFailed,
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: "repository/dataBuild",
				},
			},
		},
		{ // 5
			inStatus:  buildapi.BuildPhaseError,
			outStatus: buildapi.BuildPhaseError,
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: "repository/dataBuild",
				},
			},
		},
		{ // 6
			inStatus:      buildapi.BuildPhaseNew,
			outStatus:     buildapi.BuildPhaseError,
			buildStrategy: &errStrategy{},
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: "repository/dataBuild",
				},
			},
		},
		{ // 7
			inStatus:   buildapi.BuildPhaseNew,
			outStatus:  buildapi.BuildPhaseError,
			podManager: &errPodManager{},
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: "repository/dataBuild",
				},
			},
		},
		{ // 8
			inStatus:   buildapi.BuildPhaseNew,
			outStatus:  buildapi.BuildPhasePending,
			podManager: &errExistsPodManager{},
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: "repository/dataBuild",
				},
			},
		},
		{ // 9
			inStatus:     buildapi.BuildPhaseNew,
			outStatus:    buildapi.BuildPhasePending,
			buildUpdater: &errBuildUpdater{},
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: "repository/dataBuild",
				},
			},
		},
		{ // 10
			inStatus:  buildapi.BuildPhaseNew,
			outStatus: buildapi.BuildPhasePending,
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "ImageStreamTag",
					Name: "foo:tag",
				},
			},
			outputSpec: "image/repo:tag",
		},
		{ // 11
			inStatus:  buildapi.BuildPhaseNew,
			outStatus: buildapi.BuildPhasePending,
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind:      "ImageStreamTag",
					Name:      "foo:tag",
					Namespace: "bar",
				},
			},
			outputSpec: "image/repo:tag",
		},
		{ // 12
			inStatus:    buildapi.BuildPhaseNew,
			outStatus:   buildapi.BuildPhaseError,
			imageClient: &errNotFoundImageStreamClient{},
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "ImageStreamTag",
					Name: "foo:tag",
				},
			},
		},
		{ // 13
			inStatus:    buildapi.BuildPhaseNew,
			outStatus:   buildapi.BuildPhaseError,
			imageClient: &errImageStreamClient{},
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "ImageStreamTag",
					Name: "foo:tag",
				},
			},
		},
		{ // 14
			inStatus:  buildapi.BuildPhaseNew,
			outStatus: buildapi.BuildPhasePending,
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind:      "ImageStreamTag",
					Name:      "foo:tag",
					Namespace: "bar",
				},
			},
			outputSpec: "image/repo:tag",
			// an error updating the build is not reported as an error.
			buildUpdater: &errBuildUpdater{},
		},
	}

	for i, tc := range tests {
		build := mockBuild(tc.inStatus, tc.buildOutput)
		ctrl := mockBuildController()
		if tc.buildStrategy != nil {
			ctrl.BuildStrategy = tc.buildStrategy
		}
		if tc.buildUpdater != nil {
			ctrl.BuildUpdater = tc.buildUpdater
		}
		if tc.podManager != nil {
			ctrl.PodManager = tc.podManager
		}
		if tc.imageClient != nil {
			ctrl.ImageStreamClient = tc.imageClient
		}

		err := ctrl.HandleBuild(build)

		// ensure we return an error for cases where expected output is an error.
		// these will be retried by the retrycontroller
		if tc.inStatus != buildapi.BuildPhaseError && tc.outStatus == buildapi.BuildPhaseError {
			if err == nil {
				t.Errorf("(%d) Expected an error from HandleBuild, got none!", i)
			}
			continue
		}

		if err != nil {
			t.Errorf("(%d) Unexpected error %v", i, err)
		}
		if build.Status.Phase != tc.outStatus {
			t.Errorf("(%d) Expected %s, got %s!", i, tc.outStatus, build.Status.Phase)
		}
		if tc.inStatus != buildapi.BuildPhaseError && build.Status.Phase == buildapi.BuildPhaseError && len(build.Status.Message) == 0 {
			t.Errorf("(%d) errored build should set message: %#v", i, build)
		}
		if len(tc.outputSpec) != 0 {
			build := ctrl.BuildStrategy.(*okStrategy).build
			if build.Spec.Output.To.Name != tc.outputSpec {
				t.Errorf("(%d) expected build sent to strategy to have docker spec %s, got %s", i, tc.outputSpec, build.Spec.Output.To.Name)
			}
		}
	}
}

func TestHandlePod(t *testing.T) {
	type handlePodTest struct {
		matchID             bool
		inStatus            buildapi.BuildPhase
		outStatus           buildapi.BuildPhase
		startTimestamp      *util.Time
		completionTimestamp *util.Time
		podStatus           kapi.PodPhase
		exitCode            int
		buildUpdater        buildclient.BuildUpdater
		podManager          podManager
	}
	dummy := util.Now()
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
	}

	for i, tc := range tests {
		build := mockBuild(tc.inStatus, buildapi.BuildOutput{})
		ctrl := mockBuildPodController(build)
		pod := mockPod(tc.podStatus, tc.exitCode)
		if tc.matchID {
			build.Name = "name"
		}
		if tc.buildUpdater != nil {
			ctrl.BuildUpdater = tc.buildUpdater
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

func TestCancelBuild(t *testing.T) {
	type handleCancelBuildTest struct {
		inStatus            buildapi.BuildPhase
		outStatus           buildapi.BuildPhase
		podStatus           kapi.PodPhase
		exitCode            int
		buildUpdater        buildclient.BuildUpdater
		podManager          podManager
		startTimestamp      *util.Time
		completionTimestamp *util.Time
	}
	dummy := util.Now()
	curtime := &dummy

	tests := []handleCancelBuildTest{
		{ // 0
			inStatus:            buildapi.BuildPhaseNew,
			outStatus:           buildapi.BuildPhaseCancelled,
			exitCode:            0,
			startTimestamp:      nil,
			completionTimestamp: curtime,
		},
		{ // 1
			inStatus:            buildapi.BuildPhasePending,
			outStatus:           buildapi.BuildPhaseCancelled,
			podStatus:           kapi.PodRunning,
			exitCode:            0,
			startTimestamp:      nil,
			completionTimestamp: curtime,
		},
		{ // 2
			inStatus:            buildapi.BuildPhaseRunning,
			outStatus:           buildapi.BuildPhaseCancelled,
			podStatus:           kapi.PodRunning,
			exitCode:            0,
			startTimestamp:      nil,
			completionTimestamp: curtime,
		},
		{ // 3
			inStatus:            buildapi.BuildPhaseComplete,
			outStatus:           buildapi.BuildPhaseComplete,
			podStatus:           kapi.PodSucceeded,
			exitCode:            0,
			startTimestamp:      nil,
			completionTimestamp: nil,
		},
		{ // 4
			inStatus:            buildapi.BuildPhaseFailed,
			outStatus:           buildapi.BuildPhaseFailed,
			podStatus:           kapi.PodFailed,
			exitCode:            1,
			startTimestamp:      nil,
			completionTimestamp: nil,
		},
		{ // 5
			inStatus:            buildapi.BuildPhaseNew,
			outStatus:           buildapi.BuildPhaseNew,
			podStatus:           kapi.PodFailed,
			exitCode:            1,
			podManager:          &errPodManager{},
			startTimestamp:      nil,
			completionTimestamp: nil,
		},
		{ // 6
			inStatus:            buildapi.BuildPhaseNew,
			outStatus:           buildapi.BuildPhaseNew,
			podStatus:           kapi.PodFailed,
			exitCode:            1,
			buildUpdater:        &errBuildUpdater{},
			startTimestamp:      nil,
			completionTimestamp: nil,
		},
	}

	for i, tc := range tests {
		build := mockBuild(tc.inStatus, buildapi.BuildOutput{})
		ctrl := mockBuildPodController(build)
		pod := mockPod(tc.podStatus, tc.exitCode)
		if tc.buildUpdater != nil {
			ctrl.BuildUpdater = tc.buildUpdater
		}
		if tc.podManager != nil {
			ctrl.PodManager = tc.podManager
		}

		err := ctrl.CancelBuild(build, pod)

		if tc.podManager != nil && reflect.TypeOf(tc.podManager).Elem().Name() == "errPodManager" {
			if err == nil {
				t.Errorf("(%d) Expected error, got none", i)
			}
		}
		if tc.buildUpdater != nil && reflect.TypeOf(tc.buildUpdater).Elem().Name() == "errBuildUpdater" {
			if err == nil {
				t.Errorf("(%d) Expected error, got none", i)
			}
			// can't check tc.outStatus because the local build object does get updated
			// in this test (but would not be updated in etcd)
			continue
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
			t.Errorf("(%d) nil start timestamp!", i)
		}
		if tc.completionTimestamp != nil && !tc.completionTimestamp.Before(*build.Status.CompletionTimestamp) && tc.completionTimestamp.Time != build.Status.CompletionTimestamp.Time {
			t.Errorf("(%d) Expected build completion timestamp %v to be equal to or later than %v!", i, build.Status.CompletionTimestamp, tc.completionTimestamp)
		}

		if build.Status.Phase != tc.outStatus {
			t.Errorf("(%d) Expected %s, got %s!", i, tc.outStatus, build.Status.Phase)
		}
	}
}

type customPodManager struct {
	CreatePodFunc func(namespace string, pod *kapi.Pod) (*kapi.Pod, error)
	DeletePodFunc func(namespace string, pod *kapi.Pod) error
	GetPodFunc    func(namespace, name string) (*kapi.Pod, error)
}

func (c *customPodManager) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return c.CreatePodFunc(namespace, pod)
}

func (c *customPodManager) DeletePod(namespace string, pod *kapi.Pod) error {
	return c.DeletePodFunc(namespace, pod)
}

func (c *customPodManager) GetPod(namespace, name string) (*kapi.Pod, error) {
	return c.GetPodFunc(namespace, name)
}

func TestHandleHandleBuildDeletionOK(t *testing.T) {
	deleteWasCalled := false
	build := mockBuild(buildapi.BuildStatusComplete, buildapi.BuildOutput{})
	ctrl := BuildDeleteController{&customPodManager{
		GetPodFunc: func(namespace, names string) (*kapi.Pod, error) {
			return &kapi.Pod{ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{buildapi.BuildLabel: build.Name}}}, nil
		},
		DeletePodFunc: func(namespace string, pod *kapi.Pod) error {
			deleteWasCalled = true
			return nil
		},
	}}

	err := ctrl.HandleBuildDeletion(build)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if !deleteWasCalled {
		t.Error("DeletePod was not called when it should!")
	}
}

func TestHandleHandleBuildDeletionFailGetPod(t *testing.T) {
	build := mockBuild(buildapi.BuildStatusComplete, buildapi.BuildOutput{})
	ctrl := BuildDeleteController{&customPodManager{
		GetPodFunc: func(namespace, name string) (*kapi.Pod, error) {
			return nil, errors.New("random")
		},
	}}

	err := ctrl.HandleBuildDeletion(build)
	if err == nil {
		t.Error("Expected random error got none!")
	}
}

func TestHandleHandleBuildDeletionGetPodNotFound(t *testing.T) {
	deleteWasCalled := false
	build := mockBuild(buildapi.BuildStatusComplete, buildapi.BuildOutput{})
	ctrl := BuildDeleteController{&customPodManager{
		GetPodFunc: func(namespace, name string) (*kapi.Pod, error) {
			return nil, kerrors.NewNotFound("Pod", name)
		},
		DeletePodFunc: func(namespace string, pod *kapi.Pod) error {
			deleteWasCalled = true
			return nil
		},
	}}

	err := ctrl.HandleBuildDeletion(build)
	if err != nil {
		t.Errorf("Unexpected error, %v", err)
	}
	if deleteWasCalled {
		t.Error("DeletePod was called when it should not!")
	}
}

func TestHandleHandleBuildDeletionMismatchedLabels(t *testing.T) {
	deleteWasCalled := false
	build := mockBuild(buildapi.BuildStatusComplete, buildapi.BuildOutput{})
	ctrl := BuildDeleteController{&customPodManager{
		GetPodFunc: func(namespace, names string) (*kapi.Pod, error) {
			return &kapi.Pod{}, nil
		},
		DeletePodFunc: func(namespace string, pod *kapi.Pod) error {
			deleteWasCalled = true
			return nil
		},
	}}

	err := ctrl.HandleBuildDeletion(build)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if deleteWasCalled {
		t.Error("DeletePod was called when it should not!")
	}
}

func TestHandleHandleBuildDeletionDeletePodError(t *testing.T) {
	build := mockBuild(buildapi.BuildStatusComplete, buildapi.BuildOutput{})
	ctrl := BuildDeleteController{&customPodManager{
		GetPodFunc: func(namespace, names string) (*kapi.Pod, error) {
			return &kapi.Pod{ObjectMeta: kapi.ObjectMeta{Labels: map[string]string{buildapi.BuildLabel: build.Name}}}, nil
		},
		DeletePodFunc: func(namespace string, pod *kapi.Pod) error {
			return errors.New("random")
		},
	}}

	err := ctrl.HandleBuildDeletion(build)
	if err == nil {
		t.Error("Expected random error got none!")
	}
}

type customBuildUpdater struct {
	UpdateFunc func(namespace string, build *buildapi.Build) error
}

func (c *customBuildUpdater) Update(namespace string, build *buildapi.Build) error {
	return c.UpdateFunc(namespace, build)
}

func mockBuildPodDeleteController(build *buildapi.Build, buildUpdater *customBuildUpdater, err error) *BuildPodDeleteController {
	return &BuildPodDeleteController{
		BuildStore:   buildtest.FakeBuildStore{Build: build, Err: err},
		BuildUpdater: buildUpdater,
	}
}

func TestHandleBuildPodDeletionOK(t *testing.T) {
	updateWasCalled := false
	// only not finished build (buildutil.IsBuildComplete) should be handled
	build := mockBuild(buildapi.BuildStatusRunning, buildapi.BuildOutput{})
	ctrl := mockBuildPodDeleteController(build, &customBuildUpdater{
		UpdateFunc: func(namespace string, build *buildapi.Build) error {
			updateWasCalled = true
			return nil
		},
	}, nil)
	pod := mockPod(kapi.PodSucceeded, 0)

	err := ctrl.HandleBuildPodDeletion(pod)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if !updateWasCalled {
		t.Error("UpdateBuild was not called when it should!")
	}
}

func TestHandleBuildPodDeletionOKFinishedBuild(t *testing.T) {
	updateWasCalled := false
	// finished build buildutil.IsBuildComplete should not be handled
	build := mockBuild(buildapi.BuildStatusComplete, buildapi.BuildOutput{})
	ctrl := mockBuildPodDeleteController(build, &customBuildUpdater{
		UpdateFunc: func(namespace string, build *buildapi.Build) error {
			updateWasCalled = true
			return nil
		},
	}, nil)
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
	build := mockBuild(buildapi.BuildStatusError, buildapi.BuildOutput{})
	ctrl := mockBuildPodDeleteController(build, &customBuildUpdater{
		UpdateFunc: func(namespace string, build *buildapi.Build) error {
			updateWasCalled = true
			return nil
		},
	}, nil)
	pod := mockPod(kapi.PodSucceeded, 0)

	err := ctrl.HandleBuildPodDeletion(pod)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if updateWasCalled {
		t.Error("UpdateBuild was called when it should not!")
	}
}

func TestHandleBuildPodDeletionBuildGetError(t *testing.T) {
	ctrl := mockBuildPodDeleteController(nil, &customBuildUpdater{}, errors.New("random"))
	pod := mockPod(kapi.PodSucceeded, 0)

	err := ctrl.HandleBuildPodDeletion(pod)
	if err == nil {
		t.Error("Expected random error, but got none!")
	}
}

func TestHandleBuildPodDeletionBuildNotExists(t *testing.T) {
	updateWasCalled := false
	ctrl := mockBuildPodDeleteController(nil, &customBuildUpdater{
		UpdateFunc: func(namespace string, build *buildapi.Build) error {
			updateWasCalled = true
			return nil
		},
	}, nil)
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
	build := mockBuild(buildapi.BuildStatusRunning, buildapi.BuildOutput{})
	ctrl := mockBuildPodDeleteController(build, &customBuildUpdater{
		UpdateFunc: func(namespace string, build *buildapi.Build) error {
			return errors.New("random")
		},
	}, nil)
	pod := mockPod(kapi.PodSucceeded, 0)

	err := ctrl.HandleBuildPodDeletion(pod)
	if err == nil {
		t.Error("Expected random error, but got none!")
	}
}
