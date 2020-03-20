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
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"k8s.io/klog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/apiserver-network-proxy/pkg/agent/agentserver"
	"sigs.k8s.io/apiserver-network-proxy/pkg/util"
	"sigs.k8s.io/apiserver-network-proxy/konnectivity-client/proto/client"
	"sigs.k8s.io/apiserver-network-proxy/proto/agent"
)

func main() {
	// flag.CommandLine.Parse(os.Args[1:])
	proxy := &Proxy{}
	o := newProxyRunOptions()
	command := newProxyCommand(proxy, o)
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

type ProxyRunOptions struct {
	// Certificate setup for securing communication to the "client" i.e. the Kube API Server.
	serverCert   string
	serverKey    string
	serverCaCert string
	// Certificate setup for securing communication to the "agent" i.e. the managed cluster.
	clusterCert   string
	clusterKey    string
	clusterCaCert string
	// Flag to switch between gRPC and HTTP Connect
	mode string
	// Location for use by the "unix" network. Setting enables UDS for server connections.
	udsName string
	// Port we listen for server connections on.
	serverPort uint
	// Port we listen for agent connections on.
	agentPort uint
	// Port we listen for admin connections on.
	adminPort uint

	// ID of this proxy server.
	serverID string
	// Number of proxy server instances, should be 1 unless it is a HA proxy server.
	serverCount uint
	// Agent pod's namespace for token-based agent authentication
	agentNamespace string
	// Agent pod's service account for token-based agent authentication
	agentServiceAccount string
	// Token's audience for token-based agent authentication
	authenticationAudience string
	// Path to kubeconfig (used by kubernetes client)
	kubeconfigPath string
}

func (o *ProxyRunOptions) Flags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("proxy-server", pflag.ContinueOnError)
	flags.StringVar(&o.serverCert, "server-cert", o.serverCert, "If non-empty secure communication with this cert.")
	flags.StringVar(&o.serverKey, "server-key", o.serverKey, "If non-empty secure communication with this key.")
	flags.StringVar(&o.serverCaCert, "server-ca-cert", o.serverCaCert, "If non-empty the CA we use to validate KAS clients.")
	flags.StringVar(&o.clusterCert, "cluster-cert", o.clusterCert, "If non-empty secure communication with this cert.")
	flags.StringVar(&o.clusterKey, "cluster-key", o.clusterKey, "If non-empty secure communication with this key.")
	flags.StringVar(&o.clusterCaCert, "cluster-ca-cert", o.clusterCaCert, "If non-empty the CA we use to validate Agent clients.")
	flags.StringVar(&o.mode, "mode", o.mode, "Mode can be either 'grpc' or 'http-connect'.")
	flags.StringVar(&o.udsName, "uds-name", o.udsName, "uds-name should be empty for TCP traffic. For UDS set to its name.")
	flags.UintVar(&o.serverPort, "server-port", o.serverPort, "Port we listen for server connections on. Set to 0 for UDS.")
	flags.UintVar(&o.agentPort, "agent-port", o.agentPort, "Port we listen for agent connections on.")
	flags.UintVar(&o.adminPort, "admin-port", o.adminPort, "Port we listen for admin connections on.")
	flags.StringVar(&o.serverID, "server-id", o.serverID, "The unique ID of this server.")
	flags.UintVar(&o.serverCount, "server-count", o.serverCount, "The number of proxy server instances, should be 1 unless it is an HA server.")
	flags.StringVar(&o.agentNamespace, "agent-namespace", o.agentNamespace, "Expected agent's namespace during agent authentication (used with agent-service-account, authentication-audience, kubeconfig).")
	flags.StringVar(&o.agentServiceAccount, "agent-service-account", o.agentServiceAccount, "Expected agent's service account during agent authentication (used with agent-namespace, authentication-audience, kubeconfig).")
	flags.StringVar(&o.kubeconfigPath, "kubeconfig", o.kubeconfigPath, "absolute path to the kubeconfig file (used with agent-namespace, agent-service-account, authentication-audience).")
	flags.StringVar(&o.authenticationAudience, "authentication-audience", o.authenticationAudience, "Expected agent's token authentication audience (used with agent-namespace, agent-service-account, kubeconfig).")
	return flags
}

