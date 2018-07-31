package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
)

func main() {
	re := regexp.MustCompile(`([\W])(quay\.io/coreos[/\w\-]*)(\:[a-zA-Z\d][a-zA-Z\d\-_]*[a-zA-Z\d]|@\w+:\w+)?`)
	data, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	out := re.ReplaceAllFunc(data, func(data []byte) []byte {
		fmt.Fprintf(os.Stderr, "found: %s\n", string(data))
		return data
	})
	fmt.Println(string(out))
}
