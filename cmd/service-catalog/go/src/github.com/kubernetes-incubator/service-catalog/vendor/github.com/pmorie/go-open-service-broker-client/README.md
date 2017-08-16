# `go-open-service-broker-client`

[![Build Status](https://travis-ci.org/pmorie/go-open-service-broker-client.svg?branch=master)](https://travis-ci.org/pmorie/go-open-service-broker-client)
[![Coverage Status](https://coveralls.io/repos/github/pmorie/go-open-service-broker-client/badge.svg)](https://coveralls.io/github/pmorie/go-open-service-broker-client)

A golang client for communicating with service brokers implementing the
[Open Service Broker API](https://github.com/openservicebrokerapi/servicebroker).

## Who should use this library?

This library is most interesting if you are implementing an integration
between an application platform and the Open Service Broker API.

## Example

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

## Documentation

This client library supports the following versions of the
[Open Service Broker API](https://github.com/openservicebrokerapi/servicebroker):

- [v2.12](https://github.com/openservicebrokerapi/servicebroker/tree/v2.12)
- [v2.11](https://github.com/openservicebrokerapi/servicebroker/tree/v2.11)

Only fields supported by the version configured for a client are
sent/returned.

Check out the
[API specification](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md).

Check out the detailed docs for the [v2 client here](docs/).

## Goals

Overall, to make an excellent golang client for the Open Service Broker API.
Specifically:

- Provide useful insights to newcomers to the API
- Support moving between major and minor versions of the OSB API easily
- Support new auth modes in a backward-compatible manner
- Support alpha features in the Open Service Broker API in a clear manner
- Allow advanced configuration of TLS configuration to a broker
- Provide a fake client suitable for unit-type testing

For the content of the project, goals are:

- Provide high-quality godoc comments
- High degree of unit test coverage
- Code should pass vet and lint checks

## Non-goals

This project does not aim to provide:

- A v1 client
- A fake _service broker_ (I definitely want this, but I do not think it should live in this repo)
- A conformance suite for service brokers (I definitely want this, but I do not think it should live in this repo)
- Any 'custom' API features with are not either in a released version of the
  Open Service Broker API spec or accepted into the spec but not yet released

## Current status

This repository is used in the 
[Kubernetes `service-catalog`](https://github.com/kubernetes-incubator/service-catalog)
incubator repo.

## Why?

This repository is a pet project of mine.  During the day, I work on the
[Kubernetes](https://github.com/kubernetes/kubernetes)
[Service Catalog](https://github.com/kubernetes-incubator/service-catalog) as
well as the Open Service Broker API specification.  I have never written a
client library for a REST API before, and this one was relevant, so here we
are.

