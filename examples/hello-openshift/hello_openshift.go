package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello OpenShift!")
}

func echoHandler(port string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, "<html><body><h1>Hello World on Port %s</h1>", port)
		for k, v := range req.Header {
			fmt.Fprintf(w, "Header %s ==> %s<br/>", k, v)
		}
		fmt.Fprint(w, "<br/>")
		for k, v := range req.URL.Query() {
			fmt.Fprintf(w, "Parameter %s ==> %s<br/>", k, v)
		}
		fmt.Fprint(w, "</body></html>")
	}
}

func runServer(port string) {
	fmt.Printf("serving on %s\n", port)

	mux := http.NewServeMux()
	mux.HandleFunc("/echo", echoHandler(port))
	mux.HandleFunc("/", helloHandler)

	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func runServers() {
	ports := []string{"8080", "8888"}
	if len(os.Args) > 1 {
		ports = os.Args[1:]
	}

	for _, port := range ports {
		go runServer(port)
	}
}

func checkEnvVars() {
	if len(os.Getenv("FAIL")) > 0 {
		log.Fatal("FAIL set")
	}
}

func main() {
	checkEnvVars()
	runServers()

	i := 0
	for {
		fmt.Printf("%s: Looping for %d times\n", time.Now().Format(time.UnixDate), i)
		time.Sleep(5 * time.Second)
		i++
	}
}
