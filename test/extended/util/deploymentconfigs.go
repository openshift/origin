package util

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	appsv1 "github.com/openshift/api/apps/v1"
	appsv1clienttyped "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	"github.com/openshift/library-go/pkg/apps/appsutil"
)

// WaitForDeploymentConfig waits for a DeploymentConfig to complete transition
// to a given version and report minimum availability.
func WaitForDeploymentConfig(kc kubernetes.Interface, dcClient appsv1clienttyped.DeploymentConfigsGetter, namespace, name string, version int64, enforceNotProgressing bool, cli *CLI) error {
	e2e.Logf("waiting for deploymentconfig %s/%s to be available with version %d\n", namespace, name, version)
	var dc *appsv1.DeploymentConfig

	start := time.Now()
	err := wait.Poll(time.Second, 15*time.Minute, func() (done bool, err error) {
		dc, err = dcClient.DeploymentConfigs(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// TODO re-enable this check once @mfojtik introduces a test that ensures we'll only ever get
		// exactly one deployment triggered.
		/*
			if dc.Status.LatestVersion > version {
				return false, fmt.Errorf("latestVersion %d passed %d", dc.Status.LatestVersion, version)
			}
		*/
		if dc.Status.LatestVersion < version {
			return false, nil
		}

		var progressing, available *appsv1.DeploymentCondition
		for i, condition := range dc.Status.Conditions {
			switch condition.Type {
			case appsv1.DeploymentProgressing:
				progressing = &dc.Status.Conditions[i]

			case appsv1.DeploymentAvailable:
				available = &dc.Status.Conditions[i]
			}
		}

		if enforceNotProgressing {
			if progressing != nil && progressing.Status == corev1.ConditionFalse {
				return false, fmt.Errorf("not progressing")
			}
		}

		if progressing != nil &&
			progressing.Status == corev1.ConditionTrue &&
			progressing.Reason == appsutil.NewRcAvailableReason &&
			available != nil &&
			available.Status == corev1.ConditionTrue {
			return true, nil
		}

		return false, nil
	})

	if err != nil {
		e2e.Logf("got error %q when waiting for deploymentconfig %s/%s to be available with version %d\n", err, namespace, name, version)
		cli.Run("get").Args("dc", dc.Name, "-o", "yaml").Execute()

		DumpDeploymentLogs(name, version, cli)
		DumpApplicationPodLogs(name, cli)

		return err
	}

	requirement, err := labels.NewRequirement(appsutil.DeploymentLabel, selection.Equals, []string{appsutil.LatestDeploymentNameForConfigAndVersion(
		dc.Name, dc.Status.LatestVersion)})
	if err != nil {
		return err
	}

	podnames, err := GetPodNamesByFilter(kc.CoreV1().Pods(namespace), labels.NewSelector().Add(*requirement), func(corev1.Pod) bool { return true })
	if err != nil {
		return err
	}

	e2e.Logf("deploymentconfig %s/%s available after %s\npods: %s\n", namespace, name, time.Now().Sub(start), strings.Join(podnames, ", "))

	return nil
}

// RemoveDeploymentConfigs deletes the given DeploymentConfigs in a namespace
func RemoveDeploymentConfigs(oc *CLI, dcs ...string) error {
	errs := []error{}
	for _, dc := range dcs {
		e2e.Logf("Removing deployment config %s/%s", oc.Namespace(), dc)
		if err := oc.AdminAppsClient().AppsV1().DeploymentConfigs(oc.Namespace()).Delete(context.Background(), dc, metav1.DeleteOptions{}); err != nil {
			e2e.Logf("Error occurred removing deployment config: %v", err)
			errs = append(errs, err)
		}

		err := wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
			pods, err := GetApplicationPods(oc, dc)
			if err != nil {
				e2e.Logf("Unable to get pods for dc/%s: %v", dc, err)
				return false, err
			}
			if len(pods.Items) > 0 {
				e2e.Logf("Waiting for pods for dc/%s to terminate", dc)
				return false, nil
			}
			e2e.Logf("Pods for dc/%s have terminated", dc)
			return true, nil
		})

		if err != nil {
			e2e.Logf("Error occurred waiting for pods to terminate for dc/%s: %v", dc, err)
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return kutilerrors.NewAggregate(errs)
	}

	return nil
}

func GetDeploymentConfigPods(oc *CLI, dcName string, version int64) (*corev1.PodList, error) {
	return oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).List(context.Background(), metav1.ListOptions{LabelSelector: ParseLabelsOrDie(fmt.Sprintf("%s=%s-%d",
		appsv1.DeployerPodForDeploymentLabel, dcName, version)).String()})
}

func GetApplicationPods(oc *CLI, dcName string) (*corev1.PodList, error) {
	return oc.AdminKubeClient().CoreV1().Pods(oc.Namespace()).List(context.Background(), metav1.ListOptions{LabelSelector: ParseLabelsOrDie(fmt.Sprintf("deploymentconfig=%s", dcName)).String()})
}

// DumpDeploymentLogs will dump the latest deployment logs for a DeploymentConfig for debug purposes
func DumpDeploymentLogs(dcName string, version int64, oc *CLI) {
	e2e.Logf("Dumping deployment logs for deploymentconfig %q\n", dcName)

	pods, err := GetDeploymentConfigPods(oc, dcName, version)
	if err != nil {
		e2e.Logf("Unable to retrieve pods for deploymentconfig %q: %v\n", dcName, err)
		return
	}

	DumpPodLogs(pods.Items, oc)
}

// DumpApplicationPodLogs will dump the latest application logs for a DeploymentConfig for debug purposes
func DumpApplicationPodLogs(dcName string, oc *CLI) {
	e2e.Logf("Dumping application logs for deploymentconfig %q\n", dcName)

	pods, err := GetApplicationPods(oc, dcName)
	if err != nil {
		e2e.Logf("Unable to retrieve pods for deploymentconfig %q: %v\n", dcName, err)
		return
	}

	DumpPodLogs(pods.Items, oc)
}
