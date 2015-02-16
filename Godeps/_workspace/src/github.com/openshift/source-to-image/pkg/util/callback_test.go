package util

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"testing"
)

type FakePost struct {
	err      error
	response http.Response
	body     []byte
	url      string
}

func (f *FakePost) post(url, contentType string, body io.Reader) (resp *http.Response, err error) {
	f.url = url
	f.body, _ = ioutil.ReadAll(body)
	return &f.response, f.err
}

func TestExecuteCallback(t *testing.T) {
	fp := FakePost{}
	cb := callbackInvoker{
		postFunc: fp.post,
	}

	cb.ExecuteCallback("http://the.callback.url/test", true, []string{"msg1", "msg2"})

	type postBody struct {
		Payload string
		Success bool
	}
	var pb postBody
	json.Unmarshal(fp.body, &pb)
	if pb.Payload != "msg1\nmsg2\n" {
		t.Errorf("Unexpected payload: %s", pb.Payload)
	}
	if pb.Success != true {
		t.Errorf("Unexpected success flag: %v", pb.Success)
	}
}
