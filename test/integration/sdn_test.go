package integration

import (
	"fmt"
	"testing"
	"time"

	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/restclient"
	utilwait "k8s.io/kubernetes/pkg/util/wait"

	osclient "github.com/openshift/origin/pkg/client"
	sdnapi "github.com/openshift/origin/pkg/sdn/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func createProject(osClient *osclient.Client, clientConfig *restclient.Config, name string) (*sdnapi.NetNamespace, error) {
	_, err := testserver.CreateNewProject(osClient, *clientConfig, name, name)
	if err != nil {
		return nil, fmt.Errorf("error creating project %q: %v", name, err)
	}

	backoff := utilwait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   2,
		Steps:    5,
	}
	var netns *sdnapi.NetNamespace
	err = utilwait.ExponentialBackoff(backoff, func() (bool, error) {
		netns, err = osClient.NetNamespaces().Get(name)
		if kapierrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not get NetNamepsace %q: %v", name, err)
	}
	return netns, nil
}

func updateNetNamespace(osClient *osclient.Client, netns *sdnapi.NetNamespace, action sdnapi.PodNetworkAction, args string) (*sdnapi.NetNamespace, error) {
	sdnapi.SetChangePodNetworkAnnotation(netns, action, args)
	_, err := osClient.NetNamespaces().Update(netns)
	if err != nil {
		return nil, err
	}

	backoff := utilwait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   2,
		Steps:    5,
	}
	name := netns.Name
	err = utilwait.ExponentialBackoff(backoff, func() (bool, error) {
		netns, err = osClient.NetNamespaces().Get(name)
		if err != nil {
			return false, err
		}

		if _, _, err := sdnapi.GetChangePodNetworkAnnotation(netns); err == sdnapi.ErrorPodNetworkAnnotationNotFound {
			return true, nil
		} else {
			return false, nil
		}
	})
	if err != nil {
		return nil, err
	}
	return netns, nil
}

func TestOadmPodNetwork(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	masterConfig.NetworkConfig.NetworkPluginName = sdnapi.MultiTenantPluginName
	kubeConfigFile, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	osClient, err := testutil.GetClusterAdminClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %v", err)
	}
	clientConfig, err := testutil.GetClusterAdminClientConfig(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client config: %v", err)
	}

	origNetns1, err := createProject(osClient, clientConfig, "one")
	if err != nil {
		t.Fatalf("could not create namespace %q: %v", "one", err)
	}
	origNetns2, err := createProject(osClient, clientConfig, "two")
	if err != nil {
		t.Fatalf("could not create namespace %q: %v", "two", err)
	}
	origNetns3, err := createProject(osClient, clientConfig, "three")
	if err != nil {
		t.Fatalf("could not create namespace %q: %v", "three", err)
	}

	if origNetns1.NetID == 0 || origNetns2.NetID == 0 || origNetns3.NetID == 0 {
		t.Fatalf("expected non-0 NetIDs, got %d, %d, %d", origNetns1.NetID, origNetns2.NetID, origNetns3.NetID)
	}
	if origNetns1.NetID == origNetns2.NetID || origNetns1.NetID == origNetns3.NetID || origNetns2.NetID == origNetns3.NetID {
		t.Fatalf("expected unique NetIDs, got %d, %d, %d", origNetns1.NetID, origNetns2.NetID, origNetns3.NetID)
	}

	newNetns2, err := updateNetNamespace(osClient, origNetns2, sdnapi.JoinPodNetwork, "one")
	if err != nil {
		t.Fatalf("error updating namespace: %v", err)
	}
	if newNetns2.NetID != origNetns1.NetID {
		t.Fatalf("expected netns2 (%d) to be joined to netns1 (%d)", newNetns2.NetID, origNetns1.NetID)
	}
	newNetns1, err := osClient.NetNamespaces().Get("one")
	if err != nil {
		t.Fatalf("error getting refetching NetNamespace: %v", err)
	}
	if newNetns1.NetID != origNetns1.NetID {
		t.Fatalf("expected netns1 (%d) to be unchanged (%d)", newNetns1.NetID, origNetns1.NetID)
	}

	newNetns1, err = updateNetNamespace(osClient, origNetns1, sdnapi.GlobalPodNetwork, "")
	if err != nil {
		t.Fatalf("error updating namespace: %v", err)
	}
	if newNetns1.NetID != 0 {
		t.Fatalf("expected netns1 (%d) to be global", newNetns1.NetID)
	}
	newNetns2, err = osClient.NetNamespaces().Get("two")
	if err != nil {
		t.Fatalf("error getting refetching NetNamespace: %v", err)
	}
	if newNetns2.NetID != origNetns1.NetID {
		t.Fatalf("expected netns2 (%d) to be unchanged (%d)", newNetns2.NetID, origNetns1.NetID)
	}

	newNetns1, err = updateNetNamespace(osClient, newNetns1, sdnapi.IsolatePodNetwork, "")
	if err != nil {
		t.Fatalf("error updating namespace: %v", err)
	}
	if newNetns1.NetID == 0 {
		t.Fatalf("expected netns1 (%d) to be non-global", newNetns1.NetID)
	}
	if newNetns1.NetID == newNetns2.NetID || newNetns1.NetID == origNetns3.NetID {
		t.Fatalf("expected netns1 (%d) to be unique (not %d, %d)", newNetns1.NetID, newNetns2.NetID, origNetns3.NetID)
	}
}
