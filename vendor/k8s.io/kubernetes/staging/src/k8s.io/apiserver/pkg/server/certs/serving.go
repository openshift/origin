package certs

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"reflect"
	"sync/atomic"
	"time"

	"k8s.io/klog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/cert"
)

// LoopbackClientServerNameOverride is passed to the apiserver from the loopback client in order to
// select the loopback certificate via SNI if TLS is used.
const LoopbackClientServerNameOverride = "apiserver-loopback-client"

type CertKeyFileReference struct {
	Cert string
	Key  string
}

type CABundleFileReferences struct {
	CABundles []string
}

// DynamicLoader dynamically loads certificates and provides a golang tls compatible dynamic GetCertificate func.
type DynamicServingLoader struct {
	// BaseTLSConfig is the static portion of the tlsConfig for serving to clients.  It is copied and the copy is mutated
	// based on the dynamic cert state.
	BaseTLSConfig tls.Config

	ClientCA           CABundleFileReferences
	DefaultCertificate CertKeyFileReference
	NameToCertificate  map[string]*CertKeyFileReference
	LoopbackCert       *tls.Certificate

	currentContent dynamicCertificateContent
	currentValue   atomic.Value
}

type dynamicCertificateContent struct {
	ClientCA           certKeyFileContent
	DefaultCertificate certKeyFileContent
	NameToCertificate  map[string]*certKeyFileContent
}

type certKeyFileContent struct {
	Cert []byte
	Key  []byte
}

type runtimeDynamicLoader struct {
	tlsConfig tls.Config
}

func (c *DynamicServingLoader) GetConfigForClient(clientHello *tls.ClientHelloInfo) (*tls.Config, error) {
	uncastObj := c.currentValue.Load()
	if uncastObj == nil {
		return nil, errors.New("tls: configuration not ready")
	}
	runtimeConfig, ok := uncastObj.(*runtimeDynamicLoader)
	if !ok {
		return nil, errors.New("tls: unexpected config type")
	}
	return runtimeConfig.GetConfigForClient(clientHello)
}

func (c *DynamicServingLoader) Run(stopCh <-chan struct{}) {
	klog.Infof("Starting DynamicLoader")
	defer klog.Infof("Shutting down DynamicLoader")

	go wait.Until(func() {
		err := c.CheckCerts()
		if err != nil {
			utilruntime.HandleError(err)
		}
	}, 1*time.Minute, stopCh)

	<-stopCh
}

