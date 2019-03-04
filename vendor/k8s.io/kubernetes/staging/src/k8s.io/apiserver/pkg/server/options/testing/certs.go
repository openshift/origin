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

package testing

import (
	"bytes"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

type TestCertSpec struct {
	Host       string
	Names, IPs []string // in certificate
}

func parseIPList(ips []string) []net.IP {
	var netIPs []net.IP
	for _, ip := range ips {
		netIPs = append(netIPs, net.ParseIP(ip))
	}
	return netIPs
}

func CreateTestTLSCerts(spec TestCertSpec) (tlsCert tls.Certificate, err error) {
	certPem, keyPem, err := generateSelfSignedCertKey(spec.Host, parseIPList(spec.IPs), spec.Names)
	if err != nil {
		return tlsCert, err
	}

	tlsCert, err = tls.X509KeyPair(certPem, keyPem)
	return tlsCert, err
}

func GetOrCreateTestCertFiles(certFileName, keyFileName string, spec TestCertSpec) (err error) {
	if _, err := os.Stat(certFileName); err == nil {
		if _, err := os.Stat(keyFileName); err == nil {
			return nil
		}
	}

	certPem, keyPem, err := generateSelfSignedCertKey(spec.Host, parseIPList(spec.IPs), spec.Names)
	if err != nil {
		return err
	}

	os.MkdirAll(filepath.Dir(certFileName), os.FileMode(0755))
	err = ioutil.WriteFile(certFileName, certPem, os.FileMode(0755))
	if err != nil {
		return err
	}

	os.MkdirAll(filepath.Dir(keyFileName), os.FileMode(0755))
	err = ioutil.WriteFile(keyFileName, keyPem, os.FileMode(0755))
	if err != nil {
		return err
	}

	return nil
}

func CACertFromBundle(bundlePath string) (*x509.Certificate, error) {
	pemData, err := ioutil.ReadFile(bundlePath)
	if err != nil {
		return nil, err
	}

	// fetch last block
	var block *pem.Block
	for {
		var nextBlock *pem.Block
		nextBlock, pemData = pem.Decode(pemData)
		if nextBlock == nil {
			if block == nil {
				return nil, fmt.Errorf("no certificate found in %q", bundlePath)

			}
			return x509.ParseCertificate(block.Bytes)
		}
		block = nextBlock
	}
}

func X509CertSignature(cert *x509.Certificate) string {
	return base64.StdEncoding.EncodeToString(cert.Signature)
}

func CertFileSignature(certFile, keyFile string) (string, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return "", err
	}
	return CertSignature(cert)
}

func CertSignature(cert tls.Certificate) (string, error) {
	x509Certs, err := x509.ParseCertificates(cert.Certificate[0])
	if err != nil {
		return "", err
	}
	return X509CertSignature(x509Certs[0]), nil
}

// generateSelfSignedCertKey creates a self-signed certificate and key for the given host.
// Host may be an IP or a DNS name
// You may also specify additional subject alt names (either ip or dns names) for the certificate
func generateSelfSignedCertKey(host string, alternateIPs []net.IP, alternateDNS []string) ([]byte, []byte, error) {
	priv, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("%s@%d", host, time.Now().Unix()),
		},
		NotBefore: time.Unix(0, 0),
		NotAfter:  time.Now().Add(time.Hour * 24 * 365 * 100),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA: true,
	}

	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, host)
	}

	template.IPAddresses = append(template.IPAddresses, alternateIPs...)
	template.DNSNames = append(template.DNSNames, alternateDNS...)

	derBytes, err := x509.CreateCertificate(cryptorand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}

	// Generate cert
	certBuffer := bytes.Buffer{}
	if err := pem.Encode(&certBuffer, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return nil, nil, err
	}

	// Generate key
	keyBuffer := bytes.Buffer{}
	if err := pem.Encode(&keyBuffer, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
		return nil, nil, err
	}

	return certBuffer.Bytes(), keyBuffer.Bytes(), nil
}
