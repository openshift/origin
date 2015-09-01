Mock
----

# Overview

The origin/mock is a docker image that is built during 'make release'.
Its main purpose is to have a flexible component that can safely be
used and asserted in tests and that have an expected behaviour:

- CRUD on key/value,
- name/hostname
- shutdown,
- simple redirect
- chaining call concept 

The server must have MOCK_NAME as env variable that will contains
the name of the server otherwise it raise a panic.

By default the server serves on the port 8080. By setting environment
variable MOCK_PORT to another port the server will use this particular
port.

# Server spawning

Although this server is expected to run as docker container you can
run it this way:

    $ cd test/mock-app
    $ MOCK_NAME=mock1 go run server.go -logtostderr
    Started mock1, serving at 8080

Starting a second server:

    $ MOCK_NAME=mock2 MOCK_PORT=8081 go run server.go -logtostderr
    Started mock2, serving at 8081
    
# API

## /name

Returns the name of the server

    $ curl http://localhost:8080/name
    mock1
    
    $ curl http://localhost:8081/name
    mock2

## /hostname

Returns the hostname of the server

    $ curl http://localhost:8080/hostname
    zerihgerg
    
## /put/{key}/{value}

Puts the value "value" under the key {key}. Always returns ok.

    $ curl http://localhost:8080/put/k1/value1
    ok
    $ curl http://localhost:8080/put/k2/value2
    ok
    $ curl http://localhost:8080/put/k3/value3
    ok

## /contains/{key}

Returns "ok" if the server contains {key}, "ko" otherwise.

    $ curl http://localhost:8080/contains/k1
    ok
     
    $ curl http://localhost:8081/contains/k1
    ko
     
## /get/{key}

Returns the value behind the key {key}, empty string if there is no
value for this key.

    $ curl http://localhost:8080/get/k1
    value1

    $ curl http://localhost:8081/get/k1
    ""           <-- empty string without "

## /keys

Returns all the keys

   $ curl http://localhost:8080/keys
   k1
   k2
   k3

## /delete/{key}

Deletes the entry where key {key} is stored. Returns "ok" if deletion occurs, "ko" if not.

    $  curl http://localhost:8080/delete/k2
    ok
    
    $  curl http://localhost:8080/delete/k2
    ko
    
## /redirect/{scheme}/{host}/{port}/{path}

This redirects the contents of the url {scheme}://{host}:{port}/{path} 

    $ curl http://localhost:8080/redirect/http/localhost/8081/name
    Querying http://localhost:8081/name got response mock2

## /env/{var}

This returns the value of the environment variable {var} on the
server. Returns "" (empty string if {var} does not exist.

    $ curl http://localhost:8080/env/MOCK_NAME
    mock1
    
    $ curl http://localhost:8080/env/SHELL
    /bin/shell
    

## /shutdown

This properly shutdowns the server itself immediatly. (exit code to 0)

## /shutdown/{delay}

This properly shutdowns the server itself in a delay of {delay} seconds. (exit code to 0)

## /fail

This kills the server itself immediatly with exit code to -1

## /fail/{exitCode}

This kills the server itself immediately with exit code to {exitCode}.


## /chainredirect/{scheme}/{chainPath}

Chain Redirect is /redirect on steroids, it allows communication
scheme between several mock servers and returns the concatenation of
all calls. For instance:

- mock1 calls mock2/name,
- mock2 calls mock1/put/k1/newvalue
- mock1 calls mock1/get/k1

the output will be:

    mock2
    ok
    newvalue

### {scheme}

Scheme is the protocol that is use for all communications. For now:
http 

### {chainPath} format explained

    (host "_" port "_" path) ("-" (host "_" port "_" path))+

where 'path' itself can be composed with '_' in between like

    "put_k1_v1"
    
For instance

    localhost_8081_name-localhost_8080_put_k1_v1
    
is a valid chainPath. In command line, this gives:

    $ curl http://localhost:8080/chainredirect/http/localhost_8081_name-localhost_8080_put_k1_v1
    mock2
    ok

curl calls mock1 initiating chainredirect, mock1 calls mock2 name then
mock2 calls mock1 put/k1/v1. This can be as long as GET allows it.

We can shutdown all the servers with this command line

    $ http://localhost:8080/chainredirect/http/localhost_8081_shutdown_2-localhost_8080_shutdown_2
    ok
    ok

