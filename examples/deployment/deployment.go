package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	version := "v1"
	if len(os.Args) > 1 {
		version = os.Args[1]
	}
	subtitle := os.Getenv("SUBTITLE")
	color := os.Getenv("COLOR")
	if len(color) == 0 {
		color = "#303030"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Deployment Demonstration</title>
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    HTML{height:100%%;}
    BODY{font-family:Helvetica,Arial;display:flex;display:-webkit-flex;align-items:center;justify-content:center;-webkit-align-items:center;-webkit-box-align:center;-webkit-justify-content:center;height:100%%;}
    .box{background:%[3]s;color:white;text-align:center;border-radius:10px;display:inline-block;}
    H1{font-size:10em;line-height:1.5em;margin:0 0.5em;}
    H2{margin-top:0;}
  </style>
</head>
<body>
<div class="box"><h1>%[1]s</h1><h2>%[2]s</h2></div>
</body>
</html>`, version, subtitle, color)
	})

	http.HandleFunc("/_healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	log.Printf("Listening on :8080 at %s ...", version)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
