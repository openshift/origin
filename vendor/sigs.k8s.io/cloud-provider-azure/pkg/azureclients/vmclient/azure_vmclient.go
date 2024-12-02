/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vmclient

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2022-08-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"

	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	azclients "sigs.k8s.io/cloud-provider-azure/pkg/azureclients"
	"sigs.k8s.io/cloud-provider-azure/pkg/azureclients/armclient"
	"sigs.k8s.io/cloud-provider-azure/pkg/metrics"
	"sigs.k8s.io/cloud-provider-azure/pkg/retry"
)

var _ Interface = &Client{}

const vmResourceType = "Microsoft.Compute/virtualMachines"

// Client implements VirtualMachine client Interface.
type Client struct {
	armClient      armclient.Interface
	subscriptionID string
	cloudName      string

	// Rate limiting configures.
	rateLimiterReader flowcontrol.RateLimiter
	rateLimiterWriter flowcontrol.RateLimiter

	// ARM throttling configures.
	RetryAfterReader time.Time
	RetryAfterWriter time.Time
}

// New creates a new VirtualMachine client with ratelimiting.
func New(config *azclients.ClientConfig) *Client {
	baseURI := config.ResourceManagerEndpoint
	authorizer := config.Authorizer
	apiVersion := APIVersion
	if strings.EqualFold(config.CloudName, AzureStackCloudName) && !config.DisableAzureStackCloud {
		apiVersion = AzureStackCloudAPIVersion
	}
	armClient := armclient.New(authorizer, *config, baseURI, apiVersion)
	rateLimiterReader, rateLimiterWriter := azclients.NewRateLimiter(config.RateLimitConfig)

	if azclients.RateLimitEnabled(config.RateLimitConfig) {
		klog.V(2).Infof("Azure VirtualMachine client (read ops) using rate limit config: QPS=%g, bucket=%d",
			config.RateLimitConfig.CloudProviderRateLimitQPS,
			config.RateLimitConfig.CloudProviderRateLimitBucket)
		klog.V(2).Infof("Azure VirtualMachine client (write ops) using rate limit config: QPS=%g, bucket=%d",
			config.RateLimitConfig.CloudProviderRateLimitQPSWrite,
			config.RateLimitConfig.CloudProviderRateLimitBucketWrite)
	}

	client := &Client{
		armClient:         armClient,
		rateLimiterReader: rateLimiterReader,
		rateLimiterWriter: rateLimiterWriter,
		subscriptionID:    config.SubscriptionID,
		cloudName:         config.CloudName,
	}

	return client
}

// Get gets a VirtualMachine.
func (c *Client) Get(ctx context.Context, resourceGroupName string, VMName string, expand compute.InstanceViewTypes) (compute.VirtualMachine, *retry.Error) {
	mc := metrics.NewMetricContext("vm", "get", resourceGroupName, c.subscriptionID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterReader.TryAccept() {
		mc.RateLimitedCount()
		return compute.VirtualMachine{}, retry.GetRateLimitError(false, "VMGet")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterReader.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("VMGet", "client throttled", c.RetryAfterReader)
		return compute.VirtualMachine{}, rerr
	}

	result, rerr := c.getVM(ctx, resourceGroupName, VMName, expand)
	mc.Observe(rerr)
	if rerr != nil {
		if rerr.IsThrottled() {
			// Update RetryAfterReader so that no more requests would be sent until RetryAfter expires.
			c.RetryAfterReader = rerr.RetryAfter
		}

		return result, rerr
	}

	return result, nil
}

// getVM gets a VirtualMachine.
func (c *Client) getVM(ctx context.Context, resourceGroupName string, VMName string, expand compute.InstanceViewTypes) (compute.VirtualMachine, *retry.Error) {
	resourceID := armclient.GetResourceID(
		c.subscriptionID,
		resourceGroupName,
		vmResourceType,
		VMName,
	)
	result := compute.VirtualMachine{}

	response, rerr := c.armClient.GetResourceWithExpandQuery(ctx, resourceID, string(expand))
	defer c.armClient.CloseResponse(ctx, response)
	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vm.get.request", resourceID, rerr.Error())
		return result, rerr
	}

	err := autorest.Respond(
		response,
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result))
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vm.get.respond", resourceID, err)
		return result, retry.GetError(response, err)
	}

	result.Response = autorest.Response{Response: response}
	return result, nil
}

// List gets a list of VirtualMachine in the resourceGroupName.
func (c *Client) List(ctx context.Context, resourceGroupName string) ([]compute.VirtualMachine, *retry.Error) {
	return c.list(ctx, resourceGroupName, false)
}

