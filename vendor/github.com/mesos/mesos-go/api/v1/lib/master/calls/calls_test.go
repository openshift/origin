package calls_test

import (
	"context"
	"time"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/maintenance"
	"github.com/mesos/mesos-go/api/v1/lib/master"
	. "github.com/mesos/mesos-go/api/v1/lib/master/calls"
	"github.com/mesos/mesos-go/api/v1/lib/quota"
)

func Example() {
	var (
		check = func(err error) {
			if err != nil {
				panic(err)
			}
		}
		swallow = func(_ mesos.Response, err error) { check(err) }

		ctx       = context.Background()
		sender    = SenderFunc(func(_ context.Context, _ Request) (_ mesos.Response, _ error) { return })
		blackhole = func(c *master.Call) { swallow(sender.Send(ctx, NonStreaming(c))) }

		d = time.Duration(0)
	)
	blackhole(GetHealth())
	blackhole(GetFlags())
	blackhole(GetVersion())
	blackhole(GetMetrics(nil))
	blackhole(GetMetrics(&d))
	blackhole(GetLoggingLevel())
	blackhole(ListFiles(""))
	blackhole(ReadFile("", 0))
	blackhole(ReadFileWithLength("", 0, 0))
	blackhole(GetState())
	blackhole(GetAgents())
	blackhole(GetFrameworks())
	blackhole(GetExecutors())
	blackhole(GetOperations())
	blackhole(GetTasks())
	blackhole(GetRoles())
	blackhole(GetWeights())
	blackhole(GetMaster())
	blackhole(GetMaintenanceStatus())
	blackhole(GetMaintenanceSchedule())
	blackhole(GetQuota())
	blackhole(Subscribe())

	blackhole = func(c *master.Call) {
		check(SendNoData(ctx, sender, NonStreaming(c)))
	}
	blackhole(SetLoggingLevel(0, 0))
	blackhole(UpdateWeights())
	blackhole(ReserveResources(mesos.AgentID{}))
	blackhole(UnreserveResources(mesos.AgentID{}))
	blackhole(CreateVolumes(mesos.AgentID{}))
	blackhole(DestroyVolumes(mesos.AgentID{}))
	blackhole(GrowVolume(&mesos.AgentID{}, mesos.Resource{}, mesos.Resource{}))
	blackhole(ShrinkVolume(&mesos.AgentID{}, mesos.Resource{}, mesos.Value_Scalar{}))
	blackhole(UpdateMaintenanceSchedule(maintenance.Schedule{}))
	blackhole(StartMaintenance())
	blackhole(StopMaintenance())
	blackhole(SetQuota(quota.QuotaRequest{}))
	blackhole(RemoveQuota(""))
	blackhole(MarkAgentGone(mesos.AgentID{}))
	blackhole(Teardown(mesos.FrameworkID{}))

	// Output:
}
