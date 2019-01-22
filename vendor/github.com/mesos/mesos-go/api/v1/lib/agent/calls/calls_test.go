package calls_test

import (
	"context"
	"time"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/agent"
	. "github.com/mesos/mesos-go/api/v1/lib/agent/calls"
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
		blackhole = func(calls ...*agent.Call) {
			for i := range calls {
				swallow(sender.Send(ctx, NonStreaming(calls[i])))
			}
		}

		d = time.Duration(0)
	)
	blackhole(
		GetHealth(),
		GetFlags(),
		GetVersion(),
		GetMetrics(nil),
		GetMetrics(&d),
		GetLoggingLevel(),
		ListFiles(""),
		ReadFile("", 0),
		ReadFileWithLength("", 0, 0),
		GetState(),
		GetContainers(),
		GetFrameworks(),
		GetExecutors(),
		GetOperations(),
		GetTasks(),
		GetAgent(),
		GetResourceProviders(),
		WaitContainer(mesos.ContainerID{}),
		WaitNestedContainer(mesos.ContainerID{}),
		LaunchNestedContainerSession(mesos.ContainerID{}, nil, nil),
	)

	blackhole = func(calls ...*agent.Call) {
		for i := range calls {
			check(SendNoData(ctx, sender, NonStreaming(calls[i])))
		}
	}
	blackhole(
		SetLoggingLevel(0, d),
		LaunchContainer(mesos.ContainerID{}, nil, nil, nil),
		LaunchNestedContainer(mesos.ContainerID{}, nil, nil),
		KillContainer(mesos.ContainerID{}),
		KillNestedContainer(mesos.ContainerID{}),
		RemoveContainer(mesos.ContainerID{}),
		RemoveNestedContainer(mesos.ContainerID{}),
		AttachContainerOutput(mesos.ContainerID{}),
		AddResourceProviderConfig(mesos.ResourceProviderInfo{}),
		UpdateResourceProviderConfig(mesos.ResourceProviderInfo{}),
		RemoveResourceProviderConfig("", ""),
		PruneImages(nil),
	)

	swallow(sender.Send(ctx, Empty().Push(
		AttachContainerInput(mesos.ContainerID{}),
		AttachContainerInputTTY(nil),
		AttachContainerInputData(nil),
	)))

	// Output:
}
