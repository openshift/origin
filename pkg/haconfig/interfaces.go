package haconfig

import (
	"io"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

type HAConfiguratorPlugin interface {
	GetWatchPort() int
	GetSelector() map[string]string
	GetNamespace() string
	GetService() *kapi.Service
	Generate() *kapi.List
	Create(out io.Writer)
	Delete(out io.Writer)
}
