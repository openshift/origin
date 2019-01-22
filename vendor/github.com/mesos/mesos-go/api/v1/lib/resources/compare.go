package resources

import (
	"github.com/gogo/protobuf/proto"
	"github.com/mesos/mesos-go/api/v1/lib"
)

func Validate(resources ...mesos.Resource) error {
	type withSpec interface {
		WithSpec(mesos.Resource)
	}
	for i := range resources {
		err := resources[i].Validate()
		if err != nil {
			// augment resourceError's with the resource that failed to validate
			if resourceError, ok := err.(withSpec); ok {
				r := proto.Clone(&resources[i]).(*mesos.Resource)
				resourceError.WithSpec(*r)
			}
			return err
		}
	}
	return nil
}

func Equivalent(subject, that []mesos.Resource) bool {
	return ContainsAll(subject, that) && ContainsAll(that, subject)
}

func Contains(subject []mesos.Resource, that mesos.Resource) bool {
	// NOTE: We must validate 'that' because invalid resources can lead
	// to false positives here (e.g., "cpus:-1" will return true). This
	// is because 'contains' assumes resources are valid.
	return that.Validate() == nil && contains(subject, that)
}

func contains(subject []mesos.Resource, that mesos.Resource) bool {
	// TODO(jdef): take into account the "count" of shared resources
	for i := range subject {
		if subject[i].Contains(that) {
			return true
		}
	}
	return false
}

// ContainsAll returns true if this set of resources contains that set of (presumably pre-validated) resources.
func ContainsAll(subject, that []mesos.Resource) bool {
	remaining := mesos.Resources(subject).Clone()
	for i := range that {
		// NOTE: We use contains() because resources only contain valid
		// Resource objects, and we don't want the performance hit of the
		// validity check.
		if !contains(remaining, that[i]) {
			return false
		}
		if that[i].GetDisk().GetPersistence() != nil {
			remaining.Subtract1(that[i])
		}
	}
	return true
}
