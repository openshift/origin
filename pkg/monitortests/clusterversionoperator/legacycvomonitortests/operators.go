package legacycvomonitortests

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/origin/pkg/monitortests/clusterversionoperator/operatorstateanalyzer"
	"github.com/sirupsen/logrus"

	configv1 "github.com/openshift/api/config/v1"
	clientconfigv1 "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	platformidentification2 "github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/client-go/rest"
)

// exceptionCallback consumes a suspicious condition and returns an
// exception string if does not think the condition should be fatal.
type exceptionCallback func(operator string, condition *configv1.ClusterOperatorStatusCondition, eventInterval monitorapi.Interval, clientConfig *rest.Config) (string, error)

type upgradeWindowHolder struct {
	startInterval *monitorapi.Interval
	endInterval   *monitorapi.Interval
}

func checkAuthenticationAvailableExceptions(condition *configv1.ClusterOperatorStatusCondition) bool {
	if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse {
		switch condition.Reason {
		case "APIServices_Error", "APIServerDeployment_NoDeployment", "APIServerDeployment_NoPod",
			"APIServerDeployment_PreconditionNotFulfilled", "APIServices_PreconditionNotReady",
			"OAuthServerDeployment_NoDeployment", "OAuthServerRouteEndpointAccessibleController_EndpointUnavailable",
			"OAuthServerServiceEndpointAccessibleController_EndpointUnavailable", "WellKnown_NotReady":
			return true
		}
	}
	return false
}

func testStableSystemOperatorStateTransitions(events monitorapi.Intervals, clientConfig *rest.Config) []*junitapi.JUnitTestCase {
	isSingleNode, err := isSingleNodeCheck(clientConfig)
	if err != nil {
		logrus.Warnf("Error checking for Single Node configuration on upgrade (unable to make exception): %v", err)
		isSingleNode = false
	}

	except := func(operator string, condition *configv1.ClusterOperatorStatusCondition, _ monitorapi.Interval, clientConfig *rest.Config) (string, error) {
		if condition.Status == configv1.ConditionTrue {
			if condition.Type == configv1.OperatorAvailable {
				return fmt.Sprintf("%s=%s is the happy case", condition.Type, condition.Status), nil
			}
		} else if condition.Status == configv1.ConditionFalse {
			if condition.Type == configv1.OperatorDegraded {
				return fmt.Sprintf("%s=%s is the happy case", condition.Type, condition.Status), nil
			}
		}

		if isSingleNode {
			switch operator {
			case "dns":
				if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse &&
					strings.Contains(condition.Message, `DNS "default" is unavailable.`) {
					return "dns operator is allowed to have Available=False due to serial taint tests on single node", nil
				}
				if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue &&
					strings.Contains(condition.Message, `DNS default is degraded`) {
					return "dns operator is allowed to have Degraded=True due to serial taint tests on single node", nil
				}
			case "openshift-apiserver":
				if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse &&
					strings.Contains(condition.Message, `connect: connection refused`) {
					return "openshift apiserver operator is allowed to have Available=False due kube-apiserver force rollout test on single node", nil
				}
			case "csi-snapshot-controller":
				if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse &&
					strings.Contains(condition.Message, `Waiting for Deployment`) {
					return "csi snapshot controller is allowed to have Available=False due to CSI webhook test on single node", nil
				}
			}
		}

		// For the non-upgrade case, if any operator has Available=False, fail the test.
		if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse {
			if operator == "authentication" {
				if checkAuthenticationAvailableExceptions(condition) {
					return "https://issues.redhat.com/browse/OCPBUGS-20056", nil
				}
			}
			if operator == "image-registry" {
				return "Image-registry operator is allowed to have Available=False on a non-upgrade scenario for now", nil
			}
			return "", nil
		}
		if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue {
			if operator == "cloud-controller-manager" && condition.Reason == "SyncingFailed" {
				return "https://issues.redhat.com/browse/OCPBUGS-42837", nil
			}
			if operator == "dns" && condition.Reason == "DNSDegraded" {
				return "https://issues.redhat.com/browse/OCPBUGS-38750", nil
			}
			if operator == "etcd" && (condition.Reason == "EtcdMembers_UnhealthyMembers" || condition.Reason == "NodeInstaller_InstallerPodFailed") {
				return "https://issues.redhat.com/browse/OCPBUGS-38659", nil
			}
			if operator == "network" && (condition.Reason == "ApplyOperatorConfig" || condition.Reason == "RenderError") {
				return "https://issues.redhat.com/browse/OCPBUGS-38684", nil
			}
			if operator == "machine-config" &&
				(condition.Reason == "MachineConfigDaemonFailed" ||
					condition.Reason == "MachineConfigurationFailed" ||
					condition.Reason == "MachineConfigNodeFailed" ||
					condition.Reason == "MachineConfigControllerFailed") {
				return "https://issues.redhat.com/browse/OCPBUGS-38749", nil
			}
			if operator == "authentication" && (condition.Reason == "OAuthServerDeployment_UnavailablePod" || condition.Reason == "WellKnownReadyController_SyncError") {
				return "https://issues.redhat.com/browse/OCPBUGS-38675", nil
			}
			// flake this to collect more exceptions
			return "https://issues.redhat.com/browse/TRT-1575", nil
		}
		return "We are not worried about other operator condition blips for stable-system tests yet.", nil
	}

	return testOperatorStateTransitions(events, []configv1.ClusterStatusConditionType{configv1.OperatorAvailable, configv1.OperatorDegraded}, except, clientConfig)
}

