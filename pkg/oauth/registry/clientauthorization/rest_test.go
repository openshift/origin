package clientauthorization

import (
	"errors"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	oapi "github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/test"
)

func TestCreateValidationError(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}
	storage := REST{
		registry: registry,
	}
	clientAuth := &oapi.ClientAuthorization{
		ObjectMeta: api.ObjectMeta{Name: "authTokenName"},
		// ClientName: "clientName",// Missing required field
		UserName: "userName",
		UserUID:  "userUID",
	}

	ctx := api.NewContext()
	_, err := storage.Create(ctx, clientAuth)
	if err == nil {
		t.Errorf("Expected validation error")
	}
}

func TestCreateStorageError(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}
	registry.Err = errors.New("Sample Error")

	storage := REST{
		registry: registry,
	}
	clientAuth := &oapi.ClientAuthorization{
		ObjectMeta: api.ObjectMeta{Name: "clientName"},
		ClientName: "clientName",
		UserName:   "userName",
		UserUID:    "userUID",
	}

	ctx := api.NewContext()
	channel, err := storage.Create(ctx, clientAuth)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	select {
	case r := <-channel:
		switch r := r.Object.(type) {
		case *api.Status:
			if r.Message == registry.Err.Error() {
				// expected case
			} else {
				t.Errorf("Got back unexpected error: %#v", r)
			}
		default:
			t.Errorf("Got back non-status result: %v", r)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestCreateValid(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}
	storage := REST{
		registry: registry,
	}
	clientAuth := &oapi.ClientAuthorization{
		ObjectMeta: api.ObjectMeta{Name: "clientName"},
		ClientName: "clientName",
		UserName:   "userName",
		UserUID:    "userUID",
	}

	ctx := api.NewContext()
	channel, err := storage.Create(ctx, clientAuth)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	select {
	case r := <-channel:
		switch r := r.Object.(type) {
		case *api.Status:
			t.Errorf("Got back unexpected status: %#v", r)
		case *oapi.ClientAuthorization:
			// expected case
		default:
			t.Errorf("Got unexpected type: %#v", r)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestGetError(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}
	registry.Err = errors.New("Sample Error")
	storage := REST{
		registry: registry,
	}
	ctx := api.NewContext()
	_, err := storage.Get(ctx, "name")
	if err == nil {
		t.Errorf("expected error")
		return
	}
	if err != registry.Err {
		t.Errorf("got unexpected error: %v", err)
		return
	}
}

func TestGetValid(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}

	registry.ClientAuthorization = &oapi.ClientAuthorization{ObjectMeta: api.ObjectMeta{Name: "clientName"}}
	storage := REST{
		registry: registry,
	}
	ctx := api.NewContext()
	clientAuth, err := storage.Get(ctx, "name")
	if err != nil {
		t.Errorf("got unexpected error: %v", err)
		return
	}
	if clientAuth != registry.ClientAuthorization {
		t.Errorf("got unexpected clientAuthorization: %v", clientAuth)
		return
	}
}

func TestListError(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}

	registry.Err = errors.New("Sample Error")
	storage := REST{
		registry: registry,
	}
	ctx := api.NewContext()
	_, err := storage.List(ctx, labels.Everything(), labels.Everything())
	if err == nil {
		t.Errorf("expected error")
		return
	}
	if err != registry.Err {
		t.Errorf("got unexpected error: %v", err)
		return
	}
}

func TestListEmpty(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}

	registry.ClientAuthorizations = &oapi.ClientAuthorizationList{}
	storage := REST{
		registry: registry,
	}
	ctx := api.NewContext()
	clientAuths, err := storage.List(ctx, labels.Everything(), labels.Everything())
	if err != registry.Err {
		t.Errorf("got unexpected error: %v", err)
		return
	}
	switch clientAuths := clientAuths.(type) {
	case *oapi.ClientAuthorizationList:
		if len(clientAuths.Items) != 0 {
			t.Errorf("expected empty list, got %#v", clientAuths)
		}
	default:
		t.Errorf("expected clientAuthList, got: %v", clientAuths)
		return
	}
}

func TestList(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}

	registry.ClientAuthorizations = &oapi.ClientAuthorizationList{
		Items: []oapi.ClientAuthorization{
			{},
			{},
		},
	}
	storage := REST{
		registry: registry,
	}
	ctx := api.NewContext()
	clientAuths, err := storage.List(ctx, labels.Everything(), labels.Everything())
	if err != registry.Err {
		t.Errorf("got unexpected error: %v", err)
		return
	}
	switch clientAuths := clientAuths.(type) {
	case *oapi.ClientAuthorizationList:
		if len(clientAuths.Items) != 2 {
			t.Errorf("expected list with 2 items, got %#v", clientAuths)
		}
	default:
		t.Errorf("expected clientAuthList, got: %v", clientAuths)
		return
	}
}

func TestUpdateNotSupported(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}

	registry.Err = errors.New("Storage Error")
	storage := REST{
		registry: registry,
	}
	client := &oapi.ClientAuthorization{
		ObjectMeta: api.ObjectMeta{Name: "clientName"},
	}

	ctx := api.NewContext()
	_, err := storage.Update(ctx, client)
	if err == nil {
		t.Errorf("expected unsupported error, but update succeeded")
		return
	}
	if err == registry.Err {
		t.Errorf("expected unsupported error, but registry was called")
		return
	}
}

func TestDeleteError(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}

	registry.Err = errors.New("Sample Error")
	storage := REST{
		registry: registry,
	}

	ctx := api.NewContext()
	channel, err := storage.Delete(ctx, "foo")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	select {
	case r := <-channel:
		switch r := r.Object.(type) {
		case *api.Status:
			if r.Message == registry.Err.Error() {
				// expected case
			} else {
				t.Errorf("Got back unexpected error: %#v", r)
			}
		default:
			t.Errorf("Got back non-status result: %v", r)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}

func TestDeleteValid(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}

	registry.ClientAuthorization = &oapi.ClientAuthorization{ObjectMeta: api.ObjectMeta{Name: "foo"}}
	storage := REST{
		registry: registry,
	}

	ctx := api.NewContext()
	channel, err := storage.Delete(ctx, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case r := <-channel:
		switch r := r.Object.(type) {
		case *api.Status:
			if r.Status != "Success" {
				t.Fatalf("Got back non-success status: %#v", r)
			}
		default:
			t.Fatalf("Got back non-status result: %v", r)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}
}
