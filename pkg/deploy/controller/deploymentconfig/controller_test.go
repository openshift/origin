package deploymentconfig

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	api "github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client/testclient"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// TestHandle_initialOk ensures that an initial config (version 0) doesn't result
// in a new deployment.
func TestHandle_initialOk(t *testing.T) {
	controller := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(config, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call with deployment %v", deployment)
				return nil, nil
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				return &kapi.ReplicationControllerList{}, nil
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected update call with deployment %v", deployment)
				return nil, nil
			},
		},
		osClient: testclient.NewSimpleFake(deploytest.OkDeploymentConfig(0)),
	}

	err := controller.Handle(deploytest.OkDeploymentConfig(0))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestHandle_updateOk ensures that an updated config (version >0) results in
// a new deployment with the appropriate replica count based on a variety of
// existing prior deployments.
func TestHandle_updateOk(t *testing.T) {
	var (
		config              *deployapi.DeploymentConfig
		deployed            *kapi.ReplicationController
		existingDeployments *kapi.ReplicationControllerList
	)

	controller := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(config, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				deployed = deployment
				return deployment, nil
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				return existingDeployments, nil
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected update call with deployment %v", deployment)
				return nil, nil
			},
		},
		osClient: testclient.NewSimpleFake(),
	}

	type existing struct {
		version  int
		replicas int
		status   deployapi.DeploymentStatus
	}

	type scenario struct {
		version          int
		expectedReplicas int
		existing         []existing
	}

	scenarios := []scenario{
		{1, 1, []existing{}},
		{2, 1, []existing{
			{1, 1, deployapi.DeploymentStatusComplete},
		}},
		{3, 4, []existing{
			{1, 0, deployapi.DeploymentStatusComplete},
			{2, 4, deployapi.DeploymentStatusComplete},
		}},
		{3, 4, []existing{
			{1, 4, deployapi.DeploymentStatusComplete},
			{2, 1, deployapi.DeploymentStatusFailed},
		}},
		{4, 2, []existing{
			{1, 0, deployapi.DeploymentStatusComplete},
			{2, 0, deployapi.DeploymentStatusFailed},
			{3, 2, deployapi.DeploymentStatusComplete},
		}},
		// Scramble the order of the previous to ensure we still get it right.
		{4, 2, []existing{
			{2, 0, deployapi.DeploymentStatusFailed},
			{3, 2, deployapi.DeploymentStatusComplete},
			{1, 0, deployapi.DeploymentStatusComplete},
		}},
	}

	for _, scenario := range scenarios {
		deployed = nil
		config = deploytest.OkDeploymentConfig(scenario.version)
		config.Triggers = []deployapi.DeploymentTriggerPolicy{}
		existingDeployments = &kapi.ReplicationControllerList{}
		for _, e := range scenario.existing {
			d, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(e.version), api.Codec)
			d.Spec.Replicas = e.replicas
			d.Annotations[deployapi.DeploymentStatusAnnotation] = string(e.status)
			existingDeployments.Items = append(existingDeployments.Items, *d)
		}

		err := controller.Handle(config)

		if deployed == nil {
			t.Fatalf("expected a deployment")
		}

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		desired, hasDesired := deployutil.DeploymentDesiredReplicas(deployed)
		if !hasDesired {
			t.Fatalf("expected desired replicas")
		}
		if e, a := scenario.expectedReplicas, desired; e != a {
			t.Errorf("expected desired replicas %d, got %d", e, a)
		}
	}
}

// TestHandle_nonfatalLookupError ensures that an API failure to look up the
// existing deployment for an updated config results in a nonfatal error.
func TestHandle_nonfatalLookupError(t *testing.T) {
	configController := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(config, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call with deployment %v", deployment)
				return nil, nil
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				return nil, kerrors.NewInternalError(fmt.Errorf("fatal test error"))
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected update call with deployment %v", deployment)
				return nil, nil
			},
		},
		osClient: testclient.NewSimpleFake(),
	}

	err := configController.Handle(deploytest.OkDeploymentConfig(1))
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, isFatal := err.(fatalError); isFatal {
		t.Fatalf("expected a retryable error, got a fatal error: %v", err)
	}
}

