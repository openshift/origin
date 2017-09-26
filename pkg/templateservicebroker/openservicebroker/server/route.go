package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	restful "github.com/emicklei/go-restful"
	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/authentication/user"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/templateservicebroker/openservicebroker/api"
)

// minimum supported client version
const minAPIVersionMajor, minAPIVersionMinor = 2, 7

// Route adds the necessary routes to a restful.Container for a given Broker
// implementing the OSB spec.
func Route(container *restful.Container, path string, b api.Broker) {
	shim := func(f func(api.Broker, *restful.Request) *api.Response) func(*restful.Request, *restful.Response) {
		return func(req *restful.Request, resp *restful.Response) {
			response := f(b, req)
			if response.Err != nil {
				glog.V(2).Infof("Service broker: call to %s returned %v", path, response.Err)

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

/*
	The following properties MUST appear within the JSON encoded `value`:

	| Property | Type | Description |
	| --- | --- | --- |
	| username | string | The `username` property from the Kubenernetes `user.info` object. |
	| uid | string | The `uid` property from the Kubenernetes `user.info` object. |
	| groups | string | The `groups` property from the Kubenernetes `user.info` object. |

	Platforms MAY include additional properties.

	For example, a `value` of:
	```
	{
	  "username": "duke",
	  "uid": "c2dde242-5ce4-11e7-988c-000c2946f14f",
	  "groups": { "admin", "dev" }
	}
	```
	would appear in the HTTP Header as:
	```
	X-Broker-API-Originating-Identity: kubernetes eyANCiAgInVzZXJuYW1lIjogImR1a2UiLA0KICAidWlkIjogImMyZGRlMjQyLTVjZTQtMTFlNy05ODhjLTAwMGMyOTQ2ZjE0ZiIsDQogICJncm91cHMiOiB7ICJhZG1pbiIsICJkZXYiIH0NCn0=
	```
*/
func getUser(req *restful.Request) (user.Info, error) {
	identity := req.Request.Header.Get(api.XBrokerAPIOriginatingIdentity)
	parts := strings.SplitN(identity, " ", 2)
	if !strings.EqualFold(parts[0], api.OriginatingIdentitySchemeKubernetes) || len(parts) != 2 {
		return nil, fmt.Errorf("couldn't parse %s header", api.XBrokerAPIOriginatingIdentity)
	}

	templatereq := templateapi.TemplateInstanceRequester{}
	decodestrbytes, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("couldn't parse %s header: %v", api.XBrokerAPIOriginatingIdentity, err)
	}
	err = json.Unmarshal(decodestrbytes, &templatereq)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse %s header with value %s: %v", api.XBrokerAPIOriginatingIdentity, string(decodestrbytes), err)
	}
	u := api.ConvertTemplateInstanceRequesterToUser(&templatereq)

	return u, nil
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

	if req.QueryParameter("accepts_incomplete") != "true" {
		return api.NewResponse(http.StatusUnprocessableEntity, &api.AsyncRequired, nil)
	}

	u, err := getUser(req)
	if err != nil {
		return api.BadRequest(err)
	}

	return b.Provision(u, instanceID, &preq)
}

func deprovision(b api.Broker, req *restful.Request) *api.Response {
	instanceID := req.PathParameter("instance_id")
	if errors := api.ValidateUUID(field.NewPath("instance_id"), instanceID); errors != nil {
		return api.BadRequest(errors.ToAggregate())
	}

	if req.QueryParameter("accepts_incomplete") != "true" {
		return api.NewResponse(http.StatusUnprocessableEntity, &api.AsyncRequired, nil)
	}

	u, err := getUser(req)
	if err != nil {
		return api.BadRequest(err)
	}

	return b.Deprovision(u, instanceID)
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

	u, err := getUser(req)
	if err != nil {
		return api.BadRequest(err)
	}

	return b.LastOperation(u, instanceID, operation)
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

	u, err := getUser(req)
	if err != nil {
		return api.BadRequest(err)
	}

	return b.Bind(u, instanceID, bindingID, &breq)
}

func unbind(b api.Broker, req *restful.Request) *api.Response {
	instanceID := req.PathParameter("instance_id")
	errors := api.ValidateUUID(field.NewPath("instance_id"), instanceID)

	bindingID := req.PathParameter("binding_id")
	errors = append(errors, api.ValidateUUID(field.NewPath("binding_id"), bindingID)...)

	if len(errors) > 0 {
		return api.BadRequest(errors.ToAggregate())
	}

	u, err := getUser(req)
	if err != nil {
		return api.BadRequest(err)
	}

	return b.Unbind(u, instanceID, bindingID)
}
