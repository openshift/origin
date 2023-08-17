package defaultmonitortests

import (
	"fmt"

	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/apiservergracefulrestart"

	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortests/authentication/legacyauthenticationmonitortests"
	"github.com/openshift/origin/pkg/monitortests/clusterversionoperator/legacycvomonitortests"
	"github.com/openshift/origin/pkg/monitortests/clusterversionoperator/operatorstateanalyzer"
	"github.com/openshift/origin/pkg/monitortests/etcd/etcdloganalyzer"
	"github.com/openshift/origin/pkg/monitortests/etcd/legacyetcdmonitortests"
	"github.com/openshift/origin/pkg/monitortests/imageregistry/disruptionimageregistry"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/auditloganalyzer"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/disruptionlegacyapiservers"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/disruptionnewapiserver"
	"github.com/openshift/origin/pkg/monitortests/kubeapiserver/legacykubeapiservermonitortests"
	"github.com/openshift/origin/pkg/monitortests/network/disruptioningress"
	"github.com/openshift/origin/pkg/monitortests/network/disruptionpodnetwork"
	"github.com/openshift/origin/pkg/monitortests/network/disruptionserviceloadbalancer"
	"github.com/openshift/origin/pkg/monitortests/network/legacynetworkmonitortests"
	"github.com/openshift/origin/pkg/monitortests/node/kubeletlogcollector"
	"github.com/openshift/origin/pkg/monitortests/node/legacynodemonitortests"
	"github.com/openshift/origin/pkg/monitortests/node/nodestateanalyzer"
	"github.com/openshift/origin/pkg/monitortests/node/watchnodes"
	"github.com/openshift/origin/pkg/monitortests/node/watchpods"
	"github.com/openshift/origin/pkg/monitortests/storage/legacystoragemonitortests"
	"github.com/openshift/origin/pkg/monitortests/testframework/additionaleventscollector"
	"github.com/openshift/origin/pkg/monitortests/testframework/alertanalyzer"
	"github.com/openshift/origin/pkg/monitortests/testframework/clusterinfoserializer"
	"github.com/openshift/origin/pkg/monitortests/testframework/disruptionexternalservicemonitoring"
	"github.com/openshift/origin/pkg/monitortests/testframework/disruptionserializer"
	"github.com/openshift/origin/pkg/monitortests/testframework/e2etestanalyzer"
	"github.com/openshift/origin/pkg/monitortests/testframework/intervalserializer"
	"github.com/openshift/origin/pkg/monitortests/testframework/knownimagechecker"
	"github.com/openshift/origin/pkg/monitortests/testframework/legacytestframeworkmonitortests"
	"github.com/openshift/origin/pkg/monitortests/testframework/pathologicaleventanalyzer"
	"github.com/openshift/origin/pkg/monitortests/testframework/timelineserializer"
	"github.com/openshift/origin/pkg/monitortests/testframework/trackedresourcesserializer"
	"github.com/openshift/origin/pkg/monitortests/testframework/uploadtolokiserializer"
	"github.com/openshift/origin/pkg/monitortests/testframework/watchclusteroperators"
	"github.com/openshift/origin/pkg/monitortests/testframework/watchevents"
)

func NewMonitorTestsFor(info monitortestframework.MonitorTestInitializationInfo) monitortestframework.MonitorTestRegistry {
	switch info.ClusterStabilityDuringTest {
	case monitortestframework.Stable:
		return newDefaultMonitorTests(info)
	case monitortestframework.Disruptive:
		return newDisruptiveMonitorTests()
	default:
		panic(fmt.Sprintf("unknown cluster stability level: %q", info.ClusterStabilityDuringTest))
	}
}

func newDefaultMonitorTests(info monitortestframework.MonitorTestInitializationInfo) monitortestframework.MonitorTestRegistry {
	monitorTestRegistry := monitortestframework.NewMonitorTestRegistry()

	monitorTestRegistry.AddRegistryOrDie(newUniversalMonitorTests())

	monitorTestRegistry.AddMonitorTestOrDie("image-registry-availability", "Image Registry", disruptionimageregistry.NewAvailabilityInvariant())

	monitorTestRegistry.AddMonitorTestOrDie("apiserver-availability", "kube-apiserver", disruptionlegacyapiservers.NewAvailabilityInvariant())

	monitorTestRegistry.AddMonitorTestOrDie("pod-network-avalibility", "Network / ovn-kubernetes", disruptionpodnetwork.NewPodNetworkAvalibilityInvariant(info))
	monitorTestRegistry.AddMonitorTestOrDie("service-type-load-balancer-availability", "NetworkEdge", disruptionserviceloadbalancer.NewAvailabilityInvariant())
	monitorTestRegistry.AddMonitorTestOrDie("ingress-availability", "NetworkEdge", disruptioningress.NewAvailabilityInvariant())

	monitorTestRegistry.AddMonitorTestOrDie("external-service-availability", "Test Framework", disruptionexternalservicemonitoring.NewAvailabilityInvariant())

	return monitorTestRegistry
}

