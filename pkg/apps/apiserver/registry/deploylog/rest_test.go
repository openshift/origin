package deploylog

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/kubernetes/fake"
	fakeexternal "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsv1 "github.com/openshift/api/apps/v1"
	appsfake "github.com/openshift/client-go/apps/clientset/versioned/fake"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	appstest "github.com/openshift/origin/pkg/apps/util/test"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

var testSelector = map[string]string{"test": "rest"}

func makeDeployment(version int64) corev1.ReplicationController {
	deployment, err := appsutil.MakeDeployment(appstest.OkDeploymentConfig(version))
	if err != nil {
		panic(err)
	}
	deployment.Namespace = metav1.NamespaceDefault
	deployment.Spec.Selector = testSelector
	return *deployment
}

func makeDeploymentList(versions int64) *corev1.ReplicationControllerList {
	list := &corev1.ReplicationControllerList{}
	for v := int64(1); v <= versions; v++ {
		list.Items = append(list.Items, makeDeployment(v))
	}
	return list
}

var (
	fakePodList = &corev1.PodList{
		Items: []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "config-5-application-pod-1",
					Namespace:         metav1.NamespaceDefault,
					CreationTimestamp: metav1.Date(2016, time.February, 1, 1, 0, 1, 0, time.UTC),
					Labels:            testSelector,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "config-5-container-1",
						},
					},
					NodeName: "some-host",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "config-5-application-pod-2",
					Namespace:         metav1.NamespaceDefault,
					CreationTimestamp: metav1.Date(2016, time.February, 1, 1, 0, 3, 0, time.UTC),
					Labels:            testSelector,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "config-5-container-2",
						},
					},
					NodeName: "some-host",
				},
			},
		},
	}
)

// mockREST mocks a DeploymentLog REST
func mockREST(version, desired int64, status appsv1.DeploymentStatus) *REST {
	// Fake deploymentConfig
	config := appstest.OkDeploymentConfig(version)

	fakeDn := appsfake.NewSimpleClientset(config)
	fakeDn.PrependReactor("get", "deploymentconfigs", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, config, nil
	})

	// Used for testing validation errors prior to getting replication controllers.
	if desired > version {
		return &REST{
			dcClient: fakeDn.Apps(),
			timeout:  defaultTimeout,
		}
	}

	// Fake deployments
	fakeDeployments := makeDeploymentList(version)
	fakeRn := fake.NewSimpleClientset(fakeDeployments)
	fakeRn.PrependReactor("get", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &fakeDeployments.Items[desired-1], nil
	})

	// Fake watcher for deployments
	fakeWatch := watch.NewFake()
	fakeRn.PrependWatchReactor("replicationcontrollers", clientgotesting.DefaultWatchReactor(fakeWatch, nil))
	obj := &fakeDeployments.Items[desired-1]
	obj.Annotations[appsv1.DeploymentStatusAnnotation] = string(status)
	go fakeWatch.Add(obj)

	fakePn := fakeexternal.NewSimpleClientset()
	if status == appsv1.DeploymentStatusComplete {
		// If the deployment is complete, we will try to get the logs from the oldest
		// application pod...
		fakePn.PrependReactor("list", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, fakePodList, nil
		})
		fakePn.PrependReactor("get", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, &fakePodList.Items[0], nil
		})
	} else {
		// ...otherwise try to get the logs from the deployer pod.
		fakeDeployer := &kapi.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      appsutil.DeployerPodNameForDeployment(obj.Name),
				Namespace: metav1.NamespaceDefault,
			},
			Spec: kapi.PodSpec{
				Containers: []kapi.Container{
					{
						Name: appsutil.DeployerPodNameForDeployment(obj.Name) + "-container",
					},
				},
				NodeName: "some-host",
			},
			Status: kapi.PodStatus{
				Phase: kapi.PodRunning,
			},
		}
		fakePn.PrependReactor("get", "pods", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, fakeDeployer, nil
		})
	}

	return &REST{
		dcClient:  fakeDn.Apps(),
		rcClient:  fakeRn.Core(),
		podClient: fakePn.Core(),
		timeout:   defaultTimeout,
	}
}

func TestRESTGet(t *testing.T) {
	ctx := apirequest.NewDefaultContext()

	tests := []struct {
		testName          string
		rest              *REST
		name              string
		opts              runtime.Object
		expectedNamespace string
		expectedName      string
		expectedErr       error
	}{
		{
			testName:          "running deployment",
			rest:              mockREST(1, 1, appsv1.DeploymentStatusRunning),
			name:              "config",
			opts:              &appsapi.DeploymentLogOptions{Follow: true, Version: intp(1)},
			expectedNamespace: "default",
			expectedName:      "config-1-deploy",
			expectedErr:       nil,
		},
		{
			testName:          "complete deployment",
			rest:              mockREST(5, 5, appsv1.DeploymentStatusComplete),
			name:              "config",
			opts:              &appsapi.DeploymentLogOptions{Follow: true, Version: intp(5)},
			expectedNamespace: "default",
			expectedName:      "config-5-application-pod-1",
			expectedErr:       nil,
		},
		{
			testName:          "previous failed deployment",
			rest:              mockREST(3, 2, appsv1.DeploymentStatusFailed),
			name:              "config",
			opts:              &appsapi.DeploymentLogOptions{Follow: false, Version: intp(2)},
			expectedNamespace: "default",
			expectedName:      "config-2-deploy",
			expectedErr:       nil,
		},
		{
			testName:          "previous deployment",
			rest:              mockREST(3, 2, appsv1.DeploymentStatusFailed),
			name:              "config",
			opts:              &appsapi.DeploymentLogOptions{Follow: false, Previous: true},
			expectedNamespace: "default",
			expectedName:      "config-2-deploy",
			expectedErr:       nil,
		},
		{
			testName:    "non-existent previous deployment",
			rest:        mockREST(1 /* won't be used */, 101, ""),
			name:        "config",
			opts:        &appsapi.DeploymentLogOptions{Follow: false, Previous: true},
			expectedErr: errors.NewBadRequest("no previous deployment exists for deploymentConfig \"config\""),
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			actualPodNamespace := ""
			actualPodName := ""
			getPodLogsFn := func(podNamespace, podName string, logOpts *corev1.PodLogOptions) (runtime.Object, error) {
				actualPodNamespace = podNamespace
				actualPodName = podName
				return nil, nil
			}

			test.rest.getLogsFn = getPodLogsFn
			_, err := test.rest.Get(ctx, test.name, test.opts)
			if err != nil && test.expectedErr != nil && err.Error() != test.expectedErr.Error() {
				t.Fatalf("error mismatch: expected %v, got %v", test.expectedErr, err)
			}
			if err != nil && test.expectedErr == nil {
				t.Fatal(err)
			}
			if err == nil && test.expectedErr != nil {
				t.Fatalf("error mismatch: expected %v, got no error", test.expectedErr)
			}
			if e, a := test.expectedNamespace, actualPodNamespace; e != a {
				t.Errorf("expected %v, actual %v", e, a)
			}
			if e, a := test.expectedName, actualPodName; e != a {
				t.Errorf("expected %v, actual %v", e, a)
			}
		})
	}
}

// TODO: These kind of functions seem to be used in lots of places
// We should move it in a common location
func intp(num int64) *int64 {
	return &num
}
