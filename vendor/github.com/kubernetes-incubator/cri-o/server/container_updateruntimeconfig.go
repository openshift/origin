package server

import (
	"time"

	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// UpdateRuntimeConfig updates the configuration of a running container.
func (s *Server) UpdateRuntimeConfig(ctx context.Context, req *pb.UpdateRuntimeConfigRequest) (resp *pb.UpdateRuntimeConfigResponse, err error) {
	const operation = "update_runtime_config"
	defer func() {
		recordOperation(operation, time.Now())
		recordError(operation, err)
	}()

	return &pb.UpdateRuntimeConfigResponse{}, nil
}
