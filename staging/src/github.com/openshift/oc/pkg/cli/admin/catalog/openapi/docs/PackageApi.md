# \PackageApi

All URIs are relative to *https://localhost:5000*

Method | HTTP request | Description
------------- | ------------- | -------------
[**CreatePackage**](PackageApi.md#CreatePackage) | **Post** /api/v1/packages | Push new package release to the registry
[**DeletePackage**](PackageApi.md#DeletePackage) | **Delete** /api/v1/packages/{namespace}/{package}/{release}/{media_type} | Delete a package release
[**ListPackages**](PackageApi.md#ListPackages) | **Get** /api/v1/packages | List packages
[**PullPackage**](PackageApi.md#PullPackage) | **Get** /api/v1/packages/{namespace}/{package}/{release}/{media_type}/pull | Download the package
[**PullPackageJson**](PackageApi.md#PullPackageJson) | **Get** /api/v1/packages/{namespace}/{package}/{release}/{media_type}/pull/json | Download the package
[**ShowPackage**](PackageApi.md#ShowPackage) | **Get** /api/v1/packages/{namespace}/{package}/{release}/{media_type} | Show a package
[**ShowPackageManifests**](PackageApi.md#ShowPackageManifests) | **Get** /api/v1/packages/{namespace}/{package}/{release} | List all manifests for a package
[**ShowPackageReleases**](PackageApi.md#ShowPackageReleases) | **Get** /api/v1/packages/{namespace}/{package} | List all releases for a package



## CreatePackage

> Package CreatePackage(ctx, body, optional)
Push new package release to the registry

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**body** | [**PostPackage**](PostPackage.md)| Package object to be added to the registry | 
 **optional** | ***CreatePackageOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a CreatePackageOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------

 **force** | **optional.Bool**| Force push the release (if allowed) | [default to false]

### Return type

[**Package**](Package.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DeletePackage

> Package DeletePackage(ctx, namespace, package_, release, mediaType)
Delete a package release

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**namespace** | **string**| namespace | 
**package_** | **string**| package name | 
**release** | **string**| release name | 
**mediaType** | **string**| content type | 

### Return type

[**Package**](Package.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ListPackages

> []PackageDescription ListPackages(ctx, optional)
List packages

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
 **optional** | ***ListPackagesOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ListPackagesOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **namespace** | **optional.String**| Filter by namespace | 
 **query** | **optional.String**| Lookup value for package search | 
 **mediaType** | **optional.String**| Filter by media-type | 

### Return type

[**[]PackageDescription**](PackageDescription.md)

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


## ShowPackage

> Package ShowPackage(ctx, namespace, package_, release, mediaType)
Show a package

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**namespace** | **string**| namespace | 
**package_** | **string**| package name | 
**release** | **string**| release name | 
**mediaType** | **string**| content type | 

### Return type

[**Package**](Package.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ShowPackageManifests

> []Manifest ShowPackageManifests(ctx, namespace, package_, release)
List all manifests for a package

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**namespace** | **string**| namespace | 
**package_** | **string**| package name | 
**release** | **string**| release name | 

### Return type

[**[]Manifest**](Manifest.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ShowPackageReleases

> []Manifest ShowPackageReleases(ctx, namespace, package_, optional)
List all releases for a package

### Required Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**namespace** | **string**| namespace | 
**package_** | **string**| package name | 
 **optional** | ***ShowPackageReleasesOpts** | optional parameters | nil if no parameters

### Optional Parameters

Optional parameters are passed through a pointer to a ShowPackageReleasesOpts struct


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **mediaType** | **optional.String**| Filter by media-type | 

### Return type

[**[]Manifest**](Manifest.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

