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

func mockBuild(status buildapi.BuildStatus, output buildapi.BuildOutput) *buildapi.Build {
	return &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name:      "data-build",
			Namespace: "namespace",
			Labels: map[string]string{
				"name": "dataBuild",
			},
		},
		Parameters: buildapi.BuildParameters{
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
		Status: status,
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
		inStatus      buildapi.BuildStatus
		outStatus     buildapi.BuildStatus
		buildOutput   buildapi.BuildOutput
		buildStrategy BuildStrategy
		buildUpdater  buildclient.BuildUpdater
		imageClient   imageStreamClient
		podManager    podManager
		outputSpec    string
	}

	tests := []handleBuildTest{
		{ // 0
			inStatus:  buildapi.BuildStatusNew,
			outStatus: buildapi.BuildStatusPending,
			buildOutput: buildapi.BuildOutput{
				DockerImageReference: "repository/dataBuild",
			},
		},
		{ // 1
			inStatus:  buildapi.BuildStatusPending,
			outStatus: buildapi.BuildStatusPending,
			buildOutput: buildapi.BuildOutput{
				DockerImageReference: "repository/dataBuild",
			},
		},
		{ // 2
			inStatus:  buildapi.BuildStatusRunning,
			outStatus: buildapi.BuildStatusRunning,
			buildOutput: buildapi.BuildOutput{
				DockerImageReference: "repository/dataBuild",
			},
		},
		{ // 3
			inStatus:  buildapi.BuildStatusComplete,
			outStatus: buildapi.BuildStatusComplete,
			buildOutput: buildapi.BuildOutput{
				DockerImageReference: "repository/dataBuild",
			},
		},
		{ // 4
			inStatus:  buildapi.BuildStatusFailed,
			outStatus: buildapi.BuildStatusFailed,
			buildOutput: buildapi.BuildOutput{
				DockerImageReference: "repository/dataBuild",
			},
		},
		{ // 5
			inStatus:  buildapi.BuildStatusError,
			outStatus: buildapi.BuildStatusError,
			buildOutput: buildapi.BuildOutput{
				DockerImageReference: "repository/dataBuild",
			},
		},
		{ // 6
			inStatus:      buildapi.BuildStatusNew,
			outStatus:     buildapi.BuildStatusError,
			buildStrategy: &errStrategy{},
			buildOutput: buildapi.BuildOutput{
				DockerImageReference: "repository/dataBuild",
			},
		},
		{ // 7
			inStatus:   buildapi.BuildStatusNew,
			outStatus:  buildapi.BuildStatusError,
			podManager: &errPodManager{},
			buildOutput: buildapi.BuildOutput{
				DockerImageReference: "repository/dataBuild",
			},
		},
		{ // 8
			inStatus:   buildapi.BuildStatusNew,
			outStatus:  buildapi.BuildStatusPending,
			podManager: &errExistsPodManager{},
			buildOutput: buildapi.BuildOutput{
				DockerImageReference: "repository/dataBuild",
			},
		},
		{ // 9
			inStatus:     buildapi.BuildStatusNew,
			outStatus:    buildapi.BuildStatusPending,
			buildUpdater: &errBuildUpdater{},
			buildOutput: buildapi.BuildOutput{
				DockerImageReference: "repository/dataBuild",
			},
		},
		{ // 10
			inStatus:  buildapi.BuildStatusNew,
			outStatus: buildapi.BuildStatusPending,
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Name: "foo",
				},
			},
			outputSpec: "image/repo",
		},
		{ // 11
			inStatus:  buildapi.BuildStatusNew,
			outStatus: buildapi.BuildStatusPending,
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			outputSpec: "image/repo",
		},
		{ // 12
			inStatus:    buildapi.BuildStatusNew,
			outStatus:   buildapi.BuildStatusError,
			imageClient: &errNotFoundImageStreamClient{},
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Name: "foo",
				},
			},
		},
		{ // 13
			inStatus:    buildapi.BuildStatusNew,
			outStatus:   buildapi.BuildStatusError,
			imageClient: &errImageStreamClient{},
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Name: "foo",
				},
			},
		},
		{ // 14
			inStatus:  buildapi.BuildStatusNew,
			outStatus: buildapi.BuildStatusPending,
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Name:      "foo",
					Namespace: "bar",
				},
			},
			outputSpec: "image/repo",
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
		if tc.inStatus != buildapi.BuildStatusError && tc.outStatus == buildapi.BuildStatusError {
			if err == nil {
				t.Errorf("(%d) Expected an error from HandleBuild, got none!", i)
			}
			continue
		}

		if err != nil {
			t.Errorf("(%d) Unexpected error %v", i, err)
		}
		if build.Status != tc.outStatus {
			t.Errorf("(%d) Expected %s, got %s!", i, tc.outStatus, build.Status)
		}
		if tc.inStatus != buildapi.BuildStatusError && build.Status == buildapi.BuildStatusError && len(build.Message) == 0 {
			t.Errorf("(%d) errored build should set message: %#v", i, build)
		}
		if len(tc.outputSpec) != 0 {
			build := ctrl.BuildStrategy.(*okStrategy).build
			if build.Parameters.Output.DockerImageReference != tc.outputSpec {
				t.Errorf("(%d) expected build sent to strategy to have docker spec %q: %#v", i, tc.outputSpec, build)
			}
		}
	}
}

