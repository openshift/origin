/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package client

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

type RequestInfo struct {
	RequestHeaders http.Header
	RequestVerb    string
	RequestURL     string
	RequestBody    []byte

	ResponseStatus  string
	ResponseBody    []byte
	ResponseHeaders http.Header
	ResponseErr     error

	Duration time.Duration
}

func NewRequestInfo(req *http.Request, readBody bool) *RequestInfo {
	reqInfo := &RequestInfo{}
	reqInfo.RequestURL = req.URL.String()
	reqInfo.RequestVerb = req.Method
	reqInfo.RequestHeaders = req.Header
	if readBody && req.Body != nil {
		if body, err := ioutil.ReadAll(req.Body); err == nil {
			reqInfo.RequestBody = body
			req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		}
	}

	return reqInfo
}

func (r *RequestInfo) Complete(response *http.Response, err error, readBody bool) {
	if err != nil {
		r.ResponseErr = err
		return
	}
	r.ResponseStatus = response.Status
	r.ResponseHeaders = response.Header
	if readBody && response.Body != nil {
		if body, err := ioutil.ReadAll(response.Body); err == nil {
			r.ResponseBody = body
			response.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		}
	}
}

func (r RequestInfo) ToCurl() string {
	headers := ""
	for key, values := range map[string][]string(r.RequestHeaders) {
		for _, value := range values {
			headers += fmt.Sprintf(` -H %q`, fmt.Sprintf("%s: %s", key, value))
		}
	}

	body := ""
	if len(r.RequestBody) > 0 {
		body = fmt.Sprintf("-d %q", string(body))
	}

	return fmt.Sprintf("curl -k -v -X%s %s %s %s", r.RequestVerb, headers, body, r.RequestURL)
}

// TrackingRoundTripper keeps track of all the requests made.  You should use this with caution, because it grow with every request.
type TrackingRoundTripper struct {
	delegatedRoundTripper http.RoundTripper

	RequestInfos []RequestInfo
}

func NewTrackingRoundTripper(rt http.RoundTripper) *TrackingRoundTripper {
	return &TrackingRoundTripper{rt, []RequestInfo{}}
}

func (rt *TrackingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	reqInfo := NewRequestInfo(req, bool(glog.V(7)))

	startTime := time.Now()
	response, err := rt.delegatedRoundTripper.RoundTrip(req)
	reqInfo.Duration = time.Since(startTime)

	reqInfo.Complete(response, err, bool(glog.V(7)))
	rt.RequestInfos = append(rt.RequestInfos, *reqInfo)

	return response, err
}

// DebuggingRoundTripper will display information about the requests passing through it based on what is configured
type DebuggingRoundTripper struct {
	delegatedRoundTripper http.RoundTripper

	Levels util.StringSet
}

const (
	JustURL         string = "url"
	URLTiming       string = "urltiming"
	CurlCommand     string = "curlcommand"
	RequestBody     string = "requestbody"
	RequestHeaders  string = "requestheaders"
	ResponseStatus  string = "responsestatus"
	ResponseBody    string = "responsebody"
	ResponseHeaders string = "responseheaders"
)

func NewDebuggingRoundTripper(rt http.RoundTripper, levels ...string) *DebuggingRoundTripper {
	return &DebuggingRoundTripper{rt, util.NewStringSet(levels...)}
}

func (rt *DebuggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	reqInfo := NewRequestInfo(req, rt.Levels.Has(RequestBody))

	if rt.Levels.Has(JustURL) {
		glog.Infof("%s %s", reqInfo.RequestVerb, reqInfo.RequestURL)
	}
	if rt.Levels.Has(CurlCommand) {
		glog.Infof("%s", reqInfo.ToCurl())

	}
	if rt.Levels.Has(RequestBody) {
		glog.Infof("Request Body:\n%s", string(reqInfo.RequestBody))
	}
	if rt.Levels.Has(RequestHeaders) {
		glog.Infof("Request Headers:")
		for key, values := range reqInfo.RequestHeaders {
			for _, value := range values {
				glog.Infof("    %s: %s", key, value)
			}
		}
	}

	startTime := time.Now()
	response, err := rt.delegatedRoundTripper.RoundTrip(req)
	reqInfo.Duration = time.Since(startTime)

	reqInfo.Complete(response, err, rt.Levels.Has(ResponseBody))

	if rt.Levels.Has(URLTiming) {
		glog.Infof("%s %s %s in %d milliseconds", reqInfo.RequestVerb, reqInfo.RequestURL, reqInfo.ResponseStatus, reqInfo.Duration.Nanoseconds()/int64(time.Millisecond))
	}
	if rt.Levels.Has(ResponseStatus) {
		glog.Infof("Response Status: %s in %d milliseconds", reqInfo.ResponseStatus, reqInfo.Duration.Nanoseconds()/int64(time.Millisecond))
	}
	if rt.Levels.Has(ResponseHeaders) {
		glog.Infof("Response Headers:")
		for key, values := range reqInfo.ResponseHeaders {
			for _, value := range values {
				glog.Infof("    %s: %s", key, value)
			}
		}
	}
	if rt.Levels.Has(ResponseBody) {
		glog.V(7).Infof("Response Body:\n%s", string(reqInfo.ResponseBody))
	}

	return response, err
}

