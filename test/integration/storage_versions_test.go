package integration

import (
	"path"
	"testing"

	"golang.org/x/net/context"

	etcd "github.com/coreos/etcd/client"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/extensions"
	extensions_v1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

// TODO: enable once storage is separable
// func TestStorageVersionsSeparated(t *testing.T) {
// 	runStorageTest(t, "separated",
// 		autoscaling_v1.SchemeGroupVersion,
// 		batch_v1.SchemeGroupVersion,
// 		extensions_v1beta1.SchemeGroupVersion,
// 	)
// }

func TestStorageVersionsUnified(t *testing.T) {
	defer testutil.DumpEtcdOnFailure(t)
	runStorageTest(t, "unified",
		extensions_v1beta1.SchemeGroupVersion,
		extensions_v1beta1.SchemeGroupVersion,
		extensions_v1beta1.SchemeGroupVersion,
	)
}

type legacyExtensionsAutoscaling struct {
	kclient.HorizontalPodAutoscalerInterface
	client    *restclient.RESTClient
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

func runStorageTest(t *testing.T, ns string, autoscalingVersion, batchVersion, extensionsVersion unversioned.GroupVersion) {
	etcdServer := testutil.RequireEtcd(t)

	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	keys := etcd.NewKeysAPI(etcdServer.Client)
	getGVKFromEtcd := func(prefix, name string) (*unversioned.GroupVersionKind, error) {
		key := path.Join(masterConfig.EtcdStorageConfig.KubernetesStoragePrefix, prefix, ns, name)
		resp, err := keys.Get(context.TODO(), key, nil)
		if err != nil {
			return nil, err
		}
		_, gvk, err := runtime.UnstructuredJSONScheme.Decode([]byte(resp.Node.Value), nil, nil)
		return gvk, err
	}

	// TODO: Set storage versions for API groups
	// masterConfig.EtcdStorageConfig.StorageVersions[autoscaling.GroupName] = autoscalingVersion.String()
	// masterConfig.EtcdStorageConfig.StorageVersions[batch.GroupName] = batchVersion.String()
	// masterConfig.EtcdStorageConfig.StorageVersions[extensions.GroupName] = extensionsVersion.String()

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
	projectAdminClient, projectAdminKubeClient, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "admin")
	if err != nil {
		t.Fatalf("unexpected error getting project admin client: %v", err)
	}
	if err := testutil.WaitForPolicyUpdate(projectAdminClient, ns, "get", extensions.Resource("horizontalpodautoscalers"), true); err != nil {
		t.Fatalf("unexpected error waiting for policy update: %v", err)
	}

	jobTestcases := map[string]struct {
		creator kclient.JobInterface
	}{
		"batch": {creator: projectAdminKubeClient.Batch().Jobs(ns)},
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
		if gvk, err := getGVKFromEtcd("jobs", job.Name); err != nil {
			t.Fatalf("%s: unexpected error reading Job: %v", name, err)
		} else if *gvk != batchVersion.WithKind("Job") {
			t.Fatalf("%s: expected api version %s in etcd, got %s reading Job", name, batchVersion, gvk)
		}

		// Ensure it is accessible from both APIs
		if _, err := projectAdminKubeClient.Batch().Jobs(ns).Get(job.Name); err != nil {
			t.Errorf("%s: Error reading Job from the batch client: %#v", name, err)
		}
		if _, err := projectAdminKubeClient.Extensions().Jobs(ns).Get(job.Name); err != nil {
			t.Errorf("%s: Error reading Job from the extensions client: %#v", name, err)
		}
	}

	legacyClient := legacyExtensionsAutoscaling{
		projectAdminKubeClient.Autoscaling().HorizontalPodAutoscalers(ns),
		projectAdminKubeClient.AutoscalingClient.RESTClient,
		ns,
	}
	hpaTestcases := map[string]struct {
		creator kclient.HorizontalPodAutoscalerInterface
	}{
		"autoscaling": {creator: projectAdminKubeClient.Autoscaling().HorizontalPodAutoscalers(ns)},
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
		if gvk, err := getGVKFromEtcd("horizontalpodautoscalers", hpa.Name); err != nil {
			t.Fatalf("%s: unexpected error reading HPA: %v", name, err)
		} else if *gvk != autoscalingVersion.WithKind("HorizontalPodAutoscaler") {
			t.Fatalf("%s: expected api version %s in etcd, got %s reading HPA", name, autoscalingVersion, gvk)
		}

		// Make sure it is available from the api
		if _, err := projectAdminKubeClient.Autoscaling().HorizontalPodAutoscalers(ns).Get(hpa.Name); err != nil {
			t.Errorf("%s: Error reading HPA.autoscaling from the autoscaling/v1 API: %#v", name, err)
		}
		if _, err := legacyClient.Get(hpa.Name); err != nil {
			t.Errorf("%s: Error reading HPA.autoscaling from the extensions/v1beta1 API: %#v", name, err)
		}
	}
}
