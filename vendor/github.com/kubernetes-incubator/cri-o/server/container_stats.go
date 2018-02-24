package server

import (
	"fmt"
	"time"

	"github.com/kubernetes-incubator/cri-o/lib"
	"github.com/kubernetes-incubator/cri-o/oci"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func buildContainerStats(stats *lib.ContainerStats, container *oci.Container) *pb.ContainerStats {
	return &pb.ContainerStats{
		Attributes: &pb.ContainerAttributes{
			Id:          container.ID(),
			Metadata:    container.Metadata(),
			Labels:      container.Labels(),
			Annotations: container.Annotations(),
		},
		Cpu: &pb.CpuUsage{
			Timestamp:            stats.SystemNano,
			UsageCoreNanoSeconds: &pb.UInt64Value{Value: stats.CPUNano},
		},
		Memory: &pb.MemoryUsage{
			Timestamp:       stats.SystemNano,
			WorkingSetBytes: &pb.UInt64Value{Value: stats.MemUsage},
		},
		WritableLayer: nil,
	}
}

// ContainerStats returns stats of the container. If the container does not
// exist, the call returns an error.
func (s *Server) ContainerStats(ctx context.Context, req *pb.ContainerStatsRequest) (resp *pb.ContainerStatsResponse, err error) {
	const operation = "container_stats"
	defer func() {
		recordOperation(operation, time.Now())
		recordError(operation, err)
	}()

	container := s.GetContainer(req.ContainerId)
	if container == nil {
		return nil, fmt.Errorf("invalid container")
	}

	stats, err := s.GetContainerStats(container, &lib.ContainerStats{})
	if err != nil {
		return nil, err
	}

	return &pb.ContainerStatsResponse{Stats: buildContainerStats(stats, container)}, nil
}
