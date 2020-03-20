module sigs.k8s.io/apiserver-network-proxy

go 1.12

require (
	github.com/beorn7/perks v1.0.0 // indirect
	github.com/golang/mock v1.4.0
	github.com/golang/protobuf v1.3.2
	github.com/google/uuid v1.1.1
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/prometheus/client_golang v0.9.2
	github.com/prometheus/common v0.4.0 // indirect
	github.com/prometheus/procfs v0.0.0-20190507164030-5867b95ac084 // indirect
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.5
	golang.org/x/net v0.0.0-20191004110552-13f9640d40b9
	google.golang.org/grpc v1.26.0
	k8s.io/api v0.17.1
	k8s.io/apimachinery v0.17.1
	k8s.io/client-go v0.17.1
	k8s.io/klog v1.0.0
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.0.0
)

replace sigs.k8s.io/apiserver-network-proxy/konnectivity-client => ./konnectivity-client
