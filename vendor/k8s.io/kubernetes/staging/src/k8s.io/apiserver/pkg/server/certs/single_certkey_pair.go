package certs

import (
	"fmt"
	"io/ioutil"
	"sync/atomic"
	"time"

	"k8s.io/klog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/cert"
)

// ReactionFunc is a func that can be called on a cert change
type ReactionFunc func()

func NewDynamicCertKeyPairLoader(certFile, keyFile string, certChangeCallback ReactionFunc) *DynamicCertKeyPairLoader {
	return &DynamicCertKeyPairLoader{
		fileReference: CertKeyFileReference{
			Cert: certFile,
			Key:  keyFile,
		},
		certChangeCallback: certChangeCallback,
	}
}

// DynamicLoader dynamically loads a single cert/key pair
type DynamicCertKeyPairLoader struct {
	fileReference CertKeyFileReference

	certChangeCallback ReactionFunc
	currentContent     certKeyFileContent
	currentValue       atomic.Value
}

type runtimeCertKeyPair struct {
	rawContent certKeyFileContent
}

func (c *DynamicCertKeyPairLoader) GetRawCert() []byte {
	uncastObj := c.currentValue.Load()
	if uncastObj == nil {
		panic("not ready")
	}
	runtimeConfig, ok := uncastObj.(*runtimeCertKeyPair)
	if !ok {
		panic("unexpected config type")
	}
	return runtimeConfig.GetRawCert()
}

func (c *DynamicCertKeyPairLoader) GetRawKey() []byte {
	uncastObj := c.currentValue.Load()
	if uncastObj == nil {
		panic("not ready")
	}
	runtimeConfig, ok := uncastObj.(*runtimeCertKeyPair)
	if !ok {
		panic("unexpected config type")
	}
	return runtimeConfig.GetRawKey()
}

func (c *DynamicCertKeyPairLoader) Run(stopCh <-chan struct{}) {
	klog.Infof("Starting DynamicCertKeyPairLoader")
	defer klog.Infof("Shutting down DynamicCertKeyPairLoader")

	go wait.Until(func() {
		err := c.CheckCerts()
		if err != nil {
			utilruntime.HandleError(err)
		}
	}, 1*time.Minute, stopCh)

	<-stopCh
}

func (c *DynamicCertKeyPairLoader) CheckCerts() error {
	servingCertBytes, err := ioutil.ReadFile(c.fileReference.Cert)
	if err != nil {
		return err
	}
	if len(servingCertBytes) == 0 {
		return fmt.Errorf("cert %q must not be empty", c.fileReference.Cert)
	}
	servingKeyBytes, err := ioutil.ReadFile(c.fileReference.Key)
	if err != nil {
		return err
	}
	if len(servingKeyBytes) == 0 {
		return fmt.Errorf("key %q must not be empty", c.fileReference.Key)
	}
	newContent := certKeyFileContent{Cert: servingCertBytes, Key: servingKeyBytes}

	if newContent.Equals(&c.currentContent) {
		return nil
	}

	newRuntimeConfig := &runtimeCertKeyPair{
		rawContent: newContent,
	}
	c.currentValue.Store(newRuntimeConfig)
	c.currentContent = newContent // this is single threaded, so we have no locking issue

	certs, err := cert.ParseCertsPEM(newContent.Cert)
	if err != nil {
		return err
	}
	for i, crt := range certs {
		klog.V(2).Infof("[%d] %q certificate: %s", i, c.fileReference.Cert, getCertDetail(crt))
	}

	if c.certChangeCallback != nil {
		c.certChangeCallback()
	}

	return nil
}

func (c *runtimeCertKeyPair) GetRawCert() []byte {
	return c.rawContent.Cert
}

func (c *runtimeCertKeyPair) GetRawKey() []byte {
	return c.rawContent.Key
}
