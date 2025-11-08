package generated

import (
	"fmt"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
)

var Annotations = map[string]string{
	"[Conformance][Suite:openshift/kube-apiserver/rollout][Jira:\"kube-apiserver\"][sig-kube-apiserver] kube-apiserver should roll out new revisions without disruption [apigroup:config.openshift.io][apigroup:operator.openshift.io]": "",

	"[Conformance][sig-api-machinery][Feature:APIServer] kube-apiserver should be accessible via api-ext endpoint": " [Suite:openshift/conformance/parallel/minimal]",

	"[Conformance][sig-api-machinery][Feature:APIServer] kube-apiserver should be accessible via api-int endpoint": " [Suite:openshift/conformance/parallel/minimal]",

	"[Conformance][sig-api-machinery][Feature:APIServer] kube-apiserver should be accessible via service network endpoint": " [Suite:openshift/conformance/parallel/minimal]",

	"[Conformance][sig-api-machinery][Feature:APIServer] local kubeconfig \"check-endpoints.kubeconfig\" should be present in all kube-apiserver containers": " [Suite:openshift/conformance/parallel/minimal]",

	"[Conformance][sig-api-machinery][Feature:APIServer] local kubeconfig \"control-plane-node.kubeconfig\" should be present in all kube-apiserver containers": " [Suite:openshift/conformance/parallel/minimal]",

	"[Conformance][sig-api-machinery][Feature:APIServer] local kubeconfig \"lb-ext.kubeconfig\" should be present on all masters and work": " [Suite:openshift/conformance/parallel/minimal]",

	"[Conformance][sig-api-machinery][Feature:APIServer] local kubeconfig \"lb-int.kubeconfig\" should be present on all masters and work": " [Suite:openshift/conformance/parallel/minimal]",

	"[Conformance][sig-api-machinery][Feature:APIServer] local kubeconfig \"localhost-recovery.kubeconfig\" should be present on all masters and work": " [Suite:openshift/conformance/parallel/minimal]",

	"[Conformance][sig-api-machinery][Feature:APIServer] local kubeconfig \"localhost.kubeconfig\" should be present on all masters and work": " [Suite:openshift/conformance/parallel/minimal]",

	"[Conformance][sig-sno][Serial] Cluster should allow a fast rollout of kube-apiserver with no pods restarts during API disruption [apigroup:config.openshift.io][apigroup:operator.openshift.io]": " [Suite:openshift/conformance/serial/minimal]",

	"[Serial] [sig-auth][Feature:OAuthServer] [RequestHeaders] [IdP] test RequestHeaders IdP [apigroup:config.openshift.io][apigroup:user.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[Serial][sig-cli] oc adm upgrade recommend When the update service has conditional recommendations runs successfully when listing all updates": " [Suite:openshift/conformance/serial]",

	"[Serial][sig-cli] oc adm upgrade recommend When the update service has conditional recommendations runs successfully with conditional recommendations to the --version target": " [Suite:openshift/conformance/serial]",

	"[Serial][sig-cli] oc adm upgrade recommend When the update service has no recommendations runs successfully": " [Suite:openshift/conformance/serial]",

	"[Serial][sig-cli] oc adm upgrade recommend runs successfully with an empty channel": " [Suite:openshift/conformance/serial]",

	"[Serial][sig-cli] oc adm upgrade recommend runs successfully, even without upstream OpenShift Update Service customization": " [Suite:openshift/conformance/serial]",

	"[Suite:openshift/conformance/serial][Serial][sig-node] Node sizing should have NODE_SIZING_ENABLED=false in /etc/node-sizing-enabled.env": "",

	"[Suite:openshift/conformance/serial][Serial][sig-node] Node sizing should have NODE_SIZING_ENABLED=true when KubeletConfig with autoSizingReserved=true is applied": "",

	"[Suite:openshift/machine-config-operator/disruptive][Suite:openshift/conformance/serial][sig-mco][OCPFeatureGate:ManagedBootImagesAWS][Serial] Should degrade on a MachineSet with an OwnerReference [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][Suite:openshift/conformance/serial][sig-mco][OCPFeatureGate:ManagedBootImagesAWS][Serial] Should not update boot images on any MachineSet when not configured [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][Suite:openshift/conformance/serial][sig-mco][OCPFeatureGate:ManagedBootImagesAWS][Serial] Should stamp coreos-bootimages configmap with current MCO hash and release version [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][Suite:openshift/conformance/serial][sig-mco][OCPFeatureGate:ManagedBootImagesAWS][Serial] Should update boot images on all MachineSets when configured [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][Suite:openshift/conformance/serial][sig-mco][OCPFeatureGate:ManagedBootImagesAWS][Serial] Should update boot images only on MachineSets that are opted in [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][Suite:openshift/conformance/serial][sig-mco][OCPFeatureGate:ManagedBootImages][Serial] Should degrade on a MachineSet with an OwnerReference [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][Suite:openshift/conformance/serial][sig-mco][OCPFeatureGate:ManagedBootImages][Serial] Should not update boot images on any MachineSet when not configured [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][Suite:openshift/conformance/serial][sig-mco][OCPFeatureGate:ManagedBootImages][Serial] Should stamp coreos-bootimages configmap with current MCO hash and release version [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][Suite:openshift/conformance/serial][sig-mco][OCPFeatureGate:ManagedBootImages][Serial] Should update boot images on all MachineSets when configured [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][Suite:openshift/conformance/serial][sig-mco][OCPFeatureGate:ManagedBootImages][Serial] Should update boot images only on MachineSets that are opted in [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][Suite:openshift/conformance/serial][sig-mco][OCPFeatureGate:PinnedImages][OCPFeatureGate:MachineConfigNodes][Serial] All Nodes in a Custom Pool should have the PinnedImages in PIS [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][Suite:openshift/conformance/serial][sig-mco][OCPFeatureGate:PinnedImages][OCPFeatureGate:MachineConfigNodes][Serial] All Nodes in a custom Pool should have the PinnedImages even after Garbage Collection [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][Suite:openshift/conformance/serial][sig-mco][OCPFeatureGate:PinnedImages][OCPFeatureGate:MachineConfigNodes][Serial] All Nodes in a standard Pool should have the PinnedImages PIS [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][Suite:openshift/conformance/serial][sig-mco][OCPFeatureGate:PinnedImages][OCPFeatureGate:MachineConfigNodes][Serial] Invalid PIS leads to degraded MCN in a custom Pool [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][Suite:openshift/conformance/serial][sig-mco][OCPFeatureGate:PinnedImages][OCPFeatureGate:MachineConfigNodes][Serial] Invalid PIS leads to degraded MCN in a standard Pool [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][sig-mco][OCPFeatureGate:MachineConfigNodes] [Serial][Slow]Should properly create and remove MCN on node creation and deletion [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][sig-mco][OCPFeatureGate:MachineConfigNodes] [Serial][Slow]Should properly report MCN conditions on node degrade [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][sig-mco][OCPFeatureGate:MachineConfigNodes] [Suite:openshift/conformance/parallel]Should have MCN properties matching associated node properties for nodes in default MCPs [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][sig-mco][OCPFeatureGate:MachineConfigNodes] [Suite:openshift/conformance/parallel]Should properly block MCN updates by impersonation of the MCD SA [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][sig-mco][OCPFeatureGate:MachineConfigNodes] [Suite:openshift/conformance/parallel]Should properly block MCN updates from a MCD that is not the associated one [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][sig-mco][OCPFeatureGate:MachineConfigNodes] [Suite:openshift/conformance/serial][Serial]Should have MCN properties matching associated node properties for nodes in custom MCPs [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][sig-mco][OCPFeatureGate:MachineConfigNodes] [Suite:openshift/conformance/serial][Serial]Should properly transition through MCN conditions on rebootless node update [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][sig-mco][OCPFeatureGate:MachineConfigNodes] [Suite:openshift/conformance/serial][Serial]Should properly update the MCN from the associated MCD [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][sig-mco][OCPFeatureGate:ManagedBootImagesvSphere][Serial] Should degrade on a MachineSet with an OwnerReference [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][sig-mco][OCPFeatureGate:ManagedBootImagesvSphere][Serial] Should not update boot images on any MachineSet when not configured [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][sig-mco][OCPFeatureGate:ManagedBootImagesvSphere][Serial] Should stamp coreos-bootimages configmap with current MCO hash and release version [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][sig-mco][OCPFeatureGate:ManagedBootImagesvSphere][Serial] Should update boot images on all MachineSets when configured [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][sig-mco][OCPFeatureGate:ManagedBootImagesvSphere][Serial] Should update boot images only on MachineSets that are opted in [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/machine-config-operator/disruptive][sig-mco][OCPFeatureGate:ManagedBootImagesvSphere][Serial] Should upload the latest bootimage to the appropriate vCentre [apigroup:machineconfiguration.openshift.io]": "",

	"[Suite:openshift/usernamespace] [sig-node] [FeatureGate:ProcMountType] [FeatureGate:UserNamespacesSupport] nested container should pass podman localsystem test in baseline mode": "",

	"[sig-api-machinery] API data in etcd should be stored at the correct location and version for all resources [Serial]": " [Suite:openshift/conformance/serial]",

	"[sig-api-machinery] API health endpoints should contain the required checks for the oauth-apiserver APIs": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery] API health endpoints should contain the required checks for the openshift-apiserver APIs": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery] APIServer CR fields validation additionalCORSAllowedOrigins [apigroup:config.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery] JSON Patch [apigroup:operator.openshift.io] should delete an entry from an array with a test precondition provided": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery] JSON Patch [apigroup:operator.openshift.io] should delete an entry from an array with multiple field owners": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery] JSON Patch [apigroup:operator.openshift.io] should delete multiple entries from an array when multiple test precondition provided": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery] JSON Patch [apigroup:operator.openshift.io] should error when the test precondition provided doesn't match": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:APIServer] TestTLSDefaults": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:APIServer] anonymous browsers should get a 403 from /": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:APIServer] authenticated browser should get a 200 from /": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:APIServer] should serve openapi v3 discovery": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:APIServer] should serve openapi v3": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:APIServer][Late] API LBs follow /readyz of kube-apiserver and don't send request early": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:APIServer][Late] API LBs follow /readyz of kube-apiserver and stop sending requests": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:APIServer][Late] kube-apiserver terminates within graceful termination period": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:APIServer][Late] kubelet terminates kube-apiserver gracefully extended": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:APIServer][Late] kubelet terminates kube-apiserver gracefully": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:Audit] Basic audit should audit API calls": " [Disabled:SpecialConfig]",

	"[sig-api-machinery][Feature:ClusterResourceQuota] Cluster resource quota should control resource limits across namespaces [apigroup:quota.openshift.io][apigroup:image.openshift.io][apigroup:monitoring.coreos.com][apigroup:template.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ResourceQuota] Object count check the quota after import-image with --all option [Skipped:Disconnected]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ResourceQuota] Object count should properly count the number of imagestreams resources [apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ResourceQuota] Object count should properly count the number of persistentvolumeclaims resources [Serial]": " [Suite:openshift/conformance/serial]",

	"[sig-api-machinery][Feature:ResourceQuota] Object count when exceed openshift.io/image-tags will ban to create new image references in the project [Skipped:Disconnected]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply should work for apps.openshift.io/v1, Resource=deploymentconfigs [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply should work for build.openshift.io/v1, Resource=buildconfigs [apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply should work for build.openshift.io/v1, Resource=builds [apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply should work for image.openshift.io/v1, Resource=images [apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply should work for image.openshift.io/v1, Resource=imagestreams [apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply should work for oauth.openshift.io/v1, Resource=oauthaccesstokens [apigroup:oauth.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply should work for oauth.openshift.io/v1, Resource=oauthauthorizetokens [apigroup:oauth.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply should work for oauth.openshift.io/v1, Resource=oauthclientauthorizations [apigroup:oauth.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply should work for oauth.openshift.io/v1, Resource=oauthclients [apigroup:oauth.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply should work for route.openshift.io/v1, Resource=routes [apigroup:route.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply should work for security.openshift.io/v1, Resource=rangeallocations [apigroup:security.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply should work for template.openshift.io/v1, Resource=brokertemplateinstances [apigroup:template.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply should work for template.openshift.io/v1, Resource=templateinstances [apigroup:template.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply should work for template.openshift.io/v1, Resource=templates [apigroup:template.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply should work for user.openshift.io/v1, Resource=groups [apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply should work for user.openshift.io/v1, Resource=identities [apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-api-machinery][Feature:ServerSideApply] Server-Side Apply should work for user.openshift.io/v1, Resource=users [apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-apimachinery] server-side-apply should function properly should clear fields when they are no longer being applied in FeatureGates [apigroup:config.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-apimachinery] server-side-apply should function properly should clear fields when they are no longer being applied in built-in APIs": " [Suite:openshift/conformance/parallel]",

	"[sig-apimachinery] server-side-apply should function properly should clear fields when they are no longer being applied on CRDs": " [Suite:openshift/conformance/parallel]",

	"[sig-apps] poddisruptionbudgets with unhealthyPodEvictionPolicy should evict according to the AlwaysAllow policy": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps] poddisruptionbudgets with unhealthyPodEvictionPolicy should evict according to the IfHealthyBudget policy": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs adoption will orphan all RCs and adopt them back when recreated [apigroup:apps.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs generation should deploy based on a status version bump [apigroup:apps.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs ignores deployer and lets the config with a NewReplicationControllerCreated reason should let the deployment config with a NewReplicationControllerCreated reason [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs initially should not deploy if pods never transition to ready [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs keep the deployer pod invariant valid should deal with cancellation after deployer pod succeeded [apigroup:apps.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs keep the deployer pod invariant valid should deal with cancellation of running deployment [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs keep the deployer pod invariant valid should deal with config change in case the deployment is still running [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs paused should disable actions on deployments [apigroup:apps.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs reaper [Slow] should delete all failed deployer pods and hook pods [apigroup:apps.openshift.io]": "",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs rolled back should rollback to an older deployment [apigroup:apps.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs should adhere to Three Laws of Controllers [apigroup:apps.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs should respect image stream tag reference policy resolve the image pull spec [apigroup:apps.openshift.io][apigroup:image.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs viewing rollout history should print the rollout history [apigroup:apps.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs when changing image change trigger should successfully trigger from an updated image [apigroup:apps.openshift.io][apigroup:image.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs when run iteratively should immediately start a new deployment [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs when run iteratively should only deploy the last deployment [apigroup:apps.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs when tagging images should successfully tag the deployed image [apigroup:apps.openshift.io][apigroup:authorization.openshift.io][apigroup:image.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with custom deployments should run the custom deployment steps [apigroup:apps.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with enhanced status should include various info in status [apigroup:apps.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with env in params referencing the configmap should expand the config map key to a value [apigroup:apps.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with failing hook should get all logs from retried hooks [apigroup:apps.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with minimum ready seconds set should not transition the deployment to Complete before satisfied [apigroup:apps.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with multiple image change triggers should run a successful deployment with a trigger used by different containers [apigroup:apps.openshift.io][apigroup:image.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with multiple image change triggers should run a successful deployment with multiple triggers [apigroup:apps.openshift.io][apigroup:image.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with revision history limits should never persist more old deployments than acceptable after being observed by the controller [apigroup:apps.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs with test deployments should run a deployment to completion and then scale to zero [apigroup:apps.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:DeploymentConfig] deploymentconfigs won't deploy RC with unresolved images when patched with empty image [apigroup:apps.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:OpenShiftControllerManager] TestDeployScale [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:OpenShiftControllerManager] TestDeploymentConfigDefaults [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:OpenShiftControllerManager] TestTriggers_MultipleICTs [apigroup:apps.openshift.io][apigroup:images.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:OpenShiftControllerManager] TestTriggers_configChange [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:OpenShiftControllerManager] TestTriggers_imageChange [apigroup:apps.openshift.io][apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:OpenShiftControllerManager] TestTriggers_imageChange_nonAutomatic [apigroup:image.openshift.io][apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-apps][Feature:OpenShiftControllerManager] TestTriggers_manual [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-apps][apigroup:apps.openshift.io][OCPFeatureGate:HighlyAvailableArbiter] Deployments on HighlyAvailableArbiterMode topology should be created on arbiter nodes when arbiter node is selected": " [Suite:openshift/conformance/parallel]",

	"[sig-apps][apigroup:apps.openshift.io][OCPFeatureGate:HighlyAvailableArbiter] Deployments on HighlyAvailableArbiterMode topology should be created on master nodes when no node selected": " [Suite:openshift/conformance/parallel]",

	"[sig-apps][apigroup:apps.openshift.io][OCPFeatureGate:HighlyAvailableArbiter] Evaluate DaemonSet placement in HighlyAvailableArbiterMode topology should not create a DaemonSet on the Arbiter node": " [Suite:openshift/conformance/parallel]",

	"[sig-arch] Cluster topology single node tests Verify that OpenShift components deploy one replica in SingleReplica topology mode": " [Suite:openshift/conformance/parallel]",

	"[sig-arch] ClusterOperators [apigroup:config.openshift.io] should define at least one namespace in their lists of related objects": " [Suite:openshift/conformance/parallel]",

	"[sig-arch] ClusterOperators [apigroup:config.openshift.io] should define at least one related object that is not a namespace": " [Suite:openshift/conformance/parallel]",

	"[sig-arch] ClusterOperators [apigroup:config.openshift.io] should define valid related objects": " [Suite:openshift/conformance/parallel]",

	"[sig-arch] Managed cluster should ensure control plane operators do not make themselves unevictable": " [Suite:openshift/conformance/parallel]",

	"[sig-arch] Managed cluster should ensure control plane pods do not run in best-effort QoS": " [Suite:openshift/conformance/parallel]",

	"[sig-arch] Managed cluster should ensure platform components have system-* priority class associated": " [Suite:openshift/conformance/parallel]",

	"[sig-arch] Managed cluster should ensure pods use downstream images from our release image with proper ImagePullPolicy [apigroup:config.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-arch] Managed cluster should expose cluster services outside the cluster [apigroup:route.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-arch] Managed cluster should have operators on the cluster version [apigroup:config.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-arch] Managed cluster should only include cluster daemonsets that have maxUnavailable or maxSurge update of 10 percent or maxUnavailable of 33 percent": " [Suite:openshift/conformance/parallel]",

	"[sig-arch] Managed cluster should recover when operator-owned objects are deleted [Disruptive][apigroup:config.openshift.io]": " [Serial]",

	"[sig-arch] Managed cluster should set requests but not limits": " [Suite:openshift/conformance/parallel]",

	"[sig-arch] [Conformance] FIPS TestFIPS": " [Suite:openshift/conformance/parallel/minimal]",

	"[sig-arch] [Conformance] sysctl pod should not start for sysctl not on whitelist kernel.msgmax": " [Suite:openshift/conformance/parallel/minimal]",

	"[sig-arch] [Conformance] sysctl pod should not start for sysctl not on whitelist net.ipv4.ip_dynaddr": " [Suite:openshift/conformance/parallel/minimal]",

	"[sig-arch] [Conformance] sysctl whitelists kernel.shm_rmid_forced": " [Suite:openshift/conformance/parallel/minimal]",

	"[sig-arch] [Conformance] sysctl whitelists net.ipv4.ip_local_port_range": " [Suite:openshift/conformance/parallel/minimal]",

	"[sig-arch] [Conformance] sysctl whitelists net.ipv4.ip_unprivileged_port_start": " [Suite:openshift/conformance/parallel/minimal]",

	"[sig-arch] [Conformance] sysctl whitelists net.ipv4.ping_group_range": " [Suite:openshift/conformance/parallel/minimal]",

	"[sig-arch] [Conformance] sysctl whitelists net.ipv4.tcp_syncookies": " [Suite:openshift/conformance/parallel/minimal]",

	"[sig-arch] ocp payload should be based on existing source OLM version should contain the source commit id": " [Skipped:NoOptionalCapabilities] [Suite:openshift/conformance/parallel]",

	"[sig-arch][Early] APIs for openshift.io must have stable versions": " [Suite:openshift/conformance/parallel]",

	"[sig-arch][Early] CRDs for openshift.io should have subresource.status": " [Suite:openshift/conformance/parallel]",

	"[sig-arch][Early] Managed cluster should [apigroup:config.openshift.io] start all core operators": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-arch][Early] Operators low level operators should have at least the conditions we had in 4.17": " [Suite:openshift/conformance/parallel]",

	"[sig-arch][Feature:ClusterUpgrade] All nodes should be in ready state [Early][Suite:upgrade]": "",

	"[sig-arch][Feature:ClusterUpgrade] Cluster should be upgradeable after finishing upgrade [Late][Suite:upgrade]": "",

	"[sig-arch][Feature:ClusterUpgrade] Cluster should be upgradeable before beginning upgrade [Early][Suite:upgrade]": "",

	"[sig-arch][Feature:ClusterUpgrade] Cluster should remain functional during upgrade [Disruptive]": " [Serial]",

	"[sig-arch][Late] clients should not use APIs that are removed in upcoming releases [apigroup:apiserver.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-arch][Late][Jira:\"kube-apiserver\"] [OCPFeatureGate:ShortCertRotation] all certificates should expire in no more than 8 hours": " [Suite:openshift/conformance/parallel]",

	"[sig-arch][Late][Jira:\"kube-apiserver\"] all registered tls artifacts must have no metadata violation regressions": " [Suite:openshift/conformance/parallel]",

	"[sig-arch][Late][Jira:\"kube-apiserver\"] all tls artifacts must be registered": " [Suite:openshift/conformance/parallel]",

	"[sig-arch][Late][Jira:\"kube-apiserver\"] collect certificate data": " [Suite:openshift/conformance/parallel]",

	"[sig-arch][OCPFeatureGate:Example] should only run FeatureGated test when enabled": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:Authentication]  TestFrontProxy should succeed": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:BootstrapUser] The bootstrap user should successfully login with password decoded from kubeadmin secret [Disruptive]": " [Serial]",

	"[sig-auth][Feature:ControlPlaneSecurity] should have privileged securityContext for control plane init and main containers": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:HTPasswdAuth] HTPasswd IDP should successfully configure htpasswd and be responsive [apigroup:user.openshift.io][apigroup:route.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:LDAP] LDAP IDP should authenticate against an ldap server [apigroup:user.openshift.io][apigroup:route.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:LDAP] LDAP should start an OpenLDAP test server [apigroup:user.openshift.io][apigroup:security.openshift.io][apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:LDAP][Serial] ldap group sync can sync groups from ldap [apigroup:user.openshift.io][apigroup:authorization.openshift.io][apigroup:security.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[sig-auth][Feature:OAuthServer] ClientSecretWithPlus should create oauthclient [apigroup:oauth.openshift.io][apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] OAuth Authenticator accepts sha256 access tokens [apigroup:user.openshift.io][apigroup:oauth.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] OAuth server [apigroup:auth.openshift.io] should use http1.1 only to prevent http2 connection reuse": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] OAuth server has the correct token and certificate fallback semantics [apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] [Headers][apigroup:route.openshift.io][apigroup:config.openshift.io][apigroup:oauth.openshift.io] expected headers returned from the authorize URL": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] [Headers][apigroup:route.openshift.io][apigroup:config.openshift.io][apigroup:oauth.openshift.io] expected headers returned from the grant URL": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] [Headers][apigroup:route.openshift.io][apigroup:config.openshift.io][apigroup:oauth.openshift.io] expected headers returned from the login URL for the allow all IDP": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] [Headers][apigroup:route.openshift.io][apigroup:config.openshift.io][apigroup:oauth.openshift.io] expected headers returned from the login URL for the bootstrap IDP": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] [Headers][apigroup:route.openshift.io][apigroup:config.openshift.io][apigroup:oauth.openshift.io] expected headers returned from the login URL for when there is only one IDP": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] [Headers][apigroup:route.openshift.io][apigroup:config.openshift.io][apigroup:oauth.openshift.io] expected headers returned from the logout URL": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] [Headers][apigroup:route.openshift.io][apigroup:config.openshift.io][apigroup:oauth.openshift.io] expected headers returned from the root URL": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] [Headers][apigroup:route.openshift.io][apigroup:config.openshift.io][apigroup:oauth.openshift.io] expected headers returned from the token URL": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] [Headers][apigroup:route.openshift.io][apigroup:config.openshift.io][apigroup:oauth.openshift.io] expected headers returned from the token request URL": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] [Token Expiration] Using a OAuth client with a non-default token max age [apigroup:oauth.openshift.io] to generate tokens that do not expire works as expected when using a code authorization flow [apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] [Token Expiration] Using a OAuth client with a non-default token max age [apigroup:oauth.openshift.io] to generate tokens that do not expire works as expected when using a token authorization flow [apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] [Token Expiration] Using a OAuth client with a non-default token max age [apigroup:oauth.openshift.io] to generate tokens that expire shortly works as expected when using a code authorization flow [apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] [Token Expiration] Using a OAuth client with a non-default token max age [apigroup:oauth.openshift.io] to generate tokens that expire shortly works as expected when using a token authorization flow [apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] [apigroup:oauth.openshift.io] OAuthClientWithRedirectURIs must validate request URIs according to oauth-client definition": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OAuthServer] well-known endpoint should be reachable [apigroup:route.openshift.io] [apigroup:oauth.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization] RBAC proxy for openshift authz RunLegacyClusterRoleBindingEndpoint should succeed [apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization] RBAC proxy for openshift authz RunLegacyClusterRoleEndpoint should succeed [apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization] RBAC proxy for openshift authz RunLegacyEndpointConfirmNoEscalation [apigroup:authorization.openshift.io] should succeed": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization] RBAC proxy for openshift authz RunLegacyLocalRoleBindingEndpoint should succeed [apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization] RBAC proxy for openshift authz RunLegacyLocalRoleEndpoint should succeed [apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization] The default cluster RBAC policy should have correct RBAC rules": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization] authorization TestAuthorizationSubjectAccessReview should succeed [apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization] authorization TestAuthorizationSubjectAccessReviewAPIGroup should succeed [apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization] authorization TestBrowserSafeAuthorizer should succeed [apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization] authorization TestClusterReaderCoverage should succeed": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization] scopes TestScopeEscalations should succeed [apigroup:user.openshift.io][apigroup:authorization.openshift.io][apigroup:build.openshift.io][apigroup:oauth.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization] scopes TestScopedImpersonation should succeed [apigroup:user.openshift.io][apigroup:authorization.openshift.io][apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization] scopes TestScopedTokens should succeed [apigroup:user.openshift.io][apigroup:authorization.openshift.io][apigroup:oauth.openshift.io][apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization] scopes TestTokensWithIllegalScopes should succeed [apigroup:oauth.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization] scopes TestUnknownScopes should succeed [apigroup:user.openshift.io][apigroup:authorization.openshift.io][apigroup:project.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization] self-SAR compatibility TestBootstrapPolicySelfSubjectAccessReviews should succeed [apigroup:user.openshift.io][apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization] self-SAR compatibility TestSelfSubjectAccessReviewsNonExistingNamespace should succeed [apigroup:user.openshift.io][apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:OpenShiftAuthorization][Serial] authorization TestAuthorizationResourceAccessReview should succeed [apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[sig-auth][Feature:PodSecurity] restricted-v2 SCC should mutate empty securityContext to match restricted PSa profile": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:PodSecurity][Feature:SCC] SCC admission fails for incorrect/non-existent required-scc annotation": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:PodSecurity][Feature:SCC] creating pod controllers": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:PodSecurity][Feature:SCC] required-scc annotation is being applied to workloads": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:ProjectAPI]  TestInvalidRoleRefs should succeed [apigroup:authorization.openshift.io][apigroup:user.openshift.io][apigroup:project.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:ProjectAPI]  TestProjectIsNamespace should succeed [apigroup:project.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:ProjectAPI]  TestProjectWatch should succeed [apigroup:project.openshift.io][apigroup:authorization.openshift.io][apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:ProjectAPI]  TestProjectWatchWithSelectionPredicate should succeed [apigroup:project.openshift.io][apigroup:authorization.openshift.io][apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:ProjectAPI]  TestScopedProjectAccess should succeed [apigroup:user.openshift.io][apigroup:project.openshift.io][apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:ProjectAPI]  TestUnprivilegedNewProject [apigroup:project.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:ProjectAPI][Serial]  TestUnprivilegedNewProjectDenied [apigroup:authorization.openshift.io][apigroup:project.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[sig-auth][Feature:RoleBindingRestrictions] RoleBindingRestrictions should be functional Create a RBAC rolebinding when subject is not already bound and is not permitted by any RBR should fail [apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:RoleBindingRestrictions] RoleBindingRestrictions should be functional Create a rolebinding that also contains system:non-existing users should succeed [apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:RoleBindingRestrictions] RoleBindingRestrictions should be functional Create a rolebinding when subject is already bound should succeed [apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:RoleBindingRestrictions] RoleBindingRestrictions should be functional Create a rolebinding when subject is not already bound and is not permitted by any RBR should fail [apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:RoleBindingRestrictions] RoleBindingRestrictions should be functional Create a rolebinding when subject is permitted by RBR should succeed [apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:RoleBindingRestrictions] RoleBindingRestrictions should be functional Create a rolebinding when there are no restrictions should succeed [apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:RoleBindingRestrictions] RoleBindingRestrictions should be functional Rolebinding restrictions tests single project should succeed [apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:SCC][Early] should not have pod creation failures during install": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:SecurityContextConstraints]  TestAllowedSCCViaRBAC [apigroup:project.openshift.io][apigroup:user.openshift.io][apigroup:authorization.openshift.io][apigroup:security.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:SecurityContextConstraints]  TestAllowedSCCViaRBAC with service account [apigroup:security.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:SecurityContextConstraints]  TestPodDefaultCapabilities": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:SecurityContextConstraints]  TestPodUpdateSCCEnforcement [apigroup:user.openshift.io][apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:SecurityContextConstraints]  TestPodUpdateSCCEnforcement with service account": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:UserAPI] groups should work [apigroup:user.openshift.io][apigroup:project.openshift.io][apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Feature:UserAPI] users can manipulate groups [apigroup:user.openshift.io][apigroup:authorization.openshift.io][apigroup:project.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-auth][Suite:openshift/auth/external-oidc][Serial][Slow][Disruptive] [OCPFeatureGate:ExternalOIDCWithUIDAndExtraClaimMappings] external IdP is configured with invalid specified UID or Extra claim mappings should reject admission when Extra claim expression is not compilable CEL": "",

	"[sig-auth][Suite:openshift/auth/external-oidc][Serial][Slow][Disruptive] [OCPFeatureGate:ExternalOIDCWithUIDAndExtraClaimMappings] external IdP is configured with invalid specified UID or Extra claim mappings should reject admission when UID claim expression is not compilable CEL": "",

	"[sig-auth][Suite:openshift/auth/external-oidc][Serial][Slow][Disruptive] [OCPFeatureGate:ExternalOIDCWithUIDAndExtraClaimMappings] external IdP is configured with valid specified UID or Extra claim mappings checking cluster identity mapping should map Extra correctly": "",

	"[sig-auth][Suite:openshift/auth/external-oidc][Serial][Slow][Disruptive] [OCPFeatureGate:ExternalOIDCWithUIDAndExtraClaimMappings] external IdP is configured with valid specified UID or Extra claim mappings checking cluster identity mapping should map UID correctly": "",

	"[sig-auth][Suite:openshift/auth/external-oidc][Serial][Slow][Disruptive] [OCPFeatureGate:ExternalOIDCWithUIDAndExtraClaimMappings] external IdP is configured without specified UID or Extra claim mappings should default UID to the 'sub' claim in the access token from the IdP": "",

	"[sig-auth][Suite:openshift/auth/external-oidc][Serial][Slow][Disruptive] [OCPFeatureGate:ExternalOIDC] external IdP is configured should accept authentication via a certificate-based kubeconfig (break-glass)": "",

	"[sig-auth][Suite:openshift/auth/external-oidc][Serial][Slow][Disruptive] [OCPFeatureGate:ExternalOIDC] external IdP is configured should configure kube-apiserver": "",

	"[sig-auth][Suite:openshift/auth/external-oidc][Serial][Slow][Disruptive] [OCPFeatureGate:ExternalOIDC] external IdP is configured should map cluster identities correctly": "",

	"[sig-auth][Suite:openshift/auth/external-oidc][Serial][Slow][Disruptive] [OCPFeatureGate:ExternalOIDC] external IdP is configured should not accept tokens provided by the OAuth server": "",

	"[sig-auth][Suite:openshift/auth/external-oidc][Serial][Slow][Disruptive] [OCPFeatureGate:ExternalOIDC] external IdP is configured should remove the OpenShift OAuth stack": "",

	"[sig-auth][Suite:openshift/auth/external-oidc][Serial][Slow][Disruptive] [OCPFeatureGate:ExternalOIDC] reverting to IntegratedOAuth should accept tokens provided by the OpenShift OAuth server": "",

	"[sig-auth][Suite:openshift/auth/external-oidc][Serial][Slow][Disruptive] [OCPFeatureGate:ExternalOIDC] reverting to IntegratedOAuth should not accept tokens provided by an external IdP": "",

	"[sig-auth][Suite:openshift/auth/external-oidc][Serial][Slow][Disruptive] [OCPFeatureGate:ExternalOIDC] reverting to IntegratedOAuth should rollout configuration on the kube-apiserver successfully": "",

	"[sig-auth][Suite:openshift/auth/external-oidc][Serial][Slow][Disruptive] [OCPFeatureGate:ExternalOIDC] reverting to IntegratedOAuth should rollout the OpenShift OAuth stack": "",

	"[sig-builds][Feature:Builds] Multi-stage image builds should succeed [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] Optimized image builds should succeed [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] build can reference a cluster service with a build being created from new-build should be able to run a build that references a cluster service [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Skipped:Proxy] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] build have source revision metadata started build should contain source revision information [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] build with empty source started build should build even with an empty source in build config [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] build without output image building from templates should create an image from a S2i template without an output image reference defined [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] build without output image building from templates should create an image from a docker template without an output image reference defined [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] buildconfig secret injector should inject secrets to the appropriate buildconfigs [apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] custom build with buildah being created from new-build should complete build with custom builder image [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] imagechangetriggers imagechangetriggers should trigger builds of all types [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] oc new-app should fail with a --name longer than 58 characters [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] oc new-app should succeed with a --name of 58 characters [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Skipped:Proxy] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] oc new-app should succeed with an imagestream [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] prune builds based on settings in the buildconfig buildconfigs should have a default history limit set when created via the group api [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] prune builds based on settings in the buildconfig should prune builds after a buildConfig change [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] prune builds based on settings in the buildconfig should prune canceled builds based on the failedBuildsHistoryLimit setting [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] prune builds based on settings in the buildconfig should prune completed builds based on the successfulBuildsHistoryLimit setting [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] prune builds based on settings in the buildconfig should prune errored builds based on the failedBuildsHistoryLimit setting [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] prune builds based on settings in the buildconfig should prune failed builds based on the failedBuildsHistoryLimit setting [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] remove all builds when build configuration is removed oc delete buildconfig should start builds and delete the buildconfig [apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] result image should have proper labels set Docker build from a template should create a image from \"test-docker-build.json\" template with proper Docker labels [apigroup:build.openshift.io][apigroup:image.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] result image should have proper labels set S2I build from a template should create a image from \"test-s2i-build.json\" template with proper Docker labels [apigroup:build.openshift.io][apigroup:image.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] s2i build with a quota Building from a template should create an s2i build with a quota and run it [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] s2i build with a root user image should create a root build and fail without a privileged SCC [apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] s2i build with a root user image should create a root build and pass with a privileged SCC [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] verify /run filesystem contents are writeable using a simple Docker Strategy Build [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds] verify /run filesystem contents do not have unexpected content using a simple Docker Strategy Build [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds][Serial][Slow][Disruptive] alter builds via cluster configuration build config no ocm rollout [apigroup:config.openshift.io] Apply default proxy configuration to docker build pod through env vars [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Serial][Slow][Disruptive] alter builds via cluster configuration build config no ocm rollout [apigroup:config.openshift.io] Apply default proxy configuration to source build pod through env vars [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Serial][Slow][Disruptive] alter builds via cluster configuration build config no ocm rollout [apigroup:config.openshift.io] Apply git proxy configuration to build pod [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Serial][Slow][Disruptive] alter builds via cluster configuration build config with ocm rollout [apigroup:config.openshift.io] Apply default image label configuration to build pod": "",

	"[sig-builds][Feature:Builds][Serial][Slow][Disruptive] alter builds via cluster configuration build config with ocm rollout [apigroup:config.openshift.io] Apply env configuration to build pod": "",

	"[sig-builds][Feature:Builds][Serial][Slow][Disruptive] alter builds via cluster configuration build config with ocm rollout [apigroup:config.openshift.io] Apply node selector configuration to build pod": "",

	"[sig-builds][Feature:Builds][Serial][Slow][Disruptive] alter builds via cluster configuration build config with ocm rollout [apigroup:config.openshift.io] Apply override image label configuration to build pod [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Serial][Slow][Disruptive] alter builds via cluster configuration build config with ocm rollout [apigroup:config.openshift.io] Apply resource configuration to build pod": "",

	"[sig-builds][Feature:Builds][Serial][Slow][Disruptive] alter builds via cluster configuration build config with ocm rollout [apigroup:config.openshift.io] Apply toleration override configuration to build pod": "",

	"[sig-builds][Feature:Builds][Serial][Slow][Disruptive] alter builds via cluster configuration registries config context should allow registries to be blacklisted": "",

	"[sig-builds][Feature:Builds][Serial][Slow][Disruptive] alter builds via cluster configuration registries config context should allow registries to be whitelisted": "",

	"[sig-builds][Feature:Builds][Serial][Slow][Disruptive] alter builds via cluster configuration registries config context should default registry search to docker.io for image pulls": "",

	"[sig-builds][Feature:Builds][Slow] Capabilities should be dropped for s2i builders s2i build with a rootable builder should not be able to switch to root with an assemble script [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] build can have Dockerfile input being created from new-build should be able to start a build from Dockerfile with FROM reference to scratch [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] build can have Dockerfile input being created from new-build should create a image via new-build [apigroup:build.openshift.io][apigroup:image.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] build can have Dockerfile input being created from new-build should create a image via new-build and infer the origin tag [apigroup:build.openshift.io][apigroup:image.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] build can have Dockerfile input being created from new-build testing build image with dockerfile contains a file path uses a variable in its name [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] build can have Dockerfile input being created from new-build testing build image with invalid dockerfile content [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] build can have container image source [apigroup:image.openshift.io] buildconfig with input source image and docker strategy should complete successfully and contain the expected file [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] build can have container image source [apigroup:image.openshift.io] buildconfig with input source image and s2i strategy should complete successfully and contain the expected file [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] build can have container image source [apigroup:image.openshift.io] creating a build with an input source image and custom strategy should resolve the imagestream references and secrets [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] build can have container image source [apigroup:image.openshift.io] creating a build with an input source image and docker strategy should resolve the imagestream references and secrets [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] build can have container image source [apigroup:image.openshift.io] creating a build with an input source image and s2i strategy should resolve the imagestream references and secrets [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] build controller RunBuildCompletePodDeleteTest should succeed [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] build controller RunBuildDeleteTest should succeed [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] build controller RunBuildRunningPodDeleteTest should succeed [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] builds should have deadlines oc start-build docker-build --wait Docker: should start a build and wait for the build failed and build pod being killed by kubelet [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] builds should have deadlines oc start-build source-build --wait Source: should start a build and wait for the build failed and build pod being killed by kubelet [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] builds should support proxies start build with broken proxy and a no_proxy override should start a docker build and wait for the build to succeed [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] builds should support proxies start build with broken proxy and a no_proxy override should start an s2i build and wait for the build to succeed [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] builds should support proxies start build with broken proxy should start a build and wait for the build to fail [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] builds should support proxies start build with cluster-wide custom PKI should mount the custom PKI into the build if specified [apigroup:config.openshift.io][apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] builds with a context directory docker context directory build should docker build an application using a context directory [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] builds with a context directory s2i context directory build should s2i build an application using a context directory [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] can use build secrets build with secrets and configMaps should contain secrets during the docker strategy build": "",

	"[sig-builds][Feature:Builds][Slow] can use build secrets build with secrets and configMaps should contain secrets during the source strategy build [apigroup:build.openshift.io][apigroup:image.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] can use private repositories as build input build using an HTTP token should be able to clone source code via an HTTP token [apigroup:build.openshift.io]": " [Disabled:Broken]",

	"[sig-builds][Feature:Builds][Slow] can use private repositories as build input build using an ssh private key should be able to clone source code via ssh [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] can use private repositories as build input build using an ssh private key should be able to clone source code via ssh using SCP-style URIs [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] completed builds should have digest of the image in their status Docker build started with log level >5 should save the image digest when finished [apigroup:build.openshift.io][apigroup:image.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] completed builds should have digest of the image in their status Docker build started with normal log level should save the image digest when finished [apigroup:build.openshift.io][apigroup:image.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] completed builds should have digest of the image in their status S2I build started with log level >5 should save the image digest when finished [apigroup:build.openshift.io][apigroup:image.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] completed builds should have digest of the image in their status S2I build started with normal log level should save the image digest when finished [apigroup:build.openshift.io][apigroup:image.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] incremental s2i build Building from a template should create a build from \"incremental-auth-build.json\" template and run it [apigroup:build.openshift.io][apigroup:image.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] s2i build with environment file in sources Building from a template should create a image from \"test-env-build.json\" template and run it in a pod [apigroup:build.openshift.io][apigroup:image.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context Setting build-args on Docker builds Should accept build args that are specified in the Dockerfile [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context Setting build-args on Docker builds Should complete with a warning on non-existent build-arg [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context Setting build-args on Docker builds Should copy build args from BuildConfig to Build [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context Trigger builds with branch refs matching directories on master branch Should checkout the config branch, not config directory [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context binary builds shoud accept --from-archive with https URL as an input [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context binary builds shoud accept --from-file with https URL as an input [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context binary builds should accept --from-dir as input [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context binary builds should accept --from-file as input [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context binary builds should accept --from-repo as input [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context binary builds should accept --from-repo with --commit as input [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context binary builds should reject binary build requests without a --from-xxxx value [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context cancel a binary build that doesn't start running in 5 minutes should start a build and wait for the build to be cancelled [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context cancel a build started by oc start-build --wait should start a build and wait for the build to cancel [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context oc start-build --wait should start a build and wait for the build to complete [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context oc start-build --wait should start a build and wait for the build to fail [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context oc start-build with pr ref should start a build from a PR ref, wait for the build to complete, and confirm the right level was used [apigroup:build.openshift.io][apigroup:image.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context override environment BUILD_LOGLEVEL in buildconfig can be overridden by build-loglevel [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context override environment BUILD_LOGLEVEL in buildconfig should create verbose output [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context override environment should accept environment variables [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context s2i build maintaining symlinks should s2i build image and maintain symlinks [apigroup:build.openshift.io][apigroup:image.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] starting a build using CLI start-build test context start a build via a webhook should be able to start builds via the webhook with valid secrets and fail with invalid secrets [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] testing build configuration hooks testing postCommit hook should run docker postCommit hooks [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] testing build configuration hooks testing postCommit hook should run s2i postCommit hooks [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] update failure status Build status Docker fetch image content failure should contain the Docker build fetch image content reason and message [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] update failure status Build status Docker fetch source failure should contain the Docker build fetch source failure reason and message [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] update failure status Build status OutOfMemoryKilled should contain OutOfMemoryKilled failure reason and message [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] update failure status Build status S2I bad context dir failure should contain the S2I bad context dir failure reason and message [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] update failure status Build status S2I fetch source failure should contain the S2I fetch source failure reason and message [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] update failure status Build status failed assemble container should contain the failure reason related to an assemble script failing in s2i [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] update failure status Build status failed https proxy invalid url should contain the generic failure reason and message [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] update failure status Build status fetch builder image failure should contain the fetch builder image failure reason and message [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] update failure status Build status postcommit hook failure should contain the post commit hook failure reason and message [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] update failure status Build status push image to registry failure should contain the image push to registry failure reason and message [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] using build configuration runPolicy build configuration with Parallel build run policy runs the builds in parallel [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] using build configuration runPolicy build configuration with Serial build run policy handling cancellation starts the next build immediately after one is canceled [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] using build configuration runPolicy build configuration with Serial build run policy handling deletion starts the next build immediately after running one is deleted [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] using build configuration runPolicy build configuration with Serial build run policy handling failure starts the next build immediately after one fails [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] using build configuration runPolicy build configuration with Serial build run policy runs the builds in serial order [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] using build configuration runPolicy build configuration with SerialLatestOnly build run policy runs the builds in serial order but cancel previous builds [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] using pull secrets in a build start-build test context binary builds should be able to run a build that is implicitly pulling from the internal registry [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] using pull secrets in a build start-build test context pulling from an external authenticated registry should be able to use a pull secret in a build [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][Slow] using pull secrets in a build start-build test context pulling from an external authenticated registry should be able to use a pull secret linked to the builder service account [apigroup:build.openshift.io]": "",

	"[sig-builds][Feature:Builds][pullsearch] docker build where the registry is not specified Building from a Dockerfile whose FROM image ref does not specify the image registry should create a docker build that has buildah search from our predefined list of image registries and succeed [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds][pullsecret] docker build using a pull secret Building from a template should create a docker build that pulls using a secret run it [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds][subscription-content] builds installing subscription content [apigroup:build.openshift.io] should succeed for RHEL 7 base images": " [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds][subscription-content] builds installing subscription content [apigroup:build.openshift.io] should succeed for RHEL 8 base images": " [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds][subscription-content] builds installing subscription content [apigroup:build.openshift.io] should succeed for RHEL 9 base images": " [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds][timing] capture build stages and durations should record build stages and durations for docker [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds][timing] capture build stages and durations should record build stages and durations for s2i [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds][valueFrom] process valueFrom in build strategy environment variables should fail resolving unresolvable valueFrom in docker build environment variable references [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds][valueFrom] process valueFrom in build strategy environment variables should fail resolving unresolvable valueFrom in sti build environment variable references [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds][valueFrom] process valueFrom in build strategy environment variables should successfully resolve valueFrom in docker build environment variables [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds][valueFrom] process valueFrom in build strategy environment variables should successfully resolve valueFrom in s2i build environment variables [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds][volumes] build volumes should mount given secrets and configmaps into the build pod for docker strategy builds [apigroup:image.openshift.io][apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds][volumes] build volumes should mount given secrets and configmaps into the build pod for source strategy builds [apigroup:image.openshift.io][apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds][webhook] TestWebhook [apigroup:build.openshift.io][apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds][webhook] TestWebhookGitHubPing [apigroup:image.openshift.io][apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds][webhook] TestWebhookGitHubPushWithImage [apigroup:image.openshift.io][apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:Builds][webhook] TestWebhookGitHubPushWithImageStream [apigroup:image.openshift.io][apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-builds][Feature:JenkinsRHELImagesOnly][Feature:Jenkins][Feature:Builds][sig-devex][Slow] openshift pipeline build jenkins pipeline build config strategy using a jenkins instance launched with the ephemeral template [apigroup:build.openshift.io]": "",

	"[sig-builds][sig-node][Feature:Builds][apigroup:build.openshift.io] zstd:chunked Image should successfully run date command": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-ci] [Early] prow job name should match cluster version [apigroup:config.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-ci] [Early] prow job name should match feature set": " [Suite:openshift/conformance/parallel]",

	"[sig-ci] [Early] prow job name should match network type": " [Suite:openshift/conformance/parallel]",

	"[sig-ci] [Early] prow job name should match platform type": " [Suite:openshift/conformance/parallel]",

	"[sig-ci] [Early] prow job name should match security mode": " [Suite:openshift/conformance/parallel]",

	"[sig-ci] [OTE] OpenShift Tests Extension [Suite:openshift/ote] should support tests that succeed": "",

	"[sig-ci] [OTE] OpenShift Tests Extension [Suite:openshift/ote] should support tests with an informing lifecycle": "",

	"[sig-cli] oc --request-timeout works as expected [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc --request-timeout works as expected for deployment": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm build-chain [apigroup:build.openshift.io][apigroup:image.openshift.io][apigroup:project.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm cluster-role-reapers [Serial][apigroup:authorization.openshift.io][apigroup:user.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[sig-cli] oc adm groups [apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm images [apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm must-gather runs successfully [apigroup:config.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm must-gather runs successfully for audit logs [apigroup:config.openshift.io][apigroup:oauth.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm must-gather runs successfully with options [apigroup:config.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm must-gather when looking at the audit logs [apigroup:config.openshift.io] [sig-node] kubelet runs apiserver processes strictly sequentially in order to not risk audit log corruption": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm new-project [apigroup:project.openshift.io][apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm node-logs": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm policy [apigroup:authorization.openshift.io][apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm release extract image-references": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm role-reapers [apigroup:authorization.openshift.io][apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm role-selectors [apigroup:template.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm serviceaccounts": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm storage-admin [apigroup:authorization.openshift.io][apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm ui-project-commands [apigroup:project.openshift.io][apigroup:authorization.openshift.io][apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm user-creation [apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc adm who-can [apigroup:authorization.openshift.io][apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc annotate pod": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc api-resources can output expected information about api-resources": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc api-resources can output expected information about build.openshift.io api-resources [apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc api-resources can output expected information about image.openshift.io api-resources [apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc api-resources can output expected information about operator.openshift.io api-resources [apigroup:operator.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc api-resources can output expected information about project.openshift.io api-resources [apigroup:project.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc api-resources can output expected information about route.openshift.io api-resources and api-version [apigroup:route.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc api-resources can output expected information about snapshot.storage.k8s.io api-resources [apigroup:snapshot.storage.k8s.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc basics can create and interact with a list of resources": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc basics can create deploymentconfig and clusterquota [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc basics can describe an OAuth access token [apigroup:oauth.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc basics can get version information from API": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc basics can get version information from CLI": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc basics can output expected --dry-run text": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc basics can patch resources [apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc basics can process templates [apigroup:template.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc basics can show correct whoami result with console": " [Skipped:NoOptionalCapabilities] [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc basics can show correct whoami result": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc builds complex build start-build [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc builds complex build webhooks CRUD [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc builds get buildconfig [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc builds new-build [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc builds patch buildconfig [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc can get list of nodes": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc can route traffic to services [apigroup:route.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc can run inside of a busybox container [apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc completion returns expected help messages": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc debug deployment from a build [apigroup:image.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc debug dissect deployment config debug [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc debug dissect deployment debug": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc debug does not require a real resource on the server": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc debug ensure debug does not depend on a container actually existing for the selected resource [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc debug ensure debug does not depend on a container actually existing for the selected resource for deployment": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc debug ensure it works with image streams [apigroup:image.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc env can set environment variables [apigroup:apps.openshift.io][apigroup:image.openshift.io][apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc env can set environment variables for deployment [apigroup:image.openshift.io][apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain list uncovered GroupVersionResources": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain networking types when using openshift-sdn should contain proper fields description for special networking types": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain proper fields description for apps.openshift.io [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain proper fields description for authorization.openshift.io [apigroup:authorization.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain proper fields description for config.openshift.io [apigroup:config.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain proper fields description for console.openshift.io [apigroup:console.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain proper fields description for image.openshift.io [apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain proper fields description for oauth.openshift.io [apigroup:oauth.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain proper fields description for project.openshift.io [apigroup:project.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain proper fields description for rangeallocations of security.openshift.io, if the resource is present [apigroup:security.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain proper fields description for route.openshift.io [apigroup:route.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain proper fields description for security.internal.openshift.io [apigroup:security.internal.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain proper fields description for securitycontextconstraints of security.openshift.io, if the resource is present [apigroup:security.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain proper fields description for template.openshift.io [apigroup:template.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain proper fields description for user.openshift.io [apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain proper spec+status for CRDs": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain spec+status for apps.openshift.io [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain spec+status for build.openshift.io [apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain spec+status for image.openshift.io [apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain spec+status for podsecuritypolicyreviews of security.openshift.io, if the resource is present [apigroup:security.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain spec+status for podsecuritypolicyselfsubjectreviews of security.openshift.io, if the resource is present [apigroup:security.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain spec+status for podsecuritypolicysubjectreviews of security.openshift.io, if the resource is present [apigroup:security.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain spec+status for project.openshift.io [apigroup:project.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain spec+status for route.openshift.io [apigroup:route.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc explain should contain spec+status for template.openshift.io [apigroup:template.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc expose can ensure the expose command is functioning as expected [apigroup:route.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc help works as expected": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc idle Deployments [apigroup:route.openshift.io][apigroup:project.openshift.io][apigroup:image.openshift.io] by all": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc idle Deployments [apigroup:route.openshift.io][apigroup:project.openshift.io][apigroup:image.openshift.io] by label": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc idle Deployments [apigroup:route.openshift.io][apigroup:project.openshift.io][apigroup:image.openshift.io] by name": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc idle [apigroup:apps.openshift.io][apigroup:route.openshift.io][apigroup:project.openshift.io][apigroup:image.openshift.io] by all": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc idle [apigroup:apps.openshift.io][apigroup:route.openshift.io][apigroup:project.openshift.io][apigroup:image.openshift.io] by checking previous scale": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc idle [apigroup:apps.openshift.io][apigroup:route.openshift.io][apigroup:project.openshift.io][apigroup:image.openshift.io] by label": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc idle [apigroup:apps.openshift.io][apigroup:route.openshift.io][apigroup:project.openshift.io][apigroup:image.openshift.io] by name": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc label pod": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc observe works as expected with cluster operators [apigroup:config.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc observe works as expected": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc probe can ensure the probe command is functioning as expected on deploymentconfigs [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc probe can ensure the probe command is functioning as expected on pods": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc project --show-labels works for projects [apigroup:project.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc project can switch between different projects [apigroup:authorization.openshift.io][apigroup:user.openshift.io][apigroup:project.openshift.io][Serial]": " [Suite:openshift/conformance/serial]",

	"[sig-cli] oc rsh specific flags should work well when access to a remote shell": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc run can use --image flag correctly [apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc run can use --image flag correctly for deployment": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc secret creates and retrieves expected": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc service creates and deletes services": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc set image can set images for pods and deployments [apigroup:image.openshift.io][Skipped:Disconnected]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc set image can set images for pods and deployments [apigroup:image.openshift.io][apigroup:apps.openshift.io][Skipped:Disconnected]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc statefulset creates and deletes statefulsets": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] oc status can show correct status after switching between projects [apigroup:project.openshift.io][apigroup:image.openshift.io][Serial]": " [Suite:openshift/conformance/serial]",

	"[sig-cli] oc status returns expected help messages [apigroup:project.openshift.io][apigroup:build.openshift.io][apigroup:image.openshift.io][apigroup:route.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] policy scc-subject-review, scc-review [apigroup:authorization.openshift.io][apigroup:user.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] templates different namespaces [apigroup:user.openshift.io][apigroup:project.openshift.io][apigroup:template.openshift.io][apigroup:authorization.openshift.io][Skipped:Disconnected]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli] templates process [apigroup:template.openshift.io][Skipped:Disconnected]": " [Suite:openshift/conformance/parallel]",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/authentication.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/builds.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/config.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/deployments.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/describer.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/edit.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/env.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/framework-test.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/get.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/image-lookup.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/images.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/printer.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/quota.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/secrets.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/set-data.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/set-liveness-probe.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/setbuildhook.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/setbuildsecret.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/triggers.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][Feature:LegacyCommandTests][Disruptive][Serial] test-cmd: test/cmd/volumes.sh [apigroup:image.openshift.io]": "",

	"[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status reports correctly when the cluster is not updating": " [Suite:openshift/conformance/parallel]",

	"[sig-cli][Slow] can use rsync to upload files to pods [apigroup:template.openshift.io] copy by strategy should copy files with the rsync strategy": "",

	"[sig-cli][Slow] can use rsync to upload files to pods [apigroup:template.openshift.io] copy by strategy should copy files with the rsync-daemon strategy": "",

	"[sig-cli][Slow] can use rsync to upload files to pods [apigroup:template.openshift.io] copy by strategy should copy files with the tar strategy": "",

	"[sig-cli][Slow] can use rsync to upload files to pods [apigroup:template.openshift.io] rsync specific flags should honor multiple --exclude flags": "",

	"[sig-cli][Slow] can use rsync to upload files to pods [apigroup:template.openshift.io] rsync specific flags should honor multiple --include flags": "",

	"[sig-cli][Slow] can use rsync to upload files to pods [apigroup:template.openshift.io] rsync specific flags should honor the --exclude flag": "",

	"[sig-cli][Slow] can use rsync to upload files to pods [apigroup:template.openshift.io] rsync specific flags should honor the --include flag": "",

	"[sig-cli][Slow] can use rsync to upload files to pods [apigroup:template.openshift.io] rsync specific flags should honor the --no-perms flag": "",

	"[sig-cli][Slow] can use rsync to upload files to pods [apigroup:template.openshift.io] rsync specific flags should honor the --progress flag": "",

	"[sig-cli][Slow] can use rsync to upload files to pods [apigroup:template.openshift.io] using a watch should watch for changes and rsync them": "",

	"[sig-cloud-provider][Feature:OpenShiftCloudControllerManager][Late] Cluster scoped load balancer healthcheck port and path should be 10256/healthz": " [Suite:openshift/conformance/parallel]",

	"[sig-cloud-provider][Feature:OpenShiftCloudControllerManager][Late] Deploy an external cloud provider [apigroup:machineconfiguration.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cluster-lifecycle] CSRs from machines that are not recognized by the cloud provider are not approved": " [Suite:openshift/conformance/parallel]",

	"[sig-cluster-lifecycle] Pods cannot access the /config/master API endpoint": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-cluster-lifecycle] TestAdminAck should succeed [apigroup:config.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cluster-lifecycle][Feature:Machines] Managed cluster should [sig-scheduling][Early] control plane machine set operator should not cause an early rollout": " [Suite:openshift/conformance/parallel]",

	"[sig-cluster-lifecycle][Feature:Machines] Managed cluster should [sig-scheduling][Early] control plane machine set operator should not have any events": " [Suite:openshift/conformance/parallel]",

	"[sig-cluster-lifecycle][Feature:Machines] Managed cluster should have machine resources [apigroup:machine.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cluster-lifecycle][Feature:Machines][Disruptive] Managed cluster should recover from deleted worker machines [apigroup:machine.openshift.io]": " [Serial]",

	"[sig-cluster-lifecycle][Feature:Machines][Early] Managed cluster should have same number of Machines and Nodes [apigroup:machine.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-cluster-lifecycle][Feature:Machines][Serial] Managed cluster should grow and decrease when scaling different machineSets simultaneously [Timeout:30m][apigroup:machine.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[sig-cluster-lifecycle][OCPFeatureGate:ImageStreamImportMode] ClusterVersion API desired architecture should be valid when architecture is set in release payload metadata [apigroup:config.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-coreos] [Conformance] CoreOS bootimages TestBootimagesPresent [apigroup:machineconfiguration.openshift.io]": " [Suite:openshift/conformance/parallel/minimal]",

	"[sig-devex] check registry.redhat.io is available and samples operator can import sample imagestreams run sample related validations [apigroup:config.openshift.io][apigroup:image.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/dotnet:6.0-ubi8\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/nginx:1.20-ubi9\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/nginx:1.22-ubi8\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/nginx:1.22-ubi9\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/nginx:1.26-ubi10\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/nodejs:20-ubi8\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/nodejs:20-ubi9\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/nodejs:22-ubi10\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/perl:5.26-ubi8\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/perl:5.32-ubi9\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/perl:5.40-ubi10\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/php:7.4-ubi8\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/php:8.0-ubi8\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/php:8.0-ubi9\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/php:8.2-ubi9\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/php:8.3-ubi10\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/python:3.11-ubi8\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/python:3.11-ubi9\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/python:3.6-ubi8\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/python:3.9-ubi8\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/python:3.9-ubi9\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/ruby:2.5-ubi8\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/ruby:3.0-ubi9\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/ruby:3.3-ubi10\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/ruby:3.3-ubi8\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled returning s2i usage when running the image \"image-registry.openshift-image-registry.svc:5000/openshift/ruby:3.3-ubi9\" should print the usage": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/dotnet:6.0-ubi8\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/nginx:1.20-ubi9\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/nginx:1.22-ubi8\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/nginx:1.22-ubi9\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/nginx:1.26-ubi10\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/nodejs:20-ubi8\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/nodejs:20-ubi9\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/nodejs:22-ubi10\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/perl:5.26-ubi8\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/perl:5.32-ubi9\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/perl:5.40-ubi10\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/php:7.4-ubi8\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/php:8.0-ubi8\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/php:8.0-ubi9\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/php:8.2-ubi9\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/php:8.3-ubi10\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/python:3.11-ubi8\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/python:3.11-ubi9\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/python:3.6-ubi8\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/python:3.9-ubi8\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/python:3.9-ubi9\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/ruby:2.5-ubi8\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/ruby:3.0-ubi9\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/ruby:3.3-ubi10\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/ruby:3.3-ubi8\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift images should be SCL enabled using the SCL in s2i images \"image-registry.openshift-image-registry.svc:5000/openshift/ruby:3.3-ubi9\" should be SCL enabled": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift sample application repositories [sig-devex][Feature:ImageEcosystem][nodejs] test nodejs images with nodejs-rest-http-crud db repo Building nodejs-postgresql app from new-app should build a nodejs-postgresql image and run it in a pod [apigroup:build.openshift.io]": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift sample application repositories [sig-devex][Feature:ImageEcosystem][php] test php images with cakephp-ex db repo Building cakephp-mysql app from new-app should build a cakephp-mysql image and run it in a pod [apigroup:build.openshift.io]": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift sample application repositories [sig-devex][Feature:ImageEcosystem][python] test python images with django-ex db repo Building django-psql app from new-app should build a django-psql image and run it in a pod [apigroup:build.openshift.io]": "",

	"[sig-devex][Feature:ImageEcosystem][Slow] openshift sample application repositories [sig-devex][Feature:ImageEcosystem][ruby] test ruby images with rails-ex db repo Building rails-postgresql app from new-app should build a rails-postgresql image and run it in a pod [apigroup:build.openshift.io]": "",

	"[sig-devex][Feature:ImageEcosystem][mariadb][Slow] openshift mariadb image Creating from a template should instantiate the template [apigroup:image.openshift.io][apigroup:operator.openshift.io][apigroup:config.openshift.io][apigroup:apps.openshift.io]": " [Disabled:Broken]",

	"[sig-devex][Feature:ImageEcosystem][mysql][Slow] openshift mysql image Creating from a template should instantiate the template [apigroup:apps.openshift.io]": " [Disabled:Broken]",

	"[sig-devex][Feature:ImageEcosystem][perl][Slow] hot deploy for openshift perl image hot deploy test should work [apigroup:image.openshift.io][apigroup:operator.openshift.io][apigroup:config.openshift.io][apigroup:build.openshift.io]": "",

	"[sig-devex][Feature:ImageEcosystem][php][Slow] hot deploy for openshift php image CakePHP example should work with hot deploy [apigroup:image.openshift.io][apigroup:operator.openshift.io][apigroup:config.openshift.io][apigroup:build.openshift.io]": "",

	"[sig-devex][Feature:ImageEcosystem][python][Slow] hot deploy for openshift python image Django example should work with hot deploy [apigroup:image.openshift.io][apigroup:operator.openshift.io][apigroup:config.openshift.io][apigroup:build.openshift.io]": "",

	"[sig-devex][Feature:ImageEcosystem][ruby][Slow] hot deploy for openshift ruby image Rails example should work with hot deploy [apigroup:image.openshift.io][apigroup:operator.openshift.io][apigroup:config.openshift.io][apigroup:build.openshift.io]": "",

	"[sig-devex][Feature:OpenShiftControllerManager] TestAutomaticCreationOfPullSecrets [apigroup:config.openshift.io][apigroup:image.openshift.io]": " [Skipped:NoOptionalCapabilities] [Suite:openshift/conformance/parallel]",

	"[sig-devex][Feature:OpenShiftControllerManager] TestDockercfgTokenDeletedController [apigroup:image.openshift.io]": " [Skipped:NoOptionalCapabilities] [Suite:openshift/conformance/parallel]",

	"[sig-devex][Feature:Templates] template-api TestTemplate [apigroup:template.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-devex][Feature:Templates] template-api TestTemplateTransformationFromConfig [apigroup:template.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-devex][Feature:Templates] templateinstance creation with invalid object reports error should report a failure on creation [apigroup:template.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-devex][Feature:Templates] templateinstance cross-namespace test should create and delete objects across namespaces [apigroup:user.openshift.io][apigroup:template.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-devex][Feature:Templates] templateinstance impersonation tests [apigroup:user.openshift.io][apigroup:authorization.openshift.io] should pass impersonation creation tests [apigroup:template.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-devex][Feature:Templates] templateinstance impersonation tests [apigroup:user.openshift.io][apigroup:authorization.openshift.io] should pass impersonation deletion tests [apigroup:template.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-devex][Feature:Templates] templateinstance impersonation tests [apigroup:user.openshift.io][apigroup:authorization.openshift.io] should pass impersonation update tests [apigroup:template.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-devex][Feature:Templates] templateinstance object kinds test should create and delete objects from varying API groups [apigroup:template.openshift.io][apigroup:route.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-devex][Feature:Templates] templateinstance readiness test should report failed soon after an annotated objects has failed [apigroup:template.openshift.io][apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-devex][Feature:Templates] templateinstance readiness test should report ready soon after all annotated objects are ready [apigroup:template.openshift.io][apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-devex][Feature:Templates] templateinstance security tests [apigroup:authorization.openshift.io][apigroup:template.openshift.io] should pass security tests [apigroup:route.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-etcd] etcd cluster has the same number of master nodes and voting members from the endpoints configmap [Early][apigroup:config.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-etcd] etcd leader changes are not excessive [Late]": " [Suite:openshift/conformance/parallel]",

	"[sig-etcd] etcd record the start revision of the etcd-operator [Early]": " [Suite:openshift/conformance/parallel]",

	"[sig-etcd][Feature:CertRotation][Suite:openshift/etcd/certrotation] etcd can manually rotate metrics signer certificates [Timeout:45m]": "",

	"[sig-etcd][Feature:CertRotation][Suite:openshift/etcd/certrotation] etcd can manually rotate signer certificates [Timeout:30m]": "",

	"[sig-etcd][Feature:CertRotation][Suite:openshift/etcd/certrotation] etcd can recreate dynamic certificates [Timeout:30m]": "",

	"[sig-etcd][Feature:CertRotation][Suite:openshift/etcd/certrotation] etcd can recreate trust bundle [Timeout:15m]": "",

	"[sig-etcd][Feature:DisasterRecovery][Suite:openshift/etcd/recovery][Disruptive] etcd is able to block the rollout of a revision when the quorum is not safe": " [Serial]",

	"[sig-etcd][Feature:DisasterRecovery][Suite:openshift/etcd/recovery][Timeout:1h] [Feature:EtcdRecovery][Disruptive] Recover with quorum restore": " [Serial]",

	"[sig-etcd][Feature:DisasterRecovery][Suite:openshift/etcd/recovery][Timeout:2h] [Feature:EtcdRecovery][Disruptive] Recover with snapshot with two unhealthy nodes and lost quorum": " [Serial]",

	"[sig-etcd][Feature:DisasterRecovery][Suite:openshift/etcd/recovery][Timeout:30m] [Feature:EtcdRecovery][Disruptive] Restore snapshot from node on another single unhealthy node": " [Serial]",

	"[sig-etcd][Feature:EtcdVerticalScaling][Suite:openshift/etcd/scaling][Serial] etcd is able to vertically scale up and down when CPMS is disabled [apigroup:machine.openshift.io]": "",

	"[sig-etcd][Feature:EtcdVerticalScaling][Suite:openshift/etcd/scaling][Serial] etcd is able to vertically scale up and down with a single node [Timeout:60m][apigroup:machine.openshift.io]": "",

	"[sig-etcd][OCPFeatureGate:HardwareSpeed][Serial] etcd is able to set the hardware speed to Slower [Timeout:30m][apigroup:machine.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[sig-etcd][OCPFeatureGate:HardwareSpeed][Serial] etcd is able to set the hardware speed to Standard [Timeout:30m][apigroup:machine.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[sig-etcd][OCPFeatureGate:HardwareSpeed][Serial] etcd is able to set the hardware speed to \"\" [Timeout:30m][apigroup:machine.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node] Two Node with Fencing pods and podman containers Should validate the number of etcd pods and containers as configured": "",

	"[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node] Two Node with Fencing pods and podman containers Should verify the number of podman-etcd containers as configured": "",

	"[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Disruptive] Two Node with Fencing etcd recovery Should recover from graceful node shutdown with etcd member re-addition": " [Serial]",

	"[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Disruptive] Two Node with Fencing etcd recovery Should recover from ungraceful node shutdown with etcd member re-addition": " [Serial]",

	"[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:HighlyAvailableArbiter] Ensure etcd health and quorum in HighlyAvailableArbiterMode should have all etcd pods running and quorum met": " [Suite:openshift/conformance/parallel]",

	"[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:HighlyAvailableArbiter][Suite:openshift/two-node][Disruptive] One master node outage is handled seamlessly should maintain etcd quorum and workloads with one master node down": " [Serial]",

	"[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:HighlyAvailableArbiter][Suite:openshift/two-node][Disruptive] Recovery when arbiter node is down and master nodes restart should regain quorum after arbiter down and master nodes restart": " [Serial]",

	"[sig-imagepolicy][OCPFeatureGate:SigstoreImageVerification][Serial] Should fail clusterimagepolicy signature validation root of trust does not match the identity in the signature": " [Suite:openshift/conformance/serial]",

	"[sig-imagepolicy][OCPFeatureGate:SigstoreImageVerification][Serial] Should fail clusterimagepolicy signature validation when scope in allowedRegistries list does not skip signature verification": " [Suite:openshift/conformance/serial]",

	"[sig-imagepolicy][OCPFeatureGate:SigstoreImageVerification][Serial] Should fail imagepolicy signature validation in different namespaces root of trust does not match the identity in the signature": " [Suite:openshift/conformance/serial]",

	"[sig-imagepolicy][OCPFeatureGate:SigstoreImageVerification][Serial] Should pass clusterimagepolicy signature validation with signed image": " [Suite:openshift/conformance/serial]",

	"[sig-imagepolicy][OCPFeatureGate:SigstoreImageVerification][Serial] Should pass imagepolicy signature validation with signed image in namespaces": " [Suite:openshift/conformance/serial]",

	"[sig-imageregistry] Image --dry-run should not delete resources [apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry] Image --dry-run should not update resources [apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry] Image registry [apigroup:route.openshift.io] should redirect on blob pull [apigroup:image.openshift.io]": " [Skipped:NoOptionalCapabilities] [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageAppend] Image append should create images by appending them [apigroup:image.openshift.io]": " [Skipped:Disconnected] [Skipped:NoOptionalCapabilities] [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageExtract] Image extract should extract content from an image [apigroup:image.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageInfo] Image info should display information about images [apigroup:image.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageLayers] Image layer subresource should identify a deleted image as missing [apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageLayers] Image layer subresource should return layers from tagged images [apigroup:image.openshift.io][apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageLookup] Image policy should perform lookup when the Deployment gets the resolve-names annotation later [apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageLookup] Image policy should perform lookup when the object has the resolve-names annotation [apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageLookup] Image policy should update OpenShift object image fields when local names are on [apigroup:image.openshift.io][apigroup:apps.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageLookup] Image policy should update standard Kube object image fields when local names are on [apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageMirror][Slow] Image mirror mirror image from integrated registry into few external registries [apigroup:image.openshift.io][apigroup:build.openshift.io]": "",

	"[sig-imageregistry][Feature:ImageMirror][Slow] Image mirror mirror image from integrated registry to external registry [apigroup:image.openshift.io][apigroup:build.openshift.io]": "",

	"[sig-imageregistry][Feature:ImagePrune][Serial][Suite:openshift/registry/serial][Local] Image hard prune [apigroup:apps.openshift.io][apigroup:user.openshift.io] should delete orphaned blobs [apigroup:image.openshift.io]": "",

	"[sig-imageregistry][Feature:ImagePrune][Serial][Suite:openshift/registry/serial][Local] Image hard prune [apigroup:apps.openshift.io][apigroup:user.openshift.io] should show orphaned blob deletions in dry-run mode [apigroup:image.openshift.io]": "",

	"[sig-imageregistry][Feature:ImagePrune][Serial][Suite:openshift/registry/serial][Local] Image prune [apigroup:user.openshift.io] of schema 1 should prune old image [apigroup:build.openshift.io][apigroup:image.openshift.io]": "",

	"[sig-imageregistry][Feature:ImagePrune][Serial][Suite:openshift/registry/serial][Local] Image prune [apigroup:user.openshift.io] of schema 2 should prune old image with config [apigroup:build.openshift.io][apigroup:image.openshift.io]": "",

	"[sig-imageregistry][Feature:ImagePrune][Serial][Suite:openshift/registry/serial][Local] Image prune [apigroup:user.openshift.io] with --all=false flag should prune only internally managed images [apigroup:build.openshift.io][apigroup:image.openshift.io]": "",

	"[sig-imageregistry][Feature:ImagePrune][Serial][Suite:openshift/registry/serial][Local] Image prune [apigroup:user.openshift.io] with --prune-registry==false should prune old image but skip registry [apigroup:build.openshift.io][apigroup:image.openshift.io]": "",

	"[sig-imageregistry][Feature:ImagePrune][Serial][Suite:openshift/registry/serial][Local] Image prune [apigroup:user.openshift.io] with default --all flag should prune both internally managed and external images [apigroup:build.openshift.io][apigroup:image.openshift.io]": "",

	"[sig-imageregistry][Feature:ImageQuota] Image resource quota should deny a push of built image exceeding openshift.io/imagestreams quota [apigroup:image.openshift.io]": " [Disabled:SpecialConfig]",

	"[sig-imageregistry][Feature:ImageQuota][Serial][Suite:openshift/registry/serial] Image limit range [apigroup:config.openshift.io][apigroup:image.openshift.io][apigroup:operator.openshift.io] should deny a container image reference exceeding limit on openshift.io/image-tags resource [apigroup:build.openshift.io]": " [Disabled:SpecialConfig]",

	"[sig-imageregistry][Feature:ImageQuota][Serial][Suite:openshift/registry/serial] Image limit range [apigroup:config.openshift.io][apigroup:image.openshift.io][apigroup:operator.openshift.io] should deny a push of built image exceeding limit on openshift.io/images resource [apigroup:build.openshift.io]": " [Disabled:SpecialConfig]",

	"[sig-imageregistry][Feature:ImageQuota][Serial][Suite:openshift/registry/serial] Image limit range [apigroup:config.openshift.io][apigroup:image.openshift.io][apigroup:operator.openshift.io] should deny a push of built image exceeding openshift.io/Image limit [apigroup:build.openshift.io]": " [Disabled:SpecialConfig]",

	"[sig-imageregistry][Feature:ImageQuota][Serial][Suite:openshift/registry/serial] Image limit range [apigroup:config.openshift.io][apigroup:image.openshift.io][apigroup:operator.openshift.io] should deny an import of a repository exceeding limit on openshift.io/image-tags resource [apigroup:build.openshift.io]": " [Disabled:SpecialConfig]",

	"[sig-imageregistry][Feature:ImageStreamImport][Serial][Slow] ImageStream API [apigroup:config.openshift.io] TestImportImageFromBlockedRegistry [apigroup:image.openshift.io]": "",

	"[sig-imageregistry][Feature:ImageStreamImport][Serial][Slow] ImageStream API [apigroup:config.openshift.io] TestImportImageFromInsecureRegistry [apigroup:image.openshift.io]": "",

	"[sig-imageregistry][Feature:ImageStreamImport][Serial][Slow] ImageStream API [apigroup:config.openshift.io] TestImportRepositoryFromBlockedRegistry [apigroup:image.openshift.io]": "",

	"[sig-imageregistry][Feature:ImageStreamImport][Serial][Slow] ImageStream API [apigroup:config.openshift.io] TestImportRepositoryFromInsecureRegistry [apigroup:image.openshift.io]": "",

	"[sig-imageregistry][Feature:ImageTriggers] Annotation trigger reconciles after the image is overwritten [apigroup:image.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageTriggers] Image change build triggers TestMultipleImageChangeBuildTriggers [apigroup:image.openshift.io][apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageTriggers] Image change build triggers TestSimpleImageChangeBuildTriggerFromImageStreamTagCustom [apigroup:image.openshift.io][apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageTriggers] Image change build triggers TestSimpleImageChangeBuildTriggerFromImageStreamTagCustomWithConfigChange [apigroup:image.openshift.io][apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageTriggers] Image change build triggers TestSimpleImageChangeBuildTriggerFromImageStreamTagDocker [apigroup:image.openshift.io][apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageTriggers] Image change build triggers TestSimpleImageChangeBuildTriggerFromImageStreamTagDockerWithConfigChange [apigroup:image.openshift.io][apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageTriggers] Image change build triggers TestSimpleImageChangeBuildTriggerFromImageStreamTagSTI [apigroup:image.openshift.io][apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageTriggers] Image change build triggers TestSimpleImageChangeBuildTriggerFromImageStreamTagSTIWithConfigChange [apigroup:image.openshift.io][apigroup:build.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:ImageTriggers][Serial] ImageStream API TestImageStreamMappingCreate [apigroup:image.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[sig-imageregistry][Feature:ImageTriggers][Serial] ImageStream API TestImageStreamTagLifecycleHook [apigroup:image.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[sig-imageregistry][Feature:ImageTriggers][Serial] ImageStream API TestImageStreamWithoutDockerImageConfig [apigroup:image.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[sig-imageregistry][Feature:ImageTriggers][Serial] ImageStream admission TestImageStreamAdmitSpecUpdate [apigroup:image.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[sig-imageregistry][Feature:ImageTriggers][Serial] ImageStream admission TestImageStreamAdmitStatusUpdate [apigroup:image.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[sig-imageregistry][Feature:ImageTriggers][Serial] ImageStream admission TestImageStreamTagsAdmission [apigroup:image.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[sig-imageregistry][Feature:Image] oc tag should change image reference for internal images [apigroup:build.openshift.io][apigroup:image.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:Image] oc tag should preserve image reference for external images [apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:Image] oc tag should work when only imagestreams api is available [apigroup:image.openshift.io][apigroup:authorization.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:Image] signature TestImageAddSignature [apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][Feature:Image] signature TestImageRemoveSignature [apigroup:image.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-imageregistry][OCPFeatureGate:ChunkSizeMiB][Serial][apigroup:imageregistry.operator.openshift.io] Image Registry Config ChunkSizeMiB should not accept invalid ChunkSizeMiB value": " [Suite:openshift/conformance/serial]",

	"[sig-imageregistry][OCPFeatureGate:ChunkSizeMiB][Serial][apigroup:imageregistry.operator.openshift.io] Image Registry Config ChunkSizeMiB should reject ChunkSizeMiB value greater than 5 GiB": " [Suite:openshift/conformance/serial]",

	"[sig-imageregistry][OCPFeatureGate:ChunkSizeMiB][Serial][apigroup:imageregistry.operator.openshift.io] Image Registry Config ChunkSizeMiB should set ChunkSizeMiB value": " [Suite:openshift/conformance/serial]",

	"[sig-imageregistry][OCPFeatureGate:ChunkSizeMiB][Serial][apigroup:imageregistry.operator.openshift.io] Image Registry Config ChunkSizeMiB should set maximum valid ChunkSizeMiB value": " [Suite:openshift/conformance/serial]",

	"[sig-imageregistry][OCPFeatureGate:ChunkSizeMiB][Serial][apigroup:imageregistry.operator.openshift.io] Image Registry Config ChunkSizeMiB should set minimum valid ChunkSizeMiB value": " [Suite:openshift/conformance/serial]",

	"[sig-imageregistry][OCPFeatureGate:ImageStreamImportMode][Serial] ImageStream API import mode should be Legacy if the import mode specified in image.config.openshift.io config is Legacy [apigroup:image.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[sig-imageregistry][OCPFeatureGate:ImageStreamImportMode][Serial] ImageStream API import mode should be PreserveOriginal if the import mode specified in image.config.openshift.io config is PreserveOriginal [apigroup:image.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[sig-imageregistry][OCPFeatureGate:ImageStreamImportMode][Serial] ImageStream API import mode should be PreserveOriginal or Legacy depending on desired.architecture field in the CV [apigroup:image.openshift.io]": " [Suite:openshift/conformance/serial]",

	"[sig-imageregistry][Serial] Image signature workflow can push a signed image to openshift registry and verify it [apigroup:user.openshift.io][apigroup:image.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/serial]",

	"[sig-installer][Feature:baremetal] Baremetal platform should have baremetalhost resources": " [Suite:openshift/conformance/parallel]",

	"[sig-installer][Feature:baremetal] Baremetal platform should have hostfirmwaresetting resources": " [Suite:openshift/conformance/parallel]",

	"[sig-installer][Feature:baremetal] Baremetal platform should have preprovisioning images for workers": " [Suite:openshift/conformance/parallel]",

	"[sig-installer][Feature:baremetal] Baremetal platform should not allow updating BootMacAddress": " [Suite:openshift/conformance/parallel]",

	"[sig-installer][Feature:baremetal] Baremetal/OpenStack/vSphere/None/AWS/Azure/GCP platforms have a metal3 deployment": " [Suite:openshift/conformance/parallel]",

	"[sig-installer][Feature:baremetal][Serial] Baremetal platform should ensure [apigroup:config.openshift.io] cluster baremetal operator and metal3 deployment return back healthy after they are deleted": " [Suite:openshift/conformance/serial]",

	"[sig-installer][Feature:baremetal][Serial] Baremetal platform should skip inspection when disabled by annotation": " [Suite:openshift/conformance/serial]",

	"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster should have a AlertmanagerReceiversNotConfigured alert in firing state": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster should have important platform topology metrics [apigroup:config.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster should have non-Pod host cAdvisor metrics": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster should provide ingress metrics": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster should provide named network metrics [apigroup:project.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster should report telemetry [Serial] [Late]": " [Skipped:Disconnected] [Suite:openshift/conformance/serial]",

	"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster should start and expose a secured proxy and unsecured metrics [apigroup:config.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster shouldn't have failing rules evaluation": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster shouldn't report any alerts in firing state apart from Watchdog and AlertmanagerReceiversNotConfigured [Early][apigroup:config.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-instrumentation] Prometheus [apigroup:image.openshift.io] when installed on the cluster when using openshift-sdn should be able to get the sdn ovs flows": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-instrumentation][Late] Alerts shouldn't exceed the series limit of total series sent via telemetry from each cluster": " [Suite:openshift/conformance/parallel]",

	"[sig-instrumentation][Late] OpenShift alerting rules [apigroup:image.openshift.io] should have a runbook_url annotation if the alert is critical": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-instrumentation][Late] OpenShift alerting rules [apigroup:image.openshift.io] should have a valid severity label": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-instrumentation][Late] OpenShift alerting rules [apigroup:image.openshift.io] should have description and summary annotations": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-instrumentation][Late] OpenShift alerting rules [apigroup:image.openshift.io] should link to a valid URL if the runbook_url annotation is defined": " [Suite:openshift/conformance/parallel]",

	"[sig-instrumentation][Late] OpenShift alerting rules [apigroup:image.openshift.io] should link to an HTTP(S) location if the runbook_url annotation is defined": " [Suite:openshift/conformance/parallel]",

	"[sig-instrumentation][Late] Platform Prometheus targets should not be accessible without auth [Serial]": " [Suite:openshift/conformance/serial]",

	"[sig-instrumentation][OCPFeatureGate:MetricsCollectionProfiles][Serial] The collection profiles feature-set in a heterogeneous environment, should expose information about the applied collection profile using meta-metrics": " [Suite:openshift/conformance/serial]",

	"[sig-instrumentation][OCPFeatureGate:MetricsCollectionProfiles][Serial] The collection profiles feature-set in a heterogeneous environment, should have at least one implementation for each collection profile": " [Suite:openshift/conformance/serial]",

	"[sig-instrumentation][OCPFeatureGate:MetricsCollectionProfiles][Serial] The collection profiles feature-set in a heterogeneous environment, should revert to default collection profile when an empty collection profile value is specified": " [Suite:openshift/conformance/serial]",

	"[sig-instrumentation][OCPFeatureGate:MetricsCollectionProfiles][Serial] The collection profiles feature-set in a homogeneous minimal environment, should hide default metrics": " [Suite:openshift/conformance/serial]",

	"[sig-instrumentation][OCPFeatureGate:MetricsCollectionProfiles][Serial] The collection profiles feature-set initially, in a homogeneous default environment, should expose default metrics": " [Suite:openshift/conformance/serial]",

	"[sig-instrumentation][sig-builds][Feature:Builds] Prometheus when installed on the cluster should start and expose a secured proxy and verify build metrics [apigroup:build.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-kubevirt] migration when running openshift cluster on KubeVirt virtual machines and live migrate hosted control plane workers [Early] should maintain node readiness": " [Suite:openshift/conformance/parallel]",

	"[sig-kubevirt] services when running openshift cluster on KubeVirt virtual machines should allow connections to pods from guest cluster PodNetwork pod via LoadBalancer service across different guest nodes": " [Suite:openshift/conformance/parallel]",

	"[sig-kubevirt] services when running openshift cluster on KubeVirt virtual machines should allow connections to pods from guest hostNetwork pod via NodePort across different guest nodes": " [Suite:openshift/conformance/parallel]",

	"[sig-kubevirt] services when running openshift cluster on KubeVirt virtual machines should allow connections to pods from guest podNetwork pod via NodePort across different guest nodes": " [Suite:openshift/conformance/parallel]",

	"[sig-kubevirt] services when running openshift cluster on KubeVirt virtual machines should allow connections to pods from infra cluster pod via LoadBalancer service across different guest nodes": " [Suite:openshift/conformance/parallel]",

	"[sig-kubevirt] services when running openshift cluster on KubeVirt virtual machines should allow connections to pods from infra cluster pod via NodePort across different infra nodes": " [Suite:openshift/conformance/parallel]",

	"[sig-kubevirt] services when running openshift cluster on KubeVirt virtual machines should allow direct connections to pods from guest cluster pod in host network across different guest nodes": " [Suite:openshift/conformance/parallel]",

	"[sig-kubevirt] services when running openshift cluster on KubeVirt virtual machines should allow direct connections to pods from guest cluster pod in pod network across different guest nodes": " [Suite:openshift/conformance/parallel]",

	"[sig-network-edge] DNS should answer A and AAAA queries for a dual-stack service [apigroup:config.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-network-edge] DNS should answer endpoint and wildcard queries for the cluster": " [Disabled:Broken]",

	"[sig-network-edge] DNS should answer queries using the local DNS endpoint": " [Suite:openshift/conformance/parallel]",

	"[sig-network-edge][Conformance][Area:Networking][Feature:Router] The HAProxy router should be able to connect to a service that is idled because a GET on the route will unidle it": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel/minimal]",

	"[sig-network-edge][Conformance][Area:Networking][Feature:Router] The HAProxy router should pass the gRPC interoperability tests [apigroup:route.openshift.io][apigroup:operator.openshift.io]": " [Suite:openshift/conformance/parallel/minimal]",

	"[sig-network-edge][Conformance][Area:Networking][Feature:Router][apigroup:route.openshift.io] The HAProxy router should pass the h2spec conformance tests [apigroup:authorization.openshift.io][apigroup:user.openshift.io][apigroup:security.openshift.io][apigroup:operator.openshift.io]": " [Suite:openshift/conformance/parallel/minimal]",

	"[sig-network-edge][Conformance][Area:Networking][Feature:Router][apigroup:route.openshift.io][apigroup:config.openshift.io] The HAProxy router should pass the http2 tests [apigroup:image.openshift.io][apigroup:operator.openshift.io]": " [Suite:openshift/conformance/parallel/minimal]",

	"[sig-network-edge][Feature:Idling] Idling with a single service and DeploymentConfig [apigroup:route.openshift.io] should idle the service and DeploymentConfig properly [apigroup:apps.openshift.io]": " [Disabled:Broken]",

	"[sig-network-edge][Feature:Idling] Idling with a single service and ReplicationController should idle the service and ReplicationController properly": " [Suite:openshift/conformance/parallel]",

	"[sig-network-edge][Feature:Idling] Unidling [apigroup:apps.openshift.io][apigroup:route.openshift.io] should handle many TCP connections by possibly dropping those over a certain bound [Serial]": " [Suite:openshift/conformance/serial]",

	"[sig-network-edge][Feature:Idling] Unidling [apigroup:apps.openshift.io][apigroup:route.openshift.io] should handle many UDP senders (by continuing to drop all packets on the floor) [Serial]": " [Suite:openshift/conformance/serial]",

	"[sig-network-edge][Feature:Idling] Unidling [apigroup:apps.openshift.io][apigroup:route.openshift.io] should work with TCP (when fully idled)": " [Suite:openshift/conformance/parallel]",

	"[sig-network-edge][Feature:Idling] Unidling [apigroup:apps.openshift.io][apigroup:route.openshift.io] should work with TCP (while idling)": " [Disabled:Broken]",

	"[sig-network-edge][Feature:Idling] Unidling [apigroup:apps.openshift.io][apigroup:route.openshift.io] should work with UDP": " [Suite:openshift/conformance/parallel]",

	"[sig-network-edge][Feature:Idling] Unidling with Deployments [apigroup:route.openshift.io] should handle many TCP connections by possibly dropping those over a certain bound [Serial]": " [Suite:openshift/conformance/serial]",

	"[sig-network-edge][Feature:Idling] Unidling with Deployments [apigroup:route.openshift.io] should handle many UDP senders (by continuing to drop all packets on the floor) [Serial]": " [Suite:openshift/conformance/serial]",

	"[sig-network-edge][Feature:Idling] Unidling with Deployments [apigroup:route.openshift.io] should work with TCP (when fully idled)": " [Suite:openshift/conformance/parallel]",

	"[sig-network-edge][Feature:Idling] Unidling with Deployments [apigroup:route.openshift.io] should work with TCP (while idling)": " [Disabled:Broken]",

	"[sig-network-edge][Feature:Idling] Unidling with Deployments [apigroup:route.openshift.io] should work with UDP": " [Suite:openshift/conformance/parallel]",

	"[sig-network-edge][OCPFeatureGate:GatewayAPIController][Feature:Router][apigroup:gateway.networking.k8s.io] Ensure HTTPRoute object is created": " [Suite:openshift/conformance/parallel]",

	"[sig-network-edge][OCPFeatureGate:GatewayAPIController][Feature:Router][apigroup:gateway.networking.k8s.io] Ensure LB, service, and dnsRecord are created for a Gateway object": " [Suite:openshift/conformance/parallel]",

	"[sig-network-edge][OCPFeatureGate:GatewayAPIController][Feature:Router][apigroup:gateway.networking.k8s.io] Ensure OSSM and OLM related resources are created after creating GatewayClass": " [Suite:openshift/conformance/parallel]",

	"[sig-network-edge][OCPFeatureGate:GatewayAPIController][Feature:Router][apigroup:gateway.networking.k8s.io] Ensure custom gatewayclass can be accepted": " [Suite:openshift/conformance/parallel]",

	"[sig-network-edge][OCPFeatureGate:GatewayAPIController][Feature:Router][apigroup:gateway.networking.k8s.io] Ensure default gatewayclass is accepted": " [Suite:openshift/conformance/parallel]",

	"[sig-network] Internal connectivity for TCP and UDP on ports 9000-9999 is allowed [Serial:Self]": " [Suite:openshift/conformance/parallel]",

	"[sig-network] external gateway address when using openshift ovn-kubernetes should match the address family of the pod": " [Suite:openshift/conformance/parallel]",

	"[sig-network] load balancer should be managed by OpenShift": " [Suite:openshift/conformance/parallel]",

	"[sig-network] load balancer should not be managed by OpenShift": " [Suite:openshift/conformance/parallel]",

	"[sig-network] multicast when using one of the OpenshiftSDN modes 'redhat/openshift-ovs-multitenant, redhat/openshift-ovs-networkpolicy' should allow multicast traffic in namespaces where it is enabled": " [Suite:openshift/conformance/parallel]",

	"[sig-network] multicast when using one of the OpenshiftSDN modes 'redhat/openshift-ovs-multitenant, redhat/openshift-ovs-networkpolicy' should block multicast traffic in namespaces where it is disabled": " [Suite:openshift/conformance/parallel]",

	"[sig-network] multicast when using one of the OpenshiftSDN modes 'redhat/openshift-ovs-subnet' should block multicast traffic": " [Suite:openshift/conformance/parallel]",

	"[sig-network] network isolation when using a plugin in a mode that does not isolate namespaces by default should allow communication between pods in different namespaces on different nodes": " [Suite:openshift/conformance/parallel]",

	"[sig-network] network isolation when using a plugin in a mode that does not isolate namespaces by default should allow communication between pods in different namespaces on the same node": " [Suite:openshift/conformance/parallel]",

	"[sig-network] network isolation when using a plugin in a mode that isolates namespaces by default should allow communication from default to non-default namespace on a different node": " [Suite:openshift/conformance/parallel]",

	"[sig-network] network isolation when using a plugin in a mode that isolates namespaces by default should allow communication from default to non-default namespace on the same node": " [Suite:openshift/conformance/parallel]",

	"[sig-network] network isolation when using a plugin in a mode that isolates namespaces by default should allow communication from non-default to default namespace on a different node": " [Suite:openshift/conformance/parallel]",

	"[sig-network] network isolation when using a plugin in a mode that isolates namespaces by default should allow communication from non-default to default namespace on the same node": " [Suite:openshift/conformance/parallel]",

	"[sig-network] network isolation when using a plugin in a mode that isolates namespaces by default should prevent communication between pods in different namespaces on different nodes": " [Suite:openshift/conformance/parallel]",

	"[sig-network] network isolation when using a plugin in a mode that isolates namespaces by default should prevent communication between pods in different namespaces on the same node": " [Suite:openshift/conformance/parallel]",

	"[sig-network] services basic functionality should allow connections to another pod on a different node via a service IP": " [Suite:openshift/conformance/parallel]",

	"[sig-network] services basic functionality should allow connections to another pod on the same node via a service IP": " [Suite:openshift/conformance/parallel]",

	"[sig-network] services when running openshift ipv4 cluster ensures external ip policy is configured correctly on the cluster [apigroup:config.openshift.io] [Serial]": " [Suite:openshift/conformance/serial]",

	"[sig-network] services when running openshift ipv4 cluster on bare metal [apigroup:config.openshift.io] ensures external auto assign cidr is configured correctly on the cluster [apigroup:config.openshift.io] [Serial]": " [Suite:openshift/conformance/serial]",

	"[sig-network] services when using a plugin in a mode that does not isolate namespaces by default should allow connections to pods in different namespaces on different nodes via service IPs": " [Suite:openshift/conformance/parallel]",

	"[sig-network] services when using a plugin in a mode that does not isolate namespaces by default should allow connections to pods in different namespaces on the same node via service IPs": " [Suite:openshift/conformance/parallel]",

	"[sig-network] services when using a plugin in a mode that isolates namespaces by default should allow connections from pods in the default namespace to a service in another namespace on a different node": " [Suite:openshift/conformance/parallel]",

	"[sig-network] services when using a plugin in a mode that isolates namespaces by default should allow connections from pods in the default namespace to a service in another namespace on the same node": " [Suite:openshift/conformance/parallel]",

	"[sig-network] services when using a plugin in a mode that isolates namespaces by default should allow connections to services in the default namespace from a pod in another namespace on a different node": " [Suite:openshift/conformance/parallel]",

	"[sig-network] services when using a plugin in a mode that isolates namespaces by default should allow connections to services in the default namespace from a pod in another namespace on the same node": " [Suite:openshift/conformance/parallel]",

	"[sig-network] services when using a plugin in a mode that isolates namespaces by default should prevent connections to pods in different namespaces on different nodes via service IPs": " [Suite:openshift/conformance/parallel]",

	"[sig-network] services when using a plugin in a mode that isolates namespaces by default should prevent connections to pods in different namespaces on the same node via service IPs": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:CNIMigration] All nodes should be in ready state [Early][Suite:openshift/network/live-migration]": "",

	"[sig-network][Feature:CNIMigration] Cluster operators should be stable [Late][Suite:openshift/network/live-migration]": "",

	"[sig-network][Feature:CNIMigration] Cluster should not be live migrating before beginning migration [Early][Suite:openshift/network/live-migration]": "",

	"[sig-network][Feature:CNIMigration] Should perform live migration [Disruptive][Suite:openshift/network/live-migration]": " [Serial]",

	"[sig-network][Feature:CNIMigration] Target CNI should not be deployed [Early][Suite:openshift/network/live-migration]": "",

	"[sig-network][Feature:EgressFirewall] egressFirewall should have no impact outside its namespace": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:EgressFirewall] when using openshift ovn-kubernetes should ensure egressfirewall is created": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:EgressFirewall] when using openshift-sdn should ensure egressnetworkpolicy is created [apigroup:network.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:EgressIP][apigroup:operator.openshift.io] [external-targets][apigroup:user.openshift.io][apigroup:security.openshift.io] EgressIPs can be assigned automatically [Skipped:Network/OVNKubernetes]": " [Serial] [Suite:openshift/conformance/serial]",

	"[sig-network][Feature:EgressIP][apigroup:operator.openshift.io] [external-targets][apigroup:user.openshift.io][apigroup:security.openshift.io] only pods matched by the pod selector should have the EgressIPs [Skipped:Network/OpenShiftSDN]": " [Serial] [Suite:openshift/conformance/serial]",

	"[sig-network][Feature:EgressIP][apigroup:operator.openshift.io] [external-targets][apigroup:user.openshift.io][apigroup:security.openshift.io] pods should have the assigned EgressIPs and EgressIPs can be deleted and recreated [Skipped:azure][apigroup:route.openshift.io]": " [Serial] [Suite:openshift/conformance/serial]",

	"[sig-network][Feature:EgressIP][apigroup:operator.openshift.io] [external-targets][apigroup:user.openshift.io][apigroup:security.openshift.io] pods should have the assigned EgressIPs and EgressIPs can be updated [Skipped:Network/OpenShiftSDN]": " [Serial] [Suite:openshift/conformance/serial]",

	"[sig-network][Feature:EgressIP][apigroup:operator.openshift.io] [external-targets][apigroup:user.openshift.io][apigroup:security.openshift.io] pods should keep the assigned EgressIPs when being rescheduled to another node": " [Serial] [Suite:openshift/conformance/serial]",

	"[sig-network][Feature:EgressIP][apigroup:operator.openshift.io] [internal-targets] EgressIP pods should query hostNetwork pods with the local node's SNAT": " [Disabled:Broken] [Serial]",

	"[sig-network][Feature:EgressRouterCNI] should ensure ipv4 egressrouter cni resources are created [apigroup:operator.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:EgressRouterCNI] when using openshift ovn-kubernetes should ensure ipv6 egressrouter cni resources are created [apigroup:operator.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:IPsec] IPsec resilience when using openshift ovn-kubernetes check pod traffic is working across nodes [apigroup:config.openshift.io] [Suite:openshift/network/ipsec]": "",

	"[sig-network][Feature:IPsec] IPsec resilience when using openshift ovn-kubernetes check pod traffic is working across nodes after ipsec daemonset restart [apigroup:config.openshift.io] [Suite:openshift/network/ipsec]": "",

	"[sig-network][Feature:IPsec] when using openshift ovn-kubernetes check traffic with IPsec [apigroup:config.openshift.io] [Suite:openshift/network/ipsec]": "",

	"[sig-network][Feature:Layer2LiveMigration][OCPFeatureGate:NetworkSegmentation][Suite:openshift/network/virtualization] primary UDN smoke test when using openshift ovn-kubernetes assert the primary UDN feature works as expected": "",

	"[sig-network][Feature:Layer2LiveMigration][Suite:openshift/network/virtualization] Kubevirt Virtual Machines Placeholder test for GA": "",

	"[sig-network][Feature:MultiNetworkPolicy][Serial][apigroup:operator.openshift.io] should enforce a network policies on secondary network IPv4": " [Suite:openshift/conformance/serial]",

	"[sig-network][Feature:MultiNetworkPolicy][Serial][apigroup:operator.openshift.io] should enforce a network policies on secondary network IPv6": " [Suite:openshift/conformance/serial]",

	"[sig-network][Feature:Multus] should use multus to create net1 device from network-attachment-definition [apigroup:k8s.cni.cncf.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Network Policy Audit logging] when using openshift ovn-kubernetes should ensure acl logs are created and correct [apigroup:project.openshift.io][apigroup:network.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router] The HAProxy router should enable openshift-monitoring to pull metrics": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router] The HAProxy router should expose a health check on the metrics port": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router] The HAProxy router should expose prometheus metrics for a route [apigroup:route.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router] The HAProxy router should expose the profiling endpoints": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router][apigroup:image.openshift.io] The HAProxy router should serve a route that points to two services and respect weights": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router][apigroup:operator.openshift.io] The HAProxy router should respond with 503 to unrecognized hosts": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router][apigroup:operator.openshift.io] The HAProxy router should serve routes that were created from an ingress [apigroup:route.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router][apigroup:operator.openshift.io] The HAProxy router should set Forwarded headers appropriately": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router][apigroup:route.openshift.io] The HAProxy router converges when multiple routers are writing conflicting status": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router][apigroup:route.openshift.io] The HAProxy router converges when multiple routers are writing conflicting upgrade validation status": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router][apigroup:route.openshift.io] The HAProxy router converges when multiple routers are writing status": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router][apigroup:route.openshift.io] The HAProxy router reports the expected host names in admitted routes' statuses": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router][apigroup:route.openshift.io] The HAProxy router should override the route host for overridden domains with a custom value [apigroup:image.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router][apigroup:route.openshift.io] The HAProxy router should override the route host with a custom value": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router][apigroup:route.openshift.io] The HAProxy router should run even if it has no access to update status [apigroup:image.openshift.io]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router][apigroup:route.openshift.io] The HAProxy router should serve the correct routes when running with the haproxy config manager": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router][apigroup:route.openshift.io] The HAProxy router should serve the correct routes when scoped to a single namespace and label set": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router][apigroup:route.openshift.io] when FIPS is disabled the HAProxy router should serve routes when configured with a 1024-bit RSA key": " [Feature:Networking-IPv4] [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router][apigroup:route.openshift.io] when FIPS is enabled the HAProxy router should not work when configured with a 1024-bit RSA key": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Router][apigroup:route.openshift.io][apigroup:operator.openshift.io] The HAProxy router should support reencrypt to services backed by a serving certificate automatically": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Whereabouts] should assign unique IP addresses to each pod in the event of a race condition case [apigroup:k8s.cni.cncf.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:Whereabouts] should use whereabouts net-attach-def to limit IP ranges for newly created pods [apigroup:k8s.cni.cncf.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:bond] should create a pod with bond interface [apigroup:k8s.cni.cncf.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:tap] should create a pod with a tap interface [apigroup:k8s.cni.cncf.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:tuning] pod should not start for sysctls not on whitelist [apigroup:k8s.cni.cncf.io] net.ipv4.conf.IFNAME.arp_filter": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:tuning] pod should not start for sysctls not on whitelist [apigroup:k8s.cni.cncf.io] net.ipv4.conf.all.send_redirects": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:tuning] pod should start with all sysctl on whitelist [apigroup:k8s.cni.cncf.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:tuning] pod sysctl should not affect existing pods [apigroup:k8s.cni.cncf.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:tuning] pod sysctl should not affect newly created pods [apigroup:k8s.cni.cncf.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:tuning] pod sysctls should not affect node [apigroup:k8s.cni.cncf.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:tuning] sysctl allowlist update should start a pod with custom sysctl only when the sysctl is added to whitelist": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:vlan] should create pingable pods with ipvlan interface on an in-container master [apigroup:k8s.cni.cncf.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:vlan] should create pingable pods with macvlan interface on an in-container master [apigroup:k8s.cni.cncf.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-network][Feature:vlan] should create pingable pods with vlan interface on an in-container master [apigroup:k8s.cni.cncf.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:DNSNameResolver][Feature:EgressFirewall] when using openshift ovn-kubernetes should ensure egressfirewall with wildcard dns rules is created": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:GatewayAPI][Feature:Router][apigroup:gateway.networking.k8s.io] Verify Gateway API CRDs and ensure CRD of experimental group can not be created": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:GatewayAPI][Feature:Router][apigroup:gateway.networking.k8s.io] Verify Gateway API CRDs and ensure CRD of experimental group is not installed": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:GatewayAPI][Feature:Router][apigroup:gateway.networking.k8s.io] Verify Gateway API CRDs and ensure CRD of standard group can not be created": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:GatewayAPI][Feature:Router][apigroup:gateway.networking.k8s.io] Verify Gateway API CRDs and ensure existing CRDs can not be deleted": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:GatewayAPI][Feature:Router][apigroup:gateway.networking.k8s.io] Verify Gateway API CRDs and ensure existing CRDs can not be updated": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:GatewayAPI][Feature:Router][apigroup:gateway.networking.k8s.io] Verify Gateway API CRDs and ensure required CRDs should already be installed": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkDiagnosticsConfig][Serial] Should be enabled by default": " [Suite:openshift/conformance/serial]",

	"[sig-network][OCPFeatureGate:NetworkDiagnosticsConfig][Serial] Should function without any target pods": " [Suite:openshift/conformance/serial]",

	"[sig-network][OCPFeatureGate:NetworkDiagnosticsConfig][Serial] Should move the source diagnostics pods based on the new selector and tolerations": " [Suite:openshift/conformance/serial]",

	"[sig-network][OCPFeatureGate:NetworkDiagnosticsConfig][Serial] Should move the target diagnostics pods based on the new selector and tolerations": " [Suite:openshift/conformance/serial]",

	"[sig-network][OCPFeatureGate:NetworkDiagnosticsConfig][Serial] Should remove all network diagnostics pods when disabled": " [Suite:openshift/conformance/serial]",

	"[sig-network][OCPFeatureGate:NetworkDiagnosticsConfig][Serial] Should set the condition to false if there are no nodes able to host the source pods": " [Suite:openshift/conformance/serial]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] EndpointSlices mirroring when using openshift ovn-kubernetes created using NetworkAttachmentDefinitions does not mirror EndpointSlices in namespaces not using user defined primary networks L2 dualstack primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] EndpointSlices mirroring when using openshift ovn-kubernetes created using NetworkAttachmentDefinitions does not mirror EndpointSlices in namespaces not using user defined primary networks L3 dualstack primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] EndpointSlices mirroring when using openshift ovn-kubernetes created using NetworkAttachmentDefinitions mirrors EndpointSlices managed by the default controller for namespaces with user defined primary networks L2 primary UDN, cluster-networked pods": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] EndpointSlices mirroring when using openshift ovn-kubernetes created using NetworkAttachmentDefinitions mirrors EndpointSlices managed by the default controller for namespaces with user defined primary networks L2 primary UDN, host-networked pods": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] EndpointSlices mirroring when using openshift ovn-kubernetes created using NetworkAttachmentDefinitions mirrors EndpointSlices managed by the default controller for namespaces with user defined primary networks L3 primary UDN, cluster-networked pods": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] EndpointSlices mirroring when using openshift ovn-kubernetes created using NetworkAttachmentDefinitions mirrors EndpointSlices managed by the default controller for namespaces with user defined primary networks L3 primary UDN, host-networked pods": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] EndpointSlices mirroring when using openshift ovn-kubernetes created using UserDefinedNetwork does not mirror EndpointSlices in namespaces not using user defined primary networks L2 dualstack primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] EndpointSlices mirroring when using openshift ovn-kubernetes created using UserDefinedNetwork does not mirror EndpointSlices in namespaces not using user defined primary networks L3 dualstack primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] EndpointSlices mirroring when using openshift ovn-kubernetes created using UserDefinedNetwork mirrors EndpointSlices managed by the default controller for namespaces with user defined primary networks L2 primary UDN, cluster-networked pods": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] EndpointSlices mirroring when using openshift ovn-kubernetes created using UserDefinedNetwork mirrors EndpointSlices managed by the default controller for namespaces with user defined primary networks L2 primary UDN, host-networked pods": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] EndpointSlices mirroring when using openshift ovn-kubernetes created using UserDefinedNetwork mirrors EndpointSlices managed by the default controller for namespaces with user defined primary networks L3 primary UDN, cluster-networked pods": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] EndpointSlices mirroring when using openshift ovn-kubernetes created using UserDefinedNetwork mirrors EndpointSlices managed by the default controller for namespaces with user defined primary networks L3 primary UDN, host-networked pods": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] Network Policies when using openshift ovn-kubernetes allow ingress traffic to one pod from a particular namespace in L2 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] Network Policies when using openshift ovn-kubernetes allow ingress traffic to one pod from a particular namespace in L3 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] Network Policies when using openshift ovn-kubernetes pods within namespace should be isolated when deny policy is present in L2 dualstack primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] Network Policies when using openshift ovn-kubernetes pods within namespace should be isolated when deny policy is present in L3 dualstack primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes ClusterUserDefinedNetwork CRD Controller pod connected to ClusterUserDefinedNetwork CR & managed NADs cannot be deleted when being used": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes ClusterUserDefinedNetwork CRD Controller should create NAD according to spec in each target namespace and report active namespaces": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes ClusterUserDefinedNetwork CRD Controller should create NAD in new created namespaces that apply to namespace-selector": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes ClusterUserDefinedNetwork CRD Controller when CR is deleted, should delete all managed NAD in each target namespace": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes ClusterUserDefinedNetwork CRD Controller when namespace-selector is mutated should create NAD in namespaces that apply to mutated namespace-selector": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes ClusterUserDefinedNetwork CRD Controller when namespace-selector is mutated should delete managed NAD in namespaces that no longer apply to namespace-selector": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes UDN Pod should react to k8s.ovn.org/open-default-ports annotations changes": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes UserDefinedNetwork CRD controller pod connected to UserDefinedNetwork cannot be deleted when being used": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes UserDefinedNetwork CRD controller should create NetworkAttachmentDefinition according to spec": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes UserDefinedNetwork CRD controller should delete NetworkAttachmentDefinition when UserDefinedNetwork is deleted": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using ClusterUserDefinedNetwork can perform east/west traffic between nodes for two pods connected over a L2 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using ClusterUserDefinedNetwork can perform east/west traffic between nodes two pods connected over a L3 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using ClusterUserDefinedNetwork is isolated from the default network with L2 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using ClusterUserDefinedNetwork is isolated from the default network with L3 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using ClusterUserDefinedNetwork isolates overlapping CIDRs with L2 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using ClusterUserDefinedNetwork isolates overlapping CIDRs with L3 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using NetworkAttachmentDefinitions can perform east/west traffic between nodes for two pods connected over a L2 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using NetworkAttachmentDefinitions can perform east/west traffic between nodes two pods connected over a L3 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using NetworkAttachmentDefinitions is isolated from the default network with L2 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using NetworkAttachmentDefinitions is isolated from the default network with L3 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using NetworkAttachmentDefinitions isolates overlapping CIDRs with L2 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using NetworkAttachmentDefinitions isolates overlapping CIDRs with L3 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using UserDefinedNetwork can perform east/west traffic between nodes for two pods connected over a L2 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using UserDefinedNetwork can perform east/west traffic between nodes two pods connected over a L3 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using UserDefinedNetwork is isolated from the default network with L2 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using UserDefinedNetwork is isolated from the default network with L3 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using UserDefinedNetwork isolates overlapping CIDRs with L2 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes created using UserDefinedNetwork isolates overlapping CIDRs with L3 primary UDN": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes when primary network exist, ClusterUserDefinedNetwork status should report not-ready": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] when using openshift ovn-kubernetes when primary network exist, UserDefinedNetwork status should report not-ready": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using NetworkAttachmentDefinitions [Suite:openshift/network/virtualization] should keep ip [OCPFeatureGate:NetworkSegmentation] when the VM attached to a primary UDN is migrated between nodes": "",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using NetworkAttachmentDefinitions [Suite:openshift/network/virtualization] should keep ip [OCPFeatureGate:NetworkSegmentation] when the VM attached to a primary UDN is restarted": "",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using NetworkAttachmentDefinitions [Suite:openshift/network/virtualization] should keep ip [OCPFeatureGate:NetworkSegmentation] when the VMI attached to a primary UDN is migrated between nodes": "",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using NetworkAttachmentDefinitions [Suite:openshift/network/virtualization] should keep ip [OCPFeatureGate:PreconfiguredUDNAddresses] when the VM with preconfigured IP and MAC attached to a primary UDN is migrated between nodes": "",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using NetworkAttachmentDefinitions [Suite:openshift/network/virtualization] should keep ip [OCPFeatureGate:PreconfiguredUDNAddresses] when the VM with preconfigured IPs attached to a primary UDN is restarted": "",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using NetworkAttachmentDefinitions [Suite:openshift/network/virtualization] should keep ip [OCPFeatureGate:PreconfiguredUDNAddresses] when the VM with preconfigured MAC attached to a primary UDN is restarted": "",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using NetworkAttachmentDefinitions [Suite:openshift/network/virtualization] should keep ip when the VM attached to a secondary UDN is migrated between nodes": "",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using NetworkAttachmentDefinitions [Suite:openshift/network/virtualization] should keep ip when the VM attached to a secondary UDN is restarted": "",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using NetworkAttachmentDefinitions [Suite:openshift/network/virtualization] should keep ip when the VMI attached to a secondary UDN is migrated between nodes": "",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using [OCPFeatureGate:NetworkSegmentation] UserDefinedNetwork [Suite:openshift/network/virtualization] should keep ip [OCPFeatureGate:NetworkSegmentation] when the VM attached to a primary UDN is migrated between nodes": "",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using [OCPFeatureGate:NetworkSegmentation] UserDefinedNetwork [Suite:openshift/network/virtualization] should keep ip [OCPFeatureGate:NetworkSegmentation] when the VM attached to a primary UDN is restarted": "",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using [OCPFeatureGate:NetworkSegmentation] UserDefinedNetwork [Suite:openshift/network/virtualization] should keep ip [OCPFeatureGate:NetworkSegmentation] when the VMI attached to a primary UDN is migrated between nodes": "",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using [OCPFeatureGate:NetworkSegmentation] UserDefinedNetwork [Suite:openshift/network/virtualization] should keep ip [OCPFeatureGate:PreconfiguredUDNAddresses] when the VM with preconfigured IP and MAC attached to a primary UDN is migrated between nodes": "",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using [OCPFeatureGate:NetworkSegmentation] UserDefinedNetwork [Suite:openshift/network/virtualization] should keep ip [OCPFeatureGate:PreconfiguredUDNAddresses] when the VM with preconfigured IPs attached to a primary UDN is restarted": "",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using [OCPFeatureGate:NetworkSegmentation] UserDefinedNetwork [Suite:openshift/network/virtualization] should keep ip [OCPFeatureGate:PreconfiguredUDNAddresses] when the VM with preconfigured MAC attached to a primary UDN is restarted": "",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using [OCPFeatureGate:NetworkSegmentation] UserDefinedNetwork [Suite:openshift/network/virtualization] should keep ip when the VM attached to a secondary UDN is migrated between nodes": "",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using [OCPFeatureGate:NetworkSegmentation] UserDefinedNetwork [Suite:openshift/network/virtualization] should keep ip when the VM attached to a secondary UDN is restarted": "",

	"[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration] Kubevirt Virtual Machines when using openshift ovn-kubernetes with user defined networks and persistent ips configured created using [OCPFeatureGate:NetworkSegmentation] UserDefinedNetwork [Suite:openshift/network/virtualization] should keep ip when the VMI attached to a secondary UDN is migrated between nodes": "",

	"[sig-network][OCPFeatureGate:RouteAdvertisements][Feature:RouteAdvertisements][apigroup:operator.openshift.io] when using openshift ovn-kubernetes [EgressIP] Advertising EgressIP [apigroup:user.openshift.io][apigroup:security.openshift.io] For cluster user defined networks When the network topology is Layer 3 UDN pods should have the assigned EgressIPs and EgressIPs can be created, updated and deleted [apigroup:route.openshift.io] When the network is IPv4": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteAdvertisements][Feature:RouteAdvertisements][apigroup:operator.openshift.io] when using openshift ovn-kubernetes [EgressIP] Advertising EgressIP [apigroup:user.openshift.io][apigroup:security.openshift.io] For cluster user defined networks When the network topology is Layer 3 UDN pods should have the assigned EgressIPs and EgressIPs can be created, updated and deleted [apigroup:route.openshift.io] When the network is IPv6": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteAdvertisements][Feature:RouteAdvertisements][apigroup:operator.openshift.io] when using openshift ovn-kubernetes [EgressIP] Advertising EgressIP [apigroup:user.openshift.io][apigroup:security.openshift.io] For the default network Pods should have the assigned EgressIPs and EgressIPs can be created, updated and deleted [apigroup:route.openshift.io] When the network is IPv4": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteAdvertisements][Feature:RouteAdvertisements][apigroup:operator.openshift.io] when using openshift ovn-kubernetes [EgressIP] Advertising EgressIP [apigroup:user.openshift.io][apigroup:security.openshift.io] For the default network Pods should have the assigned EgressIPs and EgressIPs can be created, updated and deleted [apigroup:route.openshift.io] When the network is IPv6": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteAdvertisements][Feature:RouteAdvertisements][apigroup:operator.openshift.io] when using openshift ovn-kubernetes [PodNetwork] Advertising a cluster user defined network [apigroup:user.openshift.io][apigroup:security.openshift.io] Over a VRF-Lite configuration Pods should be able to communicate on a secondary network [Timeout:30m]": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteAdvertisements][Feature:RouteAdvertisements][apigroup:operator.openshift.io] when using openshift ovn-kubernetes [PodNetwork] Advertising a cluster user defined network [apigroup:user.openshift.io][apigroup:security.openshift.io] Over the default VRF When the network topology is Layer 2 External host should be able to query route advertised pods by the pod IP": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteAdvertisements][Feature:RouteAdvertisements][apigroup:operator.openshift.io] when using openshift ovn-kubernetes [PodNetwork] Advertising a cluster user defined network [apigroup:user.openshift.io][apigroup:security.openshift.io] Over the default VRF When the network topology is Layer 2 Pods should communicate with external host without being SNATed": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteAdvertisements][Feature:RouteAdvertisements][apigroup:operator.openshift.io] when using openshift ovn-kubernetes [PodNetwork] Advertising a cluster user defined network [apigroup:user.openshift.io][apigroup:security.openshift.io] Over the default VRF When the network topology is Layer 3 External host should be able to query route advertised pods by the pod IP": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteAdvertisements][Feature:RouteAdvertisements][apigroup:operator.openshift.io] when using openshift ovn-kubernetes [PodNetwork] Advertising a cluster user defined network [apigroup:user.openshift.io][apigroup:security.openshift.io] Over the default VRF When the network topology is Layer 3 Pods should communicate with external host without being SNATed": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteAdvertisements][Feature:RouteAdvertisements][apigroup:operator.openshift.io] when using openshift ovn-kubernetes [PodNetwork] Advertising the default network [apigroup:user.openshift.io][apigroup:security.openshift.io] External host should be able to query route advertised pods by the pod IP": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteAdvertisements][Feature:RouteAdvertisements][apigroup:operator.openshift.io] when using openshift ovn-kubernetes [PodNetwork] Advertising the default network [apigroup:user.openshift.io][apigroup:security.openshift.io] Pods should communicate with external host without being SNATed": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with invalid setup the router should not support external certificate if inline certificate is also present": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with invalid setup the router should not support external certificate if the route termination type is Passthrough": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with invalid setup the router should not support external certificate if the secret is in a different namespace": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with invalid setup the router should not support external certificate if the secret is not of type kubernetes.io/tls": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with invalid setup the router should not support external certificate without proper permissions": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with valid setup the router should support external certificate and routes are reachable": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with valid setup the router should support external certificate and the route is updated to remove the external certificate and again re-add the same external certificate then also the route is reachable": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with valid setup the router should support external certificate and the route is updated to remove the external certificate then also the route is reachable and serves the default certificate": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with valid setup the router should support external certificate and the route is updated to use new external certificate then also the route is reachable": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with valid setup the router should support external certificate and the route is updated to use new external certificate, but RBAC permissions are not added route update is rejected": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with valid setup the router should support external certificate and the route is updated to use new external certificate, but secret does not exist route update is rejected": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with valid setup the router should support external certificate and the route is updated to use new external certificate, but secret is not of type kubernetes.io/tls route update is rejected": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with valid setup the router should support external certificate and the route is updated to use same external certificate then also the route is reachable": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with valid setup the router should support external certificate and the route is updated to use same external certificate, but RBAC permissions are dropped route update is rejected": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with valid setup the router should support external certificate and the secret is deleted and re-created again but RBAC permissions are dropped then routes are not reachable": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with valid setup the router should support external certificate and the secret is deleted and re-created again then routes are reachable": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with valid setup the router should support external certificate and the secret is deleted then routes are not reachable": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with valid setup the router should support external certificate and the secret is updated but RBAC permissions are dropped then routes are not reachable": " [Suite:openshift/conformance/parallel]",

	"[sig-network][OCPFeatureGate:RouteExternalCertificate][Feature:Router][apigroup:route.openshift.io] with valid setup the router should support external certificate and the secret is updated then also routes are reachable": " [Suite:openshift/conformance/parallel]",

	"[sig-network][endpoints] admission [apigroup:config.openshift.io] blocks manual creation of EndpointSlices pointing to the cluster or service network": " [Suite:openshift/conformance/parallel]",

	"[sig-network][endpoints] admission [apigroup:config.openshift.io] blocks manual creation of Endpoints pointing to the cluster or service network": " [Suite:openshift/conformance/parallel]",

	"[sig-node-tuning] NTO should OCP-66086 NTO Prevent from stalld continually restarting [Slow]": "",

	"[sig-node] Managed cluster record the number of nodes at the beginning of the tests [Early]": " [Suite:openshift/conformance/parallel]",

	"[sig-node] Managed cluster should report ready nodes the entire duration of the test run [Late][apigroup:monitoring.coreos.com]": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-node] Managed cluster should verify that nodes have no unexpected reboots [Late]": " [Suite:openshift/conformance/parallel]",

	"[sig-node] [Conformance] Prevent openshift node labeling on update by the node TestOpenshiftNodeLabeling": " [Suite:openshift/conformance/parallel/minimal]",

	"[sig-node] [FeatureGate:ImageVolume] ImageVolume should fail when image does not exist": " [Suite:openshift/conformance/parallel]",

	"[sig-node] [FeatureGate:ImageVolume] ImageVolume should handle multiple image volumes": " [Suite:openshift/conformance/parallel]",

	"[sig-node] [FeatureGate:ImageVolume] ImageVolume should succeed if image volume is not existing but unused": " [Suite:openshift/conformance/parallel]",

	"[sig-node] [FeatureGate:ImageVolume] ImageVolume should succeed with multiple pods and same image on the same node": " [Suite:openshift/conformance/parallel]",

	"[sig-node] [FeatureGate:ImageVolume] ImageVolume should succeed with pod and pull policy of Always": " [Suite:openshift/conformance/parallel]",

	"[sig-node] [FeatureGate:ImageVolume] ImageVolume when subPath is used should fail to mount image volume with invalid subPath": " [Suite:openshift/conformance/parallel]",

	"[sig-node] [FeatureGate:ImageVolume] ImageVolume when subPath is used should handle image volume with subPath": " [Suite:openshift/conformance/parallel]",

	"[sig-node] should override timeoutGracePeriodSeconds when annotation is set": " [Suite:openshift/conformance/parallel]",

	"[sig-node] supplemental groups Ensure supplemental groups propagate to docker should propagate requested groups to the container [apigroup:security.openshift.io]": " [Suite:openshift/conformance/parallel]",

	"[sig-node][Disruptive][Feature:KubeletGracefulShutdown] Kubelet with graceful shutdown configuration should respect pods termination grace period": " [Serial]",

	"[sig-node][Late] should not have pod creation failures due to systemd timeouts": " [Suite:openshift/conformance/parallel]",

	"[sig-node][Suite:openshift/nodes/realtime/latency][Disruptive] Real time kernel should meet latency requirements when tested with cyclictest": " [Serial]",

	"[sig-node][Suite:openshift/nodes/realtime/latency][Disruptive] Real time kernel should meet latency requirements when tested with hwlatdetect": " [Serial]",

	"[sig-node][Suite:openshift/nodes/realtime/latency][Disruptive] Real time kernel should meet latency requirements when tested with oslat": " [Serial]",

	"[sig-node][Suite:openshift/nodes/realtime][Disruptive] Real time kernel should allow deadline_test to run successfully": " [Serial]",

	"[sig-node][Suite:openshift/nodes/realtime][Disruptive] Real time kernel should allow pi_stress to run successfully with the fifo algorithm": " [Serial]",

	"[sig-node][Suite:openshift/nodes/realtime][Disruptive] Real time kernel should allow pi_stress to run successfully with the round robin algorithm": " [Serial]",

	"[sig-node][apigroup:config.openshift.io] CPU Partitioning cluster infrastructure should be configured correctly": " [Suite:openshift/conformance/parallel]",

	"[sig-node][apigroup:config.openshift.io] CPU Partitioning cluster platform workloads should be annotated correctly for DaemonSets": " [Suite:openshift/conformance/parallel]",

	"[sig-node][apigroup:config.openshift.io] CPU Partitioning cluster platform workloads should be annotated correctly for Deployments": " [Suite:openshift/conformance/parallel]",

	"[sig-node][apigroup:config.openshift.io] CPU Partitioning cluster workloads in annotated namespaces should be modified if CPUPartitioningMode = AllNodes": " [Suite:openshift/conformance/parallel]",

	"[sig-node][apigroup:config.openshift.io] CPU Partitioning cluster workloads in non-annotated namespaces should be allowed if CPUPartitioningMode = AllNodes with a warning annotation": " [Suite:openshift/conformance/parallel]",

	"[sig-node][apigroup:config.openshift.io] CPU Partitioning cluster workloads with limits should have resources modified if CPUPartitioningMode = AllNodes": " [Suite:openshift/conformance/parallel]",

	"[sig-node][apigroup:config.openshift.io] CPU Partitioning node validation should have correct cpuset and cpushare set in crio containers": " [Suite:openshift/conformance/parallel]",

	"[sig-node][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node] Two Node with Fencing topology Should validate the number of control-planes, arbiters as configured": "",

	"[sig-node][apigroup:config.openshift.io][OCPFeatureGate:HighlyAvailableArbiter] expected Master and Arbiter node counts Should validate that there are Master and Arbiter nodes as specified in the cluster": " [Suite:openshift/conformance/parallel]",

	"[sig-node][apigroup:config.openshift.io][OCPFeatureGate:HighlyAvailableArbiter] required pods on the Arbiter node Should verify that the correct number of pods are running on the Arbiter node": " [Suite:openshift/conformance/parallel]",

	"[sig-operator] OLM should Implement packages API server and list packagemanifest info with namespace not NULL [apigroup:packages.operators.coreos.com]": " [Skipped:NoOptionalCapabilities] [Suite:openshift/conformance/parallel]",

	"[sig-operator] OLM should be installed with catalogsources at version v1alpha1 [apigroup:operators.coreos.com]": " [Skipped:NoOptionalCapabilities] [Suite:openshift/conformance/parallel]",

	"[sig-operator] OLM should be installed with clusterserviceversions at version v1alpha1 [apigroup:operators.coreos.com]": " [Skipped:NoOptionalCapabilities] [Suite:openshift/conformance/parallel]",

	"[sig-operator] OLM should be installed with installplans at version v1alpha1 [apigroup:operators.coreos.com]": " [Skipped:NoOptionalCapabilities] [Suite:openshift/conformance/parallel]",

	"[sig-operator] OLM should be installed with operatorgroups at version v1 [apigroup:operators.coreos.com]": " [Skipped:NoOptionalCapabilities] [Suite:openshift/conformance/parallel]",

	"[sig-operator] OLM should be installed with packagemanifests at version v1 [apigroup:packages.operators.coreos.com]": " [Skipped:NoOptionalCapabilities] [Suite:openshift/conformance/parallel]",

	"[sig-operator] OLM should be installed with subscriptions at version v1alpha1 [apigroup:operators.coreos.com]": " [Skipped:NoOptionalCapabilities] [Suite:openshift/conformance/parallel]",

	"[sig-operator] OLM should have imagePullPolicy:IfNotPresent on thier deployments": " [Skipped:NoOptionalCapabilities] [Suite:openshift/conformance/parallel]",

	"[sig-operator] an end user can use OLM Report Upgradeable in OLM ClusterOperators status [apigroup:config.openshift.io]": " [Skipped:NoOptionalCapabilities] [Suite:openshift/conformance/parallel]",

	"[sig-operator] an end user can use OLM can subscribe to the operator [apigroup:config.openshift.io]": " [Skipped:Disconnected] [Skipped:NoOptionalCapabilities] [Suite:openshift/conformance/parallel]",

	"[sig-scalability][Feature:Performance] Load cluster should populate the cluster [Slow][Serial][apigroup:template.openshift.io][apigroup:apps.openshift.io][apigroup:build.openshift.io]": "",

	"[sig-scalability][Feature:Performance][Serial][Slow] Load cluster concurrently with templates": "",

	"[sig-scalability][Feature:Performance][Serial][Slow] Mirror cluster it should read the cluster apps [apigroup:apps.openshift.io]": "",

	"[sig-scalability][Feature:Performance][Serial][Slow] Mirror cluster it should read the node info": "",

	"[sig-scheduling][Early] The HAProxy router pods [apigroup:route.openshift.io] should be scheduled on different nodes": " [Skipped:SingleReplicaTopology] [Suite:openshift/conformance/parallel]",

	"[sig-scheduling][Early] The openshift-apiserver pods [apigroup:authorization.openshift.io][apigroup:build.openshift.io][apigroup:image.openshift.io][apigroup:project.openshift.io][apigroup:quota.openshift.io][apigroup:route.openshift.io][apigroup:security.openshift.io][apigroup:template.openshift.io] should be scheduled on different nodes": " [Skipped:SingleReplicaTopology] [Suite:openshift/conformance/parallel]",

	"[sig-scheduling][Early] The openshift-authentication pods [apigroup:oauth.openshift.io] should be scheduled on different nodes": " [Skipped:SingleReplicaTopology] [Suite:openshift/conformance/parallel]",

	"[sig-scheduling][Early] The openshift-console console pods [apigroup:console.openshift.io] should be scheduled on different nodes": " [Skipped:SingleReplicaTopology] [Suite:openshift/conformance/parallel]",

	"[sig-scheduling][Early] The openshift-console downloads pods [apigroup:console.openshift.io] should be scheduled on different nodes": " [Skipped:SingleReplicaTopology] [Suite:openshift/conformance/parallel]",

	"[sig-scheduling][Early] The openshift-etcd pods [apigroup:operator.openshift.io] should be scheduled on different nodes": " [Skipped:SingleReplicaTopology] [Suite:openshift/conformance/parallel]",

	"[sig-scheduling][Early] The openshift-image-registry pods [apigroup:imageregistry.operator.openshift.io] should be scheduled on different nodes": " [Skipped:SingleReplicaTopology] [Suite:openshift/conformance/parallel]",

	"[sig-scheduling][Early] The openshift-monitoring prometheus-adapter pods [apigroup:monitoring.coreos.com] should be scheduled on different nodes": " [Skipped:SingleReplicaTopology] [Suite:openshift/conformance/parallel]",

	"[sig-scheduling][Early] The openshift-monitoring thanos-querier pods [apigroup:monitoring.coreos.com] should be scheduled on different nodes": " [Skipped:SingleReplicaTopology] [Suite:openshift/conformance/parallel]",

	"[sig-scheduling][Early] The openshift-oauth-apiserver pods [apigroup:oauth.openshift.io][apigroup:user.openshift.io] should be scheduled on different nodes": " [Skipped:SingleReplicaTopology] [Suite:openshift/conformance/parallel]",

	"[sig-scheduling][Early] The openshift-operator-lifecycle-manager pods [apigroup:packages.operators.coreos.com] should be scheduled on different nodes": " [Skipped:SingleReplicaTopology] [Suite:openshift/conformance/parallel]",

	"[sig-storage] Managed cluster should have no crashlooping recycler pods over four minutes": " [Suite:openshift/conformance/parallel]",

	"[sig-storage][Feature:CSIInlineVolumeAdmission][Serial] baseline namespace should allow pods with inline volumes when the driver uses the baseline label": " [Suite:openshift/conformance/serial]",

	"[sig-storage][Feature:CSIInlineVolumeAdmission][Serial] baseline namespace should allow pods with inline volumes when the driver uses the restricted label": " [Suite:openshift/conformance/serial]",

	"[sig-storage][Feature:CSIInlineVolumeAdmission][Serial] baseline namespace should deny pods with inline volumes when the driver uses the privileged label": " [Suite:openshift/conformance/serial]",

	"[sig-storage][Feature:CSIInlineVolumeAdmission][Serial] privileged namespace should allow pods with inline volumes when the driver uses the privileged label": " [Suite:openshift/conformance/serial]",

	"[sig-storage][Feature:CSIInlineVolumeAdmission][Serial] privileged namespace should allow pods with inline volumes when the driver uses the restricted label": " [Suite:openshift/conformance/serial]",

	"[sig-storage][Feature:CSIInlineVolumeAdmission][Serial] restricted namespace should allow pods with inline volumes when the driver uses the restricted label": " [Suite:openshift/conformance/serial]",

	"[sig-storage][Feature:CSIInlineVolumeAdmission][Serial] restricted namespace should deny pods with inline volumes when the driver uses the baseline label": " [Suite:openshift/conformance/serial]",

	"[sig-storage][Feature:CSIInlineVolumeAdmission][Serial] restricted namespace should deny pods with inline volumes when the driver uses the privileged label": " [Suite:openshift/conformance/serial]",

	"[sig-storage][Feature:DisableStorageClass][Serial][apigroup:operator.openshift.io] should not reconcile the StorageClass when StorageClassState is Unmanaged": " [Suite:openshift/conformance/serial]",

	"[sig-storage][Feature:DisableStorageClass][Serial][apigroup:operator.openshift.io] should reconcile the StorageClass when StorageClassState is Managed": " [Suite:openshift/conformance/serial]",

	"[sig-storage][Feature:DisableStorageClass][Serial][apigroup:operator.openshift.io] should remove the StorageClass when StorageClassState is Removed": " [Suite:openshift/conformance/serial]",

	"[sig-storage][FeatureGate:VSphereDriverConfiguration][Serial][apigroup:operator.openshift.io] vSphere CSI Driver Configuration snapshot options in clusterCSIDriver should allow all limits to be set at once": " [Suite:openshift/conformance/serial]",

	"[sig-storage][FeatureGate:VSphereDriverConfiguration][Serial][apigroup:operator.openshift.io] vSphere CSI Driver Configuration snapshot options in clusterCSIDriver should allow setting VSAN limit": " [Suite:openshift/conformance/serial]",

	"[sig-storage][FeatureGate:VSphereDriverConfiguration][Serial][apigroup:operator.openshift.io] vSphere CSI Driver Configuration snapshot options in clusterCSIDriver should allow setting VVOL limit": " [Suite:openshift/conformance/serial]",

	"[sig-storage][FeatureGate:VSphereDriverConfiguration][Serial][apigroup:operator.openshift.io] vSphere CSI Driver Configuration snapshot options in clusterCSIDriver should allow setting global snapshot limit": " [Suite:openshift/conformance/serial]",

	"[sig-storage][FeatureGate:VSphereDriverConfiguration][Serial][apigroup:operator.openshift.io] vSphere CSI Driver Configuration snapshot options in clusterCSIDriver should use default when unset": " [Suite:openshift/conformance/serial]",

	"[sig-storage][Late] Metrics should report short attach times": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-storage][Late] Metrics should report short mount times": " [Skipped:Disconnected] [Suite:openshift/conformance/parallel]",

	"[sig-windows] Nodes should fail with invalid version annotation": " [Suite:openshift/conformance/parallel]",
}

func init() {
	ginkgo.GetSuite().SetAnnotateFn(func(name string, node types.TestSpec) {
		if newLabels, ok := Annotations[name]; ok {
			node.AppendText(newLabels)
		} else {
			panic(fmt.Sprintf("unable to find test %s", name))
		}
	})
}