func (c *DynamicServingLoader) CheckCerts() error {
	newContent := dynamicCertificateContent{
		NameToCertificate: map[string]*certKeyFileContent{},
	}

	servingCertBytes, err := ioutil.ReadFile(c.DefaultCertificate.Cert)
	if err != nil {
		return err
	}
	servingKeyBytes, err := ioutil.ReadFile(c.DefaultCertificate.Key)
	if err != nil {
		return err
	}
	newContent.DefaultCertificate = certKeyFileContent{Cert: servingCertBytes, Key: servingKeyBytes}
	if len(c.DefaultCertificate.Cert) > 0 && len(newContent.DefaultCertificate.Cert) == 0 {
		return fmt.Errorf("not loading an empty default cert from %q", c.DefaultCertificate.Cert)
	}
	if len(c.DefaultCertificate.Key) > 0 && len(newContent.DefaultCertificate.Key) == 0 {
		return fmt.Errorf("not loading an empty default key from %q", c.DefaultCertificate.Key)
	}

	caBundle := []byte{}
	for _, caFile := range c.ClientCA.CABundles {
		clientCABytes, err := ioutil.ReadFile(caFile)
		if err != nil {
			return err
		}
		if len(clientCABytes) == 0 {
			return fmt.Errorf("not loading an empty client ca bundle from %q", caFile)
		}
		caBundle = append(caBundle, clientCABytes...)
	}
	newContent.ClientCA = certKeyFileContent{Cert: caBundle}
	clientCAPool := x509.NewCertPool()
	if len(newContent.ClientCA.Cert) > 0 {
		clientCAs, err := cert.ParseCertsPEM(newContent.ClientCA.Cert)
		if err != nil {
			return fmt.Errorf("unable to load client CA file: %v", err)
		}
		for _, cert := range clientCAs {
			clientCAPool.AddCert(cert)
		}
	}

	for key, currRef := range c.NameToCertificate {
		certBytes, err := ioutil.ReadFile(currRef.Cert)
		if err != nil {
			return err
		}
		keyBytes, err := ioutil.ReadFile(currRef.Key)
		if err != nil {
			return err
		}
		if len(currRef.Cert) > 0 && len(certBytes) == 0 {
			return fmt.Errorf("not loading an empty cert from %q for %v", currRef.Cert, key)
		}
		if len(currRef.Key) > 0 && len(keyBytes) == 0 {
			return fmt.Errorf("not loading an empty key from %q for %v", currRef.Key, key)
		}
		newContent.NameToCertificate[key] = &certKeyFileContent{Cert: certBytes, Key: keyBytes}
	}

	if newContent.Equals(&c.currentContent) {
		return nil
	}

	tlsConfigCopy := c.BaseTLSConfig
	tlsConfigCopy.ClientCAs = clientCAPool
	tlsConfigCopy.NameToCertificate = map[string]*tls.Certificate{}

	// load main cert
	if len(newContent.DefaultCertificate.Cert) != 0 || len(newContent.DefaultCertificate.Key) != 0 {
		tlsCert, err := tls.X509KeyPair(newContent.DefaultCertificate.Cert, newContent.DefaultCertificate.Key)
		if err != nil {
			return fmt.Errorf("unable to load server certificate: %v", err)
		}
		tlsConfigCopy.Certificates = []tls.Certificate{tlsCert}
		tlsConfigCopy.Certificates = append(tlsConfigCopy.Certificates, tlsCert)
	}

	// append all named certs. Otherwise, the go tls stack will think no SNI processing
	// is necessary because there is only one cert anyway.
	// Moreover, if ServerCert.CertFile/ServerCert.KeyFile are not set, the first SNI
	// cert will become the default cert. That's what we expect anyway.
	// load SNI certs
	for name, nck := range newContent.NameToCertificate {
		tlsCert, err := tls.X509KeyPair(nck.Cert, nck.Key)
		if err != nil {
			return fmt.Errorf("failed to load SNI cert and key: %v", err)
		}
		tlsConfigCopy.NameToCertificate[name] = &tlsCert
	}
	if c.LoopbackCert != nil {
		tlsConfigCopy.NameToCertificate[LoopbackClientServerNameOverride] = c.LoopbackCert
		tlsConfigCopy.Certificates = append(tlsConfigCopy.Certificates, *c.LoopbackCert)
	}
	newRuntimeConfig := &runtimeDynamicLoader{
		tlsConfig: tlsConfigCopy,
	}

	certs, err := cert.ParseCertsPEM(servingCertBytes)
	if err != nil {
		return err
	}
	for i, crt := range certs {
		klog.V(2).Infof("[%d] %q serving certificate: %s", i, c.DefaultCertificate.Cert, getCertDetail(crt))
	}

	c.currentValue.Store(newRuntimeConfig)
	c.currentContent = newContent // this is single threaded, so we have no locking issue

	return nil
}

func (c *dynamicCertificateContent) Equals(rhs *dynamicCertificateContent) bool {
	if c == nil && rhs == nil {
		return true
	}
	if c == nil && rhs != nil {
		return false
	}
	if c != nil && rhs == nil {
		return false
	}
	cKeys := sets.StringKeySet(c.NameToCertificate)
	rhsKeys := sets.StringKeySet(rhs.NameToCertificate)
	if !cKeys.Equal(rhsKeys) {
		return false
	}

	if !c.DefaultCertificate.Equals(&rhs.DefaultCertificate) {
		return false
	}
	if !c.ClientCA.Equals(&rhs.ClientCA) {
		return false
	}
	for _, key := range cKeys.UnsortedList() {
		if !c.NameToCertificate[key].Equals(rhs.NameToCertificate[key]) {
			return false
		}
	}

	return true
}

func (c *certKeyFileContent) Equals(rhs *certKeyFileContent) bool {
	if c == nil && rhs == nil {
		return true
	}
	if c == nil && rhs != nil {
		return false
	}
	if c != nil && rhs == nil {
		return false
	}
	return reflect.DeepEqual(c.Key, rhs.Key) && reflect.DeepEqual(c.Cert, rhs.Cert)
}

// GetConfigForClient copied from tls.getCertificate
func (c *runtimeDynamicLoader) GetConfigForClient(hello *tls.ClientHelloInfo) (*tls.Config, error) {
	tlsConfigCopy := c.tlsConfig

	// if the client set SNI information, just use our "normal" SNI flow
	if len(hello.ServerName) > 0 {
		return &tlsConfigCopy, nil
	}

	// if the client didn't set SNI, then we need to inspect the requested IP so that we can choose
	// a certificate from our list if we specifically handle that IP
	host, _, err := net.SplitHostPort(hello.Conn.LocalAddr().String())
	if err != nil {
		return &tlsConfigCopy, nil
	}

	ipCert, ok := tlsConfigCopy.NameToCertificate[host]
	if !ok {
		return &tlsConfigCopy, nil
	}
	tlsConfigCopy.Certificates = []tls.Certificate{*ipCert}
	tlsConfigCopy.NameToCertificate = nil

	return &tlsConfigCopy, nil
}
