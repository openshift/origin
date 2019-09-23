package calls

import (
	"errors"
	"math/rand"
	"time"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
)

// Filters sets a scheduler.Call's internal Filters, required for Accept and Decline calls.
func Filters(fo ...mesos.FilterOpt) scheduler.CallOpt {
	return func(c *scheduler.Call) {
		switch c.Type {
		case scheduler.Call_ACCEPT:
			c.Accept.Filters = mesos.OptionalFilters(fo...)
		case scheduler.Call_ACCEPT_INVERSE_OFFERS:
			c.AcceptInverseOffers.Filters = mesos.OptionalFilters(fo...)
		case scheduler.Call_DECLINE:
			c.Decline.Filters = mesos.OptionalFilters(fo...)
		case scheduler.Call_DECLINE_INVERSE_OFFERS:
			c.DeclineInverseOffers.Filters = mesos.OptionalFilters(fo...)
		default:
			panic("filters not supported for type " + c.Type.String())
		}
	}
}

// RefuseSecondsWithJitter returns a calls.Filters option that sets RefuseSeconds to a random number
// of seconds between 0 and the given duration.
func RefuseSecondsWithJitter(r *rand.Rand, d time.Duration) scheduler.CallOpt {
	return Filters(func(f *mesos.Filters) {
		s := time.Duration(r.Int63n(int64(d))).Seconds()
		f.RefuseSeconds = &s
	})
}

// RefuseSeconds returns a calls.Filters option that sets RefuseSeconds to the given duration
func RefuseSeconds(d time.Duration) scheduler.CallOpt {
	asFloat := d.Seconds()
	return Filters(func(f *mesos.Filters) {
		f.RefuseSeconds = &asFloat
	})
}

// Framework sets a scheduler.Call's FrameworkID
func Framework(id string) scheduler.CallOpt {
	return func(c *scheduler.Call) {
		c.FrameworkID = &mesos.FrameworkID{Value: id}
	}
}

// Subscribe returns a subscribe call with the given parameters.
// The call's FrameworkID is automatically filled in from the info specification.
func Subscribe(info *mesos.FrameworkInfo) *scheduler.Call {
	return &scheduler.Call{
		Type:        scheduler.Call_SUBSCRIBE,
		FrameworkID: info.GetID(),
		Subscribe:   &scheduler.Call_Subscribe{FrameworkInfo: info},
	}
}

// SubscribeTo returns an option that configures a SUBSCRIBE call w/ a framework ID.
// If frameworkID is "" then the SUBSCRIBE call is cleared of all framework ID references.
// Panics if the call does not contain a non-nil Subscribe reference.
func SubscribeTo(frameworkID string) scheduler.CallOpt {
	return func(call *scheduler.Call) {
		if call.Subscribe == nil {
			panic("illegal call option: Call.Subscribe was unexpectedly nil")
		}
		var frameworkProto *mesos.FrameworkID
		if frameworkID != "" {
			frameworkProto = &mesos.FrameworkID{Value: frameworkID}
		}
		call.Subscribe.FrameworkInfo.ID = frameworkProto
		call.FrameworkID = frameworkProto
	}
}

type acceptBuilder struct {
	offerIDs   map[mesos.OfferID]struct{}
	operations []mesos.Offer_Operation
}

type AcceptOpt func(*acceptBuilder)

type OfferOperations []mesos.Offer_Operation

// WithOffers allows a client to pair some set of OfferOperations with multiple resource offers.
// Example: calls.Accept(calls.OfferOperations{calls.OpLaunch(tasks...)}.WithOffers(offers...))
func (ob OfferOperations) WithOffers(ids ...mesos.OfferID) AcceptOpt {
	return func(ab *acceptBuilder) {
		for i := range ids {
			ab.offerIDs[ids[i]] = struct{}{}
		}
		ab.operations = append(ab.operations, ob...)
	}
}

