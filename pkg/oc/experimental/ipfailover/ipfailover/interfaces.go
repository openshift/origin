package ipfailover

import (
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

type IPFailoverConfiguratorPlugin interface {
	Generate() (*kapi.List, error)
}
