package main

import (
	"fmt"
	"net/http"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Hello OpenShift!")
}

func main() {
	http.HandleFunc("/", helloHandler)

	go func() {
		fmt.Println("serving on 8080")
		err := http.ListenAndServe(":8080", nil)
		if err != nil {
			panic("ListenAndServe: " + err.Error())
		}
	}()

	go func() {
		fmt.Println("serving on 8888")
		err := http.ListenAndServe(":8888", nil)
		if err != nil {
			panic("ListenAndServe: " + err.Error())
		}
	}()
	select {}
}
