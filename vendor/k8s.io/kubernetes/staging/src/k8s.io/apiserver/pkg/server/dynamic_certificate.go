package server

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

// DynamicCertificateConfig provides a way to have dynamic https configuration for a fixed set of files.
// You need to remember to call CheckCerts before usage and start the go routine for Run.
type DynamicCertificateConfig struct {
	CurrentValue atomic.Value

	// LoopbackCert holds the special certificate that we create for loopback connections
	LoopbackCert *tls.Certificate

	CurrentContent DynamicCertificateContent

	CertificateReferences DynamicCertificateReferences
}

type DynamicCertificateReferences struct {
	DefaultCertificate CertKeyFileReference
	NameToCertificate  map[string]*CertKeyFileReference
}

type CertKeyFileReference struct {
	Cert string
	Key  string
}

type DynamicCertificateContent struct {
	DefaultCertificate CertKeyFileContent
	NameToCertificate  map[string]*CertKeyFileContent
}

type CertKeyFileContent struct {
	Cert []byte
	Key  []byte
}

type RuntimeDynamicCertificateConfig struct {
	Certificates      []tls.Certificate
	NameToCertificate map[string]*tls.Certificate
}

func (c *DynamicCertificateConfig) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	uncastObj := c.CurrentValue.Load()
	if uncastObj == nil {
		return nil, errors.New("tls: configuration not ready")
	}
	runtimeConfig, ok := uncastObj.(*RuntimeDynamicCertificateConfig)
	if !ok {
		return nil, errors.New("tls: unexpected config type")
	}
	return runtimeConfig.GetCertificate(clientHello)
}

func (c *DynamicCertificateConfig) Run(stopCh <-chan struct{}) {
	glog.Infof("Starting DynamicCertificateConfig")
	defer glog.Infof("Shutting down DynamicCertificateConfig")

	go wait.Until(func() {
		err := c.CheckCerts()
		if err != nil {
			utilruntime.HandleError(err)
		}
	}, 1*time.Minute, stopCh)

	<-stopCh
}

func (c *DynamicCertificateConfig) CheckCerts() error {
	newContent := DynamicCertificateContent{
		NameToCertificate: map[string]*CertKeyFileContent{},
	}

	certBytes, err := ioutil.ReadFile(c.CertificateReferences.DefaultCertificate.Cert)
	if err != nil {
		return err
	}
	keyBytes, err := ioutil.ReadFile(c.CertificateReferences.DefaultCertificate.Key)
	if err != nil {
		return err
	}
	newContent.DefaultCertificate = CertKeyFileContent{Cert: certBytes, Key: keyBytes}

	for key, currRef := range c.CertificateReferences.NameToCertificate {
		certBytes, err := ioutil.ReadFile(currRef.Cert)
		if err != nil {
			return err
		}
		keyBytes, err := ioutil.ReadFile(currRef.Key)
		if err != nil {
			return err
		}
		newContent.NameToCertificate[key] = &CertKeyFileContent{Cert: certBytes, Key: keyBytes}
	}

	if newContent.Equals(&c.CurrentContent) {
		return nil
	}

	newRuntimeConfig, err := newContent.ToRuntimeConfig()
	if err != nil {
		return err
	}
	if c.LoopbackCert != nil {
		newRuntimeConfig.NameToCertificate[LoopbackClientServerNameOverride] = c.LoopbackCert
	}
	c.CurrentValue.Store(newRuntimeConfig)
	c.CurrentContent = newContent // this is single threaded, so we have no locking issue

	return nil
}

func (c *DynamicCertificateContent) Equals(rhs *DynamicCertificateContent) bool {
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

func (c *DynamicCertificateContent) ToRuntimeConfig() (*RuntimeDynamicCertificateConfig, error) {
	ret := &RuntimeDynamicCertificateConfig{
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

func (c *CertKeyFileContent) Equals(rhs *CertKeyFileContent) bool {
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
func (c *RuntimeDynamicCertificateConfig) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
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
