package defaultmonitortests

import (
	"fmt"

	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortests/authentication/legacyauthenticationmonitortests"
	"github.com/openshift/origin/pkg/monitortests/authentication/requiredsccmonitortests"
	admupgradestatus "github.com/openshift/origin/pkg/monitortests/cli/adm_upgrade/status"
	azuremetrics "github.com/openshift/origin/pkg/monitortests/cloud/azure/metrics"
	"github.com/openshift/origin/pkg/monitortests/clusterversionoperator/clusterversionchecker"
	"github.com/openshift/origin/pkg/monitortests/clusterversionoperator/legacycvomonitortests"
	"github.com/openshift/origin/pkg/monitortests/clusterversionoperator/operatorstateanalyzer"
	"github.com/openshift/origin/pkg/monitortests/clusterversionoperator/terminationmessagepolicy"
	"github.com/openshift/origin/pkg/monitortests/etcd/etcdloganalyzer"
	"github.com/openshift/origin/pkg/monitortests/etcd/legacyetcdmonitortests"
	"github.com/openshift/origin/pkg/monitortests/imageregistry/disruptionimageregistry"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/apiservergracefulrestart"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/apiunreachablefromclientmetrics"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/auditloganalyzer"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/disruptionexternalapiserver"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/disruptioninclusterapiserver"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/disruptionnewapiserver"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/faultyloadbalancer"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/generationanalyzer"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/legacykubeapiservermonitortests"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/staticpodinstall"
	"github.com/openshift/origin/pkg/monitortests/kubelet/containerfailures"
	"github.com/openshift/origin/pkg/monitortests/machines/watchmachines"
	"github.com/openshift/origin/pkg/monitortests/monitoring/disruptionmetricsapi"
	"github.com/openshift/origin/pkg/monitortests/monitoring/statefulsetsrecreation"
	"github.com/openshift/origin/pkg/monitortests/network/disruptioningress"
	"github.com/openshift/origin/pkg/monitortests/network/disruptionpodnetwork"
	"github.com/openshift/origin/pkg/monitortests/network/disruptionserviceloadbalancer"
	"github.com/openshift/origin/pkg/monitortests/network/legacynetworkmonitortests"
	"github.com/openshift/origin/pkg/monitortests/network/onpremhaproxy"
	"github.com/openshift/origin/pkg/monitortests/network/onpremkeepalived"
	"github.com/openshift/origin/pkg/monitortests/node/kubeletlogcollector"
	"github.com/openshift/origin/pkg/monitortests/node/legacynodemonitortests"
	"github.com/openshift/origin/pkg/monitortests/node/nodestateanalyzer"
	"github.com/openshift/origin/pkg/monitortests/node/watchnodes"
	"github.com/openshift/origin/pkg/monitortests/node/watchpods"
	"github.com/openshift/origin/pkg/monitortests/storage/legacystoragemonitortests"
	"github.com/openshift/origin/pkg/monitortests/testframework/additionaleventscollector"
	"github.com/openshift/origin/pkg/monitortests/testframework/alertanalyzer"
	"github.com/openshift/origin/pkg/monitortests/testframework/clusterinfoserializer"
	"github.com/openshift/origin/pkg/monitortests/testframework/disruptionexternalawscloudservicemonitoring"
	"github.com/openshift/origin/pkg/monitortests/testframework/disruptionexternalazurecloudservicemonitoring"
	"github.com/openshift/origin/pkg/monitortests/testframework/disruptionexternalgcpcloudservicemonitoring"
	"github.com/openshift/origin/pkg/monitortests/testframework/disruptionexternalservicemonitoring"
	"github.com/openshift/origin/pkg/monitortests/testframework/disruptionserializer"
	"github.com/openshift/origin/pkg/monitortests/testframework/e2etestanalyzer"
	"github.com/openshift/origin/pkg/monitortests/testframework/etcddiskmetricsintervals"
	"github.com/openshift/origin/pkg/monitortests/testframework/highcpumetriccollector"
	"github.com/openshift/origin/pkg/monitortests/testframework/highcputestanalyzer"

	"github.com/openshift/origin/pkg/monitortests/testframework/intervalserializer"
	"github.com/openshift/origin/pkg/monitortests/testframework/knownimagechecker"
	"github.com/openshift/origin/pkg/monitortests/testframework/legacytestframeworkmonitortests"
	"github.com/openshift/origin/pkg/monitortests/testframework/metricsendpointdown"
	"github.com/openshift/origin/pkg/monitortests/testframework/operatorloganalyzer"
	"github.com/openshift/origin/pkg/monitortests/testframework/pathologicaleventanalyzer"
	"github.com/openshift/origin/pkg/monitortests/testframework/timelineserializer"
	"github.com/openshift/origin/pkg/monitortests/testframework/trackedresourcesserializer"
	"github.com/openshift/origin/pkg/monitortests/testframework/watchclusteroperators"
	"github.com/openshift/origin/pkg/monitortests/testframework/watchevents"
	"github.com/openshift/origin/pkg/monitortests/testframework/watchnamespaces"
	"github.com/sirupsen/logrus"
)

