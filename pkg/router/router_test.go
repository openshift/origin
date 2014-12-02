package router

import (
	"fmt"
<<<<<<< HEAD
	"math/rand"
=======
>>>>>>> upstream/master
	"testing"
)

//Test that when removing an alias that only the single alias is removed and not the entire host alias structure for the
//frontend
func TestRemoveAlias(t *testing.T) {
<<<<<<< HEAD
	suffix := rand.Intn(1000000)
	filename := fmt.Sprintf("/tmp/test-%v", suffix)
	router := NewRoutes(filename)
=======
	router := &Routes{
		make(map[string]Frontend),
	}

>>>>>>> upstream/master
	router.CreateFrontend("frontend1", "http://127.0.0.1/test_frontend")
	router.AddAlias("alias1", "frontend1")
	// Adding the same alias twice to also check that adding and existing alias does not add it twice
	router.AddAlias("alias1", "frontend1")
	router.AddAlias("alias2", "frontend1")

	frontend := router.GlobalRoutes["frontend1"]
	// Creating a frontend with an URL autmatically adds an alias
	if len(frontend.HostAliases) != 3 {
		t.Error("Expected 2 aliases got %i", len(frontend.HostAliases))
	}

	router.RemoveAlias("alias1", "frontend1")

	frontend = router.GlobalRoutes["frontend1"]

	if len(frontend.HostAliases) != 2 {
		t.Error("Expected 1 aliases got %i", len(frontend.HostAliases))
	}

	alias := frontend.HostAliases[1]
	if alias != "alias2" {
		t.Error("Expected to have alias2 remaining, found %s", alias)
	}
}

//test deleting a frontend removes it from global routes
func TestDeleteFrontend(t *testing.T) {
<<<<<<< HEAD
	suffix := rand.Intn(1000000)
	filename := fmt.Sprintf("/tmp/test-%v", suffix)
	router := NewRoutes(filename)

=======
	router := &Routes{
		make(map[string]Frontend),
	}
>>>>>>> upstream/master
	router.CreateFrontend("frontend1", "http://127.0.0.1/test_frontend")
	router.AddAlias("alias1", "frontend1")

	frontend, ok := router.GlobalRoutes["frontend1"]

	if !ok {
		t.Error("Expected to find frontend")
	}

	if len(frontend.HostAliases) != 2 {
		t.Error("Expected 1 host alias")
	}

	router.DeleteFrontend("frontend1")

	frontend, ok = router.GlobalRoutes["frontend1"]

	if ok {
		t.Error("Expected to not find frontend but it was found")
	}
}

//Test that when adding a route, the route is indeed added
func TestAddRoute(t *testing.T) {
<<<<<<< HEAD
	suffix := rand.Intn(1000000)
	filename := fmt.Sprintf("/tmp/test-%v", suffix)
	router := NewRoutes(filename)

=======
	router := &Routes{
		make(map[string]Frontend),
	}
>>>>>>> upstream/master
	router.CreateFrontend("frontend1", "http://127.0.0.1/test_frontend")

	frontend := router.GlobalRoutes["frontend1"]
	frontend.EndpointTable = make(map[string]Endpoint)
	frontend.Backends = make(map[string]Backend)

	protocols := []string{"http", "https", "Sti"}
	endpoint := Endpoint{ID: "my-endpoint", IP: "127.0.0.1", Port: "8080"}
	endpoints := []Endpoint{endpoint}
	fmt.Printf("Frontend before adding routes %+v\n", frontend)

	router.AddRoute("frontend1", "fe_server1", "be_server1", protocols, endpoints)

	router.ReadRoutes()
	frontendAfter := router.GlobalRoutes["frontend1"]
	fmt.Printf("Frontend after adding routes %+v\n", frontendAfter)

	if len(frontendAfter.Backends) == 0 || len(frontendAfter.Backends) < len(frontend.Backends) {
		t.Error("Expected that frontend has more routes after RouteAdd ")
	}

	// Creating and endpoint with empty port or IP to check that it is not added
	endpointEmpty := Endpoint{ID: "my-endpoint", IP: "", Port: ""}
	endpointsEmpty := []Endpoint{endpointEmpty}
	fmt.Printf("Frontend before adding routes %+v\n", frontend)

	router.AddRoute("frontend1", "fe_server1", "be_server1", protocols, endpointsEmpty)
	if len(frontendAfter.Backends) != 1 {
		t.Error("Expected that frontend has only backend ")
	}

	// Adding the same route twice to check and that the backend end is not added twice
	router.AddRoute("frontend1", "fe_server1", "be_server1", protocols, endpoints)
	if len(frontendAfter.Backends) != 1 {
		t.Error("Expected that frontend has only backend ")
	}

}

