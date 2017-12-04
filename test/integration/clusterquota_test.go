package integration

import (
	"testing"
	"time"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	quotaapi "github.com/openshift/origin/pkg/quota/apis/quota"
	quotaclient "github.com/openshift/origin/pkg/quota/generated/internalclientset"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestClusterQuota(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminKubeClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminQuotaClient := quotaclient.NewForConfigOrDie(clusterAdminClientConfig)
	clusterAdminImageClient := imageclient.NewForConfigOrDie(clusterAdminClientConfig).Image()

	cq := &quotaapi.ClusterResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "overall"},
		Spec: quotaapi.ClusterResourceQuotaSpec{
			Selector: quotaapi.ClusterResourceQuotaSelector{
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
			},
			Quota: kapi.ResourceQuotaSpec{
				Hard: kapi.ResourceList{
					kapi.ResourceConfigMaps:     resource.MustParse("2"),
					"openshift.io/imagestreams": resource.MustParse("1"),
				},
			},
		},
	}
	if _, err := clusterAdminQuotaClient.Quota().ClusterResourceQuotas().Create(cq); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, _, err := testserver.CreateNewProject(clusterAdminClientConfig, "first", "harold"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, err := testserver.CreateNewProject(clusterAdminClientConfig, "second", "harold"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := labelNamespace(clusterAdminKubeClient.Core(), "first"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := labelNamespace(clusterAdminKubeClient.Core(), "second"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := waitForQuotaLabeling(clusterAdminQuotaClient, "first"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := waitForQuotaLabeling(clusterAdminQuotaClient, "second"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configmap := &kapi.ConfigMap{}
	configmap.GenerateName = "test"
	if _, err := clusterAdminKubeClient.Core().ConfigMaps("first").Create(configmap); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := clusterAdminKubeClient.Core().ConfigMaps("second").Create(configmap); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := clusterAdminKubeClient.Core().ConfigMaps("second").Create(configmap); !kapierrors.IsForbidden(err) {
		list, err := clusterAdminQuotaClient.Quota().AppliedClusterResourceQuotas("second").List(metav1.ListOptions{})
		if err == nil {
			t.Errorf("quota is %#v", list)
		}

		list2, err := clusterAdminKubeClient.Core().ConfigMaps("").List(metav1.ListOptions{})
		if err == nil {
			t.Errorf("ConfigMaps is %#v", list2)
		}

		t.Fatalf("unexpected error: %v", err)
	}

	imagestream := &imageapi.ImageStream{}
	imagestream.GenerateName = "test"
	if _, err := clusterAdminImageClient.ImageStreams("first").Create(imagestream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := clusterAdminImageClient.ImageStreams("second").Create(imagestream); !kapierrors.IsForbidden(err) {
		list, err := clusterAdminQuotaClient.Quota().AppliedClusterResourceQuotas("second").List(metav1.ListOptions{})
		if err == nil {
			t.Errorf("quota is %#v", list)
		}

		list2, err := clusterAdminImageClient.ImageStreams("").List(metav1.ListOptions{})
		if err == nil {
			t.Errorf("ImageStreams is %#v", list2)
		}

		t.Fatalf("unexpected error: %v", err)
	}

}

func waitForQuotaLabeling(clusterAdminClient quotaclient.Interface, namespaceName string) error {
	return utilwait.PollImmediate(100*time.Millisecond, 10*time.Second, func() (done bool, err error) {
		list, err := clusterAdminClient.Quota().AppliedClusterResourceQuotas(namespaceName).List(metav1.ListOptions{})
		if err != nil {
			return false, nil
		}
		if len(list.Items) > 0 && len(list.Items[0].Status.Total.Hard) > 0 {
			return true, nil
		}
		return false, nil
	})
}

func labelNamespace(clusterAdminKubeClient kcoreclient.NamespacesGetter, namespaceName string) error {
	ns1, err := clusterAdminKubeClient.Namespaces().Get(namespaceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if ns1.Labels == nil {
		ns1.Labels = map[string]string{}
	}
	ns1.Labels["foo"] = "bar"
	if _, err := clusterAdminKubeClient.Namespaces().Update(ns1); err != nil {
		return err
	}
	return nil
}
