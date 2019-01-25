package controller

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/extras/scheduler/eventrules"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/calls"
)

func TestAckOperationUpdates_OperationPending(t *testing.T) {
	var (
		sentCalls  []*scheduler.Call
		fakeCaller = calls.CallerFunc(func(_ context.Context, c *scheduler.Call) (_ mesos.Response, _ error) {
			sentCalls = append(sentCalls, c)
			return nil, nil
		})
		rule       = AckOperationUpdates(fakeCaller)
		statusUUID = []byte{1}
		ooID       = mesos.OperationID{Value: "1"}

		evt = &scheduler.Event{
			Type: scheduler.Event_UPDATE_OPERATION_STATUS,
			UpdateOperationStatus: &scheduler.Event_UpdateOperationStatus{
				Status: mesos.OperationStatus{
					OperationID:        &ooID,
					State:              mesos.OPERATION_PENDING,
					ConvertedResources: nil,
					UUID:               &mesos.UUID{Value: statusUUID},
				},
			},
		}
	)

	_, _, err := rule(context.Background(), evt, nil /*error*/, eventrules.ChainIdentity)
	if err != nil {
		t.Errorf("unexpected error: %+v", err)
	}
	if len(sentCalls) != 1 {
		t.Errorf("expected a call to be sent")
	}

	call := sentCalls[0]
	if ty := call.GetType(); ty != scheduler.Call_ACKNOWLEDGE_OPERATION_STATUS {
		t.Errorf("unexpected call type: %v", t)
	}
	ack := call.GetAcknowledgeOperationStatus()
	if v := ack.GetOperationID().Value; v != ooID.Value {
		t.Errorf("expected offer operation ID %q instead of %q", ooID.Value, v)
	}
	if uuid := ack.GetUUID(); !reflect.DeepEqual(uuid, statusUUID) {
		t.Errorf("expected statusUUID of %+v instead of %+v", statusUUID, uuid)
	}
	// no provider, or agent
	if id := ack.GetAgentID().GetValue(); id != "" {
		t.Errorf("unexpected agent ID: %v", id)
	}
	if id := ack.GetResourceProviderID().GetValue(); id != "" {
		t.Errorf("unexpected resource provider ID: %v", id)
	}
}

func TestAckOperationUpdates_IllegalEvent(t *testing.T) {
	var (
		sentCalls  []*scheduler.Call
		fakeCaller = calls.CallerFunc(func(_ context.Context, c *scheduler.Call) (_ mesos.Response, _ error) {
			sentCalls = append(sentCalls, c)
			return nil, nil
		})
		rule       = AckOperationUpdates(fakeCaller)
		statusUUID = []byte{1}

		evt = &scheduler.Event{
			Type: scheduler.Event_UPDATE_OPERATION_STATUS,
			UpdateOperationStatus: &scheduler.Event_UpdateOperationStatus{
				Status: mesos.OperationStatus{
					OperationID:        nil, // this should cause a panic
					State:              mesos.OPERATION_FINISHED,
					ConvertedResources: []mesos.Resource{{ProviderID: &mesos.ResourceProviderID{Value: "a"}}},
					UUID:               &mesos.UUID{Value: statusUUID},
				},
			},
		}
	)
	defer func() {
		if x := recover(); x == nil {
			t.Errorf("expected panic because no operationID was specified with a non-nil statusUUID")
		}
	}()
	_, _, _ = rule(context.Background(), evt, nil /*error*/, eventrules.ChainIdentity)
}

func TestAckOperationUpdates_NoStatusUUID(t *testing.T) {
	var (
		sentCalls  []*scheduler.Call
		fakeCaller = calls.CallerFunc(func(_ context.Context, c *scheduler.Call) (_ mesos.Response, _ error) {
			sentCalls = append(sentCalls, c)
			return nil, nil
		})
		rule = AckOperationUpdates(fakeCaller)

		evt = &scheduler.Event{
			Type: scheduler.Event_UPDATE_OPERATION_STATUS,
			UpdateOperationStatus: &scheduler.Event_UpdateOperationStatus{
				Status: mesos.OperationStatus{
					State:              mesos.OPERATION_FINISHED,
					ConvertedResources: []mesos.Resource{{ProviderID: &mesos.ResourceProviderID{Value: "a"}}},
				},
			},
		}
	)

	_, _, err := rule(context.Background(), evt, nil /*error*/, eventrules.ChainIdentity)
	if err != nil {
		t.Errorf("unexpected error: %+v", err)
	}
	if len(sentCalls) != 0 {
		t.Errorf("expected no call to be sent")
	}
}

