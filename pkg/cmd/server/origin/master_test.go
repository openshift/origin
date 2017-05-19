package origin

import (
	"net/http"
	"testing"

	"github.com/emicklei/go-restful"

	"k8s.io/apiserver/pkg/server/mux"

	"github.com/openshift/origin/pkg/cmd/server/api"
)

func TestInitializeOpenshiftAPIVersionRouteHandler(t *testing.T) {
	apiContainer := mux.NewAPIContainer(http.NewServeMux(), api.Codecs, mux.NewPathRecorderMux())
	initAPIVersionRoute(apiContainer, "oapi", "v1")

	wss := apiContainer.RegisteredWebServices()
	if len(wss) != 1 {
		t.Fatalf("Exp. the OSAPI webservice but found none")
	}
	routes := wss[0].Routes()
	if len(routes) != 1 {
		t.Fatalf("Expected the OSAPI route but found none")
	}
	route := routes[0]
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