func isSingleNodeCheck(clientConfig *rest.Config) (bool, error) {
	configClient, err := clientconfigv1.NewForConfig(clientConfig)
	if err != nil {
		logrus.WithError(err).Error("Error creating config client to check for Single Node configuration")
		return false, err
	}
	return exutil.IsSingleNode(context.Background(), configClient)
}

// isInUpgradeWindow determines if the given eventInterval falls within an upgrade window.
// UpgradeStart and UpgradeRollback events start upgrade windows and can end and already started upgrade window.
// UpgradeComplete and UpgradeFailed events end upgrade windows; if there was not an already started upgrade window,
// we ignore the event.
// If we don't find any upgrade ending point, we assume the ending point is at the end of the test.
func getUpgradeWindows(eventList monitorapi.Intervals) []*upgradeWindowHolder {

	var upgradeWindows []*upgradeWindowHolder
	var currentWindow *upgradeWindowHolder

	for _, event := range eventList {
		if event.Source != monitorapi.SourceKubeEvent || event.Locator.Keys[monitorapi.LocatorClusterVersionKey] != "cluster" {
			continue
		}

		switch event.Message.Reason {
		case monitorapi.UpgradeStartedReason, monitorapi.UpgradeRollbackReason:
			if currentWindow != nil {
				// Close current window since there's already an upgrade window started
				currentWindow.endInterval = &monitorapi.Interval{
					Condition: monitorapi.Condition{
						Message: monitorapi.Message{
							Reason: event.Message.Reason,
						},
					},
					From: event.From,
					To:   event.To,
				}
			}

			// Start new window
			currentWindow = &upgradeWindowHolder{
				startInterval: &monitorapi.Interval{
					Condition: monitorapi.Condition{
						Message: monitorapi.Message{
							Reason: event.Message.Reason,
						},
					},
					From: event.From,
					To:   event.To,
				},
			}
			upgradeWindows = append(upgradeWindows, currentWindow)
		case monitorapi.UpgradeCompleteReason, monitorapi.UpgradeFailedReason:
			if currentWindow != nil {
				if currentWindow.endInterval == nil {
					// End current window
					currentWindow.endInterval = &monitorapi.Interval{
						Condition: monitorapi.Condition{
							Message: monitorapi.Message{
								Reason: event.Message.Reason,
							},
						},
						From: event.From,
						To:   event.To,
					}
				}
			} else {
				// We have no current window which means that the events indicate we completed
				// or failed an upgrade without starting one.  This is stange situation that
				// we should not see; in this case, there is no upgrade window to check against.
				logrus.Warnf("Found upgrade completion or failed event without a start or rollback event: %v", event)
			}
		}
	}

	return upgradeWindows
}

func isInUpgradeWindow(upgradeWindows []*upgradeWindowHolder, eventInterval monitorapi.Interval) bool {
	for _, upgradeWindow := range upgradeWindows {
		if eventInterval.From.After(upgradeWindow.startInterval.From) {
			if upgradeWindow.endInterval == nil || eventInterval.To.Before(upgradeWindow.endInterval.To) {
				return true
			}
		}
	}

	return false
}

