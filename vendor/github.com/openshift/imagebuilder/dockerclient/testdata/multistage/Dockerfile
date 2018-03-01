FROM golang:1.9 as builder
WORKDIR /tmp
COPY . .
RUN echo foo > /tmp/bar

FROM busybox:latest
WORKDIR /
COPY --from=builder /tmp/bar /bin/baz
COPY dir /var/dir

RUN echo /bin/baz

