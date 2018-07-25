package main

import (
	"encoding/json"
	"log"
)

import "io/ioutil"
import "os"

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: jsonformat.go <filename>")
	}

	byt, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Fatalf("ERROR: Unable to read file: %v\n", os.Args[1])
	}

	var dat map[string]interface{}

	if err := json.Unmarshal(byt, &dat); err != nil {
		log.Fatalf("ERROR: Invalid JSON file  '%v': %v\n", os.Args[1], err)
	}

	if output, err := json.MarshalIndent(dat, "", "  "); err != nil {
		log.Fatalf("ERROR: Unable to indent JSON file: %v\n", os.Args[1])
	} else {
		os.Stdout.Write(append(output, '\n'))
	}
}
