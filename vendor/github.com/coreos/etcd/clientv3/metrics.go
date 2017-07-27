package clientv3

// This file exposes constructors for DialOptions that are not accessible to the caller
// because etcd maintains its own vendoring tree. Once the need for the gRPC vendor tree
// is removed, this file can be removed and direct initialization provided.

import (
	prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
)

// PrometheusInterceptors initializes grpc.DialOptions for monitoring the etcdv3 client
// via prometheus. It is exposed here so that callers can access the vendored types.
func PrometheusInterceptors() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithUnaryInterceptor(prometheus.UnaryClientInterceptor),
		grpc.WithStreamInterceptor(prometheus.StreamClientInterceptor),
	}
}