func testUpgradeOperatorStateTransitions(events monitorapi.Intervals, clientConfig *rest.Config) []*junitapi.JUnitTestCase {
	upgradeWindows := getUpgradeWindows(events)
	isSingleNode, err := isSingleNodeCheck(clientConfig)
	if err != nil {
		logrus.Warnf("Error checking for Single Node configuration on upgrade (unable to make exception): %v", err)
		isSingleNode = false
	}

	except := func(operator string, condition *configv1.ClusterOperatorStatusCondition, eventInterval monitorapi.Interval, clientConfig *rest.Config) (string, error) {
		if condition.Status == configv1.ConditionTrue {
			if condition.Type == configv1.OperatorAvailable {
				return fmt.Sprintf("%s=%s is the happy case", condition.Type, condition.Status), nil
			}
		} else if condition.Status == configv1.ConditionFalse {
			if condition.Type == configv1.OperatorDegraded {
				return fmt.Sprintf("%s=%s is the happy case", condition.Type, condition.Status), nil
			}
		}

		withinUpgradeWindowBuffer := isInUpgradeWindow(upgradeWindows, eventInterval) && eventInterval.To.Sub(eventInterval.From) < 10*time.Minute
		if !withinUpgradeWindowBuffer {
			switch operator {
			// there are some known cases for authentication and image-registry that occur outside of upgrade window, so we will pass through and check for exceptions
			case "authentication":
				if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse {
					logrus.Infof("Operator %s is in Available=False state outside of upgrade window, but we will check for exceptions", operator)
				} else if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue {
					logrus.Infof("Operator %s is in Degraded=True state outside of upgrade window, but we will check for exceptions", operator)
				} else {
					return "", nil
				}
			case "image-registry":
				if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse {
					logrus.Infof("Operator %s is in Available=False state outside of upgrade window, but we will check for exceptions", operator)
				} else {
					return "", nil
				}
			case "monitoring":
				if condition.Type == configv1.OperatorDegraded && (condition.Status == configv1.ConditionTrue && condition.Reason == "UpdatingPrometheusFailed") {
					return "https://issues.redhat.com/browse/OCPBUGS-39026", nil
				}
			case "network":
				if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue {
					logrus.Infof("Operator %s is in Degraded=True state outside of upgrade window, but we will check for exceptions", operator)
				} else {
					return "", nil
				}
			default:
				// flake this to collect more exceptions
				if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue {
					return "outside of upgrade window https://issues.redhat.com/browse/TRT-1575", nil
				} else {
					return "", nil
				}
			}
		} else {
			// SingleNode is expected to go Available=False and Degraded=True for most / all operators during upgrade
			if isSingleNode {
				return fmt.Sprintf("Operator %s is in %s=%s state running in single replica control plane, expected availability transition during upgrade", operator, condition.Type, condition.Status), nil
			}
		}

		switch operator {
		case "authentication":
			if isSingleNode && condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue {
				return "https://issues.redhat.com/browse/OCPBUGS-38675", nil
			} else if checkAuthenticationAvailableExceptions(condition) {
				return "https://issues.redhat.com/browse/OCPBUGS-20056", nil
			}
		case "cluster-autoscaler":
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "MissingDependency" {
				return "https://issues.redhat.com/browse/OCPBUGS-42875", nil
			}
		case "cloud-controller-manager":
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "SyncingFailed" {
				return "https://issues.redhat.com/browse/OCPBUGS-42837", nil
			}
		case "cloud-credential":
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue &&
				(condition.Reason == "CredentialsFailing" ||
					condition.Reason == "StaticResourceReconcileFailed") {
				return "https://issues.redhat.com/browse/OCPBUGS-42872", nil
			}
		case "console":
			if isSingleNode && condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue {
				return "https://issues.redhat.com/browse/OCPBUGS-38676", nil
			} else if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse &&
				(condition.Reason == "RouteHealth_FailedGet" ||
					condition.Reason == "RouteHealth_RouteNotAdmitted" ||
					condition.Reason == "RouteHealth_StatusError") {
				return "https://issues.redhat.com/browse/OCPBUGS-24041", nil
			}
		case "control-plane-machine-set":
			if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse && condition.Reason == "UnavailableReplicas" {
				return "https://issues.redhat.com/browse/OCPBUGS-20061", nil
			}
		case "ingress":
			if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse && condition.Reason == "IngressUnavailable" {
				return "https://issues.redhat.com/browse/OCPBUGS-25739", nil
			}
		case "kube-storage-version-migrator":
			if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse && condition.Reason == "KubeStorageVersionMigrator_Deploying" {
				return "https://issues.redhat.com/browse/OCPBUGS-20062", nil
			}
		case "machine-api":
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "SyncingFailed" {
				return "https://issues.redhat.com/browse/OCPBUGS-44332", nil
			}
		case "machine-config":
			if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse && condition.Reason == "MachineConfigControllerFailed" && strings.Contains(condition.Message, "notAfter: Required value") {
				return "https://issues.redhat.com/browse/OCPBUGS-22364", nil
			}
			if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse && strings.Contains(condition.Message, "missing HTTP content-type") {
				return "https://issues.redhat.com/browse/OCPBUGS-24228", nil
			}
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue &&
				(condition.Reason == "MachineConfigDaemonFailed" ||
					condition.Reason == "RenderConfigFailed" ||
					condition.Reason == "MachineConfigPoolsFailed" ||
					condition.Reason == "MachineConfigControllerFailed") {
				return "https://issues.redhat.com/browse/OCPBUGS-39199", nil
			}
		case "monitoring":
			if condition.Type == configv1.OperatorAvailable &&
				(condition.Status == configv1.ConditionFalse &&
					(condition.Reason == "PlatformTasksFailed" ||
						condition.Reason == "UpdatingAlertmanagerFailed" ||
						condition.Reason == "UpdatingConsolePluginComponentsFailed" ||
						condition.Reason == "UpdatingPrometheusK8SFailed" ||
						condition.Reason == "UpdatingPrometheusOperatorFailed")) ||
				(condition.Status == configv1.ConditionUnknown && condition.Reason == "UpdatingPrometheusFailed") {
				return "https://issues.redhat.com/browse/OCPBUGS-23745", nil
			}
			if condition.Type == configv1.OperatorDegraded &&
				(condition.Status == configv1.ConditionTrue && (condition.Reason == "UpdatingPrometheusFailed" || condition.Reason == "UpdatingPrometheusOperatorFailed")) {
				return "https://issues.redhat.com/browse/OCPBUGS-39026", nil
			}
		case "openshift-apiserver":
			if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse &&
				(condition.Reason == "APIServerDeployment_NoDeployment" ||
					condition.Reason == "APIServerDeployment_NoPod" ||
					condition.Reason == "APIServerDeployment_PreconditionNotFulfilled" ||
					condition.Reason == "APIServerDeployment_UnavailablePod" ||
					condition.Reason == "APIServices_Error") {
				return "https://issues.redhat.com/browse/OCPBUGS-23746", nil
			}
		case "openshift-controller-manager":
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && (condition.Reason == "OpenshiftControllerManagerStaticResources_SyncError") {
				return "https://issues.redhat.com/browse/OCPBUGS-42870", nil
			}
		case "operator-lifecycle-manager-packageserver":
			if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionFalse && condition.Reason == "ClusterServiceVersionNotSucceeded" {
				return "https://issues.redhat.com/browse/OCPBUGS-23744", nil
			}
		case "image-registry":
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && (condition.Reason == "NodeCADaemonControllerError" || condition.Reason == "ProgressDeadlineExceeded") {
				return "https://issues.redhat.com/browse/OCPBUGS-38667", nil
			}
			// this won't handle the replicaCount==2 serial test where both pods are on nodes that get tainted.
			// need to consider how we detect that or modify the job to set replicaCount==3
			if replicaCount, _ := checkReplicas("openshift-image-registry", operator, clientConfig); replicaCount == 1 {
				return "https://issues.redhat.com/browse/OCPBUGS-22382", nil
			}
		case "dns":
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "DNSDegraded" {
				return "https://issues.redhat.com/browse/OCPBUGS-38666", nil
			}
		case "etcd":
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "EtcdCertSignerController_Error::EtcdEndpoints_ErrorUpdatingEtcdEndpoints::EtcdMembers_UnhealthyMembers::NodeController_MasterNodesReady::TargetConfigController_SynchronizationError" {
				return "https://issues.redhat.com/browse/OCPBUGS-38659", nil
			}
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "EtcdCertSignerController_Error::EtcdEndpoints_ErrorUpdatingEtcdEndpoints::EtcdMembers_UnhealthyMembers::TargetConfigController_SynchronizationError" {
				return "https://issues.redhat.com/browse/OCPBUGS-38659", nil
			}
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "EtcdCertSignerController_Error::EtcdEndpoints_ErrorUpdatingEtcdEndpoints::EtcdMembersController_ErrorUpdatingReportEtcdMembers::EtcdMembers_UnhealthyMembers::TargetConfigController_SynchronizationError" {
				return "https://issues.redhat.com/browse/OCPBUGS-38659", nil
			}
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "EtcdEndpoints_ErrorUpdatingEtcdEndpoints::EtcdMembers_UnhealthyMembers::TargetConfigController_SynchronizationError" {
				return "https://issues.redhat.com/browse/OCPBUGS-38659", nil
			}
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "NodeController_MasterNodesReady::StaticPods_Error" {
				return "https://issues.redhat.com/browse/OCPBUGS-38659", nil
			}
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "EtcdMembers_UnhealthyMembers::NodeController_MasterNodesReady::StaticPods_Error" {
				return "https://issues.redhat.com/browse/OCPBUGS-38659", nil
			}
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "EtcdCertSignerController_Error::EtcdEndpoints_ErrorUpdatingEtcdEndpoints::EtcdMembers_UnhealthyMembers::NodeController_MasterNodesReady::StaticPods_Error::TargetConfigController_SynchronizationError" {
				return "https://issues.redhat.com/browse/OCPBUGS-38659", nil
			}
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "EtcdCertSignerController_Error::EtcdEndpoints_ErrorUpdatingEtcdEndpoints::TargetConfigController_SynchronizationError" {
				return "https://issues.redhat.com/browse/OCPBUGS-38659", nil
			}
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "Unknown" {
				return "https://issues.redhat.com/browse/OCPBUGS-38659", nil
			}
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "EtcdMembers_UnhealthyMembers" {
				return "https://issues.redhat.com/browse/OCPBUGS-38659", nil
			}
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "ClusterMemberController_SyncError::EtcdMembersController_ErrorUpdatingReportEtcdMembers::EtcdMembers_UnhealthyMembers" {
				return "https://issues.redhat.com/browse/OCPBUGS-38659", nil
			}
		case "network":
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "ApplyOperatorConfig" {
				return "https://issues.redhat.com/browse/OCPBUGS-38668", nil
			}
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue &&
				(condition.Reason == "NoOperConfig" || condition.Reason == "BootstrapError" || condition.Reason == "RenderError" || condition.Reason == "RolloutHung") {
				return "https://issues.redhat.com/browse/OCPBUGS-38668", nil
			}
		case "openshift-samples":
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "APIServerServiceUnavailableError" {
				return "https://issues.redhat.com/browse/OCPBUGS-38679", nil
			}
		case "kube-apiserver":
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue {
				if isSingleNode && condition.Reason == "NodeInstaller_InstallerPodFailed" {
					return "https://issues.redhat.com/browse/OCPBUGS-38678", nil
				}
				if condition.Reason == "NodeController_MasterNodesReady::StaticPods_Error" ||
					condition.Reason == "NodeController_MasterNodesReady" ||
					condition.Reason == "KubeAPIServerStaticResources_SyncError" {
					return "https://issues.redhat.com/browse/OCPBUGS-38661", nil
				}
			}
		case "kube-controller-manager":
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "NodeController_MasterNodesReady" {
				return "https://issues.redhat.com/browse/OCPBUGS-38662", nil
			}
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "NodeController_MasterNodesReady::StaticPods_Error" {
				return "https://issues.redhat.com/browse/OCPBUGS-38662", nil
			}
		case "kube-scheduler":
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "NodeController_MasterNodesReady" {
				return "https://issues.redhat.com/browse/OCPBUGS-38663", nil
			}
			if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue && condition.Reason == "NodeController_MasterNodesReady::StaticPods_Error" {
				return "https://issues.redhat.com/browse/OCPBUGS-38663", nil
			}
		}

		// flake this to collect more exceptions
		if condition.Type == configv1.OperatorDegraded && condition.Status == configv1.ConditionTrue {
			return "https://issues.redhat.com/browse/TRT-1575", nil
		} else {
			return "", nil
		}
	}

	return testOperatorStateTransitions(events, []configv1.ClusterStatusConditionType{configv1.OperatorAvailable, configv1.OperatorDegraded}, except, clientConfig)
}

