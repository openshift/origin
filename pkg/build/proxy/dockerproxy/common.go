package dockerproxy

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
)

func unmarshalRequestBody(r *http.Request, target interface{}) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	glog.V(6).Infof("->requestBody: %s", body)
	if err := r.Body.Close(); err != nil {
		return err
	}
	r.Body = ioutil.NopCloser(bytes.NewReader(body))

	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber() // don't want large numbers in scientific format
	return d.Decode(&target)
}

func marshalRequestBody(r *http.Request, body interface{}) error {
	newBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	glog.V(6).Infof("<-requestBody: %s", newBody)
	r.Body = ioutil.NopCloser(bytes.NewReader(newBody))
	r.ContentLength = int64(len(newBody))
	return nil
}

func unmarshalResponseBody(r *http.Response, target interface{}) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	glog.V(6).Infof("->responseBody: %s", body)
	if err := r.Body.Close(); err != nil {
		return err
	}
	r.Body = ioutil.NopCloser(bytes.NewReader(body))

	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber() // don't want large numbers in scientific format
	return d.Decode(&target)
}

func marshalResponseBody(r *http.Response, body interface{}) error {
	newBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	glog.V(6).Infof("<-responseBody: %s", newBody)
	r.Body = ioutil.NopCloser(bytes.NewReader(newBody))
	r.ContentLength = int64(len(newBody))
	// Stop it being chunked, because that hangs
	r.TransferEncoding = nil
	return nil
}
