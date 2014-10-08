package build

import (
	"errors"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/build/api"
)

type okOsClient struct{}

func (_ *okOsClient) ListBuilds(ctx kapi.Context, selector labels.Selector) (*api.BuildList, error) {
	return &api.BuildList{}, nil
}

func (_ *okOsClient) UpdateBuild(kapi.Context, *api.Build) (*api.Build, error) {
	return &api.Build{}, nil
}

type errOsClient struct{}

func (_ *errOsClient) ListBuilds(ctx kapi.Context, selector labels.Selector) (*api.BuildList, error) {
	return &api.BuildList{}, errors.New("ListBuild error!")
}

func (_ *errOsClient) UpdateBuild(ctx kapi.Context, build *api.Build) (*api.Build, error) {
	return &api.Build{}, errors.New("UpdateBuild error!")
}

type okStrategy struct{}

func (_ *okStrategy) CreateBuildPod(build *api.Build) (*kapi.Pod, error) {
	return &kapi.Pod{}, nil
}

type errKubeClient struct {
	kubeclient.Fake
}

func (_ *errKubeClient) CreatePod(ctx kapi.Context, pod *kapi.Pod) (*kapi.Pod, error) {
	return &kapi.Pod{}, errors.New("CreatePod error!")
}

func (_ *errKubeClient) GetPod(ctx kapi.Context, name string) (*kapi.Pod, error) {
	return &kapi.Pod{}, errors.New("GedPod error!")
}

type okKubeClient struct {
	kubeclient.Fake
}

func (_ *okKubeClient) GetPod(ctx kapi.Context, name string) (*kapi.Pod, error) {
	return &kapi.Pod{
		CurrentState: kapi.PodState{Status: kapi.PodTerminated},
	}, nil
}

func TestSynchronizeBuildNew(t *testing.T) {
	ctrl, build, ctx := setup()
	build.Status = api.BuildNew
	status, err := ctrl.synchronize(ctx, build)
	if err != nil {
		t.Errorf("Unexpected error: %s!", err.Error())
	}
	if status != api.BuildPending {
		t.Errorf("Expected BuildPending, got %s!", status)
	}
}

func TestSynchronizeBuildPendingUnknownStrategy(t *testing.T) {
	ctrl, build, ctx := setup()
	build.Status = api.BuildPending
	build.Input.Type = "unknownStrategy"
	status, err := ctrl.synchronize(ctx, build)
	if err == nil {
		t.Error("Expected error, but none happened!")
	}
	if status != api.BuildError {
		t.Errorf("Expected BuildError, got %s!", status)
	}
}

func TestSynchronizeBuildPendingFailedCreatePod(t *testing.T) {
	ctrl, build, ctx := setup()
	ctrl.kubeClient = &errKubeClient{}
	build.Status = api.BuildPending
	status, err := ctrl.synchronize(ctx, build)
	if err == nil {
		t.Error("Expected error, but none happened!")
	}
	if status != api.BuildFailed {
		t.Errorf("Expected BuildFailed, got %s!", status)
	}
}

func TestSynchronizeBuildPending(t *testing.T) {
	ctrl, build, ctx := setup()
	build.Status = api.BuildPending
	status, err := ctrl.synchronize(ctx, build)
	if err != nil {
		t.Errorf("Unexpected error: %s!", err.Error())
	}
	if status != api.BuildRunning {
		t.Errorf("Expected BuildRunning, got %s!", status)
	}
}

func TestSynchronizeBuildRunningTimedOut(t *testing.T) {
	ctrl, build, ctx := setup()
	build.Status = api.BuildRunning
	build.CreationTimestamp.Time = time.Date(0, 0, 0, 0, 0, 0, 0, time.UTC)
	status, err := ctrl.synchronize(ctx, build)
	if err == nil {
		t.Error("Expected error, but none happened!")
	}
	if status != api.BuildFailed {
		t.Errorf("Expected BuildFailed, got %s!", status)
	}
}