func checkReplicas(namespace string, operator string, clientConfig *rest.Config) (int32, error) {
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return 0, err
	}
	_, err = kubeClient.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		return 0, err
	}
	deployment, err := kubeClient.AppsV1().Deployments(namespace).Get(context.Background(), operator, metav1.GetOptions{})
	if err != nil {
		return 0, err
	}
	if deployment.Spec.Replicas != nil {
		return *deployment.Spec.Replicas, nil
	}
	return 0, fmt.Errorf("Error fetching replicas")
}

func testOperatorStateTransitions(events monitorapi.Intervals, conditionTypes []configv1.ClusterStatusConditionType, except exceptionCallback, clientConfig *rest.Config) []*junitapi.JUnitTestCase {
	ret := []*junitapi.JUnitTestCase{}

	var start, stop time.Time
	for _, event := range events {
		if start.IsZero() || event.From.Before(start) {
			start = event.From
		}
		if stop.IsZero() || event.To.After(stop) {
			stop = event.To
		}
	}
	duration := stop.Sub(start).Seconds()

	eventsByOperator := getEventsByOperator(events)
	e2eEventIntervals := operatorstateanalyzer.E2ETestEventIntervals(events)
	for _, conditionType := range conditionTypes {
		for _, operatorName := range platformidentification.KnownOperators.List() {
			bzComponent := platformidentification.GetBugzillaComponentForOperator(operatorName)
			if bzComponent == "Unknown" {
				bzComponent = operatorName
			}
			testName := fmt.Sprintf("[bz-%v] clusteroperator/%v should not change condition/%v", bzComponent, operatorName, conditionType)
			operatorEvents := eventsByOperator[operatorName]
			if len(operatorEvents) == 0 {
				ret = append(ret, &junitapi.JUnitTestCase{
					Name:     testName,
					Duration: duration,
				})
				continue
			}

			excepted := []string{}
			fatal := []string{}

			for _, eventInterval := range operatorEvents {
				condition := monitorapi.GetOperatorConditionStatus(eventInterval)
				if condition == nil {
					continue // ignore non-condition intervals
				}
				if len(condition.Type) == 0 {
					fatal = append(fatal, fmt.Sprintf("failed to convert %v into a condition with a type", eventInterval))
				}

				if condition.Type != conditionType {
					continue
				}

				// if there was any switch, it was wrong/unexpected at some point
				failure := fmt.Sprintf("%v", eventInterval)

				overlappingE2EIntervals := operatorstateanalyzer.FindOverlap(e2eEventIntervals, eventInterval.From, eventInterval.From)
				concurrentE2E := []string{}
				for _, overlap := range overlappingE2EIntervals {
					if overlap.Level == monitorapi.Info {
						continue
					}
					e2eTest, ok := monitorapi.E2ETestFromLocator(overlap.Locator)
					if !ok {
						continue
					}
					concurrentE2E = append(concurrentE2E, fmt.Sprintf("%v", e2eTest))
				}

				if len(concurrentE2E) > 0 {
					failure = fmt.Sprintf("%s\n%d tests failed during this blip (%v to %v): %v", failure, len(concurrentE2E), eventInterval.From, eventInterval.From, strings.Join(concurrentE2E, "\n"))
				}
				exception, err := except(operatorName, condition, eventInterval, clientConfig)
				if err != nil || exception == "" {
					fatal = append(fatal, failure)
				} else {
					excepted = append(excepted, fmt.Sprintf("%s (exception: %s)", failure, exception))
				}
			}

			output := fmt.Sprintf("%d unexpected clusteroperator state transitions during e2e test run", len(fatal))
			if len(fatal) > 0 {
				output = fmt.Sprintf("%s.  These did not match any known exceptions, so they cause this test-case to fail:\n\n%v\n", output, strings.Join(fatal, "\n"))
			} else {
				output = fmt.Sprintf("%s, as desired.", output)
			}
			output = fmt.Sprintf("%s\n%d unwelcome but acceptable clusteroperator state transitions during e2e test run", output, len(excepted))
			if len(excepted) > 0 {
				output = fmt.Sprintf("%s.  These should not happen, but because they are tied to exceptions, the fact that they did happen is not sufficient to cause this test-case to fail:\n\n%v\n", output, strings.Join(excepted, "\n"))
			} else {
				output = fmt.Sprintf("%s, as desired.", output)
			}

			if len(fatal) > 0 || len(excepted) > 0 {
				ret = append(ret, &junitapi.JUnitTestCase{
					Name:      testName,
					Duration:  duration,
					SystemOut: output,
					FailureOutput: &junitapi.FailureOutput{
						Output: output,
					},
				})
			}

			if len(fatal) == 0 {
				// add a success so we flake (or pass) and don't fail
				ret = append(ret, &junitapi.JUnitTestCase{Name: testName})
			}
		}
	}

	return ret
}

