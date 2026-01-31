package features

import (
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
)

func FeatureSets(clusterProfile ClusterProfileName, featureSet configv1.FeatureSet) (*FeatureGateEnabledDisabled, error) {
	byFeatureSet, ok := allFeatureGates[clusterProfile]
	if !ok {
		return nil, fmt.Errorf("no information found for ClusterProfile=%q", clusterProfile)
	}
	featureGates, ok := byFeatureSet[featureSet]
	if !ok {
		return nil, fmt.Errorf("no information found for FeatureSet=%q under ClusterProfile=%q", featureSet, clusterProfile)
	}
	return featureGates.DeepCopy(), nil
}

func AllFeatureSets() map[ClusterProfileName]map[configv1.FeatureSet]*FeatureGateEnabledDisabled {
	ret := map[ClusterProfileName]map[configv1.FeatureSet]*FeatureGateEnabledDisabled{}

	for clusterProfile, byFeatureSet := range allFeatureGates {
		newByFeatureSet := map[configv1.FeatureSet]*FeatureGateEnabledDisabled{}

		for featureSet, enabledDisabled := range byFeatureSet {
			newByFeatureSet[featureSet] = enabledDisabled.DeepCopy()
		}
		ret[clusterProfile] = newByFeatureSet
	}

	return ret
}

