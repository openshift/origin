package controller

import (
	"errors"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

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

func (_ *okPodManager) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return &kapi.Pod{}, nil
}

func (_ *okPodManager) DeletePod(namespace string, pod *kapi.Pod) error {
	return nil
}

type errPodManager struct{}

func (_ *errPodManager) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return &kapi.Pod{}, errors.New("CreatePod error!")
}

func (_ *errPodManager) DeletePod(namespace string, pod *kapi.Pod) error {
	return errors.New("DeletePod error!")
}

type errExistsPodManager struct{}

func (_ *errExistsPodManager) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return &kapi.Pod{}, kerrors.NewAlreadyExists("kind", "name")
}

func (_ *errExistsPodManager) DeletePod(namespace string, pod *kapi.Pod) error {
	return kerrors.NewNotFound("kind", "name")
}

type okImageRepositoryClient struct{}

func (_ *okImageRepositoryClient) GetImageRepository(namespace, name string) (*imageapi.ImageRepository, error) {
	return &imageapi.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{Name: name, Namespace: namespace},
		Status: imageapi.ImageRepositoryStatus{
			DockerImageRepository: "image/repo",
		},
	}, nil
}

type errImageRepositoryClient struct{}

func (_ *errImageRepositoryClient) GetImageRepository(namespace, name string) (*imageapi.ImageRepository, error) {
	return nil, errors.New("GetImageRepository error!")
}

type errNotFoundImageRepositoryClient struct{}

func (_ *errNotFoundImageRepositoryClient) GetImageRepository(namespace, name string) (*imageapi.ImageRepository, error) {
	return nil, kerrors.NewNotFound("ImageRepository", name)
}

func mockBuildAndController(status buildapi.BuildStatus, output buildapi.BuildOutput) (build *buildapi.Build, controller *BuildController) {
	build = &buildapi.Build{
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
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{
					ContextDir: "contextimage",
				},
			},
			Output: output,
		},
		Status:  status,
		PodName: "-the-pod-id",
	}
	controller = &BuildController{
		BuildStore:            buildtest.NewFakeBuildStore(build),
		BuildUpdater:          &okBuildUpdater{},
		PodManager:            &okPodManager{},
		NextBuild:             func() *buildapi.Build { return nil },
		NextPod:               func() *kapi.Pod { return nil },
		BuildStrategy:         &okStrategy{},
		ImageRepositoryClient: &okImageRepositoryClient{},
	}
	return
}

func mockPod(status kapi.PodPhase, exitCode int) *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{Name: "PodName"},
		Status: kapi.PodStatus{
			Phase: status,
			Info: kapi.PodInfo{
				"container1": kapi.ContainerStatus{
					State: kapi.ContainerState{
						Termination: &kapi.ContainerStateTerminated{ExitCode: exitCode},
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
		imageClient   imageRepositoryClient
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
			outStatus:   buildapi.BuildStatusError, // TODO: this should be a retry
			imageClient: &errNotFoundImageRepositoryClient{},
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Name: "foo",
				},
			},
		},
		{ // 13
			inStatus:    buildapi.BuildStatusNew,
			outStatus:   buildapi.BuildStatusError,
			imageClient: &errImageRepositoryClient{},
			buildOutput: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Name: "foo",
				},
			},
		},
	}

	for i, tc := range tests {
		build, ctrl := mockBuildAndController(tc.inStatus, tc.buildOutput)
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
			ctrl.ImageRepositoryClient = tc.imageClient
		}

		ctrl.HandleBuild(build)

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
		matchID      bool
		inStatus     buildapi.BuildStatus
		outStatus    buildapi.BuildStatus
		podStatus    kapi.PodPhase
		exitCode     int
		buildUpdater buildclient.BuildUpdater
	}

	tests := []handlePodTest{
		{ // 0
			matchID:   false,
			inStatus:  buildapi.BuildStatusPending,
			outStatus: buildapi.BuildStatusPending,
			podStatus: kapi.PodPending,
			exitCode:  0,
		},
		{ // 1
			matchID:   true,
			inStatus:  buildapi.BuildStatusPending,
			outStatus: buildapi.BuildStatusPending,
			podStatus: kapi.PodPending,
			exitCode:  0,
		},
		{ // 2
			matchID:   true,
			inStatus:  buildapi.BuildStatusPending,
			outStatus: buildapi.BuildStatusRunning,
			podStatus: kapi.PodRunning,
			exitCode:  0,
		},
		{ // 3
			matchID:   true,
			inStatus:  buildapi.BuildStatusRunning,
			outStatus: buildapi.BuildStatusComplete,
			podStatus: kapi.PodSucceeded,
			exitCode:  0,
		},
		{ // 4
			matchID:   true,
			inStatus:  buildapi.BuildStatusRunning,
			outStatus: buildapi.BuildStatusFailed,
			podStatus: kapi.PodFailed,
			exitCode:  -1,
		},
		{ // 5
			matchID:      true,
			inStatus:     buildapi.BuildStatusRunning,
			outStatus:    buildapi.BuildStatusComplete,
			podStatus:    kapi.PodSucceeded,
			exitCode:     0,
			buildUpdater: &errBuildUpdater{},
		},
	}

	for i, tc := range tests {
		build, ctrl := mockBuildAndController(tc.inStatus, buildapi.BuildOutput{})
		pod := mockPod(tc.podStatus, tc.exitCode)
		if tc.matchID {
			build.PodName = pod.Name
		}
		if tc.buildUpdater != nil {
			ctrl.BuildUpdater = tc.buildUpdater
		}

		ctrl.HandlePod(pod)

		if build.Status != tc.outStatus {
			t.Errorf("(%d) Expected %s, got %s!", i, tc.outStatus, build.Status)
		}
	}
}

func TestCancelBuild(t *testing.T) {
	type handleCancelBuildTest struct {
		inStatus  buildapi.BuildStatus
		outStatus buildapi.BuildStatus
		podStatus kapi.PodPhase
		exitCode  int
	}

	tests := []handleCancelBuildTest{
		{ // 0
			inStatus:  buildapi.BuildStatusNew,
			outStatus: buildapi.BuildStatusCancelled,
			exitCode:  0,
		},
		{ // 1
			inStatus:  buildapi.BuildStatusPending,
			outStatus: buildapi.BuildStatusCancelled,
			podStatus: kapi.PodRunning,
			exitCode:  0,
		},
		{ // 2
			inStatus:  buildapi.BuildStatusRunning,
			outStatus: buildapi.BuildStatusCancelled,
			podStatus: kapi.PodRunning,
			exitCode:  0,
		},
		{ // 3
			inStatus:  buildapi.BuildStatusComplete,
			outStatus: buildapi.BuildStatusComplete,
			podStatus: kapi.PodSucceeded,
			exitCode:  0,
		},
		{ // 4
			inStatus:  buildapi.BuildStatusFailed,
			outStatus: buildapi.BuildStatusFailed,
			podStatus: kapi.PodFailed,
			exitCode:  1,
		},
	}

	for i, tc := range tests {
		build, ctrl := mockBuildAndController(tc.inStatus, buildapi.BuildOutput{})
		pod := mockPod(tc.podStatus, tc.exitCode)

		ctrl.CancelBuild(build, pod)

		if build.Status != tc.outStatus {
			t.Errorf("(%d) Expected %s, got %s!", i, tc.outStatus, build.Status)
		}
	}
}