// ListAllMonitorTests is a helper that returns a simple list of
// available monitor tests names
func ListAllMonitorTests() []string {
	monitorTestInfo := monitortestframework.MonitorTestInitializationInfo{
		ClusterStabilityDuringTest: monitortestframework.Stable,
	}
	allMonitors, err := NewMonitorTestsFor(monitorTestInfo)
	if err != nil {
		logrus.Errorf("Error listing all monitor tests: %v", err)
	}
	monitorNames := []string{}
	if allMonitors != nil {
		monitorNames = allMonitors.ListMonitorTests().List()
	}

	return monitorNames
}

func NewMonitorTestsFor(info monitortestframework.MonitorTestInitializationInfo) (monitortestframework.MonitorTestRegistry, error) {
	// get tests and apply any filtering defined in info
	var startingRegistry monitortestframework.MonitorTestRegistry

	switch info.ClusterStabilityDuringTest {
	case monitortestframework.Stable:
		startingRegistry = newDefaultMonitorTests(info)
	case monitortestframework.Disruptive:
		startingRegistry = newDisruptiveMonitorTests(info)
	default:
		panic(fmt.Sprintf("unknown cluster stability level: %q", info.ClusterStabilityDuringTest))
	}

	switch {
	case len(info.ExactMonitorTests) > 0:
		return startingRegistry.GetRegistryFor(info.ExactMonitorTests...)

	case len(info.DisableMonitorTests) > 0:
		testsToInclude := startingRegistry.ListMonitorTests()
		testsToInclude.Delete(info.DisableMonitorTests...)
		return startingRegistry.GetRegistryFor(testsToInclude.List()...)
	}

	return startingRegistry, nil
}

