// +build tools

// Package test contains cross-functional test suites for OpenShift 3.
package test

import (
	_ "github.com/go-bindata/go-bindata/go-bindata"
	_ "github.com/openshift/build-machinery-go"
	_ "github.com/stretchr/testify/require"
	_ "k8s.io/kubernetes/pkg/registry/registrytest"
)
