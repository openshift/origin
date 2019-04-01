// Copyright 2016 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// certcheck is a utility to show and check the contents of certificates.
package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/certificate-transparency-go/x509util"
)

var (
	root                      = flag.String("root", "", "Root CA certificate file")
	intermediate              = flag.String("intermediate", "", "Intermediate CA certificate file")
	useSystemRoots            = flag.Bool("system_roots", false, "Use system roots")
	verbose                   = flag.Bool("verbose", false, "Verbose output")
	validate                  = flag.Bool("validate", false, "Validate certificate signatures")
	timecheck                 = flag.Bool("timecheck", false, "Check current validity of certificate")
	revokecheck               = flag.Bool("check_revocation", false, "Check revocation status of certificate")
	ignoreUnknownCriticalExts = flag.Bool("ignore_unknown_critical_exts", false, "Ignore unknown-critical-extension errors")
)

func addCerts(filename string, pool *x509.CertPool) {
	if filename != "" {
		dataList, err := x509util.ReadPossiblePEMFile(filename, "CERTIFICATE")
		if err != nil {
			glog.Exitf("Failed to read certificate file: %v", err)
		}
		for _, data := range dataList {
			certs, err := x509.ParseCertificates(data)
			if err != nil {
				glog.Exitf("Failed to parse certificate from %s: %v", filename, err)
			}
			for _, cert := range certs {
				pool.AddCert(cert)
			}
		}
	}
}

func main() {
	flag.Parse()

	failed := false
	for _, target := range flag.Args() {
		var err error
		var chain []*x509.Certificate
		if strings.HasPrefix(target, "https://") {
			chain, err = chainFromSite(target)
		} else {
			chain, err = chainFromFile(target)
		}
		if err != nil {
			glog.Errorf("%v", err)
			failed = true
			continue
		}
		for _, cert := range chain {
			if *verbose {
				fmt.Print(x509util.CertificateToString(cert))
			}
			if *revokecheck {
				if err := checkRevocation(cert, *verbose); err != nil {
					glog.Errorf("%s: certificate is revoked: %v", target, err)
					failed = true
				}
			}
		}
		if *validate && len(chain) > 0 {
			if *ignoreUnknownCriticalExts {
				// We don't want failures from Verify due to unknown critical extensions,
				// so clear them out.
				for _, cert := range chain {
					cert.UnhandledCriticalExtensions = nil
				}
			}
			if err := validateChain(chain, *timecheck, *root, *intermediate, *useSystemRoots); err != nil {
				glog.Errorf("%s: verification error: %v", target, err)
				failed = true
			}
		}
	}
	if failed {
		os.Exit(1)
	}
}

func chainFromSite(target string) ([]*x509.Certificate, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to parse URL: %v", target, err)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("%s: non-https URL provided", target)
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		host += ":443"
	}

	conn, err := tls.Dial("tcp", host, &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return nil, fmt.Errorf("%s: failed to dial %q: %v", target, host, err)
	}
	defer conn.Close()

	// Convert base crypto/x509.Certificates to our forked x509.Certificate type.
	goChain := conn.ConnectionState().PeerCertificates
	chain := make([]*x509.Certificate, len(goChain))
	for i, goCert := range goChain {
		cert, err := x509.ParseCertificate(goCert.Raw)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to convert Go Certificate [%d]: %v", target, i, err)
		}
		chain[i] = cert
	}

	return chain, nil
}

func chainFromFile(filename string) ([]*x509.Certificate, error) {
	dataList, err := x509util.ReadPossiblePEMFile(filename, "CERTIFICATE")
	if err != nil {
		return nil, fmt.Errorf("%s: failed to read data: %v", filename, err)
	}
	var chain []*x509.Certificate
	for _, data := range dataList {
		certs, err := x509.ParseCertificates(data)
		if x509.IsFatal(err) {
			return nil, fmt.Errorf("%s: failed to parse: %v", filename, err)
		}
		if err != nil {
			glog.Errorf("%s: non-fatal error parsing: %v", filename, err)
		}
		chain = append(chain, certs...)
	}
	return chain, nil
}

func validateChain(chain []*x509.Certificate, timecheck bool, rootsFile, intermediatesFile string, useSystemRoots bool) error {
	roots := x509.NewCertPool()
	if useSystemRoots {
		systemRoots, err := x509.SystemCertPool()
		if err != nil {
			glog.Errorf("Failed to get system roots: %v", err)
		}
		roots = systemRoots
	}
	opts := x509.VerifyOptions{
		KeyUsages:         []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		Roots:             roots,
		Intermediates:     x509.NewCertPool(),
		DisableTimeChecks: !timecheck,
	}
	addCerts(rootsFile, opts.Roots)
	addCerts(intermediatesFile, opts.Intermediates)

	if !useSystemRoots && len(rootsFile) == 0 {
		// No root CA certs provided, so assume the chain is self-contained.
		count := len(chain)
		if len(chain) > 1 {
			last := chain[len(chain)-1]
			if bytes.Equal(last.RawSubject, last.RawIssuer) {
				opts.Roots.AddCert(last)
				count--
			}
		}
	}
	if len(intermediatesFile) == 0 {
		// No intermediate CA certs provided, so assume later entries in the chain are intermediates.
		for i := 1; i < len(chain); i++ {
			opts.Intermediates.AddCert(chain[i])
		}
	}
	_, err := chain[0].Verify(opts)
	return err
}

func checkRevocation(cert *x509.Certificate, verbose bool) error {
	for _, crldp := range cert.CRLDistributionPoints {
		crlDataList, err := x509util.ReadPossiblePEMURL(crldp, "X509 CRL")
		if err != nil {
			glog.Errorf("failed to retrieve CRL from %q: %v", crldp, err)
			continue
		}
		for _, crlData := range crlDataList {
			crl, err := x509.ParseCertificateList(crlData)
			if x509.IsFatal(err) {
				glog.Errorf("failed to parse CRL from %q: %v", crldp, err)
				continue
			}
			if err != nil {
				glog.Errorf("non-fatal error parsing CRL from %q: %v", crldp, err)
			}
			if verbose {
				fmt.Printf("\nRevocation data from %s:\n", crldp)
				fmt.Print(x509util.CRLToString(crl))
			}
			for _, c := range crl.TBSCertList.RevokedCertificates {
				if c.SerialNumber.Cmp(cert.SerialNumber) == 0 {
					return fmt.Errorf("certificate is revoked since %v", c.RevocationTime)
				}
			}
		}
	}
	return nil
}
