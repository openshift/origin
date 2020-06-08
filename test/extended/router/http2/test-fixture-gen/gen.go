package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"os"
	"strings"
	"time"
)

// readFile reads all data from filename, or fatally fails if an error
// occurs.
func readFile(filename string) []byte {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("failed to read %q: %v", filename, err)
	}
	return data
}

func addPrefix(lines []string, prefix string) []string {
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return lines
}

// split string into chunks limited in length by size.
// Note: assumes 1:1 mapping between bytes/chars (i.e., non-UTF).
func split(s string, size int) []string {
	var chunks []string

	for len(s) > 0 {
		if len(s) < size {
			size = len(s)
		}
		chunks, s = append(chunks, s[:size]), s[size:]
	}

	return chunks
}

func makeTarData(filenames []string) []byte {
	buf := new(bytes.Buffer)

	gz, err := gzip.NewWriterLevel(buf, gzip.BestCompression)
	if err != nil {
		log.Fatalf("Error: gzip.NewWriterLevel(): %v", err)
	}

	tw := tar.NewWriter(gz)

	for _, filename := range filenames {
		fi, err := os.Stat(filename)
		if err != nil {
			log.Fatalf("Error: failed to stat %q: %v", filename, err)
		}

		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			log.Fatalf("Error: failed to create tar header for %q: %v", filename, err)

		}

		if err := tw.WriteHeader(hdr); err != nil {
			log.Fatal(err)
		}

		if _, err := tw.Write(readFile(filename)); err != nil {
			log.Fatal(err)
		}
	}

	if err := tw.Close(); err != nil {
		log.Fatal(err)
	}

	if err := gz.Close(); err != nil {
		log.Fatal(err)
	}

	return buf.Bytes()
}

func keyToString(key *ecdsa.PrivateKey) []string {
	data, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		log.Fatalf("unable to marshal ECDSA private key: %v", err)
	}

	buf := &bytes.Buffer{}

	if err := pem.Encode(buf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: data}); err != nil {
		log.Fatal(err)
	}

	return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
}

func certToString(derBytes []byte) []string {
	buf := &bytes.Buffer{}

	if err := pem.Encode(buf, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		log.Fatalf("failed to encode cert data: %v", err)
	}

	return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
}

// Creates root cert, cert and key with optional SANs from hosts.
// Certificate is valid from "now" and expires in 100 years.
func genCertKeyPair(hosts ...string) ([]byte, []byte, *ecdsa.PrivateKey) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("failed to generate serial number: %v", err)
	}

	rootKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("failed to generate ECDSA key: %v", err)
	}

	validFor := 100 * time.Hour * 24 * 365 // 100 years
	notBefore := time.Now()
	notAfter := notBefore.Add(validFor)

	rootTemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Red Hat"},
			CommonName:   "Root CA",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	rootDerBytes, err := x509.CreateCertificate(rand.Reader, &rootTemplate, &rootTemplate, &rootKey.PublicKey, rootKey)
	if err != nil {
		log.Fatalf("failed to create root certificate: %v", err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("failed to generate ECDSA key: %v", err)
	}

	serialNumber, err = rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("failed to generate serial number: %v", err)
	}

	leafCertTemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Red Hat"},
			CommonName:   "test_cert",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			leafCertTemplate.IPAddresses = append(leafCertTemplate.IPAddresses, ip)
		} else {
			leafCertTemplate.DNSNames = append(leafCertTemplate.DNSNames, h)
		}
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &leafCertTemplate, &rootTemplate, &leafKey.PublicKey, rootKey)
	if err != nil {
		log.Fatalf("failed to create leaf certificate: %v", err)
	}

	return rootDerBytes, derBytes, leafKey
}

