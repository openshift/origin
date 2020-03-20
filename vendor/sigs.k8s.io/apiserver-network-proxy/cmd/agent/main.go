/*
Copyright 2019 The Kubernetes Authors.

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

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"k8s.io/klog"
	"sigs.k8s.io/apiserver-network-proxy/pkg/agent/agentclient"
	"sigs.k8s.io/apiserver-network-proxy/pkg/util"
)

func main() {
	agent := &Agent{}
	o := newGrpcProxyAgentOptions()
	command := newAgentCommand(agent, o)
	flags := command.Flags()
	flags.AddFlagSet(o.Flags())
	local := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	klog.InitFlags(local)
	local.VisitAll(func(fl *flag.Flag) {
		fl.Name = util.Normalize(fl.Name)
		flags.AddGoFlag(fl)
	})
	if err := command.Execute(); err != nil {
		klog.Errorf("error: %v\n", err)
		klog.Flush()
		os.Exit(1)
	}
}

type GrpcProxyAgentOptions struct {
	// Configuration for authenticating with the proxy-server
	agentCert string
	agentKey  string
	caCert    string

	// Configuration for connecting to the proxy-server
	proxyServerHost string
	proxyServerPort int

	// Ports for the health and admin server
	healthServerPort int
	adminServerPort  int

	agentID           string
	syncInterval      time.Duration
	probeInterval     time.Duration
	reconnectInterval time.Duration

	// file contains service account authorization token for enabling proxy-server token based authorization
	serviceAccountTokenPath string
}

func (o *GrpcProxyAgentOptions) ClientSetConfig(dialOption grpc.DialOption) *agentclient.ClientSetConfig {
	return &agentclient.ClientSetConfig{
		Address:                 fmt.Sprintf("%s:%d", o.proxyServerHost, o.proxyServerPort),
		AgentID:                 o.agentID,
		SyncInterval:            o.syncInterval,
		ProbeInterval:           o.probeInterval,
		ReconnectInterval:       o.reconnectInterval,
		DialOption:              dialOption,
		ServiceAccountTokenPath: o.serviceAccountTokenPath,
	}
}

func (o *GrpcProxyAgentOptions) Flags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("proxy-agent", pflag.ContinueOnError)
	flags.StringVar(&o.agentCert, "agent-cert", o.agentCert, "If non-empty secure communication with this cert.")
	flags.StringVar(&o.agentKey, "agent-key", o.agentKey, "If non-empty secure communication with this key.")
	flags.StringVar(&o.caCert, "ca-cert", o.caCert, "If non-empty the CAs we use to validate clients.")
	flags.StringVar(&o.proxyServerHost, "proxy-server-host", o.proxyServerHost, "The hostname to use to connect to the proxy-server.")
	flags.IntVar(&o.proxyServerPort, "proxy-server-port", o.proxyServerPort, "The port the proxy server is listening on.")
	flags.IntVar(&o.healthServerPort, "health-server-port", o.healthServerPort, "The port the health server is listening on.")
	flags.IntVar(&o.adminServerPort, "admin-server-port", o.adminServerPort, "The port the admin server is listening on.")
	flags.StringVar(&o.agentID, "agent-id", o.agentID, "The unique ID of this agent. Default to a generated uuid if not set.")
	flags.DurationVar(&o.syncInterval, "sync-interval", o.syncInterval, "The initial interval by which the agent periodically checks if it has connections to all instances of the proxy server.")
	flags.DurationVar(&o.probeInterval, "probe-interval", o.probeInterval, "The interval by which the agent periodically checks if its connections to the proxy server are ready.")
	flags.DurationVar(&o.reconnectInterval, "reconnect-interval", o.reconnectInterval, "The interval by which the agent tries to reconnect.")
	flags.StringVar(&o.serviceAccountTokenPath, "service-account-token-path", o.serviceAccountTokenPath, "If non-empty proxy agent uses this token to prove its identity to the proxy server.")
	return flags
}

func (o *GrpcProxyAgentOptions) Print() {
	klog.Warningf("AgentCert set to \"%s\".\n", o.agentCert)
	klog.Warningf("AgentKey set to \"%s\".\n", o.agentKey)
	klog.Warningf("CACert set to \"%s\".\n", o.caCert)
	klog.Warningf("ProxyServerHost set to \"%s\".\n", o.proxyServerHost)
	klog.Warningf("ProxyServerPort set to %d.\n", o.proxyServerPort)
	klog.Warningf("HealthServerPort set to %d.\n", o.healthServerPort)
	klog.Warningf("AdminServerPort set to %d.\n", o.adminServerPort)
	klog.Warningf("AgentID set to %s.\n", o.agentID)
	klog.Warningf("SyncInterval set to %v.\n", o.syncInterval)
	klog.Warningf("ProbeInterval set to %v.\n", o.probeInterval)
	klog.Warningf("ReconnectInterval set to %v.\n", o.reconnectInterval)
	klog.Warningf("ServiceAccountTokenPath set to \"%s\".\n", o.serviceAccountTokenPath)
}

func (o *GrpcProxyAgentOptions) Validate() error {
	if o.agentKey != "" {
		if _, err := os.Stat(o.agentKey); os.IsNotExist(err) {
			return fmt.Errorf("error checking agent key %s, got %v", o.agentKey, err)
		}
		if o.agentCert == "" {
			return fmt.Errorf("cannot have agent cert empty when agent key is set to \"%s\"", o.agentKey)
		}
	}
	if o.agentCert != "" {
		if _, err := os.Stat(o.agentCert); os.IsNotExist(err) {
			return fmt.Errorf("error checking agent cert %s, got %v", o.agentCert, err)
		}
		if o.agentKey == "" {
			return fmt.Errorf("cannot have agent key empty when agent cert is set to \"%s\"", o.agentCert)
		}
	}
	if o.caCert != "" {
		if _, err := os.Stat(o.caCert); os.IsNotExist(err) {
			return fmt.Errorf("error checking agent CA cert %s, got %v", o.caCert, err)
		}
	}
	if o.proxyServerPort <= 0 {
		return fmt.Errorf("proxy server port %d must be greater than 0", o.proxyServerPort)
	}
	if o.healthServerPort <= 0 {
		return fmt.Errorf("health server port %d must be greater than 0", o.healthServerPort)
	}
	if o.adminServerPort <= 0 {
		return fmt.Errorf("admin server port %d must be greater than 0", o.adminServerPort)
	}

	if o.serviceAccountTokenPath != "" {
		if _, err := os.Stat(o.serviceAccountTokenPath); os.IsNotExist(err) {
			return fmt.Errorf("error checking service account token path %s, got %v", o.serviceAccountTokenPath, err)
		}
	}
	return nil
}

func newGrpcProxyAgentOptions() *GrpcProxyAgentOptions {
	o := GrpcProxyAgentOptions{
		agentCert:               "",
		agentKey:                "",
		caCert:                  "",
		proxyServerHost:         "127.0.0.1",
		proxyServerPort:         8091,
		healthServerPort:        8093,
		adminServerPort:         8094,
		agentID:                 uuid.New().String(),
		syncInterval:            5 * time.Second,
		probeInterval:           5 * time.Second,
		reconnectInterval:       5 * time.Second,
		serviceAccountTokenPath: "",
	}
	return &o
}

func newAgentCommand(a *Agent, o *GrpcProxyAgentOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:  "agent",
		Long: `A gRPC agent, Connects to the proxy and then allows traffic to be forwarded to it.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.run(o)
		},
	}

	return cmd
}

type Agent struct {
}

func (a *Agent) run(o *GrpcProxyAgentOptions) error {
	o.Print()
	if err := o.Validate(); err != nil {
		return fmt.Errorf("failed to validate agent options with %v", err)
	}

	if err := a.runProxyConnection(o); err != nil {
		return fmt.Errorf("failed to run proxy connection with %v", err)
	}

	if err := a.runHealthServer(o); err != nil {
		return fmt.Errorf("failed to run health server with %v", err)
	}

	if err := a.runAdminServer(o); err != nil {
		return fmt.Errorf("failed to run admin server with %v", err)
	}

	stopCh := make(chan struct{})
	<-stopCh

	return nil
}

func (a *Agent) runProxyConnection(o *GrpcProxyAgentOptions) error {
	var tlsConfig *tls.Config
	var err error
	if tlsConfig, err = util.GetClientTLSConfig(o.caCert, o.agentCert, o.agentKey, o.proxyServerHost); err != nil {
		return err
	}
	dialOption := grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))
	cc := o.ClientSetConfig(dialOption)
	cs := cc.NewAgentClientSet()
	cs.Serve()

	return nil
}

func (a *Agent) runHealthServer(o *GrpcProxyAgentOptions) error {
	livenessHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok")
	})
	readinessHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok")
	})

	muxHandler := http.NewServeMux()
	muxHandler.HandleFunc("/healthz", livenessHandler)
	muxHandler.HandleFunc("/ready", readinessHandler)
	healthServer := &http.Server{
		Addr:           fmt.Sprintf(":%d", o.healthServerPort),
		Handler:        muxHandler,
		MaxHeaderBytes: 1 << 20,
	}

	go func() {
		err := healthServer.ListenAndServe()
		if err != nil {
			klog.Warningf("health server received %v.\n", err)
		}
		klog.Warningf("Health server stopped listening\n")
	}()

	return nil
}

func (a *Agent) runAdminServer(o *GrpcProxyAgentOptions) error {
	metricsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prometheus.Handler().ServeHTTP(w, r)
	})

	muxHandler := http.NewServeMux()
	muxHandler.HandleFunc("/metrics", metricsHandler)
	adminServer := &http.Server{
		Addr:           fmt.Sprintf("127.0.0.1:%d", o.adminServerPort),
		Handler:        muxHandler,
		MaxHeaderBytes: 1 << 20,
	}

	go func() {
		err := adminServer.ListenAndServe()
		if err != nil {
			klog.Warningf("admin server received %v.\n", err)
		}
		klog.Warningf("Admin server stopped listening\n")
	}()

	return nil
}
