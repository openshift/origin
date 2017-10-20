# Documentation

The [Open Service Broker API](https://github.com/openservicebrokerapi/servicebroker)
 describes an entity (service broker) that provides some set of capabilities
(services).  Services have different *plans* that describe different tiers of
the service.  New instances of the services are *provisioned* in order to be
used.  Some services can be *bound* to applications for programmatic use.

Example:

- Service: "database as a service"
- Instance: "My database"
- Binding: "Credentials to use my database in app 'guestbook'"

## Background Reading

Reading the
[API specification](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md) is 
recommended before reading this documentation.

## API Fundamentals

There are 7 operations in the API:

1.  Getting a broker's 'catalog' of services: [`Client.GetCatalog`](#getting-a-brokers-catalog)
2.  Provisioning a new instance of a service: [`Client.ProvisionInstance`](#provisioning-a-new-instance-of-a-service)
3.  Updating properties of an instance: [`Client.UpdateInstance`](#updating-properties-of-an-instance)
4.  Deprovisioning an instance: [`Client.DeprovisionInstance`](#deprovisioning-an-instance)
5.  Checking the status of an asynchronous operation (provision, update, or deprovision) on an instance: [`Client.PollLastOperation`](#provisioning-a-new-instance-of-a-service)
6.  Binding to an instance: [`Client.Bind`](#binding-to-an-instance)
7.  Unbinding from an instance: [`Client.Unbind`](#unbinding-from-an-instance)

### Getting a broker's catalog

A broker's catalog holds information about the services a broker provides and
their plans.  A platform implementing the OSB API must first get the broker's
catalog.

```go
import (
	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

func GetBrokerCatalog(URL string) (*osb.CatalogResponse, error) {
	config := osb.DefaultClientConfiguration()
	config.URL = URL

	client, err := osb.NewClient(config)
	if err != nil {
		return nil, err
	}

	return client.GetCatalog()
}
```

### Provisioning a new instance of a service

To provision a new instance of a service, call the `Client.Provision` method.

Key points:

1. `ProvisionInstance` returns a response from the broker for successful
   operations, or an error if the broker returned an error response or
   there was a problem communicating with the broker
2. Use the `IsHTTPError` method to test and convert errors from Brokers
   into the standard broker error type, allowing access to conventional
   broker-provided fields
3. The `response.Async` field indicates whether the broker is performing the
   provision concurrently; see the [`LastOperation`](#checking-the-status-of-an-async-operation)
   method for information about handling asynchronous operations

```go
import (
	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

func ProvisionService(client osb.Client, request osb.ProvisionRequest) (*osb.CatalogResponse, error) {
	request := &ProvisionRequest{
		InstanceID: "my-dbaas-service-instance",

		// Made up parameters for a hypothetical service
		ServiceID: "dbaas-service",
		PlanID:    "dbaas-gold-plan",
		Parameters: map[string]interface{}{
			"tablespace-page-cost":      100,
			"tablespace-io-concurrency": 5,
		},

		// Set the AcceptsIncomplete field to indicate that this client can
		// support asynchronous operations (provision, update, deprovision).
		AcceptsIncomplete: true,
	}

	// ProvisionInstance returns a response from the broker for successful
	// operations, or an error if the broker returned an error response or
	// there was a problem communicating with the broker.
	resp, err := client.ProvisionInstance(request)
	if err != nil {
		// Use the IsHTTPError method to test and convert errors from Brokers
		// into the standard broker error type, allowing access to conventional
		// broker-provided fields.
		errHttp, isError := osb.IsHTTPError(err)
		if isError {
			// handle error response from broker
		} else {
			// handle errors communicating with the broker
		}
	}

	// The response.Async field indicates whether the broker is performing the
	// provision concurrently.  See the LastOperation method for information
	// about handling asynchronous operations.
	if response.Async {
		// handle asynchronous operation
	}
}
```

### Updating properties of an instance

To update the plan and/or parameters of a service instance, call the `UpdateInstance` method.

Key points:

1. A service's plan may be changed only if that service is `PlanUpdatable`
2. `UpdateInstance` returns a response from the broker for successful
   operations, or an error if the broker returned an error response or
   there was a problem communicating with the broker
3. Use the `IsHTTPError` method to test and convert errors from Brokers
   into the standard broker error type, allowing access to conventional
   broker-provided fields
4. The `response.Async` field indicates whether the broker is performing the
   provision concurrently; see the [`LastOperation`](#checking-the-status-of-an-async-operation)
   method for information about handling asynchronous operations
5. Passing `PlanID` or `Parameters` fields to this operation indicates
   that the user wishes to update those fields; values for these fields
   should not be passed if those fields have not changed

```go
import (
	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

func UpdateService(client osb.Client) {
	newPlan := "dbaas-quadruple-plan",

	request := &osb.UpdateInstanceRequest{
		InstanceID:        "my-dbaas-service-instance",
		ServiceID:         "dbaas-service",
		AcceptsIncomplete: true,

		// Passing the plan indicates that the user
		// wants the plan to change.
		PlanID: &newPlan,

		// Passing a parameter indicates that the user
		// wants the parameter value to change.
		Parameters: map[string]interface{}{
			"tablespace-page-cost":      50,
			"tablespace-io-concurrency": 100,
		},
	}

	response, err := client.UpdateInstance(request)
	if err != nil {
		httpErr, isError := osb.IsHTTPError(err)
		if isError {
			// handle errors from broker
		} else {
			// handle errors communicating with broker
		}
	}

	if response.Async {
		// handle asynchronous update operation
	} else {
		// handle successful update
	}
}
```

### Deprovisioning an instance

To deprovision a service instance, call the `DeprovisionInstance` method.

Key points:

1. `DeprovisionInstance` returns a response from the broker for successful
   operations, or an error if the broker returned an error response or
   there was a problem communicating with the broker
2. Use the `IsHTTPError` method to test and convert errors from Brokers
   into the standard broker error type, allowing access to conventional
   broker-provided fields
3. An HTTP `Gone` response is equivalent to success -- use `IsGoneError` to
   test for this condition
4. The `response.Async` field indicates whether the broker is performing the
   deprovision concurrently; see the [`LastOperation`](#checking-the-status-of-an-async-operation)
   method for information about handling asynchronous operations

```go
import (
	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

func DeprovisionService(client osb.Client) {
	request := &osb.DeprovisionRequest{
		InstanceID:        "my-dbaas-service-instance",
		ServiceID:         "dbaas-service",
		PlanID:            "dbaas-gold-plan",
		AcceptsIncomplete: true,
	}

	response, err := client.DeprovisionInstance(request)
	if err != nil {
		httpErr, isError := osb.IsHTTPError(err)
		if isError {
			// handle errors from broker

			if osb.IsGoneError(httpErr) {
				// A 'gone' status code means that the service instance
				// doesn't exist.  This means there is no more work to do and
				// should be equivalent to a success.
			}
		} else {
			// handle errors communicating with broker
		}
	}

	if response.Async {
		// handle asynchronous deprovisions
	} else {
		// handle successful deprovision
	}
}
```

### Checking the status of an asynchronous operation

If the client returns a response from [`ProvisionInstance`](#provisioning-a-new-instance-of-a-service),
[`UpdateInstance`](#updating-properties-of-an-instance), or
[`DeprovisionInstance`](#deprovisioning-an-instance) with the `response.Async`
field set to true, it means the broker is executing the operation
asynchronously.  You must call the `PollLastOperation` method on the client to
check on the status of the operation.

```go
import (
	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

func PollServiceInstance(client osb.Client, deleting bool) error {
	request := &osb.LastOperationRequest{
		InstanceID: "my-dbaas-service-instance"
		ServiceID:  "dbaas-service",
		PlanID:     "dbaas-gold-plan",

		// Brokers may provide an identifying key for an asychronous operation.
		OperationKey: osb.OperationKey("12345")
	}
	
	response, err := client.PollLastOperation(request)
	if err != nil {
		// If the operation was for delete and we receive a http.StatusGone,
		// this is considered a success as per the spec.
		if osb.IsGoneError(err) && deleting {
			// handle instances that we were deprovisioning and that are now
			// gone
		}

		// The broker returned an error.  While polling last operation, this
		// represents an invalid response and callers should continue polling
		// last operation.
	}

	switch response.State {
	case osb.StateInProgress:
		// The operation is still in progress
	case osb.StateSucceeded:
		// The operation succeeded
	case osb.StateFailed:
		// The operation failed.
	}
}

```

### Binding to an instance

To create a new binding to an instance, call the `Bind` method.

Key points:

1. `Bind` returns a response from the broker for successful
   operations, or an error if the broker returned an error response or
   there was a problem communicating with the broker
2. Use the `IsHTTPError` method to test and convert errors from Brokers
   into the standard broker error type, allowing access to conventional
   broker-provided fields

```go
import (
	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

func BindToInstance(client osb.Client) {
	request := &osb.BindRequest{
		BindingID:  "binding-id",
		InstanceID: "instance-id",
		ServiceID:  "dbaas-service",
		PlanID:     "dbaas-gold-plan",

		// platforms might want to pass an identifier for applications here
		AppGUID: "app-guid",

		// pass parameters here
		Parameters: map[string]interface{}{},
	}

	response, err := brokerClient.Bind(request)
	if err != nil {
		httpErr, isError := osb.IsHTTPError(err)
		if isError {
			// handle errors from the broker
		} else {
			// handle errors communicating with the broker
		}
	}

	// do something with the credentials
}
```

### Unbinding from an instance

To unbind from a service instance, call the `Unbind` method.

Key points:

1. `Unbind` returns a response from the broker for successful
   operations, or an error if the broker returned an error response or
   there was a problem communicating with the broker
2. Use the `IsHTTPError` method to test and convert errors from Brokers
   into the standard broker error type, allowing access to conventional
   broker-provided fields

```go
import (
	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

func UnbindFromInstance(client osb.Client) {
	request := &osb.UnbindRequest{
		BindingID:  "binding-id",
		InstanceID: "instance-id",
		ServiceID:  "dbaas-service",
		PlanID:     "dbaas-gold-plan",
		AppGUID: "app-guid",
	}

	response, err := brokerClient.Unbind(request)
	if err != nil {
		httpErr, isError := osb.IsHTTPError(err)
		if isError {
			// handle errors from the broker
		} else {
			// handle errors communicating with the broker
		}
	}

	// handle successful unbind
}
```