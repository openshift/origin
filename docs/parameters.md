# Passing parameters to brokers

Table of Contents
- [Overview](#overview)
- [Design](#design)
  - [Basic example](#basic-example)
  - [Passing parameters as an inline JSON](#passing-parameters-as-an-inline-json)
  - [Referencing sensitive data stored in secrets](#referencing-sensitive-data-stored-in-secret)

## Overview
`parameters` and `parametersFrom` properties of `ServiceInstance` and `ServiceBinding` resources 
provide support for passing parameters to the broker relevant to the corresponding
[provisioning](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md#provisioning) or
[binding](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md#binding) request. 
The resulting structure represents an arbitrary JSON object, which is assumed to 
be valid for a particular broker. 
The Service Catalog does not enforce any extra limitations on the format and content 
of this structure.

## Design

To set input parameters, you may use the `parameters` and `parametersFrom` 
fields in the `spec` field of the `ServiceInstance` or `ServiceBinding` resource:
- `parameters` : can be used to specify a set of properties to be sent to the 
broker. The data specified will be passed "as-is" to the broker without any 
modifications - aside from converting it to JSON for transmission to the broker 
in the case of the `spec` field being specified as `YAML`. Any valid `YAML` or 
`JSON` constructs are supported. One only parameters field may be specified per
`spec`.
- `parametersFrom` : can be used to specify which secret, and key in that secret, 
which contains a `string` that represents the json to include in the set of 
parameters to be sent to the broker. The `parametersFrom` field is a list which 
supports multiple sources referenced per `spec`.

You may use either, or both, of these fields as needed.

If multiple sources in `parameters` and `parametersFrom` blocks are specified,
the final payload is a result of merging all of them at the top level.
If there are any duplicate properties defined at the top level, the specification
is considered to be invalid, the further processing of the `ServiceInstance`/`ServiceBinding`
resource stops and its `status` is marked with error condition.

The format of the `spec` will be (in YAML format):
```yaml
spec:
  ...
  parameters:
    name: value
  parametersFrom:
    - secretKeyRef:
        name: secretName
        key: myKey
```
or, in JSON format
```json
"spec": {
  "parameters": {
    "name": "value"
  },
  "parametersFrom": {
    "secretKeyRef": {
      "name": "secretName",
      "key": "myKey"
    }
  }
}
```
and the secret would need to have a key named myKey:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: secretName
type: Opaque
stringData:
  myKey: >
    {
      "password": "letmein"
    }
```
The final JSON payload to be sent to the broker would then look like:
```json
{
  "name": "value",
  "password": "letmein"
}
```

### Basic example

Let's say we want to create a `ServiceInstance` of EC2 running on AWS using a
[corresponding broker](https://github.com/cloudfoundry-samples/go_service_broker) 
which implements the Open Service Broker API.

A typical provisioning request for this broker looks [like this](https://github.com/cloudfoundry-samples/go_service_broker/blob/master/bin/curl_broker.sh):
```bash
curl -X PUT http://username:password@localhost:8001/v2/service_instances/instance_guid-111 -d '{
  "service_id":"service-guid-111",
  "plan_id":"plan-guid",
  "organization_guid": "org-guid",
  "space_guid":"space-guid",
  "parameters": {"ami_id":"ami-ecb68a84"}
}' -H "Content-Type: application/json"
```

Note that the broker accepts an `ami_id` parameter ([AMI](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/AMIs.html) 
identifier).
To configure a provisioning request in Service Catalog, we need to declare a `ServiceInstance` 
resource with an AMI identifier declared in the `parameters` field of its spec:
```yaml
apiVersion: servicecatalog.k8s.io/v1beta1
kind: ServiceInstance
metadata:
  name: ami-instance
  namespace: test-ns
spec:
  serviceClassName: aws-ami
  planName: default
  parameters:
    ami_id: ami-ecb68a84
```

### Passing parameters as an inline JSON

As shown in the example above, parameters can be specified directly in the
`ServiceInstance`/`ServiceBinding` resource specification in the `parameters` field.
The structure of parameters is not limited to just key-value pairs, arbitrary 
YAML/JSON structure supported as an input (YAML format gets translated into 
equivalent JSON structure to be passed to the broker).

Let's have a look at the broker for 
[Spring Cloud Config Server](https://docs.pivotal.io/spring-cloud-services/1-4/common/config-server/configuring-backends.html#vault),
as an example.

It requires JSON configuration like this:
```json
{
  "vault": {
    "host": "127.0.0.1",
    "port": "8200",
    "proxy": {
      "http": {
        "host": "proxy.wise.com",
        "port": "80"
      }
    }
  }
}
```
The corresponding `ServiceInstance` resource with such configuration can be defined as 
follows:
```yaml
apiVersion: servicecatalog.k8s.io/v1beta1
kind: ServiceInstance
metadata:
  name: spring-cloud-instance
  namespace: test-ns
spec:
  serviceClassName: cloud-config
  planName: default
  parameters:
    vault:
      host: 127.0.0.1
      port: "8200"
      proxy:
        http:
          host: proxy.wise.com
          port: "80"
```

### Referencing sensitive data stored in secret

`Secret` resources can be used to store sensitive data. The `parametersFrom`
field allows the user to reference the external parameters source.

If the user has sensitive data in their parameters, the entire JSON payload can 
be stored in a single `Secret` key, and passed using a `secretKeyRef` field:

```yaml
  ...
  parametersFrom:
    - secretKeyRef:
        name: mysecret
        key: mykey
```

The value stored in a secret key must be a valid JSON.