// TestHandle_configAlreadyDeployed ensures that an attempt to create a
// deployment for an updated config for which the deployment was already
// created results in a no-op.
func TestHandle_configAlreadyDeployed(t *testing.T) {
	deploymentConfig := deploytest.OkDeploymentConfig(0)

	controller := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(config, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call to to create deployment: %v", deployment)
				return nil, nil
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				existingDeployments := []kapi.ReplicationController{}
				deployment, _ := deployutil.MakeDeployment(deploymentConfig, kapi.Codec)
				existingDeployments = append(existingDeployments, *deployment)
				return &kapi.ReplicationControllerList{Items: existingDeployments}, nil
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected update call with deployment %v", deployment)
				return nil, nil
			},
		},
		osClient: testclient.NewSimpleFake(deploymentConfig),
	}

	err := controller.Handle(deploymentConfig)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestHandle_nonfatalCreateError ensures that a failed API attempt to create
// a new deployment for an updated config results in a nonfatal error.
func TestHandle_nonfatalCreateError(t *testing.T) {
	configController := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(config, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				return nil, kerrors.NewInternalError(fmt.Errorf("test error"))
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				return &kapi.ReplicationControllerList{}, nil
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected update call with deployment %v", deployment)
				return nil, nil
			},
		},
		osClient: testclient.NewSimpleFake(),
	}

	err := configController.Handle(deploytest.OkDeploymentConfig(1))
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, isFatal := err.(fatalError); isFatal {
		t.Fatalf("expected a nonfatal error, got a fatal error: %v", err)
	}
}

// TestHandle_fatalError ensures that in internal (not API) failure to make a
// deployment from an updated config results in a fatal error.
func TestHandle_fatalError(t *testing.T) {
	configController := &DeploymentConfigController{
		makeDeployment: func(config *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return nil, fmt.Errorf("couldn't make deployment")
		},
		deploymentClient: &deploymentClientImpl{
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected call to create")
				return nil, kerrors.NewInternalError(fmt.Errorf("test error"))
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				return &kapi.ReplicationControllerList{}, nil
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				t.Fatalf("unexpected update call with deployment %v", deployment)
				return nil, nil
			},
		},
		osClient: testclient.NewSimpleFake(deploytest.OkDeploymentConfig(1)),
	}

	err := configController.Handle(deploytest.OkDeploymentConfig(1))
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, isFatal := err.(fatalError); !isFatal {
		t.Fatalf("expected a fatal error, got: %v", err)
	}
}

