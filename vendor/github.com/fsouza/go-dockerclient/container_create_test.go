package docker

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"testing"
)

func TestCreateContainer(t *testing.T) {
	t.Parallel()
	jsonContainer := `{
             "Id": "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2",
	     "Warnings": []
}`
	var expected Container
	err := json.Unmarshal([]byte(jsonContainer), &expected)
	if err != nil {
		t.Fatal(err)
	}
	fakeRT := &FakeRoundTripper{message: jsonContainer, status: http.StatusOK}
	client := newTestClient(fakeRT)
	config := Config{AttachStdout: true, AttachStdin: true}
	opts := CreateContainerOptions{Name: "TestCreateContainer", Config: &config}
	container, err := client.CreateContainer(opts)
	if err != nil {
		t.Fatal(err)
	}
	id := "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2"
	if container.ID != id {
		t.Errorf("CreateContainer: wrong ID. Want %q. Got %q.", id, container.ID)
	}
	req := fakeRT.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("CreateContainer: wrong HTTP method. Want %q. Got %q.", http.MethodPost, req.Method)
	}
	expectedURL, _ := url.Parse(client.getURL("/containers/create"))
	if gotPath := req.URL.Path; gotPath != expectedURL.Path {
		t.Errorf("CreateContainer: Wrong path in request. Want %q. Got %q.", expectedURL.Path, gotPath)
	}
	var gotBody Config
	err = json.NewDecoder(req.Body).Decode(&gotBody)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateContainerImageNotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "No such image: whatever", status: http.StatusNotFound})
	config := Config{AttachStdout: true, AttachStdin: true}
	container, err := client.CreateContainer(CreateContainerOptions{Config: &config})
	if container != nil {
		t.Errorf("CreateContainer: expected <nil> container, got %#v.", container)
	}
	if !errors.Is(err, ErrNoSuchImage) {
		t.Errorf("CreateContainer: Wrong error type. Want %#v. Got %#v.", ErrNoSuchImage, err)
	}
}

func TestCreateContainerDuplicateName(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: "No such image", status: http.StatusConflict})
	config := Config{AttachStdout: true, AttachStdin: true}
	container, err := client.CreateContainer(CreateContainerOptions{Config: &config})
	if container != nil {
		t.Errorf("CreateContainer: expected <nil> container, got %#v.", container)
	}
	if !errors.Is(err, ErrContainerAlreadyExists) {
		t.Errorf("CreateContainer: Wrong error type. Want %#v. Got %#v.", ErrContainerAlreadyExists, err)
	}
}

// Workaround for 17.09 bug returning 400 instead of 409.
// See https://github.com/moby/moby/issues/35021
func TestCreateContainerDuplicateNameWorkaroundDocker17_09(t *testing.T) {
	t.Parallel()
	client := newTestClient(&FakeRoundTripper{message: `{"message":"Conflict. The container name \"/c1\" is already in use by container \"2ce137e165dfca5e087f247b5d05a2311f91ef3da4bb7772168446a1a47e2f68\". You have to remove (or rename) that container to be able to reuse that name."}`, status: http.StatusBadRequest})
	config := Config{AttachStdout: true, AttachStdin: true}
	container, err := client.CreateContainer(CreateContainerOptions{Config: &config})
	if container != nil {
		t.Errorf("CreateContainer: expected <nil> container, got %#v.", container)
	}
	if !errors.Is(err, ErrContainerAlreadyExists) {
		t.Errorf("CreateContainer: Wrong error type. Want %#v. Got %#v.", ErrContainerAlreadyExists, err)
	}
}

func TestCreateContainerWithHostConfig(t *testing.T) {
	t.Parallel()
	fakeRT := &FakeRoundTripper{message: "{}", status: http.StatusOK}
	client := newTestClient(fakeRT)
	config := Config{}
	hostConfig := HostConfig{PublishAllPorts: true}
	opts := CreateContainerOptions{Name: "TestCreateContainerWithHostConfig", Config: &config, HostConfig: &hostConfig}
	_, err := client.CreateContainer(opts)
	if err != nil {
		t.Fatal(err)
	}
	req := fakeRT.requests[0]
	var gotBody map[string]interface{}
	err = json.NewDecoder(req.Body).Decode(&gotBody)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := gotBody["HostConfig"]; !ok {
		t.Errorf("CreateContainer: wrong body. HostConfig was not serialized")
	}
}

func TestPassingNameOptToCreateContainerReturnsItInContainer(t *testing.T) {
	t.Parallel()
	jsonContainer := `{
             "Id": "4fa6e0f0c6786287e131c3852c58a2e01cc697a68231826813597e4994f1d6e2",
	     "Warnings": []
}`
	fakeRT := &FakeRoundTripper{message: jsonContainer, status: http.StatusOK}
	client := newTestClient(fakeRT)
	config := Config{AttachStdout: true, AttachStdin: true}
	opts := CreateContainerOptions{Name: "TestCreateContainer", Config: &config}
	container, err := client.CreateContainer(opts)
	if err != nil {
		t.Fatal(err)
	}
	if container.Name != "TestCreateContainer" {
		t.Errorf("Container name expected to be TestCreateContainer, was %s", container.Name)
	}
}