//Test that when adding a route, the route is indeed added
func TestReadRoute(t *testing.T) {
<<<<<<< HEAD
	suffix := rand.Intn(1000000)
	filename := fmt.Sprintf("/tmp/test-%v", suffix)
	router := NewRoutes(filename)

=======
	router := &Routes{
		make(map[string]Frontend),
	}
>>>>>>> upstream/master
	router.AddAlias("alias1", "frontend1")

}

//test deleting a backend removes it from global routes
func TestDeleteBackends(t *testing.T) {
<<<<<<< HEAD
	suffix := rand.Intn(1000000)
	filename := fmt.Sprintf("/tmp/test-%v", suffix)
	router := NewRoutes(filename)

=======
	router := &Routes{
		make(map[string]Frontend),
	}
>>>>>>> upstream/master
	router.CreateFrontend("frontend1", "http://127.0.0.1/test_frontend")
	router.AddAlias("alias1", "frontend1")

	protocols := []string{"http", "https", "Sti"}
	endpoint := Endpoint{ID: "my-endpoint", IP: "127.0.0.1", Port: "8080"}
	endpoints := []Endpoint{endpoint}

	router.AddRoute("frontend1", "fe_server1", "be_server1", protocols, endpoints)
	frontend, ok := router.GlobalRoutes["frontend1"]
	if !ok {
		t.Error("Expected to find frontend")
	}

	router.DeleteBackends("frontend1")
	frontend = router.GlobalRoutes["frontend1"]
	if len(frontend.Backends) != 0 {
		t.Error("Expected that frontend has empty backend")
	}

	// Deleting an non existing frontend
	router.DeleteBackends("frontend1_NOT_EXISTENT")
	frontend = router.GlobalRoutes["frontend1_NOT_EXISTENT"]
	if len(frontend.Backends) != 0 {
		t.Error("Expected that frontend has empty backend")
	}

}

//test creation of a frontend
func TestCreateFrontend(t *testing.T) {
<<<<<<< HEAD
	suffix := rand.Intn(1000000)
	filename := fmt.Sprintf("/tmp/test-%v", suffix)
	router := NewRoutes(filename)

=======
	router := &Routes{
		make(map[string]Frontend),
	}
>>>>>>> upstream/master
	router.CreateFrontend("frontend1", "http://127.0.0.1/test_frontend")
	frontend := router.GlobalRoutes["frontend1"]
	fmt.Printf("Frontend after creation %+v\n", frontend)

	if len(frontend.HostAliases) != 1 {
		t.Error("Expected that frontend has 1 host aliases")
	}

}

//test creation of a frontend
func TestCreateFrontendWithEmptyUrl(t *testing.T) {
<<<<<<< HEAD
	suffix := rand.Intn(1000000)
	filename := fmt.Sprintf("/tmp/test-%v", suffix)
	router := NewRoutes(filename)

=======
	router := &Routes{
		make(map[string]Frontend),
	}
>>>>>>> upstream/master
	router.CreateFrontend("frontend1", "")
	frontend := router.GlobalRoutes["frontend1"]
	fmt.Printf("Frontend after creation %+v\n", frontend)

	if len(frontend.HostAliases) != 0 {
		t.Error("Expected that frontend has no host aliases")
	}

}

//test creation of a frontend
func TestAddAliasWithNoFrontend(t *testing.T) {
<<<<<<< HEAD
	suffix := rand.Intn(1000000)
	filename := fmt.Sprintf("/tmp/test-%v", suffix)
	router := NewRoutes(filename)

=======
	router := &Routes{
		make(map[string]Frontend),
	}
>>>>>>> upstream/master
	// router.CreateFrontend("frontend1", "")
	_, err := router.AddAlias("alias1", "frontend1")
	if err == nil {
		t.Error("Adding an alias to a non existing fronted must fail")
	}
}

