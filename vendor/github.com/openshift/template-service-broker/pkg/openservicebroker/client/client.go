package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/net/context"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/authentication/user"

	openservicebrokerapi "github.com/openshift/template-service-broker/pkg/openservicebroker/api"
)

type Client interface {
	Client() *http.Client

	Catalog(ctx context.Context) (*openservicebrokerapi.CatalogResponse, error)
	Provision(ctx context.Context, u user.Info, instanceID string, preq *openservicebrokerapi.ProvisionRequest) (*openservicebrokerapi.ProvisionResponse, error)
	Deprovision(ctx context.Context, u user.Info, instanceID string) error
	Bind(ctx context.Context, u user.Info, instanceID, bindingID string, breq *openservicebrokerapi.BindRequest) (*openservicebrokerapi.BindResponse, error)
	Unbind(ctx context.Context, u user.Info, instanceID, bindingID string) error
}

type client struct {
	cli  *http.Client
	root string
}

func NewClient(cli *http.Client, root string) Client {
	return &client{cli: cli, root: root}
}

type ServerError struct {
	StatusCode  int
	Description string
}

func (e *ServerError) Error() string {
	return fmt.Sprintf("%s: %s", http.StatusText(e.StatusCode), e.Description)
}

func newServerError(statusCode int, description string) error {
	return &ServerError{StatusCode: statusCode, Description: description}
}

func (c *client) Client() *http.Client {
	return c.cli
}

func OriginatingIdentityHeader(u user.Info) (string, error) {
	templatereq := openservicebrokerapi.ConvertUserToTemplateInstanceRequester(u)

	b, err := json.Marshal(&templatereq)
	if err != nil {
		return "", err
	}
	encodeVal := base64.StdEncoding.EncodeToString(b)
	return openservicebrokerapi.OriginatingIdentitySchemeKubernetes + " " + encodeVal, nil
}