// Accept returns an accept call with the given parameters.
// Callers are expected to fill in the FrameworkID and Filters.
func Accept(ops ...AcceptOpt) *scheduler.Call {
	ab := &acceptBuilder{
		offerIDs: make(map[mesos.OfferID]struct{}, len(ops)),
	}
	for _, op := range ops {
		op(ab)
	}
	offerIDs := make([]mesos.OfferID, 0, len(ab.offerIDs))
	for id := range ab.offerIDs {
		offerIDs = append(offerIDs, id)
	}
	return &scheduler.Call{
		Type: scheduler.Call_ACCEPT,
		Accept: &scheduler.Call_Accept{
			OfferIDs:   offerIDs,
			Operations: ab.operations,
		},
	}
}

// AcceptInverseOffers returns an accept-inverse-offers call for the given offer IDs.
// Callers are expected to fill in the FrameworkID and Filters.
func AcceptInverseOffers(offerIDs ...mesos.OfferID) *scheduler.Call {
	return &scheduler.Call{
		Type: scheduler.Call_ACCEPT_INVERSE_OFFERS,
		AcceptInverseOffers: &scheduler.Call_AcceptInverseOffers{
			InverseOfferIDs: offerIDs,
		},
	}
}

// DeclineInverseOffers returns a decline-inverse-offers call for the given offer IDs.
// Callers are expected to fill in the FrameworkID and Filters.
func DeclineInverseOffers(offerIDs ...mesos.OfferID) *scheduler.Call {
	return &scheduler.Call{
		Type: scheduler.Call_DECLINE_INVERSE_OFFERS,
		DeclineInverseOffers: &scheduler.Call_DeclineInverseOffers{
			InverseOfferIDs: offerIDs,
		},
	}
}

// OpLaunch returns a launch operation builder for the given tasks
func OpLaunch(ti ...mesos.TaskInfo) mesos.Offer_Operation {
	return mesos.Offer_Operation{
		Type: mesos.Offer_Operation_LAUNCH,
		Launch: &mesos.Offer_Operation_Launch{
			TaskInfos: ti,
		},
	}
}

func OpLaunchGroup(ei mesos.ExecutorInfo, ti ...mesos.TaskInfo) mesos.Offer_Operation {
	return mesos.Offer_Operation{
		Type: mesos.Offer_Operation_LAUNCH_GROUP,
		LaunchGroup: &mesos.Offer_Operation_LaunchGroup{
			Executor: ei,
			TaskGroup: mesos.TaskGroupInfo{
				Tasks: ti,
			},
		},
	}
}

func OpReserve(rs ...mesos.Resource) mesos.Offer_Operation {
	return mesos.Offer_Operation{
		Type: mesos.Offer_Operation_RESERVE,
		Reserve: &mesos.Offer_Operation_Reserve{
			Resources: rs,
		},
	}
}

func OpUnreserve(rs ...mesos.Resource) mesos.Offer_Operation {
	return mesos.Offer_Operation{
		Type: mesos.Offer_Operation_UNRESERVE,
		Unreserve: &mesos.Offer_Operation_Unreserve{
			Resources: rs,
		},
	}
}

func OpCreate(rs ...mesos.Resource) mesos.Offer_Operation {
	return mesos.Offer_Operation{
		Type: mesos.Offer_Operation_CREATE,
		Create: &mesos.Offer_Operation_Create{
			Volumes: rs,
		},
	}
}

func OpDestroy(rs ...mesos.Resource) mesos.Offer_Operation {
	return mesos.Offer_Operation{
		Type: mesos.Offer_Operation_DESTROY,
		Destroy: &mesos.Offer_Operation_Destroy{
			Volumes: rs,
		},
	}
}

func OpGrowVolume(v mesos.Resource, a mesos.Resource) mesos.Offer_Operation {
	return mesos.Offer_Operation{
		Type: mesos.Offer_Operation_GROW_VOLUME,
		GrowVolume: &mesos.Offer_Operation_GrowVolume{
			Volume:   v,
			Addition: a,
		},
	}
}

func OpShrinkVolume(v mesos.Resource, s mesos.Value_Scalar) mesos.Offer_Operation {
	return mesos.Offer_Operation{
		Type: mesos.Offer_Operation_SHRINK_VOLUME,
		ShrinkVolume: &mesos.Offer_Operation_ShrinkVolume{
			Volume:   v,
			Subtract: s,
		},
	}
}

