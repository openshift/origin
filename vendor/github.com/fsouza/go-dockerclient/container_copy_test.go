package docker

import (
	"bytes"
	"net/http"
	"testing"
)

func TestCopyFromContainer(t *testing.T) {
	t.Parallel()
	content := "File content"
	out := stdoutMock{bytes.NewBufferString(content)}
	client := newTestClient(&FakeRoundTripper{status: http.StatusOK})
	opts := CopyFromContainerOptions{
		Container:    "a123456",
		OutputStream: &out,
	}
	err := client.CopyFromContainer(opts)
	if err != nil {
		t.Errorf("CopyFromContainer: caught error %#v while copying from container, expected nil", err.Error())
	}
	if out.String() != content {
		t.Errorf("CopyFromContainer: wrong stdout. Want %#v. Got %#v.", content, out.String())
	}
}

func TestCopyFromContainerEmptyContainer(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{status: http.StatusOK})
	err := client.CopyFromContainer(CopyFromContainerOptions{})
	_, ok := err.(*NoSuchContainer)
	if !ok {
		t.Errorf("CopyFromContainer: invalid error returned. Want NoSuchContainer, got %#v.", err)
	}
}

func TestCopyFromContainerDockerAPI124(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{status: http.StatusOK})
	client.serverAPIVersion = apiVersion124
	opts := CopyFromContainerOptions{
		Container: "a123456",
	}
	err := client.CopyFromContainer(opts)
	if err == nil {
		t.Fatal("got unexpected <nil> error")
	}
	expectedMsg := "go-dockerclient: CopyFromContainer is no longer available in Docker >= 1.12, use DownloadFromContainer instead"
	if err.Error() != expectedMsg {
		t.Errorf("wrong error message\nWant %q\nGot  %q", expectedMsg, err.Error())
	}
}
