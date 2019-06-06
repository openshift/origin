package deprecatedcerts

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog"
)

type CreateKeyPairOptions struct {
	PublicKeyFile  string
	PrivateKeyFile string

	Overwrite bool

	genericclioptions.IOStreams
}

func (o CreateKeyPairOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.PublicKeyFile) == 0 {
		return errors.New("--public-key must be provided")
	}
	if len(o.PrivateKeyFile) == 0 {
		return errors.New("--private-key must be provided")
	}
	if o.PublicKeyFile == o.PrivateKeyFile {
		return errors.New("--public-key and --private-key must be different")
	}

	return nil
}

func (o CreateKeyPairOptions) CreateKeyPair() error {
	klog.V(4).Infof("Creating a key pair with: %#v", o)

	if !o.Overwrite {
		if _, err := os.Stat(o.PrivateKeyFile); err == nil {
			klog.V(3).Infof("Keeping existing private key file %s\n", o.PrivateKeyFile)
			return nil
		}
		if _, err := os.Stat(o.PublicKeyFile); err == nil {
			klog.V(3).Infof("Keeping existing public key file %s\n", o.PublicKeyFile)
			return nil
		}
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	if err := writePrivateKeyFile(o.PrivateKeyFile, privateKey); err != nil {
		return err
	}

	if err := writePublicKeyFile(o.PublicKeyFile, &privateKey.PublicKey); err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "Generated new key pair as %s and %s\n", o.PublicKeyFile, o.PrivateKeyFile)

	return nil
}

func writePublicKeyFile(path string, key *rsa.PublicKey) error {
	// ensure parent dir
	if err := os.MkdirAll(filepath.Dir(path), os.FileMode(0755)); err != nil {
		return err
	}

	derBytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return err
	}

	b := bytes.Buffer{}
	if err := pem.Encode(&b, &pem.Block{Type: "PUBLIC KEY", Bytes: derBytes}); err != nil {
		return err
	}

	return ioutil.WriteFile(path, b.Bytes(), os.FileMode(0600))
}

func writePrivateKeyFile(path string, key *rsa.PrivateKey) error {
	// ensure parent dir
	if err := os.MkdirAll(filepath.Dir(path), os.FileMode(0755)); err != nil {
		return err
	}

	b := bytes.Buffer{}
	err := pem.Encode(&b, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, b.Bytes(), os.FileMode(0600))
}