type startedStaged struct {
	// OSUpdateStarted is the event Reason emitted by the machine config operator when a node begins extracting
	// it's OS content.
	OSUpdateStarted time.Time
	// OSUpdateStaged is the event Reason emitted by the machine config operator when a node has extracted it's
	// OS content and is ready to begin the update. For the purposes of this test, we're looking for how long it
	// took from Started -> Staged to try to identify disk i/o problems that may be occurring.
	OSUpdateStaged time.Time
}

func testOperatorOSUpdateStaged(events monitorapi.Intervals, clientConfig *rest.Config) []*junitapi.JUnitTestCase {
	testName := "[bz-Machine Config Operator] Nodes should reach OSUpdateStaged in a timely fashion"
	success := &junitapi.JUnitTestCase{Name: testName}
	flakeThreshold := 5 * time.Minute
	failThreshold := 10 * time.Minute

	// Scan all OSUpdateStarted and OSUpdateStaged events, sort by node.
	nodeNameToOSUpdateTimes := map[string]*startedStaged{}
	for _, e := range events {
		nodeName := e.Locator.Keys[monitorapi.LocatorNodeKey]
		if len(nodeName) == 0 {
			continue
		}

		reason := e.Message.Reason
		phase := e.Message.Annotations[monitorapi.AnnotationPhase]
		switch {
		case reason == "OSUpdateStarted":
			_, ok := nodeNameToOSUpdateTimes[nodeName]
			if !ok {
				nodeNameToOSUpdateTimes[nodeName] = &startedStaged{}
			}
			// for this type of event, the from/to time are identical as this is a point in time event.
			ss := nodeNameToOSUpdateTimes[nodeName]
			ss.OSUpdateStarted = e.To

		case reason == "OSUpdateStaged":
			_, ok := nodeNameToOSUpdateTimes[nodeName]
			if !ok {
				nodeNameToOSUpdateTimes[nodeName] = &startedStaged{}
			}
			// for this type of event, the from/to time are identical as this is a point in time event.
			ss := nodeNameToOSUpdateTimes[nodeName]
			// this value takes priority over the backstop set based on the node update completion, so there's no reason
			// to perform a check, just directly overwrite.
			ss.OSUpdateStaged = e.To

		case phase == "Update":
			_, ok := nodeNameToOSUpdateTimes[nodeName]
			if !ok {
				nodeNameToOSUpdateTimes[nodeName] = &startedStaged{}
			}
			// This type of event indicates that an update completed. If an update completed  (which indicates we did
			// not receive it likely due to kube API/client issues), then we know that the latest
			// possible time that it could have OSUpdateStaged is when the update is finished.  If we have not yet observed
			// an OSUpdateStaged event, record this time as the final time.
			// Events are best effort, so if a process ends before an event is sent, it is never seen.
			// Ultimately, depending on, "I see everything as it happens and never miss anything" doesn't age well and
			// a change like this prevents failures due to, "something we don't really care about isn't absolutely perfect"
			// versus failures that really matter.  Without this, we're getting noise that we aren't going to devote time
			// to addressing.
			ss := nodeNameToOSUpdateTimes[nodeName]
			if ss.OSUpdateStaged.IsZero() {
				ss.OSUpdateStaged = e.To
			}
		}

	}

	// Iterate the data we assembled looking for any nodes with an excessive time between Started/Staged, or those
	// missing a Staged
	slowStageMessages := []string{}
	var failTest bool // set true if we see anything over 10 minutes, our failure threshold
	for node, ss := range nodeNameToOSUpdateTimes {
		if ss.OSUpdateStarted.IsZero() {
			// This case is handled by a separate test below.
			continue
		} else if ss.OSUpdateStaged.IsZero() || ss.OSUpdateStarted.After(ss.OSUpdateStaged) {
			// Watch that we don't do multiple started->staged transitions, if we see started > staged, we must have
			// failed to make it to staged on a later update:
			slowStageMessages = append(slowStageMessages, fmt.Sprintf("node/%s OSUpdateStarted at %s, did not make it to OSUpdateStaged", node, ss.OSUpdateStarted.Format(time.RFC3339)))
			failTest = true
		} else if ss.OSUpdateStaged.Sub(ss.OSUpdateStarted) > flakeThreshold {
			slowStageMessages = append(slowStageMessages, fmt.Sprintf("node/%s OSUpdateStarted at %s, OSUpdateStaged at %s: %s", node,
				ss.OSUpdateStarted.Format(time.RFC3339), ss.OSUpdateStaged.Format(time.RFC3339), ss.OSUpdateStaged.Sub(ss.OSUpdateStarted)))

			if ss.OSUpdateStaged.Sub(ss.OSUpdateStarted) > failThreshold {
				failTest = true
			}
		}
	}

	// Make sure we flake instead of fail the test on platforms that struggle to meet these thresholds.
	if failTest {
		// If an error occurs getting the platform, we're just going to let the test result stand.
		jobType, err := platformidentification2.GetJobType(context.TODO(), clientConfig)
		if err == nil && (jobType.Platform == "ovirt" || jobType.Platform == "metal") {
			failTest = false
		}
	}

	if len(slowStageMessages) > 0 {
		output := fmt.Sprintf("%d nodes took over %s to stage OSUpdate:\n\n%s",
			len(slowStageMessages), flakeThreshold, strings.Join(slowStageMessages, "\n"))
		failure := &junitapi.JUnitTestCase{
			Name:      testName,
			SystemOut: output,
			FailureOutput: &junitapi.FailureOutput{
				Output: output,
			},
		}
		if failTest {
			return []*junitapi.JUnitTestCase{failure}
		}
		return []*junitapi.JUnitTestCase{failure, success}
	}

	return []*junitapi.JUnitTestCase{success}
}

