package operators

import (
	"context"
	"fmt"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/test/e2e/framework"
	"strings"

	exutil "github.com/openshift/origin/test/extended/util"
)

var (
	operatorResourceToRequiredConditions = map[schema.GroupVersionResource][]string{
		{Group: "operator.openshift.io", Version: "v1", Resource: "authentications"}: {
			"APIServerDeploymentAvailable",
			"APIServerDeploymentDegraded",
			"APIServerDeploymentProgressing",
			"APIServerStaticResourcesDegraded",
			"APIServerWorkloadDegraded",
			"APIServicesAvailable",
			"APIServicesDegraded",
			"AuditPolicyDegraded",
			"AuthConfigDegraded",
			"AuthenticatorCertKeyProgressing",
			"CustomRouteControllerDegraded",
			"Encrypted",
			"EncryptionKeyControllerDegraded",
			"EncryptionMigrationControllerDegraded",
			"EncryptionMigrationControllerProgressing",
			"EncryptionPruneControllerDegraded",
			"EncryptionStateControllerDegraded",
			"IngressConfigDegraded",
			"IngressStateEndpointsDegraded",
			"IngressStatePodsDegraded",
			"ManagementStateDegraded",
			"OAuthAPIServerConfigObservationDegraded",
			"OAuthClientsControllerDegraded",
			"OAuthConfigDegraded",
			"OAuthConfigIngressDegraded",
			"OAuthConfigRouteDegraded",
			"OAuthConfigServiceDegraded",
			"OAuthServerConfigObservationDegraded",
			"OAuthServerDeploymentAvailable",
			"OAuthServerDeploymentDegraded",
			"OAuthServerDeploymentProgressing",
			"OAuthServerRouteEndpointAccessibleControllerAvailable",
			"OAuthServerRouteEndpointAccessibleControllerDegraded",
			"OAuthServerServiceEndpointAccessibleControllerAvailable",
			"OAuthServerServiceEndpointAccessibleControllerDegraded",
			"OAuthServerServiceEndpointsEndpointAccessibleControllerAvailable",
			"OAuthServerServiceEndpointsEndpointAccessibleControllerDegraded",
			"OAuthServerWorkloadDegraded",
			"OAuthServiceDegraded",
			"OAuthSessionSecretDegraded",
			"OAuthSystemMetadataDegraded",
			"OpenshiftAuthenticationStaticResourcesDegraded",
			"ProxyConfigControllerDegraded",
			"ReadyIngressNodesAvailable",
			"ResourceSyncControllerDegraded",
			"RevisionControllerDegraded",
			"RouterCertsDegraded",
			"RouterCertsDomainValidationControllerDegraded",
			"SystemServiceCAConfigDegraded",
			"UnsupportedConfigOverridesUpgradeable",
			"WebhookAuthenticatorCertApprover_OpenShiftAuthenticatorDegraded",
			"WebhookAuthenticatorControllerDegraded",
			"WellKnownAvailable",
			"WellKnownReadyControllerDegraded",
			"WellKnownReadyControllerProgressing",
		},
		//{Group: "operator.openshift.io", Version: "v1", Resource: "clustercsidrivers"}:           {}, // TODO special names
		{Group: "operator.openshift.io", Version: "v1", Resource: "configs"}: {
			"AWSPlatformServiceLocationControllerDegraded",
			"FeatureGateControllerDegraded",
			"FeatureGatesUpgradeable",
			"KubeCloudConfigControllerDegraded",
			"LatencySensitiveRemovalControllerDegraded",
			"MigrationPlatformStatusControllerDegraded",
			"OperatorAvailable",
			"OperatorProgressing",
			"OperatorUpgradeable",
		},
		{Group: "operator.openshift.io", Version: "v1", Resource: "consoles"}: {
			"AuthStatusHandlerDegraded",
			"AuthStatusHandlerProgressing",
			"CLIAuthStatusHandlerDegraded",
			"CLIAuthStatusHandlerProgressing",
			"CLIOIDCClientStatusDegraded",
			"CLIOIDCClientStatusProgressing",
			"ConfigMapSyncDegraded",
			"ConfigMapSyncProgressing",
			"ConsoleConfigDegraded",
			"ConsoleCustomRouteSyncDegraded",
			"ConsoleCustomRouteSyncProgressing",
			"ConsoleCustomRouteSyncUpgradeable",
			"ConsoleDefaultRouteSyncDegraded",
			"ConsoleDefaultRouteSyncProgressing",
			"ConsoleDefaultRouteSyncUpgradeable",
			"ConsoleNotificationSyncDegraded",
			"ConsoleNotificationSyncProgressing",
			"ConsolePublicConfigMapDegraded",
			"CustomLogoSyncDegraded",
			"CustomLogoSyncProgressing",
			"DeploymentAvailable",
			"DeploymentSyncDegraded",
			"DeploymentSyncProgressing",
			"DownloadsCustomRouteSyncDegraded",
			"DownloadsCustomRouteSyncProgressing",
			"DownloadsCustomRouteSyncUpgradeable",
			"DownloadsDefaultRouteSyncDegraded",
			"DownloadsDefaultRouteSyncProgressing",
			"DownloadsDefaultRouteSyncUpgradeable",
			"DownloadsDeploymentSyncDegraded",
			"DownloadsDeploymentSyncProgressing",
			"ManagementStateDegraded",
			"OAuthClientsControllerDegraded",
			"OAuthClientSecretGetDegraded",
			"OAuthClientSecretGetProgressing",
			"OAuthClientSecretSyncDegraded",
			"OAuthClientSecretSyncProgressing",
			"OAuthClientSyncDegraded",
			"OAuthClientSyncProgressing",
			"OAuthServingCertValidationDegraded",
			"OAuthServingCertValidationProgressing",
			"OCDownloadsSyncDegraded",
			"ODODownloadsSyncDegraded",
			"OIDCClientConfigDegraded",
			"OIDCClientConfigProgressing",
			"PDBSyncDegraded",
			"PDBSyncProgressing",
			"RedirectServiceSyncDegraded",
			"RedirectServiceSyncProgressing",
			"ResourceSyncControllerDegraded",
			"RouteHealthAvailable",
			"RouteHealthDegraded",
			"RouteHealthProgressing",
			"ServiceCASyncDegraded",
			"ServiceCASyncProgressing",
			"ServiceSyncDegraded",
			"ServiceSyncProgressing",
			"SyncLoopRefreshDegraded",
			"SyncLoopRefreshProgressing",
			"TrustedCASyncDegraded",
			"TrustedCASyncProgressing",
			"UnsupportedConfigOverridesUpgradeable",
		},
		{Group: "operator.openshift.io", Version: "v1", Resource: "csisnapshotcontrollers"}: {
			"CSISnapshotControllerAvailable",
			"CSISnapshotControllerDegraded",
			"CSISnapshotControllerProgressing",
			"CSISnapshotControllerUpgradeable",
			"CSISnapshotGuestStaticResourceControllerDegraded",
			"CSISnapshotStaticResourceControllerDegraded",
			"CSISnapshotWebhookControllerAvailable",
			"CSISnapshotWebhookControllerDegraded",
			"CSISnapshotWebhookControllerProgressing",
			"ManagementStateDegraded",
			"WebhookControllerDegraded",
		},
		//{Group: "operator.openshift.io", Version: "v1", Resource: "dnses"}:                       {}, // different name
		{Group: "operator.openshift.io", Version: "v1", Resource: "etcds"}: {
			"BackingResourceControllerDegraded",
			"BootstrapTeardownDegraded",
			"ClusterMemberControllerDegraded",
			"ClusterMemberRemovalControllerDegraded",
			"ConfigObservationDegraded",
			//"DefragControllerDegraded", // disabled in single node
			"DefragControllerDisabled",
			"EnvVarControllerDegraded",
			"EtcdBootstrapMemberRemoved",
			"EtcdCertSignerControllerDegraded",
			"EtcdEndpointsDegraded",
			"EtcdMembersAvailable",
			"EtcdMembersControllerDegraded",
			"EtcdMembersDegraded",
			"EtcdMembersProgressing",
			"EtcdRunningInCluster",
			"EtcdStaticResourcesDegraded",
			"FSyncControllerDegraded",
			"GuardControllerDegraded",
			"InstallerControllerDegraded",
			"InstallerPodContainerWaitingDegraded",
			"InstallerPodNetworkingDegraded",
			"InstallerPodPendingDegraded",
			"MachineDeletionHooksControllerDegraded",
			"MissingStaticPodControllerDegraded",
			"NodeControllerDegraded",
			"NodeInstallerDegraded",
			"NodeInstallerProgressing",
			"ResourceSyncControllerDegraded",
			"RevisionControllerDegraded",
			"ScriptControllerDegraded",
			"StaticPodsAvailable",
			"StaticPodsDegraded",
			"TargetConfigControllerDegraded",
			"UnsupportedConfigOverridesUpgradeable",
		},
		//{Group: "operator.openshift.io", Version: "v1", Resource: "insightsoperators"}:           {}, // didn't appear to have any???
		{Group: "operator.openshift.io", Version: "v1", Resource: "kubeapiservers"}: {
			"AuditPolicyDegraded",
			"BackingResourceControllerDegraded",
			"CertRotation_AggregatorProxyClientCert_Degraded",
			"CertRotation_CheckEndpointsClient_Degraded",
			"CertRotation_ControlPlaneNodeAdminClient_Degraded",
			"CertRotation_ExternalLoadBalancerServing_Degraded",
			"CertRotation_InternalLoadBalancerServing_Degraded",
			"CertRotation_KubeAPIServerToKubeletClientCert_Degraded",
			"CertRotation_KubeControllerManagerClient_Degraded",
			"CertRotation_KubeSchedulerClient_Degraded",
			"CertRotation_LocalhostRecoveryServing_Degraded",
			"CertRotation_LocalhostServing_Degraded",
			"CertRotation_NodeSystemAdminClient_Degraded",
			"CertRotation_ServiceNetworkServing_Degraded",
			"CertRotationTimeUpgradeable",
			"ConfigObservationDegraded",
			"CRDConversionWebhookConfigurationError",
			"Encrypted",
			"EncryptionKeyControllerDegraded",
			"EncryptionMigrationControllerDegraded",
			"EncryptionMigrationControllerProgressing",
			"EncryptionPruneControllerDegraded",
			"EncryptionStateControllerDegraded",
			"GuardControllerDegraded",
			"InstallerControllerDegraded",
			"InstallerPodContainerWaitingDegraded",
			"InstallerPodNetworkingDegraded",
			"InstallerPodPendingDegraded",
			"KubeAPIServerStaticResourcesDegraded",
			"KubeletMinorVersionUpgradeable",
			"MissingStaticPodControllerDegraded",
			"MutatingAdmissionWebhookConfigurationError",
			"NodeControllerDegraded",
			"NodeInstallerDegraded",
			"NodeInstallerProgressing",
			"NodeKubeconfigControllerDegraded",
			"PodSecurityCustomerEvaluationConditionsDetected",
			"PodSecurityDisabledSyncerEvaluationConditionsDetected",
			"PodSecurityOpenshiftEvaluationConditionsDetected",
			"PodSecurityRunLevelZeroEvaluationConditionsDetected",
			"ResourceSyncControllerDegraded",
			"RevisionControllerDegraded",
			"StartupMonitorPodContainerExcessiveRestartsDegraded",
			"StartupMonitorPodDegraded",
			"StaticPodFallbackRevisionDegraded",
			"StaticPodsAvailable",
			"StaticPodsDegraded",
			"TargetConfigControllerDegraded",
			"UnsupportedConfigOverridesUpgradeable",
			"ValidatingAdmissionWebhookConfigurationError",
			"VirtualResourceAdmissionError",
			"WorkerLatencyProfileComplete",
			"WorkerLatencyProfileDegraded",
			"WorkerLatencyProfileProgressing",
		},
		{Group: "operator.openshift.io", Version: "v1", Resource: "kubecontrollermanagers"}: {
			"BackingResourceControllerDegraded",
			"CertRotation_CSRSigningCert_Degraded",
			"ConfigObservationDegraded",
			"GarbageCollectorDegraded",
			"GuardControllerDegraded",
			"InstallerControllerDegraded",
			"InstallerPodContainerWaitingDegraded",
			"InstallerPodNetworkingDegraded",
			"InstallerPodPendingDegraded",
			"KubeControllerManagerStaticResourcesDegraded",
			"MissingStaticPodControllerDegraded",
			"NodeControllerDegraded",
			"NodeInstallerDegraded",
			"NodeInstallerProgressing",
			"ResourceSyncControllerDegraded",
			"RevisionControllerDegraded",
			"SATokenSignerDegraded",
			"StaticPodsAvailable",
			"StaticPodsDegraded",
			"TargetConfigControllerDegraded",
			"UnsupportedConfigOverridesUpgradeable",
			"Upgradeable",
			"WorkerLatencyProfileComplete",
			"WorkerLatencyProfileDegraded",
			"WorkerLatencyProfileProgressing",
		},
		{Group: "operator.openshift.io", Version: "v1", Resource: "kubeschedulers"}: {
			"BackingResourceControllerDegraded",
			"ConfigObservationDegraded",
			"GuardControllerDegraded",
			"InstallerControllerDegraded",
			"InstallerPodContainerWaitingDegraded",
			"InstallerPodNetworkingDegraded",
			"InstallerPodPendingDegraded",
			"KubeControllerManagerStaticResourcesDegraded",
			"MissingStaticPodControllerDegraded",
			"NodeControllerDegraded",
			"NodeInstallerDegraded",
			"NodeInstallerProgressing",
			"ResourceSyncControllerDegraded",
			"RevisionControllerDegraded",
			"StaticPodsAvailable",
			"StaticPodsDegraded",
			"TargetConfigControllerDegraded",
			"UnsupportedConfigOverridesUpgradeable",
		},
		{Group: "operator.openshift.io", Version: "v1", Resource: "kubestorageversionmigrators"}: {
			"DefaultUpgradeable",
			"KubeStorageVersionMigratorAvailable",
			"KubeStorageVersionMigratorDegraded",
			"KubeStorageVersionMigratorProgressing",
			"KubeStorageVersionMigratorStaticResourcesDegraded",
		},
		{Group: "operator.openshift.io", Version: "v1", Resource: "networks"}: {
			"Available",
			"Degraded",
			"ManagementStateDegraded",
			"Progressing",
			"Upgradeable",
		},
		{Group: "operator.openshift.io", Version: "v1", Resource: "openshiftapiservers"}: {
			"APIServerDeploymentAvailable",
			"APIServerDeploymentDegraded",
			"APIServerDeploymentProgressing",
			"APIServerStaticResourcesDegraded",
			"APIServerWorkloadDegraded",
			"APIServicesAvailable",
			"APIServicesDegraded",
			"AuditPolicyDegraded",
			"ConfigObservationDegraded",
			"Encrypted",
			"EncryptionKeyControllerDegraded",
			"EncryptionMigrationControllerDegraded",
			"EncryptionMigrationControllerProgressing",
			"EncryptionPruneControllerDegraded",
			"EncryptionStateControllerDegraded",
			"OperatorConfigProgressing",
			"ResourceSyncControllerDegraded",
			"RevisionControllerDegraded",
			"UnsupportedConfigOverridesUpgradeable",
		},
		{Group: "operator.openshift.io", Version: "v1", Resource: "openshiftcontrollermanagers"}: {
			"Available",
			"ConfigObservationDegraded",
			"OpenshiftControllerManagerStaticResourcesDegraded",
			"Progressing",
			"ResourceSyncControllerDegraded",
			"Upgradeable",
			"WorkloadDegraded",
		},
		{Group: "operator.openshift.io", Version: "v1", Resource: "servicecas"}: {
			"Available",
			"Degraded",
			"Progressing",
			"ResourceSyncControllerDegraded",
			"Upgradeable",
		},
		{Group: "operator.openshift.io", Version: "v1", Resource: "storages"}: { // much isn't listed here and varies by platform
			"ConfigObservationDegraded",
			"CSIDriverStarterDegraded",
			"DefaultStorageClassControllerAvailable",
			"DefaultStorageClassControllerDegraded",
			"DefaultStorageClassControllerProgressing",
			"ManagementStateDegraded",
			"VSphereProblemDetectorStarterDegraded",
		},
	}
)