func newDisruptiveMonitorTests() monitortestframework.MonitorTestRegistry {
	monitorTestRegistry := monitortestframework.NewMonitorTestRegistry()

	monitorTestRegistry.AddRegistryOrDie(newUniversalMonitorTests())

	monitorTestRegistry.AddMonitorTestOrDie("image-registry-availability", "Image Registry", disruptionimageregistry.NewRecordAvailabilityOnly())

	monitorTestRegistry.AddMonitorTestOrDie("apiserver-availability", "kube-apiserver", disruptionlegacyapiservers.NewRecordAvailabilityOnly())

	monitorTestRegistry.AddMonitorTestOrDie("service-type-load-balancer-availability", "NetworkEdge", disruptionserviceloadbalancer.NewRecordAvailabilityOnly())
	monitorTestRegistry.AddMonitorTestOrDie("ingress-availability", "NetworkEdge", disruptioningress.NewRecordAvailabilityOnly())

	monitorTestRegistry.AddMonitorTestOrDie("external-service-availability", "Test Framework", disruptionexternalservicemonitoring.NewRecordAvailabilityOnly())

	return monitorTestRegistry
}

func newUniversalMonitorTests() monitortestframework.MonitorTestRegistry {
	monitorTestRegistry := monitortestframework.NewMonitorTestRegistry()

	monitorTestRegistry.AddMonitorTestOrDie("legacy-authentication-invariants", "Authentication", legacyauthenticationmonitortests.NewLegacyTests())

	monitorTestRegistry.AddMonitorTestOrDie("legacy-cvo-invariants", "Cluster Version Operator", legacycvomonitortests.NewLegacyTests())
	monitorTestRegistry.AddMonitorTestOrDie("operator-state-analyzer", "Cluster Version Operator", operatorstateanalyzer.NewAnalyzer())

	monitorTestRegistry.AddMonitorTestOrDie("etcd-log-analyzer", "etcd", etcdloganalyzer.NewEtcdLogAnalyzer())
	monitorTestRegistry.AddMonitorTestOrDie("legacy-etcd-invariants", "etcd", legacyetcdmonitortests.NewLegacyTests())

	monitorTestRegistry.AddMonitorTestOrDie("audit-log-analyzer", "kube-apiserver", auditloganalyzer.NewAuditLogAnalyzer())
	monitorTestRegistry.AddMonitorTestOrDie("apiserver-new-disruption-invariant", "kube-apiserver", disruptionnewapiserver.NewDisruptionInvariant())
	monitorTestRegistry.AddMonitorTestOrDie("legacy-kube-apiserver-invariants", "kube-apiserver", legacykubeapiservermonitortests.NewLegacyTests())
	monitorTestRegistry.AddMonitorTestOrDie("graceful-shutdown-analyzer", "kube-apiserver", apiservergracefulrestart.NewGracefulShutdownAnalyzer())

	monitorTestRegistry.AddMonitorTestOrDie("legacy-networking-invariants", "Networking", legacynetworkmonitortests.NewLegacyTests())

	monitorTestRegistry.AddMonitorTestOrDie("kubelet-log-collector", "Node", kubeletlogcollector.NewKubeletLogCollector())
	monitorTestRegistry.AddMonitorTestOrDie("legacy-node-invariants", "Node", legacynodemonitortests.NewLegacyTests())
	monitorTestRegistry.AddMonitorTestOrDie("node-state-analyzer", "Node", nodestateanalyzer.NewAnalyzer())
	monitorTestRegistry.AddMonitorTestOrDie("pod-lifecycle", "Node", watchpods.NewPodWatcher())
	monitorTestRegistry.AddMonitorTestOrDie("node-lifecycle", "Node", watchnodes.NewNodeWatcher())

	monitorTestRegistry.AddMonitorTestOrDie("legacy-storage-invariants", "Storage", legacystoragemonitortests.NewLegacyTests())

	monitorTestRegistry.AddMonitorTestOrDie("legacy-test-framework-invariants", "Test Framework", legacytestframeworkmonitortests.NewLegacyTests())
	monitorTestRegistry.AddMonitorTestOrDie("pathological-event-analyzer", "Test Framework", pathologicaleventanalyzer.NewAnalyzer())
	monitorTestRegistry.AddMonitorTestOrDie("timeline-serializer", "Test Framework", timelineserializer.NewTimelineSerializer())
	monitorTestRegistry.AddMonitorTestOrDie("interval-serializer", "Test Framework", intervalserializer.NewIntervalSerializer())
	monitorTestRegistry.AddMonitorTestOrDie("tracked-resources-serializer", "Test Framework", trackedresourcesserializer.NewTrackedResourcesSerializer())
	monitorTestRegistry.AddMonitorTestOrDie("disruption-summary-serializer", "Test Framework", disruptionserializer.NewDisruptionSummarySerializer())
	monitorTestRegistry.AddMonitorTestOrDie("alert-summary-serializer", "Test Framework", alertanalyzer.NewAlertSummarySerializer())
	monitorTestRegistry.AddMonitorTestOrDie("cluster-info-serializer", "Test Framework", clusterinfoserializer.NewClusterInfoSerializer())
	monitorTestRegistry.AddMonitorTestOrDie("additional-events-collector", "Test Framework", additionaleventscollector.NewIntervalSerializer())
	monitorTestRegistry.AddMonitorTestOrDie("known-image-checker", "Test Framework", knownimagechecker.NewEnsureValidImages())
	monitorTestRegistry.AddMonitorTestOrDie("upload-to-loki-serializer", "Test Framework", uploadtolokiserializer.NewUploadSerializer())
	monitorTestRegistry.AddMonitorTestOrDie("e2e-test-analyzer", "Test Framework", e2etestanalyzer.NewAnalyzer())
	monitorTestRegistry.AddMonitorTestOrDie("event-collector", "Test Framework", watchevents.NewEventWatcher())
	monitorTestRegistry.AddMonitorTestOrDie("clusteroperator-collector", "Test Framework", watchclusteroperators.NewOperatorWatcher())

	return monitorTestRegistry
}
