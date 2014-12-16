package main

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"io"
	"net/http"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello OpenShift!")
}

func echoSocketHandler(ws *websocket.Conn) {
	io.Copy(ws, ws)
}

func main() {
	http.HandleFunc("/", helloHandler)
	http.Handle("/echo", websocket.Handler(echoSocketHandler))

	fmt.Println("Started, serving at 8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