func (o *ProxyRunOptions) Print() {
	klog.Warningf("ServerCert set to %q.\n", o.serverCert)
	klog.Warningf("ServerKey set to %q.\n", o.serverKey)
	klog.Warningf("ServerCACert set to %q.\n", o.serverCaCert)
	klog.Warningf("ClusterCert set to %q.\n", o.clusterCert)
	klog.Warningf("ClusterKey set to %q.\n", o.clusterKey)
	klog.Warningf("ClusterCACert set to %q.\n", o.clusterCaCert)
	klog.Warningf("Mode set to %q.\n", o.mode)
	klog.Warningf("UDSName set to %q.\n", o.udsName)
	klog.Warningf("Server port set to %d.\n", o.serverPort)
	klog.Warningf("Agent port set to %d.\n", o.agentPort)
	klog.Warningf("Admin port set to %d.\n", o.adminPort)
	klog.Warningf("ServerID set to %s.\n", o.serverID)
	klog.Warningf("ServerCount set to %d.\n", o.serverCount)
	klog.Warningf("AgentNamespace set to %q.\n", o.agentNamespace)
	klog.Warningf("AgentServiceAccount set to %q.\n", o.agentServiceAccount)
	klog.Warningf("AuthenticationAudience set to %q.\n", o.authenticationAudience)
	klog.Warningf("KubeconfigPath set to %q.\n", o.kubeconfigPath)
}

func (o *ProxyRunOptions) Validate() error {
	if o.serverKey != "" {
		if _, err := os.Stat(o.serverKey); os.IsNotExist(err) {
			return fmt.Errorf("error checking server key %s, got %v", o.serverKey, err)
		}
		if o.serverCert == "" {
			return fmt.Errorf("cannot have server cert empty when server key is set to %q", o.serverKey)
		}
	}
	if o.serverCert != "" {
		if _, err := os.Stat(o.serverCert); os.IsNotExist(err) {
			return fmt.Errorf("error checking server cert %s, got %v", o.serverCert, err)
		}
		if o.serverKey == "" {
			return fmt.Errorf("cannot have server key empty when server cert is set to %q", o.serverCert)
		}
	}
	if o.serverCaCert != "" {
		if _, err := os.Stat(o.serverCaCert); os.IsNotExist(err) {
			return fmt.Errorf("error checking server CA cert %s, got %v", o.serverCaCert, err)
		}
	}
	if o.clusterKey != "" {
		if _, err := os.Stat(o.clusterKey); os.IsNotExist(err) {
			return fmt.Errorf("error checking cluster key %s, got %v", o.clusterKey, err)
		}
		if o.clusterCert == "" {
			return fmt.Errorf("cannot have cluster cert empty when cluster key is set to %q", o.clusterKey)
		}
	}
	if o.clusterCert != "" {
		if _, err := os.Stat(o.clusterCert); os.IsNotExist(err) {
			return fmt.Errorf("error checking cluster cert %s, got %v", o.clusterCert, err)
		}
		if o.clusterKey == "" {
			return fmt.Errorf("cannot have cluster key empty when cluster cert is set to %q", o.clusterCert)
		}
	}
	if o.clusterCaCert != "" {
		if _, err := os.Stat(o.clusterCaCert); os.IsNotExist(err) {
			return fmt.Errorf("error checking cluster CA cert %s, got %v", o.clusterCaCert, err)
		}
	}
	if o.mode != "grpc" && o.mode != "http-connect" {
		return fmt.Errorf("mode must be set to either 'grpc' or 'http-connect' not %q", o.mode)
	}
	if o.udsName != "" {
		if o.serverPort != 0 {
			return fmt.Errorf("server port should be set to 0 not %d for UDS", o.serverPort)
		}
		if o.serverKey != "" {
			return fmt.Errorf("server key should not be set for UDS")
		}
		if o.serverCert != "" {
			return fmt.Errorf("server cert should not be set for UDS")
		}
		if o.serverCaCert != "" {
			return fmt.Errorf("server ca cert should not be set for UDS")
		}
	}
	if o.serverPort > 49151 {
		return fmt.Errorf("please do not try to use ephemeral port %d for the server port", o.serverPort)
	}
	if o.agentPort > 49151 {
		return fmt.Errorf("please do not try to use ephemeral port %d for the agent port", o.agentPort)
	}
	if o.adminPort > 49151 {
		return fmt.Errorf("please do not try to use ephemeral port %d for the admin port", o.adminPort)
	}
	if o.serverPort < 1024 {
		if o.udsName == "" {
			return fmt.Errorf("please do not try to use reserved port %d for the server port", o.serverPort)
		}
	}
	if o.agentPort < 1024 {
		return fmt.Errorf("please do not try to use reserved port %d for the agent port", o.agentPort)
	}
	if o.adminPort < 1024 {
		return fmt.Errorf("please do not try to use reserved port %d for the admin port", o.adminPort)
	}

	// validate agent authentication params
	// all 4 parametes must be empty or must have value (except kubeconfigPath that might be empty)
	if o.agentNamespace != "" || o.agentServiceAccount != "" || o.authenticationAudience != "" || o.kubeconfigPath != "" {
		if o.agentNamespace == "" {
			return fmt.Errorf("agentNamespace cannot be empty when agent authentication is enabled")
		}
		if o.agentServiceAccount == "" {
			return fmt.Errorf("agentServiceAccount cannot be empty when agent authentication is enabled")
		}
		if o.authenticationAudience == "" {
			return fmt.Errorf("authenticationAudience cannot be empty when agent authentication is enabled")
		}
		if o.kubeconfigPath != "" {
			if _, err := os.Stat(o.kubeconfigPath); os.IsNotExist(err) {
				return fmt.Errorf("error checking kubeconfigPath %q, got %v", o.kubeconfigPath, err)
			}
		}
	}

	return nil
}

