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
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/http2"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	servercerts "k8s.io/apiserver/pkg/server/certs"
	"k8s.io/klog"
)

const (
	defaultKeepAlivePeriod = 3 * time.Minute
)

// Serve runs the secure http server. It fails only if certificates cannot be loaded or the initial listen call fails.
// The actual server loop (stoppable by closing stopCh) runs in a go routine, i.e. Serve does not block.
// It returns a stoppedCh that is closed when all non-hijacked active requests have been processed.
func (s *SecureServingInfo) Serve(handler http.Handler, shutdownTimeout time.Duration, stopCh <-chan struct{}) (<-chan struct{}, error) {
	if s.Listener == nil {
		return nil, fmt.Errorf("listener must not be nil")
	}

	secureServer := &http.Server{
		Addr:           s.Listener.Addr().String(),
		Handler:        handler,
		MaxHeaderBytes: 1 << 20,
	}

	baseTLSConfig := tls.Config{
		// Can't use SSLv3 because of POODLE and BEAST
		// Can't use TLSv1.0 because of POODLE and BEAST using CBC cipher
		// Can't use TLSv1.1 because of RC4 cipher usage
		MinVersion: tls.VersionTLS12,
		// enable HTTP2 for go's 1.7 HTTP Server
		NextProtos: []string{"h2", "http/1.1"},
	}

	if s.DisableHTTP2 {
		klog.Info("Forcing use of http/1.1 only")
		secureServer.TLSConfig.NextProtos = []string{"http/1.1"}
	}

	if s.MinTLSVersion > 0 {
		baseTLSConfig.MinVersion = s.MinTLSVersion
	}
	if len(s.CipherSuites) > 0 {
		baseTLSConfig.CipherSuites = s.CipherSuites
	}
	if len(s.ClientCA.CABundles) > 0 {
		// Populate PeerCertificates in requests, but don't reject connections without certificates
		// This allows certificates to be validated by authenticators, while still allowing other auth types
		baseTLSConfig.ClientAuth = tls.RequestClientCert
	}

	// this option overrides the provided certs
	// TODO this should be mutually exclusive, but I'm not sure what that will do today
	if len(s.ClientCA.CABundles) > 0 || s.LoopbackCert != nil || s.NameToCertificate != nil || len(s.DefaultCertificate.Key) != 0 || len(s.DefaultCertificate.Cert) != 0 {
		loader := servercerts.DynamicServingLoader{
			ClientCA:           s.ClientCA,
			DefaultCertificate: s.DefaultCertificate,
			NameToCertificate:  s.NameToCertificate,
			LoopbackCert:       s.LoopbackCert,
		}
		loader.BaseTLSConfig = baseTLSConfig // set a copy so that further changes don't get reflected

		// need to load the certs at least once
		if err := loader.CheckCerts(); err != nil {
			return nil, err
		}
		go loader.Run(stopCh)

		// now wire the server for certificates
		secureServer.TLSConfig = &tls.Config{
			GetConfigForClient: loader.GetConfigForClient,
		}
	}

	// At least 99% of serialized resources in surveyed clusters were smaller than 256kb.
	// This should be big enough to accommodate most API POST requests in a single frame,
	// and small enough to allow a per connection buffer of this size multiplied by `MaxConcurrentStreams`.
	const resourceBody99Percentile = 256 * 1024

	http2Options := &http2.Server{}

	// shrink the per-stream buffer and max framesize from the 1MB default while still accommodating most API POST requests in a single frame
	http2Options.MaxUploadBufferPerStream = resourceBody99Percentile
	http2Options.MaxReadFrameSize = resourceBody99Percentile

	// use the overridden concurrent streams setting or make the default of 250 explicit so we can size MaxUploadBufferPerConnection appropriately
	if s.HTTP2MaxStreamsPerConnection > 0 {
		http2Options.MaxConcurrentStreams = uint32(s.HTTP2MaxStreamsPerConnection)
	} else {
		http2Options.MaxConcurrentStreams = 250
	}

	// increase the connection buffer size from the 1MB default to handle the specified number of concurrent streams
	http2Options.MaxUploadBufferPerConnection = http2Options.MaxUploadBufferPerStream * int32(http2Options.MaxConcurrentStreams)

	if !s.DisableHTTP2 {
		// apply settings to the server
		if err := http2.ConfigureServer(secureServer, http2Options); err != nil {
			return nil, fmt.Errorf("error configuring http2: %v", err)
		}
	}

	klog.Infof("Serving securely on %s", secureServer.Addr)
	return RunServer(secureServer, s.Listener, shutdownTimeout, stopCh)
}

