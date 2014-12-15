package templaterouter

import (
	"testing"
)

func emptyRouter() templateRouter {
	return templateRouter{state: map[string]Frontend{}}
}

const (
	key = "frontend1"
)

func TestRemoveAlias(t *testing.T) {
	router := emptyRouter()
	router.CreateFrontend(key, "http://127.0.0.1/test_frontend")
	router.AddAlias(key, "alias1")
	// Adding the same alias twice to also check that adding an existing alias does not add it twice
	router.AddAlias(key, "alias1")
	router.AddAlias(key, "alias2")

	if len(router.state[key].HostAliases) != 3 {
		t.Errorf("Expected 3 aliases got %v: %v", len(router.state[key].HostAliases), router.state[key].HostAliases)
	}

	router.RemoveAlias(key, "alias1")

	if len(router.state[key].HostAliases) != 2 {
		t.Errorf("Expected 2 aliases got %v", len(router.state[key].HostAliases))
	}

	alias := router.state[key].HostAliases[1]
	if alias != "alias2" {
		t.Error("Expected to have alias2 remaining, found %s", alias)
	}
}

func TestDeleteFrontend(t *testing.T) {
	router := emptyRouter()
	router.CreateFrontend(key, "http://127.0.0.1/test_frontend")
	router.AddAlias(key, "alias1")

	_, ok := router.state[key]
	if !ok {
		t.Error("Expected to find frontend")
	}

	if len(router.state[key].HostAliases) != 2 {
		t.Error("Expected 1 host alias")
	}

	router.DeleteFrontend(key)

	_, ok = router.state[key]
	if ok {
		t.Error("Expected to not find frontend but it was found")
	}
}

func TestAddRoute(t *testing.T) {
	router := emptyRouter()
	router.CreateFrontend(key, "http://127.0.0.1/test_frontend")
	_, ok := router.FindFrontend(key)
	if !ok {
		t.Error("Expected frontend to be created")
	}

	protocols := []string{"http", "https", "Sti"}
	endpoint := Endpoint{ID: "my-endpoint", IP: "127.0.0.1", Port: "8080"}
	endpoints := []Endpoint{endpoint}

	backend := Backend{FePath: "fe_server1", BePath: "be_server1", Protocols: protocols}
	router.AddRoute(key, &backend, endpoints)

	if len(router.state[key].Backends) == 0 {
		t.Error("Expected that frontend has more routes after RouteAdd ")
	}

	// Creating and endpoint with empty port or IP to check that it is not added
	endpointEmpty := Endpoint{ID: "my-endpoint", IP: "", Port: ""}
	endpointsEmpty := []Endpoint{endpointEmpty}

	router.AddRoute(key, &backend, endpointsEmpty)
	if len(router.state[key].Backends) != 1 {
		t.Error("Expected that frontend has only backend ")
	}

	// Adding the same route twice to check and that the backend end is not added twice
	router.AddRoute(key, &backend, endpoints)
	if len(router.state[key].Backends) != 1 {
		t.Error("Expected that frontend has only backend ")
	}

}

func TestDeleteBackends(t *testing.T) {
	router := emptyRouter()

	router.CreateFrontend(key, "http://127.0.0.1/test_frontend")
	router.AddAlias(key, "alias1")

	frontend := router.state[key]

	protocols := []string{"http", "https", "Sti"}
	endpoint := Endpoint{ID: "my-endpoint", IP: "127.0.0.1", Port: "8080"}
	endpoints := []Endpoint{endpoint}

	backend := Backend{FePath: "fe_server1", BePath: "be_server1", Protocols: protocols}
	router.AddRoute(key, &backend, endpoints)

	frontend, ok := router.state[key]
	if !ok {
		t.Error("Expected to find frontend")
	}

	router.DeleteBackends(key)
	frontend = router.state[key]
	if len(frontend.Backends) != 0 {
		t.Error("Expected that frontend has empty backend")
	}

	// Deleting an non existing frontend
	router.DeleteBackends("frontend1_NOT_EXISTENT")
	frontend = router.state["frontend1_NOT_EXISTENT"]
	if len(frontend.Backends) != 0 {
		t.Error("Expected that frontend has empty backend")
	}

}

func TestCreateFrontend(t *testing.T) {
	router := emptyRouter()

	router.CreateFrontend(key, "http://127.0.0.1/test_frontend")
	frontend := router.state[key]

	if len(frontend.HostAliases) != 1 {
		t.Error("Expected that frontend has 1 host aliases")
	}

}

func TestCreateFrontendWithEmptyUrl(t *testing.T) {
	router := emptyRouter()
	router.CreateFrontend(key, "")
	frontend := router.state[key]

	if len(frontend.HostAliases) != 0 {
		t.Error("Expected that frontend has no host aliases")
	}

}

func TestFindFrontend(t *testing.T) {
	router := emptyRouter()

	router.CreateFrontend(key, "")
	_, ok := router.FindFrontend(key)
	if !ok {
		t.Error("Failure to find frontend")
	}
}
