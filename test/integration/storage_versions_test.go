package integration

import (
	"path"
	"testing"

	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/batch"
	batch_v1 "k8s.io/kubernetes/pkg/apis/batch/v1"
	"k8s.io/kubernetes/pkg/apis/extensions"
	extensions_v1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kautoscalingclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/autoscaling/internalversion"
	kbatchclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/batch/internalversion"
	kclientset15 "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_5"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/runtime"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

type legacyExtensionsAutoscaling struct {
	kautoscalingclient.HorizontalPodAutoscalerInterface
	client    restclient.Interface
	namespace string
}

// List takes label and field selectors, and returns the list of horizontalPodAutoscalers that match those selectors.
func (c legacyExtensionsAutoscaling) List(opts kapi.ListOptions) (result *autoscaling.HorizontalPodAutoscalerList, err error) {
	result = &autoscaling.HorizontalPodAutoscalerList{}
	err = c.client.Get().Namespace(c.namespace).Resource("horizontalPodAutoscalers").VersionedParams(&opts, kapi.ParameterCodec).Do().Into(result)
	return
}

func (c legacyExtensionsAutoscaling) Create(hpa *autoscaling.HorizontalPodAutoscaler) (*autoscaling.HorizontalPodAutoscaler, error) {
	var result autoscaling.HorizontalPodAutoscaler
	return &result, c.client.Post().Resource("horizontalpodautoscalers").Namespace(c.namespace).Body(hpa).Do().Into(&result)
}

func (c legacyExtensionsAutoscaling) Get(name string) (*autoscaling.HorizontalPodAutoscaler, error) {
	var result autoscaling.HorizontalPodAutoscaler
	return &result, c.client.Get().Resource("horizontalpodautoscalers").Namespace(c.namespace).Name(name).Do().Into(&result)
}

func getGVKFromEtcd(etcdClient etcd.Client, masterConfig *configapi.MasterConfig, prefix, ns, name string) (*unversioned.GroupVersionKind, error) {
	keys := etcd.NewKeysAPI(etcdClient)
	key := path.Join(masterConfig.EtcdStorageConfig.KubernetesStoragePrefix, prefix, ns, name)
	resp, err := keys.Get(context.TODO(), key, nil)
	if err != nil {
		return nil, err
	}
	_, gvk, err := runtime.UnstructuredJSONScheme.Decode([]byte(resp.Node.Value), nil, nil)
	return gvk, err
}

func setupStorageTests(t *testing.T, ns string) (*configapi.MasterConfig, kclientset.Interface, kclientset15.Interface) {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// create the containing project
	if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, ns, "admin"); err != nil {
		t.Fatalf("unexpected error creating the project: %v", err)
	}
	projectAdminClient, projectAdminKubeClient, projectAdminKubeConfig, err := testutil.GetClientForUser(*clusterAdminClientConfig, "admin")
	if err != nil {
		t.Fatalf("unexpected error getting project admin client: %v", err)
	}
	projectAdminKubeClient15 := kclientset15.NewForConfigOrDie(projectAdminKubeConfig)
	if err := testutil.WaitForPolicyUpdate(projectAdminClient, ns, "get", extensions.Resource("horizontalpodautoscalers"), true); err != nil {
		t.Fatalf("unexpected error waiting for policy update: %v", err)
	}

	return masterConfig, projectAdminKubeClient, projectAdminKubeClient15
}

