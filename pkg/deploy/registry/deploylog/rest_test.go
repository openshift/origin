package deploylog

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	genericrest "k8s.io/kubernetes/pkg/registry/generic/rest"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/client/testclient"
	"github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

var testSelector = map[string]string{"test": "rest"}

func makeDeployment(version int64) kapi.ReplicationController {
	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(version), kapi.Codecs.LegacyCodec(api.SchemeGroupVersion))
	deployment.Namespace = kapi.NamespaceDefault
	deployment.Spec.Selector = testSelector
	return *deployment
}

func makeDeploymentList(versions int64) *kapi.ReplicationControllerList {
	list := &kapi.ReplicationControllerList{}
	for v := int64(1); v <= versions; v++ {
		list.Items = append(list.Items, makeDeployment(v))
	}
	return list
}

var (
	fakePodList = &kapi.PodList{
		Items: []kapi.Pod{
			{
				ObjectMeta: kapi.ObjectMeta{
					Name:              "config-5-application-pod-1",
					Namespace:         kapi.NamespaceDefault,
					CreationTimestamp: unversioned.Date(2016, time.February, 1, 1, 0, 1, 0, time.UTC),
					Labels:            testSelector,
				},
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
						{
							Name: "config-5-container-1",
						},
					},
					NodeName: "some-host",
				},
			},
			{
				ObjectMeta: kapi.ObjectMeta{
					Name:              "config-5-application-pod-2",
					Namespace:         kapi.NamespaceDefault,
					CreationTimestamp: unversioned.Date(2016, time.February, 1, 1, 0, 3, 0, time.UTC),
					Labels:            testSelector,
				},
				Spec: kapi.PodSpec{
					Containers: []kapi.Container{
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
func mockREST(version, desired int64, status api.DeploymentStatus) *REST {
	connectionInfo := &kubeletclient.HTTPKubeletClient{Config: &kubeletclient.KubeletClientConfig{EnableHttps: true, Port: 12345}, Client: &http.Client{}}

	// Fake deploymentConfig
	config := deploytest.OkDeploymentConfig(version)
	fakeDn := testclient.NewSimpleFake(config)
	fakeDn.PrependReactor("get", "deploymentconfigs", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, config, nil
	})

	// Used for testing validation errors prior to getting replication controllers.
	if desired > version {
		return &REST{
			dn:       fakeDn,
			connInfo: connectionInfo,
			timeout:  defaultTimeout,
		}
	}

	// Fake deployments
	fakeDeployments := makeDeploymentList(version)
	fakeRn := ktestclient.NewSimpleFake(fakeDeployments)
	fakeRn.PrependReactor("get", "replicationcontrollers", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, &fakeDeployments.Items[desired-1], nil
	})

	// Fake watcher for deployments
	fakeWatch := watch.NewFake()
	fakeRn.PrependWatchReactor("replicationcontrollers", ktestclient.DefaultWatchReactor(fakeWatch, nil))
	obj := &fakeDeployments.Items[desired-1]
	obj.Annotations[api.DeploymentStatusAnnotation] = string(status)
	go fakeWatch.Add(obj)

	fakePn := ktestclient.NewSimpleFake()
	if status == api.DeploymentStatusComplete {
		// If the deployment is complete, we will try to get the logs from the oldest
		// application pod...
		fakePn.PrependReactor("list", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			return true, fakePodList, nil
		})
		fakePn.PrependReactor("get", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			return true, &fakePodList.Items[0], nil
		})
	} else {
		// ...otherwise try to get the logs from the deployer pod.
		fakeDeployer := &kapi.Pod{
			ObjectMeta: kapi.ObjectMeta{
				Name:      deployutil.DeployerPodNameForDeployment(obj.Name),
				Namespace: kapi.NamespaceDefault,
			},
			Spec: kapi.PodSpec{
				Containers: []kapi.Container{
					{
						Name: deployutil.DeployerPodNameForDeployment(obj.Name) + "-container",
					},
				},
				NodeName: "some-host",
			},
			Status: kapi.PodStatus{
				Phase: kapi.PodRunning,
			},
		}
		fakePn.PrependReactor("get", "pods", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			return true, fakeDeployer, nil
		})
	}

	return &REST{
		dn:       fakeDn,
		rn:       fakeRn,
		pn:       fakePn,
		connInfo: connectionInfo,
		timeout:  defaultTimeout,
	}
}