func TestAddRouteWithNoFrontend(t *testing.T) {
<<<<<<< HEAD
	suffix := rand.Intn(1000000)
	filename := fmt.Sprintf("/tmp/test-%v", suffix)
	router := NewRoutes(filename)

=======
	router := &Routes{
		make(map[string]Frontend),
	}
>>>>>>> upstream/master
	// router.CreateFrontend("frontend1", "")
	protocols := []string{"http", "https", "Sti"}
	endpoint := Endpoint{ID: "my-endpoint", IP: "127.0.0.1", Port: "8080"}
	endpoints := []Endpoint{endpoint}
	_, err := router.AddRoute("frontend1", "fe_server1", "be_server1", protocols, endpoints)

	if err == nil {
		t.Error("Adding a route to a non existing fronted must fail")
	}
}

func TestRemoveAliasWithNoFrontend(t *testing.T) {
<<<<<<< HEAD
	suffix := rand.Intn(1000000)
	filename := fmt.Sprintf("/tmp/test-%v", suffix)
	router := NewRoutes(filename)

=======
	router := &Routes{
		make(map[string]Frontend),
	}
>>>>>>> upstream/master
	// router.CreateFrontend("frontend1", "")
	_, err := router.RemoveAlias("alias1", "frontend1")
	if err == nil {
		t.Error("Removing an alias to a non existing fronted must fail")
	}
}

func TestWriteRoutes(t *testing.T) {
<<<<<<< HEAD
	router := NewRoutes("/dev/null")
	_, err := router.WriteRoutes()
=======
	router := &Routes{}
	_, err := router.WriteRoutes("/dev/null")
>>>>>>> upstream/master
	if err != nil {
		t.Error("Writing route file failed")
	}
}

func TestWriteRoutesFailure(t *testing.T) {
<<<<<<< HEAD
	router := NewRoutes("/root/AAAtmpAAA/test")
	_, err := router.WriteRoutes()
=======
	router := &Routes{}
	_, err := router.WriteRoutes("/root/AAAtmpAAA/test")
>>>>>>> upstream/master
	if err == nil {
		t.Error("Writing route file should have failed")
	}
}

func TestReadRoutes(t *testing.T) {
<<<<<<< HEAD
	suffix := rand.Intn(1000000)
	filename := fmt.Sprintf("/tmp/test-%v", suffix)
	router := NewRoutes(filename)

	_, err := router.WriteRoutes()
	if err != nil {
		t.Error("Writing route file failed")
	}
	_, err = router.ReadRoutes()
=======
	router := &Routes{}
	_, err := router.WriteRoutes("/tmp/test")
	if err != nil {
		t.Error("Writing route file failed")
	}
	_, err = router.ReadRoutes("/tmp/test")
>>>>>>> upstream/master
	if err != nil {
		t.Error("Reading route file failed")
	}
}

func TestReadRoutesFailure(t *testing.T) {
<<<<<<< HEAD
	suffix := rand.Intn(1000000)
	filename := fmt.Sprintf("/tmp/test-%v", suffix)
	router := NewRoutes(filename)

	_, err := router.WriteRoutes()
	if err != nil {
		t.Error("Writing route file failed")
	}
	router2 := NewRoutes("/dev/tmp/test")
	_, err = router2.ReadRoutes()
=======
	router := &Routes{}
	_, err := router.WriteRoutes("/tmp/test")
	if err != nil {
		t.Error("Writing route file failed")
	}
	_, err = router.ReadRoutes("/dev/tmp/test")
>>>>>>> upstream/master
	if err == nil {
		t.Error("Reading route should have failed")
	}
}

func TestFindFrontend(t *testing.T) {
<<<<<<< HEAD
	suffix := rand.Intn(1000000)
	filename := fmt.Sprintf("/tmp/test-%v", suffix)
	router := NewRoutes(filename)

=======
	router := &Routes{make(map[string]Frontend)}
>>>>>>> upstream/master
	router.CreateFrontend("frontend1", "")
	_, ok := router.FindFrontend("frontend1")
	if !ok {
		t.Error("Failre to find frontend")
	}
}
