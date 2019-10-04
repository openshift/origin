package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/lestrrat/go-jsschema"
	"github.com/lestrrat/go-jsschema/validator"
)

func main() {
	os.Exit(_main())
}

func usage() {
	fmt.Printf("jsschema [schema file] [target file]\n")
}

func dumpJSON(v interface{}) error {
	buf, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Printf("failed to encode to JSON: %s", err)
		return err
	}

	os.Stdout.Write(buf)
	os.Stdout.Write([]byte{'\n'})
	return nil
}

func _main() int {
	if len(os.Args) < 2 {
		usage()
		return 1
	}

	schemaf, err := os.Open(os.Args[1])
	if err != nil {
		log.Printf("failed to open schema: %s", err)
		return 1
	}
	defer schemaf.Close()

	s, err := schema.Read(schemaf)
	if err != nil {
		log.Printf("failed to read schema: %s", err)
		return 1
	}

	if err := dumpJSON(s); err != nil {
		return 1
	}

	if len(os.Args) < 3 {
		return 0
	}

	f, err := os.Open(os.Args[2])
	if err != nil {
		log.Printf("failed to open data: %s", err)
		return 1
	}
	defer f.Close()

	in, err := ioutil.ReadAll(f)
	if err != nil {
		log.Printf("failed to read data: %s", err)
		return 1
	}

	var v interface{}
	if err := json.Unmarshal(in, &v); err != nil {
		log.Printf("failed to decode data: %s", err)
		return 1
	}

	valid := validator.New(s)
	if err := valid.Validate(v); err != nil {
		log.Printf("validation failed")
		return 1
	}

	if err := dumpJSON(v); err != nil {
		return 1
	}

	return 0
}