// ListWithInstanceView gets a list of VirtualMachine in the resourceGroupName with InstanceView.
func (c *Client) ListWithInstanceView(ctx context.Context, resourceGroupName string) ([]compute.VirtualMachine, *retry.Error) {
	return c.list(ctx, resourceGroupName, true)
}

func (c *Client) list(ctx context.Context, resourceGroupName string, withInstanceView bool) ([]compute.VirtualMachine, *retry.Error) {
	mc := metrics.NewMetricContext("vm", "list", resourceGroupName, c.subscriptionID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterReader.TryAccept() {
		mc.RateLimitedCount()
		return nil, retry.GetRateLimitError(false, "VMList")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterReader.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("VMList", "client throttled", c.RetryAfterReader)
		return nil, rerr
	}

	result, rerr := c.listVM(ctx, resourceGroupName, withInstanceView)
	mc.Observe(rerr)
	if rerr != nil {
		if rerr.IsThrottled() {
			// Update RetryAfterReader so that no more requests would be sent until RetryAfter expires.
			c.RetryAfterReader = rerr.RetryAfter
		}

		return result, rerr
	}

	return result, nil
}

// listVM gets a list of VirtualMachines in the resourceGroupName.
func (c *Client) listVM(ctx context.Context, resourceGroupName string, withInstanceView bool) ([]compute.VirtualMachine, *retry.Error) {
	resourceID := armclient.GetResourceListID(c.subscriptionID, resourceGroupName, vmResourceType)

	result := make([]compute.VirtualMachine, 0)
	page := &VirtualMachineListResultPage{}
	page.fn = c.listNextResults

	var resp *http.Response
	var rerr *retry.Error
	if withInstanceView {
		queries := make(map[string]interface{})
		queries["$expand"] = autorest.Encode("query", "instanceView")
		resp, rerr = c.armClient.GetResourceWithQueries(ctx, resourceID, queries)
	} else {
		resp, rerr = c.armClient.GetResource(ctx, resourceID)
	}
	defer c.armClient.CloseResponse(ctx, resp)
	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vm.list.request", resourceID, rerr.Error())
		return result, rerr
	}

	var err error
	page.vmlr, err = c.listResponder(resp)
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vm.list.respond", resourceID, err)
		return result, retry.GetError(resp, err)
	}

	for {
		result = append(result, page.Values()...)

		// Abort the loop when there's no nextLink in the response.
		if ptr.Deref(page.Response().NextLink, "") == "" {
			break
		}

		if err = page.NextWithContext(ctx); err != nil {
			klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vm.list.next", resourceID, err)
			return result, retry.GetError(page.Response().Response.Response, err)
		}
	}

	return result, nil
}

// ListVmssFlexVMsWithoutInstanceView gets a list of VirtualMachine in the VMSS Flex without InstanceView.
func (c *Client) ListVmssFlexVMsWithoutInstanceView(ctx context.Context, vmssFlexID string) ([]compute.VirtualMachine, *retry.Error) {
	mc := metrics.NewMetricContext("vm", "list", "", c.subscriptionID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterReader.TryAccept() {
		mc.RateLimitedCount()
		return nil, retry.GetRateLimitError(false, "VMList")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterReader.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("VMList", "client throttled", c.RetryAfterReader)
		return nil, rerr
	}

	result, rerr := c.listVmssFlexVMs(ctx, vmssFlexID, false)
	mc.Observe(rerr)
	if rerr != nil {
		if rerr.IsThrottled() {
			// Update RetryAfterReader so that no more requests would be sent until RetryAfter expires.
			c.RetryAfterReader = rerr.RetryAfter
		}

		return result, rerr
	}

	return result, nil
}

// ListVmssFlexVMsWithOnlyInstanceView gets a list of VirtualMachine in the VMSS Flex with only InstanceView.
func (c *Client) ListVmssFlexVMsWithOnlyInstanceView(ctx context.Context, vmssFlexID string) ([]compute.VirtualMachine, *retry.Error) {
	mc := metrics.NewMetricContext("vm", "list", "", c.subscriptionID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterReader.TryAccept() {
		mc.RateLimitedCount()
		return nil, retry.GetRateLimitError(false, "VMList")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterReader.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("VMList", "client throttled", c.RetryAfterReader)
		return nil, rerr
	}

	result, rerr := c.listVmssFlexVMs(ctx, vmssFlexID, true)
	mc.Observe(rerr)
	if rerr != nil {
		if rerr.IsThrottled() {
			// Update RetryAfterReader so that no more requests would be sent until RetryAfter expires.
			c.RetryAfterReader = rerr.RetryAfter
		}

		return result, rerr
	}

	return result, nil
}

// listVmssFlexVMs gets a list of VirtualMachines in the VMSS Flex.
func (c *Client) listVmssFlexVMs(ctx context.Context, vmssFlexID string, statusOnly bool) ([]compute.VirtualMachine, *retry.Error) {
	resourceID := armclient.GetProviderResourceID(c.subscriptionID, vmResourceType)

	result := make([]compute.VirtualMachine, 0)
	page := &VirtualMachineListResultPage{}
	page.fn = c.listNextResults

	queries := make(map[string]interface{})
	queries["$filter"] = "'virtualMachineScaleSet/id' eq '" + vmssFlexID + "'"
	if statusOnly {
		queries["statusOnly"] = true
	}
	resp, rerr := c.armClient.GetResourceWithQueries(ctx, resourceID, queries)
	defer c.armClient.CloseResponse(ctx, resp)
	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vm.list.request", resourceID, rerr.Error())
		return result, rerr
	}

	var err error
	page.vmlr, err = c.listResponder(resp)
	if err != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vm.list.respond", resourceID, err)
		return result, retry.GetError(resp, err)
	}

	for {
		result = append(result, page.Values()...)

		// Abort the loop when there's no nextLink in the response.
		if ptr.Deref(page.Response().NextLink, "") == "" {
			break
		}

		if err = page.NextWithContext(ctx); err != nil {
			klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vm.list.next", resourceID, err)
			return result, retry.GetError(page.Response().Response.Response, err)
		}
	}

	return result, nil
}

