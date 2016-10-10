package aggregated_logging

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"

	routes "github.com/openshift/origin/pkg/route/api"
)

const routeUnaccepted = `
An unaccepted route is most likely due to one of the following reasons:

* No router has been deployed to serve the route.
* Another route with the same host already exists.

If a router has been deployed, look for duplicate matching routes by
running the following:

  oc get --all-namespaces routes --template='{{range .items}}{{if eq .spec.host "%[2]s"}}{{println .metadata.name "in" .metadata.namespace}}{{end}}{{end}}'

`
const routeCertMissingHostName = `
Try updating the route certificate to include its host as either the CommonName (CN) or one of the alternate names.
`

//checkRoutes looks through the logging infra routes to see if they have been accepted, and ...
func checkRoutes(r diagnosticReporter, adapter routesAdapter, project string) {
	r.Debug("AGL0300", "Checking routes...")
	routeList, err := adapter.routes(project, kapi.ListOptions{LabelSelector: loggingSelector.AsSelector()})
	if err != nil {
		r.Error("AGL0305", err, fmt.Sprintf("There was an error retrieving routes in the project '%s' with selector '%s': %s", project, loggingSelector.AsSelector(), err))
		return
	}
	if len(routeList.Items) == 0 {
		r.Error("AGL0310", nil, fmt.Sprintf("There were no routes found to support logging in project '%s'", project))
		return
	}
	for _, route := range routeList.Items {
		if !wasAccepted(r, route) {
			r.Error("AGL0325", nil, fmt.Sprintf("Route '%s' has not been accepted by any routers."+routeUnaccepted, route.ObjectMeta.Name, route.Spec.Host))
		}
		if route.Spec.TLS != nil && len(route.Spec.TLS.Certificate) != 0 && len(route.Spec.TLS.Key) != 0 {
			checkRouteCertificate(r, route)
		} else {
			r.Debug("AGL0331", fmt.Sprintf("Skipping key and certificate checks on route '%s'.  Either of them may be missing.", route.ObjectMeta.Name))
		}
	}
}

func checkRouteCertificate(r diagnosticReporter, route routes.Route) {
	r.Debug("AGL0330", fmt.Sprintf("Checking certificate for route '%s'...", route.ObjectMeta.Name))
	block, _ := pem.Decode([]byte(route.Spec.TLS.Certificate))
	//verify hostname
	if block != nil {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			r.Error("AGL0335", err, fmt.Sprintf("Unable to parse the certificate for route '%s': %s", route.ObjectMeta.Name, err))
			return
		}
		r.Debug("AGL0340", fmt.Sprintf("Cert CommonName: '%s' Cert DNSNames: '%s'", cert.Subject.CommonName, cert.DNSNames))
		if err := cert.VerifyHostname(route.Spec.Host); err != nil {
			r.Error("AGL0345", err, fmt.Sprintf("Route '%[1]s' certficate does not include route host '%[2]s'"+routeCertMissingHostName, route.ObjectMeta.Name, route.Spec.Host))
		}
	} else {
		r.Error("AGL0350", errors.New("Unable to decode the TLS Certificate"), "Unable to decode the TLS Certificate")
	}

	//verify key matches cert
	r.Debug("AGL0355", fmt.Sprintf("Checking certificate matches key for route '%s'", route.ObjectMeta.Name))
	_, err := tls.X509KeyPair([]byte(route.Spec.TLS.Certificate), []byte(route.Spec.TLS.Key))
	if err != nil {
		r.Error("AGL0365", err, fmt.Sprintf("Route '%s' key and certificate do not match: %s.  The router will be unable to pass traffic using this route.", route.ObjectMeta.Name, err))
	}
}

func wasAccepted(r diagnosticReporter, route routes.Route) bool {
	r.Debug("AGL0310", fmt.Sprintf("Checking if route '%s' was accepted...", route.ObjectMeta.Name))
	accepted := 0
	for _, status := range route.Status.Ingress {
		r.Debug("AGL0315", fmt.Sprintf("Status for router: '%s', host: '%s'", status.RouterName, status.Host))
		for _, condition := range status.Conditions {
			r.Debug("AGL0320", fmt.Sprintf("Condition type: '%s' status: '%s'", condition.Type, condition.Status))
			if condition.Type == routes.RouteAdmitted && condition.Status == kapi.ConditionTrue {
				accepted = accepted + 1
			}
		}
	}
	//Add check to compare acceptance to the number of available routers?
	return accepted > 0
}
