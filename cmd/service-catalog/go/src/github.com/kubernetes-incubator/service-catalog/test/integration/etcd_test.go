/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/coreos/etcd/embed"
	"github.com/coreos/pkg/capnslog"
	"github.com/golang/glog"
)

type EtcdContext struct {
	etcd     *embed.Etcd
	dir      string
	Endpoint string
}

var etcdContext = EtcdContext{}

func startEtcd() error {
	var err error
	if etcdContext.dir, err = ioutil.TempDir(os.TempDir(), "service_catalog_integration_test"); err != nil {
		return fmt.Errorf("could not create TempDir: %v", err)
	}
	cfg := embed.NewConfig()
	// default of INFO prints useless information
	capnslog.SetGlobalLogLevel(capnslog.WARNING)
	cfg.Dir = etcdContext.dir

	if etcdContext.etcd, err = embed.StartEtcd(cfg); err != nil {
		return fmt.Errorf("Failed starting etcd: %+v", err)
	}

	select {
	case <-etcdContext.etcd.Server.ReadyNotify():
		glog.Info("server is ready!")
	case <-time.After(60 * time.Second):
		etcdContext.etcd.Server.Stop() // trigger a shutdown
		glog.Error("server took too long to start!")
	}
	return nil
}

func stopEtcd() {
	etcdContext.etcd.Server.Stop()
	os.RemoveAll(etcdContext.dir)
}

func TestMain(m *testing.M) {
	// Setup
	if err := startEtcd(); err != nil {
		panic(fmt.Sprintf("Failed to start etcd, %v", err))
	}

	// Tests
	result := m.Run()

	// Teardown
	stopEtcd()
	os.Exit(result)
}
