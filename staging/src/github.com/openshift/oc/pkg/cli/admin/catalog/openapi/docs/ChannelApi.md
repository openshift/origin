# \ChannelApi

All URIs are relative to *https://localhost:5000*

Method | HTTP request | Description
------------- | ------------- | -------------
[**CreateChannel**](ChannelApi.md#CreateChannel) | **Post** /api/v1/packages/{namespace}/{package}/channels | Create a new channel
[**CreateChannelRelease**](ChannelApi.md#CreateChannelRelease) | **Post** /api/v1/packages/{namespace}/{package}/channels/{channel}/{release} | Add a release to a channel
[**DeleteChannel**](ChannelApi.md#DeleteChannel) | **Delete** /api/v1/packages/{namespace}/{package}/channels/{channel} | Delete channel
[**DeleteChannelRelease**](ChannelApi.md#DeleteChannelRelease) | **Delete** /api/v1/packages/{namespace}/{package}/channels/{channel}/{release} | Remove a release from the channel
[**ListChannels**](ChannelApi.md#ListChannels) | **Get** /api/v1/packages/{namespace}/{package}/channels | List channels
[**ShowChannel**](ChannelApi.md#ShowChannel) | **Get** /api/v1/packages/{namespace}/{package}/channels/{channel} | show channel



## CreateChannel

> Channel CreateChannel(ctx, name, namespace, package_)
Create a new channel

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**name** | **string**| Channel name | 
**namespace** | **string**| namespace | 
**package_** | **string**| package name | 

### Return type

[**Channel**](Channel.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## CreateChannelRelease

> Channel CreateChannelRelease(ctx, channel, namespace, package_, release)
Add a release to a channel

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**channel** | **string**| channel name | 
**namespace** | **string**| namespace | 
**package_** | **string**| full package name | 
**release** | **string**| Release name | 

### Return type

[**Channel**](Channel.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DeleteChannel

> []Channel DeleteChannel(ctx, namespace, channel, package_)
Delete channel

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**namespace** | **string**| namespace | 
**channel** | **string**| channel name | 
**package_** | **string**| full package name | 

### Return type

[**[]Channel**](Channel.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DeleteChannelRelease

> []Channel DeleteChannelRelease(ctx, channel, namespace, package_, release)
Remove a release from the channel

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**channel** | **string**| channel name | 
**namespace** | **string**| namespace | 
**package_** | **string**| full package name | 
**release** | **string**| Release name | 

### Return type

[**[]Channel**](Channel.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ListChannels

> []Channel ListChannels(ctx, namespace, package_)
List channels

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**namespace** | **string**| namespace | 
**package_** | **string**| package name | 

### Return type

[**[]Channel**](Channel.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ShowChannel

> []Channel ShowChannel(ctx, channel, namespace, package_)
show channel

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**channel** | **string**| channel name | 
**namespace** | **string**| namespace | 
**package_** | **string**| package name | 

### Return type

[**[]Channel**](Channel.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

