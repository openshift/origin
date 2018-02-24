package server

import (
	"time"

	"github.com/kubernetes-incubator/cri-o/lib"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// ListContainerStats returns stats of all running containers.
func (s *Server) ListContainerStats(ctx context.Context, req *pb.ListContainerStatsRequest) (resp *pb.ListContainerStatsResponse, err error) {
	const operation = "list_container_stats"
	defer func() {
		recordOperation(operation, time.Now())
		recordError(operation, err)
	}()

	ctrList, err := s.ContainerServer.ListContainers()
	if err != nil {
		return nil, err
	}
	filter := req.GetFilter()
	if filter != nil {
		cFilter := &pb.ContainerFilter{
			Id:            req.Filter.Id,
			PodSandboxId:  req.Filter.PodSandboxId,
			LabelSelector: req.Filter.LabelSelector,
		}
		ctrList = s.filterContainerList(cFilter, ctrList)
	}

	var allStats []*pb.ContainerStats

	for _, container := range ctrList {
		stats, err := s.GetContainerStats(container, &lib.ContainerStats{})
		if err != nil {
			logrus.Warn("unable to get stats for container %s", container.ID())
			continue
		}
		response := buildContainerStats(stats, container)
		allStats = append(allStats, response)
	}

	return &pb.ListContainerStatsResponse{
		Stats: allStats,
	}, nil
}
