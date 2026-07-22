package hapolicymanagement

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type haPolicyManagementChecker struct {
	kubeClient kubernetes.Interface
}

func NewAnalyzer() monitortestframework.MonitorTest {
	return &haPolicyManagementChecker{}
}

func (w *haPolicyManagementChecker) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *haPolicyManagementChecker) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	var err error
	w.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	return nil
}

func checkNonMetricsPorts(container corev1.Container) bool {
	for _, port := range container.Ports {
		if port.Name != "metrics" {
			return true
		}
	}
	return false
}

func checkPodReadinessLivenessProbes(namespace string, initOrSidecar bool, container corev1.Container) error {
	if initOrSidecar {
		return nil
	}

	if container.LivenessProbe == nil {
		return fmt.Errorf("container %s in ns/%s should have livenessProbe", container.Name, namespace)
	}

	if !checkNonMetricsPorts(container) {
		return nil
	}

	if container.ReadinessProbe == nil {
		return fmt.Errorf("container %s in ns/%s should have readinessProbe", container.Name, namespace)
	}

	return nil
}

func checkPodStartupProbes(namespace string, initOrSidecar bool, container corev1.Container) error {
	if initOrSidecar {
		return nil
	}

	if container.StartupProbe == nil && checkNonMetricsPorts(container) {
		return fmt.Errorf("container %s in ns/%s should have startupProbe", container.Name, namespace)
	}

	return nil
}

func checkPodDisruptionBudget(namespace string, name string, podTemplate corev1.PodTemplateSpec, selector *metav1.LabelSelector, pdbList *policyv1.PodDisruptionBudgetList) error {
	depSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return fmt.Errorf("invalid label selector: %w", err)
	}

	podLabels := labels.Set(podTemplate.Labels)

	var matchedPDB *policyv1.PodDisruptionBudget
	for _, pdb := range pdbList.Items {
		if pdb.Spec.Selector == nil {
			continue
		}

		pdbSelector, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
		if err != nil {
			continue
		}

		// Perform bidirectional selector matching: verify that both the PDB selector
		// and the Deployment selector successfully target the same underlying Pod labels.
		if pdbSelector.Matches(podLabels) && depSelector.Matches(podLabels) {
			matchedPDB = &pdb
			break
		}
	}

	if matchedPDB == nil {
		return fmt.Errorf("workload %s in ns/%s should have PodDisruptionBudget", name, namespace)
	}

	return nil
}

func checkPodAntiAffinity(namespace string, name string, podTemplate corev1.PodTemplateSpec) error {
	podSpec := podTemplate.Spec

	if podSpec.Affinity == nil || podSpec.Affinity.PodAntiAffinity == nil {
		return nil
	}

	antiAffinity := podSpec.Affinity.PodAntiAffinity
	hasRequired := len(antiAffinity.RequiredDuringSchedulingIgnoredDuringExecution) > 0
	hasPreferred := len(antiAffinity.PreferredDuringSchedulingIgnoredDuringExecution) > 0

	if !hasRequired && !hasPreferred {
		return fmt.Errorf("workload %s in ns/%s should have podAntiAffinity", name, namespace)
	}

	return nil
}

func hasControllerOwner(pod *corev1.Pod) bool {
	if len(pod.OwnerReferences) == 0 {
		return false
	}

	for _, ref := range pod.OwnerReferences {
		if ref.Controller != nil && *ref.Controller {
			return true
		}
	}

	return false
}