func newDefaultMonitorTests(info monitortestframework.MonitorTestInitializationInfo) monitortestframework.MonitorTestRegistry {
	monitorTestRegistry := monitortestframework.NewMonitorTestRegistry()

	monitorTestRegistry.AddRegistryOrDie(newUniversalMonitorTests(info))

	monitorTestRegistry.AddMonitorTestOrDie("image-registry-availability", "Image Registry", disruptionimageregistry.NewAvailabilityInvariant())

	monitorTestRegistry.AddMonitorTestOrDie("apiserver-disruption-invariant", "kube-apiserver", disruptionnewapiserver.NewDisruptionInvariant())
	monitorTestRegistry.AddMonitorTestOrDie("apiserver-external-availability", "kube-apiserver", disruptionexternalapiserver.NewExternalDisruptionInvariant(info))
	monitorTestRegistry.AddMonitorTestOrDie("apiserver-incluster-availability", "kube-apiserver", disruptioninclusterapiserver.NewInvariantInClusterDisruption(info))

	monitorTestRegistry.AddMonitorTestOrDie("pod-network-avalibility", "Network / ovn-kubernetes", disruptionpodnetwork.NewPodNetworkAvalibilityInvariant(info))
	monitorTestRegistry.AddMonitorTestOrDie("service-type-load-balancer-availability", "Networking / router", disruptionserviceloadbalancer.NewAvailabilityInvariant())
	monitorTestRegistry.AddMonitorTestOrDie("ingress-availability", "Networking / router", disruptioningress.NewAvailabilityInvariant())
	monitorTestRegistry.AddMonitorTestOrDie("on-prem-keepalived", "Networking / On-Prem Loadbalancer", onpremkeepalived.InitialAndFinalOperatorLogScraper())

	monitorTestRegistry.AddMonitorTestOrDie("on-prem-haproxy", "Networking / On-Prem Host Networking", onpremhaproxy.InitialAndFinalOperatorLogScraper())

	monitorTestRegistry.AddMonitorTestOrDie("alert-summary-serializer", "Test Framework", alertanalyzer.NewAlertSummarySerializer())
	monitorTestRegistry.AddMonitorTestOrDie("metrics-endpoints-down", "Test Framework", metricsendpointdown.NewMetricsEndpointDown())
	monitorTestRegistry.AddMonitorTestOrDie("external-service-availability", "Test Framework", disruptionexternalservicemonitoring.NewAvailabilityInvariant())
	monitorTestRegistry.AddMonitorTestOrDie("external-gcp-cloud-service-availability", "Test Framework", disruptionexternalgcpcloudservicemonitoring.NewCloudAvailabilityInvariant())
	monitorTestRegistry.AddMonitorTestOrDie("external-aws-cloud-service-availability", "Test Framework", disruptionexternalawscloudservicemonitoring.NewCloudAvailabilityInvariant())
	monitorTestRegistry.AddMonitorTestOrDie("external-azure-cloud-service-availability", "Test Framework", disruptionexternalazurecloudservicemonitoring.NewCloudAvailabilityInvariant())
	monitorTestRegistry.AddMonitorTestOrDie("pathological-event-analyzer", "Test Framework", pathologicaleventanalyzer.NewAnalyzer())
	monitorTestRegistry.AddMonitorTestOrDie("disruption-summary-serializer", "Test Framework", disruptionserializer.NewDisruptionSummarySerializer())

	monitorTestRegistry.AddMonitorTestOrDie("monitoring-statefulsets-recreation", "Monitoring", statefulsetsrecreation.NewStatefulsetsChecker())
	monitorTestRegistry.AddMonitorTestOrDie("metrics-api-availability", "Monitoring", disruptionmetricsapi.NewAvailabilityInvariant())
	monitorTestRegistry.AddMonitorTestOrDie(apiunreachablefromclientmetrics.MonitorName, "kube-apiserver", apiunreachablefromclientmetrics.NewMonitorTest())
	monitorTestRegistry.AddMonitorTestOrDie(faultyloadbalancer.MonitorName, "kube-apiserver", faultyloadbalancer.NewMonitorTest())
	monitorTestRegistry.AddMonitorTestOrDie(staticpodinstall.MonitorName, "kube-apiserver", staticpodinstall.NewStaticPodInstallMonitorTest())
	monitorTestRegistry.AddMonitorTestOrDie(containerfailures.MonitorName, "Node / Kubelet", containerfailures.NewContainerFailuresTests())
	monitorTestRegistry.AddMonitorTestOrDie(legacytestframeworkmonitortests.PathologicalMonitorName, "Test Framework", legacytestframeworkmonitortests.NewLegacyPathologicalMonitorTests(info))
	monitorTestRegistry.AddMonitorTestOrDie("legacy-cvo-invariants", "Cluster Version Operator", legacycvomonitortests.NewLegacyTests())

	return monitorTestRegistry
}