func newProxyRunOptions() *ProxyRunOptions {
	o := ProxyRunOptions{
		serverCert:             "",
		serverKey:              "",
		serverCaCert:           "",
		clusterCert:            "",
		clusterKey:             "",
		clusterCaCert:          "",
		mode:                   "grpc",
		udsName:                "",
		serverPort:             8090,
		agentPort:              8091,
		adminPort:              8092,
		serverID:               uuid.New().String(),
		serverCount:            1,
		agentNamespace:         "",
		agentServiceAccount:    "",
		kubeconfigPath:         "",
		authenticationAudience: "",
	}
	return &o
}

func newProxyCommand(p *Proxy, o *ProxyRunOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:  "proxy",
		Long: `A gRPC proxy server, receives requests from the API server and forwards to the agent.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return p.run(o)
		},
	}

	return cmd
}

type Proxy struct {
}

type StopFunc func()

func (p *Proxy) run(o *ProxyRunOptions) error {
	o.Print()
	if err := o.Validate(); err != nil {
		return fmt.Errorf("failed to validate server options with %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var k8sClient *kubernetes.Clientset
	if o.agentNamespace != "" {
		config, err := clientcmd.BuildConfigFromFlags("", o.kubeconfigPath)
		if err != nil {
			return fmt.Errorf("failed to load kubernetes client config: %v", err)
		}

		k8sClient, err = kubernetes.NewForConfig(config)
		if err != nil {
			return fmt.Errorf("failed to create kubernetes clientset: %v", err)
		}
	}

	authOpt := &agentserver.AgentTokenAuthenticationOptions{
		Enabled:                o.agentNamespace != "",
		AgentNamespace:         o.agentNamespace,
		AgentServiceAccount:    o.agentServiceAccount,
		KubernetesClient:       k8sClient,
		AuthenticationAudience: o.authenticationAudience,
	}
	server := agentserver.NewProxyServer(o.serverID, int(o.serverCount), authOpt)

	klog.Info("Starting master server for client connections.")
	masterStop, err := p.runMasterServer(ctx, o, server)
	if err != nil {
		return fmt.Errorf("failed to run the master server: %v", err)
	}

	klog.Info("Starting agent server for tunnel connections.")
	err = p.runAgentServer(o, server)
	if err != nil {
		return fmt.Errorf("failed to run the agent server: %v", err)
	}

	klog.Info("Starting admin server for debug connections.")
	err = p.runAdminServer(o, server)
	if err != nil {
		return fmt.Errorf("failed to run the admin server: %v", err)
	}

	stopCh := SetupSignalHandler()
	<-stopCh
	klog.Info("Shutting down server.")

	if masterStop != nil {
		masterStop()
	}

	return nil
}

var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

func SetupSignalHandler() (stopCh <-chan struct{}) {
	stop := make(chan struct{})
	c := make(chan os.Signal, 2)
	signal.Notify(c, shutdownSignals...)
	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return stop
}

func (p *Proxy) runMasterServer(ctx context.Context, o *ProxyRunOptions, server *agentserver.ProxyServer) (StopFunc, error) {
	if o.udsName != "" {
		return p.runUDSMasterServer(ctx, o, server)
	}
	return p.runMTLSMasterServer(ctx, o, server)
}

func (p *Proxy) runUDSMasterServer(ctx context.Context, o *ProxyRunOptions, server *agentserver.ProxyServer) (StopFunc, error) {
	var stop StopFunc
	if o.mode == "grpc" {
		grpcServer := grpc.NewServer()
		client.RegisterProxyServiceServer(grpcServer, server)
		var lc net.ListenConfig
		lis, err := lc.Listen(ctx, "unix", o.udsName)
		if err != nil {
			return nil, fmt.Errorf("failed to listen(unix) name %s: %v", o.udsName, err)
		}
		go grpcServer.Serve(lis)
		stop = grpcServer.GracefulStop
	} else {
		// http-connect
		server := &http.Server{
			Handler: &agentserver.Tunnel{
				Server: server,
			},
		}
		stop = func() { server.Shutdown(ctx) }
		go func() {
			var lc net.ListenConfig
			udsListener, err := lc.Listen(ctx, "unix", o.udsName)
			if err != nil {
				klog.Errorf("failed to listen on uds name %s: %v ", o.udsName, err)
			}
			defer func() {
				udsListener.Close()
			}()
			err = server.Serve(udsListener)
			if err != nil {
				klog.Errorf("failed to serve uds requests: %v", err)
			}
		}()
	}

	return stop, nil
}

func (p *Proxy) getTLSConfig(caFile, certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load X509 key pair %s and %s: %v", certFile, keyFile, err)
	}

	if caFile == "" {
		return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
	}

	certPool := x509.NewCertPool()
	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cluster CA cert %s: %v", caFile, err)
	}
	ok := certPool.AppendCertsFromPEM(caCert)
	if !ok {
		return nil, fmt.Errorf("failed to append cluster CA cert to the cert pool")
	}
	tlsConfig := &tls.Config{
		ClientAuth:   tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{cert},
		ClientCAs:    certPool,
	}

	return tlsConfig, nil
}

func (p *Proxy) runMTLSMasterServer(ctx context.Context, o *ProxyRunOptions, server *agentserver.ProxyServer) (StopFunc, error) {
	var stop StopFunc

	var tlsConfig *tls.Config
	var err error
	if tlsConfig, err = p.getTLSConfig(o.serverCaCert, o.serverCert, o.serverKey); err != nil {
		return nil, err
	}

	addr := fmt.Sprintf(":%d", o.serverPort)

	if o.mode == "grpc" {
		serverOption := grpc.Creds(credentials.NewTLS(tlsConfig))
		grpcServer := grpc.NewServer(serverOption)
		client.RegisterProxyServiceServer(grpcServer, server)
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("failed to listen on %s: %v", addr, err)
		}
		go grpcServer.Serve(lis)
		stop = grpcServer.GracefulStop
	} else {
		// http-connect
		server := &http.Server{
			Addr:      addr,
			TLSConfig: tlsConfig,
			Handler: &agentserver.Tunnel{
				Server: server,
			},
			TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
		}
		stop = func() { server.Shutdown(ctx) }
		go func() {
			err := server.ListenAndServeTLS("", "") // empty files defaults to tlsConfig
			if err != nil {
				klog.Errorf("failed to listen on master port %v", err)
			}
		}()
	}

	return stop, nil
}

func (p *Proxy) runAgentServer(o *ProxyRunOptions, server *agentserver.ProxyServer) error {
	var tlsConfig *tls.Config
	var err error
	if tlsConfig, err = p.getTLSConfig(o.clusterCaCert, o.clusterCert, o.clusterKey); err != nil {
		return err
	}

	addr := fmt.Sprintf(":%d", o.agentPort)
	serverOption := grpc.Creds(credentials.NewTLS(tlsConfig))
	grpcServer := grpc.NewServer(serverOption)
	agent.RegisterAgentServiceServer(grpcServer, server)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %v", addr, err)
	}
	go grpcServer.Serve(lis)

	return nil
}

func (p *Proxy) runAdminServer(o *ProxyRunOptions, server *agentserver.ProxyServer) error {
	livenessHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok")
	})
	readinessHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok")
	})
	metricsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prometheus.Handler().ServeHTTP(w, r)
	})

	muxHandler := http.NewServeMux()
	muxHandler.HandleFunc("/healthz", livenessHandler)
	muxHandler.HandleFunc("/ready", readinessHandler)
	muxHandler.HandleFunc("/metrics", metricsHandler)
	adminServer := &http.Server{
		Addr:           fmt.Sprintf("127.0.0.1:%d", o.adminPort),
		Handler:        muxHandler,
		MaxHeaderBytes: 1 << 20,
	}

	go func() {
		err := adminServer.ListenAndServe()
		if err != nil {
			klog.Warningf("health server received %v.\n", err)
		}
		klog.Warningf("Health server stopped listening\n")
	}()

	return nil
}
