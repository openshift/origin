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
	"bufio"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"k8s.io/klog"

	"sigs.k8s.io/apiserver-network-proxy/konnectivity-client/pkg/client"
	"sigs.k8s.io/apiserver-network-proxy/pkg/util"
)

func main() {
	client := &Client{}
	o := newGrpcProxyClientOptions()
	command := newGrpcProxyClientCommand(client, o)
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
	klog.Flush()
}

type GrpcProxyClientOptions struct {
	clientCert   string
	clientKey    string
	caCert       string
	requestProto string
	requestPath  string
	requestHost  string
	requestPort  int
	proxyHost    string
	proxyPort    int
	proxyUdsName string
	mode         string
}

func (o *GrpcProxyClientOptions) Flags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("proxy", pflag.ContinueOnError)
	flags.StringVar(&o.clientCert, "client-cert", o.clientCert, "If non-empty secure communication with this cert.")
	flags.StringVar(&o.clientKey, "client-key", o.clientKey, "If non-empty secure communication with this key.")
	flags.StringVar(&o.caCert, "ca-cert", o.caCert, "If non-empty the CAs we use to validate clients.")
	flags.StringVar(&o.requestProto, "request-proto", o.requestProto, "The protocol for the request to send through the proxy.")
	flags.StringVar(&o.requestPath, "request-path", o.requestPath, "The url request to send through the proxy.")
	flags.StringVar(&o.requestHost, "request-host", o.requestHost, "The host of the request server.")
	flags.IntVar(&o.requestPort, "request-port", o.requestPort, "The port the request server is listening on.")
	flags.StringVar(&o.proxyHost, "proxy-host", o.proxyHost, "The host of the proxy server.")
	flags.IntVar(&o.proxyPort, "proxy-port", o.proxyPort, "The port the proxy server is listening on.")
	flags.StringVar(&o.proxyUdsName, "proxy-uds", o.proxyUdsName, "The UDS name to connect to.")
	flags.StringVar(&o.mode, "mode", o.mode, "Mode can be either 'grpc' or 'http-connect'.")

	return flags
}

func (o *GrpcProxyClientOptions) Print() {
	klog.Warningf("ClientCert set to %q.\n", o.clientCert)
	klog.Warningf("ClientKey set to %q.\n", o.clientKey)
	klog.Warningf("CACert set to %q.\n", o.caCert)
	klog.Warningf("RequestProto set to %q.\n", o.requestProto)
	klog.Warningf("RequestPath set to %q.\n", o.requestPath)
	klog.Warningf("RequestHost set to %q.\n", o.requestHost)
	klog.Warningf("RequestPort set to %d.\n", o.requestPort)
	klog.Warningf("ProxyHost set to %q.\n", o.proxyHost)
	klog.Warningf("ProxyPort set to %d.\n", o.proxyPort)
	klog.Warningf("ProxyUdsName set to %q.\n", o.proxyUdsName)
	klog.Warningf("Mode set to %q.\n", o.mode)
}

func (o *GrpcProxyClientOptions) Validate() error {
	if o.clientKey != "" {
		if _, err := os.Stat(o.clientKey); os.IsNotExist(err) {
			return err
		}
		if o.clientCert == "" {
			return fmt.Errorf("cannot have client cert empty when client key is set to %q", o.clientKey)
		}
	}
	if o.clientCert != "" {
		if _, err := os.Stat(o.clientCert); os.IsNotExist(err) {
			return err
		}
		if o.clientKey == "" {
			return fmt.Errorf("cannot have client key empty when client cert is set to %q", o.clientCert)
		}
	}
	if o.caCert != "" {
		if _, err := os.Stat(o.caCert); os.IsNotExist(err) {
			return err
		}
	}
	if o.requestProto != "http" && o.requestProto != "https" {
		return fmt.Errorf("request protocol must be set to either 'http' or 'https' not %q", o.requestProto)
	}
	if o.mode != "grpc" && o.mode != "http-connect" {
		return fmt.Errorf("mode must be set to either 'grpc' or 'http-connect' not %q", o.mode)
	}
	if o.requestPort > 49151 {
		return fmt.Errorf("please do not try to use ephemeral port %d for the request server port", o.requestPort)
	}
	if o.proxyPort > 49151 {
		return fmt.Errorf("please do not try to use ephemeral port %d for the proxy server port", o.proxyPort)
	}
	if o.proxyPort < 1024 && o.proxyUdsName == "" {
		return fmt.Errorf("please do not try to use reserved port %d for the proxy server port", o.proxyPort)
	}
	if o.proxyUdsName != "" {
		if o.proxyHost != "" {
			return fmt.Errorf("please do set proxy host when using UDS")
		}
		if o.proxyPort != 0 {
			return fmt.Errorf("please do set proxy server port to 0 not %d when using UDS", o.proxyPort)
		}
		if o.clientKey != "" || o.clientCert != "" || o.caCert != "" {
			return fmt.Errorf("please do set cert materials when using UDS, key = %s, cert = %s, CA = %s",
				o.clientKey, o.clientCert, o.caCert)
		}
	}
	return nil
}

