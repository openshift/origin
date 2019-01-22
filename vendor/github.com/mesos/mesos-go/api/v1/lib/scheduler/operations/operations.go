package operations

import (
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/mesos/mesos-go/api/v1/lib"
	rez "github.com/mesos/mesos-go/api/v1/lib/resources"
)

type (
	operationErrorType int

	operationError struct {
		errorType operationErrorType
		opType    mesos.Offer_Operation_Type
		cause     error
	}

	offerResourceOp func(_ *mesos.Offer_Operation, res mesos.Resources, converted mesos.Resources) (mesos.Resources, error)
)

const (
	operationErrorTypeInvalid operationErrorType = iota
	operationErrorTypeUnknown
	operationErrorTypeMayNotBeApplied
)

var (
	offerResourceOpMap = func() (m map[mesos.Offer_Operation_Type]offerResourceOp) {
		opUnsupported := func(t mesos.Offer_Operation_Type) {
			m[t] = func(_ *mesos.Offer_Operation, _ mesos.Resources, _ mesos.Resources) (_ mesos.Resources, err error) {
				err = &operationError{errorType: operationErrorTypeMayNotBeApplied, opType: t}
				return
			}
		}
		invalidOp := func(t mesos.Offer_Operation_Type, op offerResourceOp) offerResourceOp {
			return func(operation *mesos.Offer_Operation, resources mesos.Resources, conv mesos.Resources) (mesos.Resources, error) {
				r, err := op(operation, resources, conv)
				if err != nil {
					err = &operationError{errorType: operationErrorTypeInvalid, opType: t, cause: err}
				}
				return r, err
			}
		}
		opRegister := func(t mesos.Offer_Operation_Type, op offerResourceOp) { m[t] = invalidOp(t, op) }

		m = make(map[mesos.Offer_Operation_Type]offerResourceOp)

		opUnsupported(mesos.Offer_Operation_LAUNCH)
		opUnsupported(mesos.Offer_Operation_LAUNCH_GROUP)

		// TODO(jdef) does it make sense to validate op totals for all operation types?

		opRegister(mesos.Offer_Operation_RESERVE, opReserve)
		opRegister(mesos.Offer_Operation_UNRESERVE, opUnreserve)
		opRegister(mesos.Offer_Operation_CREATE, opCreate)
		opRegister(mesos.Offer_Operation_DESTROY, opDestroy)
		opRegister(mesos.Offer_Operation_GROW_VOLUME, opGrowVolume)
		opRegister(mesos.Offer_Operation_SHRINK_VOLUME, opShrinkVolume)
		opRegister(mesos.Offer_Operation_CREATE_DISK, opCreateDisk)
		opRegister(mesos.Offer_Operation_DESTROY_DISK, opDestroyDisk)

		return
	}()
)

func (err *operationError) Cause() error                          { return err.cause }
func (err *operationError) Type() operationErrorType              { return err.errorType }
func (err *operationError) Operation() mesos.Offer_Operation_Type { return err.opType }

func (err *operationError) Error() string {
	switch err.errorType {
	case operationErrorTypeInvalid:
		return fmt.Sprintf("invalid "+err.opType.String()+" operation: %+v", err.cause)
	case operationErrorTypeUnknown:
		return err.cause.Error()
	default:
		return fmt.Sprintf("operation error: %+v", err.cause)
	}
}

// popReservation works for both pre- and post-reservation-refinement resources.
// Returns a shallow (mutated) copy of the resource.
func popReservation(r mesos.Resource) mesos.Resource {
	switch x := len(r.Reservations); {
	case x == 0:
		// pre-refinement format.
		r.Role = nil
		r.Reservation = nil
	case x == 1:
		// handle the special case whereby both formats are used redundantly
		r.Role = nil
		r.Reservation = nil
		r.Reservations = nil
	case x > 1:
		// post-refinement format.
		// duplicate the slice, truncated. we don't want the optimized form of
		// the "delete the last object from a slice" because it would mutate the
		// contents of the original reservation (in an attempt to avoid a mem leak).
		rs := make([]mesos.Resource_ReservationInfo, 0, x-1)
		copy(rs, r.Reservations[:x-1])
		r.Reservations = rs
	}
	return r
}

