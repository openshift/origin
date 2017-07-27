package admin

import (
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"

	"k8s.io/client-go/util/cert"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

const CSR_BLOCK_TYPE = "CERTIFICATE REQUEST"

type CreateServerCSROptions struct {
	PrivateKeyFile string
	CSRFile        string
	Hostnames      []string
}

func CSRFromFile(file string) (*x509.CertificateRequest, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return CSRFromPEM(data)
}
func CSRFromPEM(data []byte) (*x509.CertificateRequest, error) {
	var block *pem.Block
	for {
		block, data = pem.Decode(data)
		if block == nil {
			break
		}
		if block.Type != CSR_BLOCK_TYPE {
			continue
		}
		return x509.ParseCertificateRequest(block.Bytes)
	}
	return nil, fmt.Errorf("no CERTIFICATE REQUEST block found")
}

func (o *CreateServerCSROptions) CreateServerCSR() error {
	privateKey, err := cert.PrivateKeyFromFile(o.PrivateKeyFile)
	if err != nil {
		return err
	}
	req := &x509.CertificateRequest{
		Subject:            pkix.Name{CommonName: o.Hostnames[0]},
		SignatureAlgorithm: x509.SHA256WithRSA,
		ExtraExtensions:    []pkix.Extension{crypto.KeyUsageEnciphermentDigitalSignature, crypto.KeyUsageServerAuthExtension, crypto.BasicConstraintsValidExtension},
	}
	req.IPAddresses, req.DNSNames = crypto.IPAddressesDNSNames(o.Hostnames)
	csrASNData, err := x509.CreateCertificateRequest(rand.Reader, req, privateKey)
	if err != nil {
		return err
	}
	csrPEMBlock := &pem.Block{Type: CSR_BLOCK_TYPE, Bytes: csrASNData}
	csrPEMData := pem.EncodeToMemory(csrPEMBlock)
	return ioutil.WriteFile(o.CSRFile, csrPEMData, os.FileMode(0644))
}

type CreateClientCSROptions struct {
	PrivateKeyFile string
	CSRFile        string
	User           string
	Groups         []string
}

func (o *CreateClientCSROptions) CreateClientCSR() error {
	privateKey, err := cert.PrivateKeyFromFile(o.PrivateKeyFile)
	if err != nil {
		return err
	}
	req := &x509.CertificateRequest{
		Subject:            pkix.Name{CommonName: o.User, Organization: o.Groups},
		SignatureAlgorithm: x509.SHA256WithRSA,
		ExtraExtensions:    []pkix.Extension{crypto.KeyUsageEnciphermentDigitalSignature, crypto.KeyUsageClientAuthExtension, crypto.BasicConstraintsValidExtension},
	}
	csrASNData, err := x509.CreateCertificateRequest(rand.Reader, req, privateKey)
	csrPEMBlock := &pem.Block{Type: CSR_BLOCK_TYPE, Bytes: csrASNData}
	csrPEMData := pem.EncodeToMemory(csrPEMBlock)
	return ioutil.WriteFile(o.CSRFile, csrPEMData, os.FileMode(0644))
}