func TestRESTGet(t *testing.T) {
	ctx := kapi.NewDefaultContext()

	tests := []struct {
		testName    string
		rest        *REST
		name        string
		opts        runtime.Object
		expected    runtime.Object
		expectedErr error
	}{
		{
			testName: "running deployment",
			rest:     mockREST(1, 1, api.DeploymentStatusRunning),
			name:     "config",
			opts:     &api.DeploymentLogOptions{Follow: true, Version: intp(1)},
			expected: &genericrest.LocationStreamer{
				Location: &url.URL{
					Scheme:   "https",
					Host:     "some-host:12345",
					Path:     "/containerLogs/default/config-1-deploy/config-1-deploy-container",
					RawQuery: "follow=true",
				},
				Transport:       nil,
				ContentType:     "text/plain",
				Flush:           true,
				ResponseChecker: genericrest.NewGenericHttpResponseChecker(kapi.Resource("pod"), "config-1-deploy"),
			},
			expectedErr: nil,
		},
		{
			testName: "complete deployment",
			rest:     mockREST(5, 5, api.DeploymentStatusComplete),
			name:     "config",
			opts:     &api.DeploymentLogOptions{Follow: true, Version: intp(5)},
			expected: &genericrest.LocationStreamer{
				Location: &url.URL{
					Scheme:   "https",
					Host:     "some-host:12345",
					Path:     "/containerLogs/default/config-5-application-pod-1/config-5-container-1",
					RawQuery: "follow=true",
				},
				Transport:       nil,
				ContentType:     "text/plain",
				Flush:           true,
				ResponseChecker: genericrest.NewGenericHttpResponseChecker(kapi.Resource("pod"), "config-5-application-pod-1"),
			},
			expectedErr: nil,
		},
		{
			testName: "previous failed deployment",
			rest:     mockREST(3, 2, api.DeploymentStatusFailed),
			name:     "config",
			opts:     &api.DeploymentLogOptions{Follow: false, Version: intp(2)},
			expected: &genericrest.LocationStreamer{
				Location: &url.URL{
					Scheme: "https",
					Host:   "some-host:12345",
					Path:   "/containerLogs/default/config-2-deploy/config-2-deploy-container",
				},
				Transport:       nil,
				ContentType:     "text/plain",
				Flush:           false,
				ResponseChecker: genericrest.NewGenericHttpResponseChecker(kapi.Resource("pod"), "config-2-deploy"),
			},
			expectedErr: nil,
		},
		{
			testName: "previous deployment",
			rest:     mockREST(3, 2, api.DeploymentStatusFailed),
			name:     "config",
			opts:     &api.DeploymentLogOptions{Follow: false, Previous: true},
			expected: &genericrest.LocationStreamer{
				Location: &url.URL{
					Scheme: "https",
					Host:   "some-host:12345",
					Path:   "/containerLogs/default/config-2-deploy/config-2-deploy-container",
				},
				Transport:       nil,
				ContentType:     "text/plain",
				Flush:           false,
				ResponseChecker: genericrest.NewGenericHttpResponseChecker(kapi.Resource("pod"), "config-2-deploy"),
			},
			expectedErr: nil,
		},
		{
			testName:    "non-existent previous deployment",
			rest:        mockREST(1 /* won't be used */, 101, ""),
			name:        "config",
			opts:        &api.DeploymentLogOptions{Follow: false, Previous: true},
			expected:    nil,
			expectedErr: errors.NewBadRequest("no previous deployment exists for deploymentConfig \"config\""),
		},
	}

	for _, test := range tests {
		got, err := test.rest.Get(ctx, test.name, test.opts)
		if err != nil && test.expectedErr != nil && err.Error() != test.expectedErr.Error() {
			t.Errorf("%s: error mismatch: expected %v, got %v", test.testName, test.expectedErr, err)
			continue
		}
		if err != nil && test.expectedErr == nil {
			t.Errorf("%s: error mismatch: expected no error, got %v", test.testName, err)
			continue
		}
		if err == nil && test.expectedErr != nil {
			t.Errorf("%s: error mismatch: expected %v, got no error", test.testName, test.expectedErr)
			continue
		}
		if !reflect.DeepEqual(got, test.expected) {
			t.Errorf("%s: location streamer mismatch: expected\n%#v\ngot\n%#v\n", test.testName, test.expected, got)
			e := test.expected.(*genericrest.LocationStreamer)
			a := got.(*genericrest.LocationStreamer)
			if e.Location.String() != a.Location.String() {
				t.Errorf("%s: expected url:\n%v\ngot:\n%v\n", test.testName, e.Location, a.Location)
			}
		}
	}
}

// TODO: These kind of functions seem to be used in lots of places
// We should move it in a common location
func intp(num int64) *int64 {
	return &num
}