func newDisruptiveMonitorTests(info monitortestframework.MonitorTestInitializationInfo) monitortestframework.MonitorTestRegistry {
	monitorTestRegistry := monitortestframework.NewMonitorTestRegistry()

	monitorTestRegistry.AddRegistryOrDie(newUniversalMonitorTests(info))

	// this data would be interesting, but I'm betting we cannot scrub the data after the fact to exclude these.
	// monitorTestRegistry.AddMonitorTestOrDie("image-registry-availability", "Image Registry", disruptionimageregistry.NewRecordAvailabilityOnly())
	// monitorTestRegistry.AddMonitorTestOrDie("apiserver-availability", "kube-apiserver", disruptionlegacyapiservers.NewRecordAvailabilityOnly())
	// monitorTestRegistry.AddMonitorTestOrDie("service-type-load-balancer-availability", "Networking / router", disruptionserviceloadbalancer.NewRecordAvailabilityOnly())
	// monitorTestRegistry.AddMonitorTestOrDie("ingress-availability", "Networking / router", disruptioningress.NewRecordAvailabilityOnly())
	// monitorTestRegistry.AddMonitorTestOrDie("external-service-availability", "Test Framework", disruptionexternalservicemonitoring.NewRecordAvailabilityOnly())

	return monitorTestRegistry
}

