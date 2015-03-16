package clientcmd

import (
	"net/http"

	"github.com/golang/glog"
)

const (
	unauthorizedErrorMessage = `Your session has expired. Use the following command to log in again:
  osc login`
)

type statusHandlerClient struct {
	delegate *http.Client
}

func (client *statusHandlerClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := client.delegate.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		glog.Fatal(unauthorizedErrorMessage)
	}

	return resp, err
}
