# Prometheus Metrics

Service Catalog maintains metrics through the Prometheus client library and
exposes them through the Prometheus http adapter at the Controller's /metrics
endpoint. These metrics can be accessed directly via HTTP GET or more commonly
scraped by the Prometheus monitoring application which persists metrics and
facilitates analysis through a Web UI and powerful query language.

Many metrics are not created and exposed until Service Catalog performs
operations which would impact the metrics.  If you have no Service Brokers
defined, there will be no metrics for class or plan count and likely no OSB
Client operation metrics.  This just means the metric names will not show up, it
may look like Prometheus isn't collecting metrics from Service Catalog.  So
before proceeding, it's recommended you have a Broker defined and created a
Service Instance.

To view the raw metrics:

```
# setup a port forward so we can curl against Controller Manager

$ kubectl get pods -l app=catalog-catalog-controller-manager -n catalog -o name | \
    sed 's/^.*\///' | xargs -I{} kubectl port-forward {} -n catalog 8089:8080 &

$ curl -s http://localhost:8089/metrics  | grep servicecatalog
Handling connection for 8089
# HELP servicecatalog_broker_service_class_count Number of services classes by Broker.
# TYPE servicecatalog_broker_service_class_count gauge
servicecatalog_broker_service_class_count{broker="ups-broker"} 1
# HELP servicecatalog_broker_service_plan_count Number of services classes by Broker.
# TYPE servicecatalog_broker_service_plan_count gauge
servicecatalog_broker_service_plan_count{broker="ups-broker"} 2
# HELP servicecatalog_osb_request_count Cumulative number of HTTP requests from the OSB Client to the specified Service Broker grouped by broker name, broker method, and response status.
# TYPE servicecatalog_osb_request_count counter
servicecatalog_osb_request_count{broker="ups-broker",method="Bind",status="2xx"} 41
servicecatalog_osb_request_count{broker="ups-broker",method="GetCatalog",status="2xx"} 1
servicecatalog_osb_request_count{broker="ups-broker",method="ProvisionInstance",status="2xx"} 2
```

Alternatively, and the more common approach to utlizing metrics, deploy
Prometheus.  [This YAML](prometheus.yml) creates a Prometheus instance
preconfigured to gather Kubernetes platform and node metrics.  If you deploy the
Service Catalog Controller Manager via Helm with the optional
`enablePrometheusScrape` parameter set to true (either edit the parameter in
[charts/catalog/values.yaml](../../../charts/catalog/values.yaml) or specify
"--set enablePrometheusScrape=true" when installing Catalog with helm), this configuration will direct Prometheus
to automatically scrape custom metrics exposed from Service Catalog as well.
Most any Prometheus configuration for Kubernetes (ie [Prometheus
Operator](https://github.com/coreos/prometheus-operator)) will pick up the
Service Catalog metrics as long as it's looking for pods with the
`prometheus.io/scrape` annotation.

To deploy Prometheus, run:

```
$ kubectl create -f contrib/examples/prometheus/prometheus.yml
```

To access the Promentheus application, you must either expose it as a service or
provide port forwarding to the Prometheus app:

```
$ kubectl get pods -l app=prometheus -o name | \
	sed 's/^.*\///' | \
	xargs -I{} kubectl port-forward {} 9090:9090
```

Now you can view Prometheus at http://localhost:9090.  If you navigate to
"Status" -> "Targets" you will see the endpoints that Prometheus is scraping.
It should include the "catalog-controller-manager" pod if you deployed Catalog
with enablePrometheusScrape.  If you navigate back to "Graph" and type "catalog"
into the expression filter you should see metrics from Service Catlog.

**The present set of Catalog metics needs to be greatly expanded upon** -- it's
really simple to add additional metrics, or drop me (jboyd01) a note if you have
ideas but not the time to implement.  If you want to add metrics, briefly review
[pkg/metrics/metrics.go](../../../pkg/metrics/metrics.go) and
[pkg/controller/controller_broker.go](../../../pkg/controller/controller_broker.go)
for reference.

## Useful metrics queries

tbd

## Helpful Prometheus Links

Getting started with Prometheus: https://prometheus.io/docs/prometheus/latest/getting_started/  
Basics for Querying Prometheus: https://prometheus.io/docs/prometheus/latest/querying/basics/  
Instrumenting your App: https://godoc.org/github.com/prometheus/client_golang/prometheus   