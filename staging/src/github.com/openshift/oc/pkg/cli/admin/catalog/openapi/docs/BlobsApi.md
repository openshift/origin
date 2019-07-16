# \BlobsApi

All URIs are relative to *https://localhost:5000*

Method | HTTP request | Description
------------- | ------------- | -------------
[**PullBlob**](BlobsApi.md#PullBlob) | **Get** /api/v1/packages/{namespace}/{package}/blobs/sha256/{digest} | Pull a package blob by digest
[**PullBlobJson**](BlobsApi.md#PullBlobJson) | **Get** /api/v1/packages/{namespace}/{package}/blobs/sha256/{digest}/json | Pull a package blob by digest
[**PullPackage**](BlobsApi.md#PullPackage) | **Get** /api/v1/packages/{namespace}/{package}/{release}/{media_type}/pull | Download the package
[**PullPackageJson**](BlobsApi.md#PullPackageJson) | **Get** /api/v1/packages/{namespace}/{package}/{release}/{media_type}/pull/json | Download the package



## PullBlob

> *os.File PullBlob(ctx, namespace, package_, digest)
Pull a package blob by digest

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**namespace** | **string**| namespace | 
**package_** | **string**| package name | 
**digest** | **string**| content digest | 

### Return type

[***os.File**](*os.File.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/x-gzip

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## PullBlobJson

> PullJson PullBlobJson(ctx, namespace, package_, digest, optional)
Pull a package blob by digest

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**namespace** | **string**| namespace | 
**package_** | **string**| package name | 
**digest** | **string**| content digest | 
 **optional** | ***PullBlobJsonOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a PullBlobJsonOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **format** | **optional.String**| return format type(json or gzip) | [default to gzip]

### Return type

[**PullJson**](PullJson.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## PullPackage

> *os.File PullPackage(ctx, namespace, package_, release, mediaType, optional)
Download the package

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**namespace** | **string**| namespace | 
**package_** | **string**| package name | 
**release** | **string**| release name | 
**mediaType** | **string**| content type | 
 **optional** | ***PullPackageOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a PullPackageOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------




 **format** | **optional.String**| reponse format: json or blob | 

### Return type

[***os.File**](*os.File.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/x-gzip

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## PullPackageJson

> PullJson PullPackageJson(ctx, namespace, package_, release, mediaType, optional)
Download the package

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**namespace** | **string**| namespace | 
**package_** | **string**| package name | 
**release** | **string**| release name | 
**mediaType** | **string**| content type | 
 **optional** | ***PullPackageJsonOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a PullPackageJsonOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------




 **format** | **optional.String**| reponse format: json or blob | 

### Return type

[**PullJson**](PullJson.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

