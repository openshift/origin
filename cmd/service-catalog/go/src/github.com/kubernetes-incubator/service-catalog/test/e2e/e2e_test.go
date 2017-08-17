/*
Copyright 2015 The Kubernetes Authors.

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

package e2e

import (
	"flag"
	"testing"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/service-catalog/test/e2e/framework"
)

var brokerImageFlag string

func init() {
	flag.StringVar(&brokerImageFlag, "broker-image", "quay.io/kubernetes-service-catalog/user-broker:latest",
		"The container image for the broker to test against")
	framework.RegisterParseFlags()

	if "" == framework.TestContext.KubeConfig {
		glog.Fatalf("environment variable %v must be set", clientcmd.RecommendedConfigPathEnvVar)
	}
	if "" == framework.TestContext.ServiceCatalogConfig {
		glog.Fatalf("environment variable %v must be set", framework.RecommendedConfigPathEnvVar)
	}
}

func TestE2E(t *testing.T) {
	RunE2ETests(t)
}
