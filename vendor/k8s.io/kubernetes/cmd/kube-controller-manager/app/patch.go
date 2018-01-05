package app

import (
	"k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	"k8s.io/kubernetes/pkg/controller"
)

// This allows overriding from inside the same process.  It's not pretty, but its fairly easy to maintain because conflicts are small.
var CreateControllerContext func(s *options.CMServer, rootClientBuilder, clientBuilder controller.ControllerClientBuilder, stop <-chan struct{}) (ControllerContext, error) = createControllerContext

// StartInformers allows overriding inside of the same process.
var StartInformers func(stop <-chan struct{}) = nil
