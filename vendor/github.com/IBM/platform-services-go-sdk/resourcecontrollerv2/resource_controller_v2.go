/**
 * (C) Copyright IBM Corp. 2024.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/*
 * IBM OpenAPI SDK Code Generator Version: 3.94.1-71478489-20240820-161623
 */

// Package resourcecontrollerv2 : Operations and models for the ResourceControllerV2 service
package resourcecontrollerv2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
	common "github.com/IBM/platform-services-go-sdk/common"
	"github.com/go-openapi/strfmt"
)

// ResourceControllerV2 : Manage lifecycle of your Cloud resources using Resource Controller APIs. Resources are
// provisioned globally in an account scope. Supports asynchronous provisioning of resources. Enables consumption of a
// global resource through a Cloud Foundry space in any region.
//
// API Version: 2.0
type ResourceControllerV2 struct {
	Service *core.BaseService
}

// DefaultServiceURL is the default URL to make service requests to.
const DefaultServiceURL = "https://resource-controller.cloud.ibm.com"

// DefaultServiceName is the default key used to find external configuration information.
const DefaultServiceName = "resource_controller"

// ResourceControllerV2Options : Service options
type ResourceControllerV2Options struct {
	ServiceName   string
	URL           string
	Authenticator core.Authenticator
}

// NewResourceControllerV2UsingExternalConfig : constructs an instance of ResourceControllerV2 with passed in options and external configuration.
func NewResourceControllerV2UsingExternalConfig(options *ResourceControllerV2Options) (resourceController *ResourceControllerV2, err error) {
	if options.ServiceName == "" {
		options.ServiceName = DefaultServiceName
	}

	if options.Authenticator == nil {
		options.Authenticator, err = core.GetAuthenticatorFromEnvironment(options.ServiceName)
		if err != nil {
			err = core.SDKErrorf(err, "", "env-auth-error", common.GetComponentInfo())
			return
		}
	}

	resourceController, err = NewResourceControllerV2(options)
	err = core.RepurposeSDKProblem(err, "new-client-error")
	if err != nil {
		return
	}

	err = resourceController.Service.ConfigureService(options.ServiceName)
	if err != nil {
		err = core.SDKErrorf(err, "", "client-config-error", common.GetComponentInfo())
		return
	}

	if options.URL != "" {
		err = resourceController.Service.SetServiceURL(options.URL)
		err = core.RepurposeSDKProblem(err, "url-set-error")
	}
	return
}

// NewResourceControllerV2 : constructs an instance of ResourceControllerV2 with passed in options.
func NewResourceControllerV2(options *ResourceControllerV2Options) (service *ResourceControllerV2, err error) {
	serviceOptions := &core.ServiceOptions{
		URL:           DefaultServiceURL,
		Authenticator: options.Authenticator,
	}

	baseService, err := core.NewBaseService(serviceOptions)
	if err != nil {
		err = core.SDKErrorf(err, "", "new-base-error", common.GetComponentInfo())
		return
	}

	if options.URL != "" {
		err = baseService.SetServiceURL(options.URL)
		if err != nil {
			err = core.SDKErrorf(err, "", "set-url-error", common.GetComponentInfo())
			return
		}
	}

	service = &ResourceControllerV2{
		Service: baseService,
	}

	return
}

// GetServiceURLForRegion returns the service URL to be used for the specified region
func GetServiceURLForRegion(region string) (string, error) {
	return "", core.SDKErrorf(nil, "service does not support regional URLs", "no-regional-support", common.GetComponentInfo())
}

// Clone makes a copy of "resourceController" suitable for processing requests.
func (resourceController *ResourceControllerV2) Clone() *ResourceControllerV2 {
	if core.IsNil(resourceController) {
		return nil
	}
	clone := *resourceController
	clone.Service = resourceController.Service.Clone()
	return &clone
}

// SetServiceURL sets the service URL
func (resourceController *ResourceControllerV2) SetServiceURL(url string) error {
	err := resourceController.Service.SetServiceURL(url)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-set-error", common.GetComponentInfo())
	}
	return err
}

// GetServiceURL returns the service URL
func (resourceController *ResourceControllerV2) GetServiceURL() string {
	return resourceController.Service.GetServiceURL()
}

// SetDefaultHeaders sets HTTP headers to be sent in every request
func (resourceController *ResourceControllerV2) SetDefaultHeaders(headers http.Header) {
	resourceController.Service.SetDefaultHeaders(headers)
}

// SetEnableGzipCompression sets the service's EnableGzipCompression field
func (resourceController *ResourceControllerV2) SetEnableGzipCompression(enableGzip bool) {
	resourceController.Service.SetEnableGzipCompression(enableGzip)
}

// GetEnableGzipCompression returns the service's EnableGzipCompression field
func (resourceController *ResourceControllerV2) GetEnableGzipCompression() bool {
	return resourceController.Service.GetEnableGzipCompression()
}

// EnableRetries enables automatic retries for requests invoked for this service instance.
// If either parameter is specified as 0, then a default value is used instead.
func (resourceController *ResourceControllerV2) EnableRetries(maxRetries int, maxRetryInterval time.Duration) {
	resourceController.Service.EnableRetries(maxRetries, maxRetryInterval)
}

// DisableRetries disables automatic retries for requests invoked for this service instance.
func (resourceController *ResourceControllerV2) DisableRetries() {
	resourceController.Service.DisableRetries()
}

