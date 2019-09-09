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
	"testing"

	restclient "k8s.io/client-go/rest"

	"github.com/heketi/tests"
)

type dummyLogger struct{}

func (*dummyLogger) LogError(s string, v ...interface{}) error {
	e := fmt.Errorf(s, v...)
	fmt.Printf("Error: %v\n", e)
	return e
}

func (*dummyLogger) Err(e error) error {
	fmt.Printf("Err: %v\n", e)
	return e
}

func (*dummyLogger) Critical(s string, v ...interface{}) {
	fmt.Printf(s+"\n", v...)
}

func (*dummyLogger) Debug(s string, v ...interface{}) {
	fmt.Printf(s+"\n", v...)
}

func TestDummyConfig(t *testing.T) {
	icc := InClusterConfig
	defer func() {
		InClusterConfig = icc
	}()
	InClusterConfig = func() (*restclient.Config, error) {
		return nil, nil
	}
	cc, err := InClusterConfig()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, cc == nil, "expected cc == nil, got:", cc)
	InClusterConfig = func() (*restclient.Config, error) {
		return &restclient.Config{}, nil
	}
	cc, err = InClusterConfig()
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
	tests.Assert(t, cc.Host == "", "expected cc.Host == \"\", got:", cc.Host)
}

func TestConnEmptyConfig(t *testing.T) {
	l := &dummyLogger{}
	_, err := NewKubeConnWithConfig(l, &restclient.Config{})
	tests.Assert(t, err == nil, "expected err == nil, got:", err)
}
