package router

import "testing"


//Test that when removing an alias that only the single alias is removed and not the entire host alias structure for the
//frontend
func TestRemoveAlias(t *testing.T){
	router := &Routes{
		make(map[string]Frontend),
	}

	router.AddAlias("alias1", "frontend1")
	router.AddAlias("alias2", "frontend1")

	frontend := router.GlobalRoutes["frontend1"]

	if len(frontend.HostAliases) != 2{
		t.Error("Expected 2 aliases got %i", len(frontend.HostAliases))
	}

	router.RemoveAlias("alias1", "frontend1")

	frontend = router.GlobalRoutes["frontend1"]

	if len(frontend.HostAliases) != 1 {
		t.Error("Expected 1 aliases got %i", len(frontend.HostAliases))
	}

	alias := frontend.HostAliases[0]

	if alias != "alias2"{
		t.Error("Expected to have alias2 remaining, found %s", alias)
	}
}

//test deleting a frontend removes it from global routes
func TestDeleteFrontend(t *testing.T){
	router := &Routes{
		make(map[string]Frontend),
	}

	router.AddAlias("alias1", "frontend1")

	frontend, ok := router.GlobalRoutes["frontend1"]

	if !ok {
		t.Error("Expected to find frontend")
	}

	if len(frontend.HostAliases) != 1{
		t.Error("Expected 1 host alias")
	}

	router.DeleteFrontend("frontend1")

	frontend, ok = router.GlobalRoutes["frontend1"]

	if ok {
		t.Error("Expected to not find frontend but it was found")
	}
}
