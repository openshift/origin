package main

import (
	"fmt"
	"net/http"
)

func main() {
	port := 8080
	path := "/home/deads/workspaces/origin/src/github.com/openshift/origin/e2echart"

	handler := http.FileServer(http.Dir(path))
	fmt.Printf(fmt.Sprintf("Listening on :%d\n", port))
	panic(http.ListenAndServe(fmt.Sprintf(":%d", port), handler))
}