type userAgentRoundTripper struct {
	agent string
	rt    http.RoundTripper
}

func NewUserAgentRoundTripper(agent string, rt http.RoundTripper) http.RoundTripper {
	return &userAgentRoundTripper{agent, rt}
}

func (rt *userAgentRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(req.Header.Get("User-Agent")) != 0 {
		return rt.rt.RoundTrip(req)
	}
	req = cloneRequest(req)
	req.Header.Set("User-Agent", rt.agent)
	return rt.rt.RoundTrip(req)
}

type basicAuthRoundTripper struct {
	username string
	password string
	rt       http.RoundTripper
}

func NewBasicAuthRoundTripper(username, password string, rt http.RoundTripper) http.RoundTripper {
	return &basicAuthRoundTripper{username, password, rt}
}

func (rt *basicAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = cloneRequest(req)
	req.SetBasicAuth(rt.username, rt.password)
	return rt.rt.RoundTrip(req)
}

type bearerAuthRoundTripper struct {
	bearer string
	rt     http.RoundTripper
}

func NewBearerAuthRoundTripper(bearer string, rt http.RoundTripper) http.RoundTripper {
	return &bearerAuthRoundTripper{bearer, rt}
}

func (rt *bearerAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = cloneRequest(req)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", rt.bearer))
	return rt.rt.RoundTrip(req)
}

// TLSConfigFor returns a tls.Config that will provide the transport level security defined
// by the provided Config. Will return nil if no transport level security is requested.
func TLSConfigFor(config *Config) (*tls.Config, error) {
	hasCA := len(config.CAFile) > 0 || len(config.CAData) > 0
	hasCert := len(config.CertFile) > 0 || len(config.CertData) > 0

	if hasCA && config.Insecure {
		return nil, fmt.Errorf("specifying a root certificates file with the insecure flag is not allowed")
	}
	if err := LoadTLSFiles(config); err != nil {
		return nil, err
	}
	var tlsConfig *tls.Config
	switch {
	case hasCert:
		cfg, err := NewClientCertTLSConfig(config.CertData, config.KeyData, config.CAData)
		if err != nil {
			return nil, err
		}
		tlsConfig = cfg
	case hasCA:
		cfg, err := NewTLSConfig(config.CAData)
		if err != nil {
			return nil, err
		}
		tlsConfig = cfg
	case config.Insecure:
		tlsConfig = NewUnsafeTLSConfig()
	}

	return tlsConfig, nil
}

// LoadTLSFiles copies the data from the CertFile, KeyFile, and CAFile fields into the CertData,
// KeyData, and CAFile fields, or returns an error. If no error is returned, all three fields are
// either populated or were empty to start.
func LoadTLSFiles(config *Config) error {
	certData, err := dataFromSliceOrFile(config.CertData, config.CertFile)
	if err != nil {
		return err
	}
	config.CertData = certData
	keyData, err := dataFromSliceOrFile(config.KeyData, config.KeyFile)
	if err != nil {
		return err
	}
	config.KeyData = keyData
	caData, err := dataFromSliceOrFile(config.CAData, config.CAFile)
	if err != nil {
		return err
	}
	config.CAData = caData

	return nil
}

// dataFromSliceOrFile returns data from the slice (if non-empty), or from the file,
// or an error if an error occurred reading the file
func dataFromSliceOrFile(data []byte, file string) ([]byte, error) {
	if len(data) > 0 {
		return data, nil
	}
	if len(file) > 0 {
		fileData, err := ioutil.ReadFile(file)
		if err != nil {
			return []byte{}, err
		}
		return fileData, nil
	}
	return nil, nil
}

func NewClientCertTLSConfig(certData, keyData, caData []byte) (*tls.Config, error) {
	cert, err := tls.X509KeyPair(certData, keyData)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caData)
	return &tls.Config{
		// Change default from SSLv3 to TLSv1.0 (because of POODLE vulnerability)
		MinVersion: tls.VersionTLS10,
		Certificates: []tls.Certificate{
			cert,
		},
		RootCAs:    certPool,
		ClientCAs:  certPool,
		ClientAuth: tls.RequireAndVerifyClientCert,
	}, nil
}

func NewTLSConfig(caData []byte) (*tls.Config, error) {
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caData)
	return &tls.Config{
		// Change default from SSLv3 to TLSv1.0 (because of POODLE vulnerability)
		MinVersion: tls.VersionTLS10,
		RootCAs:    certPool,
	}, nil
}

func NewUnsafeTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
	}
}

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
func cloneRequest(r *http.Request) *http.Request {
	// shallow copy of the struct
	r2 := new(http.Request)
	*r2 = *r
	// deep copy of the Header
	r2.Header = make(http.Header)
	for k, s := range r.Header {
		r2.Header[k] = s
	}
	return r2
}
