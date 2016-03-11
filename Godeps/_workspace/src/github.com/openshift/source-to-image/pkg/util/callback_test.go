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

	labels := map[string]string{
		"foo": "bar",
	}
	cb.ExecuteCallback("http://the.callback.url/test", true, labels, []string{"msg1", "msg2"})

	type postBody struct {
		Labels  map[string]string
		Payload string
		Success bool
	}
	var pb postBody
	json.Unmarshal(fp.body, &pb)
	if pb.Payload != "msg1\nmsg2\n" {
		t.Errorf("Unexpected payload: %s", pb.Payload)
	}
	if len(pb.Labels) == 0 {
		t.Errorf("Expected labels to be present in payload")
	}
	if pb.Labels["foo"] != "bar" {
		t.Errorf("Expected 'foo' to be 'bar', got %q", pb.Labels["foo"])
	}
	if pb.Success != true {
		t.Errorf("Unexpected success flag: %v", pb.Success)
	}
}
