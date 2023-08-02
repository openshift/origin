package defaultinvariants

import (
	"fmt"

	"github.com/openshift/origin/pkg/invariants/disruptionnewapiserver"

	"github.com/openshift/origin/pkg/invariants/etcdloganalyzer"

	"github.com/openshift/origin/pkg/invariants/auditloganalyzer"

	"github.com/openshift/origin/pkg/invariants/kubeletlogcollector"

	"github.com/openshift/origin/pkg/invariants/uploadtolokiserializer"

	"github.com/openshift/origin/pkg/invariants"
	"github.com/openshift/origin/pkg/invariants/additionaleventscollector"
	"github.com/openshift/origin/pkg/invariants/alertserializer"
	"github.com/openshift/origin/pkg/invariants/clusterinfoserializer"
	"github.com/openshift/origin/pkg/invariants/disruptionimageregistry"
	"github.com/openshift/origin/pkg/invariants/disruptionlegacyapiservers"
	"github.com/openshift/origin/pkg/invariants/disruptionserializer"
	"github.com/openshift/origin/pkg/invariants/disruptionserviceloadbalancer"
	"github.com/openshift/origin/pkg/invariants/intervalserializer"
	"github.com/openshift/origin/pkg/invariants/knownimagechecker"
	"github.com/openshift/origin/pkg/invariants/timelineserializer"
	"github.com/openshift/origin/pkg/invariants/trackedresourcesserializer"
	"github.com/openshift/origin/pkg/invariants/watchpods"
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

	invariantTests.AddInvariantOrDie("service-type-load-balancer-availability", "NetworkEdge", disruptionserviceloadbalancer.NewAvailabilityInvariant())

	invariantTests.AddInvariantOrDie("image-registry-availability", "Image Registry", disruptionimageregistry.NewAvailabilityInvariant())

	invariantTests.AddInvariantOrDie("apiserver-availability", "kube-apiserver", disruptionlegacyapiservers.NewAvailabilityInvariant())

	return invariantTests
}

func newDisruptiveInvariants() invariants.InvariantRegistry {
	invariantTests := invariants.NewInvariantRegistry()

	invariantTests.AddRegistryOrDie(newUniversalInvariants())

	invariantTests.AddInvariantOrDie("service-type-load-balancer-availability", "NetworkEdge", disruptionserviceloadbalancer.NewRecordAvailabilityOnly())

	invariantTests.AddInvariantOrDie("image-registry-availability", "Image Registry", disruptionimageregistry.NewRecordAvailabilityOnly())

	invariantTests.AddInvariantOrDie("apiserver-availability", "kube-apiserver", disruptionlegacyapiservers.NewRecordAvailabilityOnly())

	return invariantTests
}

func newUniversalInvariants() invariants.InvariantRegistry {
	invariantTests := invariants.NewInvariantRegistry()

	invariantTests.AddInvariantOrDie("pod-lifecycle", "Test Framework", watchpods.NewPodWatcher())
	invariantTests.AddInvariantOrDie("timeline-serializer", "Test Framework", timelineserializer.NewTimelineSerializer())
	invariantTests.AddInvariantOrDie("interval-serializer", "Test Framework", intervalserializer.NewIntervalSerializer())
	invariantTests.AddInvariantOrDie("tracked-resources-serializer", "Test Framework", trackedresourcesserializer.NewTrackedResourcesSerializer())
	invariantTests.AddInvariantOrDie("disruption-summary-serializer", "Test Framework", disruptionserializer.NewDisruptionSummarySerializer())
	invariantTests.AddInvariantOrDie("alert-summary-serializer", "Test Framework", alertserializer.NewAlertSummarySerializer())
	invariantTests.AddInvariantOrDie("cluster-info-serializer", "Test Framework", clusterinfoserializer.NewClusterInfoSerializer())
	invariantTests.AddInvariantOrDie("additional-events-collector", "Test Framework", additionaleventscollector.NewIntervalSerializer())
	invariantTests.AddInvariantOrDie("known-image-checker", "Test Framework", knownimagechecker.NewEnsureValidImages())
	invariantTests.AddInvariantOrDie("upload-to-loki-serializer", "Test Framework", uploadtolokiserializer.NewUploadSerializer())

	invariantTests.AddInvariantOrDie("kubelet-log-collector", "Node", kubeletlogcollector.NewKubeletLogCollector())

	invariantTests.AddInvariantOrDie("audit-log-analyzer", "kube-apiserver", auditloganalyzer.NewAuditLogAnalyzer())
	invariantTests.AddInvariantOrDie("apiserver-new-disruption-invariant", "kube-apiserver", disruptionnewapiserver.NewDisruptionInvariant())
	invariantTests.AddInvariantOrDie("etcd-log-analyzer", "etcd", etcdloganalyzer.NewEtcdLogAnalyzer())

	return invariantTests
}
