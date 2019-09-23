FROM gliderlabs/alpine:3.3
MAINTAINER James DeFelice <james@mesosphere.com>

cmd []
entrypoint [ "/opt/example-scheduler", "-executor=/opt/example-executor", "-url=http://leader.mesos:5050/api/v1/scheduler" ]

workdir	/opt
add	example-scheduler ./
add	example-executor ./