// Update updates a VirtualMachine.
func (c *Client) Update(ctx context.Context, resourceGroupName string, VMName string, parameters compute.VirtualMachineUpdate, source string) (*compute.VirtualMachine, *retry.Error) {
	mc := metrics.NewMetricContext("vm", "update", resourceGroupName, c.subscriptionID, source)

	// Report errors if the client is rate limited.
	if !c.rateLimiterWriter.TryAccept() {
		mc.RateLimitedCount()
		return nil, retry.GetRateLimitError(true, "VMUpdate")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterWriter.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("VMUpdate", "client throttled", c.RetryAfterWriter)
		return nil, rerr
	}

	result, rerr := c.updateVM(ctx, resourceGroupName, VMName, parameters, source)
	mc.Observe(rerr)
	if rerr != nil {
		if rerr.IsThrottled() {
			// Update RetryAfterReader so that no more requests would be sent until RetryAfter expires.
			c.RetryAfterWriter = rerr.RetryAfter
		}
		return result, rerr
	}
	return result, rerr
}

// UpdateAsync updates a VirtualMachine asynchronously
func (c *Client) UpdateAsync(ctx context.Context, resourceGroupName string, VMName string, parameters compute.VirtualMachineUpdate, source string) (*azure.Future, *retry.Error) {
	mc := metrics.NewMetricContext("vm", "updateasync", resourceGroupName, c.subscriptionID, source)

	// Report errors if the client is rate limited.
	if !c.rateLimiterWriter.TryAccept() {
		mc.RateLimitedCount()
		return nil, retry.GetRateLimitError(true, "VMUpdateAsync")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterWriter.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("VMUpdateAsync", "client throttled", c.RetryAfterWriter)
		return nil, rerr
	}

	resourceID := armclient.GetResourceID(
		c.subscriptionID,
		resourceGroupName,
		vmResourceType,
		VMName,
	)

	future, rerr := c.armClient.PatchResourceAsync(ctx, resourceID, parameters)
	mc.Observe(rerr)
	if rerr != nil {
		if rerr.IsThrottled() {
			// Update RetryAfterReader so that no more requests would be sent until RetryAfter expires.
			c.RetryAfterWriter = rerr.RetryAfter
		}

		return nil, rerr
	}

	return future, nil
}

// WaitForUpdateResult waits for the response of the update request
func (c *Client) WaitForUpdateResult(ctx context.Context, future *azure.Future, resourceGroupName, source string) (*compute.VirtualMachine, *retry.Error) {
	mc := metrics.NewMetricContext("vm", "wait_for_update_result", resourceGroupName, c.subscriptionID, source)
	response, err := c.armClient.WaitForAsyncOperationResult(ctx, future, "VMWaitForUpdateResult")
	mc.Observe(retry.NewErrorOrNil(false, err))
	defer c.armClient.CloseResponse(ctx, response)

	if err != nil {
		if response != nil {
			klog.V(5).Infof("Received error in WaitForAsyncOperationResult: '%s', response code %d", err.Error(), response.StatusCode)
		} else {
			klog.V(5).Infof("Received error in WaitForAsyncOperationResult: '%s', no response", err.Error())
		}
		return nil, retry.GetError(response, err)
	}
	if response != nil && response.StatusCode != http.StatusNoContent {
		result, rerr := c.updateResponder(response)
		if rerr != nil {
			klog.V(5).Infof("Received error in WaitForAsyncOperationResult updateResponder: '%s'", rerr.Error())
		}

		return result, rerr
	}

	result := &compute.VirtualMachine{}
	result.Response = autorest.Response{Response: response}
	return result, nil
}

