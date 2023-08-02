package defaultinvariants

import (
	"fmt"

	"github.com/openshift/origin/pkg/invariants"
	"github.com/openshift/origin/pkg/invariants/authentication/legacyauthenticationinvariants"
	"github.com/openshift/origin/pkg/invariants/clusterversionoperator/legacycvoinvariants"
	"github.com/openshift/origin/pkg/invariants/clusterversionoperator/operatorstateanalyzer"
	"github.com/openshift/origin/pkg/invariants/disruptionpodnetwork"
	"github.com/openshift/origin/pkg/invariants/etcd/etcdloganalyzer"
	"github.com/openshift/origin/pkg/invariants/etcd/legacyetcdinvariants"
	"github.com/openshift/origin/pkg/invariants/imageregistry/disruptionimageregistry"
	"github.com/openshift/origin/pkg/invariants/kubeapiserver/auditloganalyzer"
	"github.com/openshift/origin/pkg/invariants/kubeapiserver/disruptionlegacyapiservers"
	"github.com/openshift/origin/pkg/invariants/kubeapiserver/disruptionnewapiserver"
	"github.com/openshift/origin/pkg/invariants/kubeapiserver/legacykubeapiserverinvariants"
	"github.com/openshift/origin/pkg/invariants/network/disruptioningress"
	"github.com/openshift/origin/pkg/invariants/network/disruptionserviceloadbalancer"
	"github.com/openshift/origin/pkg/invariants/network/legacynetworkinvariants"
	"github.com/openshift/origin/pkg/invariants/node/kubeletlogcollector"
	"github.com/openshift/origin/pkg/invariants/node/legacynodeinvariants"
	"github.com/openshift/origin/pkg/invariants/node/nodestateanalyzer"
	"github.com/openshift/origin/pkg/invariants/node/watchpods"
	"github.com/openshift/origin/pkg/invariants/storage/legacystorageinvariants"
	"github.com/openshift/origin/pkg/invariants/testframework/additionaleventscollector"
	"github.com/openshift/origin/pkg/invariants/testframework/alertanalyzer"
	"github.com/openshift/origin/pkg/invariants/testframework/clusterinfoserializer"
	"github.com/openshift/origin/pkg/invariants/testframework/disruptionexternalservicemonitoring"
	"github.com/openshift/origin/pkg/invariants/testframework/disruptionserializer"
	"github.com/openshift/origin/pkg/invariants/testframework/e2etestanalyzer"
	"github.com/openshift/origin/pkg/invariants/testframework/intervalserializer"
	"github.com/openshift/origin/pkg/invariants/testframework/knownimagechecker"
	"github.com/openshift/origin/pkg/invariants/testframework/legacytestframeworkinvariants"
	"github.com/openshift/origin/pkg/invariants/testframework/pathologicaleventanalyzer"
	"github.com/openshift/origin/pkg/invariants/testframework/timelineserializer"
	"github.com/openshift/origin/pkg/invariants/testframework/trackedresourcesserializer"
	"github.com/openshift/origin/pkg/invariants/testframework/uploadtolokiserializer"
)

type ClusterStabilityDuringTest string

var (
	// Stable means that at no point during testing do we expect a component to take downtime and upgrades are not happening.
	Stable ClusterStabilityDuringTest = "Stable"
	// TODO only bring this back if we have some reason to collect Upgrade specific information.  I can't think of reason.
	// TODO please contact @deads2k for vetting if you think you found something
	//Upgrade    ClusterStabilityDuringTest = "Upgrade"
	// Disruptive means that the suite is expected to induce outages to the cluster.
	Disruptive ClusterStabilityDuringTest = "Disruptive"
)

func NewInvariantsFor(clusterStabilityDuringTest ClusterStabilityDuringTest) invariants.InvariantRegistry {
	switch clusterStabilityDuringTest {
	case Stable:
		return newDefaultInvariants()
	case Disruptive:
		return newDisruptiveInvariants()
	default:
		panic(fmt.Sprintf("unknown cluster stability level: %q", clusterStabilityDuringTest))
	}
}

func newDefaultInvariants() invariants.InvariantRegistry {
	invariantTests := invariants.NewInvariantRegistry()

	invariantTests.AddRegistryOrDie(newUniversalInvariants())

	invariantTests.AddInvariantOrDie("image-registry-availability", "Image Registry", disruptionimageregistry.NewAvailabilityInvariant())

	invariantTests.AddInvariantOrDie("apiserver-availability", "kube-apiserver", disruptionlegacyapiservers.NewAvailabilityInvariant())

	invariantTests.AddInvariantOrDie("service-type-load-balancer-availability", "NetworkEdge", disruptionserviceloadbalancer.NewAvailabilityInvariant())
	invariantTests.AddInvariantOrDie("ingress-availability", "NetworkEdge", disruptioningress.NewAvailabilityInvariant())

	invariantTests.AddInvariantOrDie("external-service-availability", "Test Framework", disruptionexternalservicemonitoring.NewAvailabilityInvariant())

	return invariantTests
}

