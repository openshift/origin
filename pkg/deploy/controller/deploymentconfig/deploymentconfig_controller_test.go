package deploymentconfig

import (
	"sort"
	"strconv"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"
	kfakeexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset/fake"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	kinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"

	"github.com/openshift/origin/pkg/client/testclient"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	_ "github.com/openshift/origin/pkg/deploy/api/install"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployv1 "github.com/openshift/origin/pkg/deploy/api/v1"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func alwaysReady() bool { return true }

func TestHandleScenarios(t *testing.T) {
	type deployment struct {
		// version is the deployment version
		version int64
		// replicas is the spec replicas of the deployment
		replicas int32
		// test is whether this is a test deployment config
		test bool
		// replicasA is the annotated replica value for backwards compat checks
		replicasA *int32
		desiredA  *int32
		status    deployapi.DeploymentStatus
		cancelled bool
	}

	mkdeployment := func(d deployment) *kapi.ReplicationController {
		config := deploytest.OkDeploymentConfig(d.version)
		if d.test {
			config = deploytest.TestDeploymentConfig(config)
		}
		config.Namespace = "test"
		deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion))
		deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(d.status)
		if d.cancelled {
			deployment.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue
			deployment.Annotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentCancelledNewerDeploymentExists
		}
		if d.desiredA != nil {
			deployment.Annotations[deployapi.DesiredReplicasAnnotation] = strconv.Itoa(int(*d.desiredA))
		} else {
			delete(deployment.Annotations, deployapi.DesiredReplicasAnnotation)
		}
		deployment.Spec.Replicas = d.replicas
		return deployment
	}

	tests := []struct {
		name string
		// replicas is the config replicas prior to the update
		replicas int32
		// test is whether this is a test deployment config
		test bool
		// newVersion is the version of the config at the time of the update
		newVersion int64
		// expectedReplicas is the expected config replica count after the update
		expectedReplicas int32
		// before is the state of all deployments prior to the update
		before []deployment
		// after is the expected state of all deployments after the update
		after []deployment
		// errExpected is whether the update should produce an error
		errExpected bool
	}{
		{
			name:             "version is zero",
			replicas:         1,
			newVersion:       0,
			expectedReplicas: 1,
			before:           []deployment{},
			after:            []deployment{},
			errExpected:      false,
		},
		{
			name:             "first deployment",
			replicas:         1,
			newVersion:       1,
			expectedReplicas: 1,
			before:           []deployment{},
			after: []deployment{
				{version: 1, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusNew, cancelled: false},
			},
			errExpected: false,
		},
		{
			name:             "initial deployment already in progress",
			replicas:         1,
			newVersion:       1,
			expectedReplicas: 1,
			before: []deployment{
				{version: 1, replicas: 1, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusNew, cancelled: false},
			},
			after: []deployment{
				{version: 1, replicas: 1, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusNew, cancelled: false},
			},
			errExpected: false,
		},
		{
			name:             "new version",
			replicas:         1,
			newVersion:       2,
			expectedReplicas: 1,
			before: []deployment{
				{version: 1, replicas: 1, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
			},
			after: []deployment{
				{version: 1, replicas: 1, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusNew, cancelled: false},
			},
			errExpected: false,
		},
		{
			name:             "already in progress",
			replicas:         1,
			newVersion:       2,
			expectedReplicas: 1,
			before: []deployment{
				{version: 1, replicas: 1, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusNew, cancelled: false},
			},
			after: []deployment{
				{version: 1, replicas: 1, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusNew, cancelled: false},
			},
			errExpected: false,
		},
		{
			name:             "already deployed",
			replicas:         1,
			newVersion:       1,
			expectedReplicas: 1,
			before: []deployment{
				{version: 1, replicas: 1, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
			},
			after: []deployment{
				{version: 1, replicas: 1, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
			},
			errExpected: false,
		},
		{
			name:             "awaiting cancellation of older deployments",
			replicas:         1,
			newVersion:       3,
			expectedReplicas: 1,
			before: []deployment{
				{version: 1, replicas: 1, replicasA: newInt32(1), desiredA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 1, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusRunning, cancelled: false},
			},
			after: []deployment{
				{version: 1, replicas: 1, replicasA: newInt32(1), desiredA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 1, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusRunning, cancelled: true},
			},
			errExpected: true,
		},
		{
			name:             "awaiting cancellation of older deployments (already cancelled)",
			replicas:         1,
			newVersion:       2,
			expectedReplicas: 1,
			before: []deployment{
				{version: 1, replicas: 1, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusRunning, cancelled: true},
			},
			after: []deployment{
				{version: 1, replicas: 1, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusRunning, cancelled: true},
			},
			errExpected: true,
		},
		{
			name:             "steady state replica corrections (latest == active)",
			replicas:         1,
			newVersion:       5,
			expectedReplicas: 1,
			before: []deployment{
				{version: 1, replicas: 0, replicasA: newInt32(0), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 1, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 3, replicas: 1, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: true},
				{version: 4, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: false},
				{version: 5, replicas: 1, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
			},
			after: []deployment{
				{version: 1, replicas: 0, replicasA: newInt32(0), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 0, replicasA: newInt32(0), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 3, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: true},
				{version: 4, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: false},
				{version: 5, replicas: 1, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
			},
			errExpected: false,
		},
		{
			name:             "steady state replica corrections (latest != active)",
			replicas:         1,
			newVersion:       5,
			expectedReplicas: 1,
			before: []deployment{
				{version: 1, replicas: 0, replicasA: newInt32(0), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 1, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 3, replicas: 1, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: true},
				{version: 4, replicas: 1, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 5, replicas: 1, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: false},
			},
			after: []deployment{
				{version: 1, replicas: 0, replicasA: newInt32(0), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 0, replicasA: newInt32(0), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 3, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: true},
				{version: 4, replicas: 1, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 5, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: false},
			},
			errExpected: false,
		},
		{
			name:             "already deployed, no active deployment",
			replicas:         1,
			newVersion:       2,
			expectedReplicas: 1,
			before: []deployment{
				{version: 1, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: false},
				{version: 2, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: false},
			},
			after: []deployment{
				{version: 1, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: false},
				{version: 2, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: false},
			},
			errExpected: false,
		},
		{
			name:             "scale up latest/active completed deployment",
			replicas:         5,
			newVersion:       2,
			expectedReplicas: 5,
			before: []deployment{
				{version: 1, replicas: 0, replicasA: newInt32(0), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 1, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
			},
			after: []deployment{
				{version: 1, replicas: 0, replicasA: newInt32(0), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 5, replicasA: newInt32(5), status: deployapi.DeploymentStatusComplete, cancelled: false},
			},
			errExpected: false,
		},
		{
			name:             "scale up active (not latest) completed deployment",
			replicas:         5,
			newVersion:       2,
			expectedReplicas: 5,
			before: []deployment{
				{version: 1, replicas: 1, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: true},
			},
			after: []deployment{
				{version: 1, replicas: 5, replicasA: newInt32(5), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: true},
			},
			errExpected: false,
		},
		{
			name:             "scale down latest/active completed deployment",
			replicas:         1,
			newVersion:       2,
			expectedReplicas: 1,
			before: []deployment{
				{version: 1, replicas: 0, replicasA: newInt32(0), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 5, replicasA: newInt32(5), status: deployapi.DeploymentStatusComplete, cancelled: false},
			},
			after: []deployment{
				{version: 1, replicas: 0, replicasA: newInt32(0), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 1, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
			},
			errExpected: false,
		},
		{
			name:             "scale down active (not latest) completed deployment",
			replicas:         1,
			newVersion:       2,
			expectedReplicas: 1,
			before: []deployment{
				{version: 1, replicas: 5, replicasA: newInt32(5), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: true},
			},
			after: []deployment{
				{version: 1, replicas: 1, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: true},
			},
			errExpected: false,
		},
		{
			name:             "fallback to last completed deployment",
			replicas:         1,
			newVersion:       2,
			expectedReplicas: 1,
			before: []deployment{
				{version: 1, replicas: 0, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: true},
			},
			after: []deployment{
				{version: 1, replicas: 1, replicasA: newInt32(1), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(1), status: deployapi.DeploymentStatusFailed, cancelled: true},
			},
			errExpected: false,
		},
		{
			name:             "fallback to last completed deployment (partial rollout)",
			replicas:         5,
			newVersion:       2,
			expectedReplicas: 5,
			before: []deployment{
				{version: 1, replicas: 2, replicasA: newInt32(5), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 2, replicasA: newInt32(0), desiredA: newInt32(5), status: deployapi.DeploymentStatusFailed, cancelled: true},
			},
			after: []deployment{
				{version: 1, replicas: 5, replicasA: newInt32(5), status: deployapi.DeploymentStatusComplete, cancelled: false},
				{version: 2, replicas: 0, replicasA: newInt32(0), desiredA: newInt32(5), status: deployapi.DeploymentStatusFailed, cancelled: true},
			},
			errExpected: false,
		},
	}

	for _, test := range tests {
		t.Logf("evaluating test: %s", test.name)

		var updatedConfig *deployapi.DeploymentConfig
		deployments := map[string]*kapi.ReplicationController{}
		toStore := []*kapi.ReplicationController{}
		for _, template := range test.before {
			deployment := mkdeployment(template)
			deployments[deployment.Name] = deployment
			toStore = append(toStore, deployment)
		}

		oc := &testclient.Fake{}
		oc.AddReactor("update", "deploymentconfigs", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			dc := action.(clientgotesting.UpdateAction).GetObject().(*deployapi.DeploymentConfig)
			updatedConfig = dc
			return true, dc, nil
		})
		kc := &fake.Clientset{}
		kc.AddReactor("create", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			rc := action.(clientgotesting.CreateAction).GetObject().(*kapi.ReplicationController)
			deployments[rc.Name] = rc
			return true, rc, nil
		})
		kc.AddReactor("update", "replicationcontrollers", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			rc := action.(clientgotesting.UpdateAction).GetObject().(*kapi.ReplicationController)
			deployments[rc.Name] = rc
			return true, rc, nil
		})
		codec := kapi.Codecs.LegacyCodec(deployv1.SchemeGroupVersion)

		dcInformer := cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return oc.DeploymentConfigs(metav1.NamespaceAll).List(options)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return oc.DeploymentConfigs(metav1.NamespaceAll).Watch(options)
				},
			},
			&deployapi.DeploymentConfig{},
			2*time.Minute,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		)

		kubeInformerFactory := kinformers.NewSharedInformerFactory(kc, 0)
		rcInformer := kubeInformerFactory.Core().InternalVersion().ReplicationControllers()
		podInformer := kubeInformerFactory.Core().InternalVersion().Pods()
		c := NewDeploymentConfigController(dcInformer, rcInformer, podInformer, oc, kc, kfakeexternal.NewSimpleClientset(), codec)
		c.dcStoreSynced = alwaysReady
		c.podListerSynced = alwaysReady
		c.rcListerSynced = alwaysReady

		for i := range toStore {
			rcInformer.Informer().GetStore().Add(toStore[i])
		}

		config := deploytest.OkDeploymentConfig(test.newVersion)
		if test.test {
			config = deploytest.TestDeploymentConfig(config)
		}
		config.Spec.Replicas = test.replicas
		config.Namespace = "test"

		if err := c.Handle(config); err != nil && !test.errExpected {
			t.Errorf("unexpected error: %s", err)
			continue
		}

		expectedDeployments := []*kapi.ReplicationController{}
		for _, template := range test.after {
			expectedDeployments = append(expectedDeployments, mkdeployment(template))
		}
		actualDeployments := []*kapi.ReplicationController{}
		for _, deployment := range deployments {
			actualDeployments = append(actualDeployments, deployment)
		}
		sort.Sort(deployutil.ByLatestVersionDesc(expectedDeployments))
		sort.Sort(deployutil.ByLatestVersionDesc(actualDeployments))

		if updatedConfig != nil {
			config = updatedConfig
		}

		if e, a := test.expectedReplicas, config.Spec.Replicas; e != a {
			t.Errorf("expected config replicas to be %d, got %d", e, a)
			continue
		}
		for i := 0; i < len(expectedDeployments); i++ {
			expected, actual := expectedDeployments[i], actualDeployments[i]
			if !kapi.Semantic.DeepEqual(expected, actual) {
				t.Errorf("actual deployment don't match expected: %v", diff.ObjectDiff(expected, actual))
			}
		}
	}
}

func newInt32(i int32) *int32 {
	return &i
}
