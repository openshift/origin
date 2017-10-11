/*
Copyright 2016 The Kubernetes Authors.

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

package server

import (
	"flag"
	"io"
	"os"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/service-catalog/pkg"
	"github.com/kubernetes-incubator/service-catalog/pkg/registry/servicecatalog/server"
	"github.com/kubernetes-incubator/service-catalog/plugin/pkg/admission/broker/authsarcheck"
	"github.com/kubernetes-incubator/service-catalog/plugin/pkg/admission/namespace/lifecycle"
	siclifecycle "github.com/kubernetes-incubator/service-catalog/plugin/pkg/admission/servicebindings/lifecycle"
	"github.com/kubernetes-incubator/service-catalog/plugin/pkg/admission/serviceplan/changevalidator"
	"github.com/kubernetes-incubator/service-catalog/plugin/pkg/admission/serviceplan/defaultserviceplan"
	"github.com/spf13/cobra"
	"k8s.io/apiserver/pkg/admission"
	genericserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/kubernetes/pkg/util/interrupt"
)

const (
	// Store generated SSL certificates in a place that won't collide with the
	// k8s core API server.
	certDirectory = "/var/run/kubernetes-service-catalog"

	// I made this up to match some existing paths. I am not sure if there
	// are any restrictions on the format or structure beyond text
	// separated by slashes.
	etcdPathPrefix = "/k8s.io/service-catalog"

	// GroupName I made this up. Maybe we'll need it.
	GroupName = "service-catalog.k8s.io"

	storageTypeFlagName = "storageType"
)

// NewCommandServer creates a new cobra command to run our server.
func NewCommandServer(
	out io.Writer,
) (*cobra.Command, error) {
	// Create the command that runs the API server
	cmd := &cobra.Command{
		Short: "run a service-catalog server",
	}
	// We pass flags object to sub option structs to have them configure
	// themselves. Each options adds its own command line flags
	// in addition to the flags that are defined above.
	flags := cmd.Flags()
	flags.AddGoFlagSet(flag.CommandLine)

	stopCh := make(chan struct{})
	opts := &ServiceCatalogServerOptions{
		GenericServerRunOptions: genericserveroptions.NewServerRunOptions(),
		AdmissionOptions:        genericserveroptions.NewAdmissionOptions(),
		SecureServingOptions:    genericserveroptions.NewSecureServingOptions(),
		AuthenticationOptions:   genericserveroptions.NewDelegatingAuthenticationOptions(),
		AuthorizationOptions:    genericserveroptions.NewDelegatingAuthorizationOptions(),
		AuditOptions:            genericserveroptions.NewAuditOptions(),
		EtcdOptions:             NewEtcdOptions(),
		StopCh:                  stopCh,
		StandaloneMode:          standaloneMode(),
	}
	opts.addFlags(flags)
	// register all admission plugins
	registerAllAdmissionPlugins(opts.AdmissionOptions.Plugins)
	// Set generated SSL cert path correctly
	opts.SecureServingOptions.ServerCert.CertDirectory = certDirectory

	version := pkg.VersionFlag(cmd.Flags())

	flags.Parse(os.Args[1:])

	version.PrintAndExitIfRequested()

	storageType, err := opts.StorageType()
	if err != nil {
		glog.Fatalf("invalid storage type '%s' (%s)", storageType, err)
		return nil, err
	}
	if storageType == server.StorageTypeEtcd {
		glog.Infof("using etcd for storage")
		// Store resources in etcd under our special prefix
		opts.EtcdOptions.StorageConfig.Prefix = etcdPathPrefix
	} else {
		// This should never happen, catch for potential bugs
		panic("Unexpected storage type: " + storageType)
	}

	cmd.Run = func(c *cobra.Command, args []string) {
		h := interrupt.New(nil, func() {
			close(stopCh)
		})
		if err := h.Run(func() error { return RunServer(opts) }); err != nil {
			glog.Fatalf("error running server (%s)", err)
			return
		}
	}

	return cmd, nil
}

// registerAllAdmissionPlugins registers all admission plugins
func registerAllAdmissionPlugins(plugins *admission.Plugins) {
	lifecycle.Register(plugins)
	defaultserviceplan.Register(plugins)
	siclifecycle.Register(plugins)
	changevalidator.Register(plugins)
	authsarcheck.Register(plugins)
}