func TestAckOperationUpdates_OperationFinished(t *testing.T) {
	var (
		sentCalls  []*scheduler.Call
		fakeCaller = calls.CallerFunc(func(_ context.Context, c *scheduler.Call) (_ mesos.Response, _ error) {
			sentCalls = append(sentCalls, c)
			return nil, nil
		})
		rule       = AckOperationUpdates(fakeCaller)
		statusUUID = []byte{1}
		ooID       = mesos.OperationID{Value: "1"}

		evt = &scheduler.Event{
			Type: scheduler.Event_UPDATE_OPERATION_STATUS,
			UpdateOperationStatus: &scheduler.Event_UpdateOperationStatus{
				Status: mesos.OperationStatus{
					OperationID:        &ooID,
					State:              mesos.OPERATION_FINISHED,
					ConvertedResources: []mesos.Resource{{ProviderID: &mesos.ResourceProviderID{Value: "a"}}},
					UUID:               &mesos.UUID{Value: statusUUID},
				},
			},
		}
	)

	_, _, err := rule(context.Background(), evt, nil /*error*/, eventrules.ChainIdentity)
	if err != nil {
		t.Errorf("unexpected error: %+v", err)
	}
	if len(sentCalls) != 1 {
		t.Errorf("expected a call to be sent")
	}

	call := sentCalls[0]
	if ty := call.GetType(); ty != scheduler.Call_ACKNOWLEDGE_OPERATION_STATUS {
		t.Errorf("unexpected call type: %v", t)
	}
	ack := call.GetAcknowledgeOperationStatus()
	if v := ack.GetOperationID().Value; v != ooID.Value {
		t.Errorf("expected offer operation ID %q instead of %q", ooID.Value, v)
	}
	if uuid := ack.GetUUID(); !reflect.DeepEqual(uuid, statusUUID) {
		t.Errorf("expected statusUUID of %+v instead of %+v", statusUUID, uuid)
	}
	// or agent
	if id := ack.GetAgentID().GetValue(); id != "" {
		t.Errorf("unexpected agent ID: %v", id)
	}
	if id := ack.GetResourceProviderID().GetValue(); id != "a" {
		t.Errorf("expected resource provider ID of 'a' instead of %v", id)
	}
}

func TestAckOperationUpdates_CallerError(t *testing.T) {
	var (
		callerError = errors.New("call failed")
		fakeCaller  = calls.CallerFunc(func(_ context.Context, c *scheduler.Call) (_ mesos.Response, _ error) {
			return nil, callerError
		})
		rule       = AckOperationUpdates(fakeCaller)
		statusUUID = []byte{1}
		ooID       = mesos.OperationID{Value: "1"}

		evt = &scheduler.Event{
			Type: scheduler.Event_UPDATE_OPERATION_STATUS,
			UpdateOperationStatus: &scheduler.Event_UpdateOperationStatus{
				Status: mesos.OperationStatus{
					OperationID:        &ooID,
					State:              mesos.OPERATION_FINISHED,
					ConvertedResources: []mesos.Resource{{ProviderID: &mesos.ResourceProviderID{Value: "a"}}},
					UUID:               &mesos.UUID{Value: statusUUID},
				},
			},
		}
	)

	_, _, err := rule(context.Background(), evt, nil /*error*/, eventrules.ChainIdentity)
	if ackErr, _ := err.(*calls.AckError); ackErr != nil && ackErr.Cause != callerError {
		t.Errorf("unexpected error: %+v", err)
	}
}
