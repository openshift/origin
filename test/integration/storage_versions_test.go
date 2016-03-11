// +build integration

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
	"k8s.io/kubernetes/pkg/apis/extensions"
	extensions_v1beta1 "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
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
	runStorageTest(t, "unified",
		extensions_v1beta1.SchemeGroupVersion,
		extensions_v1beta1.SchemeGroupVersion,
		extensions_v1beta1.SchemeGroupVersion,
	)
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
		"batch":      {creator: projectAdminKubeClient.Batch().Jobs(ns)},
		"extensions": {creator: projectAdminKubeClient.Extensions().Jobs(ns)},
	}
	for name, testcase := range jobTestcases {
		job := extensions.Job{
			ObjectMeta: kapi.ObjectMeta{Name: name + "-job"},
			Spec: extensions.JobSpec{
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

	hpaTestcases := map[string]struct {
		creator kclient.HorizontalPodAutoscalerInterface
	}{
		"autoscaling": {creator: projectAdminKubeClient.Autoscaling().HorizontalPodAutoscalers(ns)},
		"extensions":  {creator: projectAdminKubeClient.Extensions().HorizontalPodAutoscalers(ns)},
	}
	for name, testcase := range hpaTestcases {
		hpa := extensions.HorizontalPodAutoscaler{
			ObjectMeta: kapi.ObjectMeta{Name: name + "-hpa"},
			Spec: extensions.HorizontalPodAutoscalerSpec{
				MaxReplicas: 1,
				ScaleRef:    extensions.SubresourceReference{Kind: "ReplicationController", Name: "myrc", Subresource: "scale"},
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

		// Make sure it is available from both APIs
		if _, err := projectAdminKubeClient.Autoscaling().HorizontalPodAutoscalers(ns).Get(hpa.Name); err != nil {
			t.Errorf("%s: Error reading HPA.autoscaling from the autoscaling/v1 API: %#v", name, err)
		}
		if _, err := projectAdminKubeClient.Extensions().HorizontalPodAutoscalers(ns).Get(hpa.Name); err != nil {
			t.Errorf("%s: Error reading HPA.extensions from the extensions/v1beta1 API: %#v", name, err)
		}
	}
}
