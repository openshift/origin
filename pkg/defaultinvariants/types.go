package defaultinvariants

import (
	"github.com/openshift/origin/pkg/invariants"
	"github.com/openshift/origin/pkg/invariants/additional_events_collector"
	"github.com/openshift/origin/pkg/invariants/alert_serializer"
	"github.com/openshift/origin/pkg/invariants/availability_image_registry"
	"github.com/openshift/origin/pkg/invariants/availability_service_loadbalancer"
	"github.com/openshift/origin/pkg/invariants/clusterinfo_serializer"
	"github.com/openshift/origin/pkg/invariants/disruption_serializer"
	"github.com/openshift/origin/pkg/invariants/interval_serializer"
	"github.com/openshift/origin/pkg/invariants/timeline_serializer"
	"github.com/openshift/origin/pkg/invariants/trackedresources_serializer"
	watchpods "github.com/openshift/origin/pkg/invariants/watch_pods"
)

func NewDefaultInvariants() invariants.InvariantRegistry {
	invariantTests := invariants.NewInvariantRegistry()

	// TODO add invariantTests here
	invariantTests.AddInvariantOrDie("pod-lifecycle", "Test Framework", watchpods.NewPodWatcher())
	invariantTests.AddInvariantOrDie("timeline-serializer", "Test Framework", timeline_serializer.NewTimelineSerializer())
	invariantTests.AddInvariantOrDie("interval-serializer", "Test Framework", interval_serializer.NewIntervalSerializer())
	invariantTests.AddInvariantOrDie("tracked-resources-serializer", "Test Framework", trackedresources_serializer.NewTrackedResourcesSerializer())
	invariantTests.AddInvariantOrDie("disruption-summary-serializer", "Test Framework", disruption_serializer.NewDisruptionSummarySerializer())
	invariantTests.AddInvariantOrDie("alert-summary-serializer", "Test Framework", alert_serializer.NewAlertSummarySerializer())
	invariantTests.AddInvariantOrDie("cluster-info-serializer", "Test Framework", clusterinfo_serializer.NewClusterInfoSerializer())
	invariantTests.AddInvariantOrDie("additional-events-collector", "Test Framework", additional_events_collector.NewIntervalSerializer())

	invariantTests.AddInvariantOrDie("service-type-load-balancer-availability", "NetworkEdge", availability_service_loadbalancer.NewAvailabilityInvariant())

	invariantTests.AddInvariantOrDie("image-registry-availability", "Image Registry", availability_image_registry.NewAvailabilityInvariant())

	return invariantTests
}
