package util

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// CallbackInvoker posts results to a callback URL when a STI build is done.
type CallbackInvoker interface {
	ExecuteCallback(callbackURL string, success bool, labels map[string]string, messages []string) []string
}

// NewCallbackInvoker creates an instance of the default CallbackInvoker implementation
func NewCallbackInvoker() CallbackInvoker {
	invoker := &callbackInvoker{}
	invoker.postFunc = invoker.httpPost
	return invoker
}

type callbackInvoker struct {
	postFunc func(url, contentType string, body io.Reader) (resp *http.Response, err error)
}

// ExecuteCallback prepares a JSON payload and posts it to the specified callback URL
func (c *callbackInvoker) ExecuteCallback(callbackURL string, success bool, labels map[string]string, messages []string) []string {
	buf := new(bytes.Buffer)
	writer := bufio.NewWriter(buf)

	data := map[string]interface{}{
		"success": success,
	}

	if len(labels) > 0 {
		data["labels"] = labels
	}

	jsonBuffer := new(bytes.Buffer)
	writer = bufio.NewWriter(jsonBuffer)
	jsonWriter := json.NewEncoder(writer)
	jsonWriter.Encode(data)
	writer.Flush()

	var (
		resp *http.Response
		err  error
	)
	for retries := 0; retries < 3; retries++ {
		resp, err = c.postFunc(callbackURL, "application/json", jsonBuffer)
		if err != nil {
			errorMessage := fmt.Sprintf("Unable to invoke callback: %v", err)
			messages = append(messages, errorMessage)
		}
		if resp != nil {
			if resp.StatusCode >= 300 {
				errorMessage := fmt.Sprintf("Callback returned with error code: %d", resp.StatusCode)
				messages = append(messages, errorMessage)
			}
			break
		}
	}
	return messages
}

func (*callbackInvoker) httpPost(url, contentType string, body io.Reader) (resp *http.Response, err error) {
	return http.Post(url, contentType, body)
}
