package docker

import (
	"bytes"
	"net/http"
	"testing"
)

func TestUploadToContainer(t *testing.T) {
	t.Parallel()
	content := "File content"
	in := stdinMock{bytes.NewBufferString(content)}
	fakeRT := &FakeRoundTripper{status: http.StatusOK}
	client := newTestClient(fakeRT)
	opts := UploadToContainerOptions{
		Path:        "abc",
		InputStream: in,
	}
	err := client.UploadToContainer("a123456", opts)
	if err != nil {
		t.Errorf("UploadToContainer: caught error %#v while uploading archive to container, expected nil", err)
	}

	req := fakeRT.requests[0]

	if req.Method != http.MethodPut {
		t.Errorf("UploadToContainer{Path:abc}: Wrong HTTP method.  Want PUT. Got %s", req.Method)
	}

	if pathParam := req.URL.Query().Get("path"); pathParam != "abc" {
		t.Errorf("ListImages({Path:abc}): Wrong parameter. Want path=abc.  Got path=%s", pathParam)
	}
}

func TestDownloadFromContainer(t *testing.T) {
	t.Parallel()
	filecontent := "File content"
	client := newTestClient(&FakeRoundTripper{message: filecontent, status: http.StatusOK})

	var out bytes.Buffer
	opts := DownloadFromContainerOptions{
		OutputStream: &out,
	}
	err := client.DownloadFromContainer("a123456", opts)
	if err != nil {
		t.Errorf("DownloadFromContainer: caught error %#v while downloading from container, expected nil", err.Error())
	}
	if out.String() != filecontent {
		t.Errorf("DownloadFromContainer: wrong stdout. Want %#v. Got %#v.", filecontent, out.String())
	}
}
