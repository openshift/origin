//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package kube

import (
	"fmt"

	client "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

var (
	// allow test code to override default cluster configuration
	InClusterConfig = func() (*restclient.Config, error) {
		return restclient.InClusterConfig()
	}
)

// logger interface exists to allow the library to use a
// logging object provided by the caller.
type logger interface {
	LogError(s string, v ...interface{}) error
	Err(e error) error
	Critical(s string, v ...interface{})
	Debug(s string, v ...interface{})
}

// KubeConn provides a higher level object to manage the connection(s)
// to a k8s cluster.
type KubeConn struct {
	kubeConfig *restclient.Config
	kube       *client.Clientset
	logger     logger
	counter    *connectionCounter
	// tunables for connection threshold handling
	MaxConnThreshold uint64
}

// NewKubeConnWithConfig creates a new KubeConn with the provided
// logger and k8s client configuration. If a connection can not
// be established a non-nil error is returned.
func NewKubeConnWithConfig(l logger, rc *restclient.Config) (*KubeConn, error) {
	var (
		err error
		k   = &KubeConn{
			logger:     l,
			kubeConfig: rc,
			counter:    newConnectionCounter(),
		}
	)

	// Get a Go-client for Kubernetes
	k.kube, err = client.NewForConfig(k.kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("Unable to create a client set: %v", err)
	}
	return k, nil
}

// NewKubeConn creates a new KubeConn with the default
// in-cluster k8s client configuration. If a connection can
// not be established a non-nil error is returned.
func NewKubeConn(l logger) (*KubeConn, error) {
	// Create a Kube client configuration using pkg callback
	rc, err := InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf(
			"Unable to create configuration for Kubernetes: %v", err)
	}
	return NewKubeConnWithConfig(l, rc)
}

type MaxConnectionsErr struct {
	Count uint64
}

func NewMaxConnectionsErr(value uint64) MaxConnectionsErr {
	return MaxConnectionsErr{Count: value}
}

func (e MaxConnectionsErr) Error() string {
	return fmt.Sprintf("Too many kube exec connections active (%v)", e.Count)
}
