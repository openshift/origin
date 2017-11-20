package server

import (
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

// Version returns the runtime name, runtime version and runtime API version
func (s *Server) Version(ctx context.Context, req *pb.VersionRequest) (*pb.VersionResponse, error) {

	runtimeVersion, err := s.Runtime().Version()
	if err != nil {
		return nil, err
	}

	// TODO: Track upstream code. For now it expects 0.1.0
	version := "0.1.0"

	// taking const address
	rav := runtimeAPIVersion
	runtimeName := s.Runtime().Name()

	return &pb.VersionResponse{
		Version:           version,
		RuntimeName:       runtimeName,
		RuntimeVersion:    runtimeVersion,
		RuntimeApiVersion: rav,
	}, nil
}
