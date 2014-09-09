package examples

import (
	"fmt"
	"net"
	"net/http"
	"testing"
)

func TestRemoteValueGenerator(t *testing.T) {
	generator := NewRemoteValueGenerator()

	_, err := generator.GenerateValue("[GET:http://api.example.com/new]")
	if err == nil {
		t.Errorf("Expected error while fetching non-existent remote.")
	}
}

func TestFakeRemoteValueGenerator(t *testing.T) {
	// Run the fake remote server
	http.HandleFunc("/v1/value/generate", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "NewRandomString")
	})
	listener, _ := net.Listen("tcp", ":12345")
	go http.Serve(listener, nil)

	generator := NewRemoteValueGenerator()

	value, err := generator.GenerateValue("[GET:http://127.0.0.1:12345/v1/value/generate]")
	if err != nil {
		t.Errorf(err.Error())
	}
	if value != "NewRandomString" {
		t.Errorf("Failed to fetch remote value using GET.")
	}
}
