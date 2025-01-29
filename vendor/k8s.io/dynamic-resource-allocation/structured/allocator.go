/*
Copyright 2024 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package structured

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	v1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	"k8s.io/apimachinery/pkg/util/sets"
	draapi "k8s.io/dynamic-resource-allocation/api"
	"k8s.io/dynamic-resource-allocation/cel"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

type deviceClassLister interface {
	// List returns a list of all DeviceClasses.
	List() ([]*resourceapi.DeviceClass, error)
	// Get returns the DeviceClass with the given className.
	Get(className string) (*resourceapi.DeviceClass, error)
}

// Allocator calculates how to allocate a set of unallocated claims which use
// structured parameters.
//
// It needs as input the node where the allocated claims are meant to be
// available and the current state of the cluster (claims, classes, resource
// slices).
type Allocator struct {
	adminAccessEnabled bool
	claimsToAllocate   []*resourceapi.ResourceClaim
	allocatedDevices   sets.Set[DeviceID]
	classLister        deviceClassLister
	slices             []*resourceapi.ResourceSlice
	celCache           *cel.Cache
}

// NewAllocator returns an allocator for a certain set of claims or an error if
// some problem was detected which makes it impossible to allocate claims.
//
// The returned Allocator can be used multiple times and is thread-safe.
func NewAllocator(ctx context.Context,
	adminAccessEnabled bool,
	claimsToAllocate []*resourceapi.ResourceClaim,
	allocatedDevices sets.Set[DeviceID],
	classLister deviceClassLister,
	slices []*resourceapi.ResourceSlice,
	celCache *cel.Cache,
) (*Allocator, error) {
	return &Allocator{
		adminAccessEnabled: adminAccessEnabled,
		claimsToAllocate:   claimsToAllocate,
		allocatedDevices:   allocatedDevices,
		classLister:        classLister,
		slices:             slices,
		celCache:           celCache,
	}, nil
}

// ClaimsToAllocate returns the claims that the allocator was created for.
func (a *Allocator) ClaimsToAllocate() []*resourceapi.ResourceClaim {
	return a.claimsToAllocate
}

// Allocate calculates the allocation(s) for one particular node.
//
// It returns an error only if some fatal problem occurred. These are errors
// caused by invalid input data, like for example errors in CEL selectors, so a
// scheduler should abort and report that problem instead of trying to find
// other nodes where the error doesn't occur.
//
// In the future, special errors will be defined which enable the caller to
// identify which object (like claim or class) caused the problem. This will
// enable reporting the problem as event for those objects.
//
// If the claims cannot be allocated, it returns nil. This includes the
// situation where the resource slices are incomplete at the moment.
//
// If the claims can be allocated, then it prepares one allocation result for
// each unallocated claim. It is the responsibility of the caller to persist
// those allocations, if desired.
//
// Allocate is thread-safe. If the caller wants to get the node name included
// in log output, it can use contextual logging and add the node as an
// additional value. A name can also be useful because log messages do not
// have a common prefix. V(5) is used for one-time log entries, V(6) for important
// progress reports, and V(7) for detailed debug output.
func (a *Allocator) Allocate(ctx context.Context, node *v1.Node) (finalResult []resourceapi.AllocationResult, finalErr error) {
	alloc := &allocator{
		Allocator:            a,
		ctx:                  ctx, // all methods share the same a and thus ctx
		logger:               klog.FromContext(ctx),
		deviceMatchesRequest: make(map[matchKey]bool),
		constraints:          make([][]constraint, len(a.claimsToAllocate)),
		requestData:          make(map[requestIndices]requestData),
		result:               make([]internalAllocationResult, len(a.claimsToAllocate)),
	}
	alloc.logger.V(5).Info("Starting allocation", "numClaims", len(alloc.claimsToAllocate))
	defer alloc.logger.V(5).Info("Done with allocation", "success", len(finalResult) == len(alloc.claimsToAllocate), "err", finalErr)

	// First determine all eligible pools.
	pools, err := GatherPools(ctx, alloc.slices, node)
	if err != nil {
		return nil, fmt.Errorf("gather pool information: %w", err)
	}
	alloc.pools = pools
	if loggerV := alloc.logger.V(7); loggerV.Enabled() {
		loggerV.Info("Gathered pool information", "numPools", len(pools), "pools", pools)
	} else {
		alloc.logger.V(5).Info("Gathered pool information", "numPools", len(pools))
	}

	// We allocate one claim after the other and for each claim, all of
	// its requests. For each individual device we pick one possible
	// candidate after the other, checking constraints as we go.
	// Each chosen candidate is marked as "in use" and the process
	// continues, recursively. This way, all requests get matched against
	// all candidates in all possible orders.
	//
	// The first full solution is chosen.
	//
	// In other words, this is an exhaustive search. This is okay because
	// it aborts early. Once scoring gets added, more intelligence may be
	// needed to avoid trying "equivalent" solutions (two identical
	// requests, two identical devices, two solutions that are the same in
	// practice).

	// This is where we sanity check that we can actually handle the claims
	// and their requests. For each claim we determine how many devices
	// need to be allocated. If not all can be stored in the result, the
	// claim cannot be allocated.
	numDevicesTotal := 0
	for claimIndex, claim := range alloc.claimsToAllocate {
		numDevicesPerClaim := 0

		// If we have any any request that wants "all" devices, we need to
		// figure out how much "all" is. If some pool is incomplete, we stop
		// here because allocation cannot succeed. Once we do scoring, we should
		// stop in all cases, not just when "all" devices are needed, because
		// pulling from an incomplete might not pick the best solution and it's
		// better to wait. This does not matter yet as long the incomplete pool
		// has some matching device.
		for requestIndex := range claim.Spec.Devices.Requests {
			request := &claim.Spec.Devices.Requests[requestIndex]
			for i, selector := range request.Selectors {
				if selector.CEL == nil {
					// Unknown future selector type!
					return nil, fmt.Errorf("claim %s, request %s, selector #%d: CEL expression empty (unsupported selector type?)", klog.KObj(claim), request.Name, i)
				}
			}

			if !a.adminAccessEnabled && request.AdminAccess != nil {
				return nil, fmt.Errorf("claim %s, request %s: admin access is requested, but the feature is disabled", klog.KObj(claim), request.Name)
			}

			// Should be set. If it isn't, something changed and we should refuse to proceed.
			if request.DeviceClassName == "" {
				return nil, fmt.Errorf("claim %s, request %s: missing device class name (unsupported request type?)", klog.KObj(claim), request.Name)
			}
			class, err := alloc.classLister.Get(request.DeviceClassName)
			if err != nil {
				return nil, fmt.Errorf("claim %s, request %s: could not retrieve device class %s: %w", klog.KObj(claim), request.Name, request.DeviceClassName, err)
			}

			// Start collecting information about the request.
			// The class must be set and stored before calling isSelectable.
			requestData := requestData{
				class: class,
			}
			requestKey := requestIndices{claimIndex: claimIndex, requestIndex: requestIndex}
			alloc.requestData[requestKey] = requestData

			switch request.AllocationMode {
			case resourceapi.DeviceAllocationModeExactCount:
				numDevices := request.Count
				if numDevices > math.MaxInt {
					// Allowed by API validation, but doesn't make sense.
					return nil, fmt.Errorf("claim %s, request %s: exact count %d is too large", klog.KObj(claim), request.Name, numDevices)
				}
				requestData.numDevices = int(numDevices)
			case resourceapi.DeviceAllocationModeAll:
				requestData.allDevices = make([]deviceWithID, 0, resourceapi.AllocationResultsMaxSize)
				for _, pool := range pools {
					if pool.IsIncomplete {
						return nil, fmt.Errorf("claim %s, request %s: asks for all devices, but resource pool %s is currently being updated", klog.KObj(claim), request.Name, pool.PoolID)
					}
					if pool.IsInvalid {
						return nil, fmt.Errorf("claim %s, request %s: asks for all devices, but resource pool %s is currently invalid", klog.KObj(claim), request.Name, pool.PoolID)
					}

					for _, slice := range pool.Slices {
						for deviceIndex := range slice.Spec.Devices {
							selectable, err := alloc.isSelectable(requestKey, slice, deviceIndex)
							if err != nil {
								return nil, err
							}
							if selectable {
								device := deviceWithID{
									id:    DeviceID{Driver: slice.Spec.Driver, Pool: slice.Spec.Pool.Name, Device: slice.Spec.Devices[deviceIndex].Name},
									basic: slice.Spec.Devices[deviceIndex].Basic,
									slice: slice,
								}
								requestData.allDevices = append(requestData.allDevices, device)
							}
						}
					}
				}
				requestData.numDevices = len(requestData.allDevices)
				alloc.logger.V(6).Info("Request for 'all' devices", "claim", klog.KObj(claim), "request", request.Name, "numDevicesPerRequest", requestData.numDevices)
			default:
				return nil, fmt.Errorf("claim %s, request %s: unsupported count mode %s", klog.KObj(claim), request.Name, request.AllocationMode)
			}
			alloc.requestData[requestKey] = requestData
			numDevicesPerClaim += requestData.numDevices
		}
		alloc.logger.V(6).Info("Checked claim", "claim", klog.KObj(claim), "numDevices", numDevicesPerClaim)

		// Check that we don't end up with too many results.
		if numDevicesPerClaim > resourceapi.AllocationResultsMaxSize {
			return nil, fmt.Errorf("claim %s: number of requested devices %d exceeds the claim limit of %d", klog.KObj(claim), numDevicesPerClaim, resourceapi.AllocationResultsMaxSize)
		}

		// If we don't, then we can pre-allocate the result slices for
		// appending the actual results later.
		alloc.result[claimIndex].devices = make([]internalDeviceResult, 0, numDevicesPerClaim)

		// Constraints are assumed to be monotonic: once a constraint returns
		// false, adding more devices will not cause it to return true. This
		// allows the search to stop early once a constraint returns false.
		constraints := make([]constraint, len(claim.Spec.Devices.Constraints))
		for i, constraint := range claim.Spec.Devices.Constraints {
			switch {
			case constraint.MatchAttribute != nil:
				matchAttribute := draapi.FullyQualifiedName(*constraint.MatchAttribute)
				logger := alloc.logger
				if loggerV := alloc.logger.V(6); loggerV.Enabled() {
					logger = klog.LoggerWithName(logger, "matchAttributeConstraint")
					logger = klog.LoggerWithValues(logger, "matchAttribute", matchAttribute)
				}
				m := &matchAttributeConstraint{
					logger:        logger,
					requestNames:  sets.New(constraint.Requests...),
					attributeName: matchAttribute,
				}
				constraints[i] = m
			default:
				// Unknown constraint type!
				return nil, fmt.Errorf("claim %s, constraint #%d: empty constraint (unsupported constraint type?)", klog.KObj(claim), i)
			}
		}
		alloc.constraints[claimIndex] = constraints
		numDevicesTotal += numDevicesPerClaim
	}

	// Selecting a device for a request is independent of what has been
	// allocated already. Therefore the result of checking a request against
	// a device instance in the pool can be cached. The pointer to both
	// can serve as key because they are static for the duration of
	// the Allocate call and can be compared in Go.
	alloc.deviceMatchesRequest = make(map[matchKey]bool)

	// We can estimate the size based on what we need to allocate.
	alloc.allocatingDevices = make(map[DeviceID]bool, numDevicesTotal)

	alloc.logger.V(6).Info("Gathered information about devices", "numAllocated", len(alloc.allocatedDevices), "toBeAllocated", numDevicesTotal)

	// In practice, there aren't going to be many different CEL
	// expressions. Most likely, there is going to be handful of different
	// device classes that get used repeatedly. Different requests may all
	// use the same selector. Therefore compiling CEL expressions on demand
	// could be a useful performance enhancement. It's not implemented yet
	// because the key is more complex (just the string?) and the memory
	// for both key and cached content is larger than for device matches.
	//
	// We may also want to cache this in the shared [Allocator] instance,
	// which implies adding locking.

	// All errors get created such that they can be returned by Allocate
	// without further wrapping.
	done, err := alloc.allocateOne(deviceIndices{})
	if errors.Is(err, errStop) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !done {
		return nil, nil
	}

	result := make([]resourceapi.AllocationResult, len(alloc.result))
	for claimIndex, internalResult := range alloc.result {
		claim := alloc.claimsToAllocate[claimIndex]
		allocationResult := &result[claimIndex]
		allocationResult.Devices.Results = make([]resourceapi.DeviceRequestAllocationResult, len(internalResult.devices))
		for i, internal := range internalResult.devices {
			allocationResult.Devices.Results[i] = resourceapi.DeviceRequestAllocationResult{
				Request:     internal.request,
				Driver:      internal.id.Driver.String(),
				Pool:        internal.id.Pool.String(),
				Device:      internal.id.Device.String(),
				AdminAccess: internal.adminAccess,
			}
		}

		// Populate configs.
		for requestIndex := range claim.Spec.Devices.Requests {
			class := alloc.requestData[requestIndices{claimIndex: claimIndex, requestIndex: requestIndex}].class
			if class != nil {
				for _, config := range class.Spec.Config {
					allocationResult.Devices.Config = append(allocationResult.Devices.Config, resourceapi.DeviceAllocationConfiguration{
						Source:              resourceapi.AllocationConfigSourceClass,
						Requests:            nil, // All of them...
						DeviceConfiguration: config.DeviceConfiguration,
					})
				}
			}
		}
		for _, config := range claim.Spec.Devices.Config {
			allocationResult.Devices.Config = append(allocationResult.Devices.Config, resourceapi.DeviceAllocationConfiguration{
				Source:              resourceapi.AllocationConfigSourceClaim,
				Requests:            config.Requests,
				DeviceConfiguration: config.DeviceConfiguration,
			})
		}

		// Determine node selector.
		nodeSelector, err := alloc.createNodeSelector(internalResult.devices)
		if err != nil {
			return nil, fmt.Errorf("create NodeSelector for claim %s: %w", claim.Name, err)
		}
		allocationResult.NodeSelector = nodeSelector
	}

	return result, nil
}

// errStop is a special error that gets returned by allocateOne if it detects
// that allocation cannot succeed.
var errStop = errors.New("stop allocation")

// allocator is used while an [Allocator.Allocate] is running. Only a single
// goroutine works with it, so there is no need for locking.
type allocator struct {
	*Allocator
	ctx                  context.Context
	logger               klog.Logger
	pools                []*Pool
	deviceMatchesRequest map[matchKey]bool
	constraints          [][]constraint                 // one list of constraints per claim
	requestData          map[requestIndices]requestData // one entry per request
	allocatingDevices    map[DeviceID]bool
	result               []internalAllocationResult
}

// matchKey identifies a device/request pair.
type matchKey struct {
	DeviceID
	requestIndices
}

// requestIndices identifies one specific request by its
// claim and request index.
type requestIndices struct {
	claimIndex, requestIndex int
}

// deviceIndices identifies one specific required device inside
// a request of a certain claim.
type deviceIndices struct {
	claimIndex, requestIndex, deviceIndex int
}

type requestData struct {
	class      *resourceapi.DeviceClass
	numDevices int

	// pre-determined set of devices for allocating "all" devices
	allDevices []deviceWithID
}

type deviceWithID struct {
	id    DeviceID
	basic *draapi.BasicDevice
	slice *draapi.ResourceSlice
}

type internalAllocationResult struct {
	devices []internalDeviceResult
}

type internalDeviceResult struct {
	request     string
	id          DeviceID
	slice       *draapi.ResourceSlice
	adminAccess *bool
}

type constraint interface {
	// add is called whenever a device is about to be allocated. It must
	// check whether the device matches the constraint and if yes,
	// track that it is allocated.
	add(requestName string, device *draapi.BasicDevice, deviceID DeviceID) bool

	// For every successful add there is exactly one matching removed call
	// with the exact same parameters.
	remove(requestName string, device *draapi.BasicDevice, deviceID DeviceID)
}

// matchAttributeConstraint compares an attribute value across devices.
// All devices must share the same value. When the set of devices is
// empty, any device that has the attribute can be added. After that,
// only matching devices can be added.
//
// We don't need to track *which* devices are part of the set, only
// how many.
type matchAttributeConstraint struct {
	logger        klog.Logger // Includes name and attribute name, so no need to repeat in log messages.
	requestNames  sets.Set[string]
	attributeName draapi.FullyQualifiedName

	attribute  *draapi.DeviceAttribute
	numDevices int
}

func (m *matchAttributeConstraint) add(requestName string, device *draapi.BasicDevice, deviceID DeviceID) bool {
	if m.requestNames.Len() > 0 && !m.requestNames.Has(requestName) {
		// Device not affected by constraint.
		m.logger.V(7).Info("Constraint does not apply to request", "request", requestName)
		return true
	}

	attribute := lookupAttribute(device, deviceID, m.attributeName)
	if attribute == nil {
		// Doesn't have the attribute.
		m.logger.V(7).Info("Constraint not satisfied, attribute not set")
		return false
	}

	if m.numDevices == 0 {
		// The first device can always get picked.
		m.attribute = attribute
		m.numDevices = 1
		m.logger.V(7).Info("First in set")
		return true
	}

	switch {
	case attribute.StringValue != nil:
		if m.attribute.StringValue == nil || *attribute.StringValue != *m.attribute.StringValue {
			m.logger.V(7).Info("String values different")
			return false
		}
	case attribute.IntValue != nil:
		if m.attribute.IntValue == nil || *attribute.IntValue != *m.attribute.IntValue {
			m.logger.V(7).Info("Int values different")
			return false
		}
	case attribute.BoolValue != nil:
		if m.attribute.BoolValue == nil || *attribute.BoolValue != *m.attribute.BoolValue {
			m.logger.V(7).Info("Bool values different")
			return false
		}
	case attribute.VersionValue != nil:
		// semver 2.0.0 requires that version strings are in their
		// minimal form (in particular, no leading zeros). Therefore a
		// strict "exact equal" check can do a string comparison.
		if m.attribute.VersionValue == nil || *attribute.VersionValue != *m.attribute.VersionValue {
			m.logger.V(7).Info("Version values different")
			return false
		}
	default:
		// Unknown value type, cannot match.
		m.logger.V(7).Info("Match attribute type unknown")
		return false
	}

	m.numDevices++
	m.logger.V(7).Info("Constraint satisfied by device", "device", deviceID, "numDevices", m.numDevices)
	return true
}

func (m *matchAttributeConstraint) remove(requestName string, device *draapi.BasicDevice, deviceID DeviceID) {
	if m.requestNames.Len() > 0 && !m.requestNames.Has(requestName) {
		// Device not affected by constraint.
		return
	}

	m.numDevices--
	m.logger.V(7).Info("Device removed from constraint set", "device", deviceID, "numDevices", m.numDevices)
}

func lookupAttribute(device *draapi.BasicDevice, deviceID DeviceID, attributeName draapi.FullyQualifiedName) *draapi.DeviceAttribute {
	// Fully-qualified match?
	if attr, ok := device.Attributes[draapi.QualifiedName(attributeName)]; ok {
		return &attr
	}
	index := strings.Index(string(attributeName), "/")
	if index < 0 {
		// Should not happen for a valid fully qualified name.
		return nil
	}

	if string(attributeName[0:index]) != deviceID.Driver.String() {
		// Not an attribute of the driver and not found above,
		// so it is not available.
		return nil
	}

	// Domain matches the driver, so let's check just the ID.
	if attr, ok := device.Attributes[draapi.QualifiedName(attributeName[index+1:])]; ok {
		return &attr
	}

	return nil
}

// allocateOne iterates over all eligible devices (not in use, match selector,
// satisfy constraints) for a specific required device. It returns true if
// everything got allocated, an error if allocation needs to stop.
func (alloc *allocator) allocateOne(r deviceIndices) (bool, error) {
	if r.claimIndex >= len(alloc.claimsToAllocate) {
		// Done! If we were doing scoring, we would compare the current allocation result
		// against the previous one, keep the best, and continue. Without scoring, we stop
		// and use the first solution.
		alloc.logger.V(6).Info("Allocation result found")
		return true, nil
	}

	claim := alloc.claimsToAllocate[r.claimIndex]
	if r.requestIndex >= len(claim.Spec.Devices.Requests) {
		// Done with the claim, continue with the next one.
		return alloc.allocateOne(deviceIndices{claimIndex: r.claimIndex + 1})
	}

	// We already know how many devices per request are needed.
	// Ready to move on to the next request?
	requestData := alloc.requestData[requestIndices{claimIndex: r.claimIndex, requestIndex: r.requestIndex}]
	if r.deviceIndex >= requestData.numDevices {
		return alloc.allocateOne(deviceIndices{claimIndex: r.claimIndex, requestIndex: r.requestIndex + 1})
	}

	request := &alloc.claimsToAllocate[r.claimIndex].Spec.Devices.Requests[r.requestIndex]
	doAllDevices := request.AllocationMode == resourceapi.DeviceAllocationModeAll
	alloc.logger.V(6).Info("Allocating one device", "currentClaim", r.claimIndex, "totalClaims", len(alloc.claimsToAllocate), "currentRequest", r.requestIndex, "totalRequestsPerClaim", len(claim.Spec.Devices.Requests), "currentDevice", r.deviceIndex, "devicesPerRequest", requestData.numDevices, "allDevices", doAllDevices, "adminAccess", request.AdminAccess)

	if doAllDevices {
		// For "all" devices we already know which ones we need. We
		// just need to check whether we can use them.
		deviceWithID := requestData.allDevices[r.deviceIndex]
		success, _, err := alloc.allocateDevice(r, deviceWithID, true)
		if err != nil {
			return false, err
		}
		if !success {
			// The order in which we allocate "all" devices doesn't matter,
			// so we only try with the one which was up next. If we couldn't
			// get all of them, then there is no solution and we have to stop.
			return false, errStop
		}
		done, err := alloc.allocateOne(deviceIndices{claimIndex: r.claimIndex, requestIndex: r.requestIndex, deviceIndex: r.deviceIndex + 1})
		if err != nil {
			return false, err
		}
		if !done {
			// Backtrack.
			return false, nil
		}
		return done, nil
	}

	// We need to find suitable devices.
	for _, pool := range alloc.pools {
		for _, slice := range pool.Slices {
			for deviceIndex := range slice.Spec.Devices {
				deviceID := DeviceID{Driver: pool.Driver, Pool: pool.Pool, Device: slice.Spec.Devices[deviceIndex].Name}

				// Checking for "in use" is cheap and thus gets done first.
				if !ptr.Deref(request.AdminAccess, false) && (alloc.allocatedDevices.Has(deviceID) || alloc.allocatingDevices[deviceID]) {
					alloc.logger.V(7).Info("Device in use", "device", deviceID)
					continue
				}

				// Next check selectors.
				selectable, err := alloc.isSelectable(requestIndices{claimIndex: r.claimIndex, requestIndex: r.requestIndex}, slice, deviceIndex)
				if err != nil {
					return false, err
				}
				if !selectable {
					alloc.logger.V(7).Info("Device not selectable", "device", deviceID)
					continue
				}

				// If the pool is not valid, then fail now. It's okay when pools of one driver
				// are invalid if we allocate from some other pool, but it's not safe to
				// allocated from an invalid pool.
				if pool.IsInvalid {
					return false, fmt.Errorf("pool %s is invalid: %s", pool.Pool, pool.InvalidReason)
				}

				// Finally treat as allocated and move on to the next device.
				device := deviceWithID{
					id:    deviceID,
					basic: slice.Spec.Devices[deviceIndex].Basic,
					slice: slice,
				}
				allocated, deallocate, err := alloc.allocateDevice(r, device, false)
				if err != nil {
					return false, err
				}
				if !allocated {
					// In use or constraint violated...
					alloc.logger.V(7).Info("Device not usable", "device", deviceID)
					continue
				}
				done, err := alloc.allocateOne(deviceIndices{claimIndex: r.claimIndex, requestIndex: r.requestIndex, deviceIndex: r.deviceIndex + 1})
				if err != nil {
					return false, err
				}

				// If we found a solution, then we can stop.
				if done {
					return done, nil
				}

				// Otherwise try some other device after rolling back.
				deallocate()
			}
		}
	}

	// If we get here without finding a solution, then there is none.
	return false, nil
}

// isSelectable checks whether a device satisfies the request and class selectors.
func (alloc *allocator) isSelectable(r requestIndices, slice *draapi.ResourceSlice, deviceIndex int) (bool, error) {
	// This is the only supported device type at the moment.
	device := slice.Spec.Devices[deviceIndex].Basic
	if device == nil {
		// Must be some future, unknown device type. We cannot select it.
		return false, nil
	}

	deviceID := DeviceID{Driver: slice.Spec.Driver, Pool: slice.Spec.Pool.Name, Device: slice.Spec.Devices[deviceIndex].Name}
	matchKey := matchKey{DeviceID: deviceID, requestIndices: r}
	if matches, ok := alloc.deviceMatchesRequest[matchKey]; ok {
		// No need to check again.
		return matches, nil
	}

	requestData := alloc.requestData[r]
	if requestData.class != nil {
		match, err := alloc.selectorsMatch(r, device, deviceID, requestData.class, requestData.class.Spec.Selectors)
		if err != nil {
			return false, err
		}
		if !match {
			alloc.deviceMatchesRequest[matchKey] = false
			return false, nil
		}
	}

	request := &alloc.claimsToAllocate[r.claimIndex].Spec.Devices.Requests[r.requestIndex]
	match, err := alloc.selectorsMatch(r, device, deviceID, nil, request.Selectors)
	if err != nil {
		return false, err
	}
	if !match {
		alloc.deviceMatchesRequest[matchKey] = false
		return false, nil
	}

	alloc.deviceMatchesRequest[matchKey] = true
	return true, nil

}

func (alloc *allocator) selectorsMatch(r requestIndices, device *draapi.BasicDevice, deviceID DeviceID, class *resourceapi.DeviceClass, selectors []resourceapi.DeviceSelector) (bool, error) {
	for i, selector := range selectors {
		expr := alloc.celCache.GetOrCompile(selector.CEL.Expression)
		if expr.Error != nil {
			// Could happen if some future apiserver accepted some
			// future expression and then got downgraded. Normally
			// the "stored expression" mechanism prevents that, but
			// this code here might be more than one release older
			// than the cluster it runs in.
			if class != nil {
				return false, fmt.Errorf("class %s: selector #%d: CEL compile error: %w", class.Name, i, expr.Error)
			}
			return false, fmt.Errorf("claim %s: selector #%d: CEL compile error: %w", klog.KObj(alloc.claimsToAllocate[r.claimIndex]), i, expr.Error)
		}

		// If this conversion turns out to be expensive, the CEL package could be converted
		// to use unique strings.
		var d resourceapi.BasicDevice
		if err := draapi.Convert_api_BasicDevice_To_v1beta1_BasicDevice(device, &d, nil); err != nil {
			return false, fmt.Errorf("convert BasicDevice: %w", err)
		}
		matches, details, err := expr.DeviceMatches(alloc.ctx, cel.Device{Driver: deviceID.Driver.String(), Attributes: d.Attributes, Capacity: d.Capacity})
		if class != nil {
			alloc.logger.V(7).Info("CEL result", "device", deviceID, "class", klog.KObj(class), "selector", i, "expression", selector.CEL.Expression, "matches", matches, "actualCost", ptr.Deref(details.ActualCost(), 0), "err", err)
		} else {
			alloc.logger.V(7).Info("CEL result", "device", deviceID, "claim", klog.KObj(alloc.claimsToAllocate[r.claimIndex]), "selector", i, "expression", selector.CEL.Expression, "actualCost", ptr.Deref(details.ActualCost(), 0), "matches", matches, "err", err)
		}

		if err != nil {
			// TODO (future): more detailed errors which reference class resp. claim.
			if class != nil {
				return false, fmt.Errorf("class %s: selector #%d: CEL runtime error: %w", class.Name, i, err)
			}
			return false, fmt.Errorf("claim %s: selector #%d: CEL runtime error: %w", klog.KObj(alloc.claimsToAllocate[r.claimIndex]), i, err)
		}
		if !matches {
			return false, nil
		}
	}

	// All of them match.
	return true, nil
}

// allocateDevice checks device availability and constraints for one
// candidate. The device must be selectable.
//
// If that candidate works out okay, the shared state gets updated
// as if that candidate had been allocated. If allocation cannot continue later
// and must try something else, then the rollback function can be invoked to
// restore the previous state.
func (alloc *allocator) allocateDevice(r deviceIndices, device deviceWithID, must bool) (bool, func(), error) {
	claim := alloc.claimsToAllocate[r.claimIndex]
	request := &claim.Spec.Devices.Requests[r.requestIndex]
	adminAccess := ptr.Deref(request.AdminAccess, false)
	if !adminAccess && (alloc.allocatedDevices.Has(device.id) || alloc.allocatingDevices[device.id]) {
		alloc.logger.V(7).Info("Device in use", "device", device.id)
		return false, nil, nil
	}

	// It's available. Now check constraints.
	for i, constraint := range alloc.constraints[r.claimIndex] {
		added := constraint.add(request.Name, device.basic, device.id)
		if !added {
			if must {
				// It does not make sense to declare a claim where a constraint prevents getting
				// all devices. Treat this as an error.
				return false, nil, fmt.Errorf("claim %s, request %s: cannot add device %s because a claim constraint would not be satisfied", klog.KObj(claim), request.Name, device.id)
			}

			// Roll back for all previous constraints before we return.
			for e := 0; e < i; e++ {
				alloc.constraints[r.claimIndex][e].remove(request.Name, device.basic, device.id)
			}
			return false, nil, nil
		}
	}

	// All constraints satisfied. Mark as in use (unless we do admin access)
	// and record the result.
	alloc.logger.V(7).Info("Device allocated", "device", device.id)
	if !adminAccess {
		alloc.allocatingDevices[device.id] = true
	}
	result := internalDeviceResult{
		request: request.Name,
		id:      device.id,
		slice:   device.slice,
	}
	if adminAccess {
		result.adminAccess = &adminAccess
	}
	previousNumResults := len(alloc.result[r.claimIndex].devices)
	alloc.result[r.claimIndex].devices = append(alloc.result[r.claimIndex].devices, result)

	return true, func() {
		for _, constraint := range alloc.constraints[r.claimIndex] {
			constraint.remove(request.Name, device.basic, device.id)
		}
		if !adminAccess {
			alloc.allocatingDevices[device.id] = false
		}
		// Truncate, but keep the underlying slice.
		alloc.result[r.claimIndex].devices = alloc.result[r.claimIndex].devices[:previousNumResults]
		alloc.logger.V(7).Info("Device deallocated", "device", device.id)
	}, nil
}

// createNodeSelector constructs a node selector for the allocation, if needed,
// otherwise it returns nil.
func (alloc *allocator) createNodeSelector(result []internalDeviceResult) (*v1.NodeSelector, error) {
	// Selector with one term. That term gets extended with additional
	// requirements from the different devices.
	nodeSelector := &v1.NodeSelector{
		NodeSelectorTerms: []v1.NodeSelectorTerm{{}},
	}

	for i := range result {
		slice := result[i].slice
		if slice.Spec.NodeName != draapi.NullUniqueString {
			// At least one device is local to one node. This
			// restricts the allocation to that node.
			return &v1.NodeSelector{
				NodeSelectorTerms: []v1.NodeSelectorTerm{{
					MatchFields: []v1.NodeSelectorRequirement{{
						Key:      "metadata.name",
						Operator: v1.NodeSelectorOpIn,
						Values:   []string{slice.Spec.NodeName.String()},
					}},
				}},
			}, nil
		}
		if slice.Spec.NodeSelector != nil {
			switch len(slice.Spec.NodeSelector.NodeSelectorTerms) {
			case 0:
				// Nothing?
			case 1:
				// Add all terms if they are not present already.
				addNewNodeSelectorRequirements(slice.Spec.NodeSelector.NodeSelectorTerms[0].MatchFields, &nodeSelector.NodeSelectorTerms[0].MatchFields)
				addNewNodeSelectorRequirements(slice.Spec.NodeSelector.NodeSelectorTerms[0].MatchExpressions, &nodeSelector.NodeSelectorTerms[0].MatchExpressions)
			default:
				// This shouldn't occur, validation must prevent creation of such slices.
				return nil, fmt.Errorf("unsupported ResourceSlice.NodeSelector with %d terms", len(slice.Spec.NodeSelector.NodeSelectorTerms))
			}
		}
	}

	if len(nodeSelector.NodeSelectorTerms[0].MatchFields) > 0 || len(nodeSelector.NodeSelectorTerms[0].MatchExpressions) > 0 {
		// We have a valid node selector.
		return nodeSelector, nil
	}

	// Available everywhere.
	return nil, nil
}

func addNewNodeSelectorRequirements(from []v1.NodeSelectorRequirement, to *[]v1.NodeSelectorRequirement) {
	for _, requirement := range from {
		if !containsNodeSelectorRequirement(*to, requirement) {
			*to = append(*to, requirement)
		}
	}
}

func containsNodeSelectorRequirement(requirements []v1.NodeSelectorRequirement, requirement v1.NodeSelectorRequirement) bool {
	values := sets.New(requirement.Values...)
	for _, existingRequirement := range requirements {
		if existingRequirement.Key != requirement.Key {
			continue
		}
		if existingRequirement.Operator != requirement.Operator {
			continue
		}
		if !sets.New(existingRequirement.Values...).Equal(values) {
			continue
		}
		return true
	}
	return false
}
