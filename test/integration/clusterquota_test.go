package integration

import (
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	imagev1 "github.com/openshift/api/image/v1"
	quotav1 "github.com/openshift/api/quota/v1"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned"
	quotaclient "github.com/openshift/client-go/quota/clientset/versioned"
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
	clusterAdminQuotaClient := quotaclient.NewForConfigOrDie(testutil.NonProtobufConfig(clusterAdminClientConfig))
	clusterAdminImageClient := imagev1client.NewForConfigOrDie(clusterAdminClientConfig).ImageV1()

	if err := testutil.WaitForClusterResourceQuotaCRDAvailable(clusterAdminClientConfig); err != nil {
		t.Fatal(err)
	}

	cq := &quotav1.ClusterResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "overall"},
		Spec: quotav1.ClusterResourceQuotaSpec{
			Selector: quotav1.ClusterResourceQuotaSelector{
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}},
			},
			Quota: corev1.ResourceQuotaSpec{
				Hard: corev1.ResourceList{
					corev1.ResourceConfigMaps:   resource.MustParse("2"),
					"openshift.io/imagestreams": resource.MustParse("1"),
				},
			},
		},
	}
	if _, err := clusterAdminQuotaClient.QuotaV1().ClusterResourceQuotas().Create(cq); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, _, err := testserver.CreateNewProject(clusterAdminClientConfig, "first", "harold"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, err := testserver.CreateNewProject(clusterAdminClientConfig, "second", "harold"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := labelNamespace(clusterAdminKubeClient.CoreV1(), "first"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := labelNamespace(clusterAdminKubeClient.CoreV1(), "second"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := waitForQuotaLabeling(clusterAdminQuotaClient, "first"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := waitForQuotaLabeling(clusterAdminQuotaClient, "second"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := waitForQuotaStatus(clusterAdminQuotaClient, cq.Name, func(quota *quotav1.ClusterResourceQuota) error {
		if !equality.Semantic.DeepEqual(quota.Spec.Quota.Hard, quota.Status.Total.Hard) {
			return fmt.Errorf("%#v != %#v", quota.Spec.Quota.Hard, quota.Status.Total.Hard)
		}
		return nil
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	configmap := &corev1.ConfigMap{}
	configmap.GenerateName = "test"
	if _, err := clusterAdminKubeClient.CoreV1().ConfigMaps("first").Create(configmap); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := waitForQuotaStatus(clusterAdminQuotaClient, cq.Name, func(quota *quotav1.ClusterResourceQuota) error {
		q := quota.Status.Total.Used[corev1.ResourceConfigMaps]
		if i, ok := q.AsInt64(); ok {
			if i == 1 {
				return nil
			}
			return fmt.Errorf("%d != 1", i)
		}
		return fmt.Errorf("quota=%+v AsInt64() failed", q)
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := clusterAdminKubeClient.CoreV1().ConfigMaps("second").Create(configmap); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := waitForQuotaStatus(clusterAdminQuotaClient, cq.Name, func(quota *quotav1.ClusterResourceQuota) error {
		q := quota.Status.Total.Used[corev1.ResourceConfigMaps]
		if i, ok := q.AsInt64(); ok {
			if i == 2 {
				return nil
			}
			return fmt.Errorf("%d != 1", i)
		}
		return fmt.Errorf("quota=%+v AsInt64() failed", q)
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := clusterAdminKubeClient.CoreV1().ConfigMaps("second").Create(configmap); !apierrors.IsForbidden(err) {
		list, err := clusterAdminQuotaClient.QuotaV1().AppliedClusterResourceQuotas("second").List(metav1.ListOptions{})
		if err == nil {
			t.Errorf("quota is %#v", list)
		}

		list2, err := clusterAdminKubeClient.CoreV1().ConfigMaps("").List(metav1.ListOptions{})
		if err == nil {
			t.Errorf("ConfigMaps is %#v", list2)
		}

		t.Fatalf("unexpected error: %v", err)
	}

	imagestream := &imagev1.ImageStream{}
	imagestream.GenerateName = "test"
	if _, err := clusterAdminImageClient.ImageStreams("first").Create(imagestream); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := waitForQuotaStatus(clusterAdminQuotaClient, cq.Name, func(quota *quotav1.ClusterResourceQuota) error {
		q := quota.Status.Total.Used["openshift.io/imagestreams"]
		if i, ok := q.AsInt64(); ok {
			if i == 1 {
				return nil
			}
			return fmt.Errorf("%d != 1", i)
		}
		return fmt.Errorf("quota=%+v AsInt64() failed", q)
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := clusterAdminImageClient.ImageStreams("second").Create(imagestream); !apierrors.IsForbidden(err) {
		list, err := clusterAdminQuotaClient.QuotaV1().AppliedClusterResourceQuotas("second").List(metav1.ListOptions{})
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
		list, err := clusterAdminClient.QuotaV1().AppliedClusterResourceQuotas(namespaceName).List(metav1.ListOptions{})
		if err != nil {
			return false, nil
		}
		if len(list.Items) > 0 && len(list.Items[0].Status.Total.Hard) > 0 {
			return true, nil
		}
		return false, nil
	})
}

func labelNamespace(clusterAdminKubeClient corev1client.NamespacesGetter, namespaceName string) error {
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

func waitForQuotaStatus(clusterAdminClient quotaclient.Interface, name string, conditionFn func(*quotav1.ClusterResourceQuota) error) error {
	var pollErr error
	err := utilwait.PollImmediate(100*time.Millisecond, 30*time.Second, func() (done bool, err error) {
		quota, err := clusterAdminClient.QuotaV1().ClusterResourceQuotas().Get(name, metav1.GetOptions{})
		if err != nil {
			pollErr = err
			return false, nil
		}
		err = conditionFn(quota)
		if err == nil {
			return true, nil
		}
		pollErr = err
		return false, nil
	})
	if err == nil {
		// since now we run each process separately we need to wait for the informers
		// to catch up on the update and only then continue
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		err = fmt.Errorf("%s: %s", err, pollErr)
	}
	return err
}
