# Contents
* [Overview](#overview)
* [Development](#development)
* [Authentication Model](#authentication-model)
* [Asynchronous Operations](#asynchronous-operations)
* [API](#api)
    * [Clusters](#clusters)
        * [Create Cluster](#create-cluster)
        * [Set Cluster Flags](#set-cluster-flags)
        * [Cluster Information](#cluster-information)
        * [List Clusters](#list-clusters)
        * [Delete Cluster](#delete-cluster)
    * [Nodes](#nodes)
        * [Add node](#add-node)
        * [Node Information](#node-information)
        * [Set Node Tags](#set-node-tags)
        * [Delete node](#delete-node)
    * [Devices](#devices)
        * [Add device](#add-device)
        * [Device Information](#device-information)
        * [Set Device Tags](#set-device-tags)
        * [Delete device](#delete-device)
    * [Volumes](#volumes)
        * [Create a Volume](#create-a-volume)
        * [Volume Information](#volume-information)
        * [Expand a Volume](#expand-a-volume)
        * [Delete Volume](#delete-volume)
        * [List Volumes](#list-volumes)
    * [Metrics](#metrics)
        * [Get Metrics](#get-metrics)

# Overview
Heketi provides a RESTful management interface which can be used to manage the life cycle of GlusterFS volumes.  The goal of Heketi is to provide a simple way to create, list, and delete GlusterFS volumes in multiple storage clusters.  Heketi intelligently will manage the allocation, creation, and deletion of bricks throughout the disks in the cluster.  Heketi first needs to learn about the topologies of the clusters before satisfying any requests.  It organizes data resources into the following: Clusters, contain Nodes, which contain Devices, which will contain Bricks.

# Development
To communicate with the Heketi service, you will need to either use a client library or directly communicate with the REST endpoints.  The following client libraries are supported: Go, Python

## Go Client Library
Here is a small example of how to use the Go client library:

```go

import (
	"fmt"
	"github.com/heketi/heketi/client/api/go-client"
)

func main() {
	// Create a client
	heketi := client.NewClient(options.Url, options.User, options.Key)

	// List clusters
	list, err := heketi.ClusterList()
	if err != nil {
		return err
	}

	output := strings.Join(list.Clusters, "\n")
	fmt.Fprintf(stdout, "Clusters:\n%v\n", output)
	return nil
}
```

For more examples see the [Heketi cli client](https://github.com/heketi/heketi/tree/master/client/cli/go/cmds).

* Source: https://github.com/heketi/heketi/tree/master/client/api/go-client

## Python Client Library
The python client library can be installed either from the source or by installing the `python-heketi` package in Fedora/RHEL.  The source is available https://github.com/heketi/heketi/tree/master/client/api/python , and for examples, please check out the [unit tests](https://github.com/heketi/heketi/blob/master/client/api/python/test/unit/test_client.py)

## Running the development server
The simplest way to development a client for Heketi is to run the Heketi service in `mock` mode.  In this mode, Heketi will not need to communicate with any storage nodes, instead it mocks the communication, while still supporting all REST calls and maintaining state.  The simplest way to run the Heketi server is to run it from a container as follows:

```
# docker run -d -p 8080:8080 heketi/heketi
# curl http://localhost:8080/hello
Hello from Heketi
```

# Authentication Model
Heketi uses a stateless authentication model based on the JSON Web Token (JWT) standard as proposed to the [IETF](https://tools.ietf.org/html/draft-ietf-oauth-json-web-token-25).  As specified by the specification, a JWT token has a set of _claims_ which can be added to a token to determine its correctness.  Heketi requires the use of the following standard claims:

* [_iss_](http://self-issued.info/docs/draft-ietf-oauth-json-web-token.html#rfc.section.4.1.1): Issuer.  Heketi supports two types of issuers:
    * _admin_: Has access to all APIs
    * _user_: Has access to only _Volume_ APIs     
* [_iat_](http://self-issued.info/docs/draft-ietf-oauth-json-web-token.html#rfc.section.4.1.6): Issued-at-time
* [_exp_](http://self-issued.info/docs/draft-ietf-oauth-json-web-token.html#rfc.section.4.1.4): Time when the token should expire

And a custom one following the model as described on [Atlassian](https://developer.atlassian.com/static/connect/docs/latest/concepts/understanding-jwt.html): 

* _qsh_.  URL Tampering prevention.

Heketi supports token signatures encrypted using the HMAC SHA-256 algorithm which is specified by the specification as `HS256`.

## Clients
There are JWT libraries available for most languages as highlighted on [jwt.io](http://jwt.io).  The client libraries allow you to easily create a JWT token which must be stored in the `Authorization: Bearer {token}` header.  A new token will need to be created for each REST call.  Here is an example of the header:

`Authorization: Bearer eyJhb[...omitted for brevity...]HgQ`

### Python Example
Here is an example of how to create a token as Python client:

```python
import jwt
import datetime
import hashlib

method = 'GET'
uri = '/volumes'
secret = 'My secret'

claims = {}

# Issuer
claims['iss'] = 'admin'

# Issued at time
claims['iat'] = datetime.datetime.utcnow()

# Expiration time
claims['exp'] = datetime.datetime.utcnow() \
	+ datetime.timedelta(minutes=10)

# URI tampering protection
claims['qsh'] = hashlib.sha256(method + '&' + uri).hexdigest()

print jwt.encode(claims, secret, algorithm='HS256')
```

Example output:

```
eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJhZG1pbiIsImlhdCI6MTQzNTY4MTY4OSwicXNoIjoiYzE2MmFjYzkwMjQyNzIxMjBiYWNmZmY3NzA5YzkzMmNjMjUyMzM3ZDBhMzBmYTE1YjAyNTAxMDA2NjY2MmJlYSIsImV4cCI6MTQzNTY4MjI4OX0.ZBd_NgzEoGckcnyY4_ypgJsN6Oi7x0KxX2w8AXVyiS8
```

### Ruby Example
Run this as: `./heketi-api.rb volumes`

```ruby
#!/usr/bin/env ruby

require 'jwt'
require 'digest'

user = "admin"
pass = "password"
server = "http://heketi.example.com:8443"

uri = "/#{ARGV[0]}"

payload = {}

headers = {
  iss: 'admin',
  iat: Time.now.to_i,
  exp: Time.now.to_i + 600,
  qsh: Digest::SHA256.hexdigest("GET&#{uri}")
}

token = JWT.encode headers, pass, 'HS256'

exec("curl -H \"Authorization: Bearer #{token}\" #{server}#{uri}")
```

Copy this example token and decode it in [jwt.io](http://jwt.io) by pasting it in the token area and changing the secret to `My secret`.

## More Information
* [JWT Specification](http://self-issued.info/docs/draft-ietf-oauth-json-web-token.html)
* [Debugger and clients at jwt.io](http://jwt.io)
* [Stateless tokens with JWT](http://jonatan.nilsson.is/stateless-tokens-with-jwt/)
* Clients
    * [Go JWT client](https://github.com/auth0/go-jwt-middleware)
    * [Python JWT client](https://github.com/jpadilla/pyjwt)
    * [Java JWT client](https://bitbucket.org/b_c/jose4j/wiki/Home)
    * [Ruby JWT client](https://github.com/jwt/ruby-jwt)


# Asynchronous Operations
Some operations may take a long time to process.  For these operations, Heketi will return [202 Accepted](http://httpstatus.es/202) with a temporary resource set inside the `Location` header.  A client can then issue a _GET_ on this temporary resource and receive the following:

* **HTTP Status 200**: Request is still in progress. _We may decide to add some JSON ETA data here in future releases_.
    * **Header** _X-Pending_ will be set to the value of _true_
* **HTTP Status 404**: Temporary resource requested is not found.
* **HTTP Status [500](http://httpstatus.es/500)**: Request completed and has failed.  Body will be filled in with error information.
* **HTTP Status [303 See Other](http://httpstatus.es/303)**: Request has been completed successfully. The information requested can be retrieved by issuing a _GET_ on the resource set inside the `Location` header.
* **HTTP Status [204 Done](http://httpstatus.es/204)**: Request has been completed successfully. There is no data to return.


# API
Heketi uses JSON as its data serialization format. XML is not supported.

Most APIs use use the following methods on the URIs:

* URIs in the form of `/<REST endpoint>/{id}`
* Responses and requests are in JSON format
* **POST**: Send data to Heketi where the _body_ has data described in JSON format.
* **GET**: Retrieve data from Heketi where the _body_ has data described in JSON format.
* **DELETE**: Deletes the specified object from Heketi.

Before servicing any requests, Heketi must first learn the topology of the clusters.  Once it knows which nodes and disks to use, it can then service requests.

## Clusters
Heketi is able to manage multiple GlusterFS clusters, each composed of a set of storage nodes.  Once a cluster has been created, nodes can then be added to it for Heketi to manage.  A GlusterFS cluster is a set of nodes participating as a trusted storage pool.  Volumes do not cross cluster boundaries.

### Create Cluster
* **Method:** _POST_
* **Endpoint**:`/clusters`
* **Content-Type**: `application/json`
* **Response HTTP Status Code**: 201
* **JSON Request**: Empty body, or a JSON request with optional attributes:
    * file: _bool_, _optional_, whether this cluster should allow creation of file volumes (default: true)
    * block: _bool_, _optional_, whether this cluster should allow creation of block volumes (default: true)
    * Example:

```json
{
"block" : false
}
```

* **JSON Response**: See [Cluster Information](#cluster_info)
    * Example:

```json
{
    "id": "67e267ea403dfcdf80731165b300d1ca",
    "nodes": [],
    "volumes": [],
}
```

### Set Cluster Flags
* **Method:** _POST_
* **Endpoint**:`/clusters/{id}/flags`
* **Content-Type**: `application/json`
* **Response HTTP Status Code**: 200
* **JSON Request**:
    * file: _bool_, whether this cluster should allow creation of file volumes
    * block: _bool_, whether this cluster should allow creation of block volumes
    * Example:

```json
{
    "file": true,
    "block": false,
}
```

* **JSON Response**: None


### Cluster Information
* **Method:** _GET_  
* **Endpoint**:`/clusters/{id}`
* **Response HTTP Status Code**: 200
* **JSON Request**: None
* **JSON Response**:
    * id: _string_, UUID for node
    * nodes: _array of strings_, UUIDs of each node in the cluster
    * volumes: _array of strings_, UUIDs of each volume in the cluster
    * Example:

```json
{
    "id": "67e267ea403dfcdf80731165b300d1ca",
    "nodes": [
        "78696abbba372659effa",
        "799029acaa867a66934"
    ],
    "volumes": [
        "aa927734601288237463aa",
        "70927734601288237463aa"
    ],
}
```

### List Clusters
* **Method:** _GET_  
* **Endpoint**:`/clusters`
* **Response HTTP Status Code**: 200
* **JSON Request**: None
* **JSON Response**:
    * clusters: _array of strings_, UUIDs of clusters
    * Example:

```json
{
    "clusters": [
        "67e267ea403dfcdf80731165b300d1ca",
        "ff6667ea403dfcdf80731165b300d1ca"
    ]
}
```

### Delete Cluster
* **Method:** _DELETE_  
* **Endpoint**:`/clusters/{id}`
* **Response HTTP Status Code**: 200
* **Response HTTP Status Code**: 409, Returned if it contains nodes
* **JSON Request**: None
* **JSON Response**: None

## Nodes
The _node_ RESTful endpoint is used to register a storage system for Heketi to manage.  Devices in this node can then be registered.

### Add Node
* **Method:** _POST_  
* **Endpoint**:`/nodes`
* **Content-Type**: `application/json`
* **Response HTTP Status Code**: 202, See [Asynchronous Operations](#async)
* **Temporary Resource Response HTTP Status Code**: 303, `Location` header will contain `/nodes/{id}`. See [Node Info](#node_info) for JSON response.
* **JSON Request**:
    * zone: _int_, failure domain.  Value of `0` is not allowed.
    * hostnames: _map of strings_
        * manage: _array of strings_, List of node management hostnames.  
            * In SSH configurations, Heketi needs to be able to SSH to the host on any of the supplied management hostnames.  It is *highly* recommended to use hostnames instead of IP addresses.
            * For Kubernetes and OpenShift, this must be the name of the node as shown in `kubectl get nodes` which is running the Gluster POD
            * _NOTE:_  Even though it takes a list of hostnames, only one is supported at the moment.  The plan is to support multiple hostnames when glusterd-2 is used.  For Kubernetes and OpenShift, this must be the name of the Pod file, not the name of the node.
        * storage: _array of strings_, List of node storage network hostnames.  These storage network addresses will be used to create and access the volume.  It is *highly* recommended to use hostnames instead of IP addresses. _NOTE:_  Even though it takes a list of hostnames, only one is supported at the moment.  The plan is to support multiple ip address when glusterd-2 is used.
    * cluster: _string_, UUID of cluster to whom this node should be part of.
    * tags: _map of strings_, (optional) a mapping of tag-names to tag-values
    * Example:

```json
{
    "zone": 1,
    "hostnames": {
        "manage": [
            "node1-manage.gluster.lab.com"
        ],
        "storage": [
            "node1-storage.gluster.lab.com"
        ]
    },
    "tags": {
        "incantation": "abracadabra"
    },
    "cluster": "67e267ea403dfcdf80731165b300d1ca"
}
```

### Node Information
* **Method:** _GET_
* **Endpoint**:`/nodes/{id}`
* **Response HTTP Status Code**: 200
* **JSON Request**: None
* **JSON Response**:
    * zone: _int_, Failure Domain
    * id: _string_, UUID for node
    * cluster: _string_, UUID of cluster
    * hostnames: _map of strings_
        * manage: _array of strings_, List of node management hostnames.  Heketi needs to be able to SSH to the host on any of the supplied management hostnames.
        * storage: _array of strings_, List of node storage network hostnames.  These storage network addresses will be used to create and access the volume.
    * devices: _array maps_, See [Device Information](#device_info)
    * tags: _map_, (omitted if empty) a mapping of tag-names to tag-values
    * Example:

```json
{
    "zone": 1,
    "id": "88ddb76ad403dfcdf80731165b300d1ca",
    "cluster": "67e267ea403dfcdf80731165b300d1ca",
    "hostnames": {
        "manage": [
            "node1-manage.gluster.lab.com"
        ],
        "storage": [
            "node1-storage.gluster.lab.com"
        ]
    },
    "tags": {
        "arbiter": "supported",
        "rack": "7,4"
    },
    "devices": [
        {
            "name": "/dev/sdh",
            "storage": {
                "total": 2000000,
                "free": 2000000,
                "used": 0
            },
            "id": "49a9bd2e40df882180479024ac4c24c8",
            "bricks": [
                {
                    "id": "aaaaaad2e40df882180479024ac4c24c8",
                    "path": "/gluster/brick_aaaaaad2e40df882180479024ac4c24c8/brick",
                    "size": 0
                },
                {
                    "id": "bbbbbbd2e40df882180479024ac4c24c8",
                    "path": "/gluster/brick_bbbbbbd2e40df882180479024ac4c24c8/brick",
                    "size": 0
                }
            ]
        }
    ]
}
```

### Set Node Tags

Allows setting, updating, and deleting user specified metadata tags
on a node. Tags are key-value pairs that are stored on the server.
Certain well-known tags may be used by the server for configuration.

Specifying a `change_type` of _set_ overwrites the tags on the object
with exactly the set of tags in this request. A `change_type` of
_update_ adds new tags and updates existing tags without changing
tags not specified in the request. A `change_type` of _delete_ will
remove tags names in the request (tag values in the request will
be ignored).

* **Method**: POST
* **Endpoint**: `/nodes/{id}/tags`
* **Response HTTP Status Code**: 200
* **JSON Request**: None
    * `change_type`: _string_, one of "set", "update", "delete"
    * `tags`: _map of strings_, a mapping of tag-names to tag-values
    * Example:

```json
{
    "change_type": "set",
    "tags": {
        "arbiter": "supported",
        "rack": "7,4",
        "os_version": "linux 4.15.8"
    }
}
```
* **JSON Response**: Ignored

### Delete Node
* **Method:** _DELETE_  
* **Endpoint**:`/nodes/{id}`
* **Response HTTP Status Code**: 202, See [Asynchronous Operations](#async)
* **Response HTTP Status Code**: 409, Node contains devices
* **Temporary Resource Response HTTP Status Code**: 204

## Devices
The `devices` endpoint allows management of raw devices in the cluster.

### Add Device
* **Method:** _POST_  
* **Endpoint**:`/devices`
* **Content-Type**: `application/json`
* **Response HTTP Status Code**: 202, See [Asynchronous Operations](#async)
* **Temporary Resource Response HTTP Status Code**: 204
* **JSON Request**:
    * node: _string_, UUID of node which the devices belong to.
    * name: _string_, Device name
    * destroydata: _bool_, (optional) destroy any data on the device
    * tags: _map of strings_, (optional) a mapping of tag-names to tag-values
    * Example:

```json
{
    "node": "714c510140c20e808002f2b074bc0c50",
    "name": "/dev/sdb",
    "tags": {
        "serial_number": "a109-84338af8e43-48dd9d43-919231"
    }
}
```

### Device Information
* **Method:** _GET_
* **Endpoint**:`/devices/{id}`
* **Response HTTP Status Code**: 200
* **JSON Request**: None
* **JSON Response**:
    * name: _string_, Name of device
    * id: _string_, UUID of device
    * total: _uint64_, Total storage in KB
    * free: _uint64_, Available storage in KB
    * used: _uint64_, Allocated storage in KB
    * bricks: _array of maps_, Bricks allocated on this device
        * id: _string_, UUID of brick
        * path: _string_, Path of brick on the node
        * size: _uint64_, Size of brick in KB
    * tags: _map_, (omitted if empty) a mapping of tag-names to tag-values
    * Example:

```json
{
    "name": "/dev/sdh",
    "storage": {
        "total": 1000000,
        "free": 1000000,
        "used": 0
    },
    "id": "49a9bd2e40df882180479024ac4c24c8",
    "tags": {
        "arbiter": "required",
        "drivebay": "3"
    },
    "bricks": [
        {
            "id": "aaaaaad2e40df882180479024ac4c24c8",
            "path": "/gluster/brick_aaaaaad2e40df882180479024ac4c24c8/brick",
            "size": 0,
            "node": "714c510140c20e808002f2b074bc0c50",
            "device": "49a9bd2e40df882180479024ac4c24c8"
        },
        {
            "id": "bbbbbbd2e40df882180479024ac4c24c8",
            "path": "/gluster/brick_bbbbbbd2e40df882180479024ac4c24c8/brick",
            "size": 0,
            "node": "714c510140c20e808002f2b074bc0c50",
            "device": "49a9bd2e40df882180479024ac4c24c8"
        }
    ]
}
```

### Set Device Tags

Allows setting, updating, and deleting user specified metadata tags
on a device. Tags are key-value pairs that are stored on the server.
Certain well-known tags may be used by the server for configuration.

Specifying a `change_type` of _set_ overwrites the tags on the object
with exactly the set of tags in this request. A `change_type` of
_update_ adds new tags and updates existing tags without changing
tags not specified in the request. A `change_type` of _delete_ will
remove tags names in the request (tag values in the request will
be ignored).

* **Method**: POST
* **Endpoint**: `/devices/{id}/tags`
* **Response HTTP Status Code**: 200
* **JSON Request**: None
    * `change_type`: _string_, one of "set", "update", "delete"
    * `tags`: _map of strings_, a mapping of tag-names to tag-values
    * Example:

```json
{
    "change_type": "set",
    "tags": {
        "arbiter": "required",
        "drivebay": "3",
        "serial_number": "a109-84338af8e43-48dd9d43-919231"
    }
}
```
* **JSON Response**: Ignored

### Delete Device
* **Method:** _DELETE_  
* **Endpoint**:`/devices/{id}`
* **Response HTTP Status Code**: 202, See [Asynchronous Operations](#asynchronous-operations)
* **Response HTTP Status Code**: 409, Device contains bricks
* **Temporary Resource Response HTTP Status Code**: 204

## Volumes
These APIs inform Heketi to create a network file system of a certain size available to be used by clients.


### Create a Volume
* **Method:** _POST_  
* **Endpoint**:`/volumes`
* **Content-Type**: `application/json`
* **Response HTTP Status Code**: 202, See [Asynchronous Operations](#asynchronous-operations)
* **Temporary Resource Response HTTP Status Code**: 303, `Location` header will contain `/volumes/{id}`. See [Volume Info](#volume_info) for JSON response.
* **JSON Request**:
    * size: _int_, Size of volume requested in GiB
    * name: _string_, _optional_, Name of volume.  If not provided, the name of the volume will be `vol_{id}`, for example `vol_728faa5522838746abce2980`
    * durability: _map_, _optional_, Durability Settings
        * type: _string_, optional, Durability type.  Choices are **none** (Distributed Only), **replicate** (Distributed-Replicated), **disperse** (Distributed-Disperse).  If omitted, durability type will default to **none**.
        * replicate: _map_, _optional_, Replica settings, only used if `type` is set to *replicate*.
            * replica: _int_, _optional_, Number of replica per brick. If omitted, it will default to `2`.
        * disperse: _map_, _optional_, Erasure Code settings, only used if `type` is set to *disperse*.
            * data: _int_, _optional_ Number of dispersed data volumes. If omitted, it will default to `8`.
            * redundancy: _int_, _optional_, Level of redundancy. If omitted, it will default to `2`.
    * snapshot: _map_ 
        * enable: _bool_, _optional_, Snapshot support requested for this volume.  If omitted, it will default to `false`.
        * factor: _float32_, _optional_, Snapshot reserved space factor.  When creating a volume with snapshot enabled, the size of the brick will be set to _factor * brickSize_, where brickSize is automatically determined to satisfy the volume size request.  If omitted, it will default to _1.5_.
            * Requirement: Value must be greater than one.
    * clusters: _array of string_, _optional_, UUIDs of clusters where the volume should be created.  If omitted, each cluster will be checked until one is found that can satisfy the request.
    * Example:

```json
{
    "size": 10000000,
    "snapshot": {
        "enable": true,
        "factor": 1.2
    },
    "durability": {
        "type": "replicate",
        "replicate": {
            "replica": 3
        }
    },
    "clusters": [
        "2f84c71240f43e16808bc64b05ad0d06",
        "5a2c52d04075373e80dbfa1e291ba0de"
    ]
}
```
Note:
The volume size created depends upon the underlying brick size.
For example, for a 2 way/3 way replica volume, the minimum volume size is 1GiB as the
underlying minimum brick size is constrained to 1GiB.

So, it is not possible create a volume of size less than 1GiB.


### Volume Information
* **Method:** _GET_
* **Endpoint**:`/volumes/{id}`
* **Response HTTP Status Code**: 200
* **JSON Request**: None
* **JSON Response**:
    * name: _string_, Name of volume
    * size: _int_, Size of volume in GiB
    * id: _string_, Volume UUID
    * cluster: _string_, UUID of cluster which contains this volume
    * durability: _map_, Durability settings.  See [Volume Create](#volume_create) for more information.
    * snapshot: _map_, If omitted, snapshots are disabled.
        * enable: _bool_, Snapshot support requested for this volume.
        * factor: _float32_, _optional_, Snapshot reserved space factor if enabled
    * replica: _int_, Replica count
    * mounts: _map_, Information used to mount or gain access to the network volume file system
        * glusterfs: _map_, Mount point information for native GlusterFS FUSE mount
            * device: _string_, Mount point used for native GlusterFS FUSE mount
            * options: _map_, Optional mount options to use
                * backup-volfile-servers: _string_, List of backup volfile servers [[1](https://www.mankier.com/8/mount.glusterfs)] [[2](https://access.redhat.com/documentation/en-US/Red_Hat_Storage/2.0/html/Administration_Guide/chap-Administration_Guide-GlusterFS_Client.html#sect-Administration_Guide-GlusterFS_Client-GlusterFS_Client-Mounting_Volumes)] [[3](http://blog.gluster.org/category/mount-glusterfs/)].  It is up to the calling service to determine which of the volfile servers to use in the actual mount command.
    * brick: _array of maps_, Bricks used to create volume. See [Device Information](#device_info) for brick JSON description
    * Example:

```json
{
    "name": "vol_70927734601288237463aa",
    "id": "70927734601288237463aa",
    "cluster": "67e267ea403dfcdf80731165b300d1ca",
    "size": 123456,
    "durability": {
        "type": "replicate",
        "replicate": {
            "replica": 3
        }
    },
    "snapshot": {
        "enable": true,
        "factor": 1.2
    },
    "mount": {
        "glusterfs": {
            "device": "192.168.1.103:vol_70927734601288237463aa",
            "options": {
                "backup-volfile-servers": "192.168.1.103,192.168.101"
            }
        }
    },
    "bricks": [
        {
            "id": "aaaaaad2e40df882180479024ac4c24c8",
            "path": "/gluster/brick_aaaaaad2e40df882180479024ac4c24c8/brick",
            "size": 0,
            "node": "892761012093474071983852",
            "device": "ff2137326add231578ffa7234"
        },
        {
            "id": "bbbbbbd2e40df882180479024ac4c24c8",
            "path": "/gluster/brick_bbbbbbd2e40df882180479024ac4c24c8/brick",
            "size": 0,
            "node": "714c510140c20e808002f2b074bc0c50",
            "device": "49a9bd2e40df882180479024ac4c24c8"
        }
    ]
}
```

### Expand a Volume
New volume size will be reflected in the volume information.
* **Method:** _POST_  
* **Endpoint**:`/volumes/{id}/expand`
* **Content-Type**: `application/json`
* **Response HTTP Status Code**: 202, See [Asynchronous Operations](#async)
* **Temporary Resource Response HTTP Status Code**: 303, `Location` header will contain `/volumes/{id}`. See [Volume Info](#volume_info) for JSON response.
* **JSON Request**:
    * expand_size: _int_, Amount of storage to add to the existing volume in GiB

```json
{ "expand_size" : 1000000 }
```

### Delete Volume
When a volume is deleted, Heketi will first stop, then destroy the volume.  Once destroyed, it will remove the allocated bricks and free the allocated space.
* **Method:** _DELETE_  
* **Endpoint**:`/volumes/{id}`
* **Response HTTP Status Code**: 202, See [Asynchronous Operations](#async)
* **Temporary Resource Response HTTP Status Code**: 204

### List Volumes
* **Method:** _GET_  
* **Endpoint**:`/volumes`
* **Response HTTP Status Code**: 200
* **JSON Response**:
    * volumes: _array strings_, List of volume UUIDs.
    * Example:

```json
{
    "volumes": [
        "aa927734601288237463aa",
        "70927734601288237463aa"
    ]
}
```

### Get Metrics
Get current metrics for the heketi cluster. Metrics are exposed in the prometheus format.
* **Method:** _GET_
* **Endpoint**:`/metrics`
* **Response HTTP Status Code**: 200
* **TEXT Response**:
    * Example:

```
# HELP heketi_cluster_count Number of clusters
# TYPE heketi_cluster_count gauge
heketi_cluster_count 1
# HELP heketi_device_brick_count Number of bricks on device
# TYPE heketi_device_brick_count gauge
heketi_device_brick_count{cluster="c1",device="d1",hostname="n1"} 1
# HELP heketi_device_count Number of devices on host
# TYPE heketi_device_count gauge
heketi_device_count{cluster="c1",hostname="n1"} 1
# HELP heketi_device_free Amount of Free space available on the device
# TYPE heketi_device_free gauge
heketi_device_free{cluster="c1",device="d1",hostname="n1"} 1
# HELP heketi_device_size Total size of the device
# TYPE heketi_device_size gauge
heketi_device_size{cluster="c1",device="d1",hostname="n1"} 2
# HELP heketi_device_used Amount of space used on the device
# TYPE heketi_device_used gauge
heketi_device_used{cluster="c1",device="d1",hostname="n1"} 1
# HELP heketi_nodes_count Number of nodes on cluster
# TYPE heketi_nodes_count gauge
heketi_nodes_count{cluster="c1"} 1
# HELP heketi_up Is heketi running?
# TYPE heketi_up gauge
heketi_up 1
# HELP heketi_volumes_count Number of volumes on cluster
# TYPE heketi_volumes_count gauge
heketi_volumes_count{cluster="c1"} 0
```
