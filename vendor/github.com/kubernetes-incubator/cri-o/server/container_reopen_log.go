package server

import (
	"fmt"

	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// ReopenContainerLog reopens the containers log file
func (s *Server) ReopenContainerLog(ctx context.Context, in *pb.ReopenContainerLogRequest) (*pb.ReopenContainerLogResponse, error) {
	return nil, fmt.Errorf("not yet implemented")
}