func opReserve(operation *mesos.Offer_Operation, resources mesos.Resources, conv mesos.Resources) (mesos.Resources, error) {
	if len(conv) > 0 {
		return nil, fmt.Errorf("converted resources not expected")
	}
	opRes := operation.GetReserve().GetResources()
	err := rez.Validate(opRes...)
	if err != nil {
		return nil, err
	}
	result := resources.Clone()
	for i := range opRes {
		if !opRes[i].IsReserved("") {
			return nil, errors.New("Resource must be reserved")
		}
		if !opRes[i].IsDynamicallyReserved() {
			return nil, errors.New("Resource must be dynamically reserved")
		}

		lessReserved := popReservation(opRes[i])

		if !rez.Contains(result, lessReserved) {
			return nil, fmt.Errorf("%+v does not contain %+v", result, lessReserved)
		}

		result.Subtract(lessReserved)
		result.Add1(opRes[i])
	}
	return result, nil
}

func opUnreserve(operation *mesos.Offer_Operation, resources mesos.Resources, conv mesos.Resources) (mesos.Resources, error) {
	if len(conv) > 0 {
		return nil, fmt.Errorf("converted resources not expected")
	}
	opRes := operation.GetUnreserve().GetResources()
	err := rez.Validate(opRes...)
	if err != nil {
		return nil, err
	}
	result := resources.Clone()
	for i := range opRes {
		if !opRes[i].IsReserved("") {
			return nil, errors.New("Resource is not reserved")
		}
		if !opRes[i].IsDynamicallyReserved() {
			return nil, errors.New("Resource must be dynamically reserved")
		}
		if !rez.Contains(result, opRes[i]) {
			return nil, errors.New("resources do not contain unreserve amount") //TODO(jdef) should output nicely formatted resource quantities here
		}
		lessReserved := popReservation(opRes[i])
		result.Subtract1(opRes[i])
		result.Add(lessReserved)
	}
	return result, nil
}

func opCreate(operation *mesos.Offer_Operation, resources mesos.Resources, conv mesos.Resources) (mesos.Resources, error) {
	if len(conv) > 0 {
		return nil, fmt.Errorf("converted resources not expected")
	}
	volumes := operation.GetCreate().GetVolumes()
	err := rez.Validate(volumes...)
	if err != nil {
		return nil, err
	}
	result := resources.Clone()
	for i := range volumes {
		if volumes[i].GetDisk() == nil {
			return nil, errors.New("missing 'disk'")
		}
		if volumes[i].GetDisk().GetPersistence() == nil {
			return nil, errors.New("missing 'persistence'")
		}
		// from: https://github.com/apache/mesos/blob/master/src/common/resources.cpp
		// Strip the disk info so that we can subtract it from the
		// original resources.
		// TODO(jieyu): Non-persistent volumes are not supported for
		// now. Persistent volumes can only be be created from regular
		// disk resources. Revisit this once we start to support
		// non-persistent volumes.
		stripped := proto.Clone(&volumes[i]).(*mesos.Resource)
		if stripped.Disk.Source != nil {
			stripped.Disk.Persistence = nil
			stripped.Disk.Volume = nil
		} else {
			stripped.Disk = nil
		}

		// Since we only allow persistent volumes to be shared, the
		// original resource must be non-shared.
		stripped.Shared = nil

		if !rez.Contains(result, *stripped) {
			return nil, errors.New("insufficient disk resources")
		}
		result.Subtract1(*stripped)
		result.Add1(volumes[i])
	}
	return result, nil
}