// updateVM updates a VirtualMachine.
func (c *Client) updateVM(ctx context.Context, resourceGroupName string, VMName string, parameters compute.VirtualMachineUpdate, _ string) (*compute.VirtualMachine, *retry.Error) {
	resourceID := armclient.GetResourceID(
		c.subscriptionID,
		resourceGroupName,
		vmResourceType,
		VMName,
	)

	response, rerr := c.armClient.PatchResource(ctx, resourceID, parameters)
	defer c.armClient.CloseResponse(ctx, response)
	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vm.put.request", resourceID, rerr.Error())
		return nil, rerr
	}

	if response != nil && response.StatusCode != http.StatusNoContent {
		result, rerr := c.updateResponder(response)
		if rerr != nil {
			klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vm.put.respond", resourceID, rerr.Error())
		}
		return result, rerr
	}

	result := &compute.VirtualMachine{}
	result.Response = autorest.Response{Response: response}
	return result, nil
}

func (c *Client) updateResponder(resp *http.Response) (*compute.VirtualMachine, *retry.Error) {
	result := &compute.VirtualMachine{}
	err := autorest.Respond(
		resp,
		azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusCreated),
		autorest.ByUnmarshallingJSON(&result),
		autorest.ByClosing())
	result.Response = autorest.Response{Response: resp}
	return result, retry.GetError(resp, err)
}

func (c *Client) listResponder(resp *http.Response) (result compute.VirtualMachineListResult, err error) {
	err = autorest.Respond(
		resp,
		autorest.ByIgnoring(),
		azure.WithErrorUnlessStatusCode(http.StatusOK),
		autorest.ByUnmarshallingJSON(&result),
		autorest.ByClosing(),
	)
	result.Response = autorest.Response{Response: resp}
	return
}

// vmListResultPreparer prepares a request to retrieve the next set of results.
// It returns nil if no more results exist.
func (c *Client) vmListResultPreparer(ctx context.Context, vmlr compute.VirtualMachineListResult) (*http.Request, error) {
	if vmlr.NextLink == nil || len(ptr.Deref(vmlr.NextLink, "")) < 1 {
		return nil, nil
	}

	decorators := []autorest.PrepareDecorator{
		autorest.WithBaseURL(ptr.Deref(vmlr.NextLink, "")),
	}
	return c.armClient.PrepareGetRequest(ctx, decorators...)
}

// listNextResults retrieves the next set of results, if any.
func (c *Client) listNextResults(ctx context.Context, lastResults compute.VirtualMachineListResult) (result compute.VirtualMachineListResult, err error) {
	req, err := c.vmListResultPreparer(ctx, lastResults)
	if err != nil {
		return result, autorest.NewErrorWithError(err, "vmclient", "listNextResults", nil, "Failure preparing next results request")
	}
	if req == nil {
		return
	}

	resp, rerr := c.armClient.Send(ctx, req)
	defer c.armClient.CloseResponse(ctx, resp)
	if rerr != nil {
		result.Response = autorest.Response{Response: resp}
		return result, autorest.NewErrorWithError(rerr.Error(), "vmclient", "listNextResults", resp, "Failure sending next results request")
	}

	result, err = c.listResponder(resp)
	if err != nil {
		err = autorest.NewErrorWithError(err, "vmclient", "listNextResults", resp, "Failure responding to next results request")
	}

	return
}

// VirtualMachineListResultPage contains a page of VirtualMachine values.
type VirtualMachineListResultPage struct {
	fn   func(context.Context, compute.VirtualMachineListResult) (compute.VirtualMachineListResult, error)
	vmlr compute.VirtualMachineListResult
}

// NextWithContext advances to the next page of values.  If there was an error making
// the request the page does not advance and the error is returned.
func (page *VirtualMachineListResultPage) NextWithContext(ctx context.Context) (err error) {
	next, err := page.fn(ctx, page.vmlr)
	if err != nil {
		return err
	}
	page.vmlr = next
	return nil
}

// Next advances to the next page of values.  If there was an error making
// the request the page does not advance and the error is returned.
// Deprecated: Use NextWithContext() instead.
func (page *VirtualMachineListResultPage) Next() error {
	return page.NextWithContext(context.Background())
}