func TestHandlePod(t *testing.T) {
	type handlePodTest struct {
		matchID             bool
		inStatus            buildapi.BuildStatus
		outStatus           buildapi.BuildStatus
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
			inStatus:            buildapi.BuildStatusPending,
			outStatus:           buildapi.BuildStatusPending,
			podStatus:           kapi.PodPending,
			exitCode:            0,
			startTimestamp:      nil,
			completionTimestamp: nil,
		},
		{ // 1
			matchID:             true,
			inStatus:            buildapi.BuildStatusPending,
			outStatus:           buildapi.BuildStatusPending,
			podStatus:           kapi.PodPending,
			exitCode:            0,
			startTimestamp:      nil,
			completionTimestamp: nil,
		},
		{ // 2
			matchID:             true,
			inStatus:            buildapi.BuildStatusPending,
			outStatus:           buildapi.BuildStatusRunning,
			podStatus:           kapi.PodRunning,
			exitCode:            0,
			startTimestamp:      curtime,
			completionTimestamp: nil,
		},
		{ // 3
			matchID:             true,
			inStatus:            buildapi.BuildStatusRunning,
			outStatus:           buildapi.BuildStatusComplete,
			podStatus:           kapi.PodSucceeded,
			exitCode:            0,
			startTimestamp:      nil,
			completionTimestamp: curtime,
		},
		{ // 4
			matchID:             true,
			inStatus:            buildapi.BuildStatusRunning,
			outStatus:           buildapi.BuildStatusFailed,
			podStatus:           kapi.PodFailed,
			exitCode:            -1,
			startTimestamp:      nil,
			completionTimestamp: curtime,
		},
		{ // 5
			matchID:             true,
			inStatus:            buildapi.BuildStatusRunning,
			outStatus:           buildapi.BuildStatusComplete,
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
		if build.Status != tc.outStatus {
			t.Errorf("(%d) Expected %s, got %s!", i, tc.outStatus, build.Status)
		}

		if tc.startTimestamp == nil && build.StartTimestamp != nil {
			t.Errorf("(%d) Expected nil start timestamp, got %v!", i, build.StartTimestamp)
		}
		if tc.startTimestamp != nil && build.StartTimestamp == nil {
			t.Errorf("(%d) nil start timestamp!", i)
		}
		if tc.startTimestamp != nil && !tc.startTimestamp.Before(*build.StartTimestamp) && tc.startTimestamp.Time != build.StartTimestamp.Time {
			t.Errorf("(%d) Expected build start timestamp %v to be equal to or later than %v!", i, build.StartTimestamp, tc.startTimestamp)
		}

		if tc.completionTimestamp == nil && build.CompletionTimestamp != nil {
			t.Errorf("(%d) Expected nil completion timestamp, got %v!", i, build.CompletionTimestamp)
		}
		if tc.completionTimestamp != nil && build.CompletionTimestamp == nil {
			t.Errorf("(%d) nil completion timestamp!", i)
		}
		if tc.completionTimestamp != nil && !tc.completionTimestamp.Before(*build.CompletionTimestamp) && tc.completionTimestamp.Time != build.CompletionTimestamp.Time {
			t.Errorf("(%d) Expected build completion timestamp %v to be equal to or later than %v!", i, build.CompletionTimestamp, tc.completionTimestamp)
		}
	}
}