// TestHandle_existingDeployments ensures that an attempt to create a
// new deployment for a config that has existing deployments succeeds of fails
// depending upon the state of the existing deployments
func TestHandle_existingDeployments(t *testing.T) {
	var (
		config              *deployapi.DeploymentConfig
		deployed            *kapi.ReplicationController
		existingDeployments *kapi.ReplicationControllerList
		updatedDeployments  []kapi.ReplicationController
	)

	controller := &DeploymentConfigController{
		makeDeployment: func(cfg *deployapi.DeploymentConfig) (*kapi.ReplicationController, error) {
			return deployutil.MakeDeployment(cfg, api.Codec)
		},
		deploymentClient: &deploymentClientImpl{
			createDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				deployed = deployment
				return deployment, nil
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				return existingDeployments, nil
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				updatedDeployments = append(updatedDeployments, *deployment)
				return deployment, nil
			},
		},
		osClient: testclient.NewSimpleFake(),
	}

	type existing struct {
		version      int
		status       deployapi.DeploymentStatus
		shouldCancel bool
	}

	type scenario struct {
		version          int
		existing         []existing
		errorType        reflect.Type
		expectDeployment bool
	}

	transientErrorType := reflect.TypeOf(transientError(""))
	scenarios := []scenario{
		// No existing deployments
		{1, []existing{}, nil, true},
		// A single existing completed deployment
		{2, []existing{{1, deployapi.DeploymentStatusComplete, false}}, nil, true},
		// A single existing failed deployment
		{2, []existing{{1, deployapi.DeploymentStatusFailed, false}}, nil, true},
		// Multiple existing completed/failed deployments
		{3, []existing{{2, deployapi.DeploymentStatusFailed, false}, {1, deployapi.DeploymentStatusComplete, false}}, nil, true},

		// A single existing deployment in the default state
		{2, []existing{{1, "", false}}, transientErrorType, false},
		// A single existing new deployment
		{2, []existing{{1, deployapi.DeploymentStatusNew, false}}, transientErrorType, false},
		// A single existing pending deployment
		{2, []existing{{1, deployapi.DeploymentStatusPending, false}}, transientErrorType, false},
		// A single existing running deployment
		{2, []existing{{1, deployapi.DeploymentStatusRunning, false}}, transientErrorType, false},
		// Multiple existing deployments with one in new/pending/running
		{4, []existing{{3, deployapi.DeploymentStatusRunning, false}, {2, deployapi.DeploymentStatusComplete, false}, {1, deployapi.DeploymentStatusFailed, false}}, transientErrorType, false},

		// Latest deployment exists and has already failed/completed
		{2, []existing{{2, deployapi.DeploymentStatusFailed, false}, {1, deployapi.DeploymentStatusComplete, false}}, nil, false},
		// Latest deployment exists and is in new/pending/running state
		{2, []existing{{2, deployapi.DeploymentStatusRunning, false}, {1, deployapi.DeploymentStatusComplete, false}}, nil, false},

		// Multiple existing deployments with more than one in new/pending/running
		{4, []existing{{3, deployapi.DeploymentStatusNew, false}, {2, deployapi.DeploymentStatusRunning, true}, {1, deployapi.DeploymentStatusFailed, false}}, transientErrorType, false},
		// Multiple existing deployments with more than one in new/pending/running
		// Latest deployment has already failed
		{6, []existing{{5, deployapi.DeploymentStatusFailed, false}, {4, deployapi.DeploymentStatusRunning, false}, {3, deployapi.DeploymentStatusNew, true}, {2, deployapi.DeploymentStatusComplete, false}, {1, deployapi.DeploymentStatusNew, true}}, transientErrorType, false},
	}

	for _, scenario := range scenarios {
		updatedDeployments = []kapi.ReplicationController{}
		deployed = nil
		config = deploytest.OkDeploymentConfig(scenario.version)
		existingDeployments = &kapi.ReplicationControllerList{}
		for _, e := range scenario.existing {
			d, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(e.version), api.Codec)
			if e.status != "" {
				d.Annotations[deployapi.DeploymentStatusAnnotation] = string(e.status)
			}
			existingDeployments.Items = append(existingDeployments.Items, *d)
		}
		controller.osClient = testclient.NewSimpleFake(config)
		err := controller.Handle(config)

		if scenario.expectDeployment && deployed == nil {
			t.Fatalf("expected a deployment")
		}

		if scenario.errorType == nil {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		} else {
			if err == nil {
				t.Fatalf("expected error")
			}
			if reflect.TypeOf(err) != scenario.errorType {
				t.Fatalf("error expected: %s, got: %s", scenario.errorType, reflect.TypeOf(err))
			}
		}

		expectedCancellations := []int{}
		actualCancellations := []int{}
		for _, e := range scenario.existing {
			if e.shouldCancel {
				expectedCancellations = append(expectedCancellations, e.version)
			}
		}
		for _, d := range updatedDeployments {
			actualCancellations = append(actualCancellations, deployutil.DeploymentVersionFor(&d))
		}

		sort.Ints(actualCancellations)
		sort.Ints(expectedCancellations)
		if !reflect.DeepEqual(actualCancellations, expectedCancellations) {
			t.Fatalf("expected cancellations: %v, actual: %v", expectedCancellations, actualCancellations)
		}
	}
}

