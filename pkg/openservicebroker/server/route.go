package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	restful "github.com/emicklei/go-restful"
	"github.com/openshift/origin/pkg/openservicebroker/api"
	"k8s.io/kubernetes/pkg/util/validation/field"
)

// minimum supported client version
const minAPIVersionMajor, minAPIVersionMinor = 2, 7

func Route(container *restful.Container, path string, b api.Broker) {
	shim := func(f func(api.Broker, *restful.Request) *api.Response) func(*restful.Request, *restful.Response) {
		return func(req *restful.Request, resp *restful.Response) {
			response := f(b, req)
			if response.Err != nil {
				resp.WriteHeaderAndJson(response.Code, &api.ErrorResponse{Description: response.Err.Error()}, restful.MIME_JSON)
			} else {
				resp.WriteHeaderAndJson(response.Code, response.Body, restful.MIME_JSON)
			}
		}
	}

	ws := restful.WebService{}
	ws.Path(path + "/v2")
	ws.Filter(apiVersion)
	ws.Filter(contentType)

	ws.Route(ws.GET("/catalog").To(shim(catalog)))
	ws.Route(ws.PUT("/service_instances/{instance_id}").To(shim(provision)))
	ws.Route(ws.DELETE("/service_instances/{instance_id}").To(shim(deprovision)))
	ws.Route(ws.GET("/service_instances/{instance_id}/last_operation").To(shim(lastOperation)))
	ws.Route(ws.PUT("/service_instances/{instance_id}/service_bindings/{binding_id}").To(shim(bind)))
	ws.Route(ws.DELETE("/service_instances/{instance_id}/service_bindings/{binding_id}").To(shim(unbind)))
	container.Add(&ws)
}

func atoi(s string) int {
	rv, err := strconv.Atoi(s)
	if err != nil {
		rv = 0
	}
	return rv
}

func apiVersion(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	resp.AddHeader(api.XBrokerAPIVersion, api.APIVersion)

	versions := strings.SplitN(req.HeaderParameter(api.XBrokerAPIVersion), ".", 3)
	if len(versions) != 2 || atoi(versions[0]) != minAPIVersionMajor || atoi(versions[1]) < minAPIVersionMinor {
		resp.WriteHeaderAndJson(http.StatusPreconditionFailed, &api.ErrorResponse{Description: fmt.Sprintf("%s header must >= %d.%d", api.XBrokerAPIVersion, minAPIVersionMajor, minAPIVersionMinor)}, restful.MIME_JSON)
		return
	}

	chain.ProcessFilter(req, resp)
}

func contentType(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
	resp.AddHeader(restful.HEADER_ContentType, restful.MIME_JSON)

	if req.Request.Method == http.MethodPut && req.HeaderParameter(restful.HEADER_ContentType) != restful.MIME_JSON {
		resp.WriteHeaderAndJson(http.StatusUnsupportedMediaType, &api.ErrorResponse{Description: fmt.Sprintf("%s header must == %s", restful.HEADER_ContentType, restful.MIME_JSON)}, restful.MIME_JSON)
		return
	}

	chain.ProcessFilter(req, resp)
}

func catalog(b api.Broker, req *restful.Request) *api.Response {
	return b.Catalog()
}

func provision(b api.Broker, req *restful.Request) *api.Response {
	instanceID := req.PathParameter("instance_id")
	if errors := api.ValidateUUID(field.NewPath("instance_id"), instanceID); errors != nil {
		return api.BadRequest(errors.ToAggregate())
	}

	var preq api.ProvisionRequest
	err := req.ReadEntity(&preq)
	if err != nil {
		return api.BadRequest(err)
	}
	if errors := api.ValidateProvisionRequest(&preq); errors != nil {
		return api.BadRequest(errors.ToAggregate())
	}

	if !preq.AcceptsIncomplete {
		return api.NewResponse(http.StatusUnprocessableEntity, api.AsyncRequired, nil)
	}

	return b.Provision(instanceID, &preq)
}

func deprovision(b api.Broker, req *restful.Request) *api.Response {
	instanceID := req.PathParameter("instance_id")
	if errors := api.ValidateUUID(field.NewPath("instance_id"), instanceID); errors != nil {
		return api.BadRequest(errors.ToAggregate())
	}

	if req.QueryParameter("accepts_incomplete") != "true" {
		return api.NewResponse(http.StatusUnprocessableEntity, &api.AsyncRequired, nil)
	}

	return b.Deprovision(instanceID)
}

func lastOperation(b api.Broker, req *restful.Request) *api.Response {
	instanceID := req.PathParameter("instance_id")
	if errors := api.ValidateUUID(field.NewPath("instance_id"), instanceID); errors != nil {
		return api.BadRequest(errors.ToAggregate())
	}

	operation := api.Operation(req.QueryParameter("operation"))
	if operation != api.OperationProvisioning &&
		operation != api.OperationUpdating &&
		operation != api.OperationDeprovisioning {
		return api.BadRequest(fmt.Errorf("invalid operation"))
	}

	return b.LastOperation(instanceID, operation)
}

func bind(b api.Broker, req *restful.Request) *api.Response {
	instanceID := req.PathParameter("instance_id")
	errors := api.ValidateUUID(field.NewPath("instance_id"), instanceID)

	bindingID := req.PathParameter("binding_id")
	errors = append(errors, api.ValidateUUID(field.NewPath("binding_id"), bindingID)...)

	if len(errors) > 0 {
		return api.BadRequest(errors.ToAggregate())
	}

	var breq api.BindRequest
	err := req.ReadEntity(&breq)
	if err != nil {
		return api.BadRequest(err)
	}
	if errors = api.ValidateBindRequest(&breq); errors != nil {
		return api.BadRequest(errors.ToAggregate())
	}

	return b.Bind(instanceID, bindingID, &breq)
}

func unbind(b api.Broker, req *restful.Request) *api.Response {
	instanceID := req.PathParameter("instance_id")
	errors := api.ValidateUUID(field.NewPath("instance_id"), instanceID)

	bindingID := req.PathParameter("binding_id")
	errors = append(errors, api.ValidateUUID(field.NewPath("binding_id"), bindingID)...)

	if len(errors) > 0 {
		return api.BadRequest(errors.ToAggregate())
	}

	return b.Unbind(instanceID, bindingID)
}