func TestCancelBuild(t *testing.T) {
	type handleCancelBuildTest struct {
		inStatus            buildapi.BuildStatus
		outStatus           buildapi.BuildStatus
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
			inStatus:            buildapi.BuildStatusNew,
			outStatus:           buildapi.BuildStatusCancelled,
			exitCode:            0,
			startTimestamp:      nil,
			completionTimestamp: curtime,
		},
		{ // 1
			inStatus:            buildapi.BuildStatusPending,
			outStatus:           buildapi.BuildStatusCancelled,
			podStatus:           kapi.PodRunning,
			exitCode:            0,
			startTimestamp:      nil,
			completionTimestamp: curtime,
		},
		{ // 2
			inStatus:            buildapi.BuildStatusRunning,
			outStatus:           buildapi.BuildStatusCancelled,
			podStatus:           kapi.PodRunning,
			exitCode:            0,
			startTimestamp:      nil,
			completionTimestamp: curtime,
		},
		{ // 3
			inStatus:            buildapi.BuildStatusComplete,
			outStatus:           buildapi.BuildStatusComplete,
			podStatus:           kapi.PodSucceeded,
			exitCode:            0,
			startTimestamp:      nil,
			completionTimestamp: nil,
		},
		{ // 4
			inStatus:            buildapi.BuildStatusFailed,
			outStatus:           buildapi.BuildStatusFailed,
			podStatus:           kapi.PodFailed,
			exitCode:            1,
			startTimestamp:      nil,
			completionTimestamp: nil,
		},
		{ // 5
			inStatus:            buildapi.BuildStatusNew,
			outStatus:           buildapi.BuildStatusNew,
			podStatus:           kapi.PodFailed,
			exitCode:            1,
			podManager:          &errPodManager{},
			startTimestamp:      nil,
			completionTimestamp: nil,
		},
		{ // 6
			inStatus:            buildapi.BuildStatusNew,
			outStatus:           buildapi.BuildStatusNew,
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

		if tc.startTimestamp == nil && build.StartTimestamp != nil {
			t.Errorf("(%d) Expected nil start timestamp, got %v!", i, build.StartTimestamp)
		}
		if tc.startTimestamp != nil && build.StartTimestamp == nil {
			t.Errorf("(%d) nil start timestamp!", i)
		}
		if tc.startTimestamp != nil && !tc.startTimestamp.Before(*build.StartTimestamp) && tc.startTimestamp.Time != build.StartTimestamp.Time {
			t.Errorf("(%d) Expected build start timestamp %v to be equal to or later than %v!", i, build.StartTimestamp, tc.startTimestamp)
		}

		if tc.completionTimestamp == nil && build.CompletionTimestamp != nil {
			t.Errorf("(%d) Expected nil completion timestamp, got %v!", i, build.CompletionTimestamp)
		}
		if tc.completionTimestamp != nil && build.CompletionTimestamp == nil {
			t.Errorf("(%d) nil start timestamp!", i)
		}
		if tc.completionTimestamp != nil && !tc.completionTimestamp.Before(*build.CompletionTimestamp) && tc.completionTimestamp.Time != build.CompletionTimestamp.Time {
			t.Errorf("(%d) Expected build completion timestamp %v to be equal to or later than %v!", i, build.CompletionTimestamp, tc.completionTimestamp)
		}

		if build.Status != tc.outStatus {
			t.Errorf("(%d) Expected %s, got %s!", i, tc.outStatus, build.Status)
		}
	}
}