func TestStorageVersions(t *testing.T) {
	ns := "storageversions"
	autoscalingVersion := extensions_v1beta1.SchemeGroupVersion
	batchVersion := batch_v1.SchemeGroupVersion

	defer testutil.DumpEtcdOnFailure(t)
	etcdServer := testutil.RequireEtcd(t)
	masterConfig, kubeClient, kubeClient15 := setupStorageTests(t, ns)

	jobTestcases := map[string]struct {
		creator kbatchclient.JobInterface
	}{
		"batch": {creator: kubeClient.Batch().Jobs(ns)},
	}
	for name, testcase := range jobTestcases {
		job := batch.Job{
			ObjectMeta: kapi.ObjectMeta{Name: name + "-job"},
			Spec: batch.JobSpec{
				Template: kapi.PodTemplateSpec{
					Spec: kapi.PodSpec{
						RestartPolicy: kapi.RestartPolicyNever,
						Containers:    []kapi.Container{{Name: "containername", Image: "containerimage"}},
					},
				},
			},
		}

		// Create a Job
		if _, err := testcase.creator.Create(&job); err != nil {
			t.Fatalf("%s: unexpected error creating Job: %v", name, err)
		}

		// Ensure it is persisted correctly
		if gvk, err := getGVKFromEtcd(etcdServer.Client, masterConfig, "jobs", ns, job.Name); err != nil {
			t.Fatalf("%s: unexpected error reading Job: %v", name, err)
		} else if *gvk != batchVersion.WithKind("Job") {
			t.Fatalf("%s: expected api version %s in etcd, got %s reading Job", name, batchVersion, gvk)
		}

		// Ensure it is accessible from both APIs
		if _, err := kubeClient.Batch().Jobs(ns).Get(job.Name); err != nil {
			t.Errorf("%s: Error reading Job from the batch client: %#v", name, err)
		}
		if _, err := kubeClient15.Extensions().Jobs(ns).Get(job.Name); err != nil {
			t.Errorf("%s: Error reading Job from the extensions client: %#v", name, err)
		}
	}

	legacyClient := legacyExtensionsAutoscaling{
		kubeClient.Autoscaling().HorizontalPodAutoscalers(ns),
		kubeClient.Autoscaling().RESTClient(),
		ns,
	}
	hpaTestcases := map[string]struct {
		creator kautoscalingclient.HorizontalPodAutoscalerInterface
	}{
		"autoscaling": {creator: kubeClient.Autoscaling().HorizontalPodAutoscalers(ns)},
		"extensions": {
			creator: legacyClient,
		},
	}
	for name, testcase := range hpaTestcases {
		hpa := autoscaling.HorizontalPodAutoscaler{
			ObjectMeta: kapi.ObjectMeta{Name: name + "-hpa"},
			Spec: autoscaling.HorizontalPodAutoscalerSpec{
				MaxReplicas:    1,
				ScaleTargetRef: autoscaling.CrossVersionObjectReference{Kind: "ReplicationController", Name: "myrc"},
			},
		}

		// Create an HPA
		if _, err := testcase.creator.Create(&hpa); err != nil {
			t.Fatalf("%s: unexpected error creating HPA: %v", name, err)
		}

		// Make sure it is persisted correctly
		if gvk, err := getGVKFromEtcd(etcdServer.Client, masterConfig, "horizontalpodautoscalers", ns, hpa.Name); err != nil {
			t.Fatalf("%s: unexpected error reading HPA: %v", name, err)
		} else if *gvk != autoscalingVersion.WithKind("HorizontalPodAutoscaler") {
			t.Fatalf("%s: expected api version %s in etcd, got %s reading HPA", name, autoscalingVersion, gvk)
		}

		// Make sure it is available from the api
		if _, err := kubeClient.Autoscaling().HorizontalPodAutoscalers(ns).Get(hpa.Name); err != nil {
			t.Errorf("%s: Error reading HPA.autoscaling from the autoscaling/v1 API: %#v", name, err)
		}
		if _, err := legacyClient.Get(hpa.Name); err != nil {
			t.Errorf("%s: Error reading HPA.autoscaling from the extensions/v1beta1 API: %#v", name, err)
		}
	}
}

const extensionsv1beta1Job = `{"kind":"Job","apiVersion":"extensions/v1beta1","metadata":{"name":"extensionsv1beta1job","namespace":"storagemigration","selfLink":"/apis/batch/v1/namespaces/storagemigration/jobs/extensionsv1beta1job","uid":"4b5d9f60-dcd1-11e6-8d37-525400f25e34","creationTimestamp":"2017-01-17T16:23:35Z","labels":{"controller-uid":"4b5d9f60-dcd1-11e6-8d37-525400f25e34","job-name":"extensionsv1beta1job"}},"spec":{"parallelism":1,"completions":1,"selector":{"matchLabels":{"controller-uid":"4b5d9f60-dcd1-11e6-8d37-525400f25e34"}},"autoSelector":true,"template":{"metadata":{"creationTimestamp":null,"labels":{"controller-uid":"4b5d9f60-dcd1-11e6-8d37-525400f25e34","job-name":"extensionsv1beta1job"}},"spec":{"containers":[{"name":"containername","image":"containerimage","resources":{},"terminationMessagePath":"/dev/termination-log","imagePullPolicy":"Always"}],"restartPolicy":"Never","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","securityContext":{}}}},"status":{"startTime":"2017-01-17T16:23:35Z","active":1}}`

func TestStorageMigration(t *testing.T) {
	ns := "storagemigration"
	prefix := "jobs"
	jobName := "extensionsv1beta1job"
	batchVersion := batch_v1.SchemeGroupVersion

	defer testutil.DumpEtcdOnFailure(t)
	etcdServer := testutil.RequireEtcd(t)
	masterConfig, kubeClient, kubeClient15 := setupStorageTests(t, ns)

	// Save an extensions/v1beta1.Job directly in etcd
	keys := etcd.NewKeysAPI(etcdServer.Client)
	key := path.Join(masterConfig.EtcdStorageConfig.KubernetesStoragePrefix, prefix, ns, jobName)
	if _, err := keys.Create(context.TODO(), key, extensionsv1beta1Job); err != nil {
		t.Fatalf("Unexpected error saving extensions/v1beta1.Job: %v", err)
	}

	// Ensure it is accessible from both APIs
	job, err := kubeClient.Batch().Jobs(ns).Get(jobName)
	if err != nil {
		t.Errorf("Error reading Job from the batch client: %#v", err)
	}
	if _, err := kubeClient15.Extensions().Jobs(ns).Get(job.Name); err != nil {
		t.Errorf("Error reading Job from the extensions client: %#v", err)
	}

	// Update the job
	job.Spec.Parallelism = newInt32(2)
	if _, err := kubeClient.Batch().Jobs(ns).Update(job); err != nil {
		t.Errorf("Error updating Job: %#v", err)
	}

	// Ensure it is persisted as batch/v1.Job
	if gvk, err := getGVKFromEtcd(etcdServer.Client, masterConfig, prefix, ns, jobName); err != nil {
		t.Fatalf("Unexpected error reading Job from etcd: %v", err)
	} else if *gvk != batchVersion.WithKind("Job") {
		t.Fatalf("Expected api version %s in etcd, got %s reading Job", batchVersion, gvk)
	}
}

func newInt32(val int32) *int32 {
	p := new(int32)
	*p = val
	return p
}