// testOperatorOSUpdateStartedEventRecorded provides data on a situation we've observed where the test framework is missing
// a started event, when we have a staged (completed) event. For now this test will flake to let us track how often this is occurring
// and verify once we have it fixed.
func testOperatorOSUpdateStartedEventRecorded(events monitorapi.Intervals, clientConfig *rest.Config) []*junitapi.JUnitTestCase {
	testName := "OSUpdateStarted event should be recorded for nodes that reach OSUpdateStaged"
	success := &junitapi.JUnitTestCase{Name: testName}

	// Scan all OSUpdateStarted and OSUpdateStaged events, sort by node.
	nodeOSUpdateTimes := map[string]*startedStaged{}
	for _, e := range events {
		if e.Message.Reason == "OSUpdateStarted" {
			// locator will be of the form: node/ci-op-j34hmfqt-253f3-cq852-master-1
			_, ok := nodeOSUpdateTimes[e.Locator.OldLocator()]
			if !ok {
				nodeOSUpdateTimes[e.Locator.OldLocator()] = &startedStaged{}
			}
			// for this type of event, the from/to time are identical as this is a point in time event.
			ss := nodeOSUpdateTimes[e.Locator.OldLocator()]
			ss.OSUpdateStarted = e.To
		} else if e.Message.Reason == "OSUpdateStaged" {
			// locator will be of the form: node/ci-op-j34hmfqt-253f3-cq852-master-1
			_, ok := nodeOSUpdateTimes[e.Locator.OldLocator()]
			if !ok {
				nodeOSUpdateTimes[e.Locator.OldLocator()] = &startedStaged{}
			}
			// for this type of event, the from/to time are identical as this is a point in time event.
			ss := nodeOSUpdateTimes[e.Locator.OldLocator()]
			ss.OSUpdateStaged = e.To
		}
	}

	// Iterate the data we assembled looking for any nodes missing their start event
	missingStartedMessages := []string{}
	for node, ss := range nodeOSUpdateTimes {
		if ss.OSUpdateStarted.IsZero() {
			// We've seen this occur where we've got no start time, the event is in the gather-extra/events.json but
			// not in the junit/e2e-events.json the test framework writes afterwards.
			missingStartedMessages = append(missingStartedMessages, fmt.Sprintf(
				"%s OSUpdateStaged at %s, but no OSUpdateStarted event was recorded",
				node,
				ss.OSUpdateStaged.Format(time.RFC3339)))
		}
	}

	if len(missingStartedMessages) > 0 {
		output := fmt.Sprintf("%d nodes made it to OSUpdateStaged but we did not record OSUpdateStarted:\n\n%s",
			len(missingStartedMessages), strings.Join(missingStartedMessages, "\n"))
		failure := &junitapi.JUnitTestCase{
			Name:      testName,
			SystemOut: output,
			FailureOutput: &junitapi.FailureOutput{
				Output: output,
			},
		}
		// Include a fake success so this will always be a "flake" for now.
		return []*junitapi.JUnitTestCase{failure, success}
	}

	return []*junitapi.JUnitTestCase{success}
}

// getEventsByOperator returns map keyed by operator locator with all events associated with it.
func getEventsByOperator(events monitorapi.Intervals) map[string]monitorapi.Intervals {
	eventsByClusterOperator := map[string]monitorapi.Intervals{}
	for _, event := range events {
		operatorName, ok := event.Locator.Keys[monitorapi.LocatorClusterOperatorKey]
		if !ok {
			continue
		}
		eventsByClusterOperator[operatorName] = append(eventsByClusterOperator[operatorName], event)
	}
	return eventsByClusterOperator
}
