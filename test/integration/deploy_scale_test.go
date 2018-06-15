package integration

import (
	"fmt"
	"testing"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	discocache "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/scale"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appstest "github.com/openshift/origin/pkg/apps/apis/apps/test"
	appsclient "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestDeployScale(t *testing.T) {
	const namespace = "test-deploy-scale"

	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatal(err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = testserver.CreateNewProject(clusterAdminClientConfig, namespace, "my-test-user")
	if err != nil {
		t.Fatal(err)
	}
	_, adminConfig, err := testutil.GetClientForUser(clusterAdminClientConfig, "my-test-user")
	if err != nil {
		t.Fatal(err)
	}
	adminAppsClient := appsclient.NewForConfigOrDie(adminConfig)

	config := appstest.OkDeploymentConfig(0)
	config.Namespace = namespace
	config.Spec.Triggers = []appsapi.DeploymentTriggerPolicy{}
	config.Spec.Replicas = 1

	dc, err := adminAppsClient.Apps().DeploymentConfigs(namespace).Create(config)
	if err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v %#v", err, config)
	}
	generation := dc.Generation

	{
		// Get scale subresource
		legacyPath := fmt.Sprintf("/oapi/v1/namespaces/%s/deploymentconfigs/%s/scale", dc.Namespace, dc.Name)
		legacyScale := &unstructured.Unstructured{}
		if err := adminAppsClient.RESTClient().Get().AbsPath(legacyPath).Do().Into(legacyScale); err != nil {
			t.Fatal(err)
		}
		// Ensure correct type
		if legacyScale.GetAPIVersion() != "extensions/v1beta1" {
			t.Fatalf("Expected extensions/v1beta1, got %v", legacyScale.GetAPIVersion())
		}
		scaleBytes, err := legacyScale.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}

		// Ensure we can submit the same type back
		if err := adminAppsClient.RESTClient().Put().AbsPath(legacyPath).Body(scaleBytes).Do().Error(); err != nil {
			t.Fatal(err)
		}
	}

	{
		// Get scale subresource
		scalePath := fmt.Sprintf("/apis/apps.openshift.io/v1/namespaces/%s/deploymentconfigs/%s/scale", dc.Namespace, dc.Name)
		scale := &unstructured.Unstructured{}
		if err := adminAppsClient.RESTClient().Get().AbsPath(scalePath).Do().Into(scale); err != nil {
			t.Fatal(err)
		}
		// Ensure correct type
		if scale.GetAPIVersion() != "extensions/v1beta1" {
			t.Fatalf("Expected extensions/v1beta1, got %v", scale.GetAPIVersion())
		}
		scaleBytes, err := scale.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}

		// Ensure we can submit the same type back
		if err := adminAppsClient.RESTClient().Put().AbsPath(scalePath).Body(scaleBytes).Do().Error(); err != nil {
			t.Fatal(err)
		}
	}

	condition := func() (bool, error) {
		config, err := adminAppsClient.Apps().DeploymentConfigs(namespace).Get(dc.Name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return appsutil.HasSynced(config, generation), nil
	}
	if err := wait.PollImmediate(500*time.Millisecond, 10*time.Second, condition); err != nil {
		t.Fatalf("Deployment config never synced: %v", err)
	}

	cachedDiscovery := discocache.NewMemCacheClient(adminAppsClient.Discovery())
	restMapper := discovery.NewDeferredDiscoveryRESTMapper(cachedDiscovery, apimeta.InterfacesForUnstructured)
	restMapper.Reset()
	// we don't use cached discovery because DiscoveryScaleKindResolver does its own caching,
	// so we want to re-fetch every time when we actually ask for it
	scaleKindResolver := scale.NewDiscoveryScaleKindResolver(adminAppsClient.Discovery())
	scaleClient, err := scale.NewForConfig(adminConfig, restMapper, dynamic.LegacyAPIPathResolverFunc, scaleKindResolver)
	if err != nil {
		t.Fatal(err)
	}

	scale, err := scaleClient.Scales(namespace).Get(appsapi.Resource("deploymentconfigs"), config.Name)
	if err != nil {
		t.Fatalf("Couldn't get DeploymentConfig scale: %v", err)
	}
	if scale.Spec.Replicas != 1 {
		t.Fatalf("Expected scale.spec.replicas=1, got %#v", scale)
	}

	scaleUpdate := scale.DeepCopy()
	scaleUpdate.Spec.Replicas = 3
	updatedScale, err := scaleClient.Scales(namespace).Update(appsapi.Resource("deploymentconfigs"), scaleUpdate)
	if err != nil {
		// If this complains about "Scale" not being registered in "v1", check the kind overrides in the API registration in SubresourceGroupVersionKind
		t.Fatalf("Couldn't update DeploymentConfig scale to %#v: %v", scaleUpdate, err)
	}
	if updatedScale.Spec.Replicas != 3 {
		t.Fatalf("Expected scale.spec.replicas=3, got %#v", scale)
	}

	persistedScale, err := scaleClient.Scales(namespace).Get(appsapi.Resource("deploymentconfigs"), config.Name)
	if err != nil {
		t.Fatalf("Couldn't get DeploymentConfig scale: %v", err)
	}
	if persistedScale.Spec.Replicas != 3 {
		t.Fatalf("Expected scale.spec.replicas=3, got %#v", scale)
	}
}
