package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
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

func main() {
	flag.Parse()

	data := split(base64.StdEncoding.EncodeToString(makeTarData(flag.Args())), 76)

	fmt.Printf(`apiVersion: v1
kind: Template
objects:
- apiVersion: v1
  kind: Service
  metadata:
    name: grpc-interop
    annotations:
      service.beta.openshift.io/serving-cert-secret-name: service-certs
  spec:
    selector:
      app: grpc-interop
    ports:
      - port: 8443
        name: https
        targetPort: 8443
        protocol: TCP
      - port: 1110
        name: h2c
        targetPort: 1110
        protocol: TCP
- apiVersion: v1
  kind: ConfigMap
  labels:
    app: grpc-interop
  metadata:
    name: src-config
  data:
    data.base64: |
%s
- apiVersion: v1
  kind: ConfigMap
  metadata:
    annotations:
      service.beta.openshift.io/inject-cabundle: "true"
    labels:
      app: grpc-interop
    name: service-ca
- apiVersion: v1
  kind: Pod
  metadata:
    name: grpc-interop
    labels:
      app: grpc-interop
  spec:
    containers:
    - image: golang:1.14
      name: server
      command: ["/workdir/grpc-server"]
      env:
      - name: GRPC_GO_LOG_VERBOSITY_LEVEL
        value: "99"
      - name: GRPC_GO_LOG_SEVERITY_LEVEL
        value: "info"
      - name: GODEBUG
        value: http2debug=1
      readinessProbe:
        httpGet:
          path: /healthz
          port: 8080
        initialDelaySeconds: 10
        periodSeconds: 3
      ports:
      - containerPort: 8443
        protocol: TCP
      - containerPort: 1110
        protocol: TCP
      - containerPort: 8080
        protocol: TCP
      volumeMounts:
      - name: service-certs
        mountPath: /etc/service-certs
      - name: tmp
        mountPath: /var/run
      - name: workdir
        mountPath: /workdir
      readOnly: true
    - image: golang:1.14
      name: client-shell
      command: ["/bin/bash"]
      args: ["-c", "sleep 100000"]
      readinessProbe:
        httpGet:
          path: /healthz
          port: 8080
        initialDelaySeconds: 10
        periodSeconds: 3
      ports:
      - containerPort: 8080
        protocol: TCP
      volumeMounts:
      - name: service-certs
        secret:
          secretName: service-certs
        mountPath: /etc/service-certs
      - name: tmp
        mountPath: /var/run
      - name: workdir
        mountPath: /workdir
      - name: service-ca
        mountPath: /etc/service-ca
    initContainers:
    - image: golang:1.14
      name: builder
      command: ["/bin/bash", "-c"]
      args:
        - set -e;
          cd /workdir;
          base64 -d /go/src/data.base64 | tar zxf -;
          go build -v -mod=readonly -o /workdir/grpc-client client.go;
          go build -v -mod=readonly -o /workdir/grpc-server server.go;
      env:
      - name: GO111MODULE
        value: "auto"
      - name: GOCACHE
        value: "/tmp"
      - name: GOPROXY
        value: "https://goproxy.golang.org,direct"
      volumeMounts:
      - name: src-volume
        mountPath: /go/src
      - name: tmp
        mountPath: /var/run
      - name: workdir
        mountPath: /workdir
    volumes:
    - name: src-volume
      configMap:
        name: src-config
    - name: service-certs
      secret:
        secretName: service-certs
    - name: tmp
      emptyDir: {}
    - name: workdir
      emptyDir: {}
    - configMap:
        items:
        - key: service-ca.crt
          path: service-ca.crt
        name: service-ca
      name: service-ca
  labels:
    app: grpc-interop
- apiVersion: route.openshift.io/v1
  kind: Route
  metadata:
    annotations:
      haproxy.router.openshift.io/enable-h2c: "true"
    labels:
      app: grpc-interop
    name: grpc-interop-edge
  spec:
    port:
      targetPort: 1110
    tls:
      termination: edge
      insecureEdgeTerminationPolicy: Redirect
      key: |-
        -----BEGIN EC PRIVATE KEY-----
        MHcCAQEEILIc8XqIk7IYA4rXr88Vm/h2uor640AdLH1pa8Fny+kOoAoGCCqGSM49
        AwEHoUQDQgAEEwTH3nocuTpgDmVYN2Ep9DqTIyErsPeeY+mVJB/rzLHI+00cTRBf
        pxb9UEx7a+fWhG7/yONFcKEv5ZlIGbFG8w==
        -----END EC PRIVATE KEY-----
      certificate: |-
        -----BEGIN CERTIFICATE-----
        MIIBizCCATGgAwIBAgIQTEs2NjcG/9z0WyA5M0LrWTAKBggqhkjOPQQDAjAoMRQw
        EgYDVQQKEwtDZXJ0IEdlbiBDbzEQMA4GA1UEAxMHUm9vdCBDQTAgFw0yMDA1MTgw
        OTU1MTFaGA8yMTIwMDQyNDA5NTUxMVowLjEZMBcGA1UEChMQQ2VydCBHZW4gQ29t
        cGFueTERMA8GA1UEAxMIdGVzdGNlcnQwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNC
        AAQTBMfeehy5OmAOZVg3YSn0OpMjISuw955j6ZUkH+vMscj7TRxNEF+nFv1QTHtr
        59aEbv/I40VwoS/lmUgZsUbzozUwMzAOBgNVHQ8BAf8EBAMCBaAwEwYDVR0lBAww
        CgYIKwYBBQUHAwEwDAYDVR0TAQH/BAIwADAKBggqhkjOPQQDAgNIADBFAiEAxsUz
        yNl/2DJktyy7xDmBeZPybPbyWv4ApnpaaoQoVUMCIFEwJtfwGEO+NN4HAbPKQTOS
        pK6nUhQqHPWhu7t15yec
        -----END CERTIFICATE-----
    to:
      kind: Service
      name: grpc-interop
      weight: 100
    wildcardPolicy: None
- apiVersion: route.openshift.io/v1
  kind: Route
  metadata:
    labels:
      app: grpc-interop
    name: grpc-interop-reencrypt
  spec:
    port:
      targetPort: 8443
    tls:
      termination: reencrypt
      insecureEdgeTerminationPolicy: Redirect
      key: |-
        -----BEGIN EC PRIVATE KEY-----
        MHcCAQEEILY+Vji+NywGUmWhjLGf0teANh5t8CVSl7yNCQ/1v05woAoGCCqGSM49
        AwEHoUQDQgAEqHrMdKmzQfVZiBUHX0AI9vdMp5/GGLNc7nOA7xNmsGHUq642Av3x
        k3gJlKJOd3Il6gbP+Dd8jLVaVcPHQY/d0g==
        -----END EC PRIVATE KEY-----
      certificate: |
        -----BEGIN CERTIFICATE-----
        MIIBjDCCATKgAwIBAgIRAJGHhBTYpRl7nsRAVtlKPbAwCgYIKoZIzj0EAwIwKDEU
        MBIGA1UEChMLQ2VydCBHZW4gQ28xEDAOBgNVBAMTB1Jvb3QgQ0EwIBcNMjAwNTE4
        MDk1NTI4WhgPMjEyMDA0MjQwOTU1MjhaMC4xGTAXBgNVBAoTEENlcnQgR2VuIENv
        bXBhbnkxETAPBgNVBAMTCHRlc3RjZXJ0MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcD
        QgAEqHrMdKmzQfVZiBUHX0AI9vdMp5/GGLNc7nOA7xNmsGHUq642Av3xk3gJlKJO
        d3Il6gbP+Dd8jLVaVcPHQY/d0qM1MDMwDgYDVR0PAQH/BAQDAgWgMBMGA1UdJQQM
        MAoGCCsGAQUFBwMBMAwGA1UdEwEB/wQCMAAwCgYIKoZIzj0EAwIDSAAwRQIhAKqc
        1sgVy/pkQzVK+qY3wL2jgvMFlNNrCNWT2utyFws7AiAVKaAxGN0cSMpx+sB/HD/X
        vYc0adnWNkSLc62dIXsA7Q==
        -----END CERTIFICATE-----
    to:
      kind: Service
      name: grpc-interop
      weight: 100
    wildcardPolicy: None
- apiVersion: route.openshift.io/v1
  kind: Route
  metadata:
    labels:
      app: grpc-interop
    name: grpc-interop-passthrough
  spec:
    port:
      targetPort: 8443
    tls:
      termination: passthrough
      insecureEdgeTerminationPolicy: Redirect
    to:
      kind: Service
      name: grpc-interop
      weight: 100
    wildcardPolicy: None
`,
		strings.Join(addPrefix(data, strings.Repeat(" ", 6)), "\n"))
}
