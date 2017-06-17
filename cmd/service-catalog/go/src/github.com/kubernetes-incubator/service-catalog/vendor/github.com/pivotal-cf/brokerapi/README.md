# brokerapi

[![Build Status](https://travis-ci.org/pivotal-cf/brokerapi.svg?branch=master)](https://travis-ci.org/pivotal-cf/brokerapi)

A Go package for building [V2 Cloud Foundry Service Brokers](https://docs.cloudfoundry.org/services/api.html).

## [Docs](https://godoc.org/github.com/pivotal-cf/brokerapi)

## Dependencies

- Go 1.7+
- [lager](https://github.com/cloudfoundry/lager)
- [gorilla/mux](https://github.com/gorilla/mux)

## Usage

`brokerapi` defines a [`ServiceBroker`](https://godoc.org/github.com/pivotal-cf/brokerapi#ServiceBroker) interface. Pass an implementation of this to [`brokerapi.New`](https://godoc.org/github.com/pivotal-cf/brokerapi#New), which returns an `http.Handler` that you can use to serve handle HTTP requests.

Alternatively, if you already have a `*mux.Router` that you want to attach service broker routes to, you can use [`brokerapi.AttachRoutes`](https://godoc.org/github.com/pivotal-cf/brokerapi#AttachRoutes).

## Error types

`brokerapi` defines a handful of error types in `service_broker.go` for some common error cases that your service broker may encounter. Return these from your `ServiceBroker` methods where appropriate, and `brokerapi` will do the "right thing" (™), and give Cloud Foundry an appropriate status code, as per the [Service Broker API specification](https://docs.cloudfoundry.org/services/api.html).

### Custom Errors

`NewFailureResponse()` allows you to return a custom error from any of the `ServiceBroker` interface methods which return an error. Within this you must define an error, a HTTP response status code and a logging key. You can also use the `NewFailureResponseBuilder()` to add a custom `Error:` value in the response, or indicate that the broker should return an empty response rather than the error message.

## Example Service Broker

You can see the [cf-redis](https://github.com/pivotal-cf/cf-redis-broker/blob/2f0e9a8ebb1012a9be74bbef2d411b0b3b60352f/broker/broker.go) service broker uses the BrokerAPI package to create a service broker for Redis.