func opDestroy(operation *mesos.Offer_Operation, resources mesos.Resources, conv mesos.Resources) (mesos.Resources, error) {
	if len(conv) > 0 {
		return nil, fmt.Errorf("converted resources not expected")
	}
	volumes := operation.GetDestroy().GetVolumes()
	err := rez.Validate(volumes...)
	if err != nil {
		return nil, err
	}
	result := resources.Clone()
	for i := range volumes {
		if volumes[i].GetDisk() == nil {
			return nil, errors.New("missing 'disk'")
		}
		if volumes[i].GetDisk().GetPersistence() == nil {
			return nil, errors.New("missing 'persistence'")
		}
		if !rez.Contains(result, volumes[i]) {
			return nil, errors.New("persistent volume does not exist")
		}

		result.Subtract1(volumes[i])

		if rez.Contains(result, volumes[i]) {
			return nil, fmt.Errorf(
				"persistent volume %q cannot be removed due to additional shared copies", volumes[i])
		}

		// Strip the disk info so that we can subtract it from the
		// original resources.
		stripped := proto.Clone(&volumes[i]).(*mesos.Resource)
		if stripped.Disk.Source != nil {
			stripped.Disk.Persistence = nil
			stripped.Disk.Volume = nil
		} else {
			stripped.Disk = nil
		}

		// Since we only allow persistent volumes to be shared, we
		// return the resource to non-shared state after destroy.
		stripped.Shared = nil

		result.Add1(*stripped)
	}
	return result, nil
}

func opGrowVolume(operation *mesos.Offer_Operation, resources mesos.Resources, conv mesos.Resources) (mesos.Resources, error) {
	if len(conv) == 0 {
		return nil, fmt.Errorf("converted resources not specified")
	}

	result := resources.Clone()
	consumed := mesos.Resources{
		operation.GetGrowVolume().GetVolume(),
		operation.GetGrowVolume().GetAddition(),
	}

	if !rez.ContainsAll(result, consumed) {
		return nil, fmt.Errorf("%q does not contain %q", result, consumed)
	}

	result.Subtract(consumed...)
	result.Add(conv...)

	return result, nil
}

func opShrinkVolume(operation *mesos.Offer_Operation, resources mesos.Resources, conv mesos.Resources) (mesos.Resources, error) {
	if len(conv) == 0 {
		return nil, fmt.Errorf("converted resources not specified")
	}

	result := resources.Clone()
	consumed := operation.GetShrinkVolume().GetVolume()

	if !rez.Contains(result, consumed) {
		return nil, fmt.Errorf("%q does not contain %q", result, consumed)
	}

	result.Subtract1(consumed)
	result.Add(conv...)

	return result, nil
}

func opCreateDisk(operation *mesos.Offer_Operation, resources mesos.Resources, conv mesos.Resources) (mesos.Resources, error) {
	if len(conv) == 0 {
		return nil, fmt.Errorf("converted resources not specified")
	}

	result := resources.Clone()
	consumed := operation.GetCreateDisk().GetSource()

	if !rez.Contains(result, consumed) {
		return nil, fmt.Errorf("%q does not contain %q", result, consumed)
	}

	result.Subtract1(consumed)
	result.Add(conv...)

	return result, nil
}

func opDestroyDisk(operation *mesos.Offer_Operation, resources mesos.Resources, conv mesos.Resources) (mesos.Resources, error) {
	// NOTE: `conv` would be empty if the source disks have stale profiles.

	result := resources.Clone()
	consumed := operation.GetDestroyDisk().GetSource()

	if !rez.Contains(result, consumed) {
		return nil, fmt.Errorf("%q does not contain %q", result, consumed)
	}

	result.Subtract1(consumed)
	result.Add(conv...)

	return result, nil
}

func Apply(operation *mesos.Offer_Operation, resources []mesos.Resource, convertedResources []mesos.Resource) (result []mesos.Resource, err error) {
	f, ok := offerResourceOpMap[operation.GetType()]
	if !ok {
		return nil, &operationError{
			errorType: operationErrorTypeUnknown,
			cause:     errors.New("unknown offer operation: " + operation.GetType().String()),
		}
	}
	result, err = f(operation, resources, convertedResources)
	if err == nil {
		// sanity CHECK, same as apache/mesos does
		if !rez.SumAndCompare(resources, result...) {
			panic(fmt.Sprintf("result %+v != resources %+v", result, resources))
		}
	}
	return result, err
}
