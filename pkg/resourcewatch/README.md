# Resource Watch

The `run-resourcewatch` command observes cluster resources and records their state changes over time.

## Event Collection

The `run-resourcewatch` command accepts an `--enable-events` flag that controls whether `events.k8s.io/v1`
events are included in the resource watch. By default, event collection is disabled to reduce the volume
of data collected. Pass `--enable-events` to include events:

    openshift-tests run-resourcewatch --enable-events

In CI, the `openshift/release` repo controls how `run-resourcewatch` is invoked and can pass this flag
via environment variable plumbing in the step registry.
