package httpagent

import (
	"fmt"

	"github.com/mesos/mesos-go/api/v1/lib/agent"
	"github.com/mesos/mesos-go/api/v1/lib/client"
	"github.com/mesos/mesos-go/api/v1/lib/httpcli"
)

func classifyResponse(c *agent.Call) (rc client.ResponseClass, err error) {
	if c == nil {
		err = httpcli.ProtocolError("nil agent.Call not allowed")
		return
	}

	switch t := c.GetType(); t {
	// singleton
	case agent.Call_GET_HEALTH,
		agent.Call_GET_FLAGS,
		agent.Call_GET_VERSION,
		agent.Call_GET_METRICS,
		agent.Call_GET_LOGGING_LEVEL,
		agent.Call_LIST_FILES,
		agent.Call_READ_FILE,
		agent.Call_GET_STATE,
		agent.Call_GET_CONTAINERS,
		agent.Call_GET_FRAMEWORKS,
		agent.Call_GET_EXECUTORS,
		agent.Call_GET_OPERATIONS,
		agent.Call_GET_TASKS,
		agent.Call_GET_AGENT,
		agent.Call_GET_RESOURCE_PROVIDERS,
		agent.Call_WAIT_CONTAINER,
		agent.Call_WAIT_NESTED_CONTAINER:
		rc = client.ResponseClassSingleton

	// streaming
	case agent.Call_LAUNCH_NESTED_CONTAINER_SESSION,
		agent.Call_ATTACH_CONTAINER_OUTPUT:
		rc = client.ResponseClassStreaming

	// no-data
	case agent.Call_SET_LOGGING_LEVEL,
		agent.Call_LAUNCH_CONTAINER,
		agent.Call_LAUNCH_NESTED_CONTAINER,
		agent.Call_KILL_CONTAINER,
		agent.Call_KILL_NESTED_CONTAINER,
		agent.Call_REMOVE_CONTAINER,
		agent.Call_REMOVE_NESTED_CONTAINER,
		agent.Call_ATTACH_CONTAINER_INPUT,
		agent.Call_ADD_RESOURCE_PROVIDER_CONFIG,
		agent.Call_UPDATE_RESOURCE_PROVIDER_CONFIG,
		agent.Call_REMOVE_RESOURCE_PROVIDER_CONFIG,
		agent.Call_PRUNE_IMAGES:
		rc = client.ResponseClassNoData

	default:
		err = httpcli.ProtocolError(fmt.Sprintf("unsupported agent.Call type: %v", t))
	}
	return
}
