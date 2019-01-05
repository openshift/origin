package httpmaster

import (
	"fmt"

	"github.com/mesos/mesos-go/api/v1/lib/client"
	"github.com/mesos/mesos-go/api/v1/lib/httpcli"
	"github.com/mesos/mesos-go/api/v1/lib/master"
)

func classifyResponse(c *master.Call) (rc client.ResponseClass, err error) {
	if c == nil {
		err = httpcli.ProtocolError("nil master.Call not allowed")
		return
	}

	switch t := c.GetType(); t {
	// singleton
	case master.Call_GET_HEALTH,
		master.Call_GET_FLAGS,
		master.Call_GET_VERSION,
		master.Call_GET_METRICS,
		master.Call_GET_LOGGING_LEVEL,
		master.Call_LIST_FILES,
		master.Call_READ_FILE,
		master.Call_GET_STATE,
		master.Call_GET_AGENTS,
		master.Call_GET_FRAMEWORKS,
		master.Call_GET_EXECUTORS,
		master.Call_GET_OPERATIONS,
		master.Call_GET_TASKS,
		master.Call_GET_ROLES,
		master.Call_GET_WEIGHTS,
		master.Call_GET_MASTER,
		master.Call_GET_MAINTENANCE_STATUS,
		master.Call_GET_MAINTENANCE_SCHEDULE,
		master.Call_GET_QUOTA:
		rc = client.ResponseClassSingleton

	// streaming
	case master.Call_SUBSCRIBE:
		// for some reason, the docs say that thr format is recordio (streaming) but HTTP negotiation
		// uses application/json or application/x-protobuf; use "auto" class, similar to sched/exec API
		rc = client.ResponseClassAuto

	// no-data
	case master.Call_SET_LOGGING_LEVEL,
		master.Call_UPDATE_WEIGHTS,
		master.Call_RESERVE_RESOURCES,
		master.Call_UNRESERVE_RESOURCES,
		master.Call_CREATE_VOLUMES,
		master.Call_DESTROY_VOLUMES,
		master.Call_GROW_VOLUME,
		master.Call_SHRINK_VOLUME,
		master.Call_UPDATE_MAINTENANCE_SCHEDULE,
		master.Call_START_MAINTENANCE,
		master.Call_STOP_MAINTENANCE,
		master.Call_SET_QUOTA,
		master.Call_REMOVE_QUOTA,
		master.Call_MARK_AGENT_GONE,
		master.Call_TEARDOWN:
		rc = client.ResponseClassNoData

	default:
		err = httpcli.ProtocolError(fmt.Sprintf("unsupported master.Call type: %v", t))
	}
	return
}
