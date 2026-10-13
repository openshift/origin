package quota

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/test/e2e/framework"

	imagev1 "github.com/openshift/api/image/v1"
	quotav1 "github.com/openshift/api/quota/v1"
	templatev1 "github.com/openshift/api/template/v1"
	quotaclient "github.com/openshift/client-go/quota/clientset/versioned"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// QuotaWaitTimeout should be greater than the monitor's sync timeout value in cluster quota
// reconciliation controller (cluster-policy-controller), because it can hang for 30 seconds when a rare deadlock occurs.
// Upstream sets it to 1 minute so we set the same.
const QuotaWaitTimeout = time.Minute

var _ = g.Describe("[sig-api-machinery][Feature:ClusterResourceQuota]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("crq")

	g.Describe("Cluster resource quota", func() {
		g.It(fmt.Sprintf("should control resource limits across namespaces [apigroup:quota.openshift.io][apigroup:image.openshift.io][apigroup:monitoring.coreos.com][apigroup:template.openshift.io]"), func() {
			t := g.GinkgoT(1)

			clusterAdminKubeClient := oc.AdminKubeClient()
			clusterAdminQuotaClient := oc.AdminQuotaClient()
			clusterAdminImageClient := oc.AdminImageClient()
			clusterAdminTemplateClient := oc.AdminTemplateClient()
			clusterAdminDynamicClient := oc.AdminDynamicClient()

			labelSelectorKey := "foo-" + oc.Namespace()
			cqName := "overall-" + oc.Namespace()
			cq := &quotav1.ClusterResourceQuota{
				ObjectMeta: metav1.ObjectMeta{Name: cqName},
				Spec: quotav1.ClusterResourceQuotaSpec{
					Selector: quotav1.ClusterResourceQuotaSelector{
						LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelSelectorKey: "bar"}},
					},
					Quota: corev1.ResourceQuotaSpec{
						Hard: corev1.ResourceList{
							corev1.ResourceConfigMaps:                     resource.MustParse("2"),
							"openshift.io/imagestreams":                   resource.MustParse("1"),
							"count/templates.template.openshift.io":       resource.MustParse("1"),
							"count/servicemonitors.monitoring.coreos.com": resource.MustParse("1"),
						},
					},
				},
			}

			const kubeRootCAName = "kube-root-ca.crt"
			framework.Logf("expecting ConfigMap %q to be present", kubeRootCAName)

			const serviceCAName = "openshift-service-ca.crt"
			framework.Logf("expecting ConfigMap %q to be present", serviceCAName)

			// Each namespace is expected to have a configmap each for kube root ca and service ca
			namespaceInitialCMCount := 2

			// Ensure quota includes the 2 mandatory configmaps
			// TODO(marun) Figure out why the added quantity isn't 2
			mandatoryCMQuantity := resource.NewQuantity(int64(namespaceInitialCMCount)*2, resource.DecimalSI)
			q := cq.Spec.Quota.Hard[corev1.ResourceConfigMaps]
			q.Add(*mandatoryCMQuantity)
			cq.Spec.Quota.Hard[corev1.ResourceConfigMaps] = q

			if _, err := clusterAdminQuotaClient.QuotaV1().ClusterResourceQuotas().Create(context.Background(), cq, metav1.CreateOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			oc.AddResourceToDelete(quotav1.GroupVersion.WithResource("clusterresourcequotas"), cq)

			firstProjectName := oc.SetupProject()
			secondProjectName := oc.SetupProject()

			// Wait for the creation of the mandatory configmaps before performing checks of quota
			// enforcement to ensure reliable test execution.
			for _, ns := range []string{firstProjectName, secondProjectName} {
				for _, cm := range []string{kubeRootCAName, serviceCAName} {
					_, err := exutil.WaitForCMState(context.Background(), clusterAdminKubeClient.CoreV1(), ns, cm, func(cm *corev1.ConfigMap) (bool, error) {
						// Any event means the CM is present
						framework.Logf("configmap %q is present in namespace %q", cm, ns)
						return true, nil
					})
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
				}
			}

			if err := labelNamespace(clusterAdminKubeClient.CoreV1(), labelSelectorKey, firstProjectName); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := labelNamespace(clusterAdminKubeClient.CoreV1(), labelSelectorKey, secondProjectName); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := waitForQuotaLabeling(clusterAdminQuotaClient, firstProjectName, cqName); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := waitForQuotaLabeling(clusterAdminQuotaClient, secondProjectName, cqName); err != nil {
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
			if _, err := clusterAdminKubeClient.CoreV1().ConfigMaps(firstProjectName).Create(context.Background(), configmap, metav1.CreateOptions{}); err != nil {
				// Istio sometimes create an additional configmap in each namespace, so account for it
				// and retry.
				//
				// Note that the tests that install Istio run in parallel with this test, so we cannot
				// assume that it has or has not created the configmap when we create the quota.  Thus
				// we must check here whether the reason we got an error was because this configmap
				// put us over the quota.
				//
				// TODO: Remove the following const and if/else block when we bump to OSSM 3.0.1, which
				// ships a version of Istio that has been patched not to create these configmaps.
				outerErr := err
				const istioConfigmapName = "istio-ca-root-cert"
				if _, err := clusterAdminKubeClient.CoreV1().ConfigMaps(firstProjectName).Get(context.Background(), istioConfigmapName, metav1.GetOptions{}); err != nil {
					if !apierrors.IsNotFound(err) {
						t.Fatalf("unexpected error: %v", err)
					}
					// The Istio configmap doesn't exist, and therefore The Create must have failed
					// for some other reason; fail on the outer err.
					t.Fatalf("unexpected error: %v", outerErr)
				}

				// As the Istio configmap exists in this project, assume that it exists in all
				// projects, and adjust the clusterquota accordingly.
				namespaceInitialCMCount++
				adjustedMandatoryCMQuantity := resource.NewQuantity(int64(namespaceInitialCMCount)*2, resource.DecimalSI)
				if quota, err := clusterAdminQuotaClient.QuotaV1().ClusterResourceQuotas().Get(context.Background(), cq.Name, metav1.GetOptions{}); err != nil {
					t.Fatalf("unexpected error: %v", err)
				} else {
					cq = quota
				}
				q := cq.Spec.Quota.Hard[corev1.ResourceConfigMaps]
				q.Sub(*mandatoryCMQuantity)
				q.Add(*adjustedMandatoryCMQuantity)
				cq.Spec.Quota.Hard[corev1.ResourceConfigMaps] = q
				if _, err := clusterAdminQuotaClient.QuotaV1().ClusterResourceQuotas().Update(context.Background(), cq, metav1.UpdateOptions{}); err != nil {
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

				// Retry creating the configmap with the quota adjusted for the Istio configmap.
				if _, err := clusterAdminKubeClient.CoreV1().ConfigMaps(firstProjectName).Create(context.Background(), configmap, metav1.CreateOptions{}); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			if err := waitForQuotaStatus(clusterAdminQuotaClient, cq.Name, func(quota *quotav1.ClusterResourceQuota) error {
				expectedCount := int64(2*namespaceInitialCMCount + 1)
				q := quota.Status.Total.Used[corev1.ResourceConfigMaps]
				if i, ok := q.AsInt64(); ok {
					if i == expectedCount {
						return nil
					}
					return fmt.Errorf("%d != %d", i, expectedCount)
				}
				return fmt.Errorf("quota=%+v AsInt64() failed", q)
			}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, err := clusterAdminKubeClient.CoreV1().ConfigMaps(secondProjectName).Create(context.Background(), configmap, metav1.CreateOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := waitForQuotaStatus(clusterAdminQuotaClient, cq.Name, func(quota *quotav1.ClusterResourceQuota) error {
				expectedCount := int64(2*namespaceInitialCMCount + 2)
				q := quota.Status.Total.Used[corev1.ResourceConfigMaps]
				if i, ok := q.AsInt64(); ok {
					if i == expectedCount {
						return nil
					}
					return fmt.Errorf("%d != %d", i, expectedCount)
				}
				return fmt.Errorf("quota=%+v AsInt64() failed", q)
			}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, err := clusterAdminKubeClient.CoreV1().ConfigMaps(secondProjectName).Create(context.Background(), configmap, metav1.CreateOptions{}); !apierrors.IsForbidden(err) {
				framework.Logf("unexpected err during creation: %v", err)
				list, err := clusterAdminQuotaClient.QuotaV1().AppliedClusterResourceQuotas(secondProjectName).List(context.Background(), metav1.ListOptions{})
				if err == nil {
					t.Errorf("quota is %#v", list)
				}

				list2, err := clusterAdminKubeClient.CoreV1().ConfigMaps("").List(context.Background(), metav1.ListOptions{})
				if err == nil {
					t.Errorf("ConfigMaps is %#v", list2)
				}

				t.Fatalf("unexpected error: %v", err)
			}

			imagestream := &imagev1.ImageStream{}
			imagestream.GenerateName = "test"
			if _, err := clusterAdminImageClient.ImageV1().ImageStreams(firstProjectName).Create(context.Background(), imagestream, metav1.CreateOptions{}); err != nil {
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

			if _, err := clusterAdminImageClient.ImageV1().ImageStreams(secondProjectName).Create(context.Background(), imagestream, metav1.CreateOptions{}); !apierrors.IsForbidden(err) {
				framework.Logf("unexpected err during creation: %v", err)
				list, err := clusterAdminQuotaClient.QuotaV1().AppliedClusterResourceQuotas(secondProjectName).List(context.Background(), metav1.ListOptions{})
				if err == nil {
					t.Errorf("quota is %#v", list)
				}

				list2, err := clusterAdminImageClient.ImageV1().ImageStreams("").List(context.Background(), metav1.ListOptions{})
				if err == nil {
					t.Errorf("ImageStreams is %#v", list2)
				}

				t.Fatalf("unexpected error: %v", err)
			}

			// test templates are counted correctly
			template := &templatev1.Template{}
			template.GenerateName = "test"
			if _, err := clusterAdminTemplateClient.TemplateV1().Templates(firstProjectName).Create(context.Background(), template, metav1.CreateOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := waitForQuotaStatus(clusterAdminQuotaClient, cq.Name, func(quota *quotav1.ClusterResourceQuota) error {
				q := quota.Status.Total.Used["count/templates.template.openshift.io"]
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

			if _, err := clusterAdminTemplateClient.TemplateV1().Templates(secondProjectName).Create(context.Background(), template, metav1.CreateOptions{}); !apierrors.IsForbidden(err) {
				framework.Logf("unexpected err during creation: %v", err)
				list, err := clusterAdminQuotaClient.QuotaV1().AppliedClusterResourceQuotas(secondProjectName).List(context.Background(), metav1.ListOptions{})
				if err == nil {
					t.Errorf("quota is %#v", list)
				}

				list2, err := clusterAdminTemplateClient.TemplateV1().Templates("").List(context.Background(), metav1.ListOptions{})
				if err == nil {
					t.Errorf("Templates is %#v", list2)
				}

				t.Fatalf("unexpected error: %v", err)
			}

			// test that CRD resources are counted correctly
			serviceMonitorGVR := schema.GroupVersionResource{Group: "monitoring.coreos.com", Version: "v1", Resource: "servicemonitors"}
			firstServiceMonitor := getTestServiceMonitor("test", firstProjectName)
			secondServiceMonitor := getTestServiceMonitor("test", secondProjectName)
			if _, err := clusterAdminDynamicClient.Resource(serviceMonitorGVR).Namespace(firstProjectName).Create(context.Background(), firstServiceMonitor, metav1.CreateOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := waitForQuotaStatus(clusterAdminQuotaClient, cq.Name, func(quota *quotav1.ClusterResourceQuota) error {
				q := quota.Status.Total.Used["count/servicemonitors.monitoring.coreos.com"]
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

			if _, err := clusterAdminDynamicClient.Resource(serviceMonitorGVR).Namespace(secondProjectName).Create(context.Background(), secondServiceMonitor, metav1.CreateOptions{}); !apierrors.IsForbidden(err) {
				framework.Logf("unexpected err during creation: %v", err)
				list, err := clusterAdminQuotaClient.QuotaV1().AppliedClusterResourceQuotas(secondProjectName).List(context.Background(), metav1.ListOptions{})
				if err == nil {
					t.Errorf("quota is %#v", list)
				}

				list2, err := clusterAdminDynamicClient.Resource(serviceMonitorGVR).Namespace("").List(context.Background(), metav1.ListOptions{})
				if err == nil {
					t.Errorf("ServiceMonitors is %#v", list2)
				}

				t.Fatalf("unexpected error: %v", err)
			}
		})
	})
})

func waitForQuotaLabeling(clusterAdminClient quotaclient.Interface, namespaceName, cqName string) error {
	return utilwait.PollImmediate(100*time.Millisecond, QuotaWaitTimeout, func() (done bool, err error) {
		list, err := clusterAdminClient.QuotaV1().AppliedClusterResourceQuotas(namespaceName).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			framework.Logf("unexpected err during cluster quota listing: %v", err)
			return false, nil
		}
		if len(list.Items) > 0 && len(list.Items[0].Status.Total.Hard) > 0 {
			return true, nil
		}
		currentCQ, err := clusterAdminClient.QuotaV1().ClusterResourceQuotas().Get(context.Background(), cqName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("unexpected err during cluster quota %s retrieval: %v", cqName, err)
		}
		if currentCQ != nil {
			framework.Logf("latest status of cluster quota %s is %#v", cqName, currentCQ)
		}

		return false, nil
	})
}

func labelNamespace(clusterAdminKubeClient corev1client.NamespacesGetter, labelKey, namespaceName string) error {
	ns1, err := clusterAdminKubeClient.Namespaces().Get(context.Background(), namespaceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if ns1.Labels == nil {
		ns1.Labels = map[string]string{}
	}
	ns1.Labels[labelKey] = "bar"
	if _, err := clusterAdminKubeClient.Namespaces().Update(context.Background(), ns1, metav1.UpdateOptions{}); err != nil {
		return err
	}
	return nil
}

func waitForQuotaStatus(clusterAdminClient quotaclient.Interface, name string, conditionFn func(*quotav1.ClusterResourceQuota) error) error {
	var pollErr error
	err := utilwait.PollImmediate(100*time.Millisecond, QuotaWaitTimeout, func() (done bool, err error) {
		quota, err := clusterAdminClient.QuotaV1().ClusterResourceQuotas().Get(context.Background(), name, metav1.GetOptions{})
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

func getTestServiceMonitor(name, namespace string) *unstructured.Unstructured {
	testServiceMonitor := fmt.Sprintf(`apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  generateName: %s-
spec:
  endpoints: []
  namespaceSelector:
    matchNames:
      - %s
  selector:
    matchLabels:
      foo: bar
      app: %s
`, name, namespace, name)
	return resourceread.ReadUnstructuredOrDie([]byte(testServiceMonitor))
}