func newUniversalMonitorTests(info monitortestframework.MonitorTestInitializationInfo) monitortestframework.MonitorTestRegistry {
	monitorTestRegistry := monitortestframework.NewMonitorTestRegistry()

	monitorTestRegistry.AddMonitorTestOrDie("legacy-authentication-invariants", "apiserver-auth", legacyauthenticationmonitortests.NewLegacyTests())

	monitorTestRegistry.AddMonitorTestOrDie("termination-message-policy", "Cluster Version Operator", terminationmessagepolicy.NewAnalyzer())
	monitorTestRegistry.AddMonitorTestOrDie("operator-state-analyzer", "Cluster Version Operator", operatorstateanalyzer.NewAnalyzer())
	monitorTestRegistry.AddMonitorTestOrDie("required-scc-annotation-checker", "Cluster Version Operator", requiredsccmonitortests.NewAnalyzer())
	monitorTestRegistry.AddMonitorTestOrDie("cluster-version-checker", "Cluster Version Operator", clusterversionchecker.NewClusterVersionChecker())

	monitorTestRegistry.AddMonitorTestOrDie("etcd-log-analyzer", "etcd", etcdloganalyzer.NewEtcdLogAnalyzer())
	monitorTestRegistry.AddMonitorTestOrDie("legacy-etcd-invariants", "etcd", legacyetcdmonitortests.NewLegacyTests())
	monitorTestRegistry.AddMonitorTestOrDie("etcd-disk-metrics-intervals", "etcd", etcddiskmetricsintervals.NewEtcdDiskMetricsCollector())

	monitorTestRegistry.AddMonitorTestOrDie("audit-log-analyzer", "kube-apiserver", auditloganalyzer.NewAuditLogAnalyzer(info))
	monitorTestRegistry.AddMonitorTestOrDie("legacy-kube-apiserver-invariants", "kube-apiserver", legacykubeapiservermonitortests.NewLegacyTests())
	monitorTestRegistry.AddMonitorTestOrDie("graceful-shutdown-analyzer", "kube-apiserver", apiservergracefulrestart.NewGracefulShutdownAnalyzer())

	monitorTestRegistry.AddMonitorTestOrDie("legacy-networking-invariants", "Networking / cluster-network-operator", legacynetworkmonitortests.NewLegacyTests())

	monitorTestRegistry.AddMonitorTestOrDie("kubelet-log-collector", "Node / Kubelet", kubeletlogcollector.NewKubeletLogCollector())
	monitorTestRegistry.AddMonitorTestOrDie("legacy-node-invariants", "Node / Kubelet", legacynodemonitortests.NewLegacyTests())
	monitorTestRegistry.AddMonitorTestOrDie("node-state-analyzer", "Node / Kubelet", nodestateanalyzer.NewAnalyzer())
	monitorTestRegistry.AddMonitorTestOrDie("high-cpu-metric-collector", "Node / Kubelet", highcpumetriccollector.NewHighCPUMetricCollector())
	monitorTestRegistry.AddMonitorTestOrDie("pod-lifecycle", "Node / Kubelet", watchpods.NewPodWatcher())
	monitorTestRegistry.AddMonitorTestOrDie("node-lifecycle", "Node / Kubelet", watchnodes.NewNodeWatcher())
	monitorTestRegistry.AddMonitorTestOrDie("machine-lifecycle", "Cluster-Lifecycle / machine-api", watchmachines.NewMachineWatcher())
	monitorTestRegistry.AddMonitorTestOrDie("generation-analyzer", "kube-apiserver", generationanalyzer.NewGenerationAnalyzer())

	monitorTestRegistry.AddMonitorTestOrDie("legacy-storage-invariants", "Storage", legacystoragemonitortests.NewLegacyTests())

	monitorTestRegistry.AddMonitorTestOrDie(legacytestframeworkmonitortests.AlertsMonitorName, "Test Framework", legacytestframeworkmonitortests.NewLegacyAlertsMonitorTests(info))
	monitorTestRegistry.AddMonitorTestOrDie("timeline-serializer", "Test Framework", timelineserializer.NewTimelineSerializer())
	monitorTestRegistry.AddMonitorTestOrDie("interval-serializer", "Test Framework", intervalserializer.NewIntervalSerializer())
	monitorTestRegistry.AddMonitorTestOrDie("tracked-resources-serializer", "Test Framework", trackedresourcesserializer.NewTrackedResourcesSerializer())
	monitorTestRegistry.AddMonitorTestOrDie("cluster-info-serializer", "Test Framework", clusterinfoserializer.NewClusterInfoSerializer())
	monitorTestRegistry.AddMonitorTestOrDie("additional-events-collector", "Test Framework", additionaleventscollector.NewIntervalSerializer())
	monitorTestRegistry.AddMonitorTestOrDie("known-image-checker", "Test Framework", knownimagechecker.NewEnsureValidImages())
	monitorTestRegistry.AddMonitorTestOrDie("e2e-test-analyzer", "Test Framework", e2etestanalyzer.NewAnalyzer())
	monitorTestRegistry.AddMonitorTestOrDie("event-collector", "Test Framework", watchevents.NewEventWatcher())
	monitorTestRegistry.AddMonitorTestOrDie("clusteroperator-collector", "Test Framework", watchclusteroperators.NewOperatorWatcher())
	monitorTestRegistry.AddMonitorTestOrDie("initial-and-final-operator-log-scraper", "Test Framework", operatorloganalyzer.InitialAndFinalOperatorLogScraper())
	monitorTestRegistry.AddMonitorTestOrDie("lease-checker", "Test Framework", operatorloganalyzer.OperatorLeaseCheck())

	monitorTestRegistry.AddMonitorTestOrDie("azure-metrics-collector", "Test Framework", azuremetrics.NewAzureMetricsCollector())
	monitorTestRegistry.AddMonitorTestOrDie("watch-namespaces", "Test Framework", watchnamespaces.NewNamespaceWatcher())
	monitorTestRegistry.AddMonitorTestOrDie("high-cpu-test-analyzer", "Test Framework", highcputestanalyzer.NewHighCPUTestAnalyzer())

	monitorTestRegistry.AddMonitorTestOrDie("oc-adm-upgrade-status", "oc / update", admupgradestatus.NewOcAdmUpgradeStatusChecker())

	return monitorTestRegistry
}
