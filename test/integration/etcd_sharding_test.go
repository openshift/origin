// +build integration,etcd

package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/coreos/etcd/client"

	"golang.org/x/net/context"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/runtime"
	etcdtesting "k8s.io/kubernetes/pkg/storage/etcd/testing"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/openshift/origin/pkg/cmd/server/api"
	originetcd "github.com/openshift/origin/pkg/cmd/server/etcd"
	projectapi "github.com/openshift/origin/pkg/project/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestEtcdSharding(t *testing.T) {
	// Start a separate etcd server
	kubeEtcdServer := etcdtesting.NewEtcdTestClientServer(t)
	defer kubeEtcdServer.Terminate(t)
	kubeEtcdClient := kubeEtcdServer.Client

	// Generate openshift configuration
	openShiftMasterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Configure openshift to write kubernetes data to a separate etcd
	openShiftMasterConfig.KubernetesMasterConfig.EtcdClientInfo = &api.EtcdConnectionInfo{
		URLs: kubeEtcdServer.ClientURLs.StringSlice(),
	}

	// Start the server
	openShiftOptions := testserver.TestOptions{DeleteAllEtcdKeys: true, EnableControllers: true}
	adminConfigFile, err := testserver.StartConfiguredMasterWithOptions(openShiftMasterConfig, openShiftOptions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	adminConfig, err := testutil.GetClusterAdminClientConfig(adminConfigFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create an openshift client
	osClient, err := testutil.GetClusterAdminClient(adminConfigFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create an etcd client to talk to the openshift etcd
	osEtcdClient, err := originetcd.MakeNewEtcdClient(openShiftMasterConfig.EtcdClientInfo)
	if err != nil {
		t.Fatalf("failed to connect to openshift etcd: %v", err)
	}

	kubeClient, err := testutil.GetClusterAdminKubeClient(adminConfigFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create a project
	projName := "shard-project"
	project := &projectapi.Project{
		ObjectMeta: kapi.ObjectMeta{
			Name: projName,
		},
	}
	projectResult, err := osClient.Projects().Create(project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if projectResult.Status.Phase != kapi.NamespaceActive {
		t.Fatalf("project status is: %s", projectResult.Status.Phase)
	}

	// Login as a user
	_, _, _, err = testutil.GetClientForUser(*adminConfig, "shard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, _, _, err = testutil.GetClientForServiceAccount(kubeClient, *adminConfig, "default", "sa")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Create a template
	template := &templateapi.Template{
		Parameters: []templateapi.Parameter{
			{
				Name:  "NAME",
				Value: "shard-template",
			},
		},
	}

	templateObjects := []runtime.Object{
		&v1.Service{
			ObjectMeta: v1.ObjectMeta{
				Name:      "shard-tester",
				Namespace: "default",
			},
			Spec: v1.ServiceSpec{
				ClusterIP:       "1.2.3.4",
				SessionAffinity: "sharding",
			},
		},
	}
	templateapi.AddObjectsToTemplate(template, templateObjects, v1.SchemeGroupVersion)

	obj, err := osClient.TemplateConfigs("default").Create(template)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obj.Objects) != 1 {
		t.Fatalf("unexpected object: %#v", obj)
	}

	// Verify data is placed in the correct etcd instances
	kubeStoragePrefix := openShiftMasterConfig.EtcdStorageConfig.KubernetesStoragePrefix
	osStoragePrefix := openShiftMasterConfig.EtcdStorageConfig.OpenShiftStoragePrefix
	kubeKeys := client.NewKeysAPI(kubeEtcdClient)
	osKeys := client.NewKeysAPI(osEtcdClient)
	_, err = kubeKeys.Get(context.TODO(), fmt.Sprintf("/%s", kubeStoragePrefix), nil)
	if err != nil {
		t.Fatalf("failed to find kuberenetes prefix '%s' in kubernetes etcd instance: %v", kubeStoragePrefix, err)
	}
	_, err = kubeKeys.Get(context.TODO(), fmt.Sprintf("/%s", osStoragePrefix), nil)
	if err == nil {
		t.Fatalf("found openshift prefix '%s' in kubernetes etcd instance: %v", osStoragePrefix, err)
	}
	_, err = osKeys.Get(context.TODO(), fmt.Sprintf("/%s", osStoragePrefix), nil)
	if err != nil {
		t.Fatalf("failed to find openshift prefix '%s' in openshift etcd instance: %v", osStoragePrefix, err)
	}
	_, err = osKeys.Get(context.TODO(), fmt.Sprintf("/%s", kubeStoragePrefix), nil)
	if err == nil {
		t.Fatalf("found kuberenetes prefix '%s' in openshift etcd instance: %v", kubeStoragePrefix, err)
	}

	// Delete the project
	if err = osClient.Projects().Delete(projName); err != nil {
		t.Fatalf("expected error deleting project: %v", err)
	}
	err = wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
		_, err := kubeClient.Namespaces().Get(projName)
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatalf("unexpected error while waiting for project to delete: %v", err)
	}

	// Verify the project doesn't exist
	if err = osClient.Projects().Delete(projName); err == nil {
		t.Fatalf("project still exists after delete")
	}
}