var (
	allFeatureGates = map[ClusterProfileName]map[configv1.FeatureSet]*FeatureGateEnabledDisabled{}

	FeatureGateConsolePluginCSP = newFeatureGate("ConsolePluginContentSecurityPolicy").
					reportProblemsToJiraComponent("Management Console").
					contactPerson("jhadvig").
					productScope(ocpSpecific).
					enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					enhancementPR("https://github.com/openshift/enhancements/pull/1706").
					mustRegister()

	FeatureGateServiceAccountTokenNodeBinding = newFeatureGate("ServiceAccountTokenNodeBinding").
							reportProblemsToJiraComponent("apiserver-auth").
							contactPerson("ibihim").
							productScope(kubernetes).
							enhancementPR("https://github.com/kubernetes/enhancements/issues/4193").
							enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
							mustRegister()

	FeatureGateMutatingAdmissionPolicy = newFeatureGate("MutatingAdmissionPolicy").
						reportProblemsToJiraComponent("kube-apiserver").
						contactPerson("benluddy").
						productScope(kubernetes).
						enhancementPR("https://github.com/kubernetes/enhancements/issues/3962").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateGatewayAPI = newFeatureGate("GatewayAPI").
				reportProblemsToJiraComponent("Routing").
				contactPerson("miciah").
				productScope(ocpSpecific).
				enhancementPR(legacyFeatureGateWithoutEnhancement).
				enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
				mustRegister()

	FeatureGateOpenShiftPodSecurityAdmission = newFeatureGate("OpenShiftPodSecurityAdmission").
							reportProblemsToJiraComponent("auth").
							contactPerson("ibihim").
							productScope(ocpSpecific).
							enhancementPR("https://github.com/openshift/enhancements/pull/899").
							enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
							mustRegister()

	FeatureGateBuildCSIVolumes = newFeatureGate("BuildCSIVolumes").
					reportProblemsToJiraComponent("builds").
					contactPerson("adkaplan").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateAzureWorkloadIdentity = newFeatureGate("AzureWorkloadIdentity").
						reportProblemsToJiraComponent("cloud-credential-operator").
						contactPerson("abutcher").
						productScope(ocpSpecific).
						enhancementPR(legacyFeatureGateWithoutEnhancement).
						enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateAzureDedicatedHosts = newFeatureGate("AzureDedicatedHosts").
					reportProblemsToJiraComponent("installer").
					contactPerson("rvanderp3").
					productScope(ocpSpecific).
					enhancementPR("https://github.com/openshift/enhancements/pull/1783").
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateMaxUnavailableStatefulSet = newFeatureGate("MaxUnavailableStatefulSet").
						reportProblemsToJiraComponent("apps").
						contactPerson("atiratree").
						productScope(kubernetes).
						enhancementPR("https://github.com/kubernetes/enhancements/issues/961").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateEventedPLEG = newFeatureGate("EventedPLEG").
				reportProblemsToJiraComponent("node").
				contactPerson("sairameshv").
				productScope(kubernetes).
				enhancementPR("https://github.com/kubernetes/enhancements/issues/3386").
				mustRegister()

	FeatureGateSigstoreImageVerification = newFeatureGate("SigstoreImageVerification").
						reportProblemsToJiraComponent("node").
						contactPerson("sgrunert").
						productScope(ocpSpecific).
						enhancementPR(legacyFeatureGateWithoutEnhancement).
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade, configv1.Default, configv1.OKD).
						mustRegister()

	FeatureGateSigstoreImageVerificationPKI = newFeatureGate("SigstoreImageVerificationPKI").
						reportProblemsToJiraComponent("node").
						contactPerson("QiWang").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1658").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade, configv1.Default, configv1.OKD).
						mustRegister()

	FeatureGateAlibabaPlatform = newFeatureGate("AlibabaPlatform").
					reportProblemsToJiraComponent("cloud-provider").
					contactPerson("jspeed").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateVSphereHostVMGroupZonal = newFeatureGate("VSphereHostVMGroupZonal").
						reportProblemsToJiraComponent("splat").
						contactPerson("jcpowermac").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1677").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateVSphereMultiDisk = newFeatureGate("VSphereMultiDisk").
					reportProblemsToJiraComponent("splat").
					contactPerson("vr4manta").
					productScope(ocpSpecific).
					enhancementPR("https://github.com/openshift/enhancements/pull/1709").
					enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateRouteExternalCertificate = newFeatureGate("RouteExternalCertificate").
						reportProblemsToJiraComponent("router").
						contactPerson("chiragkyal").
						productScope(ocpSpecific).
						enhancementPR(legacyFeatureGateWithoutEnhancement).
						enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateCPMSMachineNamePrefix = newFeatureGate("CPMSMachineNamePrefix").
						reportProblemsToJiraComponent("Cloud Compute / ControlPlaneMachineSet").
						contactPerson("chiragkyal").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1714").
						enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateAdminNetworkPolicy = newFeatureGate("AdminNetworkPolicy").
					reportProblemsToJiraComponent("Networking/ovn-kubernetes").
					contactPerson("tssurya").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateNetworkSegmentation = newFeatureGate("NetworkSegmentation").
					reportProblemsToJiraComponent("Networking/ovn-kubernetes").
					contactPerson("tssurya").
					productScope(ocpSpecific).
					enhancementPR("https://github.com/openshift/enhancements/pull/1623").
					enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateNetworkConnect = newFeatureGate("NetworkConnect").
					reportProblemsToJiraComponent("Networking/ovn-kubernetes").
					contactPerson("tssurya").
					productScope(ocpSpecific).
					enhancementPR("https://github.com/ovn-kubernetes/ovn-kubernetes/pull/5246").
					enableIn(configv1.DevPreviewNoUpgrade).
					mustRegister()

	FeatureGateAdditionalRoutingCapabilities = newFeatureGate("AdditionalRoutingCapabilities").
							reportProblemsToJiraComponent("Networking/cluster-network-operator").
							contactPerson("jcaamano").
							productScope(ocpSpecific).
							enhancementPR(legacyFeatureGateWithoutEnhancement).
							enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
							mustRegister()

	FeatureGateRouteAdvertisements = newFeatureGate("RouteAdvertisements").
					reportProblemsToJiraComponent("Networking/ovn-kubernetes").
					contactPerson("jcaamano").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateEVPN = newFeatureGate("EVPN").
			reportProblemsToJiraComponent("Networking/ovn-kubernetes").
			contactPerson("jcaamano").
			productScope(ocpSpecific).
			enhancementPR("https://github.com/openshift/enhancements/pull/1862").
			enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
			mustRegister()

	FeatureGateNetworkLiveMigration = newFeatureGate("NetworkLiveMigration").
					reportProblemsToJiraComponent("Networking/ovn-kubernetes").
					contactPerson("pliu").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateNetworkDiagnosticsConfig = newFeatureGate("NetworkDiagnosticsConfig").
						reportProblemsToJiraComponent("Networking/cluster-network-operator").
						contactPerson("kyrtapz").
						productScope(ocpSpecific).
						enhancementPR(legacyFeatureGateWithoutEnhancement).
						enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateOVNObservability = newFeatureGate("OVNObservability").
					reportProblemsToJiraComponent("Networking").
					contactPerson("npinaeva").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateBackendQuotaGiB = newFeatureGate("EtcdBackendQuota").
					reportProblemsToJiraComponent("etcd").
					contactPerson("hasbro17").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateAutomatedEtcdBackup = newFeatureGate("AutomatedEtcdBackup").
					reportProblemsToJiraComponent("etcd").
					contactPerson("hasbro17").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateMachineAPIOperatorDisableMachineHealthCheckController = newFeatureGate("MachineAPIOperatorDisableMachineHealthCheckController").
										reportProblemsToJiraComponent("ecoproject").
										contactPerson("msluiter").
										productScope(ocpSpecific).
										enhancementPR(legacyFeatureGateWithoutEnhancement).
										mustRegister()

	FeatureGateDNSNameResolver = newFeatureGate("DNSNameResolver").
					reportProblemsToJiraComponent("dns").
					contactPerson("miciah").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateMachineConfigNodes = newFeatureGate("MachineConfigNodes").
					reportProblemsToJiraComponent("MachineConfigOperator").
					contactPerson("ijanssen").
					productScope(ocpSpecific).
					enhancementPR("https://github.com/openshift/enhancements/pull/1765").
					enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateImageModeStatusReporting = newFeatureGate("ImageModeStatusReporting").
						reportProblemsToJiraComponent("MachineConfigOperator").
						contactPerson("ijanssen").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1809").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateClusterAPIInstall = newFeatureGate("ClusterAPIInstall").
					reportProblemsToJiraComponent("Installer").
					contactPerson("vincepri").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					mustRegister()

	FeatureGateGCPClusterHostedDNS = newFeatureGate("GCPClusterHostedDNS").
					reportProblemsToJiraComponent("Installer").
					contactPerson("barbacbd").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateAWSClusterHostedDNS = newFeatureGate("AWSClusterHostedDNS").
					reportProblemsToJiraComponent("Installer").
					contactPerson("barbacbd").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateAzureClusterHostedDNSInstall = newFeatureGate("AzureClusterHostedDNSInstall").
						reportProblemsToJiraComponent("Installer").
						contactPerson("sadasu").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1468").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateMixedCPUsAllocation = newFeatureGate("MixedCPUsAllocation").
					reportProblemsToJiraComponent("NodeTuningOperator").
					contactPerson("titzhak").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateManagedBootImages = newFeatureGate("ManagedBootImages").
					reportProblemsToJiraComponent("MachineConfigOperator").
					contactPerson("djoshy").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateManagedBootImagesAWS = newFeatureGate("ManagedBootImagesAWS").
					reportProblemsToJiraComponent("MachineConfigOperator").
					contactPerson("djoshy").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateManagedBootImagesvSphere = newFeatureGate("ManagedBootImagesvSphere").
						reportProblemsToJiraComponent("MachineConfigOperator").
						contactPerson("rsaini").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1496").
						enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateManagedBootImagesAzure = newFeatureGate("ManagedBootImagesAzure").
						reportProblemsToJiraComponent("MachineConfigOperator").
						contactPerson("djoshy").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1761").
						enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateManagedBootImagesCPMS = newFeatureGate("ManagedBootImagesCPMS").
						reportProblemsToJiraComponent("MachineConfigOperator").
						contactPerson("djoshy").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1818").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateBootImageSkewEnforcement = newFeatureGate("BootImageSkewEnforcement").
						reportProblemsToJiraComponent("MachineConfigOperator").
						contactPerson("djoshy").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1761").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateBootcNodeManagement = newFeatureGate("BootcNodeManagement").
					reportProblemsToJiraComponent("MachineConfigOperator").
					contactPerson("inesqyx").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateSignatureStores = newFeatureGate("SignatureStores").
					reportProblemsToJiraComponent("Cluster Version Operator").
					contactPerson("lmohanty").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateKMSv1 = newFeatureGate("KMSv1").
				reportProblemsToJiraComponent("kube-apiserver").
				contactPerson("dgrisonnet").
				productScope(kubernetes).
				enhancementPR(legacyFeatureGateWithoutEnhancement).
				enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
				mustRegister()

	FeatureGatePinnedImages = newFeatureGate("PinnedImages").
				reportProblemsToJiraComponent("MachineConfigOperator").
				contactPerson("RishabhSaini").
				productScope(ocpSpecific).
				enhancementPR(legacyFeatureGateWithoutEnhancement).
				enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
				mustRegister()

	FeatureGateUpgradeStatus = newFeatureGate("UpgradeStatus").
					reportProblemsToJiraComponent("Cluster Version Operator").
					contactPerson("pmuller").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade, configv1.Default, configv1.OKD).
					mustRegister()

	FeatureGateVolumeAttributesClass = newFeatureGate("VolumeAttributesClass").
						reportProblemsToJiraComponent("Storage / Kubernetes External Components").
						contactPerson("dfajmon").
						productScope(kubernetes).
						enhancementPR("https://github.com/kubernetes/enhancements/issues/3751").
						enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateVolumeGroupSnapshot = newFeatureGate("VolumeGroupSnapshot").
					reportProblemsToJiraComponent("Storage / Kubernetes External Components").
					contactPerson("fbertina").
					productScope(kubernetes).
					enhancementPR("https://github.com/kubernetes/enhancements/issues/3476").
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateExternalSnapshotMetadata = newFeatureGate("ExternalSnapshotMetadata").
						reportProblemsToJiraComponent("Storage / Kubernetes External Components").
						contactPerson("jdobson").
						productScope(kubernetes).
						enhancementPR("https://github.com/kubernetes/enhancements/issues/3314").
						enableIn(configv1.DevPreviewNoUpgrade).
						mustRegister()

	FeatureGateExternalOIDC = newFeatureGate("ExternalOIDC").
				reportProblemsToJiraComponent("authentication").
				contactPerson("liouk").
				productScope(ocpSpecific).
				enhancementPR("https://github.com/openshift/enhancements/pull/1596").
				enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade, configv1.Default, configv1.OKD).
				mustRegister()

	FeatureGateExternalOIDCWithAdditionalClaimMappings = newFeatureGate("ExternalOIDCWithUIDAndExtraClaimMappings").
								reportProblemsToJiraComponent("authentication").
								contactPerson("bpalmer").
								productScope(ocpSpecific).
								enhancementPR("https://github.com/openshift/enhancements/pull/1777").
								enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade, configv1.Default, configv1.OKD).
								mustRegister()

	FeatureGateExternalOIDCWithUpstreamParity = newFeatureGate("ExternalOIDCWithUpstreamParity").
							reportProblemsToJiraComponent("authentication").
							contactPerson("saldawam").
							productScope(ocpSpecific).
							enhancementPR("https://github.com/openshift/enhancements/pull/1763").
							enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
							mustRegister()

	FeatureGateExample = newFeatureGate("Example").
				reportProblemsToJiraComponent("cluster-config").
				contactPerson("deads").
				productScope(ocpSpecific).
				enhancementPR(legacyFeatureGateWithoutEnhancement).
				enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
				mustRegister()

	FeatureGateExample2 = newFeatureGate("Example2").
				reportProblemsToJiraComponent("cluster-config").
				contactPerson("JoelSpeed").
				productScope(ocpSpecific).
				enhancementPR(legacyFeatureGateWithoutEnhancement).
				enableIn(configv1.DevPreviewNoUpgrade).
				mustRegister()

	FeatureGateNewOLM = newFeatureGate("NewOLM").
				reportProblemsToJiraComponent("olm").
				contactPerson("joe").
				productScope(ocpSpecific).
				enhancementPR(legacyFeatureGateWithoutEnhancement).
				enableForClusterProfile(SelfManaged, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade, configv1.Default, configv1.OKD).
				mustRegister()

	FeatureGateNewOLMCatalogdAPIV1Metas = newFeatureGate("NewOLMCatalogdAPIV1Metas").
						reportProblemsToJiraComponent("olm").
						contactPerson("jordank").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1749").
						enableForClusterProfile(SelfManaged, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateNewOLMPreflightPermissionChecks = newFeatureGate("NewOLMPreflightPermissionChecks").
							reportProblemsToJiraComponent("olm").
							contactPerson("tshort").
							productScope(ocpSpecific).
							enhancementPR("https://github.com/openshift/enhancements/pull/1768").
							enableForClusterProfile(SelfManaged, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
							mustRegister()

	FeatureGateNewOLMOwnSingleNamespace = newFeatureGate("NewOLMOwnSingleNamespace").
						reportProblemsToJiraComponent("olm").
						contactPerson("nschieder").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1849").
						enableForClusterProfile(SelfManaged, configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateNewOLMWebhookProviderOpenshiftServiceCA = newFeatureGate("NewOLMWebhookProviderOpenshiftServiceCA").
								reportProblemsToJiraComponent("olm").
								contactPerson("pegoncal").
								productScope(ocpSpecific).
								enhancementPR("https://github.com/openshift/enhancements/pull/1844").
								enableForClusterProfile(SelfManaged, configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
								mustRegister()

	FeatureGateNewOLMBoxCutterRuntime = newFeatureGate("NewOLMBoxCutterRuntime").
						reportProblemsToJiraComponent("olm").
						contactPerson("pegoncal").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1890").
						enableForClusterProfile(SelfManaged, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateInsightsOnDemandDataGather = newFeatureGate("InsightsOnDemandDataGather").
						reportProblemsToJiraComponent("insights").
						contactPerson("tremes").
						productScope(ocpSpecific).
						enhancementPR(legacyFeatureGateWithoutEnhancement).
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateInsightsConfig = newFeatureGate("InsightsConfig").
					reportProblemsToJiraComponent("insights").
					contactPerson("tremes").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateMetricsCollectionProfiles = newFeatureGate("MetricsCollectionProfiles").
						reportProblemsToJiraComponent("Monitoring").
						contactPerson("rexagod").
						productScope(ocpSpecific).
						enhancementPR(legacyFeatureGateWithoutEnhancement).
						enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateClusterAPIInstallIBMCloud = newFeatureGate("ClusterAPIInstallIBMCloud").
						reportProblemsToJiraComponent("Installer").
						contactPerson("cjschaef").
						productScope(ocpSpecific).
						enhancementPR(legacyFeatureGateWithoutEnhancement).
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateMachineAPIMigration = newFeatureGate("MachineAPIMigration").
					reportProblemsToJiraComponent("OCPCLOUD").
					contactPerson("jspeed").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateClusterAPIMachineManagement = newFeatureGate("ClusterAPIMachineManagement").
						reportProblemsToJiraComponent("Cloud Compute / Cluster API Providers").
						contactPerson("ddonati").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1465").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateClusterAPIMachineManagementVSphere = newFeatureGate("ClusterAPIMachineManagementVSphere").
							reportProblemsToJiraComponent("SPLAT").
							contactPerson("jcpowermac").
							productScope(ocpSpecific).
							enhancementPR("https://github.com/openshift/enhancements/pull/1465").
							enableIn(configv1.DevPreviewNoUpgrade).
							mustRegister()

	FeatureGateClusterMonitoringConfig = newFeatureGate("ClusterMonitoringConfig").
						reportProblemsToJiraComponent("Monitoring").
						contactPerson("marioferh").
						productScope(ocpSpecific).
						enhancementPR(legacyFeatureGateWithoutEnhancement).
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateMultiArchInstallAzure = newFeatureGate("MultiArchInstallAzure").
						reportProblemsToJiraComponent("Installer").
						contactPerson("r4f4").
						productScope(ocpSpecific).
						enhancementPR(legacyFeatureGateWithoutEnhancement).
						mustRegister()

	FeatureGateImageStreamImportMode = newFeatureGate("ImageStreamImportMode").
						reportProblemsToJiraComponent("Multi-Arch").
						contactPerson("psundara").
						productScope(ocpSpecific).
						enhancementPR(legacyFeatureGateWithoutEnhancement).
						enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateUserNamespacesSupport = newFeatureGate("UserNamespacesSupport").
						reportProblemsToJiraComponent("Node").
						contactPerson("haircommander").
						productScope(kubernetes).
						enhancementPR("https://github.com/kubernetes/enhancements/issues/127").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade, configv1.Default, configv1.OKD).
						mustRegister()

	// Note: this feature is perma-alpha, but it is safe and desireable to enable.
	// It was an oversight in upstream to not remove the feature gate after the version skew became safe in 1.33.
	// See https://github.com/kubernetes/enhancements/tree/d4226c42/keps/sig-node/127-user-namespaces#pod-security-standards-pss-integration
	FeatureGateUserNamespacesPodSecurityStandards = newFeatureGate("UserNamespacesPodSecurityStandards").
							reportProblemsToJiraComponent("Node").
							contactPerson("haircommander").
							productScope(kubernetes).
							enhancementPR("https://github.com/kubernetes/enhancements/issues/127").
							enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade, configv1.Default, configv1.OKD).
							mustRegister()

	FeatureGateProcMountType = newFeatureGate("ProcMountType").
					reportProblemsToJiraComponent("Node").
					contactPerson("haircommander").
					productScope(kubernetes).
					enhancementPR("https://github.com/kubernetes/enhancements/issues/4265").
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade, configv1.Default, configv1.OKD).
					mustRegister()

	FeatureGateVSphereMultiNetworks = newFeatureGate("VSphereMultiNetworks").
					reportProblemsToJiraComponent("SPLAT").
					contactPerson("rvanderp").
					productScope(ocpSpecific).
					enhancementPR(legacyFeatureGateWithoutEnhancement).
					enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateIngressControllerDynamicConfigurationManager = newFeatureGate("IngressControllerDynamicConfigurationManager").
								reportProblemsToJiraComponent("Networking/router").
								contactPerson("miciah").
								productScope(ocpSpecific).
								enhancementPR(legacyFeatureGateWithoutEnhancement).
								enableIn(configv1.DevPreviewNoUpgrade).
								mustRegister()

	FeatureGateMinimumKubeletVersion = newFeatureGate("MinimumKubeletVersion").
						reportProblemsToJiraComponent("Node").
						contactPerson("haircommander").
						productScope(ocpSpecific).
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						enhancementPR("https://github.com/openshift/enhancements/pull/1697").
						mustRegister()

	FeatureGateNutanixMultiSubnets = newFeatureGate("NutanixMultiSubnets").
					reportProblemsToJiraComponent("Cloud Compute / Nutanix Provider").
					contactPerson("yanhli").
					productScope(ocpSpecific).
					enhancementPR("https://github.com/openshift/enhancements/pull/1711").
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateKMSEncryptionProvider = newFeatureGate("KMSEncryptionProvider").
						reportProblemsToJiraComponent("kube-apiserver").
						contactPerson("swghosh").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1682").
						enableIn(configv1.DevPreviewNoUpgrade).
						mustRegister()

	FeatureGateKMSEncryption = newFeatureGate("KMSEncryption").
		reportProblemsToJiraComponent("kube-apiserver").
		contactPerson("ardaguclu").
		productScope(ocpSpecific).
		enhancementPR("https://github.com/openshift/enhancements/pull/1900").
		enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
		mustRegister()

	FeatureGateHighlyAvailableArbiter = newFeatureGate("HighlyAvailableArbiter").
						reportProblemsToJiraComponent("Two Node with Arbiter").
						contactPerson("eggfoobar").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1674").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade, configv1.Default, configv1.OKD).
						mustRegister()

	FeatureGateCVOConfiguration = newFeatureGate("ClusterVersionOperatorConfiguration").
					reportProblemsToJiraComponent("Cluster Version Operator").
					contactPerson("dhurta").
					productScope(ocpSpecific).
					enhancementPR("https://github.com/openshift/enhancements/pull/1492").
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateClusterUpdateAcceptRisks = newFeatureGate("ClusterUpdateAcceptRisks").
						reportProblemsToJiraComponent("Cluster Version Operator").
						contactPerson("hongkliu").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1807").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateGCPCustomAPIEndpoints = newFeatureGate("GCPCustomAPIEndpoints").
						reportProblemsToJiraComponent("Installer").
						contactPerson("barbacbd").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1492").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateDyanmicServiceEndpointIBMCloud = newFeatureGate("DyanmicServiceEndpointIBMCloud").
							reportProblemsToJiraComponent("Cloud Compute / IBM Provider").
							contactPerson("jared-hayes-dev").
							productScope(ocpSpecific).
							enhancementPR("https://github.com/openshift/enhancements/pull/1712").
							enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
							mustRegister()

	FeatureGateSELinuxMount = newFeatureGate("SELinuxMount").
				reportProblemsToJiraComponent("Storage / Kubernetes").
				contactPerson("jsafrane").
				productScope(kubernetes).
				enhancementPR("https://github.com/kubernetes/enhancements/issues/1710").
				enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
				mustRegister()

	FeatureGateDualReplica = newFeatureGate("DualReplica").
				reportProblemsToJiraComponent("Two Node Fencing").
				contactPerson("jaypoulz").
				productScope(ocpSpecific).
				enhancementPR("https://github.com/openshift/enhancements/pull/1675").
				enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
				mustRegister()

	FeatureGateGatewayAPIController = newFeatureGate("GatewayAPIController").
					reportProblemsToJiraComponent("Routing").
					contactPerson("miciah").
					productScope(ocpSpecific).
		// Previously, the "GatewayAPI" feature gate managed both the GatewayAPI CRDs
		// and the Gateway Controller. However, with the introduction of Gateway CRD
		// lifecycle management (EP#1756), these responsibilities were separated.
		// A dedicated feature gate now controls the Gateway Controller to distinguish
		// its production readiness from that of the CRDs.
		enhancementPR("https://github.com/openshift/enhancements/pull/1756").
		enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
		mustRegister()

	FeatureShortCertRotation = newFeatureGate("ShortCertRotation").
					reportProblemsToJiraComponent("kube-apiserver").
					contactPerson("vrutkovs").
					productScope(ocpSpecific).
					enhancementPR("https://github.com/openshift/enhancements/pull/1670").
					mustRegister()

	FeatureGateVSphereConfigurableMaxAllowedBlockVolumesPerNode = newFeatureGate("VSphereConfigurableMaxAllowedBlockVolumesPerNode").
									reportProblemsToJiraComponent("Storage / Kubernetes External Components").
									contactPerson("rbednar").
									productScope(ocpSpecific).
									enhancementPR("https://github.com/openshift/enhancements/pull/1748").
									enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
									mustRegister()

	FeatureGateAzureMultiDisk = newFeatureGate("AzureMultiDisk").
					reportProblemsToJiraComponent("splat").
					contactPerson("jcpowermac").
					productScope(ocpSpecific).
					enhancementPR("https://github.com/openshift/enhancements/pull/1779").
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateStoragePerformantSecurityPolicy = newFeatureGate("StoragePerformantSecurityPolicy").
							reportProblemsToJiraComponent("Storage / Kubernetes External Components").
							contactPerson("hekumar").
							productScope(ocpSpecific).
							enhancementPR("https://github.com/openshift/enhancements/pull/1804").
							enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade, configv1.Default, configv1.OKD).
							mustRegister()

	FeatureGateMultiDiskSetup = newFeatureGate("MultiDiskSetup").
					reportProblemsToJiraComponent("splat").
					contactPerson("jcpowermac").
					productScope(ocpSpecific).
					enhancementPR("https://github.com/openshift/enhancements/pull/1805").
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateAWSDedicatedHosts = newFeatureGate("AWSDedicatedHosts").
					reportProblemsToJiraComponent("splat").
					contactPerson("rvanderp3").
					productScope(ocpSpecific).
					enhancementPR("https://github.com/openshift/enhancements/pull/1781").
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateVSphereMixedNodeEnv = newFeatureGate("VSphereMixedNodeEnv").
					reportProblemsToJiraComponent("splat").
					contactPerson("vr4manta").
					productScope(ocpSpecific).
					enhancementPR("https://github.com/openshift/enhancements/pull/1772").
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGatePreconfiguredUDNAddresses = newFeatureGate("PreconfiguredUDNAddresses").
						reportProblemsToJiraComponent("Networking/ovn-kubernetes").
						contactPerson("kyrtapz").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1793").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade, configv1.Default, configv1.OKD).
						mustRegister()

	FeatureGateAWSServiceLBNetworkSecurityGroup = newFeatureGate("AWSServiceLBNetworkSecurityGroup").
							reportProblemsToJiraComponent("Cloud Compute / Cloud Controller Manager").
							contactPerson("mtulio").
							productScope(ocpSpecific).
							enhancementPR("https://github.com/openshift/enhancements/pull/1802").
							enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
							mustRegister()

	FeatureGateImageVolume = newFeatureGate("ImageVolume").
				reportProblemsToJiraComponent("Node").
				contactPerson("haircommander").
				productScope(kubernetes).
				enhancementPR("https://github.com/openshift/enhancements/pull/1792").
				enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade, configv1.Default, configv1.OKD).
				mustRegister()

	FeatureGateNoRegistryClusterInstall = newFeatureGate("NoRegistryClusterInstall").
						reportProblemsToJiraComponent("Installer / Agent based installation").
						contactPerson("andfasano").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1821").
						enableForClusterProfile(SelfManaged, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateGCPClusterHostedDNSInstall = newFeatureGate("GCPClusterHostedDNSInstall").
						reportProblemsToJiraComponent("Installer").
						contactPerson("barbacbd").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1468").
						enableIn(configv1.Default, configv1.OKD, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateAWSClusterHostedDNSInstall = newFeatureGate("AWSClusterHostedDNSInstall").
						reportProblemsToJiraComponent("Installer").
						contactPerson("barbacbd").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1468").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateGCPCustomAPIEndpointsInstall = newFeatureGate("GCPCustomAPIEndpointsInstall").
						reportProblemsToJiraComponent("Installer").
						contactPerson("barbacbd").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1492").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateIrreconcilableMachineConfig = newFeatureGate("IrreconcilableMachineConfig").
						reportProblemsToJiraComponent("MachineConfigOperator").
						contactPerson("pabrodri").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1785").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()
	FeatureGateAWSDualStackInstall = newFeatureGate("AWSDualStackInstall").
					reportProblemsToJiraComponent("Installer").
					contactPerson("sadasu").
					productScope(ocpSpecific).
					enhancementPR("https://github.com/openshift/enhancements/pull/1806").
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateAzureDualStackInstall = newFeatureGate("AzureDualStackInstall").
						reportProblemsToJiraComponent("Installer").
						contactPerson("jhixson74").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1806").
						enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
						mustRegister()

	FeatureGateGCPDualStackInstall = newFeatureGate("GCPDualStackInstall").
					reportProblemsToJiraComponent("Installer").
					contactPerson("barbacbd").
					productScope(ocpSpecific).
					enhancementPR("https://github.com/openshift/enhancements/pull/1806").
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureCBORServingAndStorage = newFeatureGate("CBORServingAndStorage").
					reportProblemsToJiraComponent("kube-apiserver").
					contactPerson("benluddy").
					productScope(kubernetes).
					enhancementPR("https://github.com/kubernetes/enhancements/issues/4222").
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureCBORClientsAllowCBOR = newFeatureGate("ClientsAllowCBOR").
					reportProblemsToJiraComponent("kube-apiserver").
					contactPerson("benluddy").
					productScope(kubernetes).
					enhancementPR("https://github.com/kubernetes/enhancements/issues/4222").
					mustRegister()

	FeatureClientsPreferCBOR = newFeatureGate("ClientsPreferCBOR").
					reportProblemsToJiraComponent("kube-apiserver").
					contactPerson("benluddy").
					productScope(kubernetes).
					enhancementPR("https://github.com/kubernetes/enhancements/issues/4222").
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureEventTTL = newFeatureGate("EventTTL").
			reportProblemsToJiraComponent("kube-apiserver").
			contactPerson("tjungblu").
			productScope(ocpSpecific).
			enhancementPR("https://github.com/openshift/enhancements/pull/1857").
			enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
			mustRegister()

	FeatureGateMutableCSINodeAllocatableCount = newFeatureGate("MutableCSINodeAllocatableCount").
							reportProblemsToJiraComponent("Storage / Kubernetes External Components").
							contactPerson("jsafrane").
							productScope(kubernetes).
							enhancementPR("https://github.com/kubernetes/enhancements/issues/4876").
							enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
							mustRegister()
	FeatureGateOSStreams = newFeatureGate("OSStreams").
				reportProblemsToJiraComponent("MachineConfigOperator").
				contactPerson("pabrodri").
				productScope(ocpSpecific).
				enhancementPR("https://github.com/openshift/enhancements/pull/1874").
				enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
				mustRegister()

	FeatureGateCRDCompatibilityRequirementOperator = newFeatureGate("CRDCompatibilityRequirementOperator").
							reportProblemsToJiraComponent("Cloud Compute / Cluster API Providers").
							contactPerson("ddonati").
							productScope(ocpSpecific).
							enhancementPR("https://github.com/openshift/enhancements/pull/1845").
							enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
							mustRegister()
	FeatureGateOnPremDNSRecords = newFeatureGate("OnPremDNSRecords").
					reportProblemsToJiraComponent("Networking / On-Prem DNS").
					contactPerson("bnemec").
					productScope(ocpSpecific).
					enhancementPR("https://github.com/openshift/enhancements/pull/1803").
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateProvisioningRequestAvailable = newFeatureGate("ProvisioningRequestAvailable").
						reportProblemsToJiraComponent("Cluster Autoscaler").
						contactPerson("elmiko").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1752").
						enableIn(configv1.DevPreviewNoUpgrade).
						mustRegister()

	FeatureGateHyperShiftOnlyDynamicResourceAllocation = newFeatureGate("HyperShiftOnlyDynamicResourceAllocation").
								reportProblemsToJiraComponent("hypershift").
								contactPerson("csrwng").
								productScope(ocpSpecific).
								enhancementPR("https://github.com/kubernetes/enhancements/issues/4381").
								enableForClusterProfile(Hypershift, configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade, configv1.Default, configv1.OKD).
								mustRegister()

	FeatureGateConfigurablePKI = newFeatureGate("ConfigurablePKI").
					reportProblemsToJiraComponent("kube-apiserver").
					contactPerson("sanchezl").
					productScope(ocpSpecific).
					enhancementPR("https://github.com/openshift/enhancements/pull/1882").
					enableIn(configv1.DevPreviewNoUpgrade, configv1.TechPreviewNoUpgrade).
					mustRegister()

	FeatureGateClusterAPIControlPlaneInstall = newFeatureGate("ClusterAPIControlPlaneInstall").
							reportProblemsToJiraComponent("Installer / openshift-installer").
							contactPerson("patrickdillon").
							productScope(ocpSpecific).
							enhancementPR("https://github.com/openshift/enhancements/pull/1465").
							enableIn(configv1.DevPreviewNoUpgrade).
							mustRegister()

	FeatureGateClusterAPIComputeInstall = newFeatureGate("ClusterAPIComputeInstall").
						reportProblemsToJiraComponent("Installer / openshift-installer").
						contactPerson("patrickdillon").
						productScope(ocpSpecific).
						enhancementPR("https://github.com/openshift/enhancements/pull/1465").
						enableIn(configv1.DevPreviewNoUpgrade).
						mustRegister()
)