func newDisruptiveInvariants() invariants.InvariantRegistry {
	invariantTests := invariants.NewInvariantRegistry()

	invariantTests.AddRegistryOrDie(newUniversalInvariants())

	invariantTests.AddInvariantOrDie("image-registry-availability", "Image Registry", disruptionimageregistry.NewRecordAvailabilityOnly())

	invariantTests.AddInvariantOrDie("apiserver-availability", "kube-apiserver", disruptionlegacyapiservers.NewRecordAvailabilityOnly())

	invariantTests.AddInvariantOrDie("service-type-load-balancer-availability", "NetworkEdge", disruptionserviceloadbalancer.NewRecordAvailabilityOnly())
	invariantTests.AddInvariantOrDie("ingress-availability", "NetworkEdge", disruptioningress.NewRecordAvailabilityOnly())
	invariantTests.AddInvariantOrDie(disruptionpodnetwork.InvariantName, disruptionpodnetwork.JIRAOwner, disruptionpodnetwork.NewPodNetworkAvalibilityInvariant())

	invariantTests.AddInvariantOrDie("external-service-availability", "Test Framework", disruptionexternalservicemonitoring.NewRecordAvailabilityOnly())

	return invariantTests
}

func newUniversalInvariants() invariants.InvariantRegistry {
	invariantTests := invariants.NewInvariantRegistry()

	invariantTests.AddInvariantOrDie("legacy-authentication-invariants", "Authentication", legacyauthenticationinvariants.NewLegacyTests())

	invariantTests.AddInvariantOrDie("legacy-cvo-invariants", "Cluster Version Operator", legacycvoinvariants.NewLegacyTests())
	invariantTests.AddInvariantOrDie("operator-state-analyzer", "Cluster Version Operator", operatorstateanalyzer.NewAnalyzer())

	invariantTests.AddInvariantOrDie("etcd-log-analyzer", "etcd", etcdloganalyzer.NewEtcdLogAnalyzer())
	invariantTests.AddInvariantOrDie("legacy-etcd-invariants", "etcd", legacyetcdinvariants.NewLegacyTests())

	invariantTests.AddInvariantOrDie("audit-log-analyzer", "kube-apiserver", auditloganalyzer.NewAuditLogAnalyzer())
	invariantTests.AddInvariantOrDie("apiserver-new-disruption-invariant", "kube-apiserver", disruptionnewapiserver.NewDisruptionInvariant())
	invariantTests.AddInvariantOrDie("legacy-kube-apiserver-invariants", "kube-apiserver", legacykubeapiserverinvariants.NewLegacyTests())

	invariantTests.AddInvariantOrDie("legacy-networking-invariants", "Networking", legacynetworkinvariants.NewLegacyTests())

	invariantTests.AddInvariantOrDie("kubelet-log-collector", "Node", kubeletlogcollector.NewKubeletLogCollector())
	invariantTests.AddInvariantOrDie("legacy-node-invariants", "Node", legacynodeinvariants.NewLegacyTests())
	invariantTests.AddInvariantOrDie("node-state-analyzer", "Node", nodestateanalyzer.NewAnalyzer())
	invariantTests.AddInvariantOrDie("pod-lifecycle", "Node", watchpods.NewPodWatcher())

	invariantTests.AddInvariantOrDie("legacy-storage-invariants", "Storage", legacystorageinvariants.NewLegacyTests())

	invariantTests.AddInvariantOrDie("legacy-test-framework-invariants", "Test Framework", legacytestframeworkinvariants.NewLegacyTests())
	invariantTests.AddInvariantOrDie("pathological-event-analyzer", "Test Framework", pathologicaleventanalyzer.NewAnalyzer())
	invariantTests.AddInvariantOrDie("timeline-serializer", "Test Framework", timelineserializer.NewTimelineSerializer())
	invariantTests.AddInvariantOrDie("interval-serializer", "Test Framework", intervalserializer.NewIntervalSerializer())
	invariantTests.AddInvariantOrDie("tracked-resources-serializer", "Test Framework", trackedresourcesserializer.NewTrackedResourcesSerializer())
	invariantTests.AddInvariantOrDie("disruption-summary-serializer", "Test Framework", disruptionserializer.NewDisruptionSummarySerializer())
	invariantTests.AddInvariantOrDie("alert-summary-serializer", "Test Framework", alertanalyzer.NewAlertSummarySerializer())
	invariantTests.AddInvariantOrDie("cluster-info-serializer", "Test Framework", clusterinfoserializer.NewClusterInfoSerializer())
	invariantTests.AddInvariantOrDie("additional-events-collector", "Test Framework", additionaleventscollector.NewIntervalSerializer())
	invariantTests.AddInvariantOrDie("known-image-checker", "Test Framework", knownimagechecker.NewEnsureValidImages())
	invariantTests.AddInvariantOrDie("upload-to-loki-serializer", "Test Framework", uploadtolokiserializer.NewUploadSerializer())
	invariantTests.AddInvariantOrDie("e2e-test-analyzer", "Test Framework", e2etestanalyzer.NewAnalyzer())

	return invariantTests
}