// RunServer spawns a go-routine continuously serving until the stopCh is
// closed.
// It returns a stoppedCh that is closed when all non-hijacked active requests
// have been processed.
// This function does not block
// TODO: make private when insecure serving is gone from the kube-apiserver
func RunServer(
	server *http.Server,
	ln net.Listener,
	shutDownTimeout time.Duration,
	stopCh <-chan struct{},
) (<-chan struct{}, error) {
	if ln == nil {
		return nil, fmt.Errorf("listener must not be nil")
	}

	// Shutdown server gracefully.
	stoppedCh := make(chan struct{})
	go func() {
		defer close(stoppedCh)
		<-stopCh
		ctx, cancel := context.WithTimeout(context.Background(), shutDownTimeout)
		server.Shutdown(ctx)
		cancel()
	}()

	go func() {
		defer utilruntime.HandleCrash()

		var listener net.Listener
		listener = tcpKeepAliveListener{ln.(*net.TCPListener)}
		if server.TLSConfig != nil {
			listener = tls.NewListener(listener, server.TLSConfig)
		}

		err := server.Serve(listener)

		msg := fmt.Sprintf("Stopped listening on %s", ln.Addr().String())
		select {
		case <-stopCh:
			klog.Info(msg)
		default:
			panic(fmt.Sprintf("%s due to error: %v", msg, err))
		}
	}()

	return stoppedCh, nil
}

type NamedTLSCert struct {
	// OriginalFileName is an optional string that can be used to provide the original backing files in the GetNamedCertificateMap
	// return value
	OriginalFileName *servercerts.CertKeyFileReference

	TLSCert tls.Certificate

	// Names is a list of domain patterns: fully qualified domain names, possibly prefixed with
	// wildcard segments.
	Names []string
}

// GetNamedCertificateMap returns a map of *tls.Certificate by name. It's
// suitable for use in tls.Config#NamedCertificates. Returns an error if any of the certs
// cannot be loaded. Returns nil if len(certs) == 0
func GetNamedCertificateMap(certs []NamedTLSCert) (map[string]*tls.Certificate, map[string]*servercerts.CertKeyFileReference, error) {
	// register certs with implicit names first, reverse order such that earlier trump over the later
	byName := map[string]*tls.Certificate{}
	fileByName := map[string]*servercerts.CertKeyFileReference{}
	for i := len(certs) - 1; i >= 0; i-- {
		if len(certs[i].Names) > 0 {
			continue
		}
		cert := &certs[i].TLSCert

		// read names from certificate common names and DNS names
		if len(cert.Certificate) == 0 {
			return nil, nil, fmt.Errorf("empty SNI certificate, skipping")
		}
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return nil, nil, fmt.Errorf("parse error for SNI certificate: %v", err)
		}
		cn := x509Cert.Subject.CommonName
		if cn == "*" || len(validation.IsDNS1123Subdomain(strings.TrimPrefix(cn, "*."))) == 0 {
			byName[cn] = cert
			fileByName[cn] = certs[i].OriginalFileName
		}
		for _, san := range x509Cert.DNSNames {
			byName[san] = cert
			fileByName[san] = certs[i].OriginalFileName
		}
		// intentionally all IPs in the cert are ignored as SNI forbids passing IPs
		// to select a cert. Before go 1.6 the tls happily passed IPs as SNI values.
	}

	// register certs with explicit names last, overwriting every of the implicit ones,
	// again in reverse order.
	for i := len(certs) - 1; i >= 0; i-- {
		namedCert := &certs[i]
		for _, name := range namedCert.Names {
			byName[name] = &certs[i].TLSCert
			fileByName[name] = certs[i].OriginalFileName
		}
	}

	return byName, fileByName, nil
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.
//
// Copied from Go 1.7.2 net/http/server.go
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (net.Conn, error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return nil, err
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(defaultKeepAlivePeriod)
	return tc, nil
}