func (w *haPolicyManagementChecker) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w.kubeClient == nil {
		return nil, nil, nil
	}

	namespaces, err := w.kubeClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}

	junits := []*junitapi.JUnitTestCase{}
	for _, ns := range namespaces.Items {
		// skip managed service namespaces
		if exutil.ManagedServiceNamespaces.Has(ns.Name) {
			continue
		}

		// Filter OpenShift namespaces
		isOpenShiftNamespace := ns.Name == "openshift" || strings.HasPrefix(ns.Name, "openshift-")
		if !(strings.HasPrefix(ns.Name, "kube-") || ns.Name == "default" || isOpenShiftNamespace) {
			continue
		}

		var failures []string

		pdbList, _ := w.kubeClient.PolicyV1().PodDisruptionBudgets(ns.Name).List(ctx, metav1.ListOptions{})
		deployments, _ := w.kubeClient.AppsV1().Deployments(ns.Name).List(ctx, metav1.ListOptions{})
		for _, dep := range deployments.Items {
			for _, container := range dep.Spec.Template.Spec.Containers {
				if err := checkPodReadinessLivenessProbes(ns.Name, true, container); err != nil {
					failures = append(failures, err.Error())
				}
				if err := checkPodStartupProbes(ns.Name, true, container); err != nil {
					failures = append(failures, err.Error())
				}
			}
			for _, container := range dep.Spec.Template.Spec.InitContainers {
				if err := checkPodReadinessLivenessProbes(ns.Name, false, container); err != nil {
					failures = append(failures, err.Error())
				}
				if err := checkPodStartupProbes(ns.Name, false, container); err != nil {
					failures = append(failures, err.Error())
				}
			}

			if dep.Spec.Replicas != nil && *dep.Spec.Replicas <= 1 {
				continue
			}

			if err := checkPodDisruptionBudget(ns.Name, dep.Name, dep.Spec.Template, dep.Spec.Selector, pdbList); err != nil {
				failures = append(failures, err.Error())
			}
			if err := checkPodAntiAffinity(ns.Name, dep.Name, dep.Spec.Template); err != nil {
				failures = append(failures, err.Error())
			}
		}

		statefulsets, _ := w.kubeClient.AppsV1().StatefulSets(ns.Name).List(ctx, metav1.ListOptions{})
		for _, sts := range statefulsets.Items {
			for _, container := range sts.Spec.Template.Spec.Containers {
				if err := checkPodReadinessLivenessProbes(ns.Name, true, container); err != nil {
					failures = append(failures, err.Error())
				}
				if err := checkPodStartupProbes(ns.Name, true, container); err != nil {
					failures = append(failures, err.Error())
				}
			}
			for _, container := range sts.Spec.Template.Spec.InitContainers {
				if err := checkPodReadinessLivenessProbes(ns.Name, false, container); err != nil {
					failures = append(failures, err.Error())
				}
				if err := checkPodStartupProbes(ns.Name, false, container); err != nil {
					failures = append(failures, err.Error())
				}
			}

			if sts.Spec.Replicas != nil && *sts.Spec.Replicas <= 1 {
				continue
			}

			if err := checkPodDisruptionBudget(ns.Name, sts.Name, sts.Spec.Template, sts.Spec.Selector, pdbList); err != nil {
				failures = append(failures, err.Error())
			}
			if err := checkPodAntiAffinity(ns.Name, sts.Name, sts.Spec.Template); err != nil {
				failures = append(failures, err.Error())
			}
		}

		daemonsets, _ := w.kubeClient.AppsV1().DaemonSets(ns.Name).List(ctx, metav1.ListOptions{})
		for _, dms := range daemonsets.Items {
			for _, container := range dms.Spec.Template.Spec.Containers {
				if err := checkPodReadinessLivenessProbes(ns.Name, true, container); err != nil {
					failures = append(failures, err.Error())
				}
				if err := checkPodStartupProbes(ns.Name, true, container); err != nil {
					failures = append(failures, err.Error())
				}
			}
			for _, container := range dms.Spec.Template.Spec.InitContainers {
				if err := checkPodReadinessLivenessProbes(ns.Name, false, container); err != nil {
					failures = append(failures, err.Error())
				}
				if err := checkPodStartupProbes(ns.Name, false, container); err != nil {
					failures = append(failures, err.Error())
				}
			}
		}

		pods, _ := w.kubeClient.CoreV1().Pods(ns.Name).List(ctx, metav1.ListOptions{})
		for _, pod := range pods.Items {
			// Filter bare pods
			if hasControllerOwner(&pod) {
				continue
			}

			for _, container := range pod.Spec.Containers {
				if err := checkPodReadinessLivenessProbes(ns.Name, true, container); err != nil {
					failures = append(failures, err.Error())
				}
				if err := checkPodStartupProbes(ns.Name, true, container); err != nil {
					failures = append(failures, err.Error())
				}
			}
			for _, container := range pod.Spec.InitContainers {
				if err := checkPodReadinessLivenessProbes(ns.Name, false, container); err != nil {
					failures = append(failures, err.Error())
				}
				if err := checkPodStartupProbes(ns.Name, false, container); err != nil {
					failures = append(failures, err.Error())
				}
			}
		}

		testName := fmt.Sprintf("[Monitor:ha-compliance][sig-arch] workload in ns/%s should comply with HA policy", ns.Name)

		if len(failures) > 0 {
			junits = append(junits, &junitapi.JUnitTestCase{
				Name:      testName,
				SystemOut: strings.Join(failures, "\n"),
			})
		} else {
			junits = append(junits, &junitapi.JUnitTestCase{
				Name: testName,
			})
		}
	}

	return nil, junits, nil
}

func (w *haPolicyManagementChecker) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *haPolicyManagementChecker) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (w *haPolicyManagementChecker) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (w *haPolicyManagementChecker) Cleanup(ctx context.Context) error {
	return nil
}