func TestFindDetails(t *testing.T) {
	existingDeployments := &kapi.ReplicationControllerList{}
	controller := &DeploymentConfigController{
		deploymentClient: &deploymentClientImpl{
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				return existingDeployments, nil
			},
		},
		osClient: testclient.NewSimpleFake(),
	}

	type reaction struct {
		verb, resource string
		fn             ktestclient.ReactionFunc
	}

	tests := []struct {
		name              string
		controller        *DeploymentConfigController
		version           int
		hasDeployments    bool
		hasNoImageChange  bool
		hasMultipleErrors bool
		status            deployapi.DeploymentStatus
		reactions         []reaction
		expectedDetails   string
		expectedLatest    bool
		expectedErr       bool
	}{
		{
			name: "cannot get existing deployments",
			controller: &DeploymentConfigController{
				deploymentClient: &deploymentClientImpl{
					listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
						return nil, errors.New("cannot return these deployments")
					},
				},
			},
			version:         0,
			hasDeployments:  false,
			expectedDetails: "",
			expectedLatest:  false,
			expectedErr:     true,
		},
		{
			name:            "complete latest",
			controller:      controller,
			version:         1,
			hasDeployments:  true,
			status:          deployapi.DeploymentStatusComplete,
			expectedDetails: "",
			expectedLatest:  true,
			expectedErr:     false,
		},
		{
			name:             "no image change triggers",
			controller:       controller,
			version:          1,
			hasDeployments:   true,
			hasNoImageChange: true,
			status:           deployapi.DeploymentStatusFailed,
			expectedDetails:  "",
			expectedLatest:   true,
			expectedErr:      false,
		},
		{
			name:           "cannot retrieve istag",
			controller:     controller,
			version:        1,
			hasDeployments: true,
			status:         deployapi.DeploymentStatusFailed,
			reactions: []reaction{
				{
					verb:     "get",
					resource: "imagestreamtags",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("unknown error while retrieving an istag")
					},
				},
			},
			expectedDetails: "",
			expectedLatest:  true,
			expectedErr:     true,
		},
		{
			name:           "found istag",
			controller:     controller,
			version:        1,
			hasDeployments: true,
			status:         deployapi.DeploymentStatusFailed,
			reactions: []reaction{
				{
					verb:     "get",
					resource: "imagestreamtags",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, &imageapi.ImageStreamTag{}, nil
					},
				},
			},
			expectedDetails: "",
			expectedLatest:  true,
			expectedErr:     false,
		},
		{
			name:           "not found istag",
			controller:     controller,
			version:        1,
			hasDeployments: true,
			status:         deployapi.DeploymentStatusFailed,
			reactions: []reaction{
				{
					verb:     "get",
					resource: "imagestreamtags",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, kerrors.NewNotFound("imagestreamtag", "test-image-stream:latest")
					},
				},
				{
					verb:     "list",
					resource: "buildconfigs",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, &buildapi.BuildConfigList{}, nil
					},
				},
				{
					verb:     "get",
					resource: "imagestreams",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, nil
					},
				},
			},
			expectedDetails: "The image trigger for image stream tag \"test-image-stream:latest\" will have no effect because image stream tag \"test-image-stream:latest\" does not exist.",
			expectedLatest:  true,
			expectedErr:     false,
		},
		{
			name:           "synthetic istag",
			controller:     controller,
			version:        1,
			hasDeployments: true,
			status:         deployapi.DeploymentStatusFailed,
			reactions: []reaction{
				{
					verb:     "get",
					resource: "imagestreamtags",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, kerrors.NewNotFound("imagestreamtag", "test-image-stream:"+imageapi.DefaultImageTag)
					},
				},
				{
					verb:     "list",
					resource: "buildconfigs",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, mkBuildConfigList(), nil
					},
				},
				{
					verb:     "get",
					resource: "imagestreams",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, nil
					},
				},
			},
			expectedDetails: "The image trigger for image stream tag \"test-image-stream:latest\" will have no effect because image stream tag \"test-image-stream:latest\" does not exist.\n\tIf image stream tag \"test-image-stream:latest\" is expected, check build config \"mybc\" which produces image stream tag \"test-image-stream:latest\".",
			expectedLatest:  true,
			expectedErr:     false,
		},
		{
			name:           "not found image stream",
			controller:     controller,
			version:        1,
			hasDeployments: true,
			status:         deployapi.DeploymentStatusFailed,
			reactions: []reaction{
				{
					verb:     "get",
					resource: "imagestreamtags",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, kerrors.NewNotFound("imagestreamtag", "test-image-stream:latest")
					},
				},
				{
					verb:     "get",
					resource: "imagestreams",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, kerrors.NewNotFound("imagestream", "test-image-stream")
					},
				},
			},
			expectedDetails: "The image trigger for image stream tag \"test-image-stream:latest\" will have no effect because image stream \"test-image-stream\" does not exist.",
			expectedLatest:  true,
			expectedErr:     false,
		},
		{
			name:       "no deployments - not found istag",
			controller: controller,
			version:    0,
			status:     deployapi.DeploymentStatusFailed,
			reactions: []reaction{
				{
					verb:     "get",
					resource: "imagestreamtags",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, kerrors.NewNotFound("imagestreamtag", "test-image-stream:latest")
					},
				},
				{
					verb:     "list",
					resource: "buildconfigs",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, &buildapi.BuildConfigList{}, nil
					},
				},
				{
					verb:     "get",
					resource: "imagestreams",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, nil
					},
				},
			},
			expectedDetails: "The image trigger for image stream tag \"test-image-stream:latest\" will have no effect because image stream tag \"test-image-stream:latest\" does not exist.",
			expectedErr:     false,
		},
		{
			name:       "no deployments - synthetic istag",
			controller: controller,
			version:    0,
			status:     deployapi.DeploymentStatusFailed,
			reactions: []reaction{
				{
					verb:     "get",
					resource: "imagestreamtags",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, kerrors.NewNotFound("imagestreamtag", "test-image-stream:"+imageapi.DefaultImageTag)
					},
				},
				{
					verb:     "list",
					resource: "buildconfigs",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, mkBuildConfigList(), nil
					},
				},
				{
					verb:     "get",
					resource: "imagestreams",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, nil
					},
				},
			},
			expectedDetails: "The image trigger for image stream tag \"test-image-stream:latest\" will have no effect because image stream tag \"test-image-stream:latest\" does not exist.\n\tIf image stream tag \"test-image-stream:latest\" is expected, check build config \"mybc\" which produces image stream tag \"test-image-stream:latest\".",
			expectedErr:     false,
		},
		{
			name:           "no deployments - not found image stream",
			controller:     controller,
			version:        1,
			hasDeployments: true,
			status:         deployapi.DeploymentStatusFailed,
			reactions: []reaction{
				{
					verb:     "get",
					resource: "imagestreamtags",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, kerrors.NewNotFound("imagestreamtag", "test-image-stream:latest")
					},
				},
				{
					verb:     "get",
					resource: "imagestreams",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, kerrors.NewNotFound("imagestream", "test-image-stream")
					},
				},
				{
					verb:     "get",
					resource: "imagestreams",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, kerrors.NewNotFound("imagestream", "test-image-stream")
					},
				},
			},
			expectedDetails: "The image trigger for image stream tag \"test-image-stream:latest\" will have no effect because image stream \"test-image-stream\" does not exist.",
			expectedLatest:  true,
			expectedErr:     false,
		},
		{
			name:              "multiple errors",
			controller:        controller,
			version:           0,
			hasMultipleErrors: true,
			status:            deployapi.DeploymentStatusFailed,
			reactions: []reaction{
				{
					verb:     "get",
					resource: "imagestreamtags",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, kerrors.NewNotFound("imagestreamtag", "test-image-stream:latest")
					},
				},
				{
					verb:     "get",
					resource: "imagestreamtags",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, kerrors.NewNotFound("imagestreamtag", "second-is:latest")
					},
				},
				{
					verb:     "list",
					resource: "buildconfigs",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, &buildapi.BuildConfigList{}, nil
					},
				},
				{
					verb:     "get",
					resource: "imagestreams",
					fn: func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, nil
					},
				},
			},
			expectedDetails: "Deployment config \"config\" blocked by multiple errors:\n\n\t* The image trigger for image stream tag \"test-image-stream:latest\" will have no effect because image stream tag \"test-image-stream:latest\" does not exist.\n\t* The image trigger for image stream tag \"second-is:latest\" will have no effect because image stream tag \"second-is:latest\" does not exist.",
			expectedErr:     false,
		},
	}

	for _, test := range tests {
		// Need to setup a couple of things before passing the config in findDetails
		// Namely, initalize the config, its triggers and its deployments
		config := deploytest.OkDeploymentConfig(test.version)
		if test.hasNoImageChange {
			config.Triggers = []deployapi.DeploymentTriggerPolicy{}
		} else if test.hasMultipleErrors {
			config.Triggers = append(config.Triggers, deployapi.DeploymentTriggerPolicy{
				Type: deployapi.DeploymentTriggerOnImageChange,
				ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
					Automatic: true,
					ContainerNames: []string{
						"container2",
					},
					From: kapi.ObjectReference{
						Kind: "ImageStream",
						Name: "second-is",
					},
					Tag: imageapi.DefaultImageTag,
				},
			})
		}
		// This should be recalculated for each scenario.
		config.Details = &deployapi.DeploymentDetails{
			Message: "preexisting message",
		}
		if test.hasDeployments {
			existingDeployments.Items = []kapi.ReplicationController{}
			d := mkdeployment(test.version)
			d.Annotations[deployapi.DeploymentStatusAnnotation] = string(test.status)
			existingDeployments.Items = append(mkDeploymentList(test.version-1).Items, d)
		}
		// We also have to setup fake client reactions for imagestreamtags and buildconfigs
		for _, act := range test.reactions {
			test.controller.osClient.(*testclient.Fake).PrependReactor(act.verb, act.resource, act.fn)
		}

		details, _, latest, err := test.controller.findDetails(config)
		// Check errors
		if err != nil && !test.expectedErr {
			t.Errorf("%s: didn't expect an error but got %v", test.name, err)
			continue
		}
		if err == nil && test.expectedErr {
			t.Errorf("%s: expected an error but got none", test.name)
			continue
		}

		// Check details
		if details != test.expectedDetails {
			t.Errorf("%s: details mismatch!\nExpected:\n%s\ngot:\n%s\n\n", test.name, test.expectedDetails, details)
			continue
		}

		// Check latest
		if latest != test.expectedLatest {
			t.Errorf("%s: latest mismatch! Expected %t but got %t", test.name, test.expectedLatest, latest)
			continue
		}
	}
}

func mkdeployment(version int) kapi.ReplicationController {
	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(version), kapi.Codec)
	return *deployment
}

func mkDeploymentList(versions int) *kapi.ReplicationControllerList {
	list := &kapi.ReplicationControllerList{}
	for v := 1; v <= versions; v++ {
		list.Items = append(list.Items, mkdeployment(v))
	}
	return list
}

func mkBuildConfigList() *buildapi.BuildConfigList {
	return &buildapi.BuildConfigList{
		Items: []buildapi.BuildConfig{
			{
				ObjectMeta: kapi.ObjectMeta{Name: "mybc"},
				Spec: buildapi.BuildConfigSpec{
					BuildSpec: buildapi.BuildSpec{
						Output: buildapi.BuildOutput{
							To: &kapi.ObjectReference{
								Name: "test-image-stream:" + imageapi.DefaultImageTag,
								Kind: "ImageStreamTag",
							},
						},
					},
				},
			},
		},
	}
}
