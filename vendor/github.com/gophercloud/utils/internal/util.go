package internal

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"

	"github.com/mitchellh/go-homedir"
)

// RemainingKeys will inspect a struct and compare it to a map. Any struct
// field that does not have a JSON tag that matches a key in the map or
// a matching lower-case field in the map will be returned as an extra.
//
// This is useful for determining the extra fields returned in response bodies
// for resources that can contain an arbitrary or dynamic number of fields.
func RemainingKeys(s interface{}, m map[string]interface{}) (extras map[string]interface{}) {
	extras = make(map[string]interface{})
	for k, v := range m {
		extras[k] = v
	}

	valueOf := reflect.ValueOf(s)
	typeOf := reflect.TypeOf(s)
	for i := 0; i < valueOf.NumField(); i++ {
		field := typeOf.Field(i)

		lowerField := strings.ToLower(field.Name)
		delete(extras, lowerField)

		if tagValue := field.Tag.Get("json"); tagValue != "" && tagValue != "-" {
			delete(extras, tagValue)
		}
	}

	return
}

// PrepareTLSConfig generates TLS config based on the specifed parameters
func PrepareTLSConfig(caCertFile, clientCertFile, clientKeyFile string, insecure *bool) (*tls.Config, error) {
	config := &tls.Config{}
	if caCertFile != "" {
		caCert, _, err := pathOrContents(caCertFile)
		if err != nil {
			return nil, fmt.Errorf("Error reading CA Cert: %s", err)
		}

		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(bytes.TrimSpace(caCert)); !ok {
			return nil, fmt.Errorf("Error parsing CA Cert from %s", caCertFile)
		}
		config.RootCAs = caCertPool
	}

	if insecure == nil {
		config.InsecureSkipVerify = false
	} else {
		config.InsecureSkipVerify = *insecure
	}

	if clientCertFile != "" && clientKeyFile != "" {
		clientCert, _, err := pathOrContents(clientCertFile)
		if err != nil {
			return nil, fmt.Errorf("Error reading Client Cert: %s", err)
		}
		clientKey, _, err := pathOrContents(clientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("Error reading Client Key: %s", err)
		}

		cert, err := tls.X509KeyPair(clientCert, clientKey)
		if err != nil {
			return nil, err
		}

		config.Certificates = []tls.Certificate{cert}
		config.BuildNameToCertificate()
	}

	return config, nil
}

func pathOrContents(poc string) ([]byte, bool, error) {
	if len(poc) == 0 {
		return nil, false, nil
	}

	path := poc
	if path[0] == '~' {
		var err error
		path, err = homedir.Expand(path)
		if err != nil {
			return []byte(path), true, err
		}
	}

	if _, err := os.Stat(path); err == nil {
		contents, err := ioutil.ReadFile(path)
		if err != nil {
			return contents, true, err
		}
		return contents, true, nil
	}

	return []byte(poc), false, nil
}
