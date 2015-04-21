package ipfailover

import (
	"io"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

type IPFailoverConfiguratorPlugin interface {
	GetWatchPort() int
	GetSelector() map[string]string
	GetNamespace() string
	GetService() *kapi.Service
	Generate() *kapi.List
	Create(out io.Writer)
}
