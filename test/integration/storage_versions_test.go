package integration

import (
	"path"
	"testing"

	etcd "github.com/coreos/etcd/client"
	"golang.org/x/net/context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	autoscaling_v1 "k8s.io/kubernetes/pkg/apis/autoscaling/v1"
	"k8s.io/kubernetes/pkg/apis/batch"
	batch_v1 "k8s.io/kubernetes/pkg/apis/batch/v1"
	extensions_v1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kautoscalingclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/autoscaling/internalversion"
	kbatchclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/batch/internalversion"
	kutil "k8s.io/kubernetes/pkg/util"

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
func (c legacyExtensionsAutoscaling) List(opts metav1.ListOptions) (result *autoscaling.HorizontalPodAutoscalerList, err error) {
	result = &autoscaling.HorizontalPodAutoscalerList{}
	err = c.client.Get().Namespace(c.namespace).Resource("horizontalPodAutoscalers").VersionedParams(&opts, kapi.ParameterCodec).Do().Into(result)
	return
}

func (c legacyExtensionsAutoscaling) Create(hpa *autoscaling.HorizontalPodAutoscaler) (*autoscaling.HorizontalPodAutoscaler, error) {
	var result autoscaling.HorizontalPodAutoscaler
	return &result, c.client.Post().Resource("horizontalpodautoscalers").Namespace(c.namespace).Body(hpa).Do().Into(&result)
}

func (c legacyExtensionsAutoscaling) Get(name string, options metav1.GetOptions) (*autoscaling.HorizontalPodAutoscaler, error) {
	var result autoscaling.HorizontalPodAutoscaler
	return &result, c.client.Get().Resource("horizontalpodautoscalers").Namespace(c.namespace).Name(name).Do().Into(&result)
}

func getGVKFromEtcd(etcdClient etcd.Client, masterConfig *configapi.MasterConfig, prefix, ns, name string) (*schema.GroupVersionKind, error) {
	keys := etcd.NewKeysAPI(etcdClient)
	key := path.Join(masterConfig.EtcdStorageConfig.KubernetesStoragePrefix, prefix, ns, name)
	resp, err := keys.Get(context.TODO(), key, nil)
	if err != nil {
		return nil, err
	}
	_, gvk, err := unstructured.UnstructuredJSONScheme.Decode([]byte(resp.Node.Value), nil, nil)
	return gvk, err
}

func setupStorageTests(t *testing.T, ns string) (*configapi.MasterConfig, kclientset.Interface) {
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
	_, projectAdminKubeClient, _, err := testutil.GetClientForUser(*clusterAdminClientConfig, "admin")
	if err != nil {
		t.Fatalf("unexpected error getting project admin client: %v", err)
	}

	return masterConfig, projectAdminKubeClient
}

func TestStorageVersions(t *testing.T) {
	ns := "storageversions"
	autoscalingVersion := autoscaling_v1.SchemeGroupVersion
	batchVersion := batch_v1.SchemeGroupVersion

	defer testutil.DumpEtcdOnFailure(t)
	etcdServer := testutil.RequireEtcd(t)
	masterConfig, kubeClient := setupStorageTests(t, ns)

	jobTestcases := map[string]struct {
		creator kbatchclient.JobInterface
	}{
		"batch": {creator: kubeClient.Batch().Jobs(ns)},
	}
	for name, testcase := range jobTestcases {
		job := batch.Job{
			ObjectMeta: metav1.ObjectMeta{Name: name + "-job"},
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
		if _, err := kubeClient.Batch().Jobs(ns).Get(job.Name, metav1.GetOptions{}); err != nil {
			t.Errorf("%s: Error reading Job from the batch client: %#v", name, err)
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
			ObjectMeta: metav1.ObjectMeta{Name: name + "-hpa"},
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
		if _, err := kubeClient.Autoscaling().HorizontalPodAutoscalers(ns).Get(hpa.Name, metav1.GetOptions{}); err != nil {
			t.Errorf("%s: Error reading HPA.autoscaling from the autoscaling/v1 API: %#v", name, err)
		}
		if _, err := legacyClient.Get(hpa.Name, metav1.GetOptions{}); err != nil {
			t.Errorf("%s: Error reading HPA.autoscaling from the extensions/v1beta1 API: %#v", name, err)
		}
	}
}

const extensionsv1beta1HPA = `{"kind":"HorizontalPodAutoscaler","apiVersion":"extensions/v1beta1","metadata":{"name":"extensionsv1beta1hpa","namespace":"storagemigration","selfLink":"/apis/autoscaling/v1/namespaces/storagemigration/horizontalpodautoscalers/extensionsv1beta1hpa","uid":"4b5d9f60-dcd1-11e6-8d37-525400f25e34","creationTimestamp":"2017-01-17T16:23:35Z"},"spec":{"scaleRef":{"kind":"Deployment","name":"web","subresource":"scale"},"minReplicas":1,"maxReplicas":10,"cpuUtilization":{"targetPercentage":70}},"status":{"observedGeneration":1,"lastScaleTime":"2017-01-17T16:23:35Z","currentReplicas":1,"desiredReplicas":5,"currentCPUUtilizationPercentage":30}}`

func TestStorageMigration(t *testing.T) {
	ns := "storagemigration"
	prefix := "horizontalpodautoscalers"
	hpaName := "extensionsv1beta1hpa"
	autoscalingVersion := autoscaling_v1.SchemeGroupVersion

	defer testutil.DumpEtcdOnFailure(t)
	etcdServer := testutil.RequireEtcd(t)
	masterConfig, kubeClient := setupStorageTests(t, ns)

	// Save an extensions/v1beta1.HorizontalPodAutoscaler directly in etcd
	keys := etcd.NewKeysAPI(etcdServer.Client)
	key := path.Join(masterConfig.EtcdStorageConfig.KubernetesStoragePrefix, prefix, ns, hpaName)
	if _, err := keys.Create(context.TODO(), key, extensionsv1beta1HPA); err != nil {
		t.Fatalf("Unexpected error saving extensions/v1beta1.HorizontalPodAutoscaler: %v", err)
	}

	// Ensure it is accessible from both APIs
	autoscalingHPA, err := kubeClient.Autoscaling().HorizontalPodAutoscalers(ns).Get(hpaName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Error reading HPA from the autoscaling client: %#v", err)
	}

	extensionsHPA := &extensions_v1beta1.HorizontalPodAutoscaler{}
	err = kubeClient.Extensions().RESTClient().Get().
		Namespace(ns).
		Resource("horizontalpodautoscalers").
		Name(hpaName).
		VersionedParams(&metav1.GetOptions{}, kapi.ParameterCodec).
		Do().
		Into(extensionsHPA)
	if err != nil {
		t.Errorf("Error reading HPA from the extensions client: %#v", err)
	}

	// Ensure that both versions of the same object are equal when converted
	convertedExtensionsHPA := &autoscaling.HorizontalPodAutoscaler{}
	if err := kapi.Scheme.Convert(extensionsHPA, convertedExtensionsHPA, nil); err != nil {
		t.Fatalf("Conversion error from extensions.HPA to autoscaling.HPA: %v", err)
	}
	if !kapi.Semantic.DeepEqual(autoscalingHPA.Spec, convertedExtensionsHPA.Spec) {
		t.Errorf("Extensions HPA and autoscaling HPA representation differ: %v", diff.ObjectDiff(convertedExtensionsHPA.Spec, autoscalingHPA.Spec))
	}

	// Update the HPA
	autoscalingHPA.Spec.MinReplicas = kutil.Int32Ptr(2)
	if _, err := kubeClient.Autoscaling().HorizontalPodAutoscalers(ns).Update(autoscalingHPA); err != nil {
		t.Errorf("Error updating HPA: %#v", err)
	}

	// Ensure it is persisted as autoscaling/v1.HorizontalPodAutoscaler
	if gvk, err := getGVKFromEtcd(etcdServer.Client, masterConfig, prefix, ns, hpaName); err != nil {
		t.Fatalf("Unexpected error reading HPA from etcd: %v", err)
	} else if *gvk != autoscalingVersion.WithKind("HorizontalPodAutoscaler") {
		t.Fatalf("Expected api version %s in etcd, got %s reading HPA", autoscalingVersion, gvk)
	}
}
