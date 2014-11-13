package controller

import (
	"errors"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildtest "github.com/openshift/origin/pkg/build/controller/test"
	osclient "github.com/openshift/origin/pkg/client"
)

type okOsClient struct{}

func (_ *okOsClient) UpdateBuild(kapi.Context, *buildapi.Build) (*buildapi.Build, error) {
	return &buildapi.Build{}, nil
}

type errOsClient struct{}

func (_ *errOsClient) UpdateBuild(ctx kapi.Context, build *buildapi.Build) (*buildapi.Build, error) {
	return &buildapi.Build{}, errors.New("UpdateBuild error!")
}

type okStrategy struct{}

func (_ *okStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	return &kapi.Pod{}, nil
}

type errStrategy struct{}

func (_ *errStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	return nil, errors.New("CreateBuildPod error!")
}

type okPodControl struct{}

func (_ *okPodControl) createPod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return &kapi.Pod{}, nil
}

type errPodControl struct{}

func (_ *errPodControl) createPod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return &kapi.Pod{}, errors.New("CreatePod error!")
}

type errExistsPodControl struct{}

func (_ *errExistsPodControl) createPod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return &kapi.Pod{}, kerrors.NewAlreadyExists("kind", "name")
}

func mockBuildAndController(status buildapi.BuildStatus) (build *buildapi.Build, controller *BuildController) {
	build = &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "dataBuild",
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
			Output: buildapi.BuildOutput{
				ImageTag: "repository/dataBuild",
			},
		},
		Status:  status,
		PodName: "-the-pod-id",
	}
	controller = &BuildController{
		BuildStore:    buildtest.NewFakeBuildStore(build),
		BuildUpdater:  &osclient.Fake{},
		PodControl:    &okPodControl{},
		NextBuild:     func() *buildapi.Build { return nil },
		NextPod:       func() *kapi.Pod { return nil },
		BuildStrategy: &okStrategy{},
	}
	return
}

func mockPod(status kapi.PodCondition) *kapi.Pod {
	return &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{Name: "PodName"},
		CurrentState: kapi.PodState{
			Status: status,
		},
	}
}

func TestHandleBuild(t *testing.T) {
	type handleBuildTest struct {
		inStatus      buildapi.BuildStatus
		outStatus     buildapi.BuildStatus
		buildStrategy BuildStrategy
		buildUpdater  buildUpdater
		podControl    PodControlInterface
	}

	tests := []handleBuildTest{
		{ // 0
			inStatus:  buildapi.BuildStatusNew,
			outStatus: buildapi.BuildStatusPending,
		},
		{ // 1
			inStatus:  buildapi.BuildStatusPending,
			outStatus: buildapi.BuildStatusPending,
		},
		{ // 2
			inStatus:  buildapi.BuildStatusRunning,
			outStatus: buildapi.BuildStatusRunning,
		},
		{ // 3
			inStatus:  buildapi.BuildStatusComplete,
			outStatus: buildapi.BuildStatusComplete,
		},
		{ // 4
			inStatus:  buildapi.BuildStatusFailed,
			outStatus: buildapi.BuildStatusFailed,
		},
		{ // 5
			inStatus:  buildapi.BuildStatusError,
			outStatus: buildapi.BuildStatusError,
		},
		{ // 6
			inStatus:      buildapi.BuildStatusNew,
			outStatus:     buildapi.BuildStatusFailed,
			buildStrategy: &errStrategy{},
		},
		{ // 7
			inStatus:   buildapi.BuildStatusNew,
			outStatus:  buildapi.BuildStatusFailed,
			podControl: &errPodControl{},
		},
		{ // 8
			inStatus:   buildapi.BuildStatusNew,
			outStatus:  buildapi.BuildStatusFailed,
			podControl: &errExistsPodControl{},
		},
		{ // 9
			inStatus:     buildapi.BuildStatusNew,
			outStatus:    buildapi.BuildStatusPending,
			buildUpdater: &errOsClient{},
		},
	}

	for i, tc := range tests {
		build, ctrl := mockBuildAndController(tc.inStatus)
		if tc.buildStrategy != nil {
			ctrl.BuildStrategy = tc.buildStrategy
		}
		if tc.buildUpdater != nil {
			ctrl.BuildUpdater = tc.buildUpdater
		}
		if tc.podControl != nil {
			ctrl.PodControl = tc.podControl
		}

		ctrl.HandleBuild(build)

		if build.Status != tc.outStatus {
			t.Errorf("(%d) Expected %s, got %s!", i, tc.outStatus, build.Status)
		}
	}
}

func TestHandlePod(t *testing.T) {
	type handlePodTest struct {
		matchID      bool
		inStatus     buildapi.BuildStatus
		outStatus    buildapi.BuildStatus
		podStatus    kapi.PodCondition
		buildUpdater buildUpdater
	}

	tests := []handlePodTest{
		{ // 0
			matchID:   false,
			inStatus:  buildapi.BuildStatusPending,
			outStatus: buildapi.BuildStatusPending,
			podStatus: kapi.PodPending,
		},
		{ // 1
			matchID:   true,
			inStatus:  buildapi.BuildStatusPending,
			outStatus: buildapi.BuildStatusPending,
			podStatus: kapi.PodPending,
		},
		{ // 2
			matchID:   true,
			inStatus:  buildapi.BuildStatusPending,
			outStatus: buildapi.BuildStatusRunning,
			podStatus: kapi.PodRunning,
		},
		{ // 3
			matchID:   true,
			inStatus:  buildapi.BuildStatusRunning,
			outStatus: buildapi.BuildStatusComplete,
			podStatus: kapi.PodSucceeded,
		},
		{ // 4
			matchID:   true,
			inStatus:  buildapi.BuildStatusRunning,
			outStatus: buildapi.BuildStatusFailed,
			podStatus: kapi.PodFailed,
		},
		{ // 5
			matchID:      true,
			inStatus:     buildapi.BuildStatusRunning,
			outStatus:    buildapi.BuildStatusComplete,
			podStatus:    kapi.PodSucceeded,
			buildUpdater: &errOsClient{},
		},
	}

	for i, tc := range tests {
		build, ctrl := mockBuildAndController(tc.inStatus)
		pod := mockPod(tc.podStatus)
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
