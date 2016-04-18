package ipfailover

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

type IPFailoverConfiguratorPlugin interface {
	Generate() (*kapi.List, error)
}
