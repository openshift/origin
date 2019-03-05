package assets

import (
	"bytes"
	"encoding/base64"
	"strings"
	"text/template"
	"time"

	"k8s.io/client-go/util/cert"
)

var templateFuncs = map[string]interface{}{
	"notAfter":  notAfter,
	"notBefore": notBefore,
	"issuer":    issuer,
	"base64":    base64encode,
	"indent":    indent,
	"load":      load,
}

func indent(indention int, v []byte) string {
	newline := "\n" + strings.Repeat(" ", indention)
	return strings.Replace(string(v), "\n", newline, -1)
}

func base64encode(v []byte) string {
	return base64.StdEncoding.EncodeToString(v)
}

func notAfter(certBytes []byte) string {
	if len(certBytes) == 0 {
		return ""
	}
	certs, err := cert.ParseCertsPEM(certBytes)
	if err != nil {
		panic(err)
	}
	return certs[0].NotAfter.Format(time.RFC3339)
}

func notBefore(certBytes []byte) string {
	if len(certBytes) == 0 {
		return ""
	}
	certs, err := cert.ParseCertsPEM(certBytes)
	if err != nil {
		panic(err)
	}
	return certs[0].NotBefore.Format(time.RFC3339)
}

func issuer(certBytes []byte) string {
	if len(certBytes) == 0 {
		return ""
	}
	certs, err := cert.ParseCertsPEM(certBytes)
	if err != nil {
		panic(err)
	}
	return certs[0].Issuer.CommonName
}

func load(n string, assets map[string][]byte) []byte {
	return assets[n]
}

func renderFile(name string, tb []byte, data interface{}) ([]byte, error) {
	tmpl, err := template.New(name).Funcs(templateFuncs).Parse(string(tb))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