func OpCreateDisk(src mesos.Resource, t mesos.Resource_DiskInfo_Source_Type) mesos.Offer_Operation {
	return mesos.Offer_Operation{
		Type: mesos.Offer_Operation_CREATE_DISK,
		CreateDisk: &mesos.Offer_Operation_CreateDisk{
			Source:     src,
			TargetType: t,
		},
	}
}

func OpDestroyDisk(src mesos.Resource) mesos.Offer_Operation {
	return mesos.Offer_Operation{
		Type: mesos.Offer_Operation_DESTROY_DISK,
		DestroyDisk: &mesos.Offer_Operation_DestroyDisk{
			Source: src,
		},
	}
}

// Revive returns a revive call.
// Callers are expected to fill in the FrameworkID.
func Revive() *scheduler.Call {
	return &scheduler.Call{Type: scheduler.Call_REVIVE}
}

// Revive returns a revive call with the given filters.
// Callers are expected to fill in the FrameworkID.
func ReviveWith(roles []string) *scheduler.Call {
	return &scheduler.Call{
		Type:   scheduler.Call_REVIVE,
		Revive: &scheduler.Call_Revive{Roles: roles},
	}
}

// Suppress returns a suppress call.
// Callers are expected to fill in the FrameworkID.
func Suppress() *scheduler.Call {
	return &scheduler.Call{Type: scheduler.Call_SUPPRESS}
}

// Suppress returns a suppress call with the given filters.
// Callers are expected to fill in the FrameworkID.
func SuppressWith(roles []string) *scheduler.Call {
	return &scheduler.Call{
		Type:     scheduler.Call_SUPPRESS,
		Suppress: &scheduler.Call_Suppress{Roles: roles},
	}
}

// Decline returns a decline call with the given parameters.
// Callers are expected to fill in the FrameworkID and Filters.
func Decline(offerIDs ...mesos.OfferID) *scheduler.Call {
	return &scheduler.Call{
		Type: scheduler.Call_DECLINE,
		Decline: &scheduler.Call_Decline{
			OfferIDs: offerIDs,
		},
	}
}

// Kill returns a kill call with the given parameters.
// Callers are expected to fill in the FrameworkID.
func Kill(taskID, agentID string) *scheduler.Call {
	return &scheduler.Call{
		Type: scheduler.Call_KILL,
		Kill: &scheduler.Call_Kill{
			TaskID:  mesos.TaskID{Value: taskID},
			AgentID: optionalAgentID(agentID),
		},
	}
}

// Shutdown returns a shutdown call with the given parameters.
// Callers are expected to fill in the FrameworkID.
func Shutdown(executorID, agentID string) *scheduler.Call {
	return &scheduler.Call{
		Type: scheduler.Call_SHUTDOWN,
		Shutdown: &scheduler.Call_Shutdown{
			ExecutorID: mesos.ExecutorID{Value: executorID},
			AgentID:    mesos.AgentID{Value: agentID},
		},
	}
}

// Acknowledge returns an acknowledge call with the given parameters.
// Callers are expected to fill in the FrameworkID.
func Acknowledge(agentID, taskID string, uuid []byte) *scheduler.Call {
	return &scheduler.Call{
		Type: scheduler.Call_ACKNOWLEDGE,
		Acknowledge: &scheduler.Call_Acknowledge{
			AgentID: mesos.AgentID{Value: agentID},
			TaskID:  mesos.TaskID{Value: taskID},
			UUID:    uuid,
		},
	}
}

// ReconcileTasks constructs a []Call_Reconcile_Task from the given mappings:
//     map[string]string{taskID:agentID}
// Map keys (taskID's) are required to be non-empty, but values (agentID's) *may* be empty.
func ReconcileTasks(tasks map[string]string) scheduler.ReconcileOpt {
	return func(cr *scheduler.Call_Reconcile) {
		if len(tasks) == 0 {
			cr.Tasks = nil
			return
		}
		result := make([]scheduler.Call_Reconcile_Task, len(tasks))
		i := 0
		for k, v := range tasks {
			result[i].TaskID = mesos.TaskID{Value: k}
			result[i].AgentID = optionalAgentID(v)
			i++
		}
		cr.Tasks = result
	}
}

