package defaultinvariants

import (
	"github.com/openshift/origin/pkg/invariants"
	"github.com/openshift/origin/pkg/invariants/additionaleventscollector"
	"github.com/openshift/origin/pkg/invariants/alertserializer"
	"github.com/openshift/origin/pkg/invariants/clusterinfoserializer"
	"github.com/openshift/origin/pkg/invariants/disruptionimageregistry"
	"github.com/openshift/origin/pkg/invariants/disruptionserializer"
	"github.com/openshift/origin/pkg/invariants/disruptionserviceloadbalancer"
	"github.com/openshift/origin/pkg/invariants/intervalserializer"
	"github.com/openshift/origin/pkg/invariants/timelineserializer"
	"github.com/openshift/origin/pkg/invariants/trackedresourcesserializer"
	"github.com/openshift/origin/pkg/invariants/watchpods"
)

func NewDefaultInvariants() invariants.InvariantRegistry {
	invariantTests := invariants.NewInvariantRegistry()

	// TODO add invariantTests here
	invariantTests.AddInvariantOrDie("pod-lifecycle", "Test Framework", watchpods.NewPodWatcher())
	invariantTests.AddInvariantOrDie("timeline-serializer", "Test Framework", timelineserializer.NewTimelineSerializer())
	invariantTests.AddInvariantOrDie("interval-serializer", "Test Framework", intervalserializer.NewIntervalSerializer())
	invariantTests.AddInvariantOrDie("tracked-resources-serializer", "Test Framework", trackedresourcesserializer.NewTrackedResourcesSerializer())
	invariantTests.AddInvariantOrDie("disruption-summary-serializer", "Test Framework", disruptionserializer.NewDisruptionSummarySerializer())
	invariantTests.AddInvariantOrDie("alert-summary-serializer", "Test Framework", alertserializer.NewAlertSummarySerializer())
	invariantTests.AddInvariantOrDie("cluster-info-serializer", "Test Framework", clusterinfoserializer.NewClusterInfoSerializer())
	invariantTests.AddInvariantOrDie("additional-events-collector", "Test Framework", additionaleventscollector.NewIntervalSerializer())

	invariantTests.AddInvariantOrDie("service-type-load-balancer-availability", "NetworkEdge", disruptionserviceloadbalancer.NewAvailabilityInvariant())

	invariantTests.AddInvariantOrDie("image-registry-availability", "Image Registry", disruptionimageregistry.NewAvailabilityInvariant())

	return invariantTests
}
