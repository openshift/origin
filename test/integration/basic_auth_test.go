package integration

import (
	"log"
	"net/http"

	"github.com/golang/glog"
)

func ExampleNewBasicAuthChallenger() {
	challenger := NewBasicAuthChallenger("realm", []User{{"username", "password", "Brave Butcher", "cowardly_butcher@example.org"}}, NewIdentifyingHandler())
	http.Handle("/", challenger)
	glog.Infoln("Auth server listening on http://localhost:1234")
	log.Fatal(http.ListenAndServe(":1234", nil))
}