var _ = g.Describe("[sig-arch][Early] Operators", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("management-plane-operators")

	g.Describe("low level operators", func() {
		g.It("should have at least the conditions we had in 4.17", func() {
			ctx := context.TODO()
			if ok, _ := exutil.IsMicroShiftCluster(oc.AdminKubeClient()); ok {
				g.Skip("microshift does not have operators.")
			}
			if ok, _ := exutil.IsHypershift(ctx, oc.AdminConfigClient()); ok {
				g.Skip("hypershift does not have operators.")
			}

			// this test is ensuring that we don't accidentally lose a condition
			failures := []string{}
			for gvr, requiredConditions := range operatorResourceToRequiredConditions {
				uncastOperatorResource, err := oc.AdminDynamicClient().Resource(gvr).Get(ctx, "cluster", metav1.GetOptions{})
				if err != nil {
					err = fmt.Errorf("failed to read: %#v %q: %w", gvr, "cluster", err)
					o.Expect(err).NotTo(o.HaveOccurred())
				}
				operatorStatus, err := getOperatorStatusFromUnstructured(uncastOperatorResource.Object)
				o.Expect(err).NotTo(o.HaveOccurred())

				for _, requiredCondition := range requiredConditions {
					condition := v1helpers.FindOperatorCondition(operatorStatus.Conditions, requiredCondition)
					if condition == nil {
						failures = append(failures, fmt.Sprintf("resource=%v is missing condition %q that was present in 4.17.  If this is intentional, update the list in this test.", gvr, requiredCondition))
					}
				}
			}
			if len(failures) > 0 {
				framework.Logf("%v", strings.Join(failures, "\n"))
			}
			o.Expect(failures).To(o.Equal([]string{}))
		})
	})
})

// TODO collapse with library-go
func getOperatorStatusFromUnstructured(obj map[string]interface{}) (*operatorv1.OperatorStatus, error) {
	uncastStatus, exists, err := unstructured.NestedMap(obj, "status")
	if !exists {
		return &operatorv1.OperatorStatus{}, nil
	}
	if err != nil {
		return nil, err
	}

	ret := &operatorv1.OperatorStatus{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(uncastStatus, ret); err != nil {
		return nil, err
	}
	return ret, nil
}
