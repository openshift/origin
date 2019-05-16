package certs

import (
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"reflect"
	"sync/atomic"
	"time"

	"k8s.io/client-go/util/cert"

	"k8s.io/klog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubex509 "k8s.io/apiserver/pkg/authentication/request/x509"
)

func NewDynamicCA(caBundleFilename string) *DynamicCA {
	return &DynamicCA{
		caFile: caFileReference{Cert: caBundleFilename},
	}
}

// DynamicCA dynamically loads ca bundle and provides a verifier compatible with kube x509 authentication
type DynamicCA struct {
	caFile caFileReference

	currentContent caFileContent
	currentValue   atomic.Value
}

type caFileReference struct {
	Cert string
}

type caFileContent struct {
	Cert []byte
}

type runtimeDynamicCA struct {
	verifyOptions x509.VerifyOptions
}

func (c *DynamicCA) GetVerifier() x509.VerifyOptions {
	uncastObj := c.currentValue.Load()
	if uncastObj == nil {
		panic("tls: configuration not ready")
	}
	runtimeConfig, ok := uncastObj.(*runtimeDynamicCA)
	if !ok {
		panic("tls: unexpected config type")
	}
	return runtimeConfig.GetVerifier()
}

func (c *DynamicCA) Run(stopCh <-chan struct{}) {
	klog.Infof("Starting DynamicCA: %v", c.caFile.Cert)
	defer klog.Infof("Shutting down DynamicCA: %v", c.caFile.Cert)

	go wait.Until(func() {
		err := c.CheckCerts()
		if err != nil {
			utilruntime.HandleError(err)
		}
	}, 1*time.Minute, stopCh)

	<-stopCh
}

func (c *DynamicCA) CheckCerts() error {
	certBytes, err := ioutil.ReadFile(c.caFile.Cert)
	if err != nil {
		return err
	}
	if len(certBytes) == 0 {
		return fmt.Errorf("ca-bundle %q must not be empty", c.caFile.Cert)
	}
	newContent := caFileContent{Cert: certBytes}

	if newContent.Equals(&c.currentContent) {
		return nil
	}

	certs, err := cert.ParseCertsPEM(newContent.Cert)
	if err != nil {
		return fmt.Errorf("unable to load client CA file %q: %v", c.caFile.Cert, err)
	}
	pool := x509.NewCertPool()
	for i, crt := range certs {
		pool.AddCert(crt)
		klog.V(2).Infof("[%d] %q client-ca certificate: %s", i, c.caFile.Cert, getCertDetail(crt))
	}

	verifyOpts := kubex509.DefaultVerifyOptions()
	verifyOpts.Roots = pool
	newRuntimeConfig := &runtimeDynamicCA{verifyOptions: verifyOpts}

	c.currentValue.Store(newRuntimeConfig)
	c.currentContent = newContent // this is single threaded, so we have no locking issue

	return nil
}

func (c *caFileContent) Equals(rhs *caFileContent) bool {
	if c == nil && rhs == nil {
		return true
	}
	if c == nil && rhs != nil {
		return false
	}
	if c != nil && rhs == nil {
		return false
	}
	return reflect.DeepEqual(c.Cert, rhs.Cert)
}

func (c *runtimeDynamicCA) GetVerifier() x509.VerifyOptions {
	return c.verifyOptions
}
