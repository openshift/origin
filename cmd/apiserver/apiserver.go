package main

import (
	"log"
	"net/http"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/service"
)

func main() {
	storage := map[string]apiserver.RESTStorage{
		"services": service.NewRESTStorage(service.MakeMemoryRegistry()),
	}

	s := &http.Server{
		Addr:           "127.0.0.1:8081",
		Handler:        apiserver.New(storage, api.Codec, "/osapi/v1beta1"),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
}
