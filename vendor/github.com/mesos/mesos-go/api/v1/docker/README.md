## How To
```sh
$ docker run -ti --rm --net=host jdef/example-scheduler-httpv1 -server.address=10.2.0.5 \
    -url=http://10.2.0.7:5050/api/v1/scheduler -tasks=10 -verbose
```