// NotDone returns true if the page enumeration should be started or is not yet complete.
func (page VirtualMachineListResultPage) NotDone() bool {
	return !page.vmlr.IsEmpty()
}

// Response returns the raw server response from the last page request.
func (page VirtualMachineListResultPage) Response() compute.VirtualMachineListResult {
	return page.vmlr
}

// Values returns the slice of values for the current page or nil if there are no values.
func (page VirtualMachineListResultPage) Values() []compute.VirtualMachine {
	if page.vmlr.IsEmpty() {
		return nil
	}
	return *page.vmlr.Value
}

// CreateOrUpdate creates or updates a VirtualMachine.
func (c *Client) CreateOrUpdate(ctx context.Context, resourceGroupName string, VMName string, parameters compute.VirtualMachine, source string) *retry.Error {
	mc := metrics.NewMetricContext("vm", "create_or_update", resourceGroupName, c.subscriptionID, source)

	// Report errors if the client is rate limited.
	if !c.rateLimiterWriter.TryAccept() {
		mc.RateLimitedCount()
		return retry.GetRateLimitError(true, "VMCreateOrUpdate")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterWriter.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("VMCreateOrUpdate", "client throttled", c.RetryAfterWriter)
		return rerr
	}

	rerr := c.createOrUpdateVM(ctx, resourceGroupName, VMName, parameters, source)
	mc.Observe(rerr)
	if rerr != nil {
		if rerr.IsThrottled() {
			// Update RetryAfterReader so that no more requests would be sent until RetryAfter expires.
			c.RetryAfterWriter = rerr.RetryAfter
		}

		return rerr
	}

	return nil
}

// createOrUpdateVM creates or updates a VirtualMachine.
func (c *Client) createOrUpdateVM(ctx context.Context, resourceGroupName string, VMName string, parameters compute.VirtualMachine, _ string) *retry.Error {
	resourceID := armclient.GetResourceID(
		c.subscriptionID,
		resourceGroupName,
		vmResourceType,
		VMName,
	)

	response, rerr := c.armClient.PutResource(ctx, resourceID, parameters)
	defer c.armClient.CloseResponse(ctx, response)
	if rerr != nil {
		klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vm.put.request", resourceID, rerr.Error())
		return rerr
	}

	if response != nil && response.StatusCode != http.StatusNoContent {
		_, rerr = c.createOrUpdateResponder(response)
		if rerr != nil {
			klog.V(5).Infof("Received error in %s: resourceID: %s, error: %s", "vm.put.respond", resourceID, rerr.Error())
			return rerr
		}
	}

	return nil
}

func (c *Client) createOrUpdateResponder(resp *http.Response) (*compute.VirtualMachine, *retry.Error) {
	result := &compute.VirtualMachine{}
	err := autorest.Respond(
		resp,
		azure.WithErrorUnlessStatusCode(http.StatusOK, http.StatusCreated),
		autorest.ByUnmarshallingJSON(&result),
		autorest.ByClosing())
	result.Response = autorest.Response{Response: resp}
	return result, retry.GetError(resp, err)
}

// Delete deletes a VirtualMachine.
func (c *Client) Delete(ctx context.Context, resourceGroupName string, VMName string) *retry.Error {
	mc := metrics.NewMetricContext("vm", "delete", resourceGroupName, c.subscriptionID, "")

	// Report errors if the client is rate limited.
	if !c.rateLimiterWriter.TryAccept() {
		mc.RateLimitedCount()
		return retry.GetRateLimitError(true, "VMDelete")
	}

	// Report errors if the client is throttled.
	if c.RetryAfterWriter.After(time.Now()) {
		mc.ThrottledCount()
		rerr := retry.GetThrottlingError("VMDelete", "client throttled", c.RetryAfterWriter)
		return rerr
	}

	rerr := c.deleteVM(ctx, resourceGroupName, VMName)
	mc.Observe(rerr)
	if rerr != nil {
		if rerr.IsThrottled() {
			// Update RetryAfterReader so that no more requests would be sent until RetryAfter expires.
			c.RetryAfterWriter = rerr.RetryAfter
		}

		return rerr
	}

	return nil
}

// deleteVM deletes a VirtualMachine.
func (c *Client) deleteVM(ctx context.Context, resourceGroupName string, VMName string) *retry.Error {
	resourceID := armclient.GetResourceID(
		c.subscriptionID,
		resourceGroupName,
		vmResourceType,
		VMName,
	)

	return c.armClient.DeleteResource(ctx, resourceID)
}