func (c *client) Catalog(ctx context.Context) (*openservicebrokerapi.CatalogResponse, error) {
	req, err := http.NewRequest(http.MethodGet, c.root+"/v2/catalog", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add(openservicebrokerapi.XBrokerAPIVersion, openservicebrokerapi.APIVersion)

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "application/json" {
		return nil, newServerError(resp.StatusCode, "invalid content type")
	}

	d := json.NewDecoder(resp.Body)
	if resp.StatusCode == http.StatusOK {
		var r *openservicebrokerapi.CatalogResponse
		err = d.Decode(&r)
		if err != nil {
			return nil, err
		}
		return r, nil
	}

	var r *openservicebrokerapi.ErrorResponse
	err = d.Decode(&r)
	if err != nil {
		return nil, err
	}
	return nil, newServerError(resp.StatusCode, r.Description)
}

func (c *client) Provision(ctx context.Context, u user.Info, instanceID string, preq *openservicebrokerapi.ProvisionRequest) (*openservicebrokerapi.ProvisionResponse, error) {
	if errs := openservicebrokerapi.ValidateUUID(field.NewPath("instanceID"), instanceID); len(errs) > 0 {
		return nil, errs.ToAggregate()
	}

	pr, pw := io.Pipe()
	go func() {
		e := json.NewEncoder(pw)
		pw.CloseWithError(e.Encode(preq))
	}()

	req, err := http.NewRequest(http.MethodPut, c.root+"/v2/service_instances/"+instanceID+"?accepts_incomplete=true", pr)
	if err != nil {
		return nil, err
	}

	req.Header.Add(openservicebrokerapi.XBrokerAPIVersion, openservicebrokerapi.APIVersion)
	req.Header.Add("Content-Type", "application/json")

	identity, err := OriginatingIdentityHeader(u)
	if err != nil {
		return nil, err
	}
	req.Header.Add(openservicebrokerapi.XBrokerAPIOriginatingIdentity, identity)

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "application/json" {
		return nil, newServerError(resp.StatusCode, "invalid content type")
	}

	d := json.NewDecoder(resp.Body)
	if resp.StatusCode == http.StatusCreated ||
		resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusAccepted {
		var r *openservicebrokerapi.ProvisionResponse
		err = d.Decode(&r)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == http.StatusAccepted {
			var state openservicebrokerapi.LastOperationState
			state, err = c.WaitForOperation(ctx, u, instanceID, r.Operation)
			if err != nil {
				return nil, err
			}
			if state != openservicebrokerapi.LastOperationStateSucceeded {
				return nil, fmt.Errorf("operation returned state %s", string(state))
			}
		}

		return r, nil
	}

	var r *openservicebrokerapi.ErrorResponse
	err = d.Decode(&r)
	if err != nil {
		return nil, err
	}
	return nil, newServerError(resp.StatusCode, r.Description)
}

func (c *client) Deprovision(ctx context.Context, u user.Info, instanceID string) error {
	if errs := openservicebrokerapi.ValidateUUID(field.NewPath("instanceID"), instanceID); len(errs) > 0 {
		return errs.ToAggregate()
	}

	req, err := http.NewRequest(http.MethodDelete, c.root+"/v2/service_instances/"+instanceID+"?accepts_incomplete=true", nil)
	if err != nil {
		return err
	}

	req.Header.Add(openservicebrokerapi.XBrokerAPIVersion, openservicebrokerapi.APIVersion)

	identity, err := OriginatingIdentityHeader(u)
	if err != nil {
		return err
	}
	req.Header.Add(openservicebrokerapi.XBrokerAPIOriginatingIdentity, identity)

	resp, err := c.cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "application/json" {
		return newServerError(resp.StatusCode, "invalid content type")
	}

	d := json.NewDecoder(resp.Body)
	if resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusAccepted ||
		resp.StatusCode == http.StatusGone {
		var r *openservicebrokerapi.DeprovisionResponse
		err = d.Decode(&r)
		if err != nil {
			return err
		}

		if resp.StatusCode == http.StatusAccepted {
			var state openservicebrokerapi.LastOperationState
			state, err = c.WaitForOperation(ctx, u, instanceID, r.Operation)
			if err != nil {
				return err
			}
			if state != openservicebrokerapi.LastOperationStateSucceeded {
				return fmt.Errorf("operation returned state %s", string(state))
			}
		}

		return nil
	}

	var r *openservicebrokerapi.ErrorResponse
	err = d.Decode(&r)
	if err != nil {
		return err
	}
	return newServerError(resp.StatusCode, r.Description)
}

func (c *client) LastOperation(ctx context.Context, u user.Info, instanceID string, operation openservicebrokerapi.Operation) (*openservicebrokerapi.LastOperationResponse, error) {
	if errs := openservicebrokerapi.ValidateUUID(field.NewPath("instanceID"), instanceID); len(errs) > 0 {
		return nil, errs.ToAggregate()
	}

	req, err := http.NewRequest(http.MethodGet, c.root+"/v2/service_instances/"+instanceID+"/last_operation?operation="+string(operation), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add(openservicebrokerapi.XBrokerAPIVersion, openservicebrokerapi.APIVersion)

	identity, err := OriginatingIdentityHeader(u)
	if err != nil {
		return nil, err
	}
	req.Header.Add(openservicebrokerapi.XBrokerAPIOriginatingIdentity, identity)

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "application/json" {
		return nil, newServerError(resp.StatusCode, "invalid content type")
	}

	d := json.NewDecoder(resp.Body)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusGone {
		var r *openservicebrokerapi.LastOperationResponse
		err = d.Decode(&r)
		if err != nil {
			return nil, err
		}
		return r, nil
	}

	var r *openservicebrokerapi.ErrorResponse
	err = d.Decode(&r)
	if err != nil {
		return nil, err
	}
	return nil, newServerError(resp.StatusCode, r.Description)
}

func (c *client) WaitForOperation(ctx context.Context, u user.Info, instanceID string, operation openservicebrokerapi.Operation) (openservicebrokerapi.LastOperationState, error) {
	done := ctx.Done()
	for {
		op, err := c.LastOperation(ctx, u, instanceID, operation)
		if err != nil {
			return openservicebrokerapi.LastOperationStateFailed, err
		}

		if op.State != openservicebrokerapi.LastOperationStateInProgress {
			return op.State, nil
		}

		select {
		case <-done:
			return openservicebrokerapi.LastOperationStateFailed, ctx.Err()
		default:
		}

		time.Sleep(1 * time.Second)
	}
}

func (c *client) Bind(ctx context.Context, u user.Info, instanceID, bindingID string, breq *openservicebrokerapi.BindRequest) (*openservicebrokerapi.BindResponse, error) {
	if errs := openservicebrokerapi.ValidateUUID(field.NewPath("instanceID"), instanceID); len(errs) > 0 {
		return nil, errs.ToAggregate()
	}

	if errs := openservicebrokerapi.ValidateUUID(field.NewPath("bindingID"), bindingID); len(errs) > 0 {
		return nil, errs.ToAggregate()
	}

	pr, pw := io.Pipe()
	go func() {
		e := json.NewEncoder(pw)
		pw.CloseWithError(e.Encode(breq))
	}()

	req, err := http.NewRequest(http.MethodPut, c.root+"/v2/service_instances/"+instanceID+"/service_bindings/"+bindingID, pr)
	if err != nil {
		return nil, err
	}

	req.Header.Add(openservicebrokerapi.XBrokerAPIVersion, openservicebrokerapi.APIVersion)
	req.Header.Add("Content-Type", "application/json")

	identity, err := OriginatingIdentityHeader(u)
	if err != nil {
		return nil, err
	}
	req.Header.Add(openservicebrokerapi.XBrokerAPIOriginatingIdentity, identity)

	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "application/json" {
		return nil, newServerError(resp.StatusCode, "invalid content type")
	}

	d := json.NewDecoder(resp.Body)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		var r *openservicebrokerapi.BindResponse
		err = d.Decode(&r)
		if err != nil {
			return nil, err
		}
		return r, nil
	}

	var r *openservicebrokerapi.ErrorResponse
	err = d.Decode(&r)
	if err != nil {
		return nil, err
	}
	return nil, newServerError(resp.StatusCode, r.Description)
}

func (c *client) Unbind(ctx context.Context, u user.Info, instanceID, bindingID string) error {
	if errs := openservicebrokerapi.ValidateUUID(field.NewPath("instanceID"), instanceID); len(errs) > 0 {
		return errs.ToAggregate()
	}

	if errs := openservicebrokerapi.ValidateUUID(field.NewPath("bindingID"), bindingID); len(errs) > 0 {
		return errs.ToAggregate()
	}

	req, err := http.NewRequest(http.MethodDelete, c.root+"/v2/service_instances/"+instanceID+"/service_bindings/"+bindingID, nil)
	if err != nil {
		return err
	}

	req.Header.Add(openservicebrokerapi.XBrokerAPIVersion, openservicebrokerapi.APIVersion)

	identity, err := OriginatingIdentityHeader(u)
	if err != nil {
		return err
	}
	req.Header.Add(openservicebrokerapi.XBrokerAPIOriginatingIdentity, identity)

	resp, err := c.cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "application/json" {
		return newServerError(resp.StatusCode, "invalid content type")
	}

	d := json.NewDecoder(resp.Body)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusGone {
		var r *openservicebrokerapi.UnbindResponse
		err = d.Decode(&r)
		if err != nil {
			return err
		}
		return nil
	}

	var r *openservicebrokerapi.ErrorResponse
	err = d.Decode(&r)
	if err != nil {
		return err
	}
	return newServerError(resp.StatusCode, r.Description)
}
