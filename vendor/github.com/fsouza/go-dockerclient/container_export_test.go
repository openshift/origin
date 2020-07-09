package docker

import (
	"bytes"
	"net/http"
	"testing"
)

func TestExportContainer(t *testing.T) {
	t.Parallel()
	content := "exported container tar content"
	out := stdoutMock{bytes.NewBufferString(content)}
	client := newTestClient(&FakeRoundTripper{status: http.StatusOK})
	opts := ExportContainerOptions{ID: "4fa6e0f0c678", OutputStream: out}
	err := client.ExportContainer(opts)
	if err != nil {
		t.Errorf("ExportContainer: caugh error %#v while exporting container, expected nil", err.Error())
	}
	if out.String() != content {
		t.Errorf("ExportContainer: wrong stdout. Want %#v. Got %#v.", content, out.String())
	}
}

func TestExportContainerNoId(t *testing.T) {
	t.Parallel()
	client := Client{}
	out := stdoutMock{bytes.NewBufferString("")}
	err := client.ExportContainer(ExportContainerOptions{OutputStream: out})
	e, ok := err.(*NoSuchContainer)
	if !ok {
		t.Errorf("ExportContainer: wrong error. Want NoSuchContainer. Got %#v.", e)
	}
	if e.ID != "" {
		t.Errorf("ExportContainer: wrong ID. Want %q. Got %q", "", e.ID)
	}
}
