FROM golang:1.9

ENV TRILLIAN_LOG_HOST=1.2.3.4 \
    TRILLIAN_LOG_PORT=9999 \
    ETCD_SERVERS=""

ENV HOST=0.0.0.0 \
    HTTP_PORT=6962

ADD . /go/src/github.com/google/certificate-transparency-go
WORKDIR /go/src/github.com/google/certificate-transparency-go

RUN go get -v ./trillian/ctfe/ct_server

ENTRYPOINT /go/bin/ct_server \
  --log_rpc_server="$TRILLIAN_LOG_HOST:$TRILLIAN_LOG_PORT" \
  --log_config="$LOG_CONFIG" \
  --etc_servers="$ETCD_SERVERS" \
  --http_endpoint="$HOST:$HTTP_PORT" \
  --alsologtostderr

EXPOSE $HTTP_PORT

HEALTHCHECK --interval=5m --timeout=3s \
  CMD curl -f http://localhost:$HTTP_PORT/debug/vars || exit 1
