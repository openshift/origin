# Simple authenticated registry image

This directory will build a Docker registry image that is configured for
BASIC authentication with user `user` and password `password` on port
5000. Intended for testing authenticated registry support.

Build with

    docker build .
