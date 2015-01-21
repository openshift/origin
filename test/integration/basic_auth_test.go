package integration

import (
	"log"
	"net/http"
)

func ExampleNewBasicAuthChallenger() {
	challenger := NewBasicAuthChallenger("realm", []User{{"username", "password", "Brave Butcher", "cowardly_butcher@example.org"}}, NewIdentifyingHandler())
	http.Handle("/", challenger)
	log.Printf("Auth server listening on http://localhost:1234")
	log.Fatal(http.ListenAndServe(":1234", nil))
}
