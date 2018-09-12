package ipfailover

import (
	"k8s.io/apimachinery/pkg/runtime"
)

type IPFailoverConfiguratorPlugin interface {
	Generate() ([]runtime.Object, error)
}