func TestSynchronizeBuildRunningFailedGetPod(t *testing.T) {
	ctrl, build, ctx := setup()
	ctrl.kubeClient = &errKubeClient{}
	build.Status = api.BuildRunning
	build.CreationTimestamp.Time = time.Now()
	status, err := ctrl.synchronize(ctx, build)
	if err == nil {
		t.Error("Expected error, but none happened!")
	}
	if status != api.BuildRunning {
		t.Errorf("Expected BuildRunning, got %s!", status)
	}
}

func TestSynchronizeBuildRunningPodRunning(t *testing.T) {
	ctrl, build, ctx := setup()
	build.Status = api.BuildRunning
	build.CreationTimestamp.Time = time.Now()
	status, err := ctrl.synchronize(ctx, build)
	if err != nil {
		t.Errorf("Unexpected error, got %s!", err.Error())
	}
	if status != api.BuildRunning {
		t.Errorf("Expected BuildRunning, got %s!", status)
	}
}

func TestSynchronizeBuildRunningPodTerminated(t *testing.T) {
	ctrl, build, ctx := setup()
	ctrl.kubeClient = &okKubeClient{}
	build.Status = api.BuildRunning
	build.CreationTimestamp.Time = time.Now()
	status, err := ctrl.synchronize(ctx, build)
	if err != nil {
		t.Errorf("Unexpected error, got %s!", err.Error())
	}
	if status != api.BuildComplete {
		t.Errorf("Expected BuildRunning, got %s!", status)
	}
}

func TestSynchronizeBuildComplete(t *testing.T) {
	ctrl, build, ctx := setup()
	build.Status = api.BuildComplete
	status, err := ctrl.synchronize(ctx, build)
	if err != nil {
		t.Errorf("Unexpected error, got %s!", err.Error())
	}
	if status != api.BuildComplete {
		t.Errorf("Expected BuildComplete, got %s!", status)
	}
}

func TestSynchronizeBuildFailed(t *testing.T) {
	ctrl, build, ctx := setup()
	build.Status = api.BuildFailed
	status, err := ctrl.synchronize(ctx, build)
	if err != nil {
		t.Errorf("Unexpected error, got %s!", err.Error())
	}
	if status != api.BuildFailed {
		t.Errorf("Expected BuildFailed, got %s!", status)
	}
}

func TestSynchronizeBuildError(t *testing.T) {
	ctrl, build, ctx := setup()
	build.Status = api.BuildError
	status, err := ctrl.synchronize(ctx, build)
	if err != nil {
		t.Errorf("Unexpected error, got %s!", err.Error())
	}
	if status != api.BuildError {
		t.Errorf("Expected BuildError, got %s!", status)
	}
}

func TestSynchronizeBuildUnknownStatus(t *testing.T) {
	ctrl, build, ctx := setup()
	build.Status = "unknownBuildStatus"
	status, err := ctrl.synchronize(ctx, build)
	if err == nil {
		t.Error("Expected error, but none happened!")
	}
	if status != api.BuildError {
		t.Errorf("Expected BuildError, got %s!", status)
	}
}

func setup() (buildController *BuildController, build *api.Build, ctx kapi.Context) {
	buildController = &BuildController{
		buildStrategies: map[api.BuildType]BuildJobStrategy{
			"okStrategy": &okStrategy{},
		},
		kubeClient: &kubeclient.Fake{},
		timeout:    1000,
	}
	build = &api.Build{
		JSONBase: kapi.JSONBase{
			ID: "dataBuild",
		},
		Input: api.BuildInput{
			Type:      "okStrategy",
			SourceURI: "http://my.build.com/the/build/Dockerfile",
			ImageTag:  "repository/dataBuild",
		},
		Status: api.BuildNew,
		PodID:  "-the-pod-id",
		Labels: map[string]string{
			"name": "dataBuild",
		},
	}
	ctx = kapi.NewDefaultContext()
	return
}
