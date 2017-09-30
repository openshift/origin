package integration

import (
	"fmt"
	"testing"
	"time"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	restclient "k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/network"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	networkclient "github.com/openshift/origin/pkg/network/generated/internalclientset/typed/network/internalversion"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func createProject(clientConfig *restclient.Config, name string) (*networkapi.NetNamespace, error) {
	_, _, err := testserver.CreateNewProject(clientConfig, name, name)
	if err != nil {
		return nil, fmt.Errorf("error creating project %q: %v", name, err)
	}

	var netns *networkapi.NetNamespace
	err = utilwait.Poll(time.Second/2, 30*time.Second, func() (bool, error) {
		netns, err = networkclient.NewForConfigOrDie(clientConfig).NetNamespaces().Get(name, metav1.GetOptions{})
		if kapierrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not get NetNamespace %q: %v", name, err)
	}
	return netns, nil
}

func updateNetNamespace(osClient networkclient.NetworkInterface, netns *networkapi.NetNamespace, action network.PodNetworkAction, args string) (*networkapi.NetNamespace, error) {
	network.SetChangePodNetworkAnnotation(netns, action, args)
	_, err := osClient.NetNamespaces().Update(netns)
	if err != nil {
		return nil, err
	}

	name := netns.Name
	err = utilwait.Poll(time.Second/2, 30*time.Second, func() (bool, error) {
		netns, err = osClient.NetNamespaces().Get(name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if _, _, err := network.GetChangePodNetworkAnnotation(netns); err == network.ErrorPodNetworkAnnotationNotFound {
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
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error creating config: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)
	masterConfig.NetworkConfig.NetworkPluginName = network.MultiTenantPluginName
	kubeConfigFile, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %v", err)
	}
	clientConfig, err := testutil.GetClusterAdminClientConfig(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client config: %v", err)
	}
	clusterAdminNetworkClient := networkclient.NewForConfigOrDie(clientConfig)

	origNetns1, err := createProject(clientConfig, "one")
	if err != nil {
		t.Fatalf("could not create namespace %q: %v", "one", err)
	}
	origNetns2, err := createProject(clientConfig, "two")
	if err != nil {
		t.Fatalf("could not create namespace %q: %v", "two", err)
	}
	origNetns3, err := createProject(clientConfig, "three")
	if err != nil {
		t.Fatalf("could not create namespace %q: %v", "three", err)
	}

	if origNetns1.NetID == 0 || origNetns2.NetID == 0 || origNetns3.NetID == 0 {
		t.Fatalf("expected non-0 NetIDs, got %d, %d, %d", origNetns1.NetID, origNetns2.NetID, origNetns3.NetID)
	}
	if origNetns1.NetID == origNetns2.NetID || origNetns1.NetID == origNetns3.NetID || origNetns2.NetID == origNetns3.NetID {
		t.Fatalf("expected unique NetIDs, got %d, %d, %d", origNetns1.NetID, origNetns2.NetID, origNetns3.NetID)
	}

	newNetns2, err := updateNetNamespace(clusterAdminNetworkClient, origNetns2, network.JoinPodNetwork, "one")
	if err != nil {
		t.Fatalf("error updating namespace: %v", err)
	}
	if newNetns2.NetID != origNetns1.NetID {
		t.Fatalf("expected netns2 (%d) to be joined to netns1 (%d)", newNetns2.NetID, origNetns1.NetID)
	}
	newNetns1, err := clusterAdminNetworkClient.NetNamespaces().Get("one", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting refetching NetNamespace: %v", err)
	}
	if newNetns1.NetID != origNetns1.NetID {
		t.Fatalf("expected netns1 (%d) to be unchanged (%d)", newNetns1.NetID, origNetns1.NetID)
	}

	newNetns1, err = updateNetNamespace(clusterAdminNetworkClient, origNetns1, network.GlobalPodNetwork, "")
	if err != nil {
		t.Fatalf("error updating namespace: %v", err)
	}
	if newNetns1.NetID != 0 {
		t.Fatalf("expected netns1 (%d) to be global", newNetns1.NetID)
	}
	newNetns2, err = clusterAdminNetworkClient.NetNamespaces().Get("two", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error getting refetching NetNamespace: %v", err)
	}
	if newNetns2.NetID != origNetns1.NetID {
		t.Fatalf("expected netns2 (%d) to be unchanged (%d)", newNetns2.NetID, origNetns1.NetID)
	}

	newNetns1, err = updateNetNamespace(clusterAdminNetworkClient, newNetns1, network.IsolatePodNetwork, "")
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