func newGrpcProxyClientOptions() *GrpcProxyClientOptions {
	o := GrpcProxyClientOptions{
		clientCert:   "",
		clientKey:    "",
		caCert:       "",
		requestProto: "http",
		requestPath:  "/",
		requestHost:  "localhost",
		requestPort:  8000,
		proxyHost:    "localhost",
		proxyPort:    8090,
		proxyUdsName: "",
		mode:         "grpc",
	}
	return &o
}

func newGrpcProxyClientCommand(c *Client, o *GrpcProxyClientOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:  "proxy-client",
		Long: `A gRPC proxy Client, primarily used to test the Kubernetes gRPC Proxy Server.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.run(o)
		},
	}

	return cmd
}

type Client struct {
}

func (c *Client) run(o *GrpcProxyClientOptions) error {
	o.Print()
	if err := o.Validate(); err != nil {
		return fmt.Errorf("failed to validate proxy client options, got %v", err)
	}

	// Run remote simple http service on server side as
	// "python -m SimpleHTTPServer"

	dialer, err := c.getDialer(o)
	if err != nil {
		return fmt.Errorf("failed to get dialer for client, got %v", err)
	}
	transport := &http.Transport{
		DialContext: dialer,
	}
	client := &http.Client{
		Transport: transport,
	}
	requestURL := fmt.Sprintf("%s://%s:%d%s", o.requestProto, o.requestHost, o.requestPort, o.requestPath)
	request, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request %s to send, got %v", requestURL, err)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("failed to send request to client, got %v", err)
	}
	defer response.Body.Close() // TODO: proxy server should handle the case where Body isn't closed.

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response from client, got %v", err)
	}
	klog.Info(string(data))

	return nil
}

func (c *Client) getDialer(o *GrpcProxyClientOptions) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	if o.proxyUdsName != "" {
		return c.getUDSDialer(o)
	}
	return c.getMTLSDialer(o)
}

func (c *Client) getUDSDialer(o *GrpcProxyClientOptions) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	var proxyConn net.Conn
	var err error

	// Setup signal handler
	ch := make(chan os.Signal, 1)
	signal.Notify(ch)

	go func() {
		<-ch
		if proxyConn != nil {
			err := proxyConn.Close()
			klog.Infof("connection closed: %v", err)
		}
	}()

	switch o.mode {
	case "grpc":
		dialOption := grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			// Ignoring addr and timeout arguments:
			// addr - comes from the closure
			// timeout - is turned off as this is test code and eases debugging.
			c, err := net.DialTimeout("unix", o.proxyUdsName, 0)
			if err != nil {
				klog.Errorf("failed to create connection to uds name %s, error: %v", o.proxyUdsName, err)
			}
			return c, err
		})
		tunnel, err := client.CreateGrpcTunnel(o.proxyUdsName, dialOption, grpc.WithInsecure())
		if err != nil {
			return nil, fmt.Errorf("failed to create tunnel %s, got %v", o.proxyUdsName, err)
		}

		requestAddress := fmt.Sprintf("%s:%d", o.requestHost, o.requestPort)
		proxyConn, err = tunnel.Dial("tcp", requestAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to dial request %s, got %v", requestAddress, err)
		}
	case "http-connect":
		requestAddress := fmt.Sprintf("%s:%d", o.requestHost, o.requestPort)

		proxyConn, err = net.Dial("unix", o.proxyUdsName)
		if err != nil {
			return nil, fmt.Errorf("dialing proxy %q failed: %v", o.proxyUdsName, err)
		}
		fmt.Fprintf(proxyConn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", requestAddress, "127.0.0.1")
		br := bufio.NewReader(proxyConn)
		res, err := http.ReadResponse(br, nil)
		if err != nil {
			return nil, fmt.Errorf("reading HTTP response from CONNECT to %s via uds proxy %s failed: %v",
				requestAddress, o.proxyUdsName, err)
		}
		if res.StatusCode != 200 {
			return nil, fmt.Errorf("proxy error from %s while dialing %s: %v", o.proxyUdsName, requestAddress, res.Status)
		}

		// It's safe to discard the bufio.Reader here and return the
		// original TCP conn directly because we only use this for
		// TLS, and in TLS the client speaks first, so we know there's
		// no unbuffered data. But we can double-check.
		if br.Buffered() > 0 {
			return nil, fmt.Errorf("unexpected %d bytes of buffered data from CONNECT uds proxy %q",
				br.Buffered(), o.proxyUdsName)
		}
	default:
		return nil, fmt.Errorf("failed to process mode %s", o.mode)
	}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return proxyConn, nil
	}, nil
}

func (c *Client) getMTLSDialer(o *GrpcProxyClientOptions) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	var tlsConfig *tls.Config
	var err error
	tlsConfig, err = util.GetClientTLSConfig(o.caCert, o.clientCert, o.clientKey, o.proxyHost)
	if err != nil {
		return nil, err
	}

	var proxyConn net.Conn

	// Setup signal handler
	ch := make(chan os.Signal, 1)
	signal.Notify(ch)

	go func() {
		<-ch
		err := proxyConn.Close()
		klog.Infof("connection closed: %v", err)
	}()

	switch o.mode {
	case "grpc":
		transportCreds := credentials.NewTLS(tlsConfig)
		dialOption := grpc.WithTransportCredentials(transportCreds)
		serverAddress := fmt.Sprintf("%s:%d", o.proxyHost, o.proxyPort)
		tunnel, err := client.CreateGrpcTunnel(serverAddress, dialOption)
		if err != nil {
			return nil, fmt.Errorf("failed to create tunnel %s, got %v", serverAddress, err)
		}

		requestAddress := fmt.Sprintf("%s:%d", o.requestHost, o.requestPort)
		proxyConn, err = tunnel.Dial("tcp", requestAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to dial request %s, got %v", requestAddress, err)
		}
	case "http-connect":
		proxyAddress := fmt.Sprintf("%s:%d", o.proxyHost, o.proxyPort)
		requestAddress := fmt.Sprintf("%s:%d", o.requestHost, o.requestPort)

		proxyConn, err = tls.Dial("tcp", proxyAddress, tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("dialing proxy %q failed: %v", proxyAddress, err)
		}
		fmt.Fprintf(proxyConn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", requestAddress, "127.0.0.1")
		br := bufio.NewReader(proxyConn)
		res, err := http.ReadResponse(br, nil)
		if err != nil {
			return nil, fmt.Errorf("reading HTTP response from CONNECT to %s via proxy %s failed: %v",
				requestAddress, proxyAddress, err)
		}
		if res.StatusCode != 200 {
			return nil, fmt.Errorf("proxy error from %s while dialing %s: %v", proxyAddress, requestAddress, res.Status)
		}

		// It's safe to discard the bufio.Reader here and return the
		// original TCP conn directly because we only use this for
		// TLS, and in TLS the client speaks first, so we know there's
		// no unbuffered data. But we can double-check.
		if br.Buffered() > 0 {
			return nil, fmt.Errorf("unexpected %d bytes of buffered data from CONNECT proxy %q",
				br.Buffered(), proxyAddress)
		}
	default:
		return nil, fmt.Errorf("failed to process mode %s", o.mode)
	}

	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return proxyConn, nil
	}, nil
}