func main() {
	flag.Parse()

	data := split(base64.StdEncoding.EncodeToString(makeTarData(flag.Args())), 76)

	tlsYAMLSpacer := strings.Repeat(" ", 8)
	dataYAMLSpacer := strings.Repeat(" ", 6)

	_, edgeCert, edgeKey := genCertKeyPair()
	_, reencryptCert, reencryptKey := genCertKeyPair()

	fmt.Printf(`apiVersion: v1
kind: Template
objects:
- apiVersion: v1
  kind: Service
  metadata:
    name: http2
    annotations:
      service.beta.openshift.io/serving-cert-secret-name: serving-cert-http2
  spec:
    selector:
      name: http2
    ports:
      - name: https
        protocol: TCP
        port: 27443
        targetPort: 8443
      - name: http
        protocol: TCP
        port: 8080
        targetPort: 8080
- apiVersion: v1
  kind: ConfigMap
  metadata:
    name: src-config
  data:
    data.base64: |
%s
- apiVersion: v1
  kind: Pod
  metadata:
    name: http2
    labels:
      name: http2
  spec:
    containers:
    - image: golang:1.14
      name: server
      command: ["/workdir/http2-server"]
      readinessProbe:
        httpGet:
          path: /healthz
          port: 8080
        initialDelaySeconds: 3
        periodSeconds: 3
      env:
      - name: GODEBUG
        value: http2debug=1
      ports:
      - containerPort: 8443
        protocol: TCP
      - containerPort: 8080
        protocol: TCP
      volumeMounts:
      - name: cert
        mountPath: /etc/serving-cert
      - name: workdir
        mountPath: /workdir
    initContainers:
    - image: golang:1.14
      name: builder
      command: ["/bin/bash", "-c"]
      args:
        - set -e;
          cd /workdir;
          base64 -d /go/src/data.base64 | tar zxf -;
          go build -v -mod=readonly -o /workdir/http2-server server.go;
      env:
      - name: GO111MODULE
        value: "auto"
      - name: GOCACHE
        value: "/tmp"
      - name: GOPROXY
        value: "https://goproxy.golang.org,direct"
      volumeMounts:
      - name: cert
        mountPath: /etc/serving-cert
      - name: src-volume
        mountPath: /go/src
      - name: workdir
        mountPath: /workdir
    volumes:
    - name: src-volume
      configMap:
        name: src-config
    - name: cert
      secret:
        secretName: serving-cert-http2
    - name: workdir
      emptyDir: {}
- apiVersion: route.openshift.io/v1
  kind: Route
  metadata:
    name: http2-default-cert-edge
  spec:
    port:
      targetPort: 8080
    tls:
      termination: edge
      insecureEdgeTerminationPolicy: Redirect
    to:
      kind: Service
      name: http2
      weight: 100
    wildcardPolicy: None
- apiVersion: route.openshift.io/v1
  kind: Route
  metadata:
    name: http2-default-cert-reencrypt
  spec:
    port:
      targetPort: 8443
    tls:
      termination: reencrypt
      insecureEdgeTerminationPolicy: Redirect
    to:
      kind: Service
      name: http2
      weight: 100
    wildcardPolicy: None
- apiVersion: route.openshift.io/v1
  kind: Route
  metadata:
    name: http2-custom-cert-edge
  spec:
    port:
      targetPort: 8080
    tls:
      termination: edge
      insecureEdgeTerminationPolicy: Redirect
      key: |-
%s
      certificate: |-
%s
    to:
      kind: Service
      name: http2
      weight: 100
    wildcardPolicy: None
- apiVersion: route.openshift.io/v1
  kind: Route
  metadata:
    name: http2-custom-cert-reencrypt
  spec:
    port:
      targetPort: 8443
    tls:
      termination: reencrypt
      insecureEdgeTerminationPolicy: Redirect
      key: |-
%s
      certificate: |-
%s
    to:
      kind: Service
      name: http2
      weight: 100
    wildcardPolicy: None
- apiVersion: route.openshift.io/v1
  kind: Route
  metadata:
    name: http2-passthrough
  spec:
    port:
      targetPort: 8443
    tls:
      termination: passthrough
      insecureEdgeTerminationPolicy: Redirect
    to:
      kind: Service
      name: http2
      weight: 100
    wildcardPolicy: None
`,
		strings.Join(addPrefix(data, dataYAMLSpacer), "\n"),
		strings.Join(addPrefix(keyToString(edgeKey), tlsYAMLSpacer), "\n"),
		strings.Join(addPrefix(certToString(edgeCert), tlsYAMLSpacer), "\n"),
		strings.Join(addPrefix(keyToString(reencryptKey), tlsYAMLSpacer), "\n"),
		strings.Join(addPrefix(certToString(reencryptCert), tlsYAMLSpacer), "\n"))
}
