# metrics-source
An example python app with simple prometheus metrics. It's intended as a simple application for testing Prometheus scraping inside Openshift clusters.

It will respond with prometheus metrics on all paths over HTTP on port 8080.

## Credits
This is the example from https://github.com/prometheus/client_python, reformatted and ready for instant deployment.

## Usage
To deploy inside an Openshift cluster run
```
oc new-app python:3.6~https://github.com/openshift/origin/ --context-dir=examples/metrics-source
```

