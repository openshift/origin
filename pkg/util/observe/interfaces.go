package observe

import "time"

type ResourceVersionObserver interface {
	// ObserveResourceVersion waits until the given resourceVersion is observed, up to the specified timeout.
	ObserveResourceVersion(resourceVersion string, timeout time.Duration) error
}