// ListResourceInstances : Get a list of all resource instances
// View a list of all available resource instances. Resources is a broad term that could mean anything from a service
// instance to a virtual machine associated with the customer account.
func (resourceController *ResourceControllerV2) ListResourceInstances(listResourceInstancesOptions *ListResourceInstancesOptions) (result *ResourceInstancesList, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.ListResourceInstancesWithContext(context.Background(), listResourceInstancesOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// ListResourceInstancesWithContext is an alternate form of the ListResourceInstances method which supports a Context parameter
func (resourceController *ResourceControllerV2) ListResourceInstancesWithContext(ctx context.Context, listResourceInstancesOptions *ListResourceInstancesOptions) (result *ResourceInstancesList, response *core.DetailedResponse, err error) {
	err = core.ValidateStruct(listResourceInstancesOptions, "listResourceInstancesOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	builder := core.NewRequestBuilder(core.GET)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_instances`, nil)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range listResourceInstancesOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "ListResourceInstances")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")

	if listResourceInstancesOptions.GUID != nil {
		builder.AddQuery("guid", fmt.Sprint(*listResourceInstancesOptions.GUID))
	}
	if listResourceInstancesOptions.Name != nil {
		builder.AddQuery("name", fmt.Sprint(*listResourceInstancesOptions.Name))
	}
	if listResourceInstancesOptions.ResourceGroupID != nil {
		builder.AddQuery("resource_group_id", fmt.Sprint(*listResourceInstancesOptions.ResourceGroupID))
	}
	if listResourceInstancesOptions.ResourceID != nil {
		builder.AddQuery("resource_id", fmt.Sprint(*listResourceInstancesOptions.ResourceID))
	}
	if listResourceInstancesOptions.ResourcePlanID != nil {
		builder.AddQuery("resource_plan_id", fmt.Sprint(*listResourceInstancesOptions.ResourcePlanID))
	}
	if listResourceInstancesOptions.Type != nil {
		builder.AddQuery("type", fmt.Sprint(*listResourceInstancesOptions.Type))
	}
	if listResourceInstancesOptions.SubType != nil {
		builder.AddQuery("sub_type", fmt.Sprint(*listResourceInstancesOptions.SubType))
	}
	if listResourceInstancesOptions.Limit != nil {
		builder.AddQuery("limit", fmt.Sprint(*listResourceInstancesOptions.Limit))
	}
	if listResourceInstancesOptions.Start != nil {
		builder.AddQuery("start", fmt.Sprint(*listResourceInstancesOptions.Start))
	}
	if listResourceInstancesOptions.State != nil {
		builder.AddQuery("state", fmt.Sprint(*listResourceInstancesOptions.State))
	}
	if listResourceInstancesOptions.UpdatedFrom != nil {
		builder.AddQuery("updated_from", fmt.Sprint(*listResourceInstancesOptions.UpdatedFrom))
	}
	if listResourceInstancesOptions.UpdatedTo != nil {
		builder.AddQuery("updated_to", fmt.Sprint(*listResourceInstancesOptions.UpdatedTo))
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "list_resource_instances", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceInstancesList)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// CreateResourceInstance : Create (provision) a new resource instance
// When you provision a service you get an instance of that service. An instance represents the resource with which you
// create, and additionally, represents a chargeable record of which billing can occur.
func (resourceController *ResourceControllerV2) CreateResourceInstance(createResourceInstanceOptions *CreateResourceInstanceOptions) (result *ResourceInstance, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.CreateResourceInstanceWithContext(context.Background(), createResourceInstanceOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// CreateResourceInstanceWithContext is an alternate form of the CreateResourceInstance method which supports a Context parameter
func (resourceController *ResourceControllerV2) CreateResourceInstanceWithContext(ctx context.Context, createResourceInstanceOptions *CreateResourceInstanceOptions) (result *ResourceInstance, response *core.DetailedResponse, err error) {
	err = core.ValidateNotNil(createResourceInstanceOptions, "createResourceInstanceOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(createResourceInstanceOptions, "createResourceInstanceOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	builder := core.NewRequestBuilder(core.POST)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_instances`, nil)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range createResourceInstanceOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "CreateResourceInstance")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")
	builder.AddHeader("Content-Type", "application/json")
	if createResourceInstanceOptions.EntityLock != nil {
		builder.AddHeader("Entity-Lock", fmt.Sprint(*createResourceInstanceOptions.EntityLock))
	}

	body := make(map[string]interface{})
	if createResourceInstanceOptions.Name != nil {
		body["name"] = createResourceInstanceOptions.Name
	}
	if createResourceInstanceOptions.Target != nil {
		body["target"] = createResourceInstanceOptions.Target
	}
	if createResourceInstanceOptions.ResourceGroup != nil {
		body["resource_group"] = createResourceInstanceOptions.ResourceGroup
	}
	if createResourceInstanceOptions.ResourcePlanID != nil {
		body["resource_plan_id"] = createResourceInstanceOptions.ResourcePlanID
	}
	if createResourceInstanceOptions.Tags != nil {
		body["tags"] = createResourceInstanceOptions.Tags
	}
	if createResourceInstanceOptions.AllowCleanup != nil {
		body["allow_cleanup"] = createResourceInstanceOptions.AllowCleanup
	}
	if createResourceInstanceOptions.Parameters != nil {
		body["parameters"] = createResourceInstanceOptions.Parameters
	}
	_, err = builder.SetBodyContentJSON(body)
	if err != nil {
		err = core.SDKErrorf(err, "", "set-json-body-error", common.GetComponentInfo())
		return
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "create_resource_instance", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceInstance)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// GetResourceInstance : Get a resource instance
// Retrieve a resource instance by URL-encoded CRN or GUID. Find more details on a particular instance, like when it was
// provisioned and who provisioned it.
func (resourceController *ResourceControllerV2) GetResourceInstance(getResourceInstanceOptions *GetResourceInstanceOptions) (result *ResourceInstance, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.GetResourceInstanceWithContext(context.Background(), getResourceInstanceOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// GetResourceInstanceWithContext is an alternate form of the GetResourceInstance method which supports a Context parameter
func (resourceController *ResourceControllerV2) GetResourceInstanceWithContext(ctx context.Context, getResourceInstanceOptions *GetResourceInstanceOptions) (result *ResourceInstance, response *core.DetailedResponse, err error) {
	err = core.ValidateNotNil(getResourceInstanceOptions, "getResourceInstanceOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(getResourceInstanceOptions, "getResourceInstanceOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *getResourceInstanceOptions.ID,
	}

	builder := core.NewRequestBuilder(core.GET)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_instances/{id}`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range getResourceInstanceOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "GetResourceInstance")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "get_resource_instance", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceInstance)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// DeleteResourceInstance : Delete a resource instance
// Delete a resource instance by URL-encoded CRN or GUID. If the resource instance has any resource keys or aliases
// associated with it, use the `recursive=true` parameter to delete it.
func (resourceController *ResourceControllerV2) DeleteResourceInstance(deleteResourceInstanceOptions *DeleteResourceInstanceOptions) (response *core.DetailedResponse, err error) {
	response, err = resourceController.DeleteResourceInstanceWithContext(context.Background(), deleteResourceInstanceOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// DeleteResourceInstanceWithContext is an alternate form of the DeleteResourceInstance method which supports a Context parameter
func (resourceController *ResourceControllerV2) DeleteResourceInstanceWithContext(ctx context.Context, deleteResourceInstanceOptions *DeleteResourceInstanceOptions) (response *core.DetailedResponse, err error) {
	err = core.ValidateNotNil(deleteResourceInstanceOptions, "deleteResourceInstanceOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(deleteResourceInstanceOptions, "deleteResourceInstanceOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *deleteResourceInstanceOptions.ID,
	}

	builder := core.NewRequestBuilder(core.DELETE)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_instances/{id}`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range deleteResourceInstanceOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "DeleteResourceInstance")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}

	if deleteResourceInstanceOptions.Recursive != nil {
		builder.AddQuery("recursive", fmt.Sprint(*deleteResourceInstanceOptions.Recursive))
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	response, err = resourceController.Service.Request(request, nil)
	if err != nil {
		core.EnrichHTTPProblem(err, "delete_resource_instance", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}

	return
}

// UpdateResourceInstance : Update a resource instance
// Use the resource instance URL-encoded CRN or GUID to make updates to the resource instance, like changing the name or
// plan.
func (resourceController *ResourceControllerV2) UpdateResourceInstance(updateResourceInstanceOptions *UpdateResourceInstanceOptions) (result *ResourceInstance, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.UpdateResourceInstanceWithContext(context.Background(), updateResourceInstanceOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// UpdateResourceInstanceWithContext is an alternate form of the UpdateResourceInstance method which supports a Context parameter
func (resourceController *ResourceControllerV2) UpdateResourceInstanceWithContext(ctx context.Context, updateResourceInstanceOptions *UpdateResourceInstanceOptions) (result *ResourceInstance, response *core.DetailedResponse, err error) {
	err = core.ValidateNotNil(updateResourceInstanceOptions, "updateResourceInstanceOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(updateResourceInstanceOptions, "updateResourceInstanceOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *updateResourceInstanceOptions.ID,
	}

	builder := core.NewRequestBuilder(core.PATCH)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_instances/{id}`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range updateResourceInstanceOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "UpdateResourceInstance")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")
	builder.AddHeader("Content-Type", "application/json")

	body := make(map[string]interface{})
	if updateResourceInstanceOptions.Name != nil {
		body["name"] = updateResourceInstanceOptions.Name
	}
	if updateResourceInstanceOptions.Parameters != nil {
		body["parameters"] = updateResourceInstanceOptions.Parameters
	}
	if updateResourceInstanceOptions.ResourcePlanID != nil {
		body["resource_plan_id"] = updateResourceInstanceOptions.ResourcePlanID
	}
	if updateResourceInstanceOptions.AllowCleanup != nil {
		body["allow_cleanup"] = updateResourceInstanceOptions.AllowCleanup
	}
	_, err = builder.SetBodyContentJSON(body)
	if err != nil {
		err = core.SDKErrorf(err, "", "set-json-body-error", common.GetComponentInfo())
		return
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "update_resource_instance", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceInstance)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// ListResourceAliasesForInstance : Get a list of all resource aliases for the instance
// Retrieving a list of all resource aliases can help you find out who's using the resource instance.
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) ListResourceAliasesForInstance(listResourceAliasesForInstanceOptions *ListResourceAliasesForInstanceOptions) (result *ResourceAliasesList, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.ListResourceAliasesForInstanceWithContext(context.Background(), listResourceAliasesForInstanceOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// ListResourceAliasesForInstanceWithContext is an alternate form of the ListResourceAliasesForInstance method which supports a Context parameter
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) ListResourceAliasesForInstanceWithContext(ctx context.Context, listResourceAliasesForInstanceOptions *ListResourceAliasesForInstanceOptions) (result *ResourceAliasesList, response *core.DetailedResponse, err error) {
	core.GetLogger().Warn("A deprecated operation has been invoked: ListResourceAliasesForInstance")
	err = core.ValidateNotNil(listResourceAliasesForInstanceOptions, "listResourceAliasesForInstanceOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(listResourceAliasesForInstanceOptions, "listResourceAliasesForInstanceOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *listResourceAliasesForInstanceOptions.ID,
	}

	builder := core.NewRequestBuilder(core.GET)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_instances/{id}/resource_aliases`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range listResourceAliasesForInstanceOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "ListResourceAliasesForInstance")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")

	if listResourceAliasesForInstanceOptions.Limit != nil {
		builder.AddQuery("limit", fmt.Sprint(*listResourceAliasesForInstanceOptions.Limit))
	}
	if listResourceAliasesForInstanceOptions.Start != nil {
		builder.AddQuery("start", fmt.Sprint(*listResourceAliasesForInstanceOptions.Start))
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "list_resource_aliases_for_instance", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceAliasesList)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// ListResourceKeysForInstance : Get a list of all the resource keys for the instance
// You may have many resource keys for one resource instance. For example, you may have a different resource key for
// each user or each role.
func (resourceController *ResourceControllerV2) ListResourceKeysForInstance(listResourceKeysForInstanceOptions *ListResourceKeysForInstanceOptions) (result *ResourceKeysList, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.ListResourceKeysForInstanceWithContext(context.Background(), listResourceKeysForInstanceOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// ListResourceKeysForInstanceWithContext is an alternate form of the ListResourceKeysForInstance method which supports a Context parameter
func (resourceController *ResourceControllerV2) ListResourceKeysForInstanceWithContext(ctx context.Context, listResourceKeysForInstanceOptions *ListResourceKeysForInstanceOptions) (result *ResourceKeysList, response *core.DetailedResponse, err error) {
	err = core.ValidateNotNil(listResourceKeysForInstanceOptions, "listResourceKeysForInstanceOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(listResourceKeysForInstanceOptions, "listResourceKeysForInstanceOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *listResourceKeysForInstanceOptions.ID,
	}

	builder := core.NewRequestBuilder(core.GET)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_instances/{id}/resource_keys`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range listResourceKeysForInstanceOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "ListResourceKeysForInstance")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")

	if listResourceKeysForInstanceOptions.Limit != nil {
		builder.AddQuery("limit", fmt.Sprint(*listResourceKeysForInstanceOptions.Limit))
	}
	if listResourceKeysForInstanceOptions.Start != nil {
		builder.AddQuery("start", fmt.Sprint(*listResourceKeysForInstanceOptions.Start))
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "list_resource_keys_for_instance", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceKeysList)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// LockResourceInstance : Lock a resource instance
// Locks a resource instance. A locked instance can not be updated or deleted. It does not affect actions performed on
// child resources like aliases, bindings, or keys.
func (resourceController *ResourceControllerV2) LockResourceInstance(lockResourceInstanceOptions *LockResourceInstanceOptions) (result *ResourceInstance, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.LockResourceInstanceWithContext(context.Background(), lockResourceInstanceOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// LockResourceInstanceWithContext is an alternate form of the LockResourceInstance method which supports a Context parameter
func (resourceController *ResourceControllerV2) LockResourceInstanceWithContext(ctx context.Context, lockResourceInstanceOptions *LockResourceInstanceOptions) (result *ResourceInstance, response *core.DetailedResponse, err error) {
	err = core.ValidateNotNil(lockResourceInstanceOptions, "lockResourceInstanceOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(lockResourceInstanceOptions, "lockResourceInstanceOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *lockResourceInstanceOptions.ID,
	}

	builder := core.NewRequestBuilder(core.POST)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_instances/{id}/lock`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range lockResourceInstanceOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "LockResourceInstance")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "lock_resource_instance", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceInstance)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// UnlockResourceInstance : Unlock a resource instance
// Unlock a resource instance to update or delete it. Unlocking a resource instance does not affect child resources like
// aliases, bindings or keys.
func (resourceController *ResourceControllerV2) UnlockResourceInstance(unlockResourceInstanceOptions *UnlockResourceInstanceOptions) (result *ResourceInstance, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.UnlockResourceInstanceWithContext(context.Background(), unlockResourceInstanceOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// UnlockResourceInstanceWithContext is an alternate form of the UnlockResourceInstance method which supports a Context parameter
func (resourceController *ResourceControllerV2) UnlockResourceInstanceWithContext(ctx context.Context, unlockResourceInstanceOptions *UnlockResourceInstanceOptions) (result *ResourceInstance, response *core.DetailedResponse, err error) {
	err = core.ValidateNotNil(unlockResourceInstanceOptions, "unlockResourceInstanceOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(unlockResourceInstanceOptions, "unlockResourceInstanceOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *unlockResourceInstanceOptions.ID,
	}

	builder := core.NewRequestBuilder(core.DELETE)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_instances/{id}/lock`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range unlockResourceInstanceOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "UnlockResourceInstance")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "unlock_resource_instance", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceInstance)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// CancelLastopResourceInstance : Cancel the in progress last operation of the resource instance
// Cancel the in progress last operation of the resource instance. After successful cancellation, the resource instance
// is removed.
func (resourceController *ResourceControllerV2) CancelLastopResourceInstance(cancelLastopResourceInstanceOptions *CancelLastopResourceInstanceOptions) (result *ResourceInstance, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.CancelLastopResourceInstanceWithContext(context.Background(), cancelLastopResourceInstanceOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// CancelLastopResourceInstanceWithContext is an alternate form of the CancelLastopResourceInstance method which supports a Context parameter
func (resourceController *ResourceControllerV2) CancelLastopResourceInstanceWithContext(ctx context.Context, cancelLastopResourceInstanceOptions *CancelLastopResourceInstanceOptions) (result *ResourceInstance, response *core.DetailedResponse, err error) {
	err = core.ValidateNotNil(cancelLastopResourceInstanceOptions, "cancelLastopResourceInstanceOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(cancelLastopResourceInstanceOptions, "cancelLastopResourceInstanceOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *cancelLastopResourceInstanceOptions.ID,
	}

	builder := core.NewRequestBuilder(core.DELETE)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_instances/{id}/last_operation`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range cancelLastopResourceInstanceOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "CancelLastopResourceInstance")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "cancel_lastop_resource_instance", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceInstance)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// ListResourceKeys : Get a list of all of the resource keys
// View all of the resource keys that exist for all of your resource instances.
func (resourceController *ResourceControllerV2) ListResourceKeys(listResourceKeysOptions *ListResourceKeysOptions) (result *ResourceKeysList, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.ListResourceKeysWithContext(context.Background(), listResourceKeysOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// ListResourceKeysWithContext is an alternate form of the ListResourceKeys method which supports a Context parameter
func (resourceController *ResourceControllerV2) ListResourceKeysWithContext(ctx context.Context, listResourceKeysOptions *ListResourceKeysOptions) (result *ResourceKeysList, response *core.DetailedResponse, err error) {
	err = core.ValidateStruct(listResourceKeysOptions, "listResourceKeysOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	builder := core.NewRequestBuilder(core.GET)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_keys`, nil)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range listResourceKeysOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "ListResourceKeys")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")

	if listResourceKeysOptions.GUID != nil {
		builder.AddQuery("guid", fmt.Sprint(*listResourceKeysOptions.GUID))
	}
	if listResourceKeysOptions.Name != nil {
		builder.AddQuery("name", fmt.Sprint(*listResourceKeysOptions.Name))
	}
	if listResourceKeysOptions.ResourceGroupID != nil {
		builder.AddQuery("resource_group_id", fmt.Sprint(*listResourceKeysOptions.ResourceGroupID))
	}
	if listResourceKeysOptions.ResourceID != nil {
		builder.AddQuery("resource_id", fmt.Sprint(*listResourceKeysOptions.ResourceID))
	}
	if listResourceKeysOptions.Limit != nil {
		builder.AddQuery("limit", fmt.Sprint(*listResourceKeysOptions.Limit))
	}
	if listResourceKeysOptions.Start != nil {
		builder.AddQuery("start", fmt.Sprint(*listResourceKeysOptions.Start))
	}
	if listResourceKeysOptions.UpdatedFrom != nil {
		builder.AddQuery("updated_from", fmt.Sprint(*listResourceKeysOptions.UpdatedFrom))
	}
	if listResourceKeysOptions.UpdatedTo != nil {
		builder.AddQuery("updated_to", fmt.Sprint(*listResourceKeysOptions.UpdatedTo))
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "list_resource_keys", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceKeysList)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// CreateResourceKey : Create a new resource key
// A resource key is a saved credential you can use to authenticate with a resource instance.
func (resourceController *ResourceControllerV2) CreateResourceKey(createResourceKeyOptions *CreateResourceKeyOptions) (result *ResourceKey, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.CreateResourceKeyWithContext(context.Background(), createResourceKeyOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// CreateResourceKeyWithContext is an alternate form of the CreateResourceKey method which supports a Context parameter
func (resourceController *ResourceControllerV2) CreateResourceKeyWithContext(ctx context.Context, createResourceKeyOptions *CreateResourceKeyOptions) (result *ResourceKey, response *core.DetailedResponse, err error) {
	err = core.ValidateNotNil(createResourceKeyOptions, "createResourceKeyOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(createResourceKeyOptions, "createResourceKeyOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	builder := core.NewRequestBuilder(core.POST)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_keys`, nil)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range createResourceKeyOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "CreateResourceKey")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")
	builder.AddHeader("Content-Type", "application/json")

	body := make(map[string]interface{})
	if createResourceKeyOptions.Name != nil {
		body["name"] = createResourceKeyOptions.Name
	}
	if createResourceKeyOptions.Source != nil {
		body["source"] = createResourceKeyOptions.Source
	}
	if createResourceKeyOptions.Parameters != nil {
		body["parameters"] = createResourceKeyOptions.Parameters
	}
	if createResourceKeyOptions.Role != nil {
		body["role"] = createResourceKeyOptions.Role
	}
	_, err = builder.SetBodyContentJSON(body)
	if err != nil {
		err = core.SDKErrorf(err, "", "set-json-body-error", common.GetComponentInfo())
		return
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "create_resource_key", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceKey)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// GetResourceKey : Get resource key
// View the details of a resource key by URL-encoded CRN or GUID, like the credentials for the key and who created it.
func (resourceController *ResourceControllerV2) GetResourceKey(getResourceKeyOptions *GetResourceKeyOptions) (result *ResourceKey, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.GetResourceKeyWithContext(context.Background(), getResourceKeyOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// GetResourceKeyWithContext is an alternate form of the GetResourceKey method which supports a Context parameter
func (resourceController *ResourceControllerV2) GetResourceKeyWithContext(ctx context.Context, getResourceKeyOptions *GetResourceKeyOptions) (result *ResourceKey, response *core.DetailedResponse, err error) {
	err = core.ValidateNotNil(getResourceKeyOptions, "getResourceKeyOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(getResourceKeyOptions, "getResourceKeyOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *getResourceKeyOptions.ID,
	}

	builder := core.NewRequestBuilder(core.GET)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_keys/{id}`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range getResourceKeyOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "GetResourceKey")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "get_resource_key", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceKey)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// DeleteResourceKey : Delete a resource key
// Deleting a resource key does not affect any resource instance or resource alias associated with the key.
func (resourceController *ResourceControllerV2) DeleteResourceKey(deleteResourceKeyOptions *DeleteResourceKeyOptions) (response *core.DetailedResponse, err error) {
	response, err = resourceController.DeleteResourceKeyWithContext(context.Background(), deleteResourceKeyOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// DeleteResourceKeyWithContext is an alternate form of the DeleteResourceKey method which supports a Context parameter
func (resourceController *ResourceControllerV2) DeleteResourceKeyWithContext(ctx context.Context, deleteResourceKeyOptions *DeleteResourceKeyOptions) (response *core.DetailedResponse, err error) {
	err = core.ValidateNotNil(deleteResourceKeyOptions, "deleteResourceKeyOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(deleteResourceKeyOptions, "deleteResourceKeyOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *deleteResourceKeyOptions.ID,
	}

	builder := core.NewRequestBuilder(core.DELETE)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_keys/{id}`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range deleteResourceKeyOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "DeleteResourceKey")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	response, err = resourceController.Service.Request(request, nil)
	if err != nil {
		core.EnrichHTTPProblem(err, "delete_resource_key", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}

	return
}

// UpdateResourceKey : Update a resource key
// Use the resource key URL-encoded CRN or GUID to update the resource key.
func (resourceController *ResourceControllerV2) UpdateResourceKey(updateResourceKeyOptions *UpdateResourceKeyOptions) (result *ResourceKey, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.UpdateResourceKeyWithContext(context.Background(), updateResourceKeyOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// UpdateResourceKeyWithContext is an alternate form of the UpdateResourceKey method which supports a Context parameter
func (resourceController *ResourceControllerV2) UpdateResourceKeyWithContext(ctx context.Context, updateResourceKeyOptions *UpdateResourceKeyOptions) (result *ResourceKey, response *core.DetailedResponse, err error) {
	err = core.ValidateNotNil(updateResourceKeyOptions, "updateResourceKeyOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(updateResourceKeyOptions, "updateResourceKeyOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *updateResourceKeyOptions.ID,
	}

	builder := core.NewRequestBuilder(core.PATCH)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_keys/{id}`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range updateResourceKeyOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "UpdateResourceKey")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")
	builder.AddHeader("Content-Type", "application/json")

	body := make(map[string]interface{})
	if updateResourceKeyOptions.Name != nil {
		body["name"] = updateResourceKeyOptions.Name
	}
	_, err = builder.SetBodyContentJSON(body)
	if err != nil {
		err = core.SDKErrorf(err, "", "set-json-body-error", common.GetComponentInfo())
		return
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "update_resource_key", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceKey)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// ListResourceBindings : Get a list of all resource bindings
// View all of the resource bindings that exist for all of your resource aliases.
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) ListResourceBindings(listResourceBindingsOptions *ListResourceBindingsOptions) (result *ResourceBindingsList, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.ListResourceBindingsWithContext(context.Background(), listResourceBindingsOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// ListResourceBindingsWithContext is an alternate form of the ListResourceBindings method which supports a Context parameter
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) ListResourceBindingsWithContext(ctx context.Context, listResourceBindingsOptions *ListResourceBindingsOptions) (result *ResourceBindingsList, response *core.DetailedResponse, err error) {
	core.GetLogger().Warn("A deprecated operation has been invoked: ListResourceBindings")
	err = core.ValidateStruct(listResourceBindingsOptions, "listResourceBindingsOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	builder := core.NewRequestBuilder(core.GET)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_bindings`, nil)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range listResourceBindingsOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "ListResourceBindings")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")

	if listResourceBindingsOptions.GUID != nil {
		builder.AddQuery("guid", fmt.Sprint(*listResourceBindingsOptions.GUID))
	}
	if listResourceBindingsOptions.Name != nil {
		builder.AddQuery("name", fmt.Sprint(*listResourceBindingsOptions.Name))
	}
	if listResourceBindingsOptions.ResourceGroupID != nil {
		builder.AddQuery("resource_group_id", fmt.Sprint(*listResourceBindingsOptions.ResourceGroupID))
	}
	if listResourceBindingsOptions.ResourceID != nil {
		builder.AddQuery("resource_id", fmt.Sprint(*listResourceBindingsOptions.ResourceID))
	}
	if listResourceBindingsOptions.RegionBindingID != nil {
		builder.AddQuery("region_binding_id", fmt.Sprint(*listResourceBindingsOptions.RegionBindingID))
	}
	if listResourceBindingsOptions.Limit != nil {
		builder.AddQuery("limit", fmt.Sprint(*listResourceBindingsOptions.Limit))
	}
	if listResourceBindingsOptions.Start != nil {
		builder.AddQuery("start", fmt.Sprint(*listResourceBindingsOptions.Start))
	}
	if listResourceBindingsOptions.UpdatedFrom != nil {
		builder.AddQuery("updated_from", fmt.Sprint(*listResourceBindingsOptions.UpdatedFrom))
	}
	if listResourceBindingsOptions.UpdatedTo != nil {
		builder.AddQuery("updated_to", fmt.Sprint(*listResourceBindingsOptions.UpdatedTo))
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "list_resource_bindings", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceBindingsList)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// CreateResourceBinding : Create a new resource binding
// A resource binding connects credentials to a resource alias. The credentials are in the form of a resource key.
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) CreateResourceBinding(createResourceBindingOptions *CreateResourceBindingOptions) (result *ResourceBinding, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.CreateResourceBindingWithContext(context.Background(), createResourceBindingOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// CreateResourceBindingWithContext is an alternate form of the CreateResourceBinding method which supports a Context parameter
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) CreateResourceBindingWithContext(ctx context.Context, createResourceBindingOptions *CreateResourceBindingOptions) (result *ResourceBinding, response *core.DetailedResponse, err error) {
	core.GetLogger().Warn("A deprecated operation has been invoked: CreateResourceBinding")
	err = core.ValidateNotNil(createResourceBindingOptions, "createResourceBindingOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(createResourceBindingOptions, "createResourceBindingOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	builder := core.NewRequestBuilder(core.POST)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_bindings`, nil)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range createResourceBindingOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "CreateResourceBinding")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")
	builder.AddHeader("Content-Type", "application/json")

	body := make(map[string]interface{})
	if createResourceBindingOptions.Source != nil {
		body["source"] = createResourceBindingOptions.Source
	}
	if createResourceBindingOptions.Target != nil {
		body["target"] = createResourceBindingOptions.Target
	}
	if createResourceBindingOptions.Name != nil {
		body["name"] = createResourceBindingOptions.Name
	}
	if createResourceBindingOptions.Parameters != nil {
		body["parameters"] = createResourceBindingOptions.Parameters
	}
	if createResourceBindingOptions.Role != nil {
		body["role"] = createResourceBindingOptions.Role
	}
	_, err = builder.SetBodyContentJSON(body)
	if err != nil {
		err = core.SDKErrorf(err, "", "set-json-body-error", common.GetComponentInfo())
		return
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "create_resource_binding", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceBinding)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// GetResourceBinding : Get a resource binding
// View a resource binding and all of its details, like who created it, the credential, and the resource alias that the
// binding is associated with.
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) GetResourceBinding(getResourceBindingOptions *GetResourceBindingOptions) (result *ResourceBinding, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.GetResourceBindingWithContext(context.Background(), getResourceBindingOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// GetResourceBindingWithContext is an alternate form of the GetResourceBinding method which supports a Context parameter
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) GetResourceBindingWithContext(ctx context.Context, getResourceBindingOptions *GetResourceBindingOptions) (result *ResourceBinding, response *core.DetailedResponse, err error) {
	core.GetLogger().Warn("A deprecated operation has been invoked: GetResourceBinding")
	err = core.ValidateNotNil(getResourceBindingOptions, "getResourceBindingOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(getResourceBindingOptions, "getResourceBindingOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *getResourceBindingOptions.ID,
	}

	builder := core.NewRequestBuilder(core.GET)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_bindings/{id}`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range getResourceBindingOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "GetResourceBinding")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "get_resource_binding", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceBinding)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// DeleteResourceBinding : Delete a resource binding
// Deleting a resource binding does not affect the resource alias that the binding is associated with.
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) DeleteResourceBinding(deleteResourceBindingOptions *DeleteResourceBindingOptions) (response *core.DetailedResponse, err error) {
	response, err = resourceController.DeleteResourceBindingWithContext(context.Background(), deleteResourceBindingOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// DeleteResourceBindingWithContext is an alternate form of the DeleteResourceBinding method which supports a Context parameter
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) DeleteResourceBindingWithContext(ctx context.Context, deleteResourceBindingOptions *DeleteResourceBindingOptions) (response *core.DetailedResponse, err error) {
	core.GetLogger().Warn("A deprecated operation has been invoked: DeleteResourceBinding")
	err = core.ValidateNotNil(deleteResourceBindingOptions, "deleteResourceBindingOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(deleteResourceBindingOptions, "deleteResourceBindingOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *deleteResourceBindingOptions.ID,
	}

	builder := core.NewRequestBuilder(core.DELETE)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_bindings/{id}`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range deleteResourceBindingOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "DeleteResourceBinding")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	response, err = resourceController.Service.Request(request, nil)
	if err != nil {
		core.EnrichHTTPProblem(err, "delete_resource_binding", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}

	return
}

// UpdateResourceBinding : Update a resource binding
// Use the resource binding URL-encoded CRN or GUID to update the resource binding.
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) UpdateResourceBinding(updateResourceBindingOptions *UpdateResourceBindingOptions) (result *ResourceBinding, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.UpdateResourceBindingWithContext(context.Background(), updateResourceBindingOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// UpdateResourceBindingWithContext is an alternate form of the UpdateResourceBinding method which supports a Context parameter
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) UpdateResourceBindingWithContext(ctx context.Context, updateResourceBindingOptions *UpdateResourceBindingOptions) (result *ResourceBinding, response *core.DetailedResponse, err error) {
	core.GetLogger().Warn("A deprecated operation has been invoked: UpdateResourceBinding")
	err = core.ValidateNotNil(updateResourceBindingOptions, "updateResourceBindingOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(updateResourceBindingOptions, "updateResourceBindingOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *updateResourceBindingOptions.ID,
	}

	builder := core.NewRequestBuilder(core.PATCH)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_bindings/{id}`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range updateResourceBindingOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "UpdateResourceBinding")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")
	builder.AddHeader("Content-Type", "application/json")

	body := make(map[string]interface{})
	if updateResourceBindingOptions.Name != nil {
		body["name"] = updateResourceBindingOptions.Name
	}
	_, err = builder.SetBodyContentJSON(body)
	if err != nil {
		err = core.SDKErrorf(err, "", "set-json-body-error", common.GetComponentInfo())
		return
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "update_resource_binding", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceBinding)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// ListResourceAliases : Get a list of all resource aliases
// View all of the resource aliases that exist for every resource instance.
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) ListResourceAliases(listResourceAliasesOptions *ListResourceAliasesOptions) (result *ResourceAliasesList, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.ListResourceAliasesWithContext(context.Background(), listResourceAliasesOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// ListResourceAliasesWithContext is an alternate form of the ListResourceAliases method which supports a Context parameter
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) ListResourceAliasesWithContext(ctx context.Context, listResourceAliasesOptions *ListResourceAliasesOptions) (result *ResourceAliasesList, response *core.DetailedResponse, err error) {
	core.GetLogger().Warn("A deprecated operation has been invoked: ListResourceAliases")
	err = core.ValidateStruct(listResourceAliasesOptions, "listResourceAliasesOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	builder := core.NewRequestBuilder(core.GET)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_aliases`, nil)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range listResourceAliasesOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "ListResourceAliases")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")

	if listResourceAliasesOptions.GUID != nil {
		builder.AddQuery("guid", fmt.Sprint(*listResourceAliasesOptions.GUID))
	}
	if listResourceAliasesOptions.Name != nil {
		builder.AddQuery("name", fmt.Sprint(*listResourceAliasesOptions.Name))
	}
	if listResourceAliasesOptions.ResourceInstanceID != nil {
		builder.AddQuery("resource_instance_id", fmt.Sprint(*listResourceAliasesOptions.ResourceInstanceID))
	}
	if listResourceAliasesOptions.RegionInstanceID != nil {
		builder.AddQuery("region_instance_id", fmt.Sprint(*listResourceAliasesOptions.RegionInstanceID))
	}
	if listResourceAliasesOptions.ResourceID != nil {
		builder.AddQuery("resource_id", fmt.Sprint(*listResourceAliasesOptions.ResourceID))
	}
	if listResourceAliasesOptions.ResourceGroupID != nil {
		builder.AddQuery("resource_group_id", fmt.Sprint(*listResourceAliasesOptions.ResourceGroupID))
	}
	if listResourceAliasesOptions.Limit != nil {
		builder.AddQuery("limit", fmt.Sprint(*listResourceAliasesOptions.Limit))
	}
	if listResourceAliasesOptions.Start != nil {
		builder.AddQuery("start", fmt.Sprint(*listResourceAliasesOptions.Start))
	}
	if listResourceAliasesOptions.UpdatedFrom != nil {
		builder.AddQuery("updated_from", fmt.Sprint(*listResourceAliasesOptions.UpdatedFrom))
	}
	if listResourceAliasesOptions.UpdatedTo != nil {
		builder.AddQuery("updated_to", fmt.Sprint(*listResourceAliasesOptions.UpdatedTo))
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "list_resource_aliases", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceAliasesList)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// CreateResourceAlias : Create a new resource alias
// Alias a resource instance into a targeted environment's (name)space.
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) CreateResourceAlias(createResourceAliasOptions *CreateResourceAliasOptions) (result *ResourceAlias, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.CreateResourceAliasWithContext(context.Background(), createResourceAliasOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// CreateResourceAliasWithContext is an alternate form of the CreateResourceAlias method which supports a Context parameter
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) CreateResourceAliasWithContext(ctx context.Context, createResourceAliasOptions *CreateResourceAliasOptions) (result *ResourceAlias, response *core.DetailedResponse, err error) {
	core.GetLogger().Warn("A deprecated operation has been invoked: CreateResourceAlias")
	err = core.ValidateNotNil(createResourceAliasOptions, "createResourceAliasOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(createResourceAliasOptions, "createResourceAliasOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	builder := core.NewRequestBuilder(core.POST)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_aliases`, nil)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range createResourceAliasOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "CreateResourceAlias")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")
	builder.AddHeader("Content-Type", "application/json")

	body := make(map[string]interface{})
	if createResourceAliasOptions.Name != nil {
		body["name"] = createResourceAliasOptions.Name
	}
	if createResourceAliasOptions.Source != nil {
		body["source"] = createResourceAliasOptions.Source
	}
	if createResourceAliasOptions.Target != nil {
		body["target"] = createResourceAliasOptions.Target
	}
	_, err = builder.SetBodyContentJSON(body)
	if err != nil {
		err = core.SDKErrorf(err, "", "set-json-body-error", common.GetComponentInfo())
		return
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "create_resource_alias", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceAlias)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// GetResourceAlias : Get a resource alias
// View a resource alias and all of its details, like who created it and the resource instance that it's associated
// with.
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) GetResourceAlias(getResourceAliasOptions *GetResourceAliasOptions) (result *ResourceAlias, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.GetResourceAliasWithContext(context.Background(), getResourceAliasOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// GetResourceAliasWithContext is an alternate form of the GetResourceAlias method which supports a Context parameter
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) GetResourceAliasWithContext(ctx context.Context, getResourceAliasOptions *GetResourceAliasOptions) (result *ResourceAlias, response *core.DetailedResponse, err error) {
	core.GetLogger().Warn("A deprecated operation has been invoked: GetResourceAlias")
	err = core.ValidateNotNil(getResourceAliasOptions, "getResourceAliasOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(getResourceAliasOptions, "getResourceAliasOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *getResourceAliasOptions.ID,
	}

	builder := core.NewRequestBuilder(core.GET)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_aliases/{id}`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range getResourceAliasOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "GetResourceAlias")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "get_resource_alias", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceAlias)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// DeleteResourceAlias : Delete a resource alias
// Delete a resource alias by URL-encoded CRN or GUID. If the resource alias has any resource keys or bindings
// associated with it, use the `recursive=true` parameter to delete it.
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) DeleteResourceAlias(deleteResourceAliasOptions *DeleteResourceAliasOptions) (response *core.DetailedResponse, err error) {
	response, err = resourceController.DeleteResourceAliasWithContext(context.Background(), deleteResourceAliasOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// DeleteResourceAliasWithContext is an alternate form of the DeleteResourceAlias method which supports a Context parameter
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) DeleteResourceAliasWithContext(ctx context.Context, deleteResourceAliasOptions *DeleteResourceAliasOptions) (response *core.DetailedResponse, err error) {
	core.GetLogger().Warn("A deprecated operation has been invoked: DeleteResourceAlias")
	err = core.ValidateNotNil(deleteResourceAliasOptions, "deleteResourceAliasOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(deleteResourceAliasOptions, "deleteResourceAliasOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *deleteResourceAliasOptions.ID,
	}

	builder := core.NewRequestBuilder(core.DELETE)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_aliases/{id}`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range deleteResourceAliasOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "DeleteResourceAlias")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}

	if deleteResourceAliasOptions.Recursive != nil {
		builder.AddQuery("recursive", fmt.Sprint(*deleteResourceAliasOptions.Recursive))
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	response, err = resourceController.Service.Request(request, nil)
	if err != nil {
		core.EnrichHTTPProblem(err, "delete_resource_alias", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}

	return
}

// UpdateResourceAlias : Update a resource alias
// Use the resource alias URL-encoded CRN or GUID to update the resource alias.
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) UpdateResourceAlias(updateResourceAliasOptions *UpdateResourceAliasOptions) (result *ResourceAlias, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.UpdateResourceAliasWithContext(context.Background(), updateResourceAliasOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// UpdateResourceAliasWithContext is an alternate form of the UpdateResourceAlias method which supports a Context parameter
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) UpdateResourceAliasWithContext(ctx context.Context, updateResourceAliasOptions *UpdateResourceAliasOptions) (result *ResourceAlias, response *core.DetailedResponse, err error) {
	core.GetLogger().Warn("A deprecated operation has been invoked: UpdateResourceAlias")
	err = core.ValidateNotNil(updateResourceAliasOptions, "updateResourceAliasOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(updateResourceAliasOptions, "updateResourceAliasOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *updateResourceAliasOptions.ID,
	}

	builder := core.NewRequestBuilder(core.PATCH)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_aliases/{id}`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range updateResourceAliasOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "UpdateResourceAlias")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")
	builder.AddHeader("Content-Type", "application/json")

	body := make(map[string]interface{})
	if updateResourceAliasOptions.Name != nil {
		body["name"] = updateResourceAliasOptions.Name
	}
	_, err = builder.SetBodyContentJSON(body)
	if err != nil {
		err = core.SDKErrorf(err, "", "set-json-body-error", common.GetComponentInfo())
		return
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "update_resource_alias", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceAlias)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// ListResourceBindingsForAlias : Get a list of all resource bindings for the alias
// View all of the resource bindings associated with a specific resource alias.
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) ListResourceBindingsForAlias(listResourceBindingsForAliasOptions *ListResourceBindingsForAliasOptions) (result *ResourceBindingsList, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.ListResourceBindingsForAliasWithContext(context.Background(), listResourceBindingsForAliasOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// ListResourceBindingsForAliasWithContext is an alternate form of the ListResourceBindingsForAlias method which supports a Context parameter
// Deprecated: this method is deprecated and may be removed in a future release.
func (resourceController *ResourceControllerV2) ListResourceBindingsForAliasWithContext(ctx context.Context, listResourceBindingsForAliasOptions *ListResourceBindingsForAliasOptions) (result *ResourceBindingsList, response *core.DetailedResponse, err error) {
	core.GetLogger().Warn("A deprecated operation has been invoked: ListResourceBindingsForAlias")
	err = core.ValidateNotNil(listResourceBindingsForAliasOptions, "listResourceBindingsForAliasOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(listResourceBindingsForAliasOptions, "listResourceBindingsForAliasOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id": *listResourceBindingsForAliasOptions.ID,
	}

	builder := core.NewRequestBuilder(core.GET)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v2/resource_aliases/{id}/resource_bindings`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range listResourceBindingsForAliasOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "ListResourceBindingsForAlias")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")

	if listResourceBindingsForAliasOptions.Limit != nil {
		builder.AddQuery("limit", fmt.Sprint(*listResourceBindingsForAliasOptions.Limit))
	}
	if listResourceBindingsForAliasOptions.Start != nil {
		builder.AddQuery("start", fmt.Sprint(*listResourceBindingsForAliasOptions.Start))
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "list_resource_bindings_for_alias", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalResourceBindingsList)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// ListReclamations : Get a list of all reclamations
// View all of the resource reclamations that exist for every resource instance.
func (resourceController *ResourceControllerV2) ListReclamations(listReclamationsOptions *ListReclamationsOptions) (result *ReclamationsList, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.ListReclamationsWithContext(context.Background(), listReclamationsOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// ListReclamationsWithContext is an alternate form of the ListReclamations method which supports a Context parameter
func (resourceController *ResourceControllerV2) ListReclamationsWithContext(ctx context.Context, listReclamationsOptions *ListReclamationsOptions) (result *ReclamationsList, response *core.DetailedResponse, err error) {
	err = core.ValidateStruct(listReclamationsOptions, "listReclamationsOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	builder := core.NewRequestBuilder(core.GET)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v1/reclamations`, nil)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range listReclamationsOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "ListReclamations")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")

	if listReclamationsOptions.AccountID != nil {
		builder.AddQuery("account_id", fmt.Sprint(*listReclamationsOptions.AccountID))
	}
	if listReclamationsOptions.ResourceInstanceID != nil {
		builder.AddQuery("resource_instance_id", fmt.Sprint(*listReclamationsOptions.ResourceInstanceID))
	}
	if listReclamationsOptions.ResourceGroupID != nil {
		builder.AddQuery("resource_group_id", fmt.Sprint(*listReclamationsOptions.ResourceGroupID))
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "list_reclamations", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalReclamationsList)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}

// RunReclamationAction : Perform a reclamation action
// Reclaim a resource instance so that it can no longer be used, or restore the resource instance so that it's usable
// again.
func (resourceController *ResourceControllerV2) RunReclamationAction(runReclamationActionOptions *RunReclamationActionOptions) (result *Reclamation, response *core.DetailedResponse, err error) {
	result, response, err = resourceController.RunReclamationActionWithContext(context.Background(), runReclamationActionOptions)
	err = core.RepurposeSDKProblem(err, "")
	return
}

// RunReclamationActionWithContext is an alternate form of the RunReclamationAction method which supports a Context parameter
func (resourceController *ResourceControllerV2) RunReclamationActionWithContext(ctx context.Context, runReclamationActionOptions *RunReclamationActionOptions) (result *Reclamation, response *core.DetailedResponse, err error) {
	err = core.ValidateNotNil(runReclamationActionOptions, "runReclamationActionOptions cannot be nil")
	if err != nil {
		err = core.SDKErrorf(err, "", "unexpected-nil-param", common.GetComponentInfo())
		return
	}
	err = core.ValidateStruct(runReclamationActionOptions, "runReclamationActionOptions")
	if err != nil {
		err = core.SDKErrorf(err, "", "struct-validation-error", common.GetComponentInfo())
		return
	}

	pathParamsMap := map[string]string{
		"id":          *runReclamationActionOptions.ID,
		"action_name": *runReclamationActionOptions.ActionName,
	}

	builder := core.NewRequestBuilder(core.POST)
	builder = builder.WithContext(ctx)
	builder.EnableGzipCompression = resourceController.GetEnableGzipCompression()
	_, err = builder.ResolveRequestURL(resourceController.Service.Options.URL, `/v1/reclamations/{id}/actions/{action_name}`, pathParamsMap)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-resolve-error", common.GetComponentInfo())
		return
	}

	for headerName, headerValue := range runReclamationActionOptions.Headers {
		builder.AddHeader(headerName, headerValue)
	}

	sdkHeaders := common.GetSdkHeaders("resource_controller", "V2", "RunReclamationAction")
	for headerName, headerValue := range sdkHeaders {
		builder.AddHeader(headerName, headerValue)
	}
	builder.AddHeader("Accept", "application/json")
	builder.AddHeader("Content-Type", "application/json")

	body := make(map[string]interface{})
	if runReclamationActionOptions.RequestBy != nil {
		body["request_by"] = runReclamationActionOptions.RequestBy
	}
	if runReclamationActionOptions.Comment != nil {
		body["comment"] = runReclamationActionOptions.Comment
	}
	_, err = builder.SetBodyContentJSON(body)
	if err != nil {
		err = core.SDKErrorf(err, "", "set-json-body-error", common.GetComponentInfo())
		return
	}

	request, err := builder.Build()
	if err != nil {
		err = core.SDKErrorf(err, "", "build-error", common.GetComponentInfo())
		return
	}

	var rawResponse map[string]json.RawMessage
	response, err = resourceController.Service.Request(request, &rawResponse)
	if err != nil {
		core.EnrichHTTPProblem(err, "run_reclamation_action", getServiceComponentInfo())
		err = core.SDKErrorf(err, "", "http-request-err", common.GetComponentInfo())
		return
	}
	if rawResponse != nil {
		err = core.UnmarshalModel(rawResponse, "", &result, UnmarshalReclamation)
		if err != nil {
			err = core.SDKErrorf(err, "", "unmarshal-resp-error", common.GetComponentInfo())
			return
		}
		response.Result = result
	}

	return
}
func getServiceComponentInfo() *core.ProblemComponent {
	return core.NewProblemComponent(DefaultServiceName, "2.0")
}

// CancelLastopResourceInstanceOptions : The CancelLastopResourceInstance options.
type CancelLastopResourceInstanceOptions struct {
	// The resource instance URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewCancelLastopResourceInstanceOptions : Instantiate CancelLastopResourceInstanceOptions
func (*ResourceControllerV2) NewCancelLastopResourceInstanceOptions(id string) *CancelLastopResourceInstanceOptions {
	return &CancelLastopResourceInstanceOptions{
		ID: core.StringPtr(id),
	}
}

// SetID : Allow user to set ID
func (_options *CancelLastopResourceInstanceOptions) SetID(id string) *CancelLastopResourceInstanceOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *CancelLastopResourceInstanceOptions) SetHeaders(param map[string]string) *CancelLastopResourceInstanceOptions {
	options.Headers = param
	return options
}

// CreateResourceAliasOptions : The CreateResourceAlias options.
type CreateResourceAliasOptions struct {
	// The name of the alias. Must be 180 characters or less and cannot include any special characters other than `(space)
	// - . _ :`.
	Name *string `json:"name" validate:"required"`

	// The ID of resource instance.
	Source *string `json:"source" validate:"required"`

	// The CRN of target name(space) in a specific environment, for example, space in Dallas YP, CFEE instance etc.
	Target *string `json:"target" validate:"required"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewCreateResourceAliasOptions : Instantiate CreateResourceAliasOptions
func (*ResourceControllerV2) NewCreateResourceAliasOptions(name string, source string, target string) *CreateResourceAliasOptions {
	return &CreateResourceAliasOptions{
		Name:   core.StringPtr(name),
		Source: core.StringPtr(source),
		Target: core.StringPtr(target),
	}
}

// SetName : Allow user to set Name
func (_options *CreateResourceAliasOptions) SetName(name string) *CreateResourceAliasOptions {
	_options.Name = core.StringPtr(name)
	return _options
}

// SetSource : Allow user to set Source
func (_options *CreateResourceAliasOptions) SetSource(source string) *CreateResourceAliasOptions {
	_options.Source = core.StringPtr(source)
	return _options
}

// SetTarget : Allow user to set Target
func (_options *CreateResourceAliasOptions) SetTarget(target string) *CreateResourceAliasOptions {
	_options.Target = core.StringPtr(target)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *CreateResourceAliasOptions) SetHeaders(param map[string]string) *CreateResourceAliasOptions {
	options.Headers = param
	return options
}

// CreateResourceBindingOptions : The CreateResourceBinding options.
type CreateResourceBindingOptions struct {
	// The ID of resource alias.
	Source *string `json:"source" validate:"required"`

	// The CRN of application to bind to in a specific environment, for example, Dallas YP, CFEE instance.
	Target *string `json:"target" validate:"required"`

	// The name of the binding. Must be 180 characters or less and cannot include any special characters other than
	// `(space) - . _ :`.
	Name *string `json:"name,omitempty"`

	// Configuration options represented as key-value pairs. Service defined options are passed through to the target
	// resource brokers, whereas platform defined options are not.
	Parameters *ResourceBindingPostParameters `json:"parameters,omitempty"`

	// The base IAM service role name (Reader, Writer, or Manager), or the service or custom role CRN. Refer to service’s
	// documentation for supported roles.
	Role *string `json:"role,omitempty"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewCreateResourceBindingOptions : Instantiate CreateResourceBindingOptions
func (*ResourceControllerV2) NewCreateResourceBindingOptions(source string, target string) *CreateResourceBindingOptions {
	return &CreateResourceBindingOptions{
		Source: core.StringPtr(source),
		Target: core.StringPtr(target),
	}
}

// SetSource : Allow user to set Source
func (_options *CreateResourceBindingOptions) SetSource(source string) *CreateResourceBindingOptions {
	_options.Source = core.StringPtr(source)
	return _options
}

// SetTarget : Allow user to set Target
func (_options *CreateResourceBindingOptions) SetTarget(target string) *CreateResourceBindingOptions {
	_options.Target = core.StringPtr(target)
	return _options
}

// SetName : Allow user to set Name
func (_options *CreateResourceBindingOptions) SetName(name string) *CreateResourceBindingOptions {
	_options.Name = core.StringPtr(name)
	return _options
}

// SetParameters : Allow user to set Parameters
func (_options *CreateResourceBindingOptions) SetParameters(parameters *ResourceBindingPostParameters) *CreateResourceBindingOptions {
	_options.Parameters = parameters
	return _options
}

// SetRole : Allow user to set Role
func (_options *CreateResourceBindingOptions) SetRole(role string) *CreateResourceBindingOptions {
	_options.Role = core.StringPtr(role)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *CreateResourceBindingOptions) SetHeaders(param map[string]string) *CreateResourceBindingOptions {
	options.Headers = param
	return options
}

// CreateResourceInstanceOptions : The CreateResourceInstance options.
type CreateResourceInstanceOptions struct {
	// The name of the instance. Must be 180 characters or less and cannot include any special characters other than
	// `(space) - . _ :`.
	Name *string `json:"name" validate:"required"`

	// The deployment location where the instance should be hosted.
	Target *string `json:"target" validate:"required"`

	// The ID of the resource group.
	ResourceGroup *string `json:"resource_group" validate:"required"`

	// The unique ID of the plan associated with the offering. This value is provided by and stored in the global catalog.
	ResourcePlanID *string `json:"resource_plan_id" validate:"required"`

	// Tags that are attached to the instance after provisioning. These tags can be searched and managed through the
	// Tagging API in IBM Cloud.
	Tags []string `json:"tags,omitempty"`

	// A boolean that dictates if the resource instance should be deleted (cleaned up) during the processing of a region
	// instance delete call.
	AllowCleanup *bool `json:"allow_cleanup,omitempty"`

	// Configuration options represented as key-value pairs that are passed through to the target resource brokers. Set the
	// `onetime_credentials` property to specify whether newly created resource key credentials can be retrieved by using
	// get resource key or get a list of all of the resource keys requests.
	Parameters map[string]interface{} `json:"parameters,omitempty"`

	// Indicates if the resource instance is locked for further update or delete operations. It does not affect actions
	// performed on child resources like aliases, bindings or keys. False by default.
	EntityLock *bool `json:"Entity-Lock,omitempty"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewCreateResourceInstanceOptions : Instantiate CreateResourceInstanceOptions
func (*ResourceControllerV2) NewCreateResourceInstanceOptions(name string, target string, resourceGroup string, resourcePlanID string) *CreateResourceInstanceOptions {
	return &CreateResourceInstanceOptions{
		Name:           core.StringPtr(name),
		Target:         core.StringPtr(target),
		ResourceGroup:  core.StringPtr(resourceGroup),
		ResourcePlanID: core.StringPtr(resourcePlanID),
	}
}

// SetName : Allow user to set Name
func (_options *CreateResourceInstanceOptions) SetName(name string) *CreateResourceInstanceOptions {
	_options.Name = core.StringPtr(name)
	return _options
}

// SetTarget : Allow user to set Target
func (_options *CreateResourceInstanceOptions) SetTarget(target string) *CreateResourceInstanceOptions {
	_options.Target = core.StringPtr(target)
	return _options
}

// SetResourceGroup : Allow user to set ResourceGroup
func (_options *CreateResourceInstanceOptions) SetResourceGroup(resourceGroup string) *CreateResourceInstanceOptions {
	_options.ResourceGroup = core.StringPtr(resourceGroup)
	return _options
}

// SetResourcePlanID : Allow user to set ResourcePlanID
func (_options *CreateResourceInstanceOptions) SetResourcePlanID(resourcePlanID string) *CreateResourceInstanceOptions {
	_options.ResourcePlanID = core.StringPtr(resourcePlanID)
	return _options
}

// SetTags : Allow user to set Tags
func (_options *CreateResourceInstanceOptions) SetTags(tags []string) *CreateResourceInstanceOptions {
	_options.Tags = tags
	return _options
}

// SetAllowCleanup : Allow user to set AllowCleanup
func (_options *CreateResourceInstanceOptions) SetAllowCleanup(allowCleanup bool) *CreateResourceInstanceOptions {
	_options.AllowCleanup = core.BoolPtr(allowCleanup)
	return _options
}

// SetParameters : Allow user to set Parameters
func (_options *CreateResourceInstanceOptions) SetParameters(parameters map[string]interface{}) *CreateResourceInstanceOptions {
	_options.Parameters = parameters
	return _options
}

// SetEntityLock : Allow user to set EntityLock
func (_options *CreateResourceInstanceOptions) SetEntityLock(entityLock bool) *CreateResourceInstanceOptions {
	_options.EntityLock = core.BoolPtr(entityLock)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *CreateResourceInstanceOptions) SetHeaders(param map[string]string) *CreateResourceInstanceOptions {
	options.Headers = param
	return options
}

// CreateResourceKeyOptions : The CreateResourceKey options.
type CreateResourceKeyOptions struct {
	// The name of the key.
	Name *string `json:"name" validate:"required"`

	// The ID of resource instance or alias.
	Source *string `json:"source" validate:"required"`

	// Configuration options represented as key-value pairs. Service defined options are passed through to the target
	// resource brokers, whereas platform defined options are not.
	Parameters *ResourceKeyPostParameters `json:"parameters,omitempty"`

	// The base IAM service role name (Reader, Writer, or Manager), or the service or custom role CRN. Refer to service’s
	// documentation for supported roles.
	Role *string `json:"role,omitempty"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewCreateResourceKeyOptions : Instantiate CreateResourceKeyOptions
func (*ResourceControllerV2) NewCreateResourceKeyOptions(name string, source string) *CreateResourceKeyOptions {
	return &CreateResourceKeyOptions{
		Name:   core.StringPtr(name),
		Source: core.StringPtr(source),
	}
}

// SetName : Allow user to set Name
func (_options *CreateResourceKeyOptions) SetName(name string) *CreateResourceKeyOptions {
	_options.Name = core.StringPtr(name)
	return _options
}

// SetSource : Allow user to set Source
func (_options *CreateResourceKeyOptions) SetSource(source string) *CreateResourceKeyOptions {
	_options.Source = core.StringPtr(source)
	return _options
}

// SetParameters : Allow user to set Parameters
func (_options *CreateResourceKeyOptions) SetParameters(parameters *ResourceKeyPostParameters) *CreateResourceKeyOptions {
	_options.Parameters = parameters
	return _options
}

// SetRole : Allow user to set Role
func (_options *CreateResourceKeyOptions) SetRole(role string) *CreateResourceKeyOptions {
	_options.Role = core.StringPtr(role)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *CreateResourceKeyOptions) SetHeaders(param map[string]string) *CreateResourceKeyOptions {
	options.Headers = param
	return options
}

// Credentials : The credentials for a resource.
// This type supports additional properties of type interface{}. Additional key-value pairs from the resource broker.
type Credentials struct {
	// If present, the user doesn't have the correct access to view the credentials and the details are redacted.  The
	// string value identifies the level of access that's required to view the credential. For additional information, see
	// [viewing a
	// credential](https://cloud.ibm.com/docs/account?topic=account-service_credentials&interface=ui#viewing-credentials-ui).
	Redacted *string `json:"REDACTED,omitempty"`

	// The API key for the credentials.
	Apikey *string `json:"apikey,omitempty"`

	// The optional description of the API key.
	IamApikeyDescription *string `json:"iam_apikey_description,omitempty"`

	// The name of the API key.
	IamApikeyName *string `json:"iam_apikey_name,omitempty"`

	// The Cloud Resource Name for the role of the credentials.
	IamRoleCRN *string `json:"iam_role_crn,omitempty"`

	// The Cloud Resource Name for the service ID of the credentials.
	IamServiceidCRN *string `json:"iam_serviceid_crn,omitempty"`

	// Additional key-value pairs from the resource broker.
	additionalProperties map[string]interface{}
}

// Constants associated with the Credentials.Redacted property.
// If present, the user doesn't have the correct access to view the credentials and the details are redacted.  The
// string value identifies the level of access that's required to view the credential. For additional information, see
// [viewing a
// credential](https://cloud.ibm.com/docs/account?topic=account-service_credentials&interface=ui#viewing-credentials-ui).
const (
	CredentialsRedactedRedactedConst         = "REDACTED"
	CredentialsRedactedRedactedExplicitConst = "REDACTED_EXPLICIT" // #nosec G101
)

// SetProperty allows the user to set an arbitrary property on an instance of Credentials.
// Additional key-value pairs from the resource broker.
func (o *Credentials) SetProperty(key string, value interface{}) {
	if o.additionalProperties == nil {
		o.additionalProperties = make(map[string]interface{})
	}
	o.additionalProperties[key] = value
}

// SetProperties allows the user to set a map of arbitrary properties on an instance of Credentials.
// Additional key-value pairs from the resource broker.
func (o *Credentials) SetProperties(m map[string]interface{}) {
	o.additionalProperties = make(map[string]interface{})
	for k, v := range m {
		o.additionalProperties[k] = v
	}
}

// GetProperty allows the user to retrieve an arbitrary property from an instance of Credentials.
func (o *Credentials) GetProperty(key string) interface{} {
	return o.additionalProperties[key]
}

// GetProperties allows the user to retrieve the map of arbitrary properties from an instance of Credentials.
func (o *Credentials) GetProperties() map[string]interface{} {
	return o.additionalProperties
}

// MarshalJSON performs custom serialization for instances of Credentials
func (o *Credentials) MarshalJSON() (buffer []byte, err error) {
	m := make(map[string]interface{})
	if len(o.additionalProperties) > 0 {
		for k, v := range o.additionalProperties {
			m[k] = v
		}
	}
	if o.Redacted != nil {
		m["REDACTED"] = o.Redacted
	}
	if o.Apikey != nil {
		m["apikey"] = o.Apikey
	}
	if o.IamApikeyDescription != nil {
		m["iam_apikey_description"] = o.IamApikeyDescription
	}
	if o.IamApikeyName != nil {
		m["iam_apikey_name"] = o.IamApikeyName
	}
	if o.IamRoleCRN != nil {
		m["iam_role_crn"] = o.IamRoleCRN
	}
	if o.IamServiceidCRN != nil {
		m["iam_serviceid_crn"] = o.IamServiceidCRN
	}
	buffer, err = json.Marshal(m)
	if err != nil {
		err = core.SDKErrorf(err, "", "model-marshal", common.GetComponentInfo())
	}
	return
}

// UnmarshalCredentials unmarshals an instance of Credentials from the specified map of raw messages.
func UnmarshalCredentials(m map[string]json.RawMessage, result interface{}) (err error) {
	obj := new(Credentials)
	err = core.UnmarshalPrimitive(m, "REDACTED", &obj.Redacted)
	if err != nil {
		err = core.SDKErrorf(err, "", "REDACTED-error", common.GetComponentInfo())
		return
	}
	delete(m, "REDACTED")
	err = core.UnmarshalPrimitive(m, "apikey", &obj.Apikey)
	if err != nil {
		err = core.SDKErrorf(err, "", "apikey-error", common.GetComponentInfo())
		return
	}
	delete(m, "apikey")
	err = core.UnmarshalPrimitive(m, "iam_apikey_description", &obj.IamApikeyDescription)
	if err != nil {
		err = core.SDKErrorf(err, "", "iam_apikey_description-error", common.GetComponentInfo())
		return
	}
	delete(m, "iam_apikey_description")
	err = core.UnmarshalPrimitive(m, "iam_apikey_name", &obj.IamApikeyName)
	if err != nil {
		err = core.SDKErrorf(err, "", "iam_apikey_name-error", common.GetComponentInfo())
		return
	}
	delete(m, "iam_apikey_name")
	err = core.UnmarshalPrimitive(m, "iam_role_crn", &obj.IamRoleCRN)
	if err != nil {
		err = core.SDKErrorf(err, "", "iam_role_crn-error", common.GetComponentInfo())
		return
	}
	delete(m, "iam_role_crn")
	err = core.UnmarshalPrimitive(m, "iam_serviceid_crn", &obj.IamServiceidCRN)
	if err != nil {
		err = core.SDKErrorf(err, "", "iam_serviceid_crn-error", common.GetComponentInfo())
		return
	}
	delete(m, "iam_serviceid_crn")
	for k := range m {
		var v interface{}
		e := core.UnmarshalPrimitive(m, k, &v)
		if e != nil {
			err = core.SDKErrorf(e, "", "additional-properties-error", common.GetComponentInfo())
			return
		}
		obj.SetProperty(k, v)
	}
	reflect.ValueOf(result).Elem().Set(reflect.ValueOf(obj))
	return
}

// DeleteResourceAliasOptions : The DeleteResourceAlias options.
type DeleteResourceAliasOptions struct {
	// The resource alias URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// Deletes the resource bindings and keys associated with the alias.
	Recursive *bool `json:"recursive,omitempty"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewDeleteResourceAliasOptions : Instantiate DeleteResourceAliasOptions
func (*ResourceControllerV2) NewDeleteResourceAliasOptions(id string) *DeleteResourceAliasOptions {
	return &DeleteResourceAliasOptions{
		ID: core.StringPtr(id),
	}
}

// SetID : Allow user to set ID
func (_options *DeleteResourceAliasOptions) SetID(id string) *DeleteResourceAliasOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetRecursive : Allow user to set Recursive
func (_options *DeleteResourceAliasOptions) SetRecursive(recursive bool) *DeleteResourceAliasOptions {
	_options.Recursive = core.BoolPtr(recursive)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *DeleteResourceAliasOptions) SetHeaders(param map[string]string) *DeleteResourceAliasOptions {
	options.Headers = param
	return options
}

// DeleteResourceBindingOptions : The DeleteResourceBinding options.
type DeleteResourceBindingOptions struct {
	// The resource binding URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewDeleteResourceBindingOptions : Instantiate DeleteResourceBindingOptions
func (*ResourceControllerV2) NewDeleteResourceBindingOptions(id string) *DeleteResourceBindingOptions {
	return &DeleteResourceBindingOptions{
		ID: core.StringPtr(id),
	}
}

// SetID : Allow user to set ID
func (_options *DeleteResourceBindingOptions) SetID(id string) *DeleteResourceBindingOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *DeleteResourceBindingOptions) SetHeaders(param map[string]string) *DeleteResourceBindingOptions {
	options.Headers = param
	return options
}

// DeleteResourceInstanceOptions : The DeleteResourceInstance options.
type DeleteResourceInstanceOptions struct {
	// The resource instance URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// Will delete resource bindings, keys and aliases associated with the instance.
	Recursive *bool `json:"recursive,omitempty"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewDeleteResourceInstanceOptions : Instantiate DeleteResourceInstanceOptions
func (*ResourceControllerV2) NewDeleteResourceInstanceOptions(id string) *DeleteResourceInstanceOptions {
	return &DeleteResourceInstanceOptions{
		ID: core.StringPtr(id),
	}
}

// SetID : Allow user to set ID
func (_options *DeleteResourceInstanceOptions) SetID(id string) *DeleteResourceInstanceOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetRecursive : Allow user to set Recursive
func (_options *DeleteResourceInstanceOptions) SetRecursive(recursive bool) *DeleteResourceInstanceOptions {
	_options.Recursive = core.BoolPtr(recursive)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *DeleteResourceInstanceOptions) SetHeaders(param map[string]string) *DeleteResourceInstanceOptions {
	options.Headers = param
	return options
}

// DeleteResourceKeyOptions : The DeleteResourceKey options.
type DeleteResourceKeyOptions struct {
	// The resource key URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewDeleteResourceKeyOptions : Instantiate DeleteResourceKeyOptions
func (*ResourceControllerV2) NewDeleteResourceKeyOptions(id string) *DeleteResourceKeyOptions {
	return &DeleteResourceKeyOptions{
		ID: core.StringPtr(id),
	}
}

// SetID : Allow user to set ID
func (_options *DeleteResourceKeyOptions) SetID(id string) *DeleteResourceKeyOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *DeleteResourceKeyOptions) SetHeaders(param map[string]string) *DeleteResourceKeyOptions {
	options.Headers = param
	return options
}

// GetResourceAliasOptions : The GetResourceAlias options.
type GetResourceAliasOptions struct {
	// The resource alias URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewGetResourceAliasOptions : Instantiate GetResourceAliasOptions
func (*ResourceControllerV2) NewGetResourceAliasOptions(id string) *GetResourceAliasOptions {
	return &GetResourceAliasOptions{
		ID: core.StringPtr(id),
	}
}

// SetID : Allow user to set ID
func (_options *GetResourceAliasOptions) SetID(id string) *GetResourceAliasOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *GetResourceAliasOptions) SetHeaders(param map[string]string) *GetResourceAliasOptions {
	options.Headers = param
	return options
}

// GetResourceBindingOptions : The GetResourceBinding options.
type GetResourceBindingOptions struct {
	// The resource binding URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewGetResourceBindingOptions : Instantiate GetResourceBindingOptions
func (*ResourceControllerV2) NewGetResourceBindingOptions(id string) *GetResourceBindingOptions {
	return &GetResourceBindingOptions{
		ID: core.StringPtr(id),
	}
}

// SetID : Allow user to set ID
func (_options *GetResourceBindingOptions) SetID(id string) *GetResourceBindingOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *GetResourceBindingOptions) SetHeaders(param map[string]string) *GetResourceBindingOptions {
	options.Headers = param
	return options
}

// GetResourceInstanceOptions : The GetResourceInstance options.
type GetResourceInstanceOptions struct {
	// The resource instance URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewGetResourceInstanceOptions : Instantiate GetResourceInstanceOptions
func (*ResourceControllerV2) NewGetResourceInstanceOptions(id string) *GetResourceInstanceOptions {
	return &GetResourceInstanceOptions{
		ID: core.StringPtr(id),
	}
}

// SetID : Allow user to set ID
func (_options *GetResourceInstanceOptions) SetID(id string) *GetResourceInstanceOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *GetResourceInstanceOptions) SetHeaders(param map[string]string) *GetResourceInstanceOptions {
	options.Headers = param
	return options
}

// GetResourceKeyOptions : The GetResourceKey options.
type GetResourceKeyOptions struct {
	// The resource key URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewGetResourceKeyOptions : Instantiate GetResourceKeyOptions
func (*ResourceControllerV2) NewGetResourceKeyOptions(id string) *GetResourceKeyOptions {
	return &GetResourceKeyOptions{
		ID: core.StringPtr(id),
	}
}

// SetID : Allow user to set ID
func (_options *GetResourceKeyOptions) SetID(id string) *GetResourceKeyOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *GetResourceKeyOptions) SetHeaders(param map[string]string) *GetResourceKeyOptions {
	options.Headers = param
	return options
}

// ListReclamationsOptions : The ListReclamations options.
type ListReclamationsOptions struct {
	// An alpha-numeric value identifying the account ID.
	AccountID *string `json:"account_id,omitempty"`

	// The GUID of the resource instance.
	ResourceInstanceID *string `json:"resource_instance_id,omitempty"`

	// The ID of the resource group.
	ResourceGroupID *string `json:"resource_group_id,omitempty"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewListReclamationsOptions : Instantiate ListReclamationsOptions
func (*ResourceControllerV2) NewListReclamationsOptions() *ListReclamationsOptions {
	return &ListReclamationsOptions{}
}

// SetAccountID : Allow user to set AccountID
func (_options *ListReclamationsOptions) SetAccountID(accountID string) *ListReclamationsOptions {
	_options.AccountID = core.StringPtr(accountID)
	return _options
}

// SetResourceInstanceID : Allow user to set ResourceInstanceID
func (_options *ListReclamationsOptions) SetResourceInstanceID(resourceInstanceID string) *ListReclamationsOptions {
	_options.ResourceInstanceID = core.StringPtr(resourceInstanceID)
	return _options
}

// SetResourceGroupID : Allow user to set ResourceGroupID
func (_options *ListReclamationsOptions) SetResourceGroupID(resourceGroupID string) *ListReclamationsOptions {
	_options.ResourceGroupID = core.StringPtr(resourceGroupID)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *ListReclamationsOptions) SetHeaders(param map[string]string) *ListReclamationsOptions {
	options.Headers = param
	return options
}

// ListResourceAliasesForInstanceOptions : The ListResourceAliasesForInstance options.
type ListResourceAliasesForInstanceOptions struct {
	// The resource instance URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// Limit on how many items should be returned.
	Limit *int64 `json:"limit,omitempty"`

	// An optional token that indicates the beginning of the page of results to be returned. Any additional query
	// parameters are ignored if a page token is present. If omitted, the first page of results is returned. This value is
	// obtained from the 'start' query parameter in the 'next_url' field of the operation response.
	Start *string `json:"start,omitempty"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewListResourceAliasesForInstanceOptions : Instantiate ListResourceAliasesForInstanceOptions
func (*ResourceControllerV2) NewListResourceAliasesForInstanceOptions(id string) *ListResourceAliasesForInstanceOptions {
	return &ListResourceAliasesForInstanceOptions{
		ID: core.StringPtr(id),
	}
}

// SetID : Allow user to set ID
func (_options *ListResourceAliasesForInstanceOptions) SetID(id string) *ListResourceAliasesForInstanceOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetLimit : Allow user to set Limit
func (_options *ListResourceAliasesForInstanceOptions) SetLimit(limit int64) *ListResourceAliasesForInstanceOptions {
	_options.Limit = core.Int64Ptr(limit)
	return _options
}

// SetStart : Allow user to set Start
func (_options *ListResourceAliasesForInstanceOptions) SetStart(start string) *ListResourceAliasesForInstanceOptions {
	_options.Start = core.StringPtr(start)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *ListResourceAliasesForInstanceOptions) SetHeaders(param map[string]string) *ListResourceAliasesForInstanceOptions {
	options.Headers = param
	return options
}

// ListResourceAliasesOptions : The ListResourceAliases options.
type ListResourceAliasesOptions struct {
	// The GUID of the alias.
	GUID *string `json:"guid,omitempty"`

	// The human-readable name of the alias.
	Name *string `json:"name,omitempty"`

	// The ID of the resource instance.
	ResourceInstanceID *string `json:"resource_instance_id,omitempty"`

	// The ID of the instance in the target environment. For example, `service_instance_id` in a given IBM Cloud
	// environment.
	RegionInstanceID *string `json:"region_instance_id,omitempty"`

	// The unique ID of the offering (service name). This value is provided by and stored in the global catalog.
	ResourceID *string `json:"resource_id,omitempty"`

	// The ID of the resource group.
	ResourceGroupID *string `json:"resource_group_id,omitempty"`

	// Limit on how many items should be returned.
	Limit *int64 `json:"limit,omitempty"`

	// An optional token that indicates the beginning of the page of results to be returned. Any additional query
	// parameters are ignored if a page token is present. If omitted, the first page of results is returned. This value is
	// obtained from the 'start' query parameter in the 'next_url' field of the operation response.
	Start *string `json:"start,omitempty"`

	// Start date inclusive filter.
	UpdatedFrom *string `json:"updated_from,omitempty"`

	// End date inclusive filter.
	UpdatedTo *string `json:"updated_to,omitempty"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewListResourceAliasesOptions : Instantiate ListResourceAliasesOptions
func (*ResourceControllerV2) NewListResourceAliasesOptions() *ListResourceAliasesOptions {
	return &ListResourceAliasesOptions{}
}

// SetGUID : Allow user to set GUID
func (_options *ListResourceAliasesOptions) SetGUID(guid string) *ListResourceAliasesOptions {
	_options.GUID = core.StringPtr(guid)
	return _options
}

// SetName : Allow user to set Name
func (_options *ListResourceAliasesOptions) SetName(name string) *ListResourceAliasesOptions {
	_options.Name = core.StringPtr(name)
	return _options
}

// SetResourceInstanceID : Allow user to set ResourceInstanceID
func (_options *ListResourceAliasesOptions) SetResourceInstanceID(resourceInstanceID string) *ListResourceAliasesOptions {
	_options.ResourceInstanceID = core.StringPtr(resourceInstanceID)
	return _options
}

// SetRegionInstanceID : Allow user to set RegionInstanceID
func (_options *ListResourceAliasesOptions) SetRegionInstanceID(regionInstanceID string) *ListResourceAliasesOptions {
	_options.RegionInstanceID = core.StringPtr(regionInstanceID)
	return _options
}

// SetResourceID : Allow user to set ResourceID
func (_options *ListResourceAliasesOptions) SetResourceID(resourceID string) *ListResourceAliasesOptions {
	_options.ResourceID = core.StringPtr(resourceID)
	return _options
}

// SetResourceGroupID : Allow user to set ResourceGroupID
func (_options *ListResourceAliasesOptions) SetResourceGroupID(resourceGroupID string) *ListResourceAliasesOptions {
	_options.ResourceGroupID = core.StringPtr(resourceGroupID)
	return _options
}

// SetLimit : Allow user to set Limit
func (_options *ListResourceAliasesOptions) SetLimit(limit int64) *ListResourceAliasesOptions {
	_options.Limit = core.Int64Ptr(limit)
	return _options
}

// SetStart : Allow user to set Start
func (_options *ListResourceAliasesOptions) SetStart(start string) *ListResourceAliasesOptions {
	_options.Start = core.StringPtr(start)
	return _options
}

// SetUpdatedFrom : Allow user to set UpdatedFrom
func (_options *ListResourceAliasesOptions) SetUpdatedFrom(updatedFrom string) *ListResourceAliasesOptions {
	_options.UpdatedFrom = core.StringPtr(updatedFrom)
	return _options
}

// SetUpdatedTo : Allow user to set UpdatedTo
func (_options *ListResourceAliasesOptions) SetUpdatedTo(updatedTo string) *ListResourceAliasesOptions {
	_options.UpdatedTo = core.StringPtr(updatedTo)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *ListResourceAliasesOptions) SetHeaders(param map[string]string) *ListResourceAliasesOptions {
	options.Headers = param
	return options
}

// ListResourceBindingsForAliasOptions : The ListResourceBindingsForAlias options.
type ListResourceBindingsForAliasOptions struct {
	// The resource alias URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// Limit on how many items should be returned.
	Limit *int64 `json:"limit,omitempty"`

	// An optional token that indicates the beginning of the page of results to be returned. Any additional query
	// parameters are ignored if a page token is present. If omitted, the first page of results is returned. This value is
	// obtained from the 'start' query parameter in the 'next_url' field of the operation response.
	Start *string `json:"start,omitempty"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewListResourceBindingsForAliasOptions : Instantiate ListResourceBindingsForAliasOptions
func (*ResourceControllerV2) NewListResourceBindingsForAliasOptions(id string) *ListResourceBindingsForAliasOptions {
	return &ListResourceBindingsForAliasOptions{
		ID: core.StringPtr(id),
	}
}

// SetID : Allow user to set ID
func (_options *ListResourceBindingsForAliasOptions) SetID(id string) *ListResourceBindingsForAliasOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetLimit : Allow user to set Limit
func (_options *ListResourceBindingsForAliasOptions) SetLimit(limit int64) *ListResourceBindingsForAliasOptions {
	_options.Limit = core.Int64Ptr(limit)
	return _options
}

// SetStart : Allow user to set Start
func (_options *ListResourceBindingsForAliasOptions) SetStart(start string) *ListResourceBindingsForAliasOptions {
	_options.Start = core.StringPtr(start)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *ListResourceBindingsForAliasOptions) SetHeaders(param map[string]string) *ListResourceBindingsForAliasOptions {
	options.Headers = param
	return options
}

// ListResourceBindingsOptions : The ListResourceBindings options.
type ListResourceBindingsOptions struct {
	// The GUID of the binding.
	GUID *string `json:"guid,omitempty"`

	// The human-readable name of the binding.
	Name *string `json:"name,omitempty"`

	// The ID of the resource group.
	ResourceGroupID *string `json:"resource_group_id,omitempty"`

	// The unique ID of the offering (service name). This value is provided by and stored in the global catalog.
	ResourceID *string `json:"resource_id,omitempty"`

	// The ID of the binding in the target environment. For example, `service_binding_id` in a given IBM Cloud environment.
	RegionBindingID *string `json:"region_binding_id,omitempty"`

	// Limit on how many items should be returned.
	Limit *int64 `json:"limit,omitempty"`

	// An optional token that indicates the beginning of the page of results to be returned. Any additional query
	// parameters are ignored if a page token is present. If omitted, the first page of results is returned. This value is
	// obtained from the 'start' query parameter in the 'next_url' field of the operation response.
	Start *string `json:"start,omitempty"`

	// Start date inclusive filter.
	UpdatedFrom *string `json:"updated_from,omitempty"`

	// End date inclusive filter.
	UpdatedTo *string `json:"updated_to,omitempty"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewListResourceBindingsOptions : Instantiate ListResourceBindingsOptions
func (*ResourceControllerV2) NewListResourceBindingsOptions() *ListResourceBindingsOptions {
	return &ListResourceBindingsOptions{}
}

// SetGUID : Allow user to set GUID
func (_options *ListResourceBindingsOptions) SetGUID(guid string) *ListResourceBindingsOptions {
	_options.GUID = core.StringPtr(guid)
	return _options
}

// SetName : Allow user to set Name
func (_options *ListResourceBindingsOptions) SetName(name string) *ListResourceBindingsOptions {
	_options.Name = core.StringPtr(name)
	return _options
}

// SetResourceGroupID : Allow user to set ResourceGroupID
func (_options *ListResourceBindingsOptions) SetResourceGroupID(resourceGroupID string) *ListResourceBindingsOptions {
	_options.ResourceGroupID = core.StringPtr(resourceGroupID)
	return _options
}

// SetResourceID : Allow user to set ResourceID
func (_options *ListResourceBindingsOptions) SetResourceID(resourceID string) *ListResourceBindingsOptions {
	_options.ResourceID = core.StringPtr(resourceID)
	return _options
}

// SetRegionBindingID : Allow user to set RegionBindingID
func (_options *ListResourceBindingsOptions) SetRegionBindingID(regionBindingID string) *ListResourceBindingsOptions {
	_options.RegionBindingID = core.StringPtr(regionBindingID)
	return _options
}

// SetLimit : Allow user to set Limit
func (_options *ListResourceBindingsOptions) SetLimit(limit int64) *ListResourceBindingsOptions {
	_options.Limit = core.Int64Ptr(limit)
	return _options
}

// SetStart : Allow user to set Start
func (_options *ListResourceBindingsOptions) SetStart(start string) *ListResourceBindingsOptions {
	_options.Start = core.StringPtr(start)
	return _options
}

// SetUpdatedFrom : Allow user to set UpdatedFrom
func (_options *ListResourceBindingsOptions) SetUpdatedFrom(updatedFrom string) *ListResourceBindingsOptions {
	_options.UpdatedFrom = core.StringPtr(updatedFrom)
	return _options
}

// SetUpdatedTo : Allow user to set UpdatedTo
func (_options *ListResourceBindingsOptions) SetUpdatedTo(updatedTo string) *ListResourceBindingsOptions {
	_options.UpdatedTo = core.StringPtr(updatedTo)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *ListResourceBindingsOptions) SetHeaders(param map[string]string) *ListResourceBindingsOptions {
	options.Headers = param
	return options
}

// ListResourceInstancesOptions : The ListResourceInstances options.
type ListResourceInstancesOptions struct {
	// The GUID of the instance.
	GUID *string `json:"guid,omitempty"`

	// The human-readable name of the instance.
	Name *string `json:"name,omitempty"`

	// The ID of the resource group.
	ResourceGroupID *string `json:"resource_group_id,omitempty"`

	// The unique ID of the offering. This value is provided by and stored in the global catalog.
	ResourceID *string `json:"resource_id,omitempty"`

	// The unique ID of the plan associated with the offering. This value is provided by and stored in the global catalog.
	ResourcePlanID *string `json:"resource_plan_id,omitempty"`

	// The type of the instance, for example, `service_instance`.
	Type *string `json:"type,omitempty"`

	// The sub-type of instance, for example, `kms`.
	SubType *string `json:"sub_type,omitempty"`

	// Limit on how many items should be returned.
	Limit *int64 `json:"limit,omitempty"`

	// An optional token that indicates the beginning of the page of results to be returned. Any additional query
	// parameters are ignored if a page token is present. If omitted, the first page of results is returned. This value is
	// obtained from the 'start' query parameter in the 'next_url' field of the operation response.
	Start *string `json:"start,omitempty"`

	// The state of the instance. If not specified, instances in state `active` and `provisioning` are returned.
	State *string `json:"state,omitempty"`

	// Start date inclusive filter.
	UpdatedFrom *string `json:"updated_from,omitempty"`

	// End date inclusive filter.
	UpdatedTo *string `json:"updated_to,omitempty"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// Constants associated with the ListResourceInstancesOptions.State property.
// The state of the instance. If not specified, instances in state `active` and `provisioning` are returned.
const (
	ListResourceInstancesOptionsStateActiveConst             = "active"
	ListResourceInstancesOptionsStateFailedConst             = "failed"
	ListResourceInstancesOptionsStateInactiveConst           = "inactive"
	ListResourceInstancesOptionsStatePendingReclamationConst = "pending_reclamation"
	ListResourceInstancesOptionsStatePreProvisioningConst    = "pre_provisioning"
	ListResourceInstancesOptionsStateProvisioningConst       = "provisioning"
	ListResourceInstancesOptionsStateRemovedConst            = "removed"
)

// NewListResourceInstancesOptions : Instantiate ListResourceInstancesOptions
func (*ResourceControllerV2) NewListResourceInstancesOptions() *ListResourceInstancesOptions {
	return &ListResourceInstancesOptions{}
}

// SetGUID : Allow user to set GUID
func (_options *ListResourceInstancesOptions) SetGUID(guid string) *ListResourceInstancesOptions {
	_options.GUID = core.StringPtr(guid)
	return _options
}

// SetName : Allow user to set Name
func (_options *ListResourceInstancesOptions) SetName(name string) *ListResourceInstancesOptions {
	_options.Name = core.StringPtr(name)
	return _options
}

// SetResourceGroupID : Allow user to set ResourceGroupID
func (_options *ListResourceInstancesOptions) SetResourceGroupID(resourceGroupID string) *ListResourceInstancesOptions {
	_options.ResourceGroupID = core.StringPtr(resourceGroupID)
	return _options
}

// SetResourceID : Allow user to set ResourceID
func (_options *ListResourceInstancesOptions) SetResourceID(resourceID string) *ListResourceInstancesOptions {
	_options.ResourceID = core.StringPtr(resourceID)
	return _options
}

// SetResourcePlanID : Allow user to set ResourcePlanID
func (_options *ListResourceInstancesOptions) SetResourcePlanID(resourcePlanID string) *ListResourceInstancesOptions {
	_options.ResourcePlanID = core.StringPtr(resourcePlanID)
	return _options
}

// SetType : Allow user to set Type
func (_options *ListResourceInstancesOptions) SetType(typeVar string) *ListResourceInstancesOptions {
	_options.Type = core.StringPtr(typeVar)
	return _options
}

// SetSubType : Allow user to set SubType
func (_options *ListResourceInstancesOptions) SetSubType(subType string) *ListResourceInstancesOptions {
	_options.SubType = core.StringPtr(subType)
	return _options
}

// SetLimit : Allow user to set Limit
func (_options *ListResourceInstancesOptions) SetLimit(limit int64) *ListResourceInstancesOptions {
	_options.Limit = core.Int64Ptr(limit)
	return _options
}

// SetStart : Allow user to set Start
func (_options *ListResourceInstancesOptions) SetStart(start string) *ListResourceInstancesOptions {
	_options.Start = core.StringPtr(start)
	return _options
}

// SetState : Allow user to set State
func (_options *ListResourceInstancesOptions) SetState(state string) *ListResourceInstancesOptions {
	_options.State = core.StringPtr(state)
	return _options
}

// SetUpdatedFrom : Allow user to set UpdatedFrom
func (_options *ListResourceInstancesOptions) SetUpdatedFrom(updatedFrom string) *ListResourceInstancesOptions {
	_options.UpdatedFrom = core.StringPtr(updatedFrom)
	return _options
}

// SetUpdatedTo : Allow user to set UpdatedTo
func (_options *ListResourceInstancesOptions) SetUpdatedTo(updatedTo string) *ListResourceInstancesOptions {
	_options.UpdatedTo = core.StringPtr(updatedTo)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *ListResourceInstancesOptions) SetHeaders(param map[string]string) *ListResourceInstancesOptions {
	options.Headers = param
	return options
}

// ListResourceKeysForInstanceOptions : The ListResourceKeysForInstance options.
type ListResourceKeysForInstanceOptions struct {
	// The resource instance URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// Limit on how many items should be returned.
	Limit *int64 `json:"limit,omitempty"`

	// An optional token that indicates the beginning of the page of results to be returned. Any additional query
	// parameters are ignored if a page token is present. If omitted, the first page of results is returned. This value is
	// obtained from the 'start' query parameter in the 'next_url' field of the operation response.
	Start *string `json:"start,omitempty"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewListResourceKeysForInstanceOptions : Instantiate ListResourceKeysForInstanceOptions
func (*ResourceControllerV2) NewListResourceKeysForInstanceOptions(id string) *ListResourceKeysForInstanceOptions {
	return &ListResourceKeysForInstanceOptions{
		ID: core.StringPtr(id),
	}
}

// SetID : Allow user to set ID
func (_options *ListResourceKeysForInstanceOptions) SetID(id string) *ListResourceKeysForInstanceOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetLimit : Allow user to set Limit
func (_options *ListResourceKeysForInstanceOptions) SetLimit(limit int64) *ListResourceKeysForInstanceOptions {
	_options.Limit = core.Int64Ptr(limit)
	return _options
}

// SetStart : Allow user to set Start
func (_options *ListResourceKeysForInstanceOptions) SetStart(start string) *ListResourceKeysForInstanceOptions {
	_options.Start = core.StringPtr(start)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *ListResourceKeysForInstanceOptions) SetHeaders(param map[string]string) *ListResourceKeysForInstanceOptions {
	options.Headers = param
	return options
}

// ListResourceKeysOptions : The ListResourceKeys options.
type ListResourceKeysOptions struct {
	// The GUID of the key.
	GUID *string `json:"guid,omitempty"`

	// The human-readable name of the key.
	Name *string `json:"name,omitempty"`

	// The ID of the resource group.
	ResourceGroupID *string `json:"resource_group_id,omitempty"`

	// The unique ID of the offering. This value is provided by and stored in the global catalog.
	ResourceID *string `json:"resource_id,omitempty"`

	// Limit on how many items should be returned.
	Limit *int64 `json:"limit,omitempty"`

	// An optional token that indicates the beginning of the page of results to be returned. Any additional query
	// parameters are ignored if a page token is present. If omitted, the first page of results is returned. This value is
	// obtained from the 'start' query parameter in the 'next_url' field of the operation response.
	Start *string `json:"start,omitempty"`

	// Start date inclusive filter.
	UpdatedFrom *string `json:"updated_from,omitempty"`

	// End date inclusive filter.
	UpdatedTo *string `json:"updated_to,omitempty"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewListResourceKeysOptions : Instantiate ListResourceKeysOptions
func (*ResourceControllerV2) NewListResourceKeysOptions() *ListResourceKeysOptions {
	return &ListResourceKeysOptions{}
}

// SetGUID : Allow user to set GUID
func (_options *ListResourceKeysOptions) SetGUID(guid string) *ListResourceKeysOptions {
	_options.GUID = core.StringPtr(guid)
	return _options
}

// SetName : Allow user to set Name
func (_options *ListResourceKeysOptions) SetName(name string) *ListResourceKeysOptions {
	_options.Name = core.StringPtr(name)
	return _options
}

// SetResourceGroupID : Allow user to set ResourceGroupID
func (_options *ListResourceKeysOptions) SetResourceGroupID(resourceGroupID string) *ListResourceKeysOptions {
	_options.ResourceGroupID = core.StringPtr(resourceGroupID)
	return _options
}

// SetResourceID : Allow user to set ResourceID
func (_options *ListResourceKeysOptions) SetResourceID(resourceID string) *ListResourceKeysOptions {
	_options.ResourceID = core.StringPtr(resourceID)
	return _options
}

// SetLimit : Allow user to set Limit
func (_options *ListResourceKeysOptions) SetLimit(limit int64) *ListResourceKeysOptions {
	_options.Limit = core.Int64Ptr(limit)
	return _options
}

// SetStart : Allow user to set Start
func (_options *ListResourceKeysOptions) SetStart(start string) *ListResourceKeysOptions {
	_options.Start = core.StringPtr(start)
	return _options
}

// SetUpdatedFrom : Allow user to set UpdatedFrom
func (_options *ListResourceKeysOptions) SetUpdatedFrom(updatedFrom string) *ListResourceKeysOptions {
	_options.UpdatedFrom = core.StringPtr(updatedFrom)
	return _options
}

// SetUpdatedTo : Allow user to set UpdatedTo
func (_options *ListResourceKeysOptions) SetUpdatedTo(updatedTo string) *ListResourceKeysOptions {
	_options.UpdatedTo = core.StringPtr(updatedTo)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *ListResourceKeysOptions) SetHeaders(param map[string]string) *ListResourceKeysOptions {
	options.Headers = param
	return options
}

// LockResourceInstanceOptions : The LockResourceInstance options.
type LockResourceInstanceOptions struct {
	// The resource instance URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewLockResourceInstanceOptions : Instantiate LockResourceInstanceOptions
func (*ResourceControllerV2) NewLockResourceInstanceOptions(id string) *LockResourceInstanceOptions {
	return &LockResourceInstanceOptions{
		ID: core.StringPtr(id),
	}
}

// SetID : Allow user to set ID
func (_options *LockResourceInstanceOptions) SetID(id string) *LockResourceInstanceOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *LockResourceInstanceOptions) SetHeaders(param map[string]string) *LockResourceInstanceOptions {
	options.Headers = param
	return options
}

// PlanHistoryItem : An element of the plan history of the instance.
type PlanHistoryItem struct {
	// The unique ID of the plan associated with the offering. This value is provided by and stored in the global catalog.
	ResourcePlanID *string `json:"resource_plan_id" validate:"required"`

	// The date on which the plan was changed.
	StartDate *strfmt.DateTime `json:"start_date" validate:"required"`

	// The subject who made the plan change.
	RequestorID *string `json:"requestor_id,omitempty"`
}

// UnmarshalPlanHistoryItem unmarshals an instance of PlanHistoryItem from the specified map of raw messages.
func UnmarshalPlanHistoryItem(m map[string]json.RawMessage, result interface{}) (err error) {
	obj := new(PlanHistoryItem)
	err = core.UnmarshalPrimitive(m, "resource_plan_id", &obj.ResourcePlanID)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_plan_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "start_date", &obj.StartDate)
	if err != nil {
		err = core.SDKErrorf(err, "", "start_date-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "requestor_id", &obj.RequestorID)
	if err != nil {
		err = core.SDKErrorf(err, "", "requestor_id-error", common.GetComponentInfo())
		return
	}
	reflect.ValueOf(result).Elem().Set(reflect.ValueOf(obj))
	return
}

// Reclamation : A reclamation.
type Reclamation struct {
	// The ID associated with the reclamation.
	ID *string `json:"id,omitempty"`

	// The ID of the entity for the reclamation.
	EntityID *string `json:"entity_id,omitempty"`

	// The ID of the entity type for the reclamation.
	EntityTypeID *string `json:"entity_type_id,omitempty"`

	// The full Cloud Resource Name (CRN) associated with the binding. For more information about this format, see [Cloud
	// Resource Names](https://cloud.ibm.com/docs/overview?topic=overview-crn).
	EntityCRN *string `json:"entity_crn,omitempty"`

	// The ID of the resource instance.
	ResourceInstanceID *string `json:"resource_instance_id,omitempty"`

	// The ID of the resource group.
	ResourceGroupID *string `json:"resource_group_id,omitempty"`

	// An alpha-numeric value identifying the account ID.
	AccountID *string `json:"account_id,omitempty"`

	// The ID of policy for the reclamation.
	PolicyID *string `json:"policy_id,omitempty"`

	// The state of the reclamation.
	State *string `json:"state,omitempty"`

	// The target time that the reclamation retention period end.
	TargetTime *string `json:"target_time,omitempty"`

	// The custom properties of the reclamation.
	CustomProperties map[string]interface{} `json:"custom_properties,omitempty"`

	// The date when the reclamation was created.
	CreatedAt *strfmt.DateTime `json:"created_at,omitempty"`

	// The subject who created the reclamation.
	CreatedBy *string `json:"created_by,omitempty"`

	// The date when the reclamation was last updated.
	UpdatedAt *strfmt.DateTime `json:"updated_at,omitempty"`

	// The subject who updated the reclamation.
	UpdatedBy *string `json:"updated_by,omitempty"`
}

// UnmarshalReclamation unmarshals an instance of Reclamation from the specified map of raw messages.
func UnmarshalReclamation(m map[string]json.RawMessage, result interface{}) (err error) {
	obj := new(Reclamation)
	err = core.UnmarshalPrimitive(m, "id", &obj.ID)
	if err != nil {
		err = core.SDKErrorf(err, "", "id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "entity_id", &obj.EntityID)
	if err != nil {
		err = core.SDKErrorf(err, "", "entity_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "entity_type_id", &obj.EntityTypeID)
	if err != nil {
		err = core.SDKErrorf(err, "", "entity_type_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "entity_crn", &obj.EntityCRN)
	if err != nil {
		err = core.SDKErrorf(err, "", "entity_crn-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_instance_id", &obj.ResourceInstanceID)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_instance_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_group_id", &obj.ResourceGroupID)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_group_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "account_id", &obj.AccountID)
	if err != nil {
		err = core.SDKErrorf(err, "", "account_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "policy_id", &obj.PolicyID)
	if err != nil {
		err = core.SDKErrorf(err, "", "policy_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "state", &obj.State)
	if err != nil {
		err = core.SDKErrorf(err, "", "state-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "target_time", &obj.TargetTime)
	if err != nil {
		err = core.SDKErrorf(err, "", "target_time-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "custom_properties", &obj.CustomProperties)
	if err != nil {
		err = core.SDKErrorf(err, "", "custom_properties-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "created_at", &obj.CreatedAt)
	if err != nil {
		err = core.SDKErrorf(err, "", "created_at-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "created_by", &obj.CreatedBy)
	if err != nil {
		err = core.SDKErrorf(err, "", "created_by-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "updated_at", &obj.UpdatedAt)
	if err != nil {
		err = core.SDKErrorf(err, "", "updated_at-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "updated_by", &obj.UpdatedBy)
	if err != nil {
		err = core.SDKErrorf(err, "", "updated_by-error", common.GetComponentInfo())
		return
	}
	reflect.ValueOf(result).Elem().Set(reflect.ValueOf(obj))
	return
}

// ReclamationsList : A list of reclamations.
type ReclamationsList struct {
	// A list of reclamations.
	Resources []Reclamation `json:"resources,omitempty"`
}

// UnmarshalReclamationsList unmarshals an instance of ReclamationsList from the specified map of raw messages.
func UnmarshalReclamationsList(m map[string]json.RawMessage, result interface{}) (err error) {
	obj := new(ReclamationsList)
	err = core.UnmarshalModel(m, "resources", &obj.Resources, UnmarshalReclamation)
	if err != nil {
		err = core.SDKErrorf(err, "", "resources-error", common.GetComponentInfo())
		return
	}
	reflect.ValueOf(result).Elem().Set(reflect.ValueOf(obj))
	return
}

// ResourceAlias : A resource alias.
type ResourceAlias struct {
	// The ID associated with the alias.
	ID *string `json:"id,omitempty"`

	// The GUID of the alias.
	GUID *string `json:"guid,omitempty"`

	// When you created a new alias, a relative URL path is created identifying the location of the alias.
	URL *string `json:"url,omitempty"`

	// The date when the alias was created.
	CreatedAt *strfmt.DateTime `json:"created_at,omitempty"`

	// The date when the alias was last updated.
	UpdatedAt *strfmt.DateTime `json:"updated_at,omitempty"`

	// The date when the alias was deleted.
	DeletedAt *strfmt.DateTime `json:"deleted_at,omitempty"`

	// The subject who created the alias.
	CreatedBy *string `json:"created_by,omitempty"`

	// The subject who updated the alias.
	UpdatedBy *string `json:"updated_by,omitempty"`

	// The subject who deleted the alias.
	DeletedBy *string `json:"deleted_by,omitempty"`

	// The human-readable name of the alias.
	Name *string `json:"name,omitempty"`

	// The ID of the resource instance that is being aliased.
	ResourceInstanceID *string `json:"resource_instance_id,omitempty"`

	// The CRN of the target namespace in the specific environment.
	TargetCRN *string `json:"target_crn,omitempty"`

	// An alpha-numeric value identifying the account ID.
	AccountID *string `json:"account_id,omitempty"`

	// The unique ID of the offering. This value is provided by and stored in the global catalog.
	ResourceID *string `json:"resource_id,omitempty"`

	// The ID of the resource group.
	ResourceGroupID *string `json:"resource_group_id,omitempty"`

	// The CRN of the alias. For more information about this format, see [Cloud Resource
	// Names](https://cloud.ibm.com/docs/overview?topic=overview-crn).
	CRN *string `json:"crn,omitempty"`

	// The ID of the instance in the target environment. For example, `service_instance_id` in a given IBM Cloud
	// environment.
	RegionInstanceID *string `json:"region_instance_id,omitempty"`

	// The CRN of the instance in the target environment.
	RegionInstanceCRN *string `json:"region_instance_crn,omitempty"`

	// The state of the alias.
	State *string `json:"state,omitempty"`

	// A boolean that dictates if the alias was migrated from a previous CF instance.
	Migrated *bool `json:"migrated,omitempty"`

	// The relative path to the resource instance.
	ResourceInstanceURL *string `json:"resource_instance_url,omitempty"`

	// The relative path to the resource bindings for the alias.
	ResourceBindingsURL *string `json:"resource_bindings_url,omitempty"`

	// The relative path to the resource keys for the alias.
	ResourceKeysURL *string `json:"resource_keys_url,omitempty"`
}

// UnmarshalResourceAlias unmarshals an instance of ResourceAlias from the specified map of raw messages.
func UnmarshalResourceAlias(m map[string]json.RawMessage, result interface{}) (err error) {
	obj := new(ResourceAlias)
	err = core.UnmarshalPrimitive(m, "id", &obj.ID)
	if err != nil {
		err = core.SDKErrorf(err, "", "id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "guid", &obj.GUID)
	if err != nil {
		err = core.SDKErrorf(err, "", "guid-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "url", &obj.URL)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "created_at", &obj.CreatedAt)
	if err != nil {
		err = core.SDKErrorf(err, "", "created_at-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "updated_at", &obj.UpdatedAt)
	if err != nil {
		err = core.SDKErrorf(err, "", "updated_at-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "deleted_at", &obj.DeletedAt)
	if err != nil {
		err = core.SDKErrorf(err, "", "deleted_at-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "created_by", &obj.CreatedBy)
	if err != nil {
		err = core.SDKErrorf(err, "", "created_by-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "updated_by", &obj.UpdatedBy)
	if err != nil {
		err = core.SDKErrorf(err, "", "updated_by-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "deleted_by", &obj.DeletedBy)
	if err != nil {
		err = core.SDKErrorf(err, "", "deleted_by-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "name", &obj.Name)
	if err != nil {
		err = core.SDKErrorf(err, "", "name-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_instance_id", &obj.ResourceInstanceID)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_instance_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "target_crn", &obj.TargetCRN)
	if err != nil {
		err = core.SDKErrorf(err, "", "target_crn-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "account_id", &obj.AccountID)
	if err != nil {
		err = core.SDKErrorf(err, "", "account_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_id", &obj.ResourceID)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_group_id", &obj.ResourceGroupID)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_group_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "crn", &obj.CRN)
	if err != nil {
		err = core.SDKErrorf(err, "", "crn-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "region_instance_id", &obj.RegionInstanceID)
	if err != nil {
		err = core.SDKErrorf(err, "", "region_instance_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "region_instance_crn", &obj.RegionInstanceCRN)
	if err != nil {
		err = core.SDKErrorf(err, "", "region_instance_crn-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "state", &obj.State)
	if err != nil {
		err = core.SDKErrorf(err, "", "state-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "migrated", &obj.Migrated)
	if err != nil {
		err = core.SDKErrorf(err, "", "migrated-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_instance_url", &obj.ResourceInstanceURL)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_instance_url-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_bindings_url", &obj.ResourceBindingsURL)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_bindings_url-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_keys_url", &obj.ResourceKeysURL)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_keys_url-error", common.GetComponentInfo())
		return
	}
	reflect.ValueOf(result).Elem().Set(reflect.ValueOf(obj))
	return
}

// ResourceAliasesList : A list of resource aliases.
type ResourceAliasesList struct {
	// The number of resource aliases in `resources`.
	RowsCount *int64 `json:"rows_count" validate:"required"`

	// The URL for requesting the next page of results.
	NextURL *string `json:"next_url" validate:"required"`

	// A list of resource aliases.
	Resources []ResourceAlias `json:"resources" validate:"required"`
}

// UnmarshalResourceAliasesList unmarshals an instance of ResourceAliasesList from the specified map of raw messages.
func UnmarshalResourceAliasesList(m map[string]json.RawMessage, result interface{}) (err error) {
	obj := new(ResourceAliasesList)
	err = core.UnmarshalPrimitive(m, "rows_count", &obj.RowsCount)
	if err != nil {
		err = core.SDKErrorf(err, "", "rows_count-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "next_url", &obj.NextURL)
	if err != nil {
		err = core.SDKErrorf(err, "", "next_url-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalModel(m, "resources", &obj.Resources, UnmarshalResourceAlias)
	if err != nil {
		err = core.SDKErrorf(err, "", "resources-error", common.GetComponentInfo())
		return
	}
	reflect.ValueOf(result).Elem().Set(reflect.ValueOf(obj))
	return
}

// Retrieve the value to be passed to a request to access the next page of results
func (resp *ResourceAliasesList) GetNextStart() (*string, error) {
	if core.IsNil(resp.NextURL) {
		return nil, nil
	}
	start, err := core.GetQueryParam(resp.NextURL, "start")
	if err != nil {
		err = core.SDKErrorf(err, "", "read-query-param-error", common.GetComponentInfo())
		return nil, err
	} else if start == nil {
		return nil, nil
	}
	return start, nil
}

// ResourceBinding : A resource binding.
type ResourceBinding struct {
	// The ID associated with the binding.
	ID *string `json:"id,omitempty"`

	// The GUID of the binding.
	GUID *string `json:"guid,omitempty"`

	// When you provision a new binding, a relative URL path is created identifying the location of the binding.
	URL *string `json:"url,omitempty"`

	// The date when the binding was created.
	CreatedAt *strfmt.DateTime `json:"created_at,omitempty"`

	// The date when the binding was last updated.
	UpdatedAt *strfmt.DateTime `json:"updated_at,omitempty"`

	// The date when the binding was deleted.
	DeletedAt *strfmt.DateTime `json:"deleted_at,omitempty"`

	// The subject who created the binding.
	CreatedBy *string `json:"created_by,omitempty"`

	// The subject who updated the binding.
	UpdatedBy *string `json:"updated_by,omitempty"`

	// The subject who deleted the binding.
	DeletedBy *string `json:"deleted_by,omitempty"`

	// The CRN of resource alias associated to the binding.
	SourceCRN *string `json:"source_crn,omitempty"`

	// The CRN of target resource, for example, application, in a specific environment.
	TargetCRN *string `json:"target_crn,omitempty"`

	// The full Cloud Resource Name (CRN) associated with the binding. For more information about this format, see [Cloud
	// Resource Names](https://cloud.ibm.com/docs/overview?topic=overview-crn).
	CRN *string `json:"crn,omitempty"`

	// The ID of the binding in the target environment. For example, `service_binding_id` in a given IBM Cloud environment.
	RegionBindingID *string `json:"region_binding_id,omitempty"`

	// The CRN of the binding in the target environment.
	RegionBindingCRN *string `json:"region_binding_crn,omitempty"`

	// The human-readable name of the binding.
	Name *string `json:"name,omitempty"`

	// An alpha-numeric value identifying the account ID.
	AccountID *string `json:"account_id,omitempty"`

	// The ID of the resource group.
	ResourceGroupID *string `json:"resource_group_id,omitempty"`

	// The state of the binding.
	State *string `json:"state,omitempty"`

	// The credentials for the binding. Additional key-value pairs are passed through from the resource brokers. After a
	// credential is created for a service, it can be viewed at any time for users that need the API key value. However,
	// all users must have the correct level of access to see the details of a credential that includes the API key value.
	// For additional details, see [viewing a
	// credential](https://cloud.ibm.com/docs/account?topic=account-service_credentials&interface=ui#viewing-credentials-ui)
	// or the service’s documentation.
	Credentials *Credentials `json:"credentials,omitempty"`

	// Specifies whether the binding’s credentials support IAM.
	IamCompatible *bool `json:"iam_compatible,omitempty"`

	// The unique ID of the offering. This value is provided by and stored in the global catalog.
	ResourceID *string `json:"resource_id,omitempty"`

	// A boolean that dictates if the alias was migrated from a previous CF instance.
	Migrated *bool `json:"migrated,omitempty"`

	// The relative path to the resource alias that this binding is associated with.
	ResourceAliasURL *string `json:"resource_alias_url,omitempty"`
}

// UnmarshalResourceBinding unmarshals an instance of ResourceBinding from the specified map of raw messages.
func UnmarshalResourceBinding(m map[string]json.RawMessage, result interface{}) (err error) {
	obj := new(ResourceBinding)
	err = core.UnmarshalPrimitive(m, "id", &obj.ID)
	if err != nil {
		err = core.SDKErrorf(err, "", "id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "guid", &obj.GUID)
	if err != nil {
		err = core.SDKErrorf(err, "", "guid-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "url", &obj.URL)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "created_at", &obj.CreatedAt)
	if err != nil {
		err = core.SDKErrorf(err, "", "created_at-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "updated_at", &obj.UpdatedAt)
	if err != nil {
		err = core.SDKErrorf(err, "", "updated_at-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "deleted_at", &obj.DeletedAt)
	if err != nil {
		err = core.SDKErrorf(err, "", "deleted_at-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "created_by", &obj.CreatedBy)
	if err != nil {
		err = core.SDKErrorf(err, "", "created_by-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "updated_by", &obj.UpdatedBy)
	if err != nil {
		err = core.SDKErrorf(err, "", "updated_by-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "deleted_by", &obj.DeletedBy)
	if err != nil {
		err = core.SDKErrorf(err, "", "deleted_by-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "source_crn", &obj.SourceCRN)
	if err != nil {
		err = core.SDKErrorf(err, "", "source_crn-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "target_crn", &obj.TargetCRN)
	if err != nil {
		err = core.SDKErrorf(err, "", "target_crn-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "crn", &obj.CRN)
	if err != nil {
		err = core.SDKErrorf(err, "", "crn-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "region_binding_id", &obj.RegionBindingID)
	if err != nil {
		err = core.SDKErrorf(err, "", "region_binding_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "region_binding_crn", &obj.RegionBindingCRN)
	if err != nil {
		err = core.SDKErrorf(err, "", "region_binding_crn-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "name", &obj.Name)
	if err != nil {
		err = core.SDKErrorf(err, "", "name-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "account_id", &obj.AccountID)
	if err != nil {
		err = core.SDKErrorf(err, "", "account_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_group_id", &obj.ResourceGroupID)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_group_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "state", &obj.State)
	if err != nil {
		err = core.SDKErrorf(err, "", "state-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalModel(m, "credentials", &obj.Credentials, UnmarshalCredentials)
	if err != nil {
		err = core.SDKErrorf(err, "", "credentials-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "iam_compatible", &obj.IamCompatible)
	if err != nil {
		err = core.SDKErrorf(err, "", "iam_compatible-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_id", &obj.ResourceID)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "migrated", &obj.Migrated)
	if err != nil {
		err = core.SDKErrorf(err, "", "migrated-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_alias_url", &obj.ResourceAliasURL)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_alias_url-error", common.GetComponentInfo())
		return
	}
	reflect.ValueOf(result).Elem().Set(reflect.ValueOf(obj))
	return
}

// ResourceBindingPostParameters : Configuration options represented as key-value pairs. Service defined options are passed through to the target
// resource brokers, whereas platform defined options are not.
// This type supports additional properties of type interface{}.
type ResourceBindingPostParameters struct {
	// An optional platform defined option to reuse an existing IAM serviceId for the role assignment.
	ServiceidCRN *string `json:"serviceid_crn,omitempty"`

	// Allows users to set arbitrary properties of type interface{}.
	additionalProperties map[string]interface{}
}

// SetProperty allows the user to set an arbitrary property on an instance of ResourceBindingPostParameters.
func (o *ResourceBindingPostParameters) SetProperty(key string, value interface{}) {
	if o.additionalProperties == nil {
		o.additionalProperties = make(map[string]interface{})
	}
	o.additionalProperties[key] = value
}

// SetProperties allows the user to set a map of arbitrary properties on an instance of ResourceBindingPostParameters.
func (o *ResourceBindingPostParameters) SetProperties(m map[string]interface{}) {
	o.additionalProperties = make(map[string]interface{})
	for k, v := range m {
		o.additionalProperties[k] = v
	}
}

// GetProperty allows the user to retrieve an arbitrary property from an instance of ResourceBindingPostParameters.
func (o *ResourceBindingPostParameters) GetProperty(key string) interface{} {
	return o.additionalProperties[key]
}

// GetProperties allows the user to retrieve the map of arbitrary properties from an instance of ResourceBindingPostParameters.
func (o *ResourceBindingPostParameters) GetProperties() map[string]interface{} {
	return o.additionalProperties
}

// MarshalJSON performs custom serialization for instances of ResourceBindingPostParameters
func (o *ResourceBindingPostParameters) MarshalJSON() (buffer []byte, err error) {
	m := make(map[string]interface{})
	if len(o.additionalProperties) > 0 {
		for k, v := range o.additionalProperties {
			m[k] = v
		}
	}
	if o.ServiceidCRN != nil {
		m["serviceid_crn"] = o.ServiceidCRN
	}
	buffer, err = json.Marshal(m)
	if err != nil {
		err = core.SDKErrorf(err, "", "model-marshal", common.GetComponentInfo())
	}
	return
}

// UnmarshalResourceBindingPostParameters unmarshals an instance of ResourceBindingPostParameters from the specified map of raw messages.
func UnmarshalResourceBindingPostParameters(m map[string]json.RawMessage, result interface{}) (err error) {
	obj := new(ResourceBindingPostParameters)
	err = core.UnmarshalPrimitive(m, "serviceid_crn", &obj.ServiceidCRN)
	if err != nil {
		err = core.SDKErrorf(err, "", "serviceid_crn-error", common.GetComponentInfo())
		return
	}
	delete(m, "serviceid_crn")
	for k := range m {
		var v interface{}
		e := core.UnmarshalPrimitive(m, k, &v)
		if e != nil {
			err = core.SDKErrorf(e, "", "additional-properties-error", common.GetComponentInfo())
			return
		}
		obj.SetProperty(k, v)
	}
	reflect.ValueOf(result).Elem().Set(reflect.ValueOf(obj))
	return
}

// ResourceBindingsList : A list of resource bindings.
type ResourceBindingsList struct {
	// The number of resource bindings in `resources`.
	RowsCount *int64 `json:"rows_count" validate:"required"`

	// The URL for requesting the next page of results.
	NextURL *string `json:"next_url" validate:"required"`

	// A list of resource bindings.
	Resources []ResourceBinding `json:"resources" validate:"required"`
}

// UnmarshalResourceBindingsList unmarshals an instance of ResourceBindingsList from the specified map of raw messages.
func UnmarshalResourceBindingsList(m map[string]json.RawMessage, result interface{}) (err error) {
	obj := new(ResourceBindingsList)
	err = core.UnmarshalPrimitive(m, "rows_count", &obj.RowsCount)
	if err != nil {
		err = core.SDKErrorf(err, "", "rows_count-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "next_url", &obj.NextURL)
	if err != nil {
		err = core.SDKErrorf(err, "", "next_url-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalModel(m, "resources", &obj.Resources, UnmarshalResourceBinding)
	if err != nil {
		err = core.SDKErrorf(err, "", "resources-error", common.GetComponentInfo())
		return
	}
	reflect.ValueOf(result).Elem().Set(reflect.ValueOf(obj))
	return
}

// Retrieve the value to be passed to a request to access the next page of results
func (resp *ResourceBindingsList) GetNextStart() (*string, error) {
	if core.IsNil(resp.NextURL) {
		return nil, nil
	}
	start, err := core.GetQueryParam(resp.NextURL, "start")
	if err != nil {
		err = core.SDKErrorf(err, "", "read-query-param-error", common.GetComponentInfo())
		return nil, err
	} else if start == nil {
		return nil, nil
	}
	return start, nil
}

// ResourceInstance : A resource instance.
type ResourceInstance struct {
	// The ID associated with the instance.
	ID *string `json:"id,omitempty"`

	// The GUID of the instance.
	GUID *string `json:"guid,omitempty"`

	// When you provision a new resource, a relative URL path is created identifying the location of the instance.
	URL *string `json:"url,omitempty"`

	// The date when the instance was created.
	CreatedAt *strfmt.DateTime `json:"created_at,omitempty"`

	// The date when the instance was last updated.
	UpdatedAt *strfmt.DateTime `json:"updated_at,omitempty"`

	// The date when the instance was deleted.
	DeletedAt *strfmt.DateTime `json:"deleted_at,omitempty"`

	// The subject who created the instance.
	CreatedBy *string `json:"created_by,omitempty"`

	// The subject who updated the instance.
	UpdatedBy *string `json:"updated_by,omitempty"`

	// The subject who deleted the instance.
	DeletedBy *string `json:"deleted_by,omitempty"`

	// The date when the instance was scheduled for reclamation.
	ScheduledReclaimAt *strfmt.DateTime `json:"scheduled_reclaim_at,omitempty"`

	// The date when the instance under reclamation was restored.
	RestoredAt *strfmt.DateTime `json:"restored_at,omitempty"`

	// The subject who restored the instance back from reclamation.
	RestoredBy *string `json:"restored_by,omitempty"`

	// The subject who initiated the instance reclamation.
	ScheduledReclaimBy *string `json:"scheduled_reclaim_by,omitempty"`

	// The human-readable name of the instance.
	Name *string `json:"name,omitempty"`

	// The deployment location where the instance was provisioned.
	RegionID *string `json:"region_id,omitempty"`

	// An alpha-numeric value identifying the account ID.
	AccountID *string `json:"account_id,omitempty"`

	// The unique ID of the reseller channel where the instance was provisioned from.
	ResellerChannelID *string `json:"reseller_channel_id,omitempty"`

	// The unique ID of the plan associated with the offering. This value is provided by and stored in the global catalog.
	ResourcePlanID *string `json:"resource_plan_id,omitempty"`

	// The ID of the resource group.
	ResourceGroupID *string `json:"resource_group_id,omitempty"`

	// The CRN of the resource group.
	ResourceGroupCRN *string `json:"resource_group_crn,omitempty"`

	// The deployment CRN as defined in the global catalog. The Cloud Resource Name (CRN) of the deployment location where
	// the instance is provisioned.
	TargetCRN *string `json:"target_crn,omitempty"`

	// Whether newly created resource key credentials can be retrieved by using get resource key or get a list of all of
	// the resource keys requests.
	OnetimeCredentials *bool `json:"onetime_credentials,omitempty"`

	// The current configuration parameters of the instance.
	Parameters map[string]interface{} `json:"parameters,omitempty"`

	// A boolean that dictates if the resource instance should be deleted (cleaned up) during the processing of a region
	// instance delete call.
	AllowCleanup *bool `json:"allow_cleanup,omitempty"`

	// The full Cloud Resource Name (CRN) associated with the instance. For more information about this format, see [Cloud
	// Resource Names](https://cloud.ibm.com/docs/overview?topic=overview-crn).
	CRN *string `json:"crn,omitempty"`

	// The current state of the instance. For example, if the instance is deleted, it will return removed.
	State *string `json:"state,omitempty"`

	// The type of the instance, for example, `service_instance`.
	Type *string `json:"type,omitempty"`

	// The sub-type of instance, for example, `cfaas`.
	SubType *string `json:"sub_type,omitempty"`

	// The unique ID of the offering. This value is provided by and stored in the global catalog.
	ResourceID *string `json:"resource_id,omitempty"`

	// The resource-broker-provided URL to access administrative features of the instance.
	DashboardURL *string `json:"dashboard_url,omitempty"`

	// The status of the last operation requested on the instance.
	LastOperation *ResourceInstanceLastOperation `json:"last_operation,omitempty"`

	// The relative path to the resource aliases for the instance.
	// Deprecated: this field is deprecated and may be removed in a future release.
	ResourceAliasesURL *string `json:"resource_aliases_url,omitempty"`

	// The relative path to the resource bindings for the instance.
	// Deprecated: this field is deprecated and may be removed in a future release.
	ResourceBindingsURL *string `json:"resource_bindings_url,omitempty"`

	// The relative path to the resource keys for the instance.
	ResourceKeysURL *string `json:"resource_keys_url,omitempty"`

	// The plan history of the instance.
	PlanHistory []PlanHistoryItem `json:"plan_history,omitempty"`

	// A boolean that dictates if the resource instance was migrated from a previous CF instance.
	Migrated *bool `json:"migrated,omitempty"`

	// Additional instance properties, contributed by the service and/or platform, are represented as key-value pairs.
	Extensions map[string]interface{} `json:"extensions,omitempty"`

	// The CRN of the resource that has control of the instance.
	ControlledBy *string `json:"controlled_by,omitempty"`

	// A boolean that dictates if the resource instance is locked or not.
	Locked *bool `json:"locked,omitempty"`
}

// Constants associated with the ResourceInstance.State property.
// The current state of the instance. For example, if the instance is deleted, it will return removed.
const (
	ResourceInstanceStateActiveConst             = "active"
	ResourceInstanceStateFailedConst             = "failed"
	ResourceInstanceStateInactiveConst           = "inactive"
	ResourceInstanceStatePendingReclamationConst = "pending_reclamation"
	ResourceInstanceStatePendingRemovalConst     = "pending_removal"
	ResourceInstanceStatePreProvisioningConst    = "pre_provisioning"
	ResourceInstanceStateProvisioningConst       = "provisioning"
	ResourceInstanceStateRemovedConst            = "removed"
)

// UnmarshalResourceInstance unmarshals an instance of ResourceInstance from the specified map of raw messages.
func UnmarshalResourceInstance(m map[string]json.RawMessage, result interface{}) (err error) {
	obj := new(ResourceInstance)
	err = core.UnmarshalPrimitive(m, "id", &obj.ID)
	if err != nil {
		err = core.SDKErrorf(err, "", "id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "guid", &obj.GUID)
	if err != nil {
		err = core.SDKErrorf(err, "", "guid-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "url", &obj.URL)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "created_at", &obj.CreatedAt)
	if err != nil {
		err = core.SDKErrorf(err, "", "created_at-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "updated_at", &obj.UpdatedAt)
	if err != nil {
		err = core.SDKErrorf(err, "", "updated_at-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "deleted_at", &obj.DeletedAt)
	if err != nil {
		err = core.SDKErrorf(err, "", "deleted_at-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "created_by", &obj.CreatedBy)
	if err != nil {
		err = core.SDKErrorf(err, "", "created_by-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "updated_by", &obj.UpdatedBy)
	if err != nil {
		err = core.SDKErrorf(err, "", "updated_by-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "deleted_by", &obj.DeletedBy)
	if err != nil {
		err = core.SDKErrorf(err, "", "deleted_by-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "scheduled_reclaim_at", &obj.ScheduledReclaimAt)
	if err != nil {
		err = core.SDKErrorf(err, "", "scheduled_reclaim_at-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "restored_at", &obj.RestoredAt)
	if err != nil {
		err = core.SDKErrorf(err, "", "restored_at-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "restored_by", &obj.RestoredBy)
	if err != nil {
		err = core.SDKErrorf(err, "", "restored_by-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "scheduled_reclaim_by", &obj.ScheduledReclaimBy)
	if err != nil {
		err = core.SDKErrorf(err, "", "scheduled_reclaim_by-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "name", &obj.Name)
	if err != nil {
		err = core.SDKErrorf(err, "", "name-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "region_id", &obj.RegionID)
	if err != nil {
		err = core.SDKErrorf(err, "", "region_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "account_id", &obj.AccountID)
	if err != nil {
		err = core.SDKErrorf(err, "", "account_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "reseller_channel_id", &obj.ResellerChannelID)
	if err != nil {
		err = core.SDKErrorf(err, "", "reseller_channel_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_plan_id", &obj.ResourcePlanID)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_plan_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_group_id", &obj.ResourceGroupID)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_group_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_group_crn", &obj.ResourceGroupCRN)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_group_crn-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "target_crn", &obj.TargetCRN)
	if err != nil {
		err = core.SDKErrorf(err, "", "target_crn-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "onetime_credentials", &obj.OnetimeCredentials)
	if err != nil {
		err = core.SDKErrorf(err, "", "onetime_credentials-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "parameters", &obj.Parameters)
	if err != nil {
		err = core.SDKErrorf(err, "", "parameters-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "allow_cleanup", &obj.AllowCleanup)
	if err != nil {
		err = core.SDKErrorf(err, "", "allow_cleanup-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "crn", &obj.CRN)
	if err != nil {
		err = core.SDKErrorf(err, "", "crn-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "state", &obj.State)
	if err != nil {
		err = core.SDKErrorf(err, "", "state-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "type", &obj.Type)
	if err != nil {
		err = core.SDKErrorf(err, "", "type-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "sub_type", &obj.SubType)
	if err != nil {
		err = core.SDKErrorf(err, "", "sub_type-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_id", &obj.ResourceID)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "dashboard_url", &obj.DashboardURL)
	if err != nil {
		err = core.SDKErrorf(err, "", "dashboard_url-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalModel(m, "last_operation", &obj.LastOperation, UnmarshalResourceInstanceLastOperation)
	if err != nil {
		err = core.SDKErrorf(err, "", "last_operation-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_aliases_url", &obj.ResourceAliasesURL)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_aliases_url-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_bindings_url", &obj.ResourceBindingsURL)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_bindings_url-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_keys_url", &obj.ResourceKeysURL)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_keys_url-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalModel(m, "plan_history", &obj.PlanHistory, UnmarshalPlanHistoryItem)
	if err != nil {
		err = core.SDKErrorf(err, "", "plan_history-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "migrated", &obj.Migrated)
	if err != nil {
		err = core.SDKErrorf(err, "", "migrated-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "extensions", &obj.Extensions)
	if err != nil {
		err = core.SDKErrorf(err, "", "extensions-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "controlled_by", &obj.ControlledBy)
	if err != nil {
		err = core.SDKErrorf(err, "", "controlled_by-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "locked", &obj.Locked)
	if err != nil {
		err = core.SDKErrorf(err, "", "locked-error", common.GetComponentInfo())
		return
	}
	reflect.ValueOf(result).Elem().Set(reflect.ValueOf(obj))
	return
}

// ResourceInstanceLastOperation : The status of the last operation requested on the instance.
// This type supports additional properties of type interface{}.
type ResourceInstanceLastOperation struct {
	// The last operation type of the resource instance.
	Type *string `json:"type" validate:"required"`

	// The last operation state of the resoure instance. This indicates if the resource's last operation is in progress,
	// succeeded or failed.
	State *string `json:"state" validate:"required"`

	// The last operation sub type of the resoure instance.
	SubType *string `json:"sub_type,omitempty"`

	// A boolean that indicates if the resource is provisioned asynchronously or not.
	Async *bool `json:"async" validate:"required"`

	// The description of the status of last operation.
	Description *string `json:"description" validate:"required"`

	// Optional string that states the reason code for the last operation state change.
	ReasonCode *string `json:"reason_code,omitempty"`

	// A field which indicates the time after which the instance's last operation is to be polled.
	PollAfter *float64 `json:"poll_after,omitempty"`

	// A boolean that indicates if the resource's last operation is cancelable or not.
	Cancelable *bool `json:"cancelable" validate:"required"`

	// A boolean that indicates if the resource broker's last operation can be polled or not.
	Poll *bool `json:"poll" validate:"required"`

	// Allows users to set arbitrary properties of type interface{}.
	additionalProperties map[string]interface{}
}

// Constants associated with the ResourceInstanceLastOperation.State property.
// The last operation state of the resoure instance. This indicates if the resource's last operation is in progress,
// succeeded or failed.
const (
	ResourceInstanceLastOperationStateFailedConst     = "failed"
	ResourceInstanceLastOperationStateInProgressConst = "in progress"
	ResourceInstanceLastOperationStateSucceededConst  = "succeeded"
)

// SetProperty allows the user to set an arbitrary property on an instance of ResourceInstanceLastOperation.
func (o *ResourceInstanceLastOperation) SetProperty(key string, value interface{}) {
	if o.additionalProperties == nil {
		o.additionalProperties = make(map[string]interface{})
	}
	o.additionalProperties[key] = value
}

// SetProperties allows the user to set a map of arbitrary properties on an instance of ResourceInstanceLastOperation.
func (o *ResourceInstanceLastOperation) SetProperties(m map[string]interface{}) {
	o.additionalProperties = make(map[string]interface{})
	for k, v := range m {
		o.additionalProperties[k] = v
	}
}

// GetProperty allows the user to retrieve an arbitrary property from an instance of ResourceInstanceLastOperation.
func (o *ResourceInstanceLastOperation) GetProperty(key string) interface{} {
	return o.additionalProperties[key]
}

// GetProperties allows the user to retrieve the map of arbitrary properties from an instance of ResourceInstanceLastOperation.
func (o *ResourceInstanceLastOperation) GetProperties() map[string]interface{} {
	return o.additionalProperties
}

// MarshalJSON performs custom serialization for instances of ResourceInstanceLastOperation
func (o *ResourceInstanceLastOperation) MarshalJSON() (buffer []byte, err error) {
	m := make(map[string]interface{})
	if len(o.additionalProperties) > 0 {
		for k, v := range o.additionalProperties {
			m[k] = v
		}
	}
	if o.Type != nil {
		m["type"] = o.Type
	}
	if o.State != nil {
		m["state"] = o.State
	}
	if o.SubType != nil {
		m["sub_type"] = o.SubType
	}
	if o.Async != nil {
		m["async"] = o.Async
	}
	if o.Description != nil {
		m["description"] = o.Description
	}
	if o.ReasonCode != nil {
		m["reason_code"] = o.ReasonCode
	}
	if o.PollAfter != nil {
		m["poll_after"] = o.PollAfter
	}
	if o.Cancelable != nil {
		m["cancelable"] = o.Cancelable
	}
	if o.Poll != nil {
		m["poll"] = o.Poll
	}
	buffer, err = json.Marshal(m)
	if err != nil {
		err = core.SDKErrorf(err, "", "model-marshal", common.GetComponentInfo())
	}
	return
}

// UnmarshalResourceInstanceLastOperation unmarshals an instance of ResourceInstanceLastOperation from the specified map of raw messages.
func UnmarshalResourceInstanceLastOperation(m map[string]json.RawMessage, result interface{}) (err error) {
	obj := new(ResourceInstanceLastOperation)
	err = core.UnmarshalPrimitive(m, "type", &obj.Type)
	if err != nil {
		err = core.SDKErrorf(err, "", "type-error", common.GetComponentInfo())
		return
	}
	delete(m, "type")
	err = core.UnmarshalPrimitive(m, "state", &obj.State)
	if err != nil {
		err = core.SDKErrorf(err, "", "state-error", common.GetComponentInfo())
		return
	}
	delete(m, "state")
	err = core.UnmarshalPrimitive(m, "sub_type", &obj.SubType)
	if err != nil {
		err = core.SDKErrorf(err, "", "sub_type-error", common.GetComponentInfo())
		return
	}
	delete(m, "sub_type")
	err = core.UnmarshalPrimitive(m, "async", &obj.Async)
	if err != nil {
		err = core.SDKErrorf(err, "", "async-error", common.GetComponentInfo())
		return
	}
	delete(m, "async")
	err = core.UnmarshalPrimitive(m, "description", &obj.Description)
	if err != nil {
		err = core.SDKErrorf(err, "", "description-error", common.GetComponentInfo())
		return
	}
	delete(m, "description")
	err = core.UnmarshalPrimitive(m, "reason_code", &obj.ReasonCode)
	if err != nil {
		err = core.SDKErrorf(err, "", "reason_code-error", common.GetComponentInfo())
		return
	}
	delete(m, "reason_code")
	err = core.UnmarshalPrimitive(m, "poll_after", &obj.PollAfter)
	if err != nil {
		err = core.SDKErrorf(err, "", "poll_after-error", common.GetComponentInfo())
		return
	}
	delete(m, "poll_after")
	err = core.UnmarshalPrimitive(m, "cancelable", &obj.Cancelable)
	if err != nil {
		err = core.SDKErrorf(err, "", "cancelable-error", common.GetComponentInfo())
		return
	}
	delete(m, "cancelable")
	err = core.UnmarshalPrimitive(m, "poll", &obj.Poll)
	if err != nil {
		err = core.SDKErrorf(err, "", "poll-error", common.GetComponentInfo())
		return
	}
	delete(m, "poll")
	for k := range m {
		var v interface{}
		e := core.UnmarshalPrimitive(m, k, &v)
		if e != nil {
			err = core.SDKErrorf(e, "", "additional-properties-error", common.GetComponentInfo())
			return
		}
		obj.SetProperty(k, v)
	}
	reflect.ValueOf(result).Elem().Set(reflect.ValueOf(obj))
	return
}

// ResourceInstancesList : A list of resource instances.
type ResourceInstancesList struct {
	// The number of resource instances in `resources`.
	RowsCount *int64 `json:"rows_count" validate:"required"`

	// The URL for requesting the next page of results.
	NextURL *string `json:"next_url" validate:"required"`

	// A list of resource instances.
	Resources []ResourceInstance `json:"resources" validate:"required"`
}

// UnmarshalResourceInstancesList unmarshals an instance of ResourceInstancesList from the specified map of raw messages.
func UnmarshalResourceInstancesList(m map[string]json.RawMessage, result interface{}) (err error) {
	obj := new(ResourceInstancesList)
	err = core.UnmarshalPrimitive(m, "rows_count", &obj.RowsCount)
	if err != nil {
		err = core.SDKErrorf(err, "", "rows_count-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "next_url", &obj.NextURL)
	if err != nil {
		err = core.SDKErrorf(err, "", "next_url-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalModel(m, "resources", &obj.Resources, UnmarshalResourceInstance)
	if err != nil {
		err = core.SDKErrorf(err, "", "resources-error", common.GetComponentInfo())
		return
	}
	reflect.ValueOf(result).Elem().Set(reflect.ValueOf(obj))
	return
}

// Retrieve the value to be passed to a request to access the next page of results
func (resp *ResourceInstancesList) GetNextStart() (*string, error) {
	if core.IsNil(resp.NextURL) {
		return nil, nil
	}
	start, err := core.GetQueryParam(resp.NextURL, "start")
	if err != nil {
		err = core.SDKErrorf(err, "", "read-query-param-error", common.GetComponentInfo())
		return nil, err
	} else if start == nil {
		return nil, nil
	}
	return start, nil
}

// ResourceKey : A resource key.
type ResourceKey struct {
	// The ID associated with the key.
	ID *string `json:"id,omitempty"`

	// The GUID of the key.
	GUID *string `json:"guid,omitempty"`

	// When you created a new key, a relative URL path is created identifying the location of the key.
	URL *string `json:"url,omitempty"`

	// The date when the key was created.
	CreatedAt *strfmt.DateTime `json:"created_at,omitempty"`

	// The date when the key was last updated.
	UpdatedAt *strfmt.DateTime `json:"updated_at,omitempty"`

	// The date when the key was deleted.
	DeletedAt *strfmt.DateTime `json:"deleted_at,omitempty"`

	// The subject who created the key.
	CreatedBy *string `json:"created_by,omitempty"`

	// The subject who updated the key.
	UpdatedBy *string `json:"updated_by,omitempty"`

	// The subject who deleted the key.
	DeletedBy *string `json:"deleted_by,omitempty"`

	// The CRN of resource instance or alias associated to the key.
	SourceCRN *string `json:"source_crn,omitempty"`

	// The human-readable name of the key.
	Name *string `json:"name,omitempty"`

	// The full Cloud Resource Name (CRN) associated with the key. For more information about this format, see [Cloud
	// Resource Names](https://cloud.ibm.com/docs/overview?topic=overview-crn).
	CRN *string `json:"crn,omitempty"`

	// The state of the key.
	State *string `json:"state,omitempty"`

	// An alpha-numeric value identifying the account ID.
	AccountID *string `json:"account_id,omitempty"`

	// The ID of the resource group.
	ResourceGroupID *string `json:"resource_group_id,omitempty"`

	// The unique ID of the offering. This value is provided by and stored in the global catalog.
	ResourceID *string `json:"resource_id,omitempty"`

	// Whether newly created resource key credentials can be retrieved by using get resource key or get a list of all of
	// the resource keys requests.
	OnetimeCredentials *bool `json:"onetime_credentials,omitempty"`

	// The credentials for the key. Additional key-value pairs are passed through from the resource brokers. After a
	// credential is created for a service, it can be viewed at any time for users that need the API key value. However,
	// all users must have the correct level of access to see the details of a credential that includes the API key value.
	// For additional details, see [viewing a
	// credential](https://cloud.ibm.com/docs/account?topic=account-service_credentials&interface=ui#viewing-credentials-ui)
	// or the service’s documentation.
	Credentials *Credentials `json:"credentials,omitempty"`

	// Specifies whether the key’s credentials support IAM.
	IamCompatible *bool `json:"iam_compatible,omitempty"`

	// A boolean that dictates if the alias was migrated from a previous CF instance.
	Migrated *bool `json:"migrated,omitempty"`

	// The relative path to the resource.
	ResourceInstanceURL *string `json:"resource_instance_url,omitempty"`

	// The relative path to the resource alias that this binding is associated with.
	ResourceAliasURL *string `json:"resource_alias_url,omitempty"`
}

// UnmarshalResourceKey unmarshals an instance of ResourceKey from the specified map of raw messages.
func UnmarshalResourceKey(m map[string]json.RawMessage, result interface{}) (err error) {
	obj := new(ResourceKey)
	err = core.UnmarshalPrimitive(m, "id", &obj.ID)
	if err != nil {
		err = core.SDKErrorf(err, "", "id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "guid", &obj.GUID)
	if err != nil {
		err = core.SDKErrorf(err, "", "guid-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "url", &obj.URL)
	if err != nil {
		err = core.SDKErrorf(err, "", "url-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "created_at", &obj.CreatedAt)
	if err != nil {
		err = core.SDKErrorf(err, "", "created_at-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "updated_at", &obj.UpdatedAt)
	if err != nil {
		err = core.SDKErrorf(err, "", "updated_at-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "deleted_at", &obj.DeletedAt)
	if err != nil {
		err = core.SDKErrorf(err, "", "deleted_at-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "created_by", &obj.CreatedBy)
	if err != nil {
		err = core.SDKErrorf(err, "", "created_by-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "updated_by", &obj.UpdatedBy)
	if err != nil {
		err = core.SDKErrorf(err, "", "updated_by-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "deleted_by", &obj.DeletedBy)
	if err != nil {
		err = core.SDKErrorf(err, "", "deleted_by-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "source_crn", &obj.SourceCRN)
	if err != nil {
		err = core.SDKErrorf(err, "", "source_crn-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "name", &obj.Name)
	if err != nil {
		err = core.SDKErrorf(err, "", "name-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "crn", &obj.CRN)
	if err != nil {
		err = core.SDKErrorf(err, "", "crn-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "state", &obj.State)
	if err != nil {
		err = core.SDKErrorf(err, "", "state-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "account_id", &obj.AccountID)
	if err != nil {
		err = core.SDKErrorf(err, "", "account_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_group_id", &obj.ResourceGroupID)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_group_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_id", &obj.ResourceID)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_id-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "onetime_credentials", &obj.OnetimeCredentials)
	if err != nil {
		err = core.SDKErrorf(err, "", "onetime_credentials-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalModel(m, "credentials", &obj.Credentials, UnmarshalCredentials)
	if err != nil {
		err = core.SDKErrorf(err, "", "credentials-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "iam_compatible", &obj.IamCompatible)
	if err != nil {
		err = core.SDKErrorf(err, "", "iam_compatible-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "migrated", &obj.Migrated)
	if err != nil {
		err = core.SDKErrorf(err, "", "migrated-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_instance_url", &obj.ResourceInstanceURL)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_instance_url-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "resource_alias_url", &obj.ResourceAliasURL)
	if err != nil {
		err = core.SDKErrorf(err, "", "resource_alias_url-error", common.GetComponentInfo())
		return
	}
	reflect.ValueOf(result).Elem().Set(reflect.ValueOf(obj))
	return
}

// ResourceKeyPostParameters : Configuration options represented as key-value pairs. Service defined options are passed through to the target
// resource brokers, whereas platform defined options are not.
// This type supports additional properties of type interface{}.
type ResourceKeyPostParameters struct {
	// An optional platform defined option to reuse an existing IAM serviceId for the role assignment.
	ServiceidCRN *string `json:"serviceid_crn,omitempty"`

	// Allows users to set arbitrary properties of type interface{}.
	additionalProperties map[string]interface{}
}

// SetProperty allows the user to set an arbitrary property on an instance of ResourceKeyPostParameters.
func (o *ResourceKeyPostParameters) SetProperty(key string, value interface{}) {
	if o.additionalProperties == nil {
		o.additionalProperties = make(map[string]interface{})
	}
	o.additionalProperties[key] = value
}

// SetProperties allows the user to set a map of arbitrary properties on an instance of ResourceKeyPostParameters.
func (o *ResourceKeyPostParameters) SetProperties(m map[string]interface{}) {
	o.additionalProperties = make(map[string]interface{})
	for k, v := range m {
		o.additionalProperties[k] = v
	}
}

// GetProperty allows the user to retrieve an arbitrary property from an instance of ResourceKeyPostParameters.
func (o *ResourceKeyPostParameters) GetProperty(key string) interface{} {
	return o.additionalProperties[key]
}

// GetProperties allows the user to retrieve the map of arbitrary properties from an instance of ResourceKeyPostParameters.
func (o *ResourceKeyPostParameters) GetProperties() map[string]interface{} {
	return o.additionalProperties
}

// MarshalJSON performs custom serialization for instances of ResourceKeyPostParameters
func (o *ResourceKeyPostParameters) MarshalJSON() (buffer []byte, err error) {
	m := make(map[string]interface{})
	if len(o.additionalProperties) > 0 {
		for k, v := range o.additionalProperties {
			m[k] = v
		}
	}
	if o.ServiceidCRN != nil {
		m["serviceid_crn"] = o.ServiceidCRN
	}
	buffer, err = json.Marshal(m)
	if err != nil {
		err = core.SDKErrorf(err, "", "model-marshal", common.GetComponentInfo())
	}
	return
}

// UnmarshalResourceKeyPostParameters unmarshals an instance of ResourceKeyPostParameters from the specified map of raw messages.
func UnmarshalResourceKeyPostParameters(m map[string]json.RawMessage, result interface{}) (err error) {
	obj := new(ResourceKeyPostParameters)
	err = core.UnmarshalPrimitive(m, "serviceid_crn", &obj.ServiceidCRN)
	if err != nil {
		err = core.SDKErrorf(err, "", "serviceid_crn-error", common.GetComponentInfo())
		return
	}
	delete(m, "serviceid_crn")
	for k := range m {
		var v interface{}
		e := core.UnmarshalPrimitive(m, k, &v)
		if e != nil {
			err = core.SDKErrorf(e, "", "additional-properties-error", common.GetComponentInfo())
			return
		}
		obj.SetProperty(k, v)
	}
	reflect.ValueOf(result).Elem().Set(reflect.ValueOf(obj))
	return
}

// ResourceKeysList : A list of resource keys.
type ResourceKeysList struct {
	// The number of resource keys in `resources`.
	RowsCount *int64 `json:"rows_count" validate:"required"`

	// The URL for requesting the next page of results.
	NextURL *string `json:"next_url" validate:"required"`

	// A list of resource keys.
	Resources []ResourceKey `json:"resources" validate:"required"`
}

// UnmarshalResourceKeysList unmarshals an instance of ResourceKeysList from the specified map of raw messages.
func UnmarshalResourceKeysList(m map[string]json.RawMessage, result interface{}) (err error) {
	obj := new(ResourceKeysList)
	err = core.UnmarshalPrimitive(m, "rows_count", &obj.RowsCount)
	if err != nil {
		err = core.SDKErrorf(err, "", "rows_count-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalPrimitive(m, "next_url", &obj.NextURL)
	if err != nil {
		err = core.SDKErrorf(err, "", "next_url-error", common.GetComponentInfo())
		return
	}
	err = core.UnmarshalModel(m, "resources", &obj.Resources, UnmarshalResourceKey)
	if err != nil {
		err = core.SDKErrorf(err, "", "resources-error", common.GetComponentInfo())
		return
	}
	reflect.ValueOf(result).Elem().Set(reflect.ValueOf(obj))
	return
}

// Retrieve the value to be passed to a request to access the next page of results
func (resp *ResourceKeysList) GetNextStart() (*string, error) {
	if core.IsNil(resp.NextURL) {
		return nil, nil
	}
	start, err := core.GetQueryParam(resp.NextURL, "start")
	if err != nil {
		err = core.SDKErrorf(err, "", "read-query-param-error", common.GetComponentInfo())
		return nil, err
	} else if start == nil {
		return nil, nil
	}
	return start, nil
}

// RunReclamationActionOptions : The RunReclamationAction options.
type RunReclamationActionOptions struct {
	// The ID associated with the reclamation.
	ID *string `json:"id" validate:"required,ne="`

	// The reclamation action name. Specify `reclaim` to delete a resource, or `restore` to restore a resource.
	ActionName *string `json:"action_name" validate:"required,ne="`

	// The request initiator, if different from the request token.
	RequestBy *string `json:"request_by,omitempty"`

	// A comment to describe the action.
	Comment *string `json:"comment,omitempty"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewRunReclamationActionOptions : Instantiate RunReclamationActionOptions
func (*ResourceControllerV2) NewRunReclamationActionOptions(id string, actionName string) *RunReclamationActionOptions {
	return &RunReclamationActionOptions{
		ID:         core.StringPtr(id),
		ActionName: core.StringPtr(actionName),
	}
}

// SetID : Allow user to set ID
func (_options *RunReclamationActionOptions) SetID(id string) *RunReclamationActionOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetActionName : Allow user to set ActionName
func (_options *RunReclamationActionOptions) SetActionName(actionName string) *RunReclamationActionOptions {
	_options.ActionName = core.StringPtr(actionName)
	return _options
}

// SetRequestBy : Allow user to set RequestBy
func (_options *RunReclamationActionOptions) SetRequestBy(requestBy string) *RunReclamationActionOptions {
	_options.RequestBy = core.StringPtr(requestBy)
	return _options
}

// SetComment : Allow user to set Comment
func (_options *RunReclamationActionOptions) SetComment(comment string) *RunReclamationActionOptions {
	_options.Comment = core.StringPtr(comment)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *RunReclamationActionOptions) SetHeaders(param map[string]string) *RunReclamationActionOptions {
	options.Headers = param
	return options
}

// UnlockResourceInstanceOptions : The UnlockResourceInstance options.
type UnlockResourceInstanceOptions struct {
	// The resource instance URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewUnlockResourceInstanceOptions : Instantiate UnlockResourceInstanceOptions
func (*ResourceControllerV2) NewUnlockResourceInstanceOptions(id string) *UnlockResourceInstanceOptions {
	return &UnlockResourceInstanceOptions{
		ID: core.StringPtr(id),
	}
}

// SetID : Allow user to set ID
func (_options *UnlockResourceInstanceOptions) SetID(id string) *UnlockResourceInstanceOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *UnlockResourceInstanceOptions) SetHeaders(param map[string]string) *UnlockResourceInstanceOptions {
	options.Headers = param
	return options
}

// UpdateResourceAliasOptions : The UpdateResourceAlias options.
type UpdateResourceAliasOptions struct {
	// The resource alias URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// The new name of the alias. Must be 180 characters or less and cannot include any special characters other than
	// `(space) - . _ :`.
	Name *string `json:"name" validate:"required"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewUpdateResourceAliasOptions : Instantiate UpdateResourceAliasOptions
func (*ResourceControllerV2) NewUpdateResourceAliasOptions(id string, name string) *UpdateResourceAliasOptions {
	return &UpdateResourceAliasOptions{
		ID:   core.StringPtr(id),
		Name: core.StringPtr(name),
	}
}

// SetID : Allow user to set ID
func (_options *UpdateResourceAliasOptions) SetID(id string) *UpdateResourceAliasOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetName : Allow user to set Name
func (_options *UpdateResourceAliasOptions) SetName(name string) *UpdateResourceAliasOptions {
	_options.Name = core.StringPtr(name)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *UpdateResourceAliasOptions) SetHeaders(param map[string]string) *UpdateResourceAliasOptions {
	options.Headers = param
	return options
}

// UpdateResourceBindingOptions : The UpdateResourceBinding options.
type UpdateResourceBindingOptions struct {
	// The resource binding URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// The new name of the binding. Must be 180 characters or less and cannot include any special characters other than
	// `(space) - . _ :`.
	Name *string `json:"name" validate:"required"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewUpdateResourceBindingOptions : Instantiate UpdateResourceBindingOptions
func (*ResourceControllerV2) NewUpdateResourceBindingOptions(id string, name string) *UpdateResourceBindingOptions {
	return &UpdateResourceBindingOptions{
		ID:   core.StringPtr(id),
		Name: core.StringPtr(name),
	}
}

// SetID : Allow user to set ID
func (_options *UpdateResourceBindingOptions) SetID(id string) *UpdateResourceBindingOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetName : Allow user to set Name
func (_options *UpdateResourceBindingOptions) SetName(name string) *UpdateResourceBindingOptions {
	_options.Name = core.StringPtr(name)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *UpdateResourceBindingOptions) SetHeaders(param map[string]string) *UpdateResourceBindingOptions {
	options.Headers = param
	return options
}

// UpdateResourceInstanceOptions : The UpdateResourceInstance options.
type UpdateResourceInstanceOptions struct {
	// The resource instance URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// The new name of the instance. Must be 180 characters or less and cannot include any special characters other than
	// `(space) - . _ :`.
	Name *string `json:"name,omitempty"`

	// The new configuration options for the instance. Set the `onetime_credentials` property to specify whether newly
	// created resource key credentials can be retrieved by using get resource key or get a list of all of the resource
	// keys requests.
	Parameters map[string]interface{} `json:"parameters,omitempty"`

	// The unique ID of the plan associated with the offering. This value is provided by and stored in the global catalog.
	ResourcePlanID *string `json:"resource_plan_id,omitempty"`

	// A boolean that dictates if the resource instance should be deleted (cleaned up) during the processing of a region
	// instance delete call.
	AllowCleanup *bool `json:"allow_cleanup,omitempty"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewUpdateResourceInstanceOptions : Instantiate UpdateResourceInstanceOptions
func (*ResourceControllerV2) NewUpdateResourceInstanceOptions(id string) *UpdateResourceInstanceOptions {
	return &UpdateResourceInstanceOptions{
		ID: core.StringPtr(id),
	}
}

// SetID : Allow user to set ID
func (_options *UpdateResourceInstanceOptions) SetID(id string) *UpdateResourceInstanceOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetName : Allow user to set Name
func (_options *UpdateResourceInstanceOptions) SetName(name string) *UpdateResourceInstanceOptions {
	_options.Name = core.StringPtr(name)
	return _options
}

// SetParameters : Allow user to set Parameters
func (_options *UpdateResourceInstanceOptions) SetParameters(parameters map[string]interface{}) *UpdateResourceInstanceOptions {
	_options.Parameters = parameters
	return _options
}

// SetResourcePlanID : Allow user to set ResourcePlanID
func (_options *UpdateResourceInstanceOptions) SetResourcePlanID(resourcePlanID string) *UpdateResourceInstanceOptions {
	_options.ResourcePlanID = core.StringPtr(resourcePlanID)
	return _options
}

// SetAllowCleanup : Allow user to set AllowCleanup
func (_options *UpdateResourceInstanceOptions) SetAllowCleanup(allowCleanup bool) *UpdateResourceInstanceOptions {
	_options.AllowCleanup = core.BoolPtr(allowCleanup)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *UpdateResourceInstanceOptions) SetHeaders(param map[string]string) *UpdateResourceInstanceOptions {
	options.Headers = param
	return options
}

// UpdateResourceKeyOptions : The UpdateResourceKey options.
type UpdateResourceKeyOptions struct {
	// The resource key URL-encoded CRN or GUID.
	ID *string `json:"id" validate:"required,ne="`

	// The new name of the key. Must be 180 characters or less and cannot include any special characters other than
	// `(space) - . _ :`.
	Name *string `json:"name" validate:"required"`

	// Allows users to set headers on API requests.
	Headers map[string]string
}

// NewUpdateResourceKeyOptions : Instantiate UpdateResourceKeyOptions
func (*ResourceControllerV2) NewUpdateResourceKeyOptions(id string, name string) *UpdateResourceKeyOptions {
	return &UpdateResourceKeyOptions{
		ID:   core.StringPtr(id),
		Name: core.StringPtr(name),
	}
}

// SetID : Allow user to set ID
func (_options *UpdateResourceKeyOptions) SetID(id string) *UpdateResourceKeyOptions {
	_options.ID = core.StringPtr(id)
	return _options
}

// SetName : Allow user to set Name
func (_options *UpdateResourceKeyOptions) SetName(name string) *UpdateResourceKeyOptions {
	_options.Name = core.StringPtr(name)
	return _options
}

// SetHeaders : Allow user to set Headers
func (options *UpdateResourceKeyOptions) SetHeaders(param map[string]string) *UpdateResourceKeyOptions {
	options.Headers = param
	return options
}

// ResourceInstancesPager can be used to simplify the use of the "ListResourceInstances" method.
type ResourceInstancesPager struct {
	hasNext     bool
	options     *ListResourceInstancesOptions
	client      *ResourceControllerV2
	pageContext struct {
		next *string
	}
}

// NewResourceInstancesPager returns a new ResourceInstancesPager instance.
func (resourceController *ResourceControllerV2) NewResourceInstancesPager(options *ListResourceInstancesOptions) (pager *ResourceInstancesPager, err error) {
	if options.Start != nil && *options.Start != "" {
		err = core.SDKErrorf(nil, "the 'options.Start' field should not be set", "no-query-setting", common.GetComponentInfo())
		return
	}

	var optionsCopy ListResourceInstancesOptions = *options
	pager = &ResourceInstancesPager{
		hasNext: true,
		options: &optionsCopy,
		client:  resourceController,
	}
	return
}

// HasNext returns true if there are potentially more results to be retrieved.
func (pager *ResourceInstancesPager) HasNext() bool {
	return pager.hasNext
}

// GetNextWithContext returns the next page of results using the specified Context.
func (pager *ResourceInstancesPager) GetNextWithContext(ctx context.Context) (page []ResourceInstance, err error) {
	if !pager.HasNext() {
		return nil, fmt.Errorf("no more results available")
	}

	pager.options.Start = pager.pageContext.next

	result, _, err := pager.client.ListResourceInstancesWithContext(ctx, pager.options)
	if err != nil {
		err = core.RepurposeSDKProblem(err, "error-getting-next-page")
		return
	}

	var next *string
	if result.NextURL != nil {
		var start *string
		start, err = core.GetQueryParam(result.NextURL, "start")
		if err != nil {
			errMsg := fmt.Sprintf("error retrieving 'start' query parameter from URL '%s': %s", *result.NextURL, err.Error())
			err = core.SDKErrorf(err, errMsg, "get-query-error", common.GetComponentInfo())
			return
		}
		next = start
	}
	pager.pageContext.next = next
	pager.hasNext = (pager.pageContext.next != nil)
	page = result.Resources

	return
}

// GetAllWithContext returns all results by invoking GetNextWithContext() repeatedly
// until all pages of results have been retrieved.
func (pager *ResourceInstancesPager) GetAllWithContext(ctx context.Context) (allItems []ResourceInstance, err error) {
	for pager.HasNext() {
		var nextPage []ResourceInstance
		nextPage, err = pager.GetNextWithContext(ctx)
		if err != nil {
			err = core.RepurposeSDKProblem(err, "error-getting-next-page")
			return
		}
		allItems = append(allItems, nextPage...)
	}
	return
}

// GetNext invokes GetNextWithContext() using context.Background() as the Context parameter.
func (pager *ResourceInstancesPager) GetNext() (page []ResourceInstance, err error) {
	page, err = pager.GetNextWithContext(context.Background())
	err = core.RepurposeSDKProblem(err, "")
	return
}

// GetAll invokes GetAllWithContext() using context.Background() as the Context parameter.
func (pager *ResourceInstancesPager) GetAll() (allItems []ResourceInstance, err error) {
	allItems, err = pager.GetAllWithContext(context.Background())
	err = core.RepurposeSDKProblem(err, "")
	return
}

// ResourceAliasesForInstancePager can be used to simplify the use of the "ListResourceAliasesForInstance" method.
type ResourceAliasesForInstancePager struct {
	hasNext     bool
	options     *ListResourceAliasesForInstanceOptions
	client      *ResourceControllerV2
	pageContext struct {
		next *string
	}
}

// NewResourceAliasesForInstancePager returns a new ResourceAliasesForInstancePager instance.
func (resourceController *ResourceControllerV2) NewResourceAliasesForInstancePager(options *ListResourceAliasesForInstanceOptions) (pager *ResourceAliasesForInstancePager, err error) {
	if options.Start != nil && *options.Start != "" {
		err = core.SDKErrorf(nil, "the 'options.Start' field should not be set", "no-query-setting", common.GetComponentInfo())
		return
	}

	var optionsCopy ListResourceAliasesForInstanceOptions = *options
	pager = &ResourceAliasesForInstancePager{
		hasNext: true,
		options: &optionsCopy,
		client:  resourceController,
	}
	return
}

// HasNext returns true if there are potentially more results to be retrieved.
func (pager *ResourceAliasesForInstancePager) HasNext() bool {
	return pager.hasNext
}

// GetNextWithContext returns the next page of results using the specified Context.
func (pager *ResourceAliasesForInstancePager) GetNextWithContext(ctx context.Context) (page []ResourceAlias, err error) {
	if !pager.HasNext() {
		return nil, fmt.Errorf("no more results available")
	}

	pager.options.Start = pager.pageContext.next

	result, _, err := pager.client.ListResourceAliasesForInstanceWithContext(ctx, pager.options)
	if err != nil {
		err = core.RepurposeSDKProblem(err, "error-getting-next-page")
		return
	}

	var next *string
	if result.NextURL != nil {
		var start *string
		start, err = core.GetQueryParam(result.NextURL, "start")
		if err != nil {
			errMsg := fmt.Sprintf("error retrieving 'start' query parameter from URL '%s': %s", *result.NextURL, err.Error())
			err = core.SDKErrorf(err, errMsg, "get-query-error", common.GetComponentInfo())
			return
		}
		next = start
	}
	pager.pageContext.next = next
	pager.hasNext = (pager.pageContext.next != nil)
	page = result.Resources

	return
}

// GetAllWithContext returns all results by invoking GetNextWithContext() repeatedly
// until all pages of results have been retrieved.
func (pager *ResourceAliasesForInstancePager) GetAllWithContext(ctx context.Context) (allItems []ResourceAlias, err error) {
	for pager.HasNext() {
		var nextPage []ResourceAlias
		nextPage, err = pager.GetNextWithContext(ctx)
		if err != nil {
			err = core.RepurposeSDKProblem(err, "error-getting-next-page")
			return
		}
		allItems = append(allItems, nextPage...)
	}
	return
}

// GetNext invokes GetNextWithContext() using context.Background() as the Context parameter.
func (pager *ResourceAliasesForInstancePager) GetNext() (page []ResourceAlias, err error) {
	page, err = pager.GetNextWithContext(context.Background())
	err = core.RepurposeSDKProblem(err, "")
	return
}

// GetAll invokes GetAllWithContext() using context.Background() as the Context parameter.
func (pager *ResourceAliasesForInstancePager) GetAll() (allItems []ResourceAlias, err error) {
	allItems, err = pager.GetAllWithContext(context.Background())
	err = core.RepurposeSDKProblem(err, "")
	return
}

// ResourceKeysForInstancePager can be used to simplify the use of the "ListResourceKeysForInstance" method.
type ResourceKeysForInstancePager struct {
	hasNext     bool
	options     *ListResourceKeysForInstanceOptions
	client      *ResourceControllerV2
	pageContext struct {
		next *string
	}
}

// NewResourceKeysForInstancePager returns a new ResourceKeysForInstancePager instance.
func (resourceController *ResourceControllerV2) NewResourceKeysForInstancePager(options *ListResourceKeysForInstanceOptions) (pager *ResourceKeysForInstancePager, err error) {
	if options.Start != nil && *options.Start != "" {
		err = core.SDKErrorf(nil, "the 'options.Start' field should not be set", "no-query-setting", common.GetComponentInfo())
		return
	}

	var optionsCopy ListResourceKeysForInstanceOptions = *options
	pager = &ResourceKeysForInstancePager{
		hasNext: true,
		options: &optionsCopy,
		client:  resourceController,
	}
	return
}

// HasNext returns true if there are potentially more results to be retrieved.
func (pager *ResourceKeysForInstancePager) HasNext() bool {
	return pager.hasNext
}

// GetNextWithContext returns the next page of results using the specified Context.
func (pager *ResourceKeysForInstancePager) GetNextWithContext(ctx context.Context) (page []ResourceKey, err error) {
	if !pager.HasNext() {
		return nil, fmt.Errorf("no more results available")
	}

	pager.options.Start = pager.pageContext.next

	result, _, err := pager.client.ListResourceKeysForInstanceWithContext(ctx, pager.options)
	if err != nil {
		err = core.RepurposeSDKProblem(err, "error-getting-next-page")
		return
	}

	var next *string
	if result.NextURL != nil {
		var start *string
		start, err = core.GetQueryParam(result.NextURL, "start")
		if err != nil {
			errMsg := fmt.Sprintf("error retrieving 'start' query parameter from URL '%s': %s", *result.NextURL, err.Error())
			err = core.SDKErrorf(err, errMsg, "get-query-error", common.GetComponentInfo())
			return
		}
		next = start
	}
	pager.pageContext.next = next
	pager.hasNext = (pager.pageContext.next != nil)
	page = result.Resources

	return
}

// GetAllWithContext returns all results by invoking GetNextWithContext() repeatedly
// until all pages of results have been retrieved.
func (pager *ResourceKeysForInstancePager) GetAllWithContext(ctx context.Context) (allItems []ResourceKey, err error) {
	for pager.HasNext() {
		var nextPage []ResourceKey
		nextPage, err = pager.GetNextWithContext(ctx)
		if err != nil {
			err = core.RepurposeSDKProblem(err, "error-getting-next-page")
			return
		}
		allItems = append(allItems, nextPage...)
	}
	return
}

// GetNext invokes GetNextWithContext() using context.Background() as the Context parameter.
func (pager *ResourceKeysForInstancePager) GetNext() (page []ResourceKey, err error) {
	page, err = pager.GetNextWithContext(context.Background())
	err = core.RepurposeSDKProblem(err, "")
	return
}

// GetAll invokes GetAllWithContext() using context.Background() as the Context parameter.
func (pager *ResourceKeysForInstancePager) GetAll() (allItems []ResourceKey, err error) {
	allItems, err = pager.GetAllWithContext(context.Background())
	err = core.RepurposeSDKProblem(err, "")
	return
}

// ResourceKeysPager can be used to simplify the use of the "ListResourceKeys" method.
type ResourceKeysPager struct {
	hasNext     bool
	options     *ListResourceKeysOptions
	client      *ResourceControllerV2
	pageContext struct {
		next *string
	}
}

// NewResourceKeysPager returns a new ResourceKeysPager instance.
func (resourceController *ResourceControllerV2) NewResourceKeysPager(options *ListResourceKeysOptions) (pager *ResourceKeysPager, err error) {
	if options.Start != nil && *options.Start != "" {
		err = core.SDKErrorf(nil, "the 'options.Start' field should not be set", "no-query-setting", common.GetComponentInfo())
		return
	}

	var optionsCopy ListResourceKeysOptions = *options
	pager = &ResourceKeysPager{
		hasNext: true,
		options: &optionsCopy,
		client:  resourceController,
	}
	return
}

// HasNext returns true if there are potentially more results to be retrieved.
func (pager *ResourceKeysPager) HasNext() bool {
	return pager.hasNext
}

// GetNextWithContext returns the next page of results using the specified Context.
func (pager *ResourceKeysPager) GetNextWithContext(ctx context.Context) (page []ResourceKey, err error) {
	if !pager.HasNext() {
		return nil, fmt.Errorf("no more results available")
	}

	pager.options.Start = pager.pageContext.next

	result, _, err := pager.client.ListResourceKeysWithContext(ctx, pager.options)
	if err != nil {
		err = core.RepurposeSDKProblem(err, "error-getting-next-page")
		return
	}

	var next *string
	if result.NextURL != nil {
		var start *string
		start, err = core.GetQueryParam(result.NextURL, "start")
		if err != nil {
			errMsg := fmt.Sprintf("error retrieving 'start' query parameter from URL '%s': %s", *result.NextURL, err.Error())
			err = core.SDKErrorf(err, errMsg, "get-query-error", common.GetComponentInfo())
			return
		}
		next = start
	}
	pager.pageContext.next = next
	pager.hasNext = (pager.pageContext.next != nil)
	page = result.Resources

	return
}

// GetAllWithContext returns all results by invoking GetNextWithContext() repeatedly
// until all pages of results have been retrieved.
func (pager *ResourceKeysPager) GetAllWithContext(ctx context.Context) (allItems []ResourceKey, err error) {
	for pager.HasNext() {
		var nextPage []ResourceKey
		nextPage, err = pager.GetNextWithContext(ctx)
		if err != nil {
			err = core.RepurposeSDKProblem(err, "error-getting-next-page")
			return
		}
		allItems = append(allItems, nextPage...)
	}
	return
}

// GetNext invokes GetNextWithContext() using context.Background() as the Context parameter.
func (pager *ResourceKeysPager) GetNext() (page []ResourceKey, err error) {
	page, err = pager.GetNextWithContext(context.Background())
	err = core.RepurposeSDKProblem(err, "")
	return
}

// GetAll invokes GetAllWithContext() using context.Background() as the Context parameter.
func (pager *ResourceKeysPager) GetAll() (allItems []ResourceKey, err error) {
	allItems, err = pager.GetAllWithContext(context.Background())
	err = core.RepurposeSDKProblem(err, "")
	return
}

// ResourceBindingsPager can be used to simplify the use of the "ListResourceBindings" method.
type ResourceBindingsPager struct {
	hasNext     bool
	options     *ListResourceBindingsOptions
	client      *ResourceControllerV2
	pageContext struct {
		next *string
	}
}

// NewResourceBindingsPager returns a new ResourceBindingsPager instance.
func (resourceController *ResourceControllerV2) NewResourceBindingsPager(options *ListResourceBindingsOptions) (pager *ResourceBindingsPager, err error) {
	if options.Start != nil && *options.Start != "" {
		err = core.SDKErrorf(nil, "the 'options.Start' field should not be set", "no-query-setting", common.GetComponentInfo())
		return
	}

	var optionsCopy ListResourceBindingsOptions = *options
	pager = &ResourceBindingsPager{
		hasNext: true,
		options: &optionsCopy,
		client:  resourceController,
	}
	return
}

// HasNext returns true if there are potentially more results to be retrieved.
func (pager *ResourceBindingsPager) HasNext() bool {
	return pager.hasNext
}

// GetNextWithContext returns the next page of results using the specified Context.
func (pager *ResourceBindingsPager) GetNextWithContext(ctx context.Context) (page []ResourceBinding, err error) {
	if !pager.HasNext() {
		return nil, fmt.Errorf("no more results available")
	}

	pager.options.Start = pager.pageContext.next

	result, _, err := pager.client.ListResourceBindingsWithContext(ctx, pager.options)
	if err != nil {
		err = core.RepurposeSDKProblem(err, "error-getting-next-page")
		return
	}

	var next *string
	if result.NextURL != nil {
		var start *string
		start, err = core.GetQueryParam(result.NextURL, "start")
		if err != nil {
			errMsg := fmt.Sprintf("error retrieving 'start' query parameter from URL '%s': %s", *result.NextURL, err.Error())
			err = core.SDKErrorf(err, errMsg, "get-query-error", common.GetComponentInfo())
			return
		}
		next = start
	}
	pager.pageContext.next = next
	pager.hasNext = (pager.pageContext.next != nil)
	page = result.Resources

	return
}

// GetAllWithContext returns all results by invoking GetNextWithContext() repeatedly
// until all pages of results have been retrieved.
func (pager *ResourceBindingsPager) GetAllWithContext(ctx context.Context) (allItems []ResourceBinding, err error) {
	for pager.HasNext() {
		var nextPage []ResourceBinding
		nextPage, err = pager.GetNextWithContext(ctx)
		if err != nil {
			err = core.RepurposeSDKProblem(err, "error-getting-next-page")
			return
		}
		allItems = append(allItems, nextPage...)
	}
	return
}

// GetNext invokes GetNextWithContext() using context.Background() as the Context parameter.
func (pager *ResourceBindingsPager) GetNext() (page []ResourceBinding, err error) {
	page, err = pager.GetNextWithContext(context.Background())
	err = core.RepurposeSDKProblem(err, "")
	return
}

// GetAll invokes GetAllWithContext() using context.Background() as the Context parameter.
func (pager *ResourceBindingsPager) GetAll() (allItems []ResourceBinding, err error) {
	allItems, err = pager.GetAllWithContext(context.Background())
	err = core.RepurposeSDKProblem(err, "")
	return
}

// ResourceAliasesPager can be used to simplify the use of the "ListResourceAliases" method.
type ResourceAliasesPager struct {
	hasNext     bool
	options     *ListResourceAliasesOptions
	client      *ResourceControllerV2
	pageContext struct {
		next *string
	}
}

// NewResourceAliasesPager returns a new ResourceAliasesPager instance.
func (resourceController *ResourceControllerV2) NewResourceAliasesPager(options *ListResourceAliasesOptions) (pager *ResourceAliasesPager, err error) {
	if options.Start != nil && *options.Start != "" {
		err = core.SDKErrorf(nil, "the 'options.Start' field should not be set", "no-query-setting", common.GetComponentInfo())
		return
	}

	var optionsCopy ListResourceAliasesOptions = *options
	pager = &ResourceAliasesPager{
		hasNext: true,
		options: &optionsCopy,
		client:  resourceController,
	}
	return
}

// HasNext returns true if there are potentially more results to be retrieved.
func (pager *ResourceAliasesPager) HasNext() bool {
	return pager.hasNext
}

// GetNextWithContext returns the next page of results using the specified Context.
func (pager *ResourceAliasesPager) GetNextWithContext(ctx context.Context) (page []ResourceAlias, err error) {
	if !pager.HasNext() {
		return nil, fmt.Errorf("no more results available")
	}

	pager.options.Start = pager.pageContext.next

	result, _, err := pager.client.ListResourceAliasesWithContext(ctx, pager.options)
	if err != nil {
		err = core.RepurposeSDKProblem(err, "error-getting-next-page")
		return
	}

	var next *string
	if result.NextURL != nil {
		var start *string
		start, err = core.GetQueryParam(result.NextURL, "start")
		if err != nil {
			errMsg := fmt.Sprintf("error retrieving 'start' query parameter from URL '%s': %s", *result.NextURL, err.Error())
			err = core.SDKErrorf(err, errMsg, "get-query-error", common.GetComponentInfo())
			return
		}
		next = start
	}
	pager.pageContext.next = next
	pager.hasNext = (pager.pageContext.next != nil)
	page = result.Resources

	return
}

// GetAllWithContext returns all results by invoking GetNextWithContext() repeatedly
// until all pages of results have been retrieved.
func (pager *ResourceAliasesPager) GetAllWithContext(ctx context.Context) (allItems []ResourceAlias, err error) {
	for pager.HasNext() {
		var nextPage []ResourceAlias
		nextPage, err = pager.GetNextWithContext(ctx)
		if err != nil {
			err = core.RepurposeSDKProblem(err, "error-getting-next-page")
			return
		}
		allItems = append(allItems, nextPage...)
	}
	return
}

// GetNext invokes GetNextWithContext() using context.Background() as the Context parameter.
func (pager *ResourceAliasesPager) GetNext() (page []ResourceAlias, err error) {
	page, err = pager.GetNextWithContext(context.Background())
	err = core.RepurposeSDKProblem(err, "")
	return
}

// GetAll invokes GetAllWithContext() using context.Background() as the Context parameter.
func (pager *ResourceAliasesPager) GetAll() (allItems []ResourceAlias, err error) {
	allItems, err = pager.GetAllWithContext(context.Background())
	err = core.RepurposeSDKProblem(err, "")
	return
}

// ResourceBindingsForAliasPager can be used to simplify the use of the "ListResourceBindingsForAlias" method.
type ResourceBindingsForAliasPager struct {
	hasNext     bool
	options     *ListResourceBindingsForAliasOptions
	client      *ResourceControllerV2
	pageContext struct {
		next *string
	}
}

// NewResourceBindingsForAliasPager returns a new ResourceBindingsForAliasPager instance.
func (resourceController *ResourceControllerV2) NewResourceBindingsForAliasPager(options *ListResourceBindingsForAliasOptions) (pager *ResourceBindingsForAliasPager, err error) {
	if options.Start != nil && *options.Start != "" {
		err = core.SDKErrorf(nil, "the 'options.Start' field should not be set", "no-query-setting", common.GetComponentInfo())
		return
	}

	var optionsCopy ListResourceBindingsForAliasOptions = *options
	pager = &ResourceBindingsForAliasPager{
		hasNext: true,
		options: &optionsCopy,
		client:  resourceController,
	}
	return
}

// HasNext returns true if there are potentially more results to be retrieved.
func (pager *ResourceBindingsForAliasPager) HasNext() bool {
	return pager.hasNext
}

// GetNextWithContext returns the next page of results using the specified Context.
func (pager *ResourceBindingsForAliasPager) GetNextWithContext(ctx context.Context) (page []ResourceBinding, err error) {
	if !pager.HasNext() {
		return nil, fmt.Errorf("no more results available")
	}

	pager.options.Start = pager.pageContext.next

	result, _, err := pager.client.ListResourceBindingsForAliasWithContext(ctx, pager.options)
	if err != nil {
		err = core.RepurposeSDKProblem(err, "error-getting-next-page")
		return
	}

	var next *string
	if result.NextURL != nil {
		var start *string
		start, err = core.GetQueryParam(result.NextURL, "start")
		if err != nil {
			errMsg := fmt.Sprintf("error retrieving 'start' query parameter from URL '%s': %s", *result.NextURL, err.Error())
			err = core.SDKErrorf(err, errMsg, "get-query-error", common.GetComponentInfo())
			return
		}
		next = start
	}
	pager.pageContext.next = next
	pager.hasNext = (pager.pageContext.next != nil)
	page = result.Resources

	return
}

// GetAllWithContext returns all results by invoking GetNextWithContext() repeatedly
// until all pages of results have been retrieved.
func (pager *ResourceBindingsForAliasPager) GetAllWithContext(ctx context.Context) (allItems []ResourceBinding, err error) {
	for pager.HasNext() {
		var nextPage []ResourceBinding
		nextPage, err = pager.GetNextWithContext(ctx)
		if err != nil {
			err = core.RepurposeSDKProblem(err, "error-getting-next-page")
			return
		}
		allItems = append(allItems, nextPage...)
	}
	return
}

// GetNext invokes GetNextWithContext() using context.Background() as the Context parameter.
func (pager *ResourceBindingsForAliasPager) GetNext() (page []ResourceBinding, err error) {
	page, err = pager.GetNextWithContext(context.Background())
	err = core.RepurposeSDKProblem(err, "")
	return
}

// GetAll invokes GetAllWithContext() using context.Background() as the Context parameter.
func (pager *ResourceBindingsForAliasPager) GetAll() (allItems []ResourceBinding, err error) {
	allItems, err = pager.GetAllWithContext(context.Background())
	err = core.RepurposeSDKProblem(err, "")
	return
}
