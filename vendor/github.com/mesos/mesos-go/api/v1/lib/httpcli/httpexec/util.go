package httpexec

import (
	"github.com/mesos/mesos-go/api/v1/lib/client"
	"github.com/mesos/mesos-go/api/v1/lib/executor"
	"github.com/mesos/mesos-go/api/v1/lib/httpcli"
)

func classifyResponse(c *executor.Call) (rc client.ResponseClass, err error) {
	switch name := executor.Call_Type_name[int32(c.GetType())]; name {
	case "", "UNKNOWN":
		err = httpcli.ProtocolError("unsupported call type")
	default:
		rc = client.ResponseClassAuto // TODO(jdef) fix this, ResponseClassAuto is deprecated
	}
	return
}