// Reconcile returns a reconcile call with the given parameters.
// See ReconcileTask.
// Callers are expected to fill in the FrameworkID.
func Reconcile(opts ...scheduler.ReconcileOpt) *scheduler.Call {
	return &scheduler.Call{
		Type:      scheduler.Call_RECONCILE,
		Reconcile: (&scheduler.Call_Reconcile{}).With(opts...),
	}
}

// Message returns a message call with the given parameters.
// Callers are expected to fill in the FrameworkID.
func Message(agentID, executorID string, data []byte) *scheduler.Call {
	return &scheduler.Call{
		Type: scheduler.Call_MESSAGE,
		Message: &scheduler.Call_Message{
			AgentID:    mesos.AgentID{Value: agentID},
			ExecutorID: mesos.ExecutorID{Value: executorID},
			Data:       data,
		},
	}
}

// Request returns a resource request call with the given parameters.
// Callers are expected to fill in the FrameworkID.
func Request(requests ...mesos.Request) *scheduler.Call {
	return &scheduler.Call{
		Type: scheduler.Call_REQUEST,
		Request: &scheduler.Call_Request{
			Requests: requests,
		},
	}
}

func optionalAgentID(agentID string) *mesos.AgentID {
	if agentID == "" {
		return nil
	}
	return &mesos.AgentID{Value: agentID}
}

func optionalResourceProviderID(id string) *mesos.ResourceProviderID {
	if id == "" {
		return nil
	}
	return &mesos.ResourceProviderID{Value: id}
}

func errInvalidCall(reason string) error {
	return errors.New("invalid call: " + reason)
}

// AcknowledgeOperationStatus acks the receipt of an operation status update. Schedulers are responsible for
// explicitly acknowledging the receipt of updates which have the 'OperationStatusUpdate.status().uuid()'
// field set. Such status updates are retried by the agent or resource provider until they are acknowledged by the scheduler.
// agentID and resourceProviderID are optional, the remaining fields are required.
func AcknowledgeOperationStatus(agentID, resourceProviderID string, uuid []byte, operationID string) *scheduler.Call {
	return &scheduler.Call{
		Type: scheduler.Call_ACKNOWLEDGE_OPERATION_STATUS,
		AcknowledgeOperationStatus: &scheduler.Call_AcknowledgeOperationStatus{
			AgentID:            optionalAgentID(agentID),
			ResourceProviderID: optionalResourceProviderID(resourceProviderID),
			UUID:               uuid,
			OperationID:        mesos.OperationID{Value: operationID},
		},
	}
}

// ReconcileOperationRequest is a convenience type for which each instance maps to an instance of
// scheduler.Call_ReconcileOfferOperations_Operation.
type ReconcileOperationRequest struct {
	OperationID        string // OperationID is required
	AgentID            string // AgentID is optional
	ResourceProviderID string // ResourceProviderID is optional
}

// ReconcileOperations allows the scheduler to query the status of operations. This causes the master to send
// back the latest status for each operation in 'req', if possible. If 'req' is empty, then the master will send
// the latest status for each operation currently known.
func ReconcileOperations(req []ReconcileOperationRequest) *scheduler.Call {
	var operations []scheduler.Call_ReconcileOperations_Operation
	for i := range req {
		operations = append(operations, scheduler.Call_ReconcileOperations_Operation{
			OperationID:        mesos.OperationID{Value: req[i].OperationID},
			AgentID:            optionalAgentID(req[i].AgentID),
			ResourceProviderID: optionalResourceProviderID(req[i].ResourceProviderID),
		})
	}
	return &scheduler.Call{
		Type: scheduler.Call_RECONCILE_OPERATIONS,
		ReconcileOperations: &scheduler.Call_ReconcileOperations{
			Operations: operations,
		},
	}
}
