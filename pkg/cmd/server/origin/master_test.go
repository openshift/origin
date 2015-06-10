package origin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emicklei/go-restful"
)

func TestInitializeOpenshiftAPIVersionRouteHandler(t *testing.T) {
	service := new(restful.WebService)
	initAPIVersionRoute(service, "osapi", "v1beta3")

	if len(service.Routes()) != 1 {
		t.Fatalf("Exp. the OSAPI route but found none")
	}
	route := service.Routes()[0]
	if !contains(route.Produces, restful.MIME_JSON) {
		t.Fatalf("Exp. route to produce mimetype json")
	}
	if !contains(route.Consumes, restful.MIME_JSON) {
		t.Fatalf("Exp. route to consume mimetype json")
	}
}

func contains(list []string, value string) bool {
	for _, entry := range list {
		if entry == value {
			return true
		}
	}
	return false
}
