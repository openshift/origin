package certs

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
)

// LoopbackClientServerNameOverride is passed to the apiserver from the loopback client in order to
// select the loopback certificate via SNI if TLS is used.
const LoopbackClientServerNameOverride = "apiserver-loopback-client"

type CertKeyFileReference struct {
	Cert string
	Key  string
}

// DynamicLoader dynamically loads certificates and provides a golang tls compatible dynamic GetCertificate func.
type DynamicLoader struct {
	DefaultCertificate CertKeyFileReference
	NameToCertificate  map[string]*CertKeyFileReference
	LoopbackCert *tls.Certificate

	currentContent dynamicCertificateContent
	currentValue   atomic.Value
}

type dynamicCertificateContent struct {
	DefaultCertificate certKeyFileContent
	NameToCertificate  map[string]*certKeyFileContent
}

type certKeyFileContent struct {
	Cert []byte
	Key  []byte
}

type runtimeDynamicLoader struct {
	Certificates      []tls.Certificate
	NameToCertificate map[string]*tls.Certificate
}

func (c *DynamicLoader) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	uncastObj := c.currentValue.Load()
	if uncastObj == nil {
		return nil, errors.New("tls: configuration not ready")
	}
	runtimeConfig, ok := uncastObj.(*runtimeDynamicLoader)
	if !ok {
		return nil, errors.New("tls: unexpected config type")
	}
	return runtimeConfig.GetCertificate(clientHello)
}

func (c *DynamicLoader) Run(stopCh <-chan struct{}) {
	glog.Infof("Starting DynamicLoader")
	defer glog.Infof("Shutting down DynamicLoader")

	go wait.Until(func() {
		err := c.CheckCerts()
		if err != nil {
			utilruntime.HandleError(err)
		}
	}, 1*time.Minute, stopCh)

	<-stopCh
}

func (c *DynamicLoader) CheckCerts() error {
	newContent := dynamicCertificateContent{
		NameToCertificate: map[string]*certKeyFileContent{},
	}

	certBytes, err := ioutil.ReadFile(c.DefaultCertificate.Cert)
	if err != nil {
		return err
	}
	keyBytes, err := ioutil.ReadFile(c.DefaultCertificate.Key)
	if err != nil {
		return err
	}
	newContent.DefaultCertificate = certKeyFileContent{Cert: certBytes, Key: keyBytes}

	for key, currRef := range c.NameToCertificate {
		certBytes, err := ioutil.ReadFile(currRef.Cert)
		if err != nil {
			return err
		}
		keyBytes, err := ioutil.ReadFile(currRef.Key)
		if err != nil {
			return err
		}
		newContent.NameToCertificate[key] = &certKeyFileContent{Cert: certBytes, Key: keyBytes}
	}

	if newContent.Equals(&c.currentContent) {
		return nil
	}

	newRuntimeConfig, err := newContent.ToRuntimeConfig()
	if err != nil {
		return err
	}
	if c.LoopbackCert != nil {
		newRuntimeConfig.NameToCertificate[LoopbackClientServerNameOverride] = c.LoopbackCert
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
	for _, key := range cKeys.UnsortedList() {
		if !c.NameToCertificate[key].Equals(rhs.NameToCertificate[key]) {
			return false
		}
	}

	return true
}

func (c *dynamicCertificateContent) ToRuntimeConfig() (*runtimeDynamicLoader, error) {
	ret := &runtimeDynamicLoader{
		NameToCertificate: map[string]*tls.Certificate{},
	}

	// load main cert
	if len(c.DefaultCertificate.Cert) != 0 || len(c.DefaultCertificate.Key) != 0 {
		tlsCert, err := tls.X509KeyPair(c.DefaultCertificate.Cert, c.DefaultCertificate.Key)
		if err != nil {
			return nil, fmt.Errorf("unable to load server certificate: %v", err)
		}
		ret.Certificates = []tls.Certificate{tlsCert}
	}

	// load SNI certs
	for name, nck := range c.NameToCertificate {
		tlsCert, err := tls.X509KeyPair(nck.Cert, nck.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to load SNI cert and key: %v", err)
		}
		ret.NameToCertificate[name] = &tlsCert
	}

	return ret, nil
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

// GetCertificate copied from tls.getCertificate
func (c *runtimeDynamicLoader) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	if len(c.Certificates) == 0 {
		return nil, errors.New("tls: no certificates configured")
	}

	if c.NameToCertificate == nil {
		// There's only one choice, so no point doing any work.
		return &c.Certificates[0], nil
	}

	name := strings.ToLower(clientHello.ServerName)
	for len(name) > 0 && name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}

	if cert, ok := c.NameToCertificate[name]; ok {
		return cert, nil
	}

	// try replacing labels in the name with wildcards until we get a
	// match.
	labels := strings.Split(name, ".")
	for i := range labels {
		labels[i] = "*"
		candidate := strings.Join(labels, ".")
		if cert, ok := c.NameToCertificate[candidate]; ok {
			return cert, nil
		}
	}

	// If nothing matches, return the first certificate.
	return &c.Certificates[0], nil
}
